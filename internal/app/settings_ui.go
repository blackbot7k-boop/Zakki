package app

// 设置页 UI：常规、外观（卡片分组 + 常驻实时预览）、备份与数据、快捷键。
// 所有字段即时应用并自动保存；「恢复本页默认值」在卡片底部，需确认，不清除笔记或备份。

import (
	"strconv"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/i18n"
	"github.com/egoist/mygo/ui"

	"zakki/locales"
)

// ---- 常规 ----

func (a *app) generalPage(c *ui.Context, t *ui.Theme, dark bool) {
	ui.Column(c).FillWidth().Gap(14).Children(func() {
		a.migrationStatus(c)
		settingsCard(c, locales.CardLanguage(), func() {
			a.languageRow(c)
		})
		settingsCard(c, locales.CardWindow(), func() {
			settingsRow(c, locales.RememberBounds(), locales.RememberBoundsHint(), func() {
				if ui.Switch(c, &a.settingsUI.draft.Window.RememberBounds).Label(locales.RememberBoundsSwitch()).Changed() {
					a.pushWindow()
				}
			}).PaddingX(14)
			a.cardResetFooter(c, pageGeneral)
		})
	})
}

// languageOptions 语言选择项：跟随系统 + 各目录语言（以语言自称显示）。
func languageOptions() []struct{ tag, display string } {
	return []struct{ tag, display string }{
		{"", locales.MenuFollowSystem()},
		{"en", i18n.Autonym("en")},
		{"zh-Hans", i18n.Autonym("zh-Hans")},
	}
}

func languageDisplay(tag string) string {
	for _, o := range languageOptions() {
		if o.tag == tag {
			return o.display
		}
	}
	return languageOptions()[0].display
}

func languageTagOfDisplay(display string) string {
	for _, o := range languageOptions() {
		if o.display == display {
			return o.tag
		}
	}
	return ""
}

// languageRow 常规页语言选择：变更经 updatePreferences 即时生效并持久化。
func (a *app) languageRow(c *ui.Context) {
	opts := languageOptions()
	displays := make([]string, len(opts))
	for i, o := range opts {
		displays[i] = o.display
	}
	settingsRow(c, locales.LanguageRow(), locales.LanguageRowHint(), func() {
		if ui.Select(c, &a.settingsUI.languageSel, displays).Changed() {
			tag := languageTagOfDisplay(a.settingsUI.languageSel)
			a.settingsUI.draft.Appearance.Language = tag
			_ = a.updatePreferences(func(p *preferences) { p.Appearance.Language = tag })
		}
	}).PaddingX(14)
}

// migrationStatus 一次性迁移结果与偏好恢复入口。
func (a *app) migrationStatus(c *ui.Context) {
	switch a.currentPrefsIssue().Kind {
	case prefsIssueLegacyBroken:
		ui.Column(c).FillWidth().Gap(6).Padding(10, 12).Radius(8).
			Background(c.Theme().Surface).BorderWidth(1).BorderColor(c.Theme().Warning).Children(func() {
			ui.Text(c, locales.LegacyConfigFailed()).Bold()
			ui.Text(c, a.currentPrefsIssue().Message).FontSize(12).TextColor(tMuted(c))
			if ui.Button(c, locales.UseExistingPrefs()).Clicked() {
				if err := a.confirmContinueWithoutLegacyConfig(); err != nil {
					a.settingsUI.keyErr = err.Error()
				}
				a.settingsUI.syncDraftFromPrefs(a.prefs)
			}
		})
	case prefsIssuePrefsBroken:
		ui.Column(c).FillWidth().Gap(6).Padding(10, 12).Radius(8).
			Background(c.Theme().Surface).BorderWidth(1).BorderColor(c.Theme().Danger).Children(func() {
			ui.Text(c, locales.PrefsNeedRecovery()).Bold()
			ui.Text(c, a.currentPrefsIssue().Message).FontSize(12).TextColor(tMuted(c))
			if ui.Button(c, locales.SaveCopyAndReset()).Clicked() {
				if err := a.confirmRecoverPreferences(); err != nil {
					a.settingsUI.keyErr = err.Error()
				}
				a.settingsUI.syncDraftFromPrefs(a.prefs)
			}
		})
	default:
		if a.prefsMigration.From != "" {
			ui.Text(c, locales.PrefsMigrationLine(a.prefsMigration.From)).FontSize(12).TextColor(tMuted(c))
		}
	}
}

