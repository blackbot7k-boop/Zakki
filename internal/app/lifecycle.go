package app

// 应用生命周期：单实例、载入数据与偏好、窗口、菜单、托盘与全局热键。

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// Run 是应用装配与事件循环入口，由根目录 main.go 调用。
func Run() {
	setupLogging()

	// 第二个实例把参数交给第一个，然后退出。
	if !mygo.App.RequestSingleInstanceLock() {
		return
	}

	mygo.App.SetPath(mygo.PathUserData, dataDir())
	mygo.Use(updatePlugin())

	a := newApp()
	if err := a.load(); err != nil {
		slog.Error("数据载入异常（已隔离原件并采用可用数据，未覆盖原文件）", "error", err)
	}

	// 偏好：preferences.json 为唯一来源；首次启动从旧 config 一次性迁移。
	if err := a.loadPreferences(); err != nil {
		slog.Warn("偏好载入未完成（详情见设置页；旧文件保持原样）", "error", err)
	}

	mygo.App.OnSecondInstance(func([]string, string) { a.showWindow() })
	// 关闭窗口后留在托盘，等全局热键或托盘再唤起。
	mygo.App.OnWindowAllClosed(func() {})
	// OnWillQuit 在所有窗口关闭后、事件循环停止前调用：取消防抖定时器、
	// 最终保存数据与偏好并等它完成，之后忽略新的保存请求。
	mygo.App.OnWillQuit(a.finishQuit)

	mygo.App.WhenReady(func() {
		a.ready = true // 之后偏好变更可安全触达原生 API
		// 手写字体用户在开窗前预热注册，避免首帧渲染时的一次性读盘卡顿；
		// 其他用户不付这份内存，首次用到手写字体栈时才懒注册。
		if key := a.prefs.Appearance.BodyFont; key == "hand" || key == "hannotate" {
			registerHandFont()
		}
		if err := a.loadLastBackupTime(); err != nil {
			slog.Warn("读取最近备份时间失败", "error", err)
		}
		if err := a.notes.LoadMirrorTime(); err != nil {
			slog.Warn("读取外部备份状态失败", "error", err)
		}
		applyTheme(a.cfg.ThemeSource)
		a.buildMenu()
		a.setupTray()
		a.syncGlobalHotkey()
		// 全新安装先出首启向导，主窗口等向导结束（或跳过）再创建。
		if a.firstRun() {
			a.openOnboarding()
		} else {
			a.createWindow()
		}
		// 启动备份开窗后异步做：磁盘慢或镜像大时不拖慢首窗；
		// 预取偏好值与窗口引用，避免与主线程的偏好变更/窗口重建产生数据竞争。
		enabled, keep := a.prefs.Backup.OnLaunch, a.prefs.Backup.Keep
		win := a.win
		go func() {
			if err := a.backupOnLaunchAsync(enabled, keep); err != nil {
				slog.Warn("启动备份失败", "error", err)
			}
			if win != nil {
				win.Update(func() {}) // 回主线程刷新状态栏的备份时间
			}
		}()
		slog.Info("应用就绪", "data", a.notes.DataFile, "prefs", a.preferencesPath())
	})

	if err := mygo.App.Run(); err != nil {
		slog.Error("应用运行失败", "error", err)
		os.Exit(1)
	}
}

// setupLogging 把 slog 落到日志目录（控制台同步输出），
// 持久化等错误统一记 INFO/WARN/ERROR。
func setupLogging() {
	opts := &slog.HandlerOptions{Level: slog.LevelInfo}
	dir, err := mygo.App.Path(mygo.PathLogs)
	if err != nil || dir == "" {
		slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, opts)))
		return
	}
	f, err := os.OpenFile(filepath.Join(dir, "zakki.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, opts)))
		return
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(io.MultiWriter(os.Stderr, f), opts)))
}

// applyTheme 把外观模式应用到系统主题。
func applyTheme(src string) {
	switch src {
	case "light":
		mygo.Theme.SetSource(mygo.ThemeLight)
	case "dark":
		mygo.Theme.SetSource(mygo.ThemeDark)
	default:
		mygo.Theme.SetSource(mygo.ThemeSystem)
	}
}

