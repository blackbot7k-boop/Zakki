package app

// 设置窗口：单实例、非模态、系统标题栏，独立记忆位置和尺寸。
// 左侧导航：常规 / 外观 / 标签与外观 / 备份与数据 / 快捷键。
// 所有字段即时生效并自动保存；每页提供「恢复本页默认值」。

import (
	"strconv"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// settingsPageID 设置页 ID。
const (
	pageGeneral    = "general"
	pageAppearance = "appearance"
	pageTabs       = "tabs"
	pageBackup     = "backup"
	pageKeymap     = "keymap"
)

// settingsUIState 设置窗口的临时输入状态（主线程）。控件绑定草稿，
// 变更时整体推入 updatePreferences；非法输入不提交并显示行内错误。
type settingsUIState struct {
	draft preferences // 控件绑定的草稿（与 prefs 同步于打开/重置时）

	// Select 控件绑定显示文案；与 draft 同步。
	appearanceModeSel string
	bodyFontSel       string
	latinFontSel      string
	monoFontSel       string
	markdownThemeSel  string
	tabAnimSpeedSel   string
	tabDotShapeSel    string

	// 标签页：展开了调色板的标签 ID（0 = 无）。
	expandedColor uint64

	// 数字字段的文本工作副本与行内错误。
	bodySizeText   string
	lineHeightText string
	bodySizeErr    string
	lineHeightErr  string

	// Keymap 页：搜索词、正在录制的命令 ID、冲突/非法提示。
	keySearch string
	recording string
	keyErr    string

	// Git 自动化 Select 绑定的显示文案。
	gitModeSel string

	resetTarget string // 待确认的「恢复本页默认值」页面
}

// syncDraftFromPrefs 把草稿重置为当前偏好（打开窗口、重置页面、重试后调用）。
func (ui *settingsUIState) syncDraftFromPrefs(p preferences) {
	ui.draft = clonePrefs(p)
	ui.appearanceModeSel = appearanceModeDisplay(p.Appearance.AppearanceMode)
	ui.markdownThemeSel = markdownThemeDisplay(p.Appearance.MarkdownTheme)
	ui.bodyFontSel = displayOf(
		bodyFontKeys, bodyFontOptions, p.Appearance.BodyFont)
	ui.latinFontSel = displayOf(
		latinFontKeys, latinFontOptions, p.Appearance.LatinFont)
	ui.monoFontSel = displayOf(
		[]string{"jbm", "sfmono", "menlo"},
		[]string{"JetBrains Mono（默认）", "SF Mono", "Menlo"}, p.Appearance.MonoFont)
	ui.bodySizeText = floatText(p.Appearance.BodySize)
	ui.lineHeightText = floatText(p.Appearance.LineHeight)
	ui.bodySizeErr, ui.lineHeightErr = "", ""
	ui.keyErr = ""
	ui.gitModeSel = gitModeDisplay(p.Backup.GitMode)
}

func floatText(v float32) string {
	if v == 0 {
		return ""
	}
	iv := int64(v * 100)
	if iv%100 == 0 {
		return strconv.Itoa(int(iv / 100))
	}
	s := strconv.Itoa(int(iv))
	out := s[:len(s)-2] + "." + s[len(s)-2:]
	for len(out) > 0 && out[len(out)-1] == '0' {
		out = out[:len(out)-1]
	}
	return out
}

// openSettings 打开（或聚焦）设置窗口；page 指定初始页。
// 重复打开只聚焦已有窗口；打开 Keymap 直接定位快捷键页。
// 事件循环未启动（测试）时只路由状态，不创建原生窗口。
func (a *app) openSettings(page string) {
	a.settingsPage = page
	if !a.ready {
		if a.settingsWin == nil {
			a.settingsUI.syncDraftFromPrefs(a.prefs)
		}
		return
	}
	if a.settingsWin != nil {
		a.settingsWin.Update(func() {})
		a.settingsWin.Restore()
		a.settingsWin.Show()
		a.settingsWin.Focus()
		return
	}
	a.settingsUI.syncDraftFromPrefs(a.prefs)
	win := mygo.NewWindow(mygo.WindowOptions{
		Title:     "设置 — " + appName,
		Width:     900,
		Height:    620,
		MinWidth:  760,
		MinHeight: 520,
		StateKey:  "settings", // 独立记忆位置和尺寸
		Content:   ui.View(a.settingsView),
	})
	a.settingsWin = win
	win.OnClosed(func() {
		a.settingsWin = nil
		a.settingsUI = settingsUIState{}
		if a.win != nil && a.winOpen {
			a.win.Update(func() {})
		}
	})
	win.Show()
	a.syncMenus()
}

// refreshSettingsWindow 偏好变更后刷新设置窗口（若开着）。
func (a *app) refreshSettingsWindow() {
	if a.settingsWin != nil {
		a.settingsWin.Update(func() {})
	}
}

// settingsView 设置窗口每帧构建。
func (a *app) settingsView(c *ui.Context) {
	dark := themeIsDark(c.Theme())
	// Tot 颜色主题用中性灰变体；其他颜色主题直接用固定调色板。
	style := a.resolveStyle(-1, dark, true)
	t := *c.Theme()
	t.Background, t.Text, t.TextMuted = style.background, style.text, style.textMuted
	t.Surface, t.Border, t.Accent = style.surface, style.border, style.accent
	t.Font, t.FontSize = bodyFontStack("pingfang", ""), 13
	c.SetTheme(&t)

	if a.settingsPage == "" {
		a.settingsPage = pageGeneral
	}
	ui.Row(c).Fill().LineHeight(1.35).Children(func() {
		if ui.Sidebar(c, &a.settingsPage, func() {
			ui.SidebarItem(c, pageGeneral, iconGear, "常规")
			ui.SidebarItem(c, pageAppearance, iconPalette, "外观")
			ui.SidebarItem(c, pageTabs, iconLayers, "标签与外观")
			ui.SidebarItem(c, pageBackup, iconDatabase, "备份与数据")
			ui.SidebarItem(c, pageKeymap, iconKeyboard, "快捷键")
		}).Width(200).FillHeight().Changed() {
			a.settingsUI.syncDraftFromPrefs(a.prefs)
			a.settingsUI.recording = ""
		}
		ui.Column(c).Fill().Grow(1).Padding(24, 20).Gap(16).Children(func() {
			switch a.settingsPage {
			case pageAppearance:
				a.appearancePage(c, &t, dark)
			case pageTabs:
				a.tabsPage(c, &t, dark)
			case pageBackup:
				a.backupPage(c, &t, dark)
			case pageKeymap:
				a.keymapPage(c, &t, dark)
			default:
				a.generalPage(c, &t, dark)
			}
		})
	})
}

// pushAppearance 把外观草稿推入偏好（校验失败回滚并提示）。
func (a *app) pushAppearance() {
	a.pushDraft(func(p *preferences) { p.Appearance = a.settingsUI.draft.Appearance })
}

// pushWindow 推入窗口偏好。
func (a *app) pushWindow() {
	a.pushDraft(func(p *preferences) { p.Window = a.settingsUI.draft.Window })
}

// pushBackup 推入备份偏好。
func (a *app) pushBackup() {
	a.pushDraft(func(p *preferences) { p.Backup = a.settingsUI.draft.Backup })
}

// pushDraft 统一推入：以草稿为基础执行 mutate，校验失败时回滚草稿对应节。
func (a *app) pushDraft(mutate func(p *preferences)) {
	err := a.updatePreferences(func(p *preferences) { mutate(p) })
	if err != nil {
		a.settingsUI.syncDraftFromPrefs(a.prefs)
		a.settingsUI.keyErr = err.Error()
	}
}

// resetPageDefaults 恢复某页默认值（需确认后调用）。
func (a *app) resetPageDefaults(page string) {
	defaults := defaultPreferences()
	_ = a.updatePreferences(func(p *preferences) {
		switch page {
		case pageGeneral:
			p.Window = defaults.Window
		case pageAppearance:
			p.Appearance = defaults.Appearance
		case pageBackup:
			p.Backup = defaults.Backup
		case pageKeymap:
			p.Keymap = defaults.Keymap
		}
	})
	a.settingsUI.syncDraftFromPrefs(a.prefs)
	a.settingsUI.recording = ""
	a.syncGlobalHotkey()
	a.refreshSettingsWindow()
}

// ---- 通用小部件 ----

// settingsRow 一行：左侧标签 + 右侧控件。
func settingsRow(c *ui.Context, label, detail string, control func()) ui.Element {
	return ui.Column(c).FillWidth().Gap(3).PaddingY(10).Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(12).Children(func() {
			ui.Text(c, label).Grow(1)
			control()
		})
		if detail != "" {
			ui.Text(c, detail).FontSize(11).TextColor(tMuted(c)).Grow(1)
		}
	})
}

// tMuted 返回当前主题的次级文字色。
func tMuted(c *ui.Context) ui.Color { return c.Theme().TextMuted }

// settingsSectionTitle 分组标题。
func settingsSectionTitle(c *ui.Context, title string) {
	ui.Text(c, title).Bold().TextColor(tMuted(c)).FontSize(12)
}

// confirmResetPage 「恢复本页默认值」：原生确认对话框，不清除笔记或备份。
func (a *app) confirmResetPage(page string) {
	go func() {
		res, err := mygo.Dialog.Message(mygo.MessageOptions{
			Type:    mygo.MessageQuestion,
			Title:   "恢复默认值",
			Message: "确定恢复本页默认设置吗？",
			Detail:  "只重置本页设置，不清除笔记内容和备份。",
			Buttons: []string{"恢复默认值", "取消"},
		})
		if err != nil || res.Button != 0 {
			return
		}
		a.withWindow(func() { a.resetPageDefaults(page) })
	}()
}

// setAlwaysOnTop 通过统一入口切换置顶。
func (a *app) setAlwaysOnTop(on bool) {
	_ = a.updatePreferences(func(p *preferences) { p.Window.AlwaysOnTop = on })
}
