package notes

import (
	"encoding/json"
	"errors"
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
	return NewStore(filepath.Join(dir, "zakki.json"), filepath.Join(dir, "backups"), now, testMigrateOpt())
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
		s.DataFile = filepath.Join(filepath.Dir(s.DataFile), "nested", "deep", "zakki.json")
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
	const raw = `{"version":4,"active":0,"tabs":[{"text":"future"}]}`
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
			old := fmt.Sprintf("zakki-%s.json", time.Date(2024, 6, 1, 12, 0, sec, 0, time.UTC).Format("20060102-150405"))
			if name == old {
				t.Fatalf("旧备份 %s 不应保留（%v）", name, kept)
			}
		}
	}
	newest := fmt.Sprintf("zakki-%s.json", time.Date(2024, 6, 1, 12, 0, 25, 0, time.UTC).Format("20060102-150405"))
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

// ---- 外部备份镜像与 Git 自动化 ----

// gitLog 记录 fake runner 的调用，并可让特定子命令失败。
type gitLog struct {
	mu     sync.Mutex
	runs   [][]string
	failOn map[string]error
	block  chan struct{} // 非 nil 时 add 阻塞直到关闭，用于测忙碌合并
}

func (g *gitLog) runner() GitRunner {
	return func(dir string, args ...string) (string, error) {
		if args[0] == "add" && g.block != nil {
			<-g.block
		}
		g.mu.Lock()
		defer g.mu.Unlock()
		g.runs = append(g.runs, append([]string(nil), args...))
		if err := g.failOn[args[0]]; err != nil {
			return "", err
		}
		if args[0] == "status" {
			return "A  zakki-x.json\n", nil // add 之后总有变更
		}
		return "", nil
	}
}

func (g *gitLog) ran() []string {
	g.mu.Lock()
	defer g.mu.Unlock()
	var out []string
	for _, r := range g.runs {
		out = append(out, r[0])
	}
	return out
}

func (g *gitLog) has(cmd string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	for _, r := range g.runs {
		if r[0] == cmd {
			return true
		}
	}
	return false
}

// waitForGit 轮询等待后台 Git 任务结束（BackupNow 返回时任务可能仍在跑）。
func waitForGit(t *testing.T, g *gitLog, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("等待 Git 条件超时，已执行: %v", g.ran())
}

func newMirrorStore(t *testing.T, clk *testClock) (*Store, string) {
	t.Helper()
	s := newTestStore(t, clk.now)
	mirror := filepath.Join(filepath.Dir(s.DataFile), "mirror")
	if err := os.MkdirAll(mirror, 0o755); err != nil {
		t.Fatal(err)
	}
	s.SetMirror(mirror, "off")
	if err := s.SaveNow(File{Version: CurrentDataVersion, Tabs: defaultTabs("镜像测试"), Settings: Settings{HideOnEscape: true}}); err != nil {
		t.Fatal(err)
	}
	return s, mirror
}

func TestMirrorBackupCopiesAndPrunes(t *testing.T) {
	clk := newTestClock()
	s, mirror := newMirrorStore(t, clk)

	if err := s.BackupNow(2); err != nil {
		t.Fatalf("BackupNow 报错: %v", err)
	}
	got := backupNames(t, mirror)
	if len(got) != 1 || got[0] != backupNames(t, s.BackupDir)[0] {
		t.Fatalf("镜像应与本地备份同名同数: mirror=%v local=%v", got, backupNames(t, s.BackupDir))
	}
	if string(readRaw(t, filepath.Join(mirror, got[0]))) != string(readRaw(t, filepath.Join(s.BackupDir, got[0]))) {
		t.Fatal("镜像内容应与本地备份一致")
	}
	if st := s.MirrorStatus(); st.Dir != mirror || st.Err != "" || st.Last.IsZero() {
		t.Fatalf("镜像状态错误: %+v", st)
	}

	// keep=2：多备几份后本地与镜像都只留最新 2 份。
	for i := 0; i < 3; i++ {
		clk.advance(time.Second)
		if err := s.BackupNow(2); err != nil {
			t.Fatalf("第 %d 次备份报错: %v", i+1, err)
		}
	}
	if local, mir := backupNames(t, s.BackupDir), backupNames(t, mirror); !reflect.DeepEqual(local, mir) || len(mir) != 2 {
		t.Fatalf("修剪后本地与镜像应一致且各留 2 份: local=%v mirror=%v", local, mir)
	}
}

func TestMirrorFailureDoesNotBlockLocalBackup(t *testing.T) {
	s := newTestStore(t, nil)
	// 镜像路径是一个文件：MkdirAll 必然失败。
	blocker := filepath.Join(filepath.Dir(s.DataFile), "blocker")
	writeRaw(t, blocker, "not a dir")
	s.SetMirror(blocker, "off")
	if err := s.SaveNow(File{Version: CurrentDataVersion, Tabs: defaultTabs("镜像失败"), Settings: Settings{HideOnEscape: true}}); err != nil {
		t.Fatal(err)
	}
	if err := s.BackupNow(20); err != nil {
		t.Fatalf("镜像失败不得影响本地备份，得到错误: %v", err)
	}
	if got := backupNames(t, s.BackupDir); len(got) != 1 {
		t.Fatalf("本地备份应存在: %v", got)
	}
	if st := s.MirrorStatus(); st.Err == "" {
		t.Fatalf("镜像失败应记入状态: %+v", st)
	}
}

