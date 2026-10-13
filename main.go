package main

// 程序入口：单实例、载入数据、窗口、菜单、托盘与全局热键。

import (
	"fmt"
	"log"
	"os"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

func main() {
	// 第二个实例把参数交给第一个，然后退出。
	if !mygo.App.RequestSingleInstanceLock() {
		return
	}

	a := newApp()
	a.load()

	mygo.App.OnSecondInstance(func([]string, string) { a.showWindow() })
	// 关闭窗口后留在托盘，等全局热键或托盘再唤起。
	mygo.App.OnWindowAllClosed(func() {})
	mygo.App.OnWillQuit(func(*mygo.QuitEvent) { a.saveNow() })

	mygo.App.WhenReady(func() {
		a.loadLastBackupTime()
		a.backupOnLaunch()
		applyTheme(a.cfg.ThemeSource)
		a.buildMenu()
		a.createWindow()
		a.setupTray()
		a.registerHotkey()
	})

	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}

// applyTheme 把保存的外观设置应用到系统主题。
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

// createWindow 新建主窗口（关闭后可重建）。
func (a *app) createWindow() {
	win := mygo.NewWindow(mygo.WindowOptions{
		Title:       appName + " — 速记便签",
		Width:       400,
		Height:      540,
		MinWidth:    300,
		MinHeight:   340,
		StateKey:    "main", // 记住上次的位置和大小
		Frameless:   true, // Tot 同款：无红绿灯、无标题栏，圆角阴影保留，顶栏自绘可拖动
		AlwaysOnTop: a.cfg.AlwaysOnTop,
		Content:     ui.View(a.view),
	})
	a.win = win
	a.winOpen = true
	a.hidden = false
	win.OnShow(func() { a.hidden = false })
	win.OnHide(func() { a.hidden = true })
	win.OnClosed(func() { a.winOpen, a.win, a.hidden = false, nil, true })
}

// withWindow 在主线程改状态；窗口开着就触发重绘。
func (a *app) withWindow(fn func()) {
	fn()
	if a.win != nil && a.winOpen {
		a.win.Update(func() {})
	}
}

// buildMenu 组装 macOS 菜单栏（其他平台为窗口菜单）。
func (a *app) buildMenu() {
	// 点 1..7，⌘1..⌘7
	dotItems := make([]*mygo.MenuItem, 0, DotCount)
	for i := 0; i < DotCount; i++ {
		i := i
		dotItems = append(dotItems, &mygo.MenuItem{
			Label:       dotLabel(i),
			Accelerator: fmt.Sprintf("CmdOrCtrl+%d", i+1),
			Click: func(*mygo.MenuItem, *mygo.Window) {
				a.withWindow(func() { a.switchDot(i) })
			},
		})
	}

	a.miPreview = &mygo.MenuItem{
		ID:          "preview",
		Label:       "Markdown 预览",
		Type:        mygo.MenuItemCheckbox,
		Checked:     a.cfg.Preview,
		Accelerator: "Shift+CmdOrCtrl+P",
		Click: func(*mygo.MenuItem, *mygo.Window) {
			a.withWindow(func() {
				a.cfg.Preview = a.miPreview.Checked
				a.syncMenus()
				a.scheduleSave()
			})
		},
	}
	a.miFind = &mygo.MenuItem{
		ID:          "find",
		Label:       "搜索本点",
		Accelerator: "CmdOrCtrl+F",
		Click: func(*mygo.MenuItem, *mygo.Window) {
			a.withWindow(func() { a.findOpen, a.query, a.matchIdx = true, "", 0 })
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
				a.cfg.AlwaysOnTop = a.miTop.Checked
				a.applyAlwaysOnTop()
				a.syncMenus()
				a.scheduleSave()
			})
		},
	}
	a.miEsc = &mygo.MenuItem{
		ID:      "eschide",
		Label:   "按 Esc 隐藏窗口",
		Type:    mygo.MenuItemCheckbox,
		Checked: a.cfg.HideOnEscape,
		Click: func(*mygo.MenuItem, *mygo.Window) {
			a.cfg.HideOnEscape = a.miEsc.Checked
			a.scheduleSave()
		},
	}
	themeRadio := func(id, label, src string) *mygo.MenuItem {
		return &mygo.MenuItem{
			ID: id, Label: label, Type: mygo.MenuItemRadio,
			Checked: a.cfg.ThemeSource == src,
			Click: func(*mygo.MenuItem, *mygo.Window) {
				a.cfg.ThemeSource = src
				applyTheme(src)
				a.scheduleSave()
			},
		}
	}

	menu := mygo.NewMenu([]*mygo.MenuItem{
		{Role: mygo.RoleAppMenu},
		{Label: "文件", Submenu: []*mygo.MenuItem{
			{Label: "立即备份", Accelerator: "CmdOrCtrl+S", Click: func(*mygo.MenuItem, *mygo.Window) { a.saveNow() }},
			{Label: "在访达中显示备份", Click: func(*mygo.MenuItem, *mygo.Window) {
				_ = os.MkdirAll(backupDir(), 0o755)
				mygo.Shell.ShowItemInFolder(dataFile())
			}},
			mygo.Separator(),
			{Label: "插入圆点", Accelerator: "Shift+CmdOrCtrl+B", Click: func(*mygo.MenuItem, *mygo.Window) {
				a.withWindow(func() { a.insertBullet = true })
			}},
			{Label: "清空当前点", Accelerator: "CmdOrCtrl+K", Click: func(*mygo.MenuItem, *mygo.Window) {
				a.withWindow(func() { a.dots[a.active] = ""; a.onEdited() })
			}},
			{Label: "复制整个点", Accelerator: "Shift+CmdOrCtrl+C", Click: func(*mygo.MenuItem, *mygo.Window) {
				mygo.Clipboard.WriteText(a.dots[a.active])
			}},
			mygo.Separator(),
			{Role: mygo.RoleClose},
		}},
		{Role: mygo.RoleEditMenu},
		{Label: "显示", Submenu: append(dotItems,
			mygo.Separator(),
			a.miPreview,
			a.miFind,
			a.miTop,
			a.miEsc,
			mygo.Separator(),
			themeRadio("theme-system", "跟随系统", "system"),
			themeRadio("theme-light", "浅色", "light"),
			themeRadio("theme-dark", "深色", "dark"),
		)},
		{Role: mygo.RoleWindowMenu},
	})
	mygo.App.SetMenu(menu)
	a.menu = menu
}

