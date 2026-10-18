package app

import "github.com/egoist/mygo/ui"

// resetWindowUI 丢弃绑定旧窗口引擎的控件状态，保留文档、选区和撤销历史。
func (a *app) resetWindowUI() {
	// ListState 保留上一帧的内部节点；不能随应用模型带入新窗口。
	a.liveList = ui.ListState{}
	a.clipSvc, a.clipSvcOK = ui.Services{}, false
	if a.selSyncStart < 0 {
		a.selSyncStart, a.selSyncEnd = a.mdCaret, a.mdSelEnd
	}
	if a.srcSelSyncStart < 0 {
		a.srcSelSyncStart, a.srcSelSyncEnd = a.srcCaret, a.srcSelEnd
	}
}

// ---- 桌面集成 ----

// applyAlwaysOnTop 把置顶设置同步到窗口。
func (a *app) applyAlwaysOnTop() {
	if a.win != nil && a.winOpen {
		a.win.SetAlwaysOnTop(a.cfg.AlwaysOnTop)
	}
}

// showWindow 显示并聚焦窗口（必要时新建）。
func (a *app) showWindow() {
	// 向导期间唤起主窗口：聚焦向导本身，避免绕过初始化流程。
	if a.onboarding.obWin != nil {
		a.onboarding.obWin.Restore()
		a.onboarding.obWin.Show()
		a.onboarding.obWin.Focus()
		return
	}
	if a.win == nil || !a.winOpen {
		a.createWindow()
		return
	}
	a.win.Restore()
	a.win.Show()
	a.win.Focus()
}

// toggleWindow 全局热键与托盘点击：显示/隐藏窗口。
func (a *app) toggleWindow() {
	if a.hidden || a.win == nil || !a.winOpen {
		a.showWindow()
		return
	}
	a.win.Hide()
}
