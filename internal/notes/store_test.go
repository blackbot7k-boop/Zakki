package notes

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"
)

func newTestStore(t *testing.T, now func() time.Time) *Store {
	t.Helper()
	dir := t.TempDir()
	if now == nil {
		now = func() time.Time { return time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC) }
	}
	return NewStore(filepath.Join(dir, "tot-md.json"), filepath.Join(dir, "backups"), now, testMigrateOpt())
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

func backupNames(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	return names
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
		if got := readFile(t, s.DataFile); !reflect.DeepEqual(got, want) {
			t.Fatalf("落盘内容 = %+v, want %+v", got, want)
		}
		if _, err := os.Stat(s.DataFile + ".tmp"); !os.IsNotExist(err) {
			t.Fatalf("原子替换后不应残留 .tmp, stat err = %v", err)
		}
	})

	t.Run("目标目录不存在时自动创建", func(t *testing.T) {
		s := newTestStore(t, nil)
		s.DataFile = filepath.Join(filepath.Dir(s.DataFile), "nested", "deep", "tot-md.json")
		f := defaultTabs("新建目录也能存")
		doc := File{Version: CurrentDataVersion, Tabs: f, Settings: Settings{HideOnEscape: true}}
		if err := s.SaveNow(doc); err != nil {
			t.Fatalf("SaveNow 报错: %v", err)
		}
		if got := readFile(t, s.DataFile); got.Tabs[0].Text != "新建目录也能存" {
			t.Fatalf("落盘内容 = %q, want 新建目录也能存", got.Tabs[0].Text)
		}
	})
}

func TestVersionTooNewLeavesOriginal(t *testing.T) {
	s := newTestStore(t, nil)
	const raw = `{"version":3,"active":0,"tabs":[{"text":"future"}]}`
	writeRaw(t, s.DataFile, raw)
	_, _, err := s.ParseRaw([]byte(raw))
	if err == nil || !strings.Contains(err.Error(), "当前版本无法读取") {
		t.Fatalf("错误 = %v, want 含“当前版本无法读取”", err)
	}
	s.QuarantineCorruptFile()
	if got := string(readRaw(t, s.DataFile)); got != raw {
		t.Fatalf("原文件被改动: %q, want %q", got, raw)
	}
}

func TestCorruptFileQuarantineAndWriteBack(t *testing.T) {
	const corruptRaw = `{"version":1,"dots":[这根本不是 JSON`
	s := newTestStore(t, nil)
	writeRaw(t, s.DataFile, corruptRaw)

	t.Run("解析报错且原文件字节不变", func(t *testing.T) {
		if _, _, err := s.ParseRaw([]byte(corruptRaw)); err == nil {
			t.Fatal("损坏 JSON 应当解析报错")
		}
		s.QuarantineCorruptFile()
		if got := string(readRaw(t, s.DataFile)); got != corruptRaw {
			t.Fatalf("原文件被改动: %q, want %q", got, corruptRaw)
		}
	})

	t.Run("生成隔离副本且内容为原件", func(t *testing.T) {
		matches, err := filepath.Glob(s.DataFile + ".corrupt-*")
		if err != nil {
			t.Fatal(err)
		}
		if len(matches) != 1 {
			t.Fatalf("隔离副本 = %v, want 恰好 1 个", matches)
		}
		if got := string(readRaw(t, matches[0])); got != corruptRaw {
			t.Fatalf("隔离副本 = %q, want %q", got, corruptRaw)
		}
	})

	t.Run("写保护期间 SaveNow 只写 .pending", func(t *testing.T) {
		s.SetSaveBlocked(true)
		if got := s.CurrentDataPath(); got != s.PendingPath() {
			t.Fatalf("写保护期间生效路径 = %q, want %q", got, s.PendingPath())
		}
		doc := File{Version: CurrentDataVersion, Tabs: []Tab{{ID: 1, Text: "暂存的新内容"}}, Settings: Settings{HideOnEscape: true}}
		if err := s.SaveNow(doc); err != nil {
			t.Fatalf("SaveNow 报错: %v", err)
		}
		if got := string(readRaw(t, s.DataFile)); got != corruptRaw {
			t.Fatalf("写保护期间原文件被覆盖: %q, want %q", got, corruptRaw)
		}
		if f := readFile(t, s.PendingPath()); f.Tabs[0].Text != "暂存的新内容" {
			t.Fatalf(".pending 内容 = %q, want 暂存的新内容", f.Tabs[0].Text)
		}
	})

	t.Run("解除写保护后才写主文件", func(t *testing.T) {
		doc := File{Version: CurrentDataVersion, Tabs: []Tab{{ID: 1, Text: "暂存的新内容"}}, Settings: Settings{HideOnEscape: true}}
		blocked := s.SaveBlocked()
		s.SetSaveBlocked(false)
		if err := s.SaveNow(doc); err != nil {
			s.SetSaveBlocked(blocked)
			t.Fatalf("写回报错: %v", err)
		}
		if err := s.RemovePending(); err != nil {
			t.Fatal(err)
		}
		if got := s.CurrentDataPath(); got != s.DataFile {
			t.Fatalf("确认后生效路径 = %q, want %q", got, s.DataFile)
		}
		if f := readFile(t, s.DataFile); f.Tabs[0].Text != "暂存的新内容" {
			t.Fatalf("主文件内容 = %q, want 暂存的新内容", f.Tabs[0].Text)
		}
		if _, err := os.Stat(s.PendingPath()); !os.IsNotExist(err) {
			t.Fatalf("确认后 .pending 应被清理, stat err = %v", err)
		}
		doc.Tabs = append(doc.Tabs, Tab{ID: 2, Text: "确认后再改"})
		if err := s.SaveNow(doc); err != nil {
			t.Fatal(err)
		}
		if f := readFile(t, s.DataFile); f.Tabs[1].Text != "确认后再改" {
			t.Fatalf("SaveNow 未写主文件: tabs[1] = %q", f.Tabs[1].Text)
		}
	})
}

