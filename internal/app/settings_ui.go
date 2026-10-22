package app

// 设置页 UI：通用页的外观分区与各分组小部件。通用页骨架与常规分区在
// settings_general.go，备份与数据页在 settings_backup.go。所有字段即时应用并自动保存；
// 「恢复本页默认值」在卡片底部，需确认，不清除笔记或备份。

import (
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"

	"zakki/locales"
)

// ---- 外观 ----

// appearanceSections 通用页的外观分区：主题、字体与排版、代码、顶栏，
// 分区以浅色分割线分隔。
func (a *app) appearanceSections(c *ui.Context) {
	settingsSectionTitle(c, locales.CardTheme())
	a.appearanceModeSelect(c)
	a.styleRow(c)
	a.colorThemeSelect(c)
	sectionDivider(c)
	settingsSectionTitle(c, locales.CardTypography())
	a.markdownThemeSelect(c)
	a.typographyOverrides(c)
	sectionDivider(c)
	settingsSectionTitle(c, locales.CardCodeTheme())
	a.codeThemeSelect(c)
	a.codeIndentSelect(c)
	sectionDivider(c)
	settingsSectionTitle(c, locales.CardTabBar())
	a.tabAppearanceSection(c)
	sectionDivider(c)
}

// styleRow 整体风格：标准 / 手账。只切风格，颜色与排版的标准选择原样保留，
// 切回标准即恢复；字体不跟着换，手账下未用手写搭配时给一键改用（中文手写、
// 英文跟随中文，英文字母也是手写体）。
func (a *app) styleRow(c *ui.Context) {
	ap := &a.settingsUI.draft.Appearance
	a.settingsUI.styleIdx = 0
	if ap.Style == styleSketch {
		a.settingsUI.styleIdx = 1
	}
	settingsRow(c, locales.StyleLabel(), "", func() {
		if ui.Segmented(c.Key("style"), &a.settingsUI.styleIdx, locales.StyleStandard(), locales.StyleSketch()).
			Label(locales.StyleLabel()).Changed() {
			ap.Style = []string{styleStandard, styleSketch}[a.settingsUI.styleIdx]
			a.pushAppearance()
		}
	})
	if ap.Style == styleSketch && (ap.BodyFont != "hand" || ap.LatinFont != "") {
		ui.Row(c).FillWidth().Padding(0, 0, 10, 0).Children(func() {
			ui.Box(c).Grow(1)
			if ui.Button(c.Key("style-hand-font"), locales.UseHandFont()).Clicked() {
				a.applySettingsFont("latinFont", "")
				a.applySettingsFont("bodyFont", "hand")
			}
		})
	}
}

// lockedSelect 手账风格下由风格决定的下拉：只显示生效值，不可选。
func lockedSelect(c *ui.Context, label, value string) {
	ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(12).PaddingY(14).Children(func() {
		ui.Column(c).Grow(1).Gap(0).Children(func() {
			ui.Text(c, label)
			ui.Text(c, locales.StyleLocked()).FontSize(11).TextColor(tMuted(c))
		})
		shown := value
		boundedSelect(c, &shown, []string{value}).Width(200).Disabled(true).Label(label)
	})
}

// appearanceModeSelect 界面明暗模式：跟随系统 / 浅色 / 深色。
func (a *app) appearanceModeSelect(c *ui.Context) {
	ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(12).PaddingY(14).Children(func() {
		ui.Text(c, locales.AppearanceLabel()).Grow(1)
		if boundedSelect(c, &a.settingsUI.appearanceModeSel, []string{locales.ModeFollowSystem(), locales.ModeLight(), locales.ModeDark()}).Width(200).Changed() {
			a.settingsUI.draft.Appearance.AppearanceMode = appearanceModeKey(a.settingsUI.appearanceModeSel)
			a.pushAppearance()
		}
	})
}

