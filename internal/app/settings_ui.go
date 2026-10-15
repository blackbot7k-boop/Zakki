package app

// 设置页 UI：常规、外观（卡片分组 + 常驻实时预览）、备份与数据、快捷键。
// 所有字段即时应用并自动保存；「恢复本页默认值」在卡片底部，需确认，不清除笔记或备份。

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// ---- 常规 ----

func (a *app) generalPage(c *ui.Context, t *ui.Theme, dark bool) {
	ui.Column(c).FillWidth().Gap(14).Children(func() {
		a.migrationStatus(c)
		settingsCard(c, "窗口", func() {
			settingsRow(c, "记忆主窗口位置与尺寸", "从下次创建主窗口起生效；关闭窗口后重新打开即应用", func() {
				if ui.Switch(c, &a.settingsUI.draft.Window.RememberBounds).Label("记忆主窗口位置开关").Changed() {
					a.pushWindow()
				}
			}).PaddingX(14)
			a.cardResetFooter(c, pageGeneral)
		})
	})
}

// migrationStatus 一次性迁移结果与偏好恢复入口。
func (a *app) migrationStatus(c *ui.Context) {
	switch a.currentPrefsIssue().Kind {
	case prefsIssueLegacyBroken:
		ui.Column(c).FillWidth().Gap(6).Padding(10, 12).Radius(8).
			Background(c.Theme().Surface).BorderWidth(1).BorderColor(c.Theme().Warning).Children(func() {
			ui.Text(c, "旧配置未能导入").Bold()
			ui.Text(c, a.currentPrefsIssue().Message).FontSize(12).TextColor(tMuted(c))
			if ui.Button(c, "使用已有偏好继续").Clicked() {
				if err := a.confirmContinueWithoutLegacyConfig(); err != nil {
					a.settingsUI.keyErr = err.Error()
				}
				a.settingsUI.syncDraftFromPrefs(a.prefs)
			}
		})
	case prefsIssuePrefsBroken:
		ui.Column(c).FillWidth().Gap(6).Padding(10, 12).Radius(8).
			Background(c.Theme().Surface).BorderWidth(1).BorderColor(c.Theme().Danger).Children(func() {
			ui.Text(c, "偏好文件需要恢复").Bold()
			ui.Text(c, a.currentPrefsIssue().Message).FontSize(12).TextColor(tMuted(c))
			if ui.Button(c, "另存原件并恢复默认设置").Clicked() {
				if err := a.confirmRecoverPreferences(); err != nil {
					a.settingsUI.keyErr = err.Error()
				}
				a.settingsUI.syncDraftFromPrefs(a.prefs)
			}
		})
	default:
		if a.prefsMigration.From != "" {
			detail := "已从 " + a.prefsMigration.From + " 迁移一次；旧文件保留原样，不再读取。"
			ui.Text(c, "偏好迁移："+detail).FontSize(12).TextColor(tMuted(c))
		}
	}
}

// ---- 外观 ----