func TestBackupThrottleAndPrune(t *testing.T) {
	clk := newTestClock()
	s := newTestStore(t, clk.now)
	doc := File{Version: CurrentDataVersion, Tabs: defaultTabs("备份的内容"), Settings: Settings{HideOnEscape: true}}
	if err := s.SaveNow(doc); err != nil {
		t.Fatal(err)
	}

	t.Run("BackupNow 五分钟内连写两份都落盘", func(t *testing.T) {
		for i := 0; i < 2; i++ {
			if err := s.BackupNow(20); err != nil {
				t.Fatalf("BackupNow 第 %d 次报错: %v", i+1, err)
			}
		}
		if got := backupNames(t, s.BackupDir); len(got) != 2 {
			t.Fatalf("备份数 = %d (%v), want 2", len(got), got)
		}
	})

	t.Run("五分钟内周期备份不新增", func(t *testing.T) {
		before := backupNames(t, s.BackupDir)
		if err := s.MaybeWritePeriodicBackup(5*time.Minute, 20); err != nil {
			t.Fatal(err)
		}
		if got := backupNames(t, s.BackupDir); !reflect.DeepEqual(got, before) {
			t.Fatalf("节流期内备份被改动: %v, want %v", got, before)
		}
	})

	t.Run("五分钟后周期备份新增", func(t *testing.T) {
		clk.advance(5*time.Minute + time.Second)
		before := backupNames(t, s.BackupDir)
		if err := s.MaybeWritePeriodicBackup(5*time.Minute, 20); err != nil {
			t.Fatal(err)
		}
		got := backupNames(t, s.BackupDir)
		if len(got) != len(before)+1 {
			t.Fatalf("备份数 = %d (%v), want %d", len(got), got, len(before)+1)
		}
	})
}

func TestBackupPruneKeepsNewest(t *testing.T) {
	clk := newTestClock()
	s := newTestStore(t, clk.now)
	if err := s.SaveNow(File{Version: CurrentDataVersion, Tabs: defaultTabs("裁剪测试"), Settings: Settings{HideOnEscape: true}}); err != nil {
		t.Fatal(err)
	}
	const total = 25
	for i := 0; i < total; i++ {
		clk.advance(time.Second)
		if err := s.BackupNow(20); err != nil {
			t.Fatalf("BackupNow 第 %d 次报错: %v", i+1, err)
		}
		got := backupNames(t, s.BackupDir)
		if want := min(i+1, 20); len(got) != want {
			t.Fatalf("第 %d 次后备份数 = %d (%v), want %d", i+1, len(got), got, want)
		}
	}
	kept := backupNames(t, s.BackupDir)
	if len(kept) != 20 {
		t.Fatalf("最终备份数 = %d (%v), want 20", len(kept), kept)
	}
	for _, name := range kept {
		for sec := 1; sec <= 5; sec++ {
			old := fmt.Sprintf("tot-md-%s.json", time.Date(2024, 6, 1, 12, 0, sec, 0, time.UTC).Format("20060102-150405"))
			if name == old {
				t.Fatalf("旧备份 %s 不应保留（%v）", name, kept)
			}
		}
	}
	newest := fmt.Sprintf("tot-md-%s.json", time.Date(2024, 6, 1, 12, 0, 25, 0, time.UTC).Format("20060102-150405"))
	if kept[len(kept)-1] != newest {
		t.Fatalf("最新备份 = %q, want %q（列表 %v）", kept[len(kept)-1], newest, kept)
	}
}

