package notes

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"
)

func newTestStore(t testing.TB, now func() time.Time) *Store {
	t.Helper()
	dir := t.TempDir()
	if now == nil {
		now = func() time.Time { return time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC) }
	}
	s := NewStore(filepath.Join(dir, "zakki.json"), now, testMigrateOpt())
	t.Cleanup(s.Close)
	return s
}

type testClock struct {
	mu  sync.Mutex
	cur time.Time
}

func newTestClock() *testClock {
	return &testClock{cur: time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)}
}

func (c *testClock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.cur
}

func (c *testClock) advance(d time.Duration) {
	c.mu.Lock()
	c.cur = c.cur.Add(d)
	c.mu.Unlock()
}

func writeRaw(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("写入 %s 失败: %v", path, err)
	}
}

func readRaw(t *testing.T, path string) []byte {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读取 %s 失败: %v", path, err)
	}
	return raw
}

func readFile(t *testing.T, path string) File {
	t.Helper()
	raw := readRaw(t, path)
	var f File
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatalf("解析 %s 失败: %v（内容 %q）", path, err, raw)
	}
	return f
}

func TestSaveNowAtomicAndNestedDir(t *testing.T) {
	t.Run("主文件是合法 JSON 且含最新内容", func(t *testing.T) {
		s := newTestStore(t, nil)
		want := File{
			Version: CurrentDataVersion,
			Active:  5,
			Tabs:    defaultTabs("中文 ✓ # 标题", "", "第三点\n多行", "", "", "", "尾点"),
			Settings: Settings{
				ThemeSource:  "dark",
				AlwaysOnTop:  true,
				HideOnEscape: true,
				Source:       true,
			},
		}
		if err := s.SaveNow(want); err != nil {
			t.Fatalf("SaveNow 报错: %v", err)
		}
		if got := readFile(t, s.DataPath()); !reflect.DeepEqual(got, want) {
			t.Fatalf("落盘内容 = %+v, want %+v", got, want)
		}
		if _, err := os.Stat(s.DataPath() + ".tmp"); !os.IsNotExist(err) {
			t.Fatalf("原子替换后不应残留 .tmp, stat err = %v", err)
		}
	})

	t.Run("目标目录不存在时自动创建", func(t *testing.T) {
		s := newTestStore(t, nil)
		s = NewStore(filepath.Join(filepath.Dir(s.DataPath()), "nested", "deep", "zakki.json"), nil, testMigrateOpt())
		f := defaultTabs("新建目录也能存")
		doc := File{Version: CurrentDataVersion, Tabs: f, Settings: Settings{HideOnEscape: true}}
		if err := s.SaveNow(doc); err != nil {
			t.Fatalf("SaveNow 报错: %v", err)
		}
		if got := readFile(t, s.DataPath()); got.Tabs[0].Text != "新建目录也能存" {
			t.Fatalf("落盘内容 = %q, want 新建目录也能存", got.Tabs[0].Text)
		}
	})
}

func TestShutdownReplacesPendingAndRejectsLaterSaves(t *testing.T) {
	t.Run("最终保存落盘", func(t *testing.T) {
		s := newTestStore(t, nil)
		want := File{
			Version: CurrentDataVersion,
			Active:  4,
			Tabs:    defaultTabs("退出前的最终内容"),
			Settings: Settings{
				ThemeSource:  "dark",
				AlwaysOnTop:  true,
				HideOnEscape: true,
				Source:       true,
			},
		}
		if err := s.Shutdown(want); err != nil {
			t.Fatal(err)
		}
		if got := readFile(t, s.DataPath()); !reflect.DeepEqual(got, want) {
			t.Fatalf("最终落盘 = %+v, want %+v", got, want)
		}
	})

	t.Run("排队 pending 被最终快照取代", func(t *testing.T) {
		s := newTestStore(t, nil)
		queued := File{Version: CurrentDataVersion, Tabs: defaultTabs("排队的旧快照"), Settings: Settings{HideOnEscape: true}}
		s.ScheduleSave(queued, 800*time.Millisecond, nil)
		final := File{Version: CurrentDataVersion, Active: 2, Tabs: defaultTabs("退出时的新快照"), Settings: Settings{HideOnEscape: true}}
		if err := s.Shutdown(final); err != nil {
			t.Fatal(err)
		}
		f := readFile(t, s.DataPath())
		if f.Tabs[0].Text != "退出时的新快照" || f.Active != 2 {
			t.Fatalf("最终落盘 = %+v, want 退出时的新快照/active=2", f)
		}
		time.Sleep(950 * time.Millisecond)
		if f := readFile(t, s.DataPath()); f.Tabs[0].Text != "退出时的新快照" || f.Active != 2 {
			t.Fatalf("防抖到期后被旧快照覆盖: %+v", f)
		}
	})

	t.Run("退出后同步保存明确拒绝", func(t *testing.T) {
		s := newTestStore(t, nil)
		before := File{Version: CurrentDataVersion, Tabs: defaultTabs("退出前内容"), Settings: Settings{HideOnEscape: true}}
		if err := s.Shutdown(before); err != nil {
			t.Fatal(err)
		}
		raw := readRaw(t, s.DataPath())
		later := File{Version: CurrentDataVersion, Tabs: defaultTabs("退出后内容"), Settings: Settings{HideOnEscape: true}}
		if err := s.SaveNow(later); !errors.Is(err, ErrStopped) {
			t.Fatalf("退出后 SaveNow = %v, want ErrStopped", err)
		}
		s.ScheduleSave(later, 800*time.Millisecond, nil)
		time.Sleep(1200 * time.Millisecond)
		if got := readRaw(t, s.DataPath()); string(got) != string(raw) {
			t.Fatalf("退出后数据文件被改动: %q, want %q", got, raw)
		}
	})
}