func TestMirrorGitModes(t *testing.T) {

	t.Run("off 不调用 git", func(t *testing.T) {
		clk := newTestClock()
		s, _ := newMirrorStore(t, clk)
		g := &gitLog{}
		s.RunGit = g.runner()
		if err := s.BackupNow(20); err != nil {
			t.Fatal(err)
		}
		time.Sleep(20 * time.Millisecond)
		if g.has("add") {
			t.Fatalf("off 模式不应调用 git: %v", g.ran())
		}
	})

	t.Run("commit 提交不推送", func(t *testing.T) {
		clk := newTestClock()
		s, _ := newMirrorStore(t, clk)
		g := &gitLog{}
		s.RunGit = g.runner()
		s.SetMirror(s.MirrorStatus().Dir, "commit")
		if err := s.BackupNow(20); err != nil {
			t.Fatal(err)
		}
		waitForGit(t, g, func() bool { return g.has("commit") })
		if g.has("push") {
			t.Fatalf("commit 模式不应推送: %v", g.ran())
		}
		if st := s.MirrorStatus(); st.Err != "" {
			t.Fatalf("成功后不应有错误: %+v", st)
		}
	})

	t.Run("commitPush 推送", func(t *testing.T) {
		clk := newTestClock()
		s, _ := newMirrorStore(t, clk)
		g := &gitLog{}
		s.RunGit = g.runner()
		s.SetMirror(s.MirrorStatus().Dir, "commitPush")
		if err := s.BackupNow(20); err != nil {
			t.Fatal(err)
		}
		waitForGit(t, g, func() bool { return g.has("push") })
	})

	t.Run("无变更跳过提交", func(t *testing.T) {
		clk := newTestClock()
		s, _ := newMirrorStore(t, clk)
		g := &gitLog{failOn: map[string]error{}}
		s.RunGit = func(dir string, args ...string) (string, error) {
			if args[0] == "status" {
				return "", nil // 工作区干净
			}
			g.mu.Lock()
			g.runs = append(g.runs, append([]string(nil), args...))
			g.mu.Unlock()
			return "", nil
		}
		s.SetMirror(s.MirrorStatus().Dir, "commit")
		if err := s.BackupNow(20); err != nil {
			t.Fatal(err)
		}
		waitForGit(t, g, func() bool { return g.has("add") })
		time.Sleep(20 * time.Millisecond) // 给后续步骤（不应发生）留时间
		if g.has("commit") {
			t.Fatalf("无变更不应提交: %v", g.ran())
		}
	})

	t.Run("git 失败如实上报且不影响备份", func(t *testing.T) {
		clk := newTestClock()
		s, _ := newMirrorStore(t, clk)
		g := &gitLog{failOn: map[string]error{"commit": errors.New("no identity")}}
		s.RunGit = g.runner()
		s.SetMirror(s.MirrorStatus().Dir, "commit")
		if err := s.BackupNow(20); err != nil {
			t.Fatalf("git 失败不得影响备份: %v", err)
		}
		waitForGit(t, g, func() bool { return s.MirrorStatus().Err != "" })
		if !strings.Contains(s.MirrorStatus().Err, "no identity") {
			t.Fatalf("错误应包含原因: %q", s.MirrorStatus().Err)
		}
		if got := backupNames(t, s.BackupDir); len(got) == 0 {
			t.Fatal("本地备份应已写入")
		}
	})
}

func TestGitBusyCoalesces(t *testing.T) {
	clk := newTestClock()
	s, _ := newMirrorStore(t, clk)
	g := &gitLog{block: make(chan struct{})}
	s.RunGit = g.runner()
	s.SetMirror(s.MirrorStatus().Dir, "commit")
	if err := s.BackupNow(20); err != nil {
		t.Fatal(err)
	}
	// 第一个任务阻塞在 add 时再备份一次：应记待办而非并发第二个任务。
	if err := s.BackupNow(20); err != nil {
		t.Fatal(err)
	}
	close(g.block)
	waitForGit(t, g, func() bool {
		g.mu.Lock()
		defer g.mu.Unlock()
		adds := 0
		for _, r := range g.runs {
			if r[0] == "add" {
				adds++
			}
		}
		return adds >= 2 // 补跑的第二轮 add
	})
}

func TestLoadMirrorTime(t *testing.T) {
	clk := newTestClock()
	s, mirror := newMirrorStore(t, clk)
	if err := s.BackupNow(20); err != nil {
		t.Fatal(err)
	}
	// 先指向别处再切回，让目录变更重置 lastMirror，验证从磁盘恢复。
	s.SetMirror("", "off")
	s.SetMirror(mirror, "off")
	if err := s.LoadMirrorTime(); err != nil {
		t.Fatal(err)
	}
	if s.MirrorStatus().Last.IsZero() {
		t.Fatal("应从镜像目录恢复最近镜像时间")
	}
	// 未配置时静默无操作。
	s.SetMirror("", "off")
	if err := s.LoadMirrorTime(); err != nil {
		t.Fatalf("未配置不应报错: %v", err)
	}
}
