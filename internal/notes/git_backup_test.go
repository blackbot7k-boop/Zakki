package notes

// 备份文件夹 Git 自动化：仓库探测、提交路径规范、推送节流与状态发布。
// 单元测试用脚本化假 runner；端到端测试用真实 git，缺 git 时跳过。

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestDirIsGitRepo(t *testing.T) {
	dir := t.TempDir()
	if DirIsGitRepo(dir) {
		t.Fatal("无 .git 不应识别为仓库")
	}
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if !DirIsGitRepo(dir) {
		t.Fatal(".git 目录应识别为仓库")
	}
	// worktree 的 .git 是文件而非目录。
	work := t.TempDir()
	if err := os.WriteFile(filepath.Join(work, ".git"), []byte("gitdir: elsewhere\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !DirIsGitRepo(work) {
		t.Fatal(".git 文件（worktree 形态）应识别为仓库")
	}
}

// fakeGit 按首条子命令分发返回值，并记录全部调用供断言命令序列。
type fakeGit struct {
	calls   [][]string
	lsFiles string
	status  string
	failOn  string
	failErr error
}

func (f *fakeGit) run(dir string, args ...string) (string, error) {
	f.calls = append(f.calls, args)
	if f.failOn != "" && args[0] == f.failOn {
		return "", f.failErr
	}
	switch args[0] {
	case "ls-files":
		return f.lsFiles, nil
	case "status":
		return f.status, nil
	}
	return "", nil
}

func (f *fakeGit) count(cmd string) int {
	n := 0
	for _, c := range f.calls {
		if c[0] == cmd {
			n++
		}
	}
	return n
}

func (f *fakeGit) ran(cmd string) []string {
	for _, c := range f.calls {
		if c[0] == cmd {
			return c
		}
	}
	return nil
}

func TestRunGitJobCommitsOnlyBackupFiles(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"zakki-20240601-120000.json", "zakki-20240601-120500.json", "notes.txt"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	g := &fakeGit{lsFiles: "tot-md-20240501-090000.json\x00", status: "M  zakki-20240601-120000.json"}
	committed, err := runGitJob(g.run, dir, "backup: zakki-20240601-120500")
	if err != nil || !committed {
		t.Fatalf("应产生提交: committed=%v err=%v", committed, err)
	}
	add := g.ran("add")
	for _, want := range []string{"zakki-20240601-120000.json", "zakki-20240601-120500.json", "tot-md-20240501-090000.json"} {
		if !strings.Contains(strings.Join(add, "\x00"), want) {
			t.Fatalf("add 应包含备份文件 %s: %v", want, add)
		}
	}
	if strings.Contains(strings.Join(add, "\x00"), "notes.txt") {
		t.Fatalf("add 不得包含仓库里用户自己的文件: %v", add)
	}
	commit := g.ran("commit")
	if commit == nil {
		t.Fatal("应执行 commit")
	}
	joined := strings.Join(commit, "\x00")
	if !strings.Contains(joined, "--only") || !strings.Contains(joined, "backup: zakki-20240601-120500") {
		t.Fatalf("commit 应带 --only 与备份消息: %v", commit)
	}
	if !strings.Contains(joined, "tot-md-20240501-090000.json") {
		t.Fatalf("commit 路径应覆盖已跟踪但本地已修剪的旧备份: %v", commit)
	}
}

func TestRunGitJobSkipsWhenNoBackupChanges(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "zakki-20240601-120000.json"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	g := &fakeGit{status: " "}
	committed, err := runGitJob(g.run, dir, "backup: x")
	if err != nil || committed {
		t.Fatalf("无变更应跳过提交: committed=%v err=%v", committed, err)
	}
	if g.count("commit") != 0 {
		t.Fatal("无变更不得执行 commit")
	}
}

func TestRunGitJobFailsWhenNothingToCommit(t *testing.T) {
	dir := t.TempDir() // 目录里没有任何备份文件，也没有已跟踪文件
	g := &fakeGit{}
	committed, err := runGitJob(g.run, dir, "backup: x")
	if err != nil || committed {
		t.Fatalf("空目录应静默跳过: committed=%v err=%v", committed, err)
	}
	if g.count("add") != 0 {
		t.Fatal("没有备份文件不应执行 add")
	}
}

// newGitTestStore 建一个指向 repo 的 Store，时钟可推进，runner 可替换。
func newGitTestStore(t *testing.T, repo string) *Store {
	t.Helper()
	dataDir := t.TempDir()
	s := NewStore(filepath.Join(dataDir, "zakki.json"), filepath.Join(dataDir, "backups"), nil, MigrateOptions{})
	if err := os.WriteFile(s.DataFile, []byte(`{"version":3,"tabs":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	s.SetBackupDir(repo)
	return s
}

func TestStoreGitDisabledByDefault(t *testing.T) {
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	s := newGitTestStore(t, repo)
	g := &fakeGit{}
	s.RunGit = g.run
	if err := s.BackupNow(20); err != nil {
		t.Fatal(err)
	}
	s.WaitGitIdle()
	if len(g.calls) != 0 {
		t.Fatalf("未启用 Git 自动化不得执行任何 git 命令: %v", g.calls)
	}
}

func TestStoreGitPushThrottle(t *testing.T) {
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	s := newGitTestStore(t, repo)
	base := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)
	current := base
	s.Now = func() time.Time { return current }
	// status 恒空：永不产生提交，只考察推送节流。
	g := &fakeGit{}
	s.RunGit = g.run
	s.SetGitAutoPush(true, 10*time.Minute)

	if err := s.BackupNow(20); err != nil {
		t.Fatal(err)
	}
	s.WaitGitIdle()
	if g.count("push") != 1 {
		t.Fatalf("首次备份应立即尝试一次推送: %d", g.count("push"))
	}
	current = base.Add(time.Minute)
	if err := s.BackupNow(20); err != nil {
		t.Fatal(err)
	}
	s.WaitGitIdle()
	if g.count("push") != 1 {
		t.Fatalf("间隔内不得重复推送: %d", g.count("push"))
	}
	current = base.Add(11 * time.Minute)
	if err := s.BackupNow(20); err != nil {
		t.Fatal(err)
	}
	s.WaitGitIdle()
	if g.count("push") != 2 {
		t.Fatalf("超过间隔应再次推送: %d", g.count("push"))
	}
	if !s.GitCommitTime().IsZero() {
		t.Fatal("从未有变更不应记录提交时间")
	}
	if s.GitPushTime().IsZero() || s.GitError() != "" {
		t.Fatalf("推送成功应记录时间且无错误: push=%v err=%q", s.GitPushTime(), s.GitError())
	}
}

func TestStoreGitCommitFailureCapturedAndRetried(t *testing.T) {
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	s := newGitTestStore(t, repo)
	base := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)
	current := base
	s.Now = func() time.Time { return current }
	boom := errors.New("no identity")
	g := &fakeGit{status: "A  zakki-20240601-120000.json", failOn: "commit", failErr: boom}
	s.RunGit = g.run
	s.SetGitAutoPush(true, time.Hour)

	if err := s.BackupNow(20); err != nil {
		t.Fatal(err) // 备份本体成功；git 失败只进状态
	}
	s.WaitGitIdle()
	if g.count("push") != 0 {
		t.Fatal("提交失败不得推送")
	}
	if !strings.Contains(s.GitError(), "no identity") {
		t.Fatalf("提交失败应记录错误: %q", s.GitError())
	}
	if !s.GitCommitTime().IsZero() {
		t.Fatal("提交失败不得记录提交时间")
	}

	// 修好身份后下次备份重试：提交成功并按节流推送。
	g.failOn = ""
	current = base.Add(2 * time.Hour)
	if err := s.BackupNow(20); err != nil {
		t.Fatal(err)
	}
	s.WaitGitIdle()
	if g.count("commit") != 2 {
		t.Fatalf("失败后应重试提交: %d", g.count("commit"))
	}
	if s.GitError() != "" || s.GitCommitTime().IsZero() {
		t.Fatalf("重试成功应清除错误并记录时间: err=%q commit=%v", s.GitError(), s.GitCommitTime())
	}
	if g.count("push") != 1 {
		t.Fatalf("提交成功后首次推送应执行: %d", g.count("push"))
	}
}

func TestStoreSetBackupDirClearsGitState(t *testing.T) {
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	s := newGitTestStore(t, repo)
	g := &fakeGit{}
	s.RunGit = g.run
	s.SetGitAutoPush(true, time.Minute)
	if err := s.BackupNow(20); err != nil {
		t.Fatal(err)
	}
	s.WaitGitIdle()
	if s.GitPushTime().IsZero() {
		t.Fatal("前置条件：应已有推送状态")
	}
	s.SetBackupDir(t.TempDir())
	if !s.GitPushTime().IsZero() || !s.GitCommitTime().IsZero() || s.GitError() != "" {
		t.Fatalf("更换备份文件夹应作废 Git 状态: commit=%v push=%v err=%q",
			s.GitCommitTime(), s.GitPushTime(), s.GitError())
	}
}

// 备份入口可能在主线程：git 命令（含推送）挂起时 BackupNow 也必须立即返回，
// 挂起期间的后续备份合并为一次排队。
func TestStoreGitDoesNotBlockBackup(t *testing.T) {
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	s := newGitTestStore(t, repo)
	release := make(chan struct{})
	started := make(chan struct{}, 1)
	var mu sync.Mutex
	adds := 0
	s.RunGit = func(dir string, args ...string) (string, error) {
		if args[0] == "add" {
			mu.Lock()
			adds++
			mu.Unlock()
			select {
			case started <- struct{}{}:
			default:
			}
			<-release
		}
		return "", nil
	}
	s.SetGitAutoPush(true, time.Hour)

	done := make(chan error, 1)
	go func() { done <- s.BackupNow(20) }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("git 挂起时 BackupNow 不应等待")
	}
	<-started
	for range 3 {
		if err := s.BackupNow(20); err != nil {
			t.Fatal(err)
		}
	}
	close(release)
	s.WaitGitIdle()
	mu.Lock()
	defer mu.Unlock()
	if adds != 2 {
		t.Fatalf("挂起期间的备份应合并为一次排队提交: add 执行 %d 次", adds)
	}
}

// 更换备份文件夹后，旧文件夹上仍在运行的 git 步骤不得发布状态。
func TestStoreGitStaleResultDropped(t *testing.T) {
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	s := newGitTestStore(t, repo)
	release := make(chan struct{})
	started := make(chan struct{})
	s.RunGit = func(dir string, args ...string) (string, error) {
		if args[0] == "add" {
			close(started)
			<-release
		}
		if args[0] == "status" {
			return "A  x", nil
		}
		return "", nil
	}
	s.SetGitAutoPush(true, time.Hour)
	if err := s.BackupNow(20); err != nil {
		t.Fatal(err)
	}
	<-started
	s.SetBackupDir(t.TempDir())
	close(release)
	s.WaitGitIdle()
	if !s.GitCommitTime().IsZero() || !s.GitPushTime().IsZero() || s.GitError() != "" {
		t.Fatalf("旧文件夹的 git 结果不应发布: commit=%v push=%v err=%q",
			s.GitCommitTime(), s.GitPushTime(), s.GitError())
	}
}

func TestDefaultGitRunnerNonInteractive(t *testing.T) {
	gitAvailable(t)
	dir := t.TempDir()
	gitRun(t, dir, "init", "-q")
	// 环境变量须传给 git 子进程：用 git 自带的 sh 别名读回。
	gitRun(t, dir, "config", "alias.envprobe", "!printf %s \"$GIT_TERMINAL_PROMPT\"")
	out, err := DefaultGitRunner()(dir, "envprobe")
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out) != "0" {
		t.Fatalf("git 子进程应禁用终端认证提示: %q", out)
	}
}

// gitAvailable 跳过没有 git 的环境；端到端测试全部落在 t.TempDir()。
func gitAvailable(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("系统无 git，跳过端到端测试")
	}
}

// gitRun 在 dir 下执行 git 命令，失败时带输出致命退出。
func gitRun(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func TestStoreGitEndToEndCommitAndPush(t *testing.T) {
	gitAvailable(t)
	repo, remote := t.TempDir(), t.TempDir()
	gitRun(t, repo, "init", "-q")
	gitRun(t, remote, "init", "--bare", "-q")
	gitRun(t, repo, "config", "user.name", "Zakki Test")
	gitRun(t, repo, "config", "user.email", "zakki@example.com")
	gitRun(t, repo, "remote", "add", "origin", remote)

	s := newGitTestStore(t, repo)
	base := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)
	current := base
	s.Now = func() time.Time { return current }
	s.SetGitAutoPush(true, time.Minute)

	if err := s.BackupNow(2); err != nil {
		t.Fatal(err)
	}
	s.WaitGitIdle()
	if head := gitRun(t, repo, "rev-parse", "HEAD"); head == "" {
		t.Fatal("备份后仓库应有提交")
	}
	if log := gitRun(t, repo, "log", "-1", "--pretty=%s"); !strings.HasPrefix(log, "backup: zakki-") {
		t.Fatalf("提交消息应为备份命名: %q", log)
	}
	if s.GitError() != "" {
		t.Fatalf("不应有错误: %q", s.GitError())
	}

	// 推送节流内不重复推；超过间隔后远端追上本地。
	if out := gitRun(t, repo, "ls-remote", "origin"); out == "" {
		t.Fatal("首次备份应已推送到远端")
	}
	if err := os.WriteFile(filepath.Join(repo, "unrelated.txt"), []byte("user"), 0o644); err != nil {
		t.Fatal(err)
	}
	current = base.Add(2 * time.Hour)
	if err := s.BackupNow(1); err != nil {
		t.Fatal(err)
	}
	s.WaitGitIdle()
	// keep=1：旧备份被修剪，删除也应进提交；用户文件不受影响。
	if entries, err := os.ReadDir(repo); err == nil {
		count := 0
		for _, e := range entries {
			if !e.IsDir() && isBackupFileName(e.Name()) {
				count++
			}
		}
		if count != 1 {
			t.Fatalf("keep=1 应只保留一份备份: %d", count)
		}
	}
	if status := gitRun(t, repo, "status", "--porcelain"); status != "?? unrelated.txt" {
		t.Fatalf("仓库应只剩用户自己的未跟踪文件: %q", status)
	}
	local := gitRun(t, repo, "rev-parse", "HEAD")
	remoteHead := gitRun(t, remote, "rev-parse", "HEAD")
	if local != remoteHead {
		t.Fatalf("推送后远端应与本地一致: local=%s remote=%s", local, remoteHead)
	}
}

// TestGitToolsReady macOS 的 /usr/bin/git 垫片只在开发者目录里真有 git 时可用；
// 其他路径（Homebrew 等）与其他平台直接视为可用，不做额外探测。
func TestGitToolsReady(t *testing.T) {
	noDir := func() (string, error) { return "", errors.New("unable to get active developer directory") }
	cltDir := func() (string, error) { return "/Library/Developer/CommandLineTools", nil }
	has := func(want string) func(string) bool { return func(p string) bool { return p == want } }
	cases := []struct {
		name, goos, path string
		dir              func() (string, error)
		exists           func(string) bool
		want             bool
	}{
		{"mac 垫片未装工具", "darwin", macGitShim, noDir, has(""), false},
		{"mac 垫片开发者目录缺 git", "darwin", macGitShim, cltDir, has(""), false},
		{"mac 垫片已装工具", "darwin", macGitShim, cltDir, has("/Library/Developer/CommandLineTools/usr/bin/git"), true},
		{"mac Homebrew git", "darwin", "/opt/homebrew/bin/git", noDir, has(""), true},
		{"windows", "windows", `C:\Program Files\Git\cmd\git.exe`, noDir, has(""), true},
	}
	for _, tc := range cases {
		if got := gitToolsReady(tc.goos, tc.path, tc.dir, tc.exists); got != tc.want {
			t.Errorf("%s: gitToolsReady = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// TestStoreGitMissingToolsIsSilent 本机没有 git 命令行工具：备份照常成功，
// 不记失败、不推送，只标记 GitMissing；装好后下一次备份自动恢复。
func TestStoreGitMissingToolsIsSilent(t *testing.T) {
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	s := newGitTestStore(t, repo)
	missing := true
	g := &fakeGit{}
	s.RunGit = func(dir string, args ...string) (string, error) {
		if missing {
			return "", ErrGitUnavailable
		}
		return g.run(dir, args...)
	}
	s.SetGitAutoPush(true, time.Minute)
	if err := s.BackupNow(20); err != nil {
		t.Fatalf("缺 git 不得影响备份: %v", err)
	}
	s.WaitGitIdle()
	if !s.GitMissing() || s.GitError() != "" {
		t.Fatalf("缺 git 应静默标记: missing=%v err=%q", s.GitMissing(), s.GitError())
	}
	missing = false
	if err := s.BackupNow(20); err != nil {
		t.Fatal(err)
	}
	s.WaitGitIdle()
	if s.GitMissing() || g.count("push") != 1 {
		t.Fatalf("装好 git 后应恢复: missing=%v push=%d", s.GitMissing(), g.count("push"))
	}
}
