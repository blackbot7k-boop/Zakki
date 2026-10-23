package atomicfile

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestWriteBytesReplacesTargetWithoutFixedTempCollision(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "data.bin")
	if err := WriteBytes(path, []byte("old")); err != nil {
		t.Fatal(err)
	}
	fixedTemp := path + ".tmp"
	if err := os.WriteFile(fixedTemp, []byte("用户文件"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := WriteBytes(path, []byte("new content")); err != nil {
		t.Fatal(err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(raw); got != "new content" {
		t.Fatalf("目标内容 = %q, want new content", got)
	}
	if got, err := os.ReadFile(fixedTemp); err != nil || string(got) != "用户文件" {
		t.Fatalf("固定 .tmp 文件不得被占用或覆盖: %q, %v", got, err)
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if got := info.Mode().Perm(); got != 0o644 {
			t.Fatalf("目标权限 = %o, want 644", got)
		}
	}
	assertNoRandomTemps(t, filepath.Dir(path), filepath.Base(path))
}

func TestWriteBytesPublishFailureCleansRandomTemp(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "target")
	if err := os.Mkdir(path, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := WriteBytes(path, []byte("不能发布")); err == nil {
		t.Fatal("用文件替换目录必须失败")
	}

	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		t.Fatalf("失败不得破坏原目标目录: info=%v err=%v", info, err)
	}
	assertNoRandomTemps(t, dir, filepath.Base(path))
}

func TestWriteBytesReportsCommittedDirectorySyncFailure(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "data.bin")
	if err := os.WriteFile(path, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	wantErr := errors.New("directory sync failed")

	err := writeBytes(path, []byte("new"), func(string) error { return wantErr })

	if !IsCommitted(err) || !errors.Is(err, wantErr) {
		t.Fatalf("错误 = %v, want committed directory sync error", err)
	}
	if got, readErr := os.ReadFile(path); readErr != nil || string(got) != "new" {
		t.Fatalf("rename 已提交的目标内容 = %q, err=%v", got, readErr)
	}
	assertNoRandomTemps(t, dir, filepath.Base(path))
}

func TestWriteJSONFormatsAndTerminatesLine(t *testing.T) {
	path := filepath.Join(t.TempDir(), "deep", "prefs.json")
	value := struct {
		Name string `json:"name"`
	}{Name: "Zakki"}

	if err := WriteJSON(path, value); err != nil {
		t.Fatal(err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(raw), "{\n  \"name\": \"Zakki\"\n}\n"; got != want {
		t.Fatalf("JSON = %q, want %q", got, want)
	}
	assertNoRandomTemps(t, filepath.Dir(path), filepath.Base(path))
}

func assertNoRandomTemps(t *testing.T, dir, base string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	prefix := "." + base + ".tmp-"
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), prefix) {
			t.Fatalf("残留临时文件: %s", entry.Name())
		}
	}
}