// createWindow 新建主窗口（关闭后可重建）。位置尺寸记忆由 StateKey 提供，
// 「记忆主窗口位置与尺寸」开关从下次创建主窗口起生效。
func (a *app) createWindow() {
	win := mygo.NewWindow(a.windowOptions())
	a.win = win
	a.winOpen = true
	a.hidden = false
	win.OnShow(func() { a.hidden = false })
	win.OnHide(func() { a.hidden = true })
	win.OnFocus(func() { win.Update(func() {}) })
	win.OnBlur(func() { a.setDocumentMenu(false) })
	win.OnClosed(func() {
		a.winOpen, a.win, a.hidden = false, nil, true
		a.syncMenus() // Markdown 命令随主窗口关闭禁用
	})
	// 后台镜像/Git 任务结束后回主线程刷新状态栏；窗口关闭后 Update 自然空转。
	a.notes.OnMirrorSettled = func() { win.Update(func() {}) }
	a.syncMenus() // 重新打开后恢复可用
}

// windowOptions 单独生成选项，便于验证偏好与 MyGo 窗口状态恢复的关系。
// 窗口透明 + VibrancyContent：整窗级毛玻璃，顶栏/底栏用中性半透明层。
func (a *app) windowOptions() mygo.WindowOptions {
	stateKey := ""
	if a.prefs.Window.RememberBounds {
		stateKey = "main"
	}
	return mygo.WindowOptions{
		Title:       appName + " — 速记便签",
		Width:       a.prefs.Advanced.WindowWidth,
		Height:      a.prefs.Advanced.WindowHeight,
		MinWidth:    300,
		MinHeight:   340,
		StateKey:    stateKey, // 开启时，上次窗口状态优先于配置尺寸
		Frameless:   true,     // Tot 同款：无红绿灯、无标题栏，圆角阴影保留，顶栏自绘可拖动
		Transparent: true,     // 透明窗口：页面透明处透出系统材质
		Vibrancy:    mygo.VibrancyContent,
		AlwaysOnTop: a.cfg.AlwaysOnTop,
		Content:     ui.View(a.view),
	}
}

// withWindow 在主线程改状态；窗口开着就触发重绘。
func (a *app) withWindow(fn func()) {
	fn()
	if a.win != nil && a.winOpen {
		a.win.Update(func() {})
	}
}

// ---- 菜单 ----

// buildMenu 组装 macOS 菜单栏（其他平台为窗口菜单）。
func (a *app) buildMenu() {
	a.menu = a.createMenu()
	mygo.App.SetMenu(a.menu)
}

// rebuildTabMenus 标签增删/重命名/排序后重建菜单（标签列表是动态的，
// 菜单无法逐项 Set 更新）。事件循环未启动（测试）时只记录，不触达原生 API。
func (a *app) rebuildTabMenus() {
	if a.ready && a.menu != nil {
		a.buildMenu()
		return
	}
	a.syncMenus()
}
func formatContextNote(id string) string {
	switch id {
	case cmdBold, cmdItalic, cmdInlineCode, cmdLink:
		return "作用于当前选区或光标位置；表格单元格内可用"
	case cmdCodeBlock, cmdRule, cmdTable:
		return "在段落上下文中使用；表格单元格内不可用"
	case cmdBody, cmdH1, cmdH2, cmdH3, cmdH4, cmdH5, cmdH6:
		return "修改当前段落或源码选区覆盖的正文行；代码块与表格内不可用"
	case cmdQuote, cmdBullet, cmdOrdered, cmdTask:
		return "处理所选行并保留文本；代码块与表格内不可用"
	}
	return ""
}