// colorThemeSelect 颜色主题下拉，左侧迷你样随当前明暗预览所选主题。
func (a *app) colorThemeSelect(c *ui.Context) {
	if a.settingsUI.draft.Appearance.Style == styleSketch {
		lockedSelect(c, locales.ColorThemeLabel(), colorThemeDisplayName("sketch"))
		return
	}
	options := colorThemeNames()
	pal := colorPaletteFor(a.settingsUI.draft.Appearance.ColorTheme, -1, themeIsDark(c.Theme()))
	ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(12).PaddingY(14).Children(func() {
		ui.Text(c, locales.ColorThemeLabel()).Grow(1)
		ui.Box(c).Width(44).Shrink(0).Children(func() { miniMock(c, pal) })
		if boundedSelect(c, &a.settingsUI.colorThemeSel, options).Width(200).Changed() {
			a.settingsUI.draft.Appearance.ColorTheme = colorThemeIDOfDisplay(a.settingsUI.colorThemeSel)
			a.pushAppearance()
		}
	})
}

// colorThemeIDOfDisplay 显示名 → 主题 ID；未知回退经典。
func colorThemeIDOfDisplay(display string) string {
	if display == colorThemeDisplayName(colorThemeClassic) {
		return colorThemeClassic
	}
	for _, def := range colorThemes {
		if colorThemeDisplayName(def.id) == display {
			return def.id
		}
	}
	return colorThemeClassic
}

func appearanceModeKey(display string) string {
	switch display {
	case locales.ModeLight():
		return "light"
	case locales.ModeDark():
		return "dark"
	}
	return "system"
}

func appearanceModeDisplay(key string) string {
	switch key {
	case "light":
		return locales.ModeLight()
	case "dark":
		return locales.ModeDark()
	}
	return locales.ModeFollowSystem()
}

// colorThemeCards 颜色主题选择卡片：名称 + 迷你界面样（顶栏 + 文字/强调色线条），
// 三列网格，选中态描边。
func (a *app) colorThemeCards(c *ui.Context, dark bool) {
	type card struct {
		id, name string
		pal      colorPalette
	}
	cards := []card{{id: colorThemeClassic, name: locales.ThemeClassic(), pal: classicNeutralPalette(dark)}}
	for _, def := range colorThemes {
		if def.id == styleSketch {
			continue
		}
		pal := def.light
		if dark {
			pal = def.dark
		}
		cards = append(cards, card{id: def.id, name: colorThemeDisplayName(def.id), pal: pal})
	}
	ui.Grid(c).Columns(3).Gap(8).FillWidth().Children(func() {
		for _, cd := range cards {
			cd := cd
			selected := a.settingsUI.draft.Appearance.ColorTheme == cd.id
			btn := ui.ButtonBase(c.Key("color-"+cd.id)).Label(locales.ColorThemeOf(cd.name)).
				Radius(8).Cursor(ui.CursorPointer).
				Padding(6, 6).BorderWidth(1.5).BorderColor(cardBorder(c, selected))
			if selected {
				btn.Background(c.Theme().Accent.Mix(c.Theme().Background, 0.92))
			}
			btn.Children(func() {
				ui.Column(c).Gap(5).Children(func() {
					miniMock(c, cd.pal)
					ui.Row(c).FillWidth().Justify(ui.Center).Children(func() {
						ui.Text(c, cd.name).FontSize(12)
					})
				})
			})
			if btn.Clicked() {
				a.settingsUI.draft.Appearance.ColorTheme = cd.id
				a.pushAppearance()
			}
		}
	})
}

// miniMock 迷你界面样：header 色条 + 背景上的文字/次级/强调三条线。
func miniMock(c *ui.Context, pal colorPalette) {
	ui.Column(c).FillWidth().Radius(5).Clip().BorderWidth(1).BorderColor(pal.border).Children(func() {
		ui.Box(c).FillWidth().Height(8).Background(pal.header)
		ui.Column(c).FillWidth().Padding(5, 6).Gap(3).Background(pal.background).Children(func() {
			ui.Box(c).Width(30).Height(3).Radius(1.5).Background(pal.text)
			ui.Box(c).Width(20).Height(3).Radius(1.5).Background(pal.textMuted)
			ui.Box(c).Width(25).Height(3).Radius(1.5).Background(pal.accent)
		})
	})
}

