package app

// 设置窗口：单实例、非模态、系统标题栏，独立记忆位置和尺寸。
// 左侧导航：常规 / 外观 / 标签与外观 / 备份与数据 / 快捷键。
// 所有字段即时生效并自动保存；每页提供「恢复本页默认值」。

import (
	"strconv"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"zakki/locales"
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
// synced 记录最近一次已应用偏好；提交时只写入相对 synced 发生变化的字段。
type settingsUIState struct {
	draft      preferences // 控件绑定的草稿
	synced     preferences // 最近一次合并进草稿的已应用偏好
	draftReady bool        // 尚未打开设置窗口时不维护草稿

	// Select 控件绑定显示文案；与 draft 同步。
	appearanceModeSel string
	languageSel       string
	bodyFontSel       string
	latinFontSel      string
	monoFontSel       string
	markdownThemeSel  string
	codeThemeSel      string
	tabAnimSpeedSel   string
	tabDotShapeSel    string

	// 标签页：展开了调色板的标签 ID（0 = 无）。
	expandedColor uint64

	// 数字字段的文本工作副本、行内错误与文本级 dirty 状态。
	bodySizeText    string
	lineHeightText  string
	bodySizeErr     string
	lineHeightErr   string
	bodySizeDirty   bool
	lineHeightDirty bool

	// Keymap 页：搜索词、正在录制的命令 ID、冲突/非法提示。
	keySearch string
	recording string
	keyErr    string

	// Git 自动化 Select 绑定的显示文案。
	gitModeSel string

	resetTarget string // 待确认的「恢复本页默认值」页面
}

// syncDraftFromPrefs 明确丢弃草稿并重置为当前偏好（打开窗口、切页、重置时调用）。
func (ui *settingsUIState) syncDraftFromPrefs(p preferences) {
	ui.draft = clonePrefs(p)
	ui.synced = clonePrefs(p)
	ui.draftReady = true
	ui.bodySizeDirty, ui.lineHeightDirty = false, false
	ui.syncDraftBindings()
	ui.bodySizeErr, ui.lineHeightErr = "", ""
	ui.keyErr = ""
}

// mergeDraftFromPrefs 把外部更新合并进未编辑字段；已编辑字段继续保留在非模态窗口中。
func (ui *settingsUIState) mergeDraftFromPrefs(p preferences) {
	if !ui.draftReady {
		return
	}
	old := ui.synced
	next := clonePrefs(p)

	mergeString := func(draft *string, before, current string) {
		if *draft == before {
			*draft = current
		}
	}
	mergeFloat := func(draft *float32, before, current float32, dirty bool) {
		if !dirty && *draft == before {
			*draft = current
		}
	}
	mergeBool := func(draft *bool, before, current bool) {
		if *draft == before {
			*draft = current
		}
	}
	mergeInt := func(draft *int, before, current int) {
		if *draft == before {
			*draft = current
		}
	}

	d, before, current := &ui.draft.Appearance, old.Appearance, next.Appearance
	mergeString(&d.AppearanceMode, before.AppearanceMode, current.AppearanceMode)
	mergeString(&d.ColorTheme, before.ColorTheme, current.ColorTheme)
	mergeString(&d.MarkdownTheme, before.MarkdownTheme, current.MarkdownTheme)
	mergeString(&d.Language, before.Language, current.Language)
	mergeString(&d.BodyFont, before.BodyFont, current.BodyFont)
	mergeString(&d.LatinFont, before.LatinFont, current.LatinFont)
	mergeString(&d.MonoFont, before.MonoFont, current.MonoFont)
	mergeFloat(&d.BodySize, before.BodySize, current.BodySize, ui.bodySizeDirty)
	mergeFloat(&d.LineHeight, before.LineHeight, current.LineHeight, ui.lineHeightDirty)

	windowDraft, windowBefore, windowCurrent := &ui.draft.Window, old.Window, next.Window
	mergeBool(&windowDraft.AlwaysOnTop, windowBefore.AlwaysOnTop, windowCurrent.AlwaysOnTop)
	mergeBool(&windowDraft.HideOnEscape, windowBefore.HideOnEscape, windowCurrent.HideOnEscape)
	mergeBool(&windowDraft.RememberBounds, windowBefore.RememberBounds, windowCurrent.RememberBounds)

	backupDraft, backupBefore, backupCurrent := &ui.draft.Backup, old.Backup, next.Backup
	mergeInt(&backupDraft.IntervalMinutes, backupBefore.IntervalMinutes, backupCurrent.IntervalMinutes)
	mergeInt(&backupDraft.Keep, backupBefore.Keep, backupCurrent.Keep)
	mergeBool(&backupDraft.OnLaunch, backupBefore.OnLaunch, backupCurrent.OnLaunch)
	mergeString(&backupDraft.MirrorDir, backupBefore.MirrorDir, backupCurrent.MirrorDir)
	mergeString(&backupDraft.GitMode, backupBefore.GitMode, backupCurrent.GitMode)

	ui.draft.Version = next.Version
	ui.draft.Keymap = next.Keymap
	ui.draft.Advanced = next.Advanced
	ui.synced = next
	ui.syncDraftBindings()
}

func (ui *settingsUIState) syncDraftBindings() {
	p := ui.draft
	ui.appearanceModeSel = appearanceModeDisplay(p.Appearance.AppearanceMode)
	ui.languageSel = languageDisplay(p.Appearance.Language)
	ui.markdownThemeSel = markdownThemeDisplay(p.Appearance.MarkdownTheme)
	ui.codeThemeSel = codeThemeDisplay(p.Appearance.CodeTheme)
	ui.bodyFontSel = displayOf(
		bodyFontKeys, bodyFontOptions, p.Appearance.BodyFont)
	ui.latinFontSel = displayOf(
		latinFontKeys, latinFontOptions, p.Appearance.LatinFont)
	ui.monoFontSel = displayOf(
		[]string{"jbm", "sfmono", "menlo"},
		[]string{locales.FontJetBrainsDefault(), locales.FontSFMono(), locales.FontMenlo()}, p.Appearance.MonoFont)
	if !ui.bodySizeDirty {
		ui.bodySizeText = floatText(p.Appearance.BodySize)
		ui.bodySizeErr = ""
	}
	if !ui.lineHeightDirty {
		ui.lineHeightText = floatText(p.Appearance.LineHeight)
		ui.lineHeightErr = ""
	}
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
		Title:     locales.SettingsTitle(appName),
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
			ui.SidebarItem(c, pageGeneral, iconGear, locales.PageGeneral())
			ui.SidebarItem(c, pageAppearance, iconPalette, locales.PageAppearance())
			ui.SidebarItem(c, pageTabs, iconLayers, locales.PageTabs())
			ui.SidebarItem(c, pageBackup, iconDatabase, locales.PageBackup())
			ui.SidebarItem(c, pageKeymap, iconKeyboard, locales.PageKeymap())
		}).Width(200).FillHeight().Changed() {
			a.settingsUI.syncDraftFromPrefs(a.prefs)
			a.settingsUI.recording = ""
		}
		ui.Column(c).Fill().Grow(1).Padding(28, 24).Gap(18).Children(func() {
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

// pushAppearance 只推入用户实际编辑的外观字段。
func (a *app) pushAppearance() {
	a.pushDraft(a.settingsUI.applyAppearanceDraft)
}

func (ui *settingsUIState) applyAppearanceDraft(p *preferences) {
	draft, before := ui.draft.Appearance, ui.synced.Appearance
	if draft.AppearanceMode != before.AppearanceMode {
		p.Appearance.AppearanceMode = draft.AppearanceMode
	}
	if draft.ColorTheme != before.ColorTheme {
		p.Appearance.ColorTheme = draft.ColorTheme
	}
	if draft.MarkdownTheme != before.MarkdownTheme {
		p.Appearance.MarkdownTheme = draft.MarkdownTheme
	}
	if draft.CodeTheme != before.CodeTheme {
		p.Appearance.CodeTheme = draft.CodeTheme
	}
	if draft.Language != before.Language {
		p.Appearance.Language = draft.Language
	}
	if draft.BodyFont != before.BodyFont {
		p.Appearance.BodyFont = draft.BodyFont
	}
	if draft.LatinFont != before.LatinFont {
		p.Appearance.LatinFont = draft.LatinFont
	}
	if draft.MonoFont != before.MonoFont {
		p.Appearance.MonoFont = draft.MonoFont
	}
	if !ui.bodySizeDirty && draft.BodySize != before.BodySize {
		p.Appearance.BodySize = draft.BodySize
	}
	if !ui.lineHeightDirty && draft.LineHeight != before.LineHeight {
		p.Appearance.LineHeight = draft.LineHeight
	}
}

// pushWindow 只推入用户实际编辑的窗口字段。
func (a *app) pushWindow() {
	a.pushDraft(a.settingsUI.applyWindowDraft)
}

func (ui *settingsUIState) applyWindowDraft(p *preferences) {
	draft, before := ui.draft.Window, ui.synced.Window
	if draft.AlwaysOnTop != before.AlwaysOnTop {
		p.Window.AlwaysOnTop = draft.AlwaysOnTop
	}
	if draft.HideOnEscape != before.HideOnEscape {
		p.Window.HideOnEscape = draft.HideOnEscape
	}
	if draft.RememberBounds != before.RememberBounds {
		p.Window.RememberBounds = draft.RememberBounds
	}
}

// pushBackup 只推入用户实际编辑的备份字段。
func (a *app) pushBackup() {
	a.pushDraft(a.settingsUI.applyBackupDraft)
}

func (ui *settingsUIState) applyBackupDraft(p *preferences) {
	draft, before := ui.draft.Backup, ui.synced.Backup
	if draft.IntervalMinutes != before.IntervalMinutes {
		p.Backup.IntervalMinutes = draft.IntervalMinutes
	}
	if draft.Keep != before.Keep {
		p.Backup.Keep = draft.Keep
	}
	if draft.OnLaunch != before.OnLaunch {
		p.Backup.OnLaunch = draft.OnLaunch
	}
	if draft.MirrorDir != before.MirrorDir {
		p.Backup.MirrorDir = draft.MirrorDir
	}
	if draft.GitMode != before.GitMode {
		p.Backup.GitMode = draft.GitMode
	}
}

// pushDraft 统一推入；校验失败时保留草稿和行内错误供用户修正。
func (a *app) pushDraft(mutate func(p *preferences)) {
	if err := a.updatePreferences(mutate); err != nil {
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
	return ui.Column(c).FillWidth().Gap(3).Padding(12, 16).Children(func() {
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
			Title:   locales.DialogResetPageTitle(),
			Message: locales.DialogResetPageMessage(),
			Detail:  locales.DialogResetPageDetail(),
			Buttons: []string{locales.DialogResetPageConfirm(), locales.CommonCancel()},
		})
		if err != nil || res.Button != 0 {
			return
		}
		a.resetPageAfterConfirmation(page)
	}()
}

// resetPageAfterConfirmation 把原生对话框结果投递回存活的主窗口。
func (a *app) resetPageAfterConfirmation(page string) {
	a.runOnMain(func() { a.resetPageDefaults(page) })
}

// setAlwaysOnTop 通过统一入口切换置顶。
func (a *app) setAlwaysOnTop(on bool) {
	_ = a.updatePreferences(func(p *preferences) { p.Window.AlwaysOnTop = on })
}