// createMenu 只组装菜单；菜单栏按 Typora 式分工：段落/格式两个顶层菜单收纳
// Markdown 命令（展示当前绑定并调用同一命令实现），编辑收系统操作与查找组，
// 显示收视图模式与标签切换。标签列表用稳定 ID 构造，设置窗口统一从应用菜单的「设置…」进入；
// 只为前 9 个标签注册数字快捷键，10–12 用顶栏或设置页访问。
func (a *app) createMenu() *mygo.Menu {
	// 标签 1..N，⌃1..⌃9（10–12 不注册数字键）。
	tabItems := make([]*mygo.MenuItem, 0, len(a.tabs))
	for i := range a.tabs {
		i := i
		item := &mygo.MenuItem{
			ID:    fmt.Sprintf("tab-%d", a.tabs[i].ID),
			Label: a.menuTitle(i),
			Click: func(*mygo.MenuItem, *mygo.Window) {
				a.withWindow(func() { a.switchTab(i) })
			},
		}
		if i < 9 {
			item.Accelerator = fmt.Sprintf("Ctrl+%d", i+1)
		}
		tabItems = append(tabItems, item)
	}

	a.miPreview = &mygo.MenuItem{
		ID:          "preview",
		Label:       "源码模式",
		Type:        mygo.MenuItemCheckbox,
		Checked:     a.cfg.Source,
		Accelerator: "Shift+CmdOrCtrl+P",
		Click: func(*mygo.MenuItem, *mygo.Window) {
			a.withWindow(func() {
				a.toggleSourceMode()
				a.syncMenus()
			})
		},
	}
	a.miFind = &mygo.MenuItem{
		ID:          "find",
		Label:       "查找",
		Accelerator: "CmdOrCtrl+F",
		Click: func(*mygo.MenuItem, *mygo.Window) {
			a.withWindow(func() { a.findOpen, a.query, a.matchIdx = true, "", 0 })
		},
	}
	a.miFindNext = &mygo.MenuItem{
		ID:          "find-next",
		Label:       "查找下一个",
		Accelerator: "CmdOrCtrl+G",
		Click: func(*mygo.MenuItem, *mygo.Window) {
			a.withWindow(func() { a.findOpenOrStep(1) })
		},
	}
	a.miFindPrev = &mygo.MenuItem{
		ID:          "find-prev",
		Label:       "查找上一个",
		Accelerator: "Shift+CmdOrCtrl+G",
		Click: func(*mygo.MenuItem, *mygo.Window) {
			a.withWindow(func() { a.findOpenOrStep(-1) })
		},
	}
	a.miTop = &mygo.MenuItem{
		ID:          "ontop",
		Label:       "窗口置顶",
		Type:        mygo.MenuItemCheckbox,
		Checked:     a.cfg.AlwaysOnTop,
		Accelerator: "Alt+CmdOrCtrl+T",
		Click: func(*mygo.MenuItem, *mygo.Window) {
			a.withWindow(func() {
				a.setAlwaysOnTop(a.miTop.IsChecked())
				a.syncMenus()
			})
		},
	}

	// 显示菜单：Typora 同样把「视图模式 + 置顶 + 打开文档切换」收在这里。
	viewItems := []*mygo.MenuItem{
		a.miPreview,
		a.miTop,
		mygo.Separator(),
		{Label: "上一个标签", Accelerator: "Alt+CmdOrCtrl+Left", Click: func(*mygo.MenuItem, *mygo.Window) {
			a.withWindow(func() { a.cycleTab(-1) })
		}},
		{Label: "下一个标签", Accelerator: "Alt+CmdOrCtrl+Right", Click: func(*mygo.MenuItem, *mygo.Window) {
			a.withWindow(func() { a.cycleTab(1) })
		}},
		{Label: "切换标签", Submenu: tabItems},
	}
	// 编辑菜单：系统编辑操作 + 查找组（查找/下一个/上一个），与 Typora 的 Edit 布局一致。
	editItems := append(a.editMenuItems(),
		mygo.Separator(),
		a.miFind,
		a.miFindNext,
		a.miFindPrev,
	)

	menu := mygo.NewMenu([]*mygo.MenuItem{
		// 自定义应用菜单时显式保留 MyGo 默认的关于、服务、隐藏与退出项；
		// 角色项默认英文标签，这里统一覆盖为中文。「服务」必须带空子菜单：
		// 否则 macOS 拿不到 servicesMenu，菜单里只剩一个点不动的死项。
		{Role: mygo.RoleAppMenu, Submenu: []*mygo.MenuItem{
			{Role: mygo.RoleAbout, Label: "关于 " + appName},
			updateMenuItem(),
			mygo.Separator(),
			{ID: "app-settings", Label: "设置…", Accelerator: "CmdOrCtrl+,",
				Click: func(*mygo.MenuItem, *mygo.Window) { a.openSettings(pageGeneral) }},
			mygo.Separator(),
			{Role: mygo.RoleServices, Label: "服务", Submenu: []*mygo.MenuItem{}},
			mygo.Separator(),
			{Role: mygo.RoleHide, Label: "隐藏 " + appName},
			{Role: mygo.RoleHideOthers, Label: "隐藏其他"},
			{Role: mygo.RoleUnhide, Label: "显示全部"},
			mygo.Separator(),
			{Role: mygo.RoleQuit, Label: "退出 " + appName},
		}},
		// 文件菜单按 macOS 惯例：新建/整理在前，保存类居中，关闭收尾。
		{Label: "文件", Submenu: []*mygo.MenuItem{
			{Label: "新建标签", Accelerator: "CmdOrCtrl+T", Click: func(*mygo.MenuItem, *mygo.Window) {
				a.withWindow(func() { a.addTab() })
			}},
			{Label: "重命名标签", Accelerator: "CmdOrCtrl+R", Click: func(*mygo.MenuItem, *mygo.Window) {
				a.withWindow(func() { a.beginRename(a.active) })
			}},
			mygo.Separator(),
			// 立即备份必须绕过节流：保存当前数据并无条件写一份备份。
			{Label: "立即备份", Accelerator: "CmdOrCtrl+S", Click: func(*mygo.MenuItem, *mygo.Window) { a.backupNow() }},
			mygo.Separator(),
			{Role: mygo.RoleClose, Label: "关闭窗口"},
		}},
		{Label: "编辑", Submenu: editItems},
		// Typora 式把 Markdown 命令拆为两个顶层菜单：段落收块级、格式收行内，
		// 不再埋在编辑下的二级子菜单里。
		{Label: "段落", Submenu: a.commandMenuItems("heading", "blocks", "insert")},
		{Label: "格式", Submenu: a.commandMenuItems("inline")},
		{Label: "显示", Submenu: viewItems},
		// 窗口菜单角色默认同样是英文标签，覆盖为中文并保留最小化/缩放/前置。
		{Role: mygo.RoleWindowMenu, Label: "窗口", Submenu: []*mygo.MenuItem{
			// 关闭窗口后驻留托盘，菜单栏也应能唤回主窗口（Typora 无托盘驻留场景）。
			{Label: "显示主窗口", Click: func(*mygo.MenuItem, *mygo.Window) { a.showWindow() }},
			mygo.Separator(),
			{Role: mygo.RoleMinimize, Label: "最小化"},
			{Role: mygo.RoleZoom, Label: "缩放"},
			mygo.Separator(),
			{Role: mygo.RoleFront, Label: "前置全部窗口"},
		}},
	})
	return menu
}