func cardBorder(c *ui.Context, selected bool) ui.Color {
	if selected {
		return c.Theme().Accent
	}
	return c.Theme().Border
}

// markdownThemeSelect 排版主题下拉：字体已解耦，排版只管间距字号；
// 下方一行说明随选中项变化。
func (a *app) markdownThemeSelect(c *ui.Context) {
	if a.settingsUI.draft.Appearance.Style == styleSketch {
		lockedSelect(c, locales.LayoutThemeLabel(), markdownLayoutDisplayName("sketch"))
		ui.Text(c, markdownLayoutNote("sketch")).FontSize(11).TextColor(tMuted(c)).Padding(2, 0, 6, 0)
		return
	}
	options := make([]string, 0, len(markdownLayouts))
	for _, l := range markdownLayouts {
		if l.id != styleSketch {
			options = append(options, markdownLayoutDisplayName(l.id))
		}
	}
	ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(12).PaddingY(14).Children(func() {
		ui.Text(c, locales.LayoutThemeLabel()).Grow(1)
		if boundedSelect(c, &a.settingsUI.markdownThemeSel, options).Width(200).Changed() {
			a.settingsUI.draft.Appearance.MarkdownTheme = markdownThemeKey(a.settingsUI.markdownThemeSel)
			a.pushAppearance()
		}
	})
	ui.Text(c, markdownLayoutNote(a.settingsUI.draft.Appearance.MarkdownTheme)).
		FontSize(11).TextColor(tMuted(c)).Padding(2, 0, 6, 0)
}

// codeThemeSelect 代码配色下拉：空串 = 跟随文本（不上色）。
func (a *app) codeThemeSelect(c *ui.Context) {
	options := []string{locales.CodeThemeNone(), locales.CodeThemeSoft(), locales.CodeThemeVivid(), locales.CodeThemeClaude(), locales.CodeThemeQoder()}
	ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(12).PaddingY(14).Children(func() {
		ui.Text(c, locales.CodeThemeLabel()).Grow(1)
		if boundedSelect(c, &a.settingsUI.codeThemeSel, options).Width(200).Changed() {
			a.settingsUI.draft.Appearance.CodeTheme = codeThemeKey(a.settingsUI.codeThemeSel)
			a.pushAppearance()
		}
	})
}

// codeIndentSelect 代码块缩进下拉：Tab 插入 2 或 4 个空格。
func (a *app) codeIndentSelect(c *ui.Context) {
	options := []string{locales.CodeIndent2(), locales.CodeIndent4()}
	ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(12).PaddingY(14).Children(func() {
		ui.Text(c, locales.CodeIndentLabel()).Grow(1)
		if boundedSelect(c, &a.settingsUI.codeIndentSel, options).Width(200).Changed() {
			a.settingsUI.draft.Appearance.CodeIndent = codeIndentKey(a.settingsUI.codeIndentSel)
			a.pushAppearance()
		}
	})
}

// codeIndentKey 显示名 → 缩进档位 id。
func codeIndentKey(display string) string {
	if display == locales.CodeIndent2() {
		return "space2"
	}
	return "space4"
}

// codeIndentDisplay 缩进档位 id → 显示名。
func codeIndentDisplay(key string) string {
	if key == "space2" {
		return locales.CodeIndent2()
	}
	return locales.CodeIndent4()
}

// codeThemeKey 显示名 → 配色方案 id。
func codeThemeKey(display string) string {
	switch display {
	case locales.CodeThemeSoft():
		return "soft"
	case locales.CodeThemeVivid():
		return "vivid"
	case locales.CodeThemeClaude():
		return "claude"
	case locales.CodeThemeQoder():
		return "qoder"
	}
	return ""
}

