package app

// 窗口作用域快捷键兜底：键位与菜单显示同源（commandRegistry +
// fixedBindingsForPlatform + 用户覆盖），保证任何平台「菜单里显示什么，
// 按下就是什么」。
//   - macOS 等有菜单环境：菜单 accelerator 在系统层先消费按键，这里天然
//     不触发（重复注册无害）；headless/无菜单环境由这里接管，测试可覆盖。
//   - Windows：原生 surface 的按键不触发菜单 accelerator，这里就是实际
//     触发路径。
// 编辑角色（撤销/剪切/复制/粘贴/全选）由 ui 引擎对主修饰键内建处理，
// 不在此重复注册；全局热键走 OS 注册，也不占窗口键位。

import (
	"runtime"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// handleWindowShortcuts 按统一键位表注册并分发窗口级快捷键；命中一条即
// 返回（视图构建期调用，主线程）。注册次序无歧义：键位表经冲突校验，
// 同一平台内物理键互不相同。
func (a *app) handleWindowShortcuts(c *ui.Context) {
	for _, def := range commandRegistry {
		if def.global {
			continue
		}
		b, ok := a.parsedCommandBinding(def.id)
		if !ok {
			continue
		}
		if c.Shortcut(b.mods, b.key) {
			a.runFormatCommand(def.id)
			return
		}
	}
	for _, fixed := range fixedBindingsForPlatform(runtime.GOOS) {
		switch fixed.key {
		case "undo", "redo", "cut", "copy", "paste", "selectAll",
			"appSwitcher", "hideApp":
			continue // 引擎内建或系统/无动作，见文件头注释
		}
		b, err := parseAccelerator(fixed.acc)
		if err != nil {
			continue
		}
		if c.Shortcut(b.mods, b.key) {
			a.runFixedShortcut(fixed.key)
			return
		}
	}
}

// runFixedShortcut 执行固定快捷键的动作；与对应菜单项的 Click 同一实现。
func (a *app) runFixedShortcut(key string) {
	switch key {
	case "find":
		a.findOpen, a.query, a.matchIdx, a.findFocus = true, "", 0, 1
	case "findNext":
		a.findOpenOrStep(1)
	case "findPrev":
		a.findOpenOrStep(-1)
	case "sourceMode":
		a.toggleSourceMode()
	case "keepOnTop":
		a.setAlwaysOnTop(!a.cfg.AlwaysOnTop)
	case "zoomIn":
		a.zoomBodySize(1)
	case "zoomOut":
		a.zoomBodySize(-1)
	case "zoomReset":
		a.zoomBodySize(0)
	case "tab1", "tab2", "tab3", "tab4", "tab5", "tab6", "tab7", "tab8", "tab9":
		if n := int(key[len(key)-1] - '1'); n < len(a.tabs) {
			a.switchTab(n)
		}
	case "newTab":
		a.addTab()
	case "renameTab":
		a.beginRename(a.active)
	case "deleteTab":
		a.confirmDeleteTab(a.active)
	case "prevTab":
		a.cycleTab(-1)
	case "nextTab":
		a.cycleTab(1)
	case "backupNow":
		a.backupNow()
	case "copyTabText":
		a.copyTabTextToClipboard()
	case "settings":
		a.openSettings(pageGeneral)
	case "quit":
		// OnWillQuit（finishQuit）负责最终保存；保存失败会取消退出。
		mygo.App.Quit()
	case "closeWindow":
		if a.win != nil && a.winOpen {
			a.win.Close()
		}
	case "minimize":
		if a.win != nil && a.winOpen {
			a.win.Minimize()
		}
	}
}