// appearancePage 外观页：宽窗口下左控制、右常驻实时预览；窗口不足 860
// 宽时退化为预览在上的单栏。预览套用草稿的排版微调，改动即时可见。
func (a *app) appearancePage(c *ui.Context, t *ui.Theme, dark bool) {
	previewPane := func() {
		ui.Column(c).FillWidth().Gap(8).Children(func() {
			ui.Text(c, "预览").Bold().TextColor(tMuted(c)).FontSize(12)
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
		settingsCard(c, "明暗模式", func() {
			ui.Column(c).FillWidth().Padding(4, 14, 10, 14).Children(func() {
				a.appearanceModeSelect(c)
			})
		})

		settingsCard(c, "颜色主题", func() {
			ui.Column(c).FillWidth().Padding(8, 14, 10, 14).Children(func() {
				a.colorThemeCards(c, dark)
			})
		})

		settingsCard(c, "Markdown 排版主题", func() {
			ui.Column(c).FillWidth().Padding(6, 0).Children(func() {
				a.markdownThemeSelect(c)
			})
			a.cardResetFooter(c, pageAppearance)
		})

		settingsCard(c, "排版微调", func() {
			ui.Column(c).FillWidth().Padding(6, 0).Children(func() {
				a.typographyOverrides(c)
			})
			a.cardResetFooter(c, pageAppearance)
		})
	})
}

func (a *app) appearanceModeSelect(c *ui.Context) {
	ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(12).Children(func() {
		ui.Text(c, "界面外观").Grow(1)
		if ui.Select(c, &a.settingsUI.appearanceModeSel, []string{"跟随系统", "浅色", "深色"}).Changed() {
			a.settingsUI.draft.Appearance.AppearanceMode = appearanceModeKey(a.settingsUI.appearanceModeSel)
			a.pushAppearance()
		}
	})
}

func appearanceModeKey(display string) string {
	switch display {
	case "浅色":
		return "light"
	case "深色":
		return "dark"
	}
	return "system"
}

func appearanceModeDisplay(key string) string {
	switch key {
	case "light":
		return "浅色"
	case "dark":
		return "深色"
	}
	return "跟随系统"
}

// colorThemeCards 颜色主题选择卡片：名称 + 迷你界面样（顶栏 + 文字/强调色线条），
// 五列网格，选中态描边。
func (a *app) colorThemeCards(c *ui.Context, dark bool) {
	type card struct {
		id, name string
		pal      colorPalette
	}
	cards := []card{{id: "tot", name: "Tot", pal: totNeutralPalette(dark)}}
	for _, def := range colorThemes {
		pal := def.light
		if dark {
			pal = def.dark
		}
		cards = append(cards, card{id: def.id, name: def.name, pal: pal})
	}
	ui.Grid(c).Columns(5).Gap(8).FillWidth().Children(func() {
		for _, cd := range cards {
			cd := cd
			selected := a.settingsUI.draft.Appearance.ColorTheme == cd.id
			btn := ui.ButtonBase(c.Key("color-"+cd.id)).Label("颜色主题 "+cd.name).
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
// 右侧预览即时呈现效果，不再放整组样例卡片。
func (a *app) markdownThemeSelect(c *ui.Context) {
	options := make([]string, 0, len(markdownLayouts))
	for _, l := range markdownLayouts {
		options = append(options, l.name)
	}
	ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(12).Children(func() {
		ui.Text(c, "排版主题").Grow(1)
		if ui.Select(c, &a.settingsUI.markdownThemeSel, options).Changed() {
			a.settingsUI.draft.Appearance.MarkdownTheme = markdownThemeKey(a.settingsUI.markdownThemeSel)
			a.pushAppearance()
		}
	})
	ui.Text(c, markdownLayoutNote(a.settingsUI.draft.Appearance.MarkdownTheme)).
		FontSize(11).TextColor(tMuted(c))
}

// markdownThemeDisplay 排版主题键 → 显示名。
func markdownThemeDisplay(key string) string {
	for _, l := range markdownLayouts {
		if l.id == key {
			return l.name
		}
	}
	return markdownLayouts[0].name
}

// markdownThemeKey 显示名 → 排版主题键。
func markdownThemeKey(display string) string {
	for _, l := range markdownLayouts {
		if l.name == display {
			return l.id
		}
	}
	return markdownLayouts[0].id
}

// markdownLayoutNote 选中排版的一行摘要。
func markdownLayoutNote(key string) string {
	l := markdownLayoutByID(key)
	return fmt.Sprintf("%s：%s，%gpx / %g 行高", l.name, layoutFontNote(l.id), l.bodySize, l.lineHeight)
}

// layoutFontNote 保留排版名字里的字体意象（仅作说明文字，不影响实际字体）。
func layoutFontNote(id string) string {
	switch id {
	case "book":
		return "衬线意象"
	case "compact":
		return "收紧间距"
	case "tot":
		return "便签排版"
	case "clear":
		return "层级明确"
	}
	return "Typora 式排版"
}

// typographyOverrides 排版覆盖：每项可「跟随 Markdown 主题」（空 = 跟随）。
func (a *app) typographyOverrides(c *ui.Context) {
	bodyOptions := []string{"霞鹜文楷（默认）", "系统苹方", "宋体"}
	bodyKeys := []string{"kai", "pingfang", "songti"}
	monoOptions := []string{"JetBrains Mono（默认）", "SF Mono", "Menlo"}
	monoKeys := []string{"jbm", "sfmono", "menlo"}

	ui.Column(c).FillWidth().Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(12).Padding(6, 14).Children(func() {
			ui.Column(c).Grow(1).Gap(0).Children(func() {
				ui.Text(c, "正文字体")
				ui.Text(c, "默认霞鹜文楷；与排版主题无关").FontSize(11).TextColor(tMuted(c))
			})
			if ui.Select(c, &a.settingsUI.bodyFontSel, bodyOptions).Changed() {
				a.settingsUI.draft.Appearance.BodyFont = keyOfDisplay(bodyKeys, bodyOptions, a.settingsUI.bodyFontSel)
				a.pushAppearance()
			}
		})
		cardDivider(c)
		ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(12).Padding(6, 14).Children(func() {
			ui.Column(c).Grow(1).Gap(0).Children(func() {
				ui.Text(c, "代码字体")
				ui.Text(c, "默认 JetBrains Mono；与排版主题无关").FontSize(11).TextColor(tMuted(c))
			})
			if ui.Select(c, &a.settingsUI.monoFontSel, monoOptions).Changed() {
				a.settingsUI.draft.Appearance.MonoFont = keyOfDisplay(monoKeys, monoOptions, a.settingsUI.monoFontSel)
				a.pushAppearance()
			}
		})
		cardDivider(c)
		// 字号（10–40）/ 行高（1–3）：非法数字不提交，保留输入并显示行内错误。
		a.floatOverrideRow(c, "字号", "10–40；清空表示跟随 Markdown 主题", "bodySize")
		cardDivider(c)
		a.floatOverrideRow(c, "行高", "1–3；清空表示跟随 Markdown 主题", "lineHeight")
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

// commitFloatField 解析字号/行高输入：合法才提交，非法保留输入并报错。
func (a *app) commitFloatField(field string) {
	var text string
	var errField *string
	var target *float32
	switch field {
	case "bodySize":
		text, errField, target = a.settingsUI.bodySizeText, &a.settingsUI.bodySizeErr, &a.settingsUI.draft.Appearance.BodySize
	case "lineHeight":
		text, errField, target = a.settingsUI.lineHeightText, &a.settingsUI.lineHeightErr, &a.settingsUI.draft.Appearance.LineHeight
	}
	*errField = ""
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		*target = 0 // 跟随主题
		a.pushAppearance()
		return
	}
	v, err := strconv.ParseFloat(trimmed, 32)
	if err != nil {
		*errField = "请输入数字"
		return
	}
	*target = float32(v)
	if err := a.updatePreferences(func(p *preferences) { p.Appearance = a.settingsUI.draft.Appearance }); err != nil {
		*errField = err.Error()
		a.settingsUI.syncDraftFromPrefs(a.prefs)
	}
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
		ui.Text(c, "标题：写作与速记 Writing").Bold().FontSize(style.headingSizes[1]).TextColor(style.text)
		ui.RichText(c).Children(func() {
			ui.Text(c, "正文 ")
			ui.Text(c, "粗体").Bold()
			ui.Text(c, " 与 ")
			ui.Text(c, "斜体").Italic()
			ui.Text(c, "、")
			ui.Text(c, "code").Font(style.monoFont).FontSize(style.codeSize).TextColor(style.textMuted)
			ui.Text(c, " 与 ")
			ui.Text(c, "链接").TextColor(style.accent).Underline()
			ui.Text(c, " 中英文混排 The quick brown fox.")
		})
		ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
			ui.Checkbox(c, &previewChecked, "")
			ui.Text(c, "任务：已完成的事项").TextColor(style.text)
		})
		ui.Row(c).Gap(6).Children(func() {
			ui.Box(c).Width(3).FillHeight().Radius(2).Background(style.quoteBorder)
			ui.Text(c, "引用：灵感来自 Tot。").TextColor(style.textMuted)
		})
		ui.Row(c).Gap(0).Children(func() {
			ui.Box(c).Grow(1).Padding(style.tablePadY, style.tablePadX).Background(style.surface).BorderWidth(0, 0, 1, 1).BorderColor(style.border).Children(func() {
				ui.Text(c, "表头").Bold().FontSize(style.codeSize)
			})
			ui.Box(c).Grow(1).Padding(style.tablePadY, style.tablePadX).Background(style.surface).BorderWidth(0, 0, 1, 0).BorderColor(style.border).Children(func() {
				ui.Text(c, "Header").Bold().FontSize(style.codeSize)
			})
		})
		ui.Row(c).Gap(0).Children(func() {
			ui.Box(c).Grow(1).Padding(style.tablePadY, style.tablePadX).BorderWidth(0, 0, 1, 1).BorderColor(style.border).Children(func() {
				ui.Text(c, "单元格").FontSize(style.codeSize)
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

		settingsCard(c, "自动备份", func() {
			settingsRow(c, "编辑后自动备份最小间隔", "1–1440 分钟；是节流间隔，不是独立定时备份", func() {
				if ui.NumberInput(c, &intervalF, 1, 1440, 1).Changed() {
					a.settingsUI.draft.Backup.IntervalMinutes = int(intervalF)
					a.pushBackup()
				}
			}).PaddingX(14)
			cardDivider(c)
			settingsRow(c, "备份保留数量", "1–1000 份；减小后在下一次成功备份时才裁剪，不立即删除", func() {
				if ui.NumberInput(c, &keepF, 1, 1000, 1).Changed() {
					a.settingsUI.draft.Backup.Keep = int(keepF)
					a.pushBackup()
				}
			}).PaddingX(14)
			cardDivider(c)
			settingsRow(c, "启动时备份", "应用启动且已有数据文件时先做一份备份", func() {
				if ui.Switch(c, &a.settingsUI.draft.Backup.OnLaunch).Label("启动时备份开关").Changed() {
					a.pushBackup()
				}
			}).PaddingX(14)
		})

		settingsCard(c, "手动操作", func() {
			ui.Column(c).FillWidth().Padding(10, 14).Gap(8).Children(func() {
				ui.Row(c).Gap(8).Children(func() {
					if ui.Button(c, "立即备份").Clicked() {
						if err := a.backupNow(); err != nil {
							a.settingsUI.keyErr = "备份失败：" + err.Error()
						} else {
							a.settingsUI.keyErr = ""
						}
					}
					if ui.Button(c, "打开备份目录").Clicked() {
						_ = mygo.Shell.OpenPath(a.notes.BackupDir)
					}
					if ui.Button(c, "显示数据位置").Clicked() {
						mygo.Shell.ShowItemInFolder(a.notes.DataFile)
					}
				})
				mono := monoFontStack("sfmono")
				ui.Text(c, "备份目录："+a.notes.BackupDir).FontSize(11).Font(mono).TextColor(tMuted(c))
				ui.Text(c, "数据文件："+a.notes.DataFile).FontSize(11).Font(mono).TextColor(tMuted(c))
			})
			a.cardResetFooter(c, pageBackup)
		})
	})
}

// persistStatus 持久化状态：未保存指示、重试入口、写保护恢复。
func (a *app) persistStatus(c *ui.Context, t *ui.Theme) {
	if msg := a.prefsSaveError(); msg != "" {
		ui.Column(c).FillWidth().Gap(6).Padding(10, 12).Radius(8).
			Background(t.Surface).BorderWidth(1).BorderColor(t.Danger).Children(func() {
			ui.Text(c, "设置未保存").Bold().TextColor(t.Danger)
			ui.Text(c, msg).FontSize(12).TextColor(tMuted(c))
			if ui.Button(c, "重试保存").Clicked() {
				a.retryPreferencesSave()
				a.settingsUI.syncDraftFromPrefs(a.prefs)
			}
		})
	}
	blocked := a.notes.SaveBlocked()
	if blocked {
		ui.Column(c).FillWidth().Gap(6).Padding(10, 12).Radius(8).
			Background(t.Surface).BorderWidth(1).BorderColor(t.Warning).Children(func() {
			ui.Text(c, "数据文件处于写保护").Bold()
			ui.Text(c, "数据文件损坏且无可用备份；新改动暂存在 .pending，原件已隔离。确认后才会写回。").FontSize(12).TextColor(tMuted(c))
			if ui.Button(c, "写回数据文件").Clicked() {
				if err := a.confirmWriteBack(); err != nil {
					a.settingsUI.keyErr = "写回失败：" + err.Error()
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
		if ui.Button(c, "恢复本页默认值").Clicked() {
			a.confirmResetPage(page)
		}
	})
}