// codeThemeDisplay 配色方案 id → 显示名。
func codeThemeDisplay(key string) string {
	switch key {
	case "soft":
		return locales.CodeThemeSoft()
	case "vivid":
		return locales.CodeThemeVivid()
	case "claude":
		return locales.CodeThemeClaude()
	case "qoder":
		return locales.CodeThemeQoder()
	}
	return locales.CodeThemeNone()
}

// markdownThemeDisplay 排版主题键 → 显示名。
func markdownThemeDisplay(key string) string {
	for _, l := range markdownLayouts {
		if l.id == key {
			return markdownLayoutDisplayName(l.id)
		}
	}
	return markdownLayoutDisplayName(markdownLayouts[0].id)
}

// markdownThemeKey 显示名 → 排版主题键。
func markdownThemeKey(display string) string {
	for _, l := range markdownLayouts {
		if markdownLayoutDisplayName(l.id) == display {
			return l.id
		}
	}
	return markdownLayouts[0].id
}

// markdownLayoutNote 选中排版的一行摘要。
func markdownLayoutNote(key string) string {
	l := markdownLayoutByID(key)
	return locales.LayoutThemeOf(markdownLayoutDisplayName(l.id), layoutFontNote(l.id), float64(l.bodySize), float64(l.lineHeight))
}

// layoutFontNote 保留排版名字里的字体意象（仅作说明文字，不影响实际字体）。
func layoutFontNote(id string) string {
	switch id {
	case "book":
		return locales.LayoutSerifNote()
	case "compact":
		return locales.LayoutTightNote()
	case "note":
		return locales.LayoutNoteNote()
	case "clear":
		return locales.LayoutClearNote()
	case "sketch":
		return locales.LayoutSketchNote()
	}
	return locales.LayoutSonnetNote()
}

// bodyFontKeys / bodyFontOptions 中文字体键与显示名（同序）。
var (
	bodyFontKeys    = []string{"kai", "pingfang", "songti", "hand", "hannotate"}
	bodyFontOptions = []string{locales.BodyFontKaiDefault(), locales.BodyFontPingFang(), locales.BodyFontSongti(), locales.BodyFontHand(), locales.BodyFontHannotate()}
)

// latinFontKeys / latinFontOptions 西文字体键与显示名（同序）；空串 = 跟随中文字体。
var (
	latinFontKeys    = []string{"", "jetbrains", "system", "noteworthy", "bradley"}
	latinFontOptions = []string{locales.FontFollowChinese(), locales.FontJetBrains(), locales.FontSystemWestern(), locales.FontNoteworthy(), locales.FontBradley()}
)

