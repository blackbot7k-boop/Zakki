package app

// 备份文件夹：选择、校验与偏好写入。
// 设置页「备份与数据」与首启向导共用这里的逻辑。备份文件夹是唯一的备份
// 存储地址：每次备份直写这里；未选择时写入应用数据目录。

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo"

	"zakki/locales"
)

// canonicalPath 解析为绝对路径并跟随符号链接，便于与数据目录比较。
func canonicalPath(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	abs = filepath.Clean(abs)
	return filepath.EvalSymlinks(abs)
}

// validateBackupDir 校验所选文件夹可作为备份文件夹：拒绝应用数据目录
// （含其子目录，避免把笔记数据自我嵌套进备份），并试写一个探测文件确认
// 可写——它是唯一的备份副本，不可写必须当场暴露。
// 返回清理后的绝对路径。
func (a *app) validateBackupDir(path string) (string, error) {
	abs, err := canonicalPath(path)
	if err != nil {
		return "", fmt.Errorf("%s", locales.BackupErrParse())
	}
	dataDir, err := canonicalPath(filepath.Dir(a.notes.DataFile))
	if err != nil {
		dataDir = filepath.Clean(filepath.Dir(a.notes.DataFile))
	}
	if abs == dataDir || strings.HasPrefix(abs, dataDir+string(filepath.Separator)) {
		return "", fmt.Errorf("%s", locales.BackupErrAppData())
	}
	probe, err := os.CreateTemp(abs, ".zakki-backup-probe-*")
	if err != nil {
		return "", fmt.Errorf("%s", locales.BackupErrNotWritable())
	}
	probeName := probe.Name()
	if err := probe.Close(); err != nil {
		_ = os.Remove(probeName)
		return "", fmt.Errorf("%s", locales.BackupErrNotWritable())
	}
	if err := os.Remove(probeName); err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("%s", locales.BackupErrNotWritable())
	}
	return abs, nil
}

// chooseBackupDirAsync 在后台 goroutine 弹原生目录选择器。mygo.Dialog.Open
// 会等待主线程分发，绝不能在主线程直接调用；结果经 notify 窗口（或主窗口）
// 的 Update 回主线程调 cb。用户取消或事件循环不可用时静默返回。
func (a *app) chooseBackupDirAsync(notify *mygo.Window, cb func(path string)) {
	if !a.ready {
		return // 启动早期或测试环境：没有事件循环，无法弹原生对话框
	}
	fallback := a.win
	go func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
			Parent:            notify,
			Title:             locales.DialogChooseBackupTitle(),
			Message:           locales.DialogChooseBackupMessage(),
			ButtonLabel:       locales.DialogChooseBackupButton(),
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

// pickBackupFolder 设置页入口：选择结果回主线程应用到偏好。
func (a *app) pickBackupFolder() {
	a.chooseBackupDirAsync(a.settingsWin, a.applyBackupDir)
}

// applyBackupDir 主线程：校验所选文件夹并写入偏好。
func (a *app) applyBackupDir(path string) {
	abs, err := a.validateBackupDir(path)
	if err != nil {
		a.settingsUI.keyErr = err.Error()
		return
	}
	if err := a.updatePreferences(func(p *preferences) {
		p.Backup.BackupDir = abs
	}); err != nil {
		a.settingsUI.keyErr = err.Error()
		return
	}
	a.settingsUI.syncDraftFromPrefs(a.prefs)
	a.settingsUI.keyErr = ""
}

// disconnectBackupFolder 断开所选文件夹，备份回到内置目录（应用数据）。
func (a *app) disconnectBackupFolder() {
	if err := a.updatePreferences(func(p *preferences) {
		p.Backup.BackupDir = ""
	}); err != nil {
		a.settingsUI.keyErr = err.Error()
		return
	}
	a.settingsUI.syncDraftFromPrefs(a.prefs)
	a.settingsUI.keyErr = ""
}
