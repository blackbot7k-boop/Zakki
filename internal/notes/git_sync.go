package notes

// 唯一笔记文件的 Git 同步；不执行 pull、rebase 或强制推送。

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// gitCommandTimeout 是单条 git 命令的超时：推送在网络问题上挂起时不能
// 长时间占住唯一的 git worker。gitWaitDelay 是超时杀进程后等待 ssh/凭据
// 助手等子进程释放输出管道的上限，避免 Wait 被孙进程拖住。
const (
	gitCommandTimeout = time.Minute
	gitWaitDelay      = 5 * time.Second
)

// gitBatchEnv 让 git 在无终端的 GUI 进程里不弹交互式认证：需要输入用户名、
// 密码或口令时直接失败并上报，而不是挂到超时。不覆盖 GIT_SSH_COMMAND，
// 以免绕过用户在 core.sshCommand 里的配置。
var gitBatchEnv = []string{"GIT_TERMINAL_PROMPT=0", "GCM_INTERACTIVE=never"}

// ErrGitUnavailable 表示本机没有可直接运行的 git 命令行工具。Git 自动化遇到它
// 静默跳过：不记失败、不弹系统安装提示，装好工具后定时重试。
var ErrGitUnavailable = errors.New("git 命令行工具不可用")

// macGitShim 是 macOS 自带的 git 垫片：未装命令行工具时运行它会弹出系统
// 安装对话框，不能当作「git 已安装」。
const macGitShim = "/usr/bin/git"

// gitToolsReady 报告 gitPath 能否直接运行而不触发系统安装提示。macOS 上的
// 垫片只在开发者目录（xcode-select -p，查询本身不弹窗）里真有 git 时可用。
func gitToolsReady(goos, gitPath string, developerDir func() (string, error), exists func(string) bool) bool {
	if goos != "darwin" || gitPath != macGitShim {
		return true
	}
	dir, err := developerDir()
	if err != nil || dir == "" {
		return false
	}
	return exists(filepath.Join(dir, "usr", "bin", "git"))
}

// xcodeDeveloperDir 查询当前激活的开发者目录；未安装命令行工具时返回错误。
func xcodeDeveloperDir() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "/usr/bin/xcode-select", "-p").Output()
	return strings.TrimSpace(string(out)), err
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// GitRunner 执行一条在 dir 目录下运行的 git 命令，返回合并的命令输出。
// 抽出来是为了测试可以注入假 runner，不依赖系统 git。
type GitRunner func(dir string, args ...string) (string, error)

// DirIsGitRepo 探测目录或祖先是否含 Git 元数据，兼容 worktree。
func DirIsGitRepo(dir string) bool {
	for {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return false
		}
		dir = parent
	}
}

// DefaultGitRunner 返回调用本机 git CLI 的 runner。认证复用用户既有的
// ssh-agent / 凭据助手配置；找不到可直接运行的 git 时返回 ErrGitUnavailable。
func DefaultGitRunner() GitRunner {
	return func(dir string, args ...string) (string, error) {
		return runGitContext(context.Background(), dir, args...)
	}
}

func runGitContext(parent context.Context, dir string, args ...string) (string, error) {
	gitPath, err := exec.LookPath("git")
	if err != nil || !gitToolsReady(runtime.GOOS, gitPath, xcodeDeveloperDir, fileExists) {
		return "", ErrGitUnavailable
	}
	ctx, cancel := context.WithTimeout(parent, gitCommandTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, gitPath, append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), gitBatchEnv...)
	cmd.WaitDelay = gitWaitDelay
	var out strings.Builder
	cmd.Stdout, cmd.Stderr = &out, &out
	if err := cmd.Run(); err != nil {
		return strings.TrimSpace(out.String()), fmt.Errorf("git %s: %w（%s）",
			strings.Join(args, " "), err, strings.TrimSpace(out.String()))
	}
	return out.String(), nil
}

// runGitJob 只暂存并提交一个 literal 路径；首次未跟踪文件也可提交。
func runGitJob(runner GitRunner, dir, path string) (bool, error) {
	literal := ":(top,literal)" + filepath.ToSlash(path)
	if _, err := runner(dir, "add", "--", literal); err != nil {
		return false, err
	}
	out, err := runner(dir, "status", "--porcelain", "--", literal)
	if err != nil {
		return false, err
	}
	if strings.TrimSpace(out) == "" {
		return false, nil
	}
	if _, err := runner(dir, "commit", "--only", "-m", "notes: update zakki.json", "--", literal); err != nil {
		return false, err
	}
	return true, nil
}
