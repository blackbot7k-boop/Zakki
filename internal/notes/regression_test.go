package notes

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestBackupNameOrderUsesNumericSequence(t *testing.T) {
	names := []string{
		"zakki-20240601-120000-10.json",
		"tot-md-20240601-120000-9.json",
		"zakki-20240601-120000.json",
		"zakki-20240531-235959.json",
	}
	sort.Slice(names, func(i, j int) bool { return backupNameNewer(names[i], names[j]) })
	want := []string{
		"zakki-20240601-120000-10.json",
		"tot-md-20240601-120000-9.json",
		"zakki-20240601-120000.json",
		"zakki-20240531-235959.json",
	}
	if !reflect.DeepEqual(names, want) {
		t.Fatalf("备份排序=%v，要 %v", names, want)
	}
}

func TestSameSecondBackupPruningKeepsNumericNewest(t *testing.T) {
	clk := newTestClock()
	s := newTestStore(t, clk.now)
	if err := s.SaveNow(File{Version: CurrentDataVersion, Tabs: defaultTabs("同秒备份"), Settings: Settings{HideOnEscape: true}}); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 11; i++ {
		if err := s.BackupNow(1); err != nil {
			t.Fatal(err)
		}
	}
	if got, want := backupNames(t, s.BackupDir), []string{"zakki-20240601-120000-11.json"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("同秒第 11 份后的保留结果=%v，要 %v", got, want)
	}
}

func TestRunGitJobPreservesUnrelatedIndexAndWorktree(t *testing.T) {
	dir, runner := newRealGitRepo(t)
	writeRaw(t, filepath.Join(dir, "staged.txt"), "user staged change")
	writeRaw(t, filepath.Join(dir, "unstaged.txt"), "user unstaged change")
	gitMust(t, runner, dir, "add", "--", "staged.txt")
	writeRaw(t, filepath.Join(dir, "zakki-20240601-120000.json"), "{}")

	if err := runGitJob(runner, dir, "commit", "20240601-120000"); err != nil {
		t.Fatalf("仅有新前缀时提交失败: %v", err)
	}
	if got, want := gitLines(t, runner, dir, "show", "--format=", "--name-only", "HEAD"), []string{"zakki-20240601-120000.json"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("自动提交文件=%v，要 %v", got, want)
	}
	if got, want := gitLines(t, runner, dir, "diff", "--cached", "--name-only"), []string{"staged.txt"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("用户暂存区=%v，要保留 %v", got, want)
	}
	if got, want := gitLines(t, runner, dir, "diff", "--name-only"), []string{"unstaged.txt"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("用户未暂存改动=%v，要保留 %v", got, want)
	}
}

func TestRunGitJobCommitsCurrentAndLegacyPrefixes(t *testing.T) {
	dir, runner := newRealGitRepo(t)
	writeRaw(t, filepath.Join(dir, "zakki-20240601-120000.json"), "{}")
	writeRaw(t, filepath.Join(dir, "tot-md-20240531-120000.json"), "{}")

	if err := runGitJob(runner, dir, "commit", "20240601-120000"); err != nil {
		t.Fatal(err)
	}
	want := []string{"tot-md-20240531-120000.json", "zakki-20240601-120000.json"}
	if got := gitLines(t, runner, dir, "show", "--format=", "--name-only", "HEAD"); !reflect.DeepEqual(got, want) {
		t.Fatalf("自动提交文件=%v，要 %v", got, want)
	}
}

func TestRunGitJobIncludesTrackedBackupDeletion(t *testing.T) {
	dir, runner := newRealGitRepo(t)
	legacy := "tot-md-20240531-120000.json"
	writeRaw(t, filepath.Join(dir, legacy), "{}")
	gitMust(t, runner, dir, "add", "--", legacy)
	gitMust(t, runner, dir, "commit", "-m", "legacy backup")
	if err := os.Remove(filepath.Join(dir, legacy)); err != nil {
		t.Fatal(err)
	}
	current := "zakki-20240601-120000.json"
	writeRaw(t, filepath.Join(dir, current), "{}")
	paths, err := gitBackupPaths(runner, dir)
	if err != nil {
		t.Fatal(err)
	}
	wantPaths := []string{legacy, current}
	sort.Strings(wantPaths)
	if !reflect.DeepEqual(paths, wantPaths) {
		t.Fatalf("Git 备份路径=%v，要 %v", paths, wantPaths)
	}

	if err := runGitJob(runner, dir, "commit", "20240601-120000"); err != nil {
		t.Fatal(err)
	}
	got := gitLines(t, runner, dir, "show", "--format=", "--name-only", "--no-renames", "HEAD")
	want := []string{current, legacy}
	sort.Strings(got)
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("自动提交文件=%v，要 %v；提交详情=%q；剩余状态=%q", got, want,
			gitMust(t, runner, dir, "show", "--format=", "--name-status", "HEAD"),
			gitMust(t, runner, dir, "status", "--porcelain"))
	}
}