// syncMenus 让菜单的勾选状态跟上设置（工具栏切换后调用）。
func (a *app) syncMenus() {
	if a.menu == nil {
		return
	}
	if item := a.menu.ItemByID("preview"); item != nil {
		item.SetChecked(a.cfg.Preview)
	}
	if item := a.menu.ItemByID("ontop"); item != nil {
		item.SetChecked(a.cfg.AlwaysOnTop)
	}
	if item := a.menu.ItemByID("eschide"); item != nil {
		item.SetChecked(a.cfg.HideOnEscape)
	}
	for id, src := range map[string]string{
		"theme-system": "system", "theme-light": "light", "theme-dark": "dark",
	} {
		if item := a.menu.ItemByID(id); item != nil {
			item.SetChecked(a.cfg.ThemeSource == src)
		}
	}
}

// setupTray 托盘图标：左键显示/隐藏，右键菜单。
func (a *app) setupTray() {
	tray, err := mygo.NewTray(mygo.TrayOptions{
		Icon:           trayPNG(),
		IconIsTemplate: false,
		ToolTip:        appName + " — 左键显示/隐藏，右键菜单",
	})
	if err != nil {
		log.Println("托盘创建失败:", err)
		return
	}
	tray.OnClick(func() { a.toggleWindow() })
	tray.OnRightClick(func() {
		menu := mygo.NewMenu([]*mygo.MenuItem{
			{Label: "显示 / 隐藏", Click: func(*mygo.MenuItem, *mygo.Window) { a.toggleWindow() }},
			{Label: "立即备份", Click: func(*mygo.MenuItem, *mygo.Window) { a.saveNow() }},
			mygo.Separator(),
			{Role: mygo.RoleQuit},
		})
		tray.SetMenu(menu)
		tray.PopUpMenu()
		tray.SetMenu(nil)
	})
}

// registerHotkey 全局热键：⌘⇧空格 呼出/隐藏（后台也有效）。
func (a *app) registerHotkey() {
	if err := mygo.GlobalShortcut.Register("CmdOrCtrl+Shift+Space", func() { a.toggleWindow() }); err != nil {
		log.Println("全局热键注册失败（可能被其他应用占用）:", err)
	}
}