// ---- 外观 ----

// appearancePage 外观页：宽窗口下左控制、右常驻实时预览；窗口不足 860
// 宽时退化为预览在上的单栏。预览套用草稿的排版微调，改动即时可见。
func (a *app) appearancePage(c *ui.Context, t *ui.Theme, dark bool) {
	previewPane := func() {
		ui.Column(c).FillWidth().Gap(8).Children(func() {
			ui.Text(c, locales.PreviewLabel()).Bold().TextColor(tMuted(c)).FontSize(12)
			a.themePreview(c, dark)
		})
	}
	if settingsWideLayout(c) {
		ui.Row(c).Fill().Grow(1).Gap(16).Children(func() {
			ui.Scroll(c.Key("appearance-controls")).Grow(1).FillHeight().Children(func() {
				a.appearanceControls(c, dark)
			})
			ui.Column(c).Width(380).Shrink(0).FillHeight().Children(func() {
				ui.Scroll(c.Key("appearance-preview")).FillWidth().Grow(1).Children(previewPane)
			})
		})
		return
	}
	ui.Scroll(c.Key("appearance-stacked")).Fill().Grow(1).Children(func() {
		ui.Column(c).FillWidth().Gap(14).Children(func() {
			previewPane()
			a.appearanceControls(c, dark)
		})
	})
}

// settingsWideLayout 外观页是否采用左右分栏：视口宽度 ≥ 860 时预览常驻右侧，
// 否则退化为预览在上的单栏。
func settingsWideLayout(c *ui.Context) bool {
	w, _ := c.Size()
	return w >= 860
}

// appearanceControls 外观页的控制项卡片列。
func (a *app) appearanceControls(c *ui.Context, dark bool) {
	ui.Column(c).FillWidth().Gap(14).Children(func() {
		settingsCard(c, locales.CardAppearanceMode(), func() {
			ui.Column(c).FillWidth().Padding(4, 14, 10, 14).Children(func() {
				a.appearanceModeSelect(c)
			})
		})

		settingsCard(c, locales.CardColorTheme(), func() {
			ui.Column(c).FillWidth().Padding(8, 14, 10, 14).Children(func() {
				a.colorThemeCards(c, dark)
			})
		})

		settingsCard(c, locales.CardMarkdownTheme(), func() {
			ui.Column(c).FillWidth().Padding(6, 0).Children(func() {
				a.markdownThemeSelect(c)
			})
			a.cardResetFooter(c, pageAppearance)
		})

		settingsCard(c, locales.CardCodeTheme(), func() {
			ui.Column(c).FillWidth().Padding(6, 0).Children(func() {
				a.codeThemeSelect(c)
			})
			a.cardResetFooter(c, pageAppearance)
		})

		settingsCard(c, locales.CardPresetSketch(), func() {
			settingsRow(c, locales.PresetSketchRow(), locales.PresetSketchHint(), func() {
				ap := a.settingsUI.draft.Appearance
				applied := ap.ColorTheme == "sketch" && ap.MarkdownTheme == "sketch" && ap.BodyFont == "hand" && ap.LatinFont == ""
				label := locales.ApplyOnce()
				if applied {
					label = locales.Applied()
				}
				if ui.Button(c.Key("preset-sketch"), label).Label(locales.ApplySketchTheme()).Clicked() && !applied {
					a.settingsUI.draft.Appearance.ColorTheme = "sketch"
					a.settingsUI.draft.Appearance.MarkdownTheme = "sketch"
					a.settingsUI.draft.Appearance.BodyFont = "hand"
					a.settingsUI.draft.Appearance.LatinFont = ""
					a.settingsUI.markdownThemeSel = markdownThemeDisplay("sketch")
					a.settingsUI.bodyFontSel = displayOf(bodyFontKeys, bodyFontOptions, "hand")
					a.settingsUI.latinFontSel = displayOf(latinFontKeys, latinFontOptions, "")
					a.pushAppearance()
				}
			}).PaddingX(14)
		})

		settingsCard(c, locales.CardTypographyTweaks(), func() {
			ui.Column(c).FillWidth().Padding(6, 0).Children(func() {
				a.typographyOverrides(c)
			})
			a.cardResetFooter(c, pageAppearance)
		})
	})
}

