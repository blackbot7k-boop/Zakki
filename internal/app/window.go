package app

import (
	"runtime"

	"github.com/egoist/mygo/ui"

	"zakki/internal/prefs"
)

// 主窗口最小尺寸；默认尺寸换算出的像素不会小于它。
const (
	minWindowWidth  = 300
	minWindowHeight = 340
)

// windowMetrics 当前生效排版下列数 × 行数与窗口像素的换算量：正文字号、行高与
// 内边距取覆盖值或排版主题默认，顶栏按平台（mac 38，Windows 隐藏标题栏 32）加状态栏。
func windowMetrics(ap appearancePrefs, goos string) prefs.WindowMetrics {
	layout := markdownLayoutByID(ap.EffectiveMarkdownTheme())
	header := tabTrackHeight
	if goos == "windows" {
		header = 32
	}
	m := prefs.WindowMetrics{
		BodySize: layout.bodySize, LineHeight: layout.lineHeight,
		PadX: layout.padX, PadY: layout.padY, Chrome: header + statusBarHeight,
	}
	if ap.BodySize > 0 {
		m.BodySize = ap.BodySize
	}
	if ap.LineHeight > 0 {
		m.LineHeight = ap.LineHeight
	}
	if ap.PadX > 0 {
		m.PadX = ap.PadX
	}
	if ap.PadY > 0 {
		m.PadY = ap.PadY
	}
	return m
}

// defaultWindowSize 新建主窗口的像素尺寸：按当前字号换算偏好里的每行字数 × 行数。
func defaultWindowSize(p preferences) (int, int) {
	w, h := prefs.WindowPixels(p.Advanced.WindowColumns, p.Advanced.WindowRows, windowMetrics(p.Appearance, runtime.GOOS))
	return max(w, minWindowWidth), max(h, minWindowHeight)
}

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
