package app

// 外部备份目的地：选择、校验与 Git 模式映射。
// 设置页「备份与数据」的「外部备份」卡片与首启向导共用这里的逻辑。

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo"

	"zakki/locales"
)

// gitModeOptions 是 Git 自动化的展示顺序（当前语言的文案），与
// gitModeDisplay/gitModeOfDisplay 对应。
func gitModeOptions() []string {
	return []string{locales.GitModeOff(), locales.GitModeCommit(), locales.GitModeCommitPush()}
}

func gitModeDisplay(mode string) string {
	switch mode {
	case "commit":
		return locales.GitModeCommit()
	case "commitPush":
		return locales.GitModeCommitPush()
	default:
		return locales.GitModeOff()
	}
}

func gitModeOfDisplay(sel string) string {
	for i, opt := range gitModeOptions() {
		if opt == sel {
			return [3]string{"off", "commit", "commitPush"}[i]
		}
	}
	return "off"
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
			Title:             locales.DialogChooseMirrorTitle(),
			Message:           locales.DialogChooseMirrorMessage(),
			ButtonLabel:       locales.DialogChooseMirrorButton(),
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
	kind := locales.CommonFolder()
	if dirIsGitRepo(dir) {
		kind = locales.CommonGitRepo()
	}
	st := a.notes.MirrorStatus()
	switch {
	case st.Err != "":
		return locales.MirrorStatusLine(kind, locales.MirrorStateFailed(st.Err))
	case st.Last.IsZero():
		return locales.MirrorStatusLine(kind, locales.MirrorStatePending())
	default:
		return locales.MirrorStatusLine(kind, locales.MirrorStateLast(st.Last.Format("01-02 15:04")))
	}
}

// gitModeDetailText 设置页 Git 自动化行按当前目录状态给出的说明。
func (a *app) gitModeDetailText() string {
	dir := a.prefs.Backup.MirrorDir
	switch {
	case dir == "":
		return locales.GitDetailPickFirst()
	case !dirIsGitRepo(dir):
		return locales.GitDetailNotRepo()
	case a.prefs.Backup.GitMode == "commitPush":
		return locales.GitModeCommitPushDetail()
	default:
		return locales.GitModeCommitDetail()
	}
}