func (a *app) appearanceModeSelect(c *ui.Context) {
	ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(12).Children(func() {
		ui.Text(c, locales.AppearanceLabel()).Grow(1)
		if ui.Select(c, &a.settingsUI.appearanceModeSel, []string{locales.ModeFollowSystem(), locales.ModeLight(), locales.ModeDark()}).Changed() {
			a.settingsUI.draft.Appearance.AppearanceMode = appearanceModeKey(a.settingsUI.appearanceModeSel)
			a.pushAppearance()
		}
	})
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
				if cd.id == "sketch" {
					// 手绘是整套风格：颜色、手账排版与手写字体一起切换，之后仍可单独调整。
					a.settingsUI.draft.Appearance.MarkdownTheme = "sketch"
					a.settingsUI.draft.Appearance.BodyFont = "hand"
					a.settingsUI.draft.Appearance.LatinFont = ""
					a.settingsUI.markdownThemeSel = markdownThemeDisplay("sketch")
					a.settingsUI.bodyFontSel = displayOf(bodyFontKeys, bodyFontOptions, "hand")
					a.settingsUI.latinFontSel = displayOf(latinFontKeys, latinFontOptions, "")
				}
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
// 右侧预览即时呈现效果，不再放整组样例卡片。
func (a *app) markdownThemeSelect(c *ui.Context) {
	options := make([]string, 0, len(markdownLayouts))
	for _, l := range markdownLayouts {
		options = append(options, markdownLayoutDisplayName(l.id))
	}
	ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(12).Children(func() {
		ui.Text(c, locales.LayoutThemeLabel()).Grow(1)
		if ui.Select(c, &a.settingsUI.markdownThemeSel, options).Changed() {
			a.settingsUI.draft.Appearance.MarkdownTheme = markdownThemeKey(a.settingsUI.markdownThemeSel)
			a.pushAppearance()
		}
	})
	ui.Text(c, markdownLayoutNote(a.settingsUI.draft.Appearance.MarkdownTheme)).
		FontSize(11).TextColor(tMuted(c))
}

// codeThemeSelect 代码配色下拉：空串 = 跟随文本（不上色）。
func (a *app) codeThemeSelect(c *ui.Context) {
	options := []string{locales.CodeThemeNone(), locales.CodeThemeSoft(), locales.CodeThemeVivid()}
	ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(12).Children(func() {
		ui.Text(c, locales.CodeThemeLabel()).Grow(1)
		if ui.Select(c, &a.settingsUI.codeThemeSel, options).Changed() {
			a.settingsUI.draft.Appearance.CodeTheme = codeThemeKey(a.settingsUI.codeThemeSel)
			a.pushAppearance()
		}
	})
}

// codeThemeKey 显示名 → 配色方案 id。
func codeThemeKey(display string) string {
	switch display {
	case locales.CodeThemeSoft():
		return "soft"
	case locales.CodeThemeVivid():
		return "vivid"
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

// typographyOverrides 排版覆盖：每项可「跟随 Markdown 主题」（空 = 跟随）。
func (a *app) typographyOverrides(c *ui.Context) {
	bodyOptions, bodyKeys := bodyFontOptions, bodyFontKeys
	monoOptions := []string{locales.FontJetBrainsDefault(), locales.FontSFMono(), locales.FontMenlo()}
	monoKeys := []string{"jbm", "sfmono", "menlo"}

	ui.Column(c).FillWidth().Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(12).Padding(6, 14).Children(func() {
			ui.Column(c).Grow(1).Gap(0).Children(func() {
				ui.Text(c, locales.FontChinese())
				ui.Text(c, locales.FontChineseHint()).FontSize(11).TextColor(tMuted(c))
			})
			if ui.Select(c, &a.settingsUI.bodyFontSel, bodyOptions).Changed() {
				a.settingsUI.draft.Appearance.BodyFont = keyOfDisplay(bodyKeys, bodyOptions, a.settingsUI.bodyFontSel)
				a.pushAppearance()
			}
		})
		cardDivider(c)
		ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(12).Padding(6, 14).Children(func() {
			ui.Column(c).Grow(1).Gap(0).Children(func() {
				ui.Text(c, locales.FontWestern())
				ui.Text(c, locales.FontWesternHint()).FontSize(11).TextColor(tMuted(c))
			})
			if ui.Select(c, &a.settingsUI.latinFontSel, latinFontOptions).Changed() {
				a.settingsUI.draft.Appearance.LatinFont = keyOfDisplay(latinFontKeys, latinFontOptions, a.settingsUI.latinFontSel)
				a.pushAppearance()
			}
		})
		cardDivider(c)
		ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(12).Padding(6, 14).Children(func() {
			ui.Column(c).Grow(1).Gap(0).Children(func() {
				ui.Text(c, locales.FontCode())
				ui.Text(c, locales.FontCodeHint()).FontSize(11).TextColor(tMuted(c))
			})
			if ui.Select(c, &a.settingsUI.monoFontSel, monoOptions).Changed() {
				a.settingsUI.draft.Appearance.MonoFont = keyOfDisplay(monoKeys, monoOptions, a.settingsUI.monoFontSel)
				a.pushAppearance()
			}
		})
		cardDivider(c)
		// 字号（10–40）/ 行高（1–3）：非法数字不提交，保留输入并显示行内错误。
		a.floatOverrideRow(c, locales.FontSizeRow(), locales.OverrideFollowHint(), "bodySize")
		cardDivider(c)
		a.floatOverrideRow(c, locales.LineHeightRow(), locales.OverrideLineHint(), "lineHeight")
	})
}

// floatOverrideRow 数字覆盖输入行；占位符显示当前排版主题的默认值。
func (a *app) floatOverrideRow(c *ui.Context, label, detail, field string) {
	layout := markdownLayoutByID(a.settingsUI.draft.Appearance.MarkdownTheme)
	placeholder := floatText(layout.bodySize)
	if field == "lineHeight" {
		placeholder = floatText(layout.lineHeight)
	}
	ui.Column(c).FillWidth().Gap(2).Padding(6, 14).Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(12).Children(func() {
			ui.Column(c).Grow(1).Gap(0).Children(func() {
				ui.Text(c, label)
				ui.Text(c, detail).FontSize(11).TextColor(tMuted(c))
			})
			ui.Column(c).Width(110).Gap(2).Children(func() {
				text := &a.settingsUI.bodySizeText
				if field == "lineHeight" {
					text = &a.settingsUI.lineHeightText
				}
				if ui.TextInput(c, text).Placeholder(placeholder).Changed() {
					a.commitFloatField(field)
				}
			})
		})
		errText := a.settingsUI.bodySizeErr
		if field == "lineHeight" {
			errText = a.settingsUI.lineHeightErr
		}
		if errText != "" {
			ui.Text(c, errText).FontSize(11).TextColor(c.Theme().Danger)
		}
	})
}