func TestBackupNamesUniqueInSameSecond(t *testing.T) {
	s := newTestStore(t, nil)
	doc := File{Version: CurrentDataVersion, Tabs: defaultTabs("同秒备份"), Settings: Settings{HideOnEscape: true}}
	if err := s.SaveNow(doc); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		if err := s.BackupNow(20); err != nil {
			t.Fatalf("BackupNow 第 %d 次报错: %v", i+1, err)
		}
	}
	names := backupNames(t, s.BackupDir)
	if len(names) != 3 {
		t.Fatalf("备份 = %v, want 3 份都在", names)
	}
	seen := map[string]bool{}
	for _, n := range names {
		if seen[n] {
			t.Fatalf("备份名重复: %v", names)
		}
		seen[n] = true
	}
	for _, n := range names {
		f := readFile(t, filepath.Join(s.BackupDir, n))
		if len(f.Tabs) == 0 || f.Tabs[0].Text != "同秒备份" {
			t.Fatalf("备份 %s 内容 = %+v, want 同秒备份", n, f.Tabs)
		}
	}
}

func TestShutdownReplacesPendingAndIgnoresLaterSaves(t *testing.T) {
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
		if got := readFile(t, s.DataFile); !reflect.DeepEqual(got, want) {
			t.Fatalf("最终落盘 = %+v, want %+v", got, want)
		}
	})

	t.Run("排队 pending 被最终快照取代", func(t *testing.T) {
		s := newTestStore(t, nil)
		queued := File{Version: CurrentDataVersion, Tabs: defaultTabs("排队的旧快照"), Settings: Settings{HideOnEscape: true}}
		s.ScheduleDebounced(queued, 800*time.Millisecond, 5*time.Minute, 20, nil)
		final := File{Version: CurrentDataVersion, Active: 2, Tabs: defaultTabs("退出时的新快照"), Settings: Settings{HideOnEscape: true}}
		if err := s.Shutdown(final); err != nil {
			t.Fatal(err)
		}
		f := readFile(t, s.DataFile)
		if f.Tabs[0].Text != "退出时的新快照" || f.Active != 2 {
			t.Fatalf("最终落盘 = %+v, want 退出时的新快照/active=2", f)
		}
		time.Sleep(950 * time.Millisecond)
		if f := readFile(t, s.DataFile); f.Tabs[0].Text != "退出时的新快照" || f.Active != 2 {
			t.Fatalf("防抖到期后被旧快照覆盖: %+v", f)
		}
	})

	t.Run("退出后保存请求静默忽略", func(t *testing.T) {
		s := newTestStore(t, nil)
		before := File{Version: CurrentDataVersion, Tabs: defaultTabs("退出前内容"), Settings: Settings{HideOnEscape: true}}
		if err := s.Shutdown(before); err != nil {
			t.Fatal(err)
		}
		raw := readRaw(t, s.DataFile)
		later := File{Version: CurrentDataVersion, Tabs: defaultTabs("退出后内容"), Settings: Settings{HideOnEscape: true}}
		if err := s.SaveNow(later); err != nil {
			t.Fatalf("退出后 SaveNow 应静默忽略，得到错误: %v", err)
		}
		s.ScheduleDebounced(later, 800*time.Millisecond, 5*time.Minute, 20, nil)
		if err := s.BackupNow(20); err != nil {
			t.Fatalf("退出后 BackupNow 应静默忽略，得到错误: %v", err)
		}
		time.Sleep(1200 * time.Millisecond)
		if got := readRaw(t, s.DataFile); string(got) != string(raw) {
			t.Fatalf("退出后数据文件被改动: %q, want %q", got, raw)
		}
		if got := backupNames(t, s.BackupDir); len(got) != 0 {
			t.Fatalf("退出后备份被改动: %v, want 无", got)
		}
	})
}
