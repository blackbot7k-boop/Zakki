package notes

import (
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

type syncGit struct {
	mu      sync.Mutex
	root    string
	calls   [][]string
	fail    string
	missing bool
	before  func([]string)
}

func (g *syncGit) run(_ string, args ...string) (string, error) {
	if g.before != nil {
		g.before(args)
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	g.calls = append(g.calls, append([]string(nil), args...))
	if g.missing {
		return "", ErrGitUnavailable
	}
	if args[0] == g.fail {
		return "", errors.New("injected git failure")
	}
	switch args[0] {
	case "rev-parse":
		return g.root, nil
	case "status":
		return " M zakki.json", nil
	}
	return "", nil
}

func (g *syncGit) count(cmd string) int {
	g.mu.Lock()
	defer g.mu.Unlock()
	n := 0
	for _, args := range g.calls {
		if args[0] == cmd {
			n++
		}
	}
	return n
}

func await(t *testing.T, check func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if check() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("等待超时")
}

func gitStore(t *testing.T) (*Store, *syncGit) {
	s := newTestStore(t, nil)
	g := &syncGit{root: filepath.Dir(s.DataPath())}
	s.RunGit = g.run
	if err := s.SaveNow(doc("磁盘")); err != nil {
		t.Fatal(err)
	}
	return s, g
}

func TestGitIndependentRetryAndNoRenderReset(t *testing.T) {
	s, g := gitStore(t)
	g.fail = "commit"
	cfg := GitConfig{AutoCommit: true, CommitInterval: 25 * time.Millisecond}
	s.ConfigureGit(cfg, nil)
	await(t, func() bool { return g.count("commit") > 0 && s.GitStatus().Error != "" })
	g.mu.Lock()
	g.fail = ""
	g.mu.Unlock()
	stop := time.Now().Add(200 * time.Millisecond)
	for time.Now().Before(stop) && s.GitStatus().CommitTime.IsZero() {
		s.ConfigureGit(cfg, nil)
		time.Sleep(time.Millisecond)
	}
	if s.GitStatus().CommitTime.IsZero() || s.GitStatus().Error != "" {
		t.Fatalf("无新输入也应重试成功：%+v", s.GitStatus())
	}
	if g.count("push") != 0 {
		t.Fatal("推送独立关闭")
	}
	s.Close()
	s.WaitGitIdle()
}

func TestGitPushRetriesWithoutInput(t *testing.T) {
	s, g := gitStore(t)
	g.fail = "push"
	s.ConfigureGit(GitConfig{AutoPush: true, PushInterval: 20 * time.Millisecond}, nil)
	await(t, func() bool { return g.count("push") > 0 && s.GitStatus().Error != "" })
	g.mu.Lock()
	g.fail = ""
	g.mu.Unlock()
	await(t, func() bool { return !s.GitStatus().PushTime.IsZero() && s.GitStatus().Error == "" })
	if g.count("commit") != 0 {
		t.Fatal("提交独立关闭")
	}
	s.Close()
	s.WaitGitIdle()
}

func TestGitMissingAndRecovery(t *testing.T) {
	s, g := gitStore(t)
	g.missing = true
	s.ConfigureGit(GitConfig{AutoCommit: true, CommitInterval: 20 * time.Millisecond}, nil)
	await(t, func() bool { return s.GitStatus().Missing })
	if s.GitStatus().Error != "" {
		t.Fatal("缺失 Git 应静默")
	}
	g.mu.Lock()
	g.missing = false
	g.mu.Unlock()
	await(t, func() bool { return !s.GitStatus().Missing && !s.GitStatus().CommitTime.IsZero() })
	s.Close()
	s.WaitGitIdle()
}

func TestGitGenerationStopsCommandsNotOnlyResults(t *testing.T) {
	for _, mode := range []string{"disable", "relocate", "blocked", "close"} {
		t.Run(mode, func(t *testing.T) {
			s, g := gitStore(t)
			started, release := make(chan struct{}), make(chan struct{})
			var once sync.Once
			g.before = func(args []string) {
				if args[0] == "rev-parse" {
					once.Do(func() { close(started); <-release })
				}
			}
			s.ConfigureGit(GitConfig{AutoCommit: true, AutoPush: true, CommitInterval: time.Millisecond, PushInterval: time.Millisecond}, nil)
			<-started
			switch mode {
			case "disable":
				s.ConfigureGit(GitConfig{}, nil)
			case "blocked":
				s.SetSaveBlocked(true)
			case "close":
				s.Close()
			case "relocate":
				target := filepath.Join(t.TempDir(), "zakki.json")
				if err := s.Relocate(target, doc("迁址"), func() error { return nil }); err != nil {
					t.Fatal(err)
				}
				s.ConfigureGit(GitConfig{}, nil)
			}
			close(release)
			s.WaitGitIdle()
			if g.count("commit") != 0 || g.count("push") != 0 || g.count("add") != 0 {
				t.Fatalf("过期任务仍在执行：%v", g.calls)
			}
			if !s.GitStatus().CommitTime.IsZero() || s.GitStatus().Error != "" {
				t.Fatalf("过期结果：%+v", s.GitStatus())
			}
		})
	}
}

func TestSlowGitDoesNotBlockSaveAndClose(t *testing.T) {
	s, g := gitStore(t)
	started, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	g.before = func(args []string) {
		if args[0] == "push" {
			once.Do(func() { close(started); <-release })
		}
	}
	s.ConfigureGit(GitConfig{AutoPush: true, PushInterval: time.Millisecond}, nil)
	<-started
	done := make(chan error, 1)
	go func() { err := s.SaveNow(doc("网络等待期间")); s.Close(); done <- err }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(200 * time.Millisecond):
		close(release)
		t.Fatal("Git 阻塞保存或 Close")
	}
	close(release)
	s.WaitGitIdle()
	if readFile(t, s.DataPath()).Tabs[0].Text != "网络等待期间" {
		t.Fatal("保存未完成")
	}
}

func TestGitExternalChangeOrPauseNoCommit(t *testing.T) {
	for _, mode := range []string{"external", "blocked", "default"} {
		t.Run(mode, func(t *testing.T) {
			s, g := gitStore(t)
			if mode == "external" {
				writeRaw(t, s.DataPath(), "external")
			}
			if mode == "blocked" {
				s.SetSaveBlocked(true)
			}
			if mode != "default" {
				s.ConfigureGit(GitConfig{AutoCommit: true, CommitInterval: 5 * time.Millisecond}, nil)
			}
			time.Sleep(30 * time.Millisecond)
			s.Close()
			s.WaitGitIdle()
			if g.count("commit") != 0 {
				t.Fatal("保护状态不应提交")
			}
		})
	}
}

func initRepo(t *testing.T, dir string) {
	gitAvailable(t)
	gitRun(t, dir, "init", "-q")
	gitRun(t, dir, "config", "user.name", "Notes Test")
	gitRun(t, dir, "config", "user.email", "notes@example.invalid")
	gitRun(t, dir, "config", "commit.gpgsign", "false")
}

func TestGitExactLiteralPathFirstUntrackedAndOtherStaged(t *testing.T) {
	for _, name := range []string{"zakki.json", "[draft]*.json", ":(glob)*.json"} {
		t.Run(name, func(t *testing.T) {
			repo := t.TempDir()
			initRepo(t, repo)
			writeRaw(t, filepath.Join(repo, "base"), "base")
			gitRun(t, repo, "add", "base")
			gitRun(t, repo, "commit", "-qm", "base")
			writeRaw(t, filepath.Join(repo, "other.txt"), "user staged")
			gitRun(t, repo, "add", "other.txt")
			path := filepath.Join(repo, "sub", name)
			s := NewStore(path, newTestClock().now, testMigrateOpt())
			defer s.Close()
			if err := s.SaveNow(doc("第一笔")); err != nil {
				t.Fatal(err)
			}
			committed, err := runGitJob(DefaultGitRunner(), repo, filepath.Join("sub", name))
			if err != nil || !committed {
				t.Fatalf("首次提交：%v %v", committed, err)
			}
			got := gitRun(t, repo, "diff-tree", "--no-commit-id", "--name-only", "-r", "HEAD")
			if got != "sub/"+name {
				t.Fatalf("只应提交精确目标：%q", got)
			}
			if staged := gitRun(t, repo, "diff", "--cached", "--name-only"); staged != "other.txt" {
				t.Fatalf("其他暂存应保留：%q", staged)
			}
			committed, err = runGitJob(DefaultGitRunner(), repo, filepath.Join("sub", name))
			if err != nil || committed {
				t.Fatalf("无变化：%v %v", committed, err)
			}
		})
	}
}

func TestGitSubdirAndWorktree(t *testing.T) {
	repo := t.TempDir()
	initRepo(t, repo)
	writeRaw(t, filepath.Join(repo, "base"), "base")
	gitRun(t, repo, "add", "base")
	gitRun(t, repo, "commit", "-qm", "base")
	work := filepath.Join(t.TempDir(), "work")
	gitRun(t, repo, "worktree", "add", "-qb", "notes-work", work)
	for _, root := range []string{repo, work} {
		s := NewStore(filepath.Join(root, "nested", "zakki.json"), newTestClock().now, testMigrateOpt())
		if err := s.SaveNow(doc("worktree")); err != nil {
			t.Fatal(err)
		}
		s.ConfigureGit(GitConfig{AutoCommit: true, CommitInterval: 20 * time.Millisecond}, nil)
		await(t, func() bool { return !s.GitStatus().CommitTime.IsZero() || s.GitStatus().Error != "" })
		s.Close()
		s.WaitGitIdle()
		if s.GitStatus().CommitTime.IsZero() {
			t.Fatalf("%+v", s.GitStatus())
		}
		canonical, _ := canonicalPath(root)
		if s.GitStatus().RepoRoot != canonical {
			t.Fatalf("仓库根：%s != %s", s.GitStatus().RepoRoot, canonical)
		}
		if status := gitRun(t, root, "status", "--porcelain"); status != "" {
			t.Fatalf("未提交：%s", status)
		}
	}
}

func TestGitPushLocalRemote(t *testing.T) {
	repo, remote := t.TempDir(), t.TempDir()
	initRepo(t, repo)
	gitRun(t, remote, "init", "--bare", "-q")
	gitRun(t, repo, "remote", "add", "origin", remote)
	// 不替用户设置上游：测试先建立分支关联，自动化只执行普通 push。
	writeRaw(t, filepath.Join(repo, "base"), "base")
	gitRun(t, repo, "add", "base")
	gitRun(t, repo, "commit", "-qm", "base")
	gitRun(t, repo, "push", "-u", "origin", "HEAD")
	s := NewStore(filepath.Join(repo, "zakki.json"), newTestClock().now, testMigrateOpt())
	defer s.Close()
	if err := s.SaveNow(doc("推送")); err != nil {
		t.Fatal(err)
	}
	s.ConfigureGit(GitConfig{AutoCommit: true, AutoPush: true, CommitInterval: 20 * time.Millisecond, PushInterval: 40 * time.Millisecond}, nil)
	await(t, func() bool { return !s.GitStatus().PushTime.IsZero() })
	s.Close()
	s.WaitGitIdle()
	if local, remoteHead := gitRun(t, repo, "rev-parse", "HEAD"), gitRun(t, remote, "rev-parse", "HEAD"); local != remoteHead {
		t.Fatalf("%s != %s", local, remoteHead)
	}
}