// typographyOverrides 排版覆盖：中西文/代码字体合并本机字体与按需准备。
func (a *app) typographyOverrides(c *ui.Context) {
	ui.Column(c).FillWidth().Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(12).PaddingY(14).Children(func() {
			ui.Column(c).Grow(1).Gap(0).Children(func() {
				ui.Text(c, locales.FontChinese())
				ui.Text(c, locales.FontChineseHint()).FontSize(11).TextColor(tMuted(c))
			})
			if a.fontPicker(c.Key("st-bodyFont"), &a.settingsUI.bodyFontKey, 0, locales.FontChinese(), 200) {
				a.applySettingsFont("bodyFont", a.settingsUI.bodyFontKey)
			}
		})
		ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(12).PaddingY(14).Children(func() {
			ui.Column(c).Grow(1).Gap(0).Children(func() {
				ui.Text(c, locales.FontWestern())
				ui.Text(c, locales.FontWesternHint()).FontSize(11).TextColor(tMuted(c))
			})
			if a.fontPicker(c.Key("st-latinFont"), &a.settingsUI.latinFontKey, 1, locales.FontWestern(), 200) {
				a.applySettingsFont("latinFont", a.settingsUI.latinFontKey)
			}
		})
		ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(12).PaddingY(14).Children(func() {
			ui.Column(c).Grow(1).Gap(0).Children(func() {
				ui.Text(c, locales.FontCode())
				ui.Text(c, locales.FontCodeHint()).FontSize(11).TextColor(tMuted(c))
			})
			if a.fontPicker(c.Key("st-monoFont"), &a.settingsUI.monoFontKey, 2, locales.FontCode(), 200) {
				a.applySettingsFont("monoFont", a.settingsUI.monoFontKey)
			}
		})
		// 所选字体缺失时的统一准备进度：可取消；失败保留旧偏好。
		prep := &a.settingsUI.fontPreparation
		if !prep.running {
			for _, job := range a.fontJobs([]string{
				a.prefs.Appearance.BodyFont, a.prefs.Appearance.LatinFont, a.prefs.Appearance.MonoFont,
			}) {
				ui.Text(c, job.resource.Family+" · "+locales.FontNeedsDownload()).
					FontSize(11).TextColor(tMuted(c))
			}
		}
		if prep.running || prep.err != "" {
			ui.Column(c).FillWidth().Gap(6).Padding(6, 0, 10, 0).Children(func() {
				fontProgressView(c, prep)
				if prep.running {
					ui.Row(c).FillWidth().Children(func() {
						ui.Box(c).Grow(1)
						if ui.Button(c, locales.FontCancel()).Clicked() {
							prep.stop()
							a.settingsUI.syncDraftBindings()
						}
					})
				}
			})
		}
		// 字号（10–40）/ 行高（1–3）/ 内边距（8–120）：非法数字不提交，保留输入并显示行内错误。
		a.floatOverrideRow(c, locales.FontSizeRow(), locales.OverrideFollowHint(), "bodySize")
		a.floatOverrideRow(c, locales.LineHeightRow(), locales.OverrideLineHint(), "lineHeight")
		a.floatOverrideRow(c, locales.PaddingXRow(), locales.OverridePadHint(), "padX")
		a.floatOverrideRow(c, locales.PaddingYRow(), locales.OverridePadHint(), "padY")
	})
}

// floatFieldState 返回数字覆盖字段的绑定目标：文本工作副本、行内错误、
// 文本级 dirty 与草稿目标；未知字段返回 nil，调用方直接跳过。
func (ui *settingsUIState) floatFieldState(field string) (text, errField *string, dirty *bool, target *float32) {
	switch field {
	case "bodySize":
		return &ui.bodySizeText, &ui.bodySizeErr, &ui.bodySizeDirty, &ui.draft.Appearance.BodySize
	case "lineHeight":
		return &ui.lineHeightText, &ui.lineHeightErr, &ui.lineHeightDirty, &ui.draft.Appearance.LineHeight
	case "padX":
		return &ui.padXText, &ui.padXErr, &ui.padXDirty, &ui.draft.Appearance.PadX
	case "padY":
		return &ui.padYText, &ui.padYErr, &ui.padYDirty, &ui.draft.Appearance.PadY
	}
	return nil, nil, nil, nil
}

// floatOverrideRow 数字覆盖输入行；占位符显示当前排版主题的默认值。
func (a *app) floatOverrideRow(c *ui.Context, label, detail, field string) {
	layout := markdownLayoutByID(a.settingsUI.draft.Appearance.EffectiveMarkdownTheme())
	text, errField, _, _ := a.settingsUI.floatFieldState(field)
	if text == nil {
		return
	}
	var placeholder string
	switch field {
	case "lineHeight":
		placeholder = floatText(layout.lineHeight)
	case "padX":
		placeholder = floatText(layout.padX)
	case "padY":
		placeholder = floatText(layout.padY)
	default:
		placeholder = floatText(layout.bodySize)
	}
	ui.Column(c).FillWidth().Gap(2).PaddingY(14).Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(12).Children(func() {
			ui.Column(c).Grow(1).Gap(0).Children(func() {
				ui.Text(c, label)
				ui.Text(c, detail).FontSize(11).TextColor(tMuted(c))
			})
			ui.Column(c).Width(200).Gap(2).Children(func() {
				if ui.TextInput(c, text).Placeholder(placeholder).Changed() {
					a.commitFloatField(field)
				}
			})
		})
		if *errField != "" {
			ui.Text(c, *errField).FontSize(11).TextColor(c.Theme().Danger)
		}
	})
}