func TestBlockedMirrorDoesNotBlockStateSaveOrShutdown(t *testing.T) {
	s := newTestStore(t, nil)
	mirror := filepath.Join(filepath.Dir(s.DataFile), "mirror")
	s.SetMirror(mirror, "off")
	initial := File{Version: CurrentDataVersion, Tabs: defaultTabs("镜像前"), Settings: Settings{HideOnEscape: true}}
	if err := s.SaveNow(initial); err != nil {
		t.Fatal(err)
	}

	started := make(chan struct{})
	release := make(chan struct{})
	settled := make(chan struct{})
	var startOnce sync.Once
	s.mirrorWrite = func(path string, raw []byte) error {
		startOnce.Do(func() { close(started) })
		<-release
		return os.WriteFile(path, raw, 0o644)
	}
	s.SetOnMirrorSettled(func() { close(settled) })
	if err := s.BackupNow(20); err != nil {
		t.Fatal(err)
	}
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("镜像写入未开始")
	}

	mustFinish(t, "MirrorStatus", func() error {
		_ = s.MirrorStatus()
		return nil
	})
	saved := File{Version: CurrentDataVersion, Tabs: defaultTabs("镜像阻塞时保存"), Settings: Settings{HideOnEscape: true}}
	mustFinish(t, "SaveNow", func() error { return s.SaveNow(saved) })
	final := File{Version: CurrentDataVersion, Tabs: defaultTabs("镜像阻塞时退出"), Settings: Settings{HideOnEscape: true}}
	mustFinish(t, "Shutdown", func() error { return s.Shutdown(final) })
	if got := readFile(t, s.DataFile); got.Tabs[0].Text != "镜像阻塞时退出" {
		t.Fatalf("最终保存=%q", got.Tabs[0].Text)
	}

	close(release)
	select {
	case <-settled:
	case <-time.After(time.Second):
		t.Fatal("镜像解除阻塞后未结束")
	}
}

func TestBlockedGitDoesNotBlockStateSaveOrShutdown(t *testing.T) {
	s := newTestStore(t, nil)
	mirror := filepath.Join(filepath.Dir(s.DataFile), "mirror")
	started := make(chan struct{})
	release := make(chan struct{})
	settled := make(chan struct{})
	var startOnce sync.Once
	s.RunGit = func(_ string, args ...string) (string, error) {
		switch args[0] {
		case "ls-files":
			return "", nil
		case "add":
			startOnce.Do(func() { close(started) })
			<-release
		case "status":
			return "A  zakki-20240601-120000.json\n", nil
		}
		return "", nil
	}
	s.SetMirror(mirror, "commit")
	s.SetOnMirrorSettled(func() { close(settled) })
	if err := s.SaveNow(File{Version: CurrentDataVersion, Tabs: defaultTabs("Git 前"), Settings: Settings{HideOnEscape: true}}); err != nil {
		t.Fatal(err)
	}
	if err := s.BackupNow(20); err != nil {
		t.Fatal(err)
	}
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("Git 未开始")
	}

	mustFinish(t, "Git 阻塞时 MirrorStatus", func() error {
		_ = s.MirrorStatus()
		return nil
	})
	saved := File{Version: CurrentDataVersion, Tabs: defaultTabs("Git 阻塞时保存"), Settings: Settings{HideOnEscape: true}}
	mustFinish(t, "Git 阻塞时 SaveNow", func() error { return s.SaveNow(saved) })
	final := File{Version: CurrentDataVersion, Tabs: defaultTabs("Git 阻塞时退出"), Settings: Settings{HideOnEscape: true}}
	mustFinish(t, "Git 阻塞时 Shutdown", func() error { return s.Shutdown(final) })

	close(release)
	select {
	case <-settled:
	case <-time.After(time.Second):
		t.Fatal("Git 解除阻塞后未结束")
	}
}

