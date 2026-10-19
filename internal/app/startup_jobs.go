package app

import "log/slog"

// startStartupTasks 只在首个窗口完成首帧后调用一次；启动备份在后台进行。
func (a *app) startStartupTasks() {
	store := a.notes
	enabled, keep := a.prefs.Backup.OnLaunch, a.prefs.Backup.Keep
	refresh := func() { a.runOnMain(func() {}) }
	go func() {
		if err := store.LoadLastBackupTime(); err != nil {
			slog.Warn("读取最近备份时间失败", "error", err)
		}
		if err := store.BackupOnLaunch(enabled, keep); err != nil {
			slog.Warn("启动备份失败", "error", err)
		}
		refresh()
	}()
}
