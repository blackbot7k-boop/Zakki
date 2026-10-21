package notes

// 备份文件夹的 Git 自动化：备份写入成功后，把备份文件的变更 commit 到备份
// 文件夹自身的 git 仓库，并按配置间隔自动 push。只提交本应用备份命名空间
// 的文件，不碰仓库里用户存放的其他内容；不做 pull/rebase/force push，远端
// 分叉导致的推送失败如实记录，等下个周期重试。

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
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

// GitRunner 执行一条在 dir 目录下运行的 git 命令，返回合并的命令输出。
// 抽出来是为了测试可以注入假 runner，不依赖系统 git。
type GitRunner func(dir string, args ...string) (string, error)

// DirIsGitRepo 报告 dir 自身是否为 git 仓库（含 worktree 的 .git 文件形态）。
// 只看所选文件夹一层：备份文件夹应当是仓库根，不向上查找嵌套仓库。
func DirIsGitRepo(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, ".git"))
	return err == nil
}

// DefaultGitRunner 返回调用本机 git CLI 的 runner。认证复用用户既有的
// ssh-agent / 凭据助手配置；git 未安装时错误会经 gitErr 如实上报。
func DefaultGitRunner() GitRunner {
	return func(dir string, args ...string) (string, error) {
		ctx, cancel := context.WithTimeout(context.Background(), gitCommandTimeout)
		defer cancel()
		cmd := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...)
		cmd.Env = append(os.Environ(), gitBatchEnv...)
		cmd.WaitDelay = gitWaitDelay
		var out strings.Builder
		cmd.Stdout, cmd.Stderr = &out, &out
		err := cmd.Run()
		if err != nil {
			return strings.TrimSpace(out.String()), fmt.Errorf("git %s: %w（%s）",
				strings.Join(args, " "), err, strings.TrimSpace(out.String()))
		}
		return out.String(), nil
	}
}

// gitBackupPaths 汇总本次提交涉及的备份文件路径：目录里现有的备份文件加上
// 已跟踪的备份文件（后者覆盖本地已修剪的删除）。只认本应用的备份命名空间。
func gitBackupPaths(runner GitRunner, dir string) ([]string, error) {
	paths := make(map[string]struct{})
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if !entry.IsDir() && isBackupFileName(entry.Name()) {
			paths[entry.Name()] = struct{}{}
		}
	}
	tracked, err := runner(dir, "ls-files", "-z", "--", "zakki-*.json", "tot-md-*.json")
	if err != nil {
		return nil, err
	}
	for _, path := range strings.Split(tracked, "\x00") {
		if path == "" || filepath.Clean(path) != filepath.Base(path) || !isBackupFileName(path) {
			continue
		}
		paths[path] = struct{}{}
	}
	out := make([]string, 0, len(paths))
	for path := range paths {
		out = append(out, path)
	}
	sort.Strings(out)
	return out, nil
}

// runGitJob 把备份文件的变更提交到 dir 的 git 仓库；mode 不再区分档位，
// 提交与否由有无变更决定，推送由调用方按间隔节流。返回本次是否产生了提交。
// 精确路径来自当前文件和已跟踪文件，既能记录删除，也不会把不存在的旧前缀
// 作为 pathspec 交给 add/commit。
func runGitJob(runner GitRunner, dir, message string) (bool, error) {
	paths, err := gitBackupPaths(runner, dir)
	if err != nil {
		return false, err
	}
	if len(paths) == 0 {
		return false, nil
	}
	if _, err := runner(dir, append([]string{"add", "--"}, paths...)...); err != nil {
		return false, err
	}
	args := append([]string{"status", "--porcelain", "--"}, paths...)
	out, err := runner(dir, args...)
	if err != nil {
		return false, err
	}
	if strings.TrimSpace(out) == "" {
		return false, nil // 没有本应用备份变更：跳过提交
	}
	args = append([]string{"commit", "--only", "-m", message, "--"}, paths...)
	if _, err := runner(dir, args...); err != nil {
		return false, err
	}
	return true, nil
}
