package notes

import (
	"errors"
	"os/exec"
	"strings"
	"testing"
)

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
