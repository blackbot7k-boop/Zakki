package fonts

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func testResource(data []byte, url string) Resource {
	digest := sha256.Sum256(data)
	return Resource{ID: "test-font", Family: "Test Font", URL: url, SHA256: hex.EncodeToString(digest[:]), Bytes: int64(len(data)), License: "OFL-1.1"}
}

func assertOnlyCache(t *testing.T, s Store, want string) {
	t.Helper()
	entries, err := os.ReadDir(s.Dir)
	if errors.Is(err, os.ErrNotExist) && want == "" {
		return
	}
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.Name() != want {
			t.Errorf("unexpected file: %s", entry.Name())
		}
	}
	if want != "" && len(entries) != 1 {
		t.Errorf("file count got %d, want 1", len(entries))
	}
}

func TestEnsureStreamingAndCache(t *testing.T) {
	data := bytes.Repeat([]byte("font"), 25000)
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		for start := 0; start < len(data); start += 4096 {
			end := min(start+4096, len(data))
			if _, err := w.Write(data[start:end]); err != nil {
				return
			}
			w.(http.Flusher).Flush()
		}
	}))
	defer server.Close()
	s := Store{Dir: t.TempDir(), Client: server.Client()}
	resource := testResource(data, server.URL)
	var progress []Progress
	got, err := s.ensure(context.Background(), resource, func(p Progress) { progress = append(progress, p) })
	if err != nil || !bytes.Equal(got, data) {
		t.Fatalf("ensure got %d bytes, %v", len(got), err)
	}
	if len(progress) < 4 || progress[0] != (Progress{Stage: "downloading", Total: resource.Bytes}) {
		t.Fatalf("invalid progress: %+v", progress)
	}
	previous := int64(0)
	for _, p := range progress {
		if p.Total != resource.Bytes || p.Downloaded < previous || p.Downloaded > p.Total {
			t.Errorf("invalid progress: %+v", p)
		}
		previous = p.Downloaded
	}
	if p := progress[len(progress)-2]; p.Stage != "verifying" || p.Downloaded != resource.Bytes {
		t.Errorf("verification got %+v", p)
	}
	if p := progress[len(progress)-1]; p.Stage != "ready" || p.Downloaded != resource.Bytes {
		t.Errorf("ready got %+v", p)
	}
	info, err := os.Stat(s.Path(resource))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Errorf("permissions got %o, want 600", info.Mode().Perm())
	}
	progress = nil
	got, err = s.ensure(context.Background(), resource, func(p Progress) { progress = append(progress, p) })
	if err != nil || !bytes.Equal(got, data) || requests.Load() != 1 {
		t.Fatalf("cache got %d bytes, %v, requests %d", len(got), err, requests.Load())
	}
	if len(progress) != 2 || progress[0].Stage != "verifying" || progress[1].Stage != "ready" {
		t.Errorf("cache progress: %+v", progress)
	}
	assertOnlyCache(t, s, resource.ID+".ttf")
}

func TestEnsureFailures(t *testing.T) {
	data := []byte("correct font data")
	for _, tc := range []struct {
		name   string
		status int
		body   []byte
		flush  bool
	}{
		{"http", http.StatusServiceUnavailable, data, false},
		{"truncated", http.StatusOK, data[:4], true},
		{"overlong", http.StatusOK, append(bytes.Clone(data), 'x'), true},
		{"checksum", http.StatusOK, bytes.Repeat([]byte("x"), len(data)), true},
		{"content-length", http.StatusOK, data[:4], false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
				if tc.flush {
					w.(http.Flusher).Flush()
				}
				_, _ = w.Write(tc.body)
			}))
			defer server.Close()
			s := Store{Dir: t.TempDir(), Client: server.Client()}
			resource := testResource(data, server.URL)
			var ready bool
			if _, err := s.ensure(context.Background(), resource, func(p Progress) { ready = ready || p.Stage == "ready" }); err == nil {
				t.Fatal("expected failure")
			}
			if ready {
				t.Fatal("reported ready after failure")
			}
			assertOnlyCache(t, s, "")
		})
	}
}

func TestEnsureCancellation(t *testing.T) {
	data := bytes.Repeat([]byte("font"), 20000)
	for _, stage := range []string{"before", "downloading", "verifying"} {
		t.Run(stage, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write(data)
			}))
			defer server.Close()
			s := Store{Dir: t.TempDir(), Client: server.Client()}
			resource := testResource(data, server.URL)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if stage == "before" {
				cancel()
			}
			_, err := s.ensure(ctx, resource, func(p Progress) {
				if p.Stage == stage {
					cancel()
				}
			})
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("got %v, want canceled", err)
			}
			assertOnlyCache(t, s, "")
		})
	}
}

func TestEnsureRequestContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() }))
	defer server.Close()
	s := Store{Dir: t.TempDir(), Client: server.Client()}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	_, err := s.ensure(ctx, testResource([]byte("font"), server.URL), nil)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("got %v, want deadline", err)
	}
	assertOnlyCache(t, s, "")
}

func TestCorruptCacheRedownload(t *testing.T) {
	data := []byte("font data")
	for _, bad := range [][]byte{[]byte("short"), bytes.Repeat([]byte("x"), len(data)), append(bytes.Clone(data), 'x')} {
		t.Run(fmt.Sprintf("%d", len(bad)), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write(data) }))
			defer server.Close()
			s := Store{Dir: t.TempDir(), Client: server.Client()}
			resource := testResource(data, server.URL)
			if err := os.WriteFile(s.Path(resource), bad, 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := s.Read(resource); !errors.Is(err, ErrCorruptCache) {
				t.Fatalf("got %v, want corruption", err)
			}
			got, err := s.ensure(context.Background(), resource, nil)
			if err != nil || !bytes.Equal(got, data) {
				t.Fatalf("redownload got %q, %v", got, err)
			}
			got, err = s.Read(resource)
			if err != nil || !bytes.Equal(got, data) {
				t.Fatalf("read got %q, %v", got, err)
			}
			assertOnlyCache(t, s, resource.ID+".ttf")
		})
	}
}

func TestCacheIOErrorDoesNotDownload(t *testing.T) {
	s := Store{Dir: t.TempDir()}
	resource := testResource([]byte("font"), "http://invalid.example")
	if err := os.Mkdir(s.Path(resource), 0700); err != nil {
		t.Fatal(err)
	}
	_, err := s.ensure(context.Background(), resource, nil)
	if err == nil || errors.Is(err, ErrCorruptCache) || !strings.Contains(err.Error(), "read font cache") {
		t.Fatalf("got %v, want cache I/O error", err)
	}
}

func TestPermissionErrorDoesNotDownload(t *testing.T) {
	s := Store{Dir: t.TempDir()}
	resource := testResource([]byte("font"), "http://invalid.example")
	if err := os.WriteFile(s.Path(resource), []byte("font"), 0000); err != nil {
		t.Fatal(err)
	}
	if _, err := os.ReadFile(s.Path(resource)); err == nil {
		t.Skip("current user bypasses permission checks")
	}
	_, err := s.ensure(context.Background(), resource, nil)
	if !errors.Is(err, os.ErrPermission) {
		t.Fatalf("got %v, want permission error", err)
	}
}

func TestResourceValidation(t *testing.T) {
	s := Store{Dir: t.TempDir()}
	for _, id := range []string{"", "..", "../wenkai", "/wenkai", `a\b`, "A", ".hidden", "a/b", strings.Repeat("a", 65)} {
		r := testResource([]byte("font"), "http://invalid.example")
		r.ID = id
		if got := s.Path(r); got != "" {
			t.Errorf("Path(%q) got %q", id, got)
		}
		if _, err := s.Read(r); !errors.Is(err, ErrInvalidResource) {
			t.Errorf("Read(%q) got %v", id, err)
		}
	}
	r := testResource([]byte("font"), "http://invalid.example")
	if _, err := s.Ensure(context.Background(), r, nil); !errors.Is(err, ErrInvalidResource) {
		t.Fatalf("custom production resource accepted: %v", err)
	}
	for _, field := range []string{"url", "sha", "bytes", "family", "license"} {
		r, _ := Find("fira")
		switch field {
		case "url":
			r.URL = "http://invalid.example"
		case "sha":
			r.SHA256 = strings.Repeat("0", 64)
		case "bytes":
			r.Bytes++
		case "family":
			r.Family = "Dank Mono"
		case "license":
			r.License = "changed"
		}
		if _, err := s.Ensure(context.Background(), r, nil); !errors.Is(err, ErrInvalidResource) {
			t.Errorf("modified %s accepted: %v", field, err)
		}
	}
	r = testResource([]byte("font"), "http://invalid.example")
	r.Family = "../../elsewhere"
	if got := s.Path(r); got != filepath.Join(s.Dir, "test-font.ttf") {
		t.Errorf("family affected path: %s", got)
	}
}

func TestFailedDownloadPreservesOldCache(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("bad")) }))
	defer server.Close()
	s := Store{Dir: t.TempDir(), Client: server.Client()}
	r := testResource([]byte("new"), server.URL)
	old := []byte("old")
	if err := os.WriteFile(s.Path(r), old, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ensure(context.Background(), r, nil); err == nil {
		t.Fatal("expected checksum failure")
	}
	got, err := os.ReadFile(s.Path(r))
	if err != nil || !bytes.Equal(got, old) {
		t.Fatalf("cache changed: %q, %v", got, err)
	}
	assertOnlyCache(t, s, r.ID+".ttf")
}