// commitFloatField 解析字号/行高/内边距输入：合法才提交，非法文本和错误跨帧保留。
func (a *app) commitFloatField(field string) {
	text, errField, dirty, target := a.settingsUI.floatFieldState(field)
	if text == nil {
		return
	}

	*dirty = true
	*errField = ""
	trimmed := strings.TrimSpace(*text)
	var value float32
	if trimmed != "" {
		v, err := strconv.ParseFloat(trimmed, 32)
		if err != nil {
			*errField = locales.ErrNotNumber()
			return
		}
		value = float32(v)
	}
	*target = value

	err := a.updatePreferences(func(p *preferences) {
		switch field {
		case "bodySize":
			p.Appearance.BodySize = value
		case "lineHeight":
			p.Appearance.LineHeight = value
		case "padX":
			p.Appearance.PadX = value
		case "padY":
			p.Appearance.PadY = value
		}
	})
	if err != nil {
		*errField = err.Error()
		return
	}
	*dirty = false
	*errField = ""
	a.settingsUI.syncDraftBindings()
}

func displayOf(keys, options []string, key string) string {
	for i, k := range keys {
		if k == key {
			return options[i]
		}
	}
	return options[0]
}

func keyOfDisplay(keys, options []string, display string) string {
	for i, o := range options {
		if o == display {
			return keys[i]
		}
	}
	return keys[0]
}

// ---- 卡片分组小部件 ----

// cardResetFooter 卡片底部的「恢复本页默认值」行（右置，需确认）。
func (a *app) cardResetFooter(c *ui.Context, key, page string) {
	ui.Row(c).FillWidth().PaddingY(12).AlignItems(ui.Center).Children(func() {
		ui.Box(c).Grow(1)
		// 弱化为次要小按钮：恢复是破坏性较低的高频入口，但不应与大控件争抢视觉。
		btn := ui.ButtonBase(c.Key("reset-"+key)).Padding(4, 10).Radius(7).
			Cursor(ui.CursorPointer).FocusRing(false)
		if btn.Hovered() {
			btn.Background(c.Theme().SurfaceHover)
		}
		btn.Children(func() {
			ui.Text(c, locales.ResetPageButton()).FontSize(12).TextColor(tMuted(c))
		})
		btn.OnClick(func() {
			a.confirmResetPage(page)
		})
	})
}

// tabAnimSpeedOptions 动画速度选择项（当前语言文案与倍率，顺序固定）。
func tabAnimSpeedOptions() []struct {
	display string
	speed   float32
} {
	return []struct {
		display string
		speed   float32
	}{
		{locales.AnimSlow(), 0.5},
		{locales.AnimNormal(), 1},
		{locales.AnimFast(), 1.5},
		{locales.AnimFaster(), 2},
	}
}

func tabAnimSpeedDisplay(speed float32) string {
	if speed <= 0 {
		speed = 1
	}
	for _, o := range tabAnimSpeedOptions() {
		if o.speed == speed {
			return o.display
		}
	}
	return tabAnimSpeedOptions()[1].display // 非法值收敛到「正常」
}

// tabDotShapeOptions 标签形状选择项（ID 与当前语言文案，顺序即下拉顺序）。
func tabDotShapeOptions() []struct {
	id, display string
} {
	return []struct {
		id, display string
	}{
		{tabDotShapeDot, locales.ShapeDot()},
		{tabDotShapeRing, locales.ShapeRing()},
	}
}

