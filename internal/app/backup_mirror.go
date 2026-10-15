package app

// 外部备份目的地：选择、校验与 Git 模式映射。
// 设置页「备份与数据」的「外部备份」卡片与首启向导共用这里的逻辑。

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo"
)

// gitModeOptions 是 Git 自动化的展示顺序，与 gitModeDisplay/gitModeOfDisplay 对应。
var gitModeOptions = []string{"关闭", "自动提交", "提交并推送"}

func gitModeDisplay(mode string) string {
	switch mode {
	case "commit":
		return "自动提交"
	case "commitPush":
		return "提交并推送"
	default:
		return "关闭"
	}
}

func gitModeOfDisplay(sel string) string {
	switch sel {
	case "自动提交":
		return "commit"
	case "提交并推送":
		return "commitPush"
	default:
		return "off"
	}
}

// dirIsGitRepo 目录是否为 Git 仓库（含 worktree 的 .git 文件形态）。
func dirIsGitRepo(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, ".git"))
	return err == nil
}

// validateMirrorDir 校验所选目录可用于外部备份：拒绝应用数据目录与本地备份
// 目录（含其子目录，避免自我嵌套），并试写一个探测文件确认可写。
// 返回清理后的绝对路径。
// canonicalPath 解析为绝对路径并跟随符号链接，便于与数据目录比较。
func canonicalPath(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	abs = filepath.Clean(abs)
	return filepath.EvalSymlinks(abs)
}

func (a *app) validateMirrorDir(path string) (string, error) {
	abs, err := canonicalPath(path)
	if err != nil {
		return "", fmt.Errorf("解析路径失败: %w", err)
	}
	backupDir, err := canonicalPath(a.notes.BackupDir)
	if err != nil {
		backupDir = filepath.Clean(a.notes.BackupDir)
	}
	dataDir, err := canonicalPath(filepath.Dir(a.notes.BackupDir))
	if err != nil {
		dataDir = filepath.Clean(filepath.Dir(a.notes.BackupDir))
	}
	if abs == backupDir || strings.HasPrefix(abs, backupDir+string(filepath.Separator)) {
		return "", fmt.Errorf("不能选择本地备份目录（含其子目录）作为外部备份")
	}
	if abs == dataDir {
		return "", fmt.Errorf("不能选择应用数据目录作为外部备份")
	}
	probe := filepath.Join(abs, ".zakki-backup-probe")
	if err := os.WriteFile(probe, []byte("ok"), 0o644); err != nil {
		return "", fmt.Errorf("文件夹不可写: %w", err)
	}
	_ = os.Remove(probe)
	return abs, nil
}

// chooseMirrorDirAsync 在后台 goroutine 弹原生目录选择器。mygo.Dialog.Open
// 会等待主线程分发，绝不能在主线程直接调用；结果经 notify 窗口（或主窗口）
// 的 Update 回主线程调 cb。用户取消或事件循环不可用时静默返回。
func (a *app) chooseMirrorDirAsync(notify *mygo.Window, cb func(path string)) {
	if !a.ready {
		return // 启动早期或测试环境：没有事件循环，无法弹原生对话框
	}
	fallback := a.win
	go func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
			Parent:            notify,
			Title:             "选择备份文件夹",
			Message:           "选择云盘同步文件夹（iCloud、坚果云等）或 Git 仓库的本地 clone；每次备份会镜像一份到这里",
			ButtonLabel:       "用作备份文件夹",
			Directory:         true,
			CreateDirectories: true,
		})
		if err != nil || len(paths) == 0 || paths[0] == "" {
			return // 用户取消：不打扰
		}
		path := paths[0]
		switch {
		case notify != nil:
			notify.Update(func() { cb(path) })
		case fallback != nil:
			fallback.Update(func() { cb(path) })
		}
	}()
}

// pickMirrorFolder 设置页入口：选择结果回主线程应用到偏好。
func (a *app) pickMirrorFolder() {
	a.chooseMirrorDirAsync(a.settingsWin, a.applyMirrorDir)
}

// applyMirrorDir 主线程：校验所选文件夹并写入偏好；非 Git 仓库时联动关闭
// Git 自动化（用户仍可把它当普通云盘同步目录用）。
func (a *app) applyMirrorDir(path string) {
	abs, err := a.validateMirrorDir(path)
	if err != nil {
		a.settingsUI.keyErr = err.Error()
		return
	}
	mode := a.prefs.Backup.GitMode
	if !dirIsGitRepo(abs) {
		mode = "off"
	}
	if err := a.updatePreferences(func(p *preferences) {
		p.Backup.MirrorDir = abs
		p.Backup.GitMode = mode
	}); err != nil {
		a.settingsUI.keyErr = err.Error()
		return
	}
	a.settingsUI.syncDraftFromPrefs(a.prefs)
	a.settingsUI.keyErr = ""
}

// disconnectMirror 断开外部备份并关闭 Git 自动化。
func (a *app) disconnectMirror() {
	if err := a.updatePreferences(func(p *preferences) {
		p.Backup.MirrorDir = ""
		p.Backup.GitMode = "off"
	}); err != nil {
		a.settingsUI.keyErr = err.Error()
		return
	}
	a.settingsUI.syncDraftFromPrefs(a.prefs)
	a.settingsUI.keyErr = ""
}

// pushMirrorGitMode 设置页 Git 分段变更：非 Git 仓库目录时拒绝非 off 档位。
func (a *app) pushMirrorGitMode() {
	mode := gitModeOfDisplay(a.settingsUI.gitModeSel)
	if dir := a.settingsUI.draft.Backup.MirrorDir; dir != "" && mode != "off" && !dirIsGitRepo(dir) {
		a.settingsUI.gitModeSel = gitModeDisplay(a.prefs.Backup.GitMode)
		a.settingsUI.keyErr = "当前备份文件夹不是 Git 仓库，无法开启 Git 自动化"
		return
	}
	a.settingsUI.draft.Backup.GitMode = mode
	a.pushBackup()
}

// mirrorStateText 设置页外部备份的状态行：类型 + 最近镜像结果。
func (a *app) mirrorStateText(dir string) string {
	kind := "文件夹"
	if dirIsGitRepo(dir) {
		kind = "Git 仓库"
	}
	st := a.notes.MirrorStatus()
	switch {
	case st.Err != "":
		return kind + " · 最近镜像失败：" + st.Err
	case st.Last.IsZero():
		return kind + " · 尚未镜像（下次备份时写入）"
	default:
		return kind + " · 最近镜像 " + st.Last.Format("01-02 15:04")
	}
}

// gitModeDetailText 设置页 Git 自动化行按当前目录状态给出的说明。
func (a *app) gitModeDetailText() string {
	dir := a.prefs.Backup.MirrorDir
	switch {
	case dir == "":
		return "先选择备份文件夹；仅对 Git 仓库目录可用"
	case !dirIsGitRepo(dir):
		return "当前文件夹不是 Git 仓库，Git 自动化不可用；作为云盘同步目录使用没有问题"
	case a.prefs.Backup.GitMode == "commitPush":
		return "每次备份写入后 git add tot-md-*.json 并提交、推送；失败只提示，不影响备份本身"
	default:
		return "每次备份写入后 git add tot-md-*.json 并提交；失败只提示，不影响备份本身"
	}
}
