package app

import (
	"strconv"

	"github.com/egoist/mygo/ui"
	"zakki/locales"
)

func (a *app) onboardingView(c *ui.Context) {
	ob := &a.onboarding
	dark := themeIsDark(c.Theme())
	if ob.appearance.AppearanceMode == "light" {
		dark = false
	}
	if ob.appearance.AppearanceMode == "dark" {
		dark = true
	}
	pal := colorPaletteFor(ob.appearance.ColorTheme, -1, dark)
	t := *c.Theme()
	t.Background, t.Text, t.TextMuted = pal.background, pal.text, pal.textMuted
	t.Surface, t.Border, t.Accent = pal.surface, pal.border, pal.accent
	t.Font, t.FontSize = bodyFontStack("pingfang", ""), 13
	c.SetTheme(&t)
	ui.Column(c).Fill().Padding(18, 32, 24, 32).Gap(16).Background(t.Background).Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(8).Children(func() {
			label := locales.OnboardStep(ob.step, onboardingLastStep)
			switch ob.phase {
			case "preparing":
				label = locales.FontPrepareTitle()
			case "ready":
				label = locales.OnboardReady()
			case "saving":
				label = locales.OnboardSaving()
			}
			ui.Text(c, label).FontSize(11).TextColor(tMuted(c)).Grow(1)
			if ob.phase == "" {
				for i := 1; i <= onboardingLastStep; i++ {
					ink := t.Border
					if i == ob.step {
						ink = t.Accent
					}
					ui.Box(c.Key("ob-dot-"+strconv.Itoa(i))).Size(5, 5).Radius(999).Background(ink)
				}
			}
		})
		ui.Scroll(c).FillWidth().Grow(1).Children(func() {
			ui.Column(c).FillWidth().Gap(14).Children(func() {
				switch ob.phase {
				case "preparing":
					ui.Text(c, locales.FontPrepareTitle()).FontSize(23).Bold()
					fontProgressView(c, &ob.preparation)
				case "ready", "saving":
					ui.Column(c).FillWidth().Height(290).Center().Gap(16).Children(func() {
						if ob.phase == "ready" {
							ui.Text(c, "✓").FontSize(30)
							ui.Text(c, locales.OnboardReady()).FontSize(23).Bold()
						} else {
							ui.Text(c, locales.OnboardSaving()).FontSize(23).Bold()
						}
					})
				default:
					if ob.step == 1 {
						a.obAppearance(c)
					} else {
						a.obNotes(c)
					}
				}
				if ob.dirErr != "" {
					ui.Text(c, ob.dirErr).FontSize(12).TextColor(t.Danger)
				}
			})
		})
		a.obFooter(c)
	})
}

func obRow(c *ui.Context, label string, control func()) {
	ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(16).Children(func() {
		ui.Text(c, label).Grow(1)
		ui.Column(c).Width(244).Children(control)
	})
}

func (a *app) obAppearance(c *ui.Context) {
	ob := &a.onboarding
	ui.Text(c, locales.OnboardAppearanceTitle()).FontSize(23).Bold()
	ui.Column(c).FillWidth().Gap(14).Padding(12, 0, 0, 0).Children(func() {
		obRow(c, locales.LanguageRow(), func() {
			ui.Segmented(c.Key("ob-language"), &ob.languageIndex, "English", "简体中文").FillWidth().Label(locales.LanguageRow()).OnChange(func() {
				tag := "en"
				if ob.languageIndex == 1 {
					tag = "zh-Hans"
				}
				ob.appearance.Language = tag
				if err := a.updatePreferences(func(p *preferences) { p.Appearance.Language = tag }); err != nil {
					ob.dirErr = locales.OnboardSaveErr(err.Error())
				}
			})
		})
		obRow(c, locales.OnboardMode(), func() {
			ui.Segmented(c.Key("ob-mode"), &ob.modeIndex, locales.ModeFollowSystem(), locales.ModeLight(), locales.ModeDark()).FillWidth().Label(locales.OnboardMode()).OnChange(func() {
				ob.appearance.AppearanceMode = []string{"system", "light", "dark"}[ob.modeIndex]
			})
		})
		obRow(c, locales.OnboardStyle(), func() {
			ui.Segmented(c.Key("ob-style"), &ob.styleIndex, locales.OnboardDefaultStyle(), locales.OnboardJournalStyle()).FillWidth().Label(locales.OnboardStyle()).OnChange(func() {
				ob.appearance.ColorTheme, ob.appearance.MarkdownTheme = "classic", "sonnet"
				if ob.styleIndex == 1 {
					ob.appearance.ColorTheme, ob.appearance.MarkdownTheme = "sketch", "sketch"
				}
			})
		})
		obRow(c, locales.FontChinese(), func() { a.fontPicker(c.Key("ob-cjk"), &ob.appearance.BodyFont, 0, locales.FontChinese(), 244) })
		obRow(c, locales.FontWestern(), func() { a.fontPicker(c.Key("ob-latin"), &ob.appearance.LatinFont, 1, locales.FontWestern(), 244) })
		if !a.fonts.loaded && a.ready {
			ui.Text(c, locales.FontScanning()).FontSize(11).TextColor(tMuted(c))
		}
	})
}