// commitFloatField 解析字号/行高输入：合法才提交，非法文本和错误跨帧保留。
func (a *app) commitFloatField(field string) {
	var text *string
	var errField *string
	var dirty *bool
	var target *float32
	switch field {
	case "bodySize":
		text = &a.settingsUI.bodySizeText
		errField = &a.settingsUI.bodySizeErr
		dirty = &a.settingsUI.bodySizeDirty
		target = &a.settingsUI.draft.Appearance.BodySize
	case "lineHeight":
		text = &a.settingsUI.lineHeightText
		errField = &a.settingsUI.lineHeightErr
		dirty = &a.settingsUI.lineHeightDirty
		target = &a.settingsUI.draft.Appearance.LineHeight
	default:
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
		if field == "bodySize" {
			p.Appearance.BodySize = value
		} else {
			p.Appearance.LineHeight = value
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

// previewStyle 用给定的外观偏好（通常是草稿，含排版微调覆盖）解析预览样式。
func (a *app) previewStyle(ap appearancePrefs, dark bool) resolvedStyle {
	saved := a.prefs
	a.prefs.Appearance = ap
	style := a.resolveStyle(-1, dark, true)
	a.prefs = saved
	return style
}

// themePreview 只读示例预览：覆盖中英文、标题、链接、代码、引用、任务及表格。
// 套用草稿的颜色主题、排版主题与排版微调（字号/行高/字体覆盖即时可见）。
func (a *app) themePreview(c *ui.Context, dark bool) {
	style := a.previewStyle(a.settingsUI.draft.Appearance, dark)

	savedTheme := c.Theme()
	defer c.SetTheme(savedTheme)
	theme := style.apply(*savedTheme)
	c.SetTheme(&theme)
	previewChecked := true
	ui.Column(c).FillWidth().Padding(14, 16).Gap(8).
		Radius(8).BorderWidth(1).BorderColor(style.border).
		Background(style.background).Font(style.bodyFont).FontSize(style.bodySize * 0.8).
		LineHeight(style.lineHeight).Children(func() {
		ui.Text(c, locales.PreviewHeading()+" Writing").Font(style.headingFont).Bold().FontSize(style.headingSizes[1]).TextColor(style.text)
		ui.RichText(c).Children(func() {
			ui.Text(c, locales.PreviewBody())
			ui.Text(c, locales.PreviewBold()).Bold()
			ui.Text(c, locales.PreviewAnd())
			ui.Text(c, locales.PreviewItalic()).Italic()
			ui.Text(c, locales.PreviewComma())
			ui.Text(c, "code").Font(style.monoFont).FontSize(style.codeSize).TextColor(style.textMuted)
			ui.Text(c, locales.PreviewAnd())
			ui.Text(c, locales.PreviewLink()).TextColor(style.accent).Underline()
			ui.Text(c, locales.PreviewMixTail())
		})
		ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
			ui.Checkbox(c, &previewChecked, "")
			ui.Text(c, locales.PreviewTask()).TextColor(style.text)
		})
		ui.Row(c).Gap(6).Children(func() {
			quoteBar(c, style, 7)
			ui.Text(c, locales.PreviewQuote()).TextColor(style.textMuted)
		})
		ui.Row(c).Gap(0).Children(func() {
			ui.Box(c).Grow(1).Padding(style.tablePadY, style.tablePadX).Background(style.surface).BorderWidth(0, 0, 1, 1).BorderColor(style.border).Children(func() {
				ui.Text(c, locales.PreviewTableHeader()).Bold().FontSize(style.codeSize)
			})
			ui.Box(c).Grow(1).Padding(style.tablePadY, style.tablePadX).Background(style.surface).BorderWidth(0, 0, 1, 0).BorderColor(style.border).Children(func() {
				ui.Text(c, "Header").Bold().FontSize(style.codeSize)
			})
		})
		ui.Row(c).Gap(0).Children(func() {
			ui.Box(c).Grow(1).Padding(style.tablePadY, style.tablePadX).BorderWidth(0, 0, 1, 1).BorderColor(style.border).Children(func() {
				ui.Text(c, locales.PreviewTableCell()).FontSize(style.codeSize)
			})
			ui.Box(c).Grow(1).Padding(style.tablePadY, style.tablePadX).BorderWidth(0, 0, 1, 0).BorderColor(style.border).Children(func() {
				ui.Text(c, "Cell").FontSize(style.codeSize)
			})
		})
	})
}

// ---- 备份与数据 ----

func (a *app) backupPage(c *ui.Context, t *ui.Theme, dark bool) {
	interval := a.settingsUI.draft.Backup.IntervalMinutes
	keep := a.settingsUI.draft.Backup.Keep
	intervalF, keepF := float64(interval), float64(keep)
	ui.Column(c).FillWidth().Gap(14).Children(func() {
		// 未保存指示与写保护状态。
		a.persistStatus(c, t)

		settingsCard(c, locales.CardExternalBackup(), func() {
			mono := monoFontStack("sfmono")
			if dir := a.prefs.Backup.MirrorDir; dir == "" {
				settingsRow(c, locales.MirrorUnconfigured(), locales.MirrorUnconfiguredHint(), func() {
					if ui.Button(c, locales.ChooseFolder()).Clicked() {
						a.pickMirrorFolder()
					}
				}).PaddingX(14)
			} else {
				ui.Column(c).FillWidth().Padding(10, 14).Gap(8).Children(func() {
					ui.Text(c, dir).FontSize(11).Font(mono).TextColor(tMuted(c))
					ui.Text(c, a.mirrorStateText(dir)).FontSize(12).TextColor(tMuted(c))
					ui.Row(c).Gap(8).Children(func() {
						if ui.Button(c, locales.OpenFolder()).Clicked() {
							_ = mygo.Shell.OpenPath(dir)
						}
						if ui.Button(c, locales.CommonChange()).Clicked() {
							a.pickMirrorFolder()
						}
						if ui.Button(c, locales.CommonDisconnect()).Clicked() {
							a.disconnectMirror()
						}
					})
				})
			}
			cardDivider(c)
			settingsRow(c, locales.GitAutomation(), a.gitModeDetailText(), func() {
				if ui.Select(c, &a.settingsUI.gitModeSel, gitModeOptions()).Changed() {
					a.pushMirrorGitMode()
				}
			}).PaddingX(14)
		})

		settingsCard(c, locales.CardAutoBackup(), func() {
			settingsRow(c, locales.AutoBackupInterval(), locales.AutoBackupIntervalHint(), func() {
				if ui.NumberInput(c, &intervalF, 1, 1440, 1).Changed() {
					a.settingsUI.draft.Backup.IntervalMinutes = int(intervalF)
					a.pushBackup()
				}
			}).PaddingX(14)
			cardDivider(c)
			settingsRow(c, locales.BackupKeep(), locales.BackupKeepHint(), func() {
				if ui.NumberInput(c, &keepF, 1, 1000, 1).Changed() {
					a.settingsUI.draft.Backup.Keep = int(keepF)
					a.pushBackup()
				}
			}).PaddingX(14)
			cardDivider(c)
			settingsRow(c, locales.BackupOnLaunch(), locales.BackupOnLaunchHint(), func() {
				if ui.Switch(c, &a.settingsUI.draft.Backup.OnLaunch).Label(locales.BackupOnLaunchSwitch()).Changed() {
					a.pushBackup()
				}
			}).PaddingX(14)
		})

		settingsCard(c, locales.CardManualActions(), func() {
			ui.Column(c).FillWidth().Padding(10, 14).Gap(8).Children(func() {
				ui.Row(c).Gap(8).Children(func() {
					if ui.Button(c, locales.BackupNowButton()).Clicked() {
						if err := a.backupNow(); err != nil {
							a.settingsUI.keyErr = locales.BackupFailedLine(err.Error())
						} else {
							a.settingsUI.keyErr = ""
						}
					}
					if ui.Button(c, locales.OpenBackupFolder()).Clicked() {
						_ = mygo.Shell.OpenPath(a.notes.BackupDir)
					}
					if ui.Button(c, locales.ShowDataLocation()).Clicked() {
						mygo.Shell.ShowItemInFolder(a.notes.DataFile)
					}
				})
				mono := monoFontStack("sfmono")
				ui.Text(c, locales.BackupDirLine(a.notes.BackupDir)).FontSize(11).Font(mono).TextColor(tMuted(c))
				ui.Text(c, locales.DataFileLine(a.notes.DataFile)).FontSize(11).Font(mono).TextColor(tMuted(c))
			})
			a.cardResetFooter(c, pageBackup)
		})

		// 本页操作错误（备份失败、目录不可用等）。
		if a.settingsUI.keyErr != "" {
			ui.Text(c, a.settingsUI.keyErr).FontSize(12).TextColor(t.Danger)
		}
	})
}

// persistStatus 持久化状态：未保存指示、重试入口、写保护恢复。
func (a *app) persistStatus(c *ui.Context, t *ui.Theme) {
	if msg := a.prefsSaveError(); msg != "" {
		ui.Column(c).FillWidth().Gap(6).Padding(10, 12).Radius(8).
			Background(t.Surface).BorderWidth(1).BorderColor(t.Danger).Children(func() {
			ui.Text(c, locales.SettingsUnsaved()).Bold().TextColor(t.Danger)
			ui.Text(c, msg).FontSize(12).TextColor(tMuted(c))
			if ui.Button(c, locales.RetrySave()).Clicked() {
				a.retryPreferencesSave()
				a.settingsUI.syncDraftFromPrefs(a.prefs)
			}
		})
	}
	blocked := a.notes.SaveBlocked()
	if blocked {
		ui.Column(c).FillWidth().Gap(6).Padding(10, 12).Radius(8).
			Background(t.Surface).BorderWidth(1).BorderColor(t.Warning).Children(func() {
			ui.Text(c, locales.WriteProtectedTitle()).Bold()
			ui.Text(c, locales.WriteProtectedBody()).FontSize(12).TextColor(tMuted(c))
			if ui.Button(c, locales.WriteBackButton()).Clicked() {
				if err := a.confirmWriteBack(); err != nil {
					a.settingsUI.keyErr = locales.WriteBackFailed(err.Error())
				}
			}
		})
	}
}

// ---- 卡片分组小部件 ----

// settingsCard 卡片分组容器：圆角描边，组标题在顶部。
func settingsCard(c *ui.Context, title string, body func()) ui.Element {
	return ui.Column(c).FillWidth().Radius(10).Clip().
		BorderWidth(1).BorderColor(c.Theme().Border).
		Background(c.Theme().Surface).Children(func() {
		if title != "" {
			ui.Text(c, title).FontSize(12).Bold().TextColor(tMuted(c)).Padding(12, 14, 0, 14)
		}
		body()
	})
}

// cardDivider 卡片内行间分隔线。
func cardDivider(c *ui.Context) {
	ui.Divider(c).FillWidth()
}

// cardResetFooter 卡片底部的「恢复本页默认值」行（右置，需确认）。
func (a *app) cardResetFooter(c *ui.Context, page string) {
	cardDivider(c)
	ui.Row(c).FillWidth().Padding(8, 14).Children(func() {
		ui.Box(c).Grow(1)
		if ui.Button(c, locales.ResetPageButton()).Clicked() {
			a.confirmResetPage(page)
		}
	})
}
