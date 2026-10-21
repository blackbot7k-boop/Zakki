package prefs

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStorageGitDecodeStrict(t *testing.T) {
	for _, raw := range []string{
		`{"storage":null}`, `{"git":null}`,
		`{"storage":{"dataFile":null}}`, `{"storage":{"saveIntervalMs":null}}`,
		`{"git":{"autoCommit":null}}`, `{"git":{"autoPush":null}}`,
		`{"git":{"commitMinutes":null}}`, `{"git":{"pushMinutes":null}}`,
		`{"storage":{"unknown":1}}`, `{"git":{"gitAutoPush":true}}`,
		`{"storage":{"dataFile":123}}`, `{"storage":{"saveIntervalMs":"2000"}}`,
		`{"storage":{"saveIntervalMs":2000.5}}`, `{"storage":{"saveIntervalMs":0}}`,
		`{"storage":{"saveIntervalMs":99}}`, `{"storage":{"saveIntervalMs":60001}}`,
		`{"git":{"autoCommit":"true"}}`, `{"git":{"autoPush":1}}`,
		`{"git":{"commitMinutes":0}}`, `{"git":{"commitMinutes":1441}}`,
		`{"git":{"pushMinutes":0}}`, `{"git":{"pushMinutes":1441}}`,
		`{"git":{"autoPush":true}}`,
		`{"version":12,"advanced":{"autosaveDelayMs":800}}`,
	} {
		t.Run(raw, func(t *testing.T) {
			if _, err := Decode([]byte(raw), nil); err == nil {
				t.Fatalf("非法配置被接受: %s", raw)
			}
		})
	}
	for _, raw := range []string{`{"version":12}`, `{"version":12,"storage":{},"git":{}}`} {
		got, err := Decode([]byte(raw), nil)
		if err != nil {
			t.Fatal(err)
		}
		def := Default()
		if got.Storage != def.Storage || got.Git != def.Git {
			t.Fatalf("缺键应补默认: %+v %+v", got.Storage, got.Git)
		}
	}
}

func TestStoragePaths(t *testing.T) {
	for _, path := range []string{" ", "\t\n", "relative.json", filepath.Join("relative", "zakki.json"), filepath.Join(t.TempDir(), "bad\x00.json"), " " + filepath.Join(t.TempDir(), "zakki.json")} {
		p := Default()
		p.Storage.DataFile = path
		if err := p.Validate(nil); err == nil {
			t.Fatalf("非法路径被接受: %q", path)
		}
		raw, err := json.Marshal(p)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := Decode(raw, nil); err == nil {
			t.Fatalf("解码接受非法路径: %q", path)
		}
	}
	for _, path := range []string{"", filepath.Join(t.TempDir(), "不存在的目录", "带 空格.json")} {
		p := Default()
		p.Storage.DataFile = path
		if err := p.Validate(nil); err != nil {
			t.Fatalf("不应探测磁盘或拒绝合法路径 %q: %v", path, err)
		}
		assertFontPreferencesSaved(t, p)
	}
}

func TestStorageGitRoundTrip(t *testing.T) {
	for _, interval := range []int{100, 2000, 60000} {
		for _, minutes := range []int{1, 1440} {
			for _, flags := range [][2]bool{{false, false}, {true, false}, {true, true}} {
				p := Default()
				p.Storage = Storage{DataFile: filepath.Join(t.TempDir(), "zakki.json"), SaveIntervalMS: interval}
				p.Git = Git{AutoCommit: flags[0], AutoPush: flags[1], CommitMinutes: minutes, PushMinutes: minutes}
				assertFontPreferencesSaved(t, p)
			}
		}
	}
	path := filepath.Join(t.TempDir(), "preferences.json")
	if err := NewStore(path, 0).WriteInitial(Default()); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), `"backup"`) || strings.Contains(string(raw), `"autosaveDelayMs"`) {
		t.Fatalf("新配置不应写出旧字段: %s", raw)
	}
}