func (a *app) obNotes(c *ui.Context) {
	ob := &a.onboarding
	ui.Text(c, locales.OnboardNotesTitle()).FontSize(23).Bold()
	ui.Column(c).FillWidth().Gap(28).Padding(12, 0, 0, 0).Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(12).Children(func() {
			ui.Text(c, locales.OnboardTabCountTitle()).Grow(1)
			ui.Text(c, locales.OnboardTabRange(MaxTabCount)).FontSize(12).TextColor(tMuted(c))
			ui.Button(c.Key("ob-minus"), "−").Label(locales.OnboardFewerTabs()).Disabled(ob.tabCount <= 1).OnClick(func() { a.setTabCount(ob.tabCount - 1) })
			ui.TextInput(c.Key("ob-count"), &ob.tabCountText).Width(40).Label(locales.OnboardTabCountTitle()).OnChange(a.commitTabCountText)
			ui.Button(c.Key("ob-plus"), "+").Label(locales.OnboardMoreTabs()).Disabled(ob.tabCount >= MaxTabCount).OnClick(func() { a.setTabCount(ob.tabCount + 1) })
		})
		ui.Column(c).FillWidth().Gap(10).Children(func() {
			ui.Text(c, locales.OnboardChooseBackupTitle()).Bold()
			ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(14).Children(func() {
				ui.Text(c, ob.dirDraft).FontSize(12).Grow(1)
				ui.Button(c, locales.CommonChange()).OnClick(a.obPickFolder)
			})
		})
	})
}

func (a *app) obFooter(c *ui.Context) {
	ob := &a.onboarding
	ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(10).Children(func() {
		switch ob.phase {
		case "preparing":
			ui.Button(c, locales.FontEditChoices()).OnClick(a.editOnboarding)
			ui.Box(c).Grow(1)
			if ob.preparation.err != "" {
				ui.Button(c, locales.FontUseSystem()).OnClick(a.useSystemOnboardingFonts)
				ui.PrimaryButton(c, locales.FontRetry()).OnClick(a.submitOnboarding)
			} else {
				ui.PrimaryButton(c, locales.FontPreparing()).Disabled(true)
			}
		case "ready":
			ui.Button(c, locales.FontEditChoices()).OnClick(a.editOnboarding)
			ui.Box(c).Grow(1)
			ui.PrimaryButton(c, locales.OnboardStartUsing()).OnClick(func() { a.finishOnboarding(true) })
		case "saving":
			ui.Box(c).Grow(1)
			ui.PrimaryButton(c, locales.OnboardSaving()).Disabled(true)
		default:
			if ob.step > 1 {
				ui.Button(c, locales.OnboardBack()).OnClick(func() { ob.step-- })
			}
			ui.Box(c).Grow(1)
			ui.PrimaryButton(c, a.obNextLabel()).Disabled(!a.fonts.loaded && a.ready).OnClick(func() {
				if ob.step == 1 {
					ob.step = 2
				} else {
					a.submitOnboarding()
				}
			})
		}
	})
}