// commandMenuItems 按分组展开命令为菜单项（段落/格式顶层菜单共用），
// 组间按 keymapGroups 顺序插分隔线。
func (a *app) commandMenuItems(groups ...string) []*mygo.MenuItem {
	inSet := func(id string) bool {
		for _, g := range groups {
			if g == id {
				return true
			}
		}
		return false
	}
	var items []*mygo.MenuItem
	for _, group := range keymapGroups {
		if group.id == "global" || !inSet(group.id) {
			continue
		}
		if len(items) > 0 {
			items = append(items, mygo.Separator())
		}
		for _, def := range commandRegistry {
			if def.group != group.id {
				continue
			}
			def := def
			items = append(items, &mygo.MenuItem{
				ID:          "cmd-" + def.id,
				Label:       def.name,
				Accelerator: a.commandBinding(def.id),
				Disabled:    !a.winOpen,
				ToolTip:     formatContextNote(def.id),
				Click: func(*mygo.MenuItem, *mygo.Window) {
					a.runFormatCommand(def.id)
				},
			})
		}
	}
	return items
}

// syncMenus 让菜单的勾选/可用状态跟上状态（工具栏切换、窗口开关后调用）。
func (a *app) syncMenus() {
	if a.menu == nil {
		return
	}
	if item := a.menu.ItemByID("preview"); item != nil {
		item.SetChecked(a.cfg.Source)
	}
	if item := a.menu.ItemByID("ontop"); item != nil {
		item.SetChecked(a.cfg.AlwaysOnTop)
	}
	// Markdown 命令：主窗口打开才可用；绑定跟随 Keymap。
	for _, def := range commandRegistry {
		if item := a.menu.ItemByID("cmd-" + def.id); item != nil {
			item.Disabled = !a.winOpen
			item.SetEnabled(a.winOpen)
			item.SetAccelerator(a.commandBinding(def.id))
		}
	}
}

// setupTray 托盘图标：左键显示/隐藏，右键菜单（显示/隐藏、设置、退出）。
func (a *app) setupTray() {
	tray, err := mygo.NewTray(mygo.TrayOptions{
		Icon:           trayPNG(),
		IconIsTemplate: true,
		ToolTip:        appName + " — 左键显示/隐藏，右键菜单",
	})
	if err != nil {
		slog.Error("托盘创建失败", "error", err)
		return
	}
	tray.OnClick(func() { a.toggleWindow() })
	tray.OnRightClick(func() {
		menu := mygo.NewMenu([]*mygo.MenuItem{
			{Label: "显示 / 隐藏", Click: func(*mygo.MenuItem, *mygo.Window) { a.toggleWindow() }},
			{Label: "设置…", Click: func(*mygo.MenuItem, *mygo.Window) { a.openSettings(pageGeneral) }},
			updateMenuItem(),
			mygo.Separator(),
			{Role: mygo.RoleQuit, Label: "退出 " + appName},
		})
		tray.SetMenu(menu)
		tray.PopUpMenu()
		tray.SetMenu(nil)
	})
}