func tabDotShapeDisplay(id string) string {
	for _, o := range tabDotShapeOptions() {
		if o.id == id {
			return o.display
		}
	}
	return tabDotShapeOptions()[0].display // 空串/非法值收敛到「圆点」
}

func tabDotShapeKey(display string) string {
	for _, o := range tabDotShapeOptions() {
		if o.display == display {
			return o.id
		}
	}
	return tabDotShapeDot
}

// tabAppearanceSection 外观页的「顶栏」区：形状、标题宽、动画速度、
// 顶栏动效、毛玻璃回退强度（标签列表在独立的「标签」页，见 settings_tabs.go）。
func (a *app) tabAppearanceSection(c *ui.Context) {
	// Select/Segmented 绑定显示文案，每帧从当前值同步。
	a.settingsUI.tabAnimSpeedSel = tabAnimSpeedDisplay(a.cfg.TabAnimSpeed)
	a.settingsUI.tabDotShapeSel = tabDotShapeDisplay(a.activeDotShape())
	a.settingsUI.tabShapeIdx = 0
	if a.activeDotShape() == tabDotShapeRing {
		a.settingsUI.tabShapeIdx = 1
	}
	titleW := float64(a.cfg.TabTitleWidth)
	if titleW <= 0 {
		titleW = 120
	}
	glass := float64(a.cfg.GlassStrength)
	if glass <= 0 {
		glass = 0.5
	}
	ui.Column(c).FillWidth().Children(func() {
		settingsCompactRow(c, locales.TabDotShape(), "", func() {
			if ui.Segmented(c, &a.settingsUI.tabShapeIdx, locales.ShapeDot(), locales.ShapeRing()).
				Label(locales.TabDotShape()).Changed() {
				if a.settingsUI.tabShapeIdx == 1 {
					a.cfg.TabDotShape = tabDotShapeRing
				} else {
					a.cfg.TabDotShape = tabDotShapeDot
				}
				a.scheduleSave()
				a.withWindow(func() {})
			}
		})
		settingsCompactRow(c, locales.TabTitleMaxWidth(), "", func() {
			if ui.NumberInput(c, &titleW, 80, 220, 10).Changed() {
				a.cfg.TabTitleWidth = float32(titleW)
				a.scheduleSave()
				a.withWindow(func() {})
			}
		})
		settingsCompactRow(c, locales.TabAnimSpeed(), "", func() {
			opts := tabAnimSpeedOptions()
			displays := make([]string, len(opts))
			for i, o := range opts {
				displays[i] = o.display
			}
			if boundedSelect(c, &a.settingsUI.tabAnimSpeedSel, displays).Width(200).Changed() {
				a.cfg.TabAnimSpeed = tabAnimSpeedKey(a.settingsUI.tabAnimSpeedSel)
				a.scheduleSave()
			}
		})
		settingsCompactRow(c, locales.TabMotion(), "", func() {
			motion := !a.cfg.TabMotionOff
			if ui.Switch(c, &motion).Label(locales.TabMotionSwitch()).Changed() {
				a.cfg.TabMotionOff = !motion
				a.scheduleSave()
				a.withWindow(func() {})
			}
		})
		settingsCompactRow(c, locales.GlassStrength(), "", func() {
			if ui.NumberInput(c, &glass, 0.2, 0.9, 0.05).Changed() {
				a.cfg.GlassStrength = float32(glass)
				a.scheduleSave()
				a.withWindow(func() {})
			}
		})
		ui.Text(c, locales.TabsPageFootnote()).
			FontSize(11).TextColor(tMuted(c)).PaddingY(10)
	})
}

func tabAnimSpeedKey(display string) float32 {
	for _, o := range tabAnimSpeedOptions() {
		if o.display == display {
			return o.speed
		}
	}
	return 1
}