func TestOnMirrorSettledSetterIsRaceSafe(t *testing.T) {
	s := newTestStore(t, nil)
	mirror := filepath.Join(filepath.Dir(s.DataFile), "mirror")
	s.SetMirror(mirror, "off")
	if err := s.SaveNow(File{Version: CurrentDataVersion, Tabs: defaultTabs("race"), Settings: Settings{HideOnEscape: true}}); err != nil {
		t.Fatal(err)
	}

	started := make(chan struct{})
	release := make(chan struct{})
	var startOnce sync.Once
	s.mirrorWrite = func(path string, raw []byte) error {
		startOnce.Do(func() { close(started) })
		<-release
		return os.WriteFile(path, raw, 0o644)
	}
	var callbacks atomic.Int64
	cb1 := func() { callbacks.Add(1) }
	cb2 := func() { callbacks.Add(1) }
	s.SetOnMirrorSettled(cb1)
	const jobs = 32
	for i := 0; i < jobs; i++ {
		if err := s.BackupNow(100); err != nil {
			t.Fatal(err)
		}
	}
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("镜像写入未开始")
	}

	setterDone := make(chan struct{})
	go func() {
		defer close(setterDone)
		for i := 0; i < 20000; i++ {
			if i%2 == 0 {
				s.SetOnMirrorSettled(cb1)
			} else {
				s.SetOnMirrorSettled(cb2)
			}
		}
	}()
	close(release)
	select {
	case <-setterDone:
	case <-time.After(3 * time.Second):
		t.Fatal("设置回调超时")
	}
	waitForMirror(t, func() bool { return callbacks.Load() == 2 })
	local := backupNames(t, s.BackupDir)
	sort.Slice(local, func(i, j int) bool { return backupNameNewer(local[i], local[j]) })
	mirrored := backupNames(t, mirror)
	want := []string{local[0], local[len(local)-1]}
	sort.Strings(want)
	if !reflect.DeepEqual(mirrored, want) {
		t.Fatalf("忙碌时应只写首个和最新镜像: got=%v want=%v", mirrored, want)
	}
}

func TestMirrorConfigChangeDropsQueuedOldJob(t *testing.T) {
	clk := newTestClock()
	s, oldMirror := newMirrorStore(t, clk)
	newMirror := filepath.Join(filepath.Dir(s.DataFile), "mirror-new")

	started := make(chan struct{})
	release := make(chan struct{})
	var first sync.Once
	s.mirrorWrite = func(path string, raw []byte) error {
		if strings.HasPrefix(path, oldMirror+string(os.PathSeparator)) {
			first.Do(func() {
				close(started)
				<-release
			})
		}
		return os.WriteFile(path, raw, 0o644)
	}
	if err := s.BackupNow(100); err != nil {
		t.Fatal(err)
	}
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("旧镜像写入未开始")
	}

	clk.advance(time.Second)
	if err := s.BackupNow(100); err != nil {
		t.Fatal(err)
	}
	s.SetMirror(newMirror, "off")
	clk.advance(time.Second)
	if err := s.BackupNow(100); err != nil {
		t.Fatal(err)
	}
	close(release)

	waitForMirror(t, func() bool {
		s.mu.Lock()
		defer s.mu.Unlock()
		return !s.mirrorRunning
	})
	local := backupNames(t, s.BackupDir)
	sort.Slice(local, func(i, j int) bool { return backupNameNewer(local[i], local[j]) })
	if got, want := backupNames(t, oldMirror), []string{local[len(local)-1]}; !reflect.DeepEqual(got, want) {
		t.Fatalf("旧目录不得写入排队快照: got=%v want=%v", got, want)
	}
	if got, want := backupNames(t, newMirror), []string{local[0]}; !reflect.DeepEqual(got, want) {
		t.Fatalf("新目录应只写切换后的最新快照: got=%v want=%v", got, want)
	}
}

func newRealGitRepo(t *testing.T) (string, GitRunner) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skipf("git 不可用: %v", err)
	}
	dir := t.TempDir()
	runner := DefaultGitRunner()
	gitMust(t, runner, dir, "init", "-q")
	gitMust(t, runner, dir, "config", "user.name", "Zakki Test")
	gitMust(t, runner, dir, "config", "user.email", "zakki-test@example.invalid")
	writeRaw(t, filepath.Join(dir, "staged.txt"), "baseline")
	writeRaw(t, filepath.Join(dir, "unstaged.txt"), "baseline")
	gitMust(t, runner, dir, "add", "--", "staged.txt", "unstaged.txt")
	gitMust(t, runner, dir, "commit", "-q", "-m", "baseline")
	return dir, runner
}

func gitMust(t *testing.T, runner GitRunner, dir string, args ...string) string {
	t.Helper()
	out, err := runner(dir, args...)
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return out
}

func gitLines(t *testing.T, runner GitRunner, dir string, args ...string) []string {
	t.Helper()
	out := strings.TrimSpace(gitMust(t, runner, dir, args...))
	if out == "" {
		return nil
	}
	lines := strings.Split(out, "\n")
	sort.Strings(lines)
	return lines
}

func mustFinish(t *testing.T, name string, fn func() error) {
	t.Helper()
	done := make(chan error, 1)
	go func() { done <- fn() }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	case <-time.After(time.Second):
		t.Fatalf("%s 被阻塞", name)
	}
}
