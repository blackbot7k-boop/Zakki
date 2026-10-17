package app

// 首启初始化向导：仅在全新安装（数据文件不存在）时出现。
// 三步：语言 → 备份文件夹（预填默认路径，可不改）→ 初始标签数量。
// 备份文件夹始终有值：用户所选或默认 ~/Documents/Zakki 备份；Git/云盘不在
// 向导出现（档位默认 off，留在设置页）。选择先存草稿，确认时才写入偏好与
// 数据文件；跳过/关闭等同接受默认。语言步骤即时经 App.SetLocale 生效。

import (
	"log/slog"
	"os"
	"path/filepath"
	"unicode/utf8"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/i18n"
	"github.com/egoist/mygo/ui"

	"zakki/locales"
)

// onboardingLastStep 向导最后一步的序号。
const onboardingLastStep = 3

// defaultMirrorDir 新安装的默认外部备份目录（~/Documents/Zakki 备份）。
// 可注入以便测试；空串表示无法定位主目录，此时向导必须手选文件夹。
var defaultMirrorDir = func() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	return filepath.Join(home, "Documents", "Zakki 备份")
}

// onboardingState 向导的草稿状态（主线程）。
type onboardingState struct {
	obWin    *mygo.Window
	step     int // 1..onboardingLastStep
	finished bool

	dirDraft string // 备份文件夹草稿，预填默认路径（确认时才写入偏好）
	tabCount int
	dirErr   string // 文件夹校验错误
}

// firstRun 全新安装判定：数据文件不存在（载入时的 initDefaults 只写内存）。
func (a *app) firstRun() bool {
	_, err := os.Stat(a.notes.DataFile)
	return os.IsNotExist(err)
}

// openOnboarding 创建并显示向导窗口；主窗口等向导结束再创建。
func (a *app) openOnboarding() {
	a.onboarding = onboardingState{
		step:     1,
		tabCount: DefaultTabCount,
		dirDraft: defaultMirrorDir(),
	}
	win := mygo.NewWindow(mygo.WindowOptions{
		Title:     locales.OnboardWindowTitle(),
		Width:     560,
		Height:    500,
		MinWidth:  520,
		MinHeight: 460,
		StateKey:  "onboarding",
		Content:   ui.View(a.onboardingView),
	})
	a.onboarding.obWin = win
	win.OnClosed(func() {
		a.onboarding.obWin = nil
		if a.onboarding.finished {
			a.onboarding = onboardingState{}
			return
		}
		a.abandonOnboarding() // 直接关窗等同跳过
	})
	win.Show()
}

// abandonOnboarding 放弃向导（跳过/关窗等同接受默认）：备份文件夹落默认路径，
// 数据写默认快照，之后照常进入主界面。
func (a *app) abandonOnboarding() {
	a.applyDefaultMirrorDir()
	if err := a.persistFirstRunSnapshot(); err != nil {
		a.notice = locales.OnboardDraftPendingNotice()
	}
	a.onboarding.finished = true
	a.enterMainFromOnboarding()
}

// applyDefaultMirrorDir 跳过/关窗路径的偏好写入：不选也有默认备份文件夹。
// Git 档位不在向导出现，保持默认 off。
func (a *app) applyDefaultMirrorDir() {
	if err := a.updatePreferences(func(p *preferences) {
		p.Backup.MirrorDir = defaultMirrorDir()
	}); err != nil {
		a.notice = locales.OnboardSaveErr(err.Error())
	}
}

// finishOnboarding 结束向导并应用配置。configured=false 表示跳过（全默认）。
func (a *app) finishOnboarding(configured bool) {
	ob := &a.onboarding
	if configured {
		dir := ob.dirDraft
		n := ob.tabCount
		if n < 1 {
			n = 1
		}
		if n > MaxTabCount {
			n = MaxTabCount
		}
		if err := a.updatePreferences(func(p *preferences) {
			p.Backup.MirrorDir = dir
		}); err != nil {
			ob.dirErr = locales.OnboardSaveErr(err.Error())
			return // 留在向导，让用户重试或跳过
		}
		if err := a.commitFirstRunData(n); err != nil {
			ob.dirErr = locales.OnboardWriteErr(err.Error())
			return // 数据未落盘时不关向导，避免偏好已改但下次仍首启
		}
		if dir != "" {
			if err := a.backupNow(); err != nil {
				slog.Warn("向导完成后首次备份失败", "error", err)
				a.notice = locales.OnboardFirstMirrorErr(err.Error())
			}
		}
	} else {
		// 跳过等同接受默认：备份文件夹落默认路径，再写默认数据快照。
		a.applyDefaultMirrorDir()
		if err := a.persistFirstRunSnapshot(); err != nil {
			ob.dirErr = locales.OnboardWriteErr(err.Error())
			return
		}
	}
	ob.finished = true
	if win := ob.obWin; win != nil {
		win.Close() // OnClosed 里清理状态
	}
	a.enterMainFromOnboarding()
}

// enterMainFromOnboarding 关闭向导后进入主界面（测试环境无事件循环时跳过建窗）。
func (a *app) enterMainFromOnboarding() {
	if a.ready {
		a.createWindow()
	}
	a.syncMenus()
}

// rebuildInitialTabs 按数量重建初始标签：第 1 个含欢迎文本，其余为空。
func (a *app) rebuildInitialTabs(n int) {
	a.tabs = make([]tab, n)
	for i := range a.tabs {
		a.tabs[i] = tab{ID: uint64(i + 1), Color: tabPaletteColors[i]}
	}
	text := welcomeTextFor(n)
	a.tabs[0].Text = text
	a.syncNextTabID()
	a.clampActive()
	a.resyncLive(utf8.RuneCountInString(text))
}

// obPickFolder 向导里的目录选择：结果回主线程应用到草稿。
func (a *app) obPickFolder() {
	a.chooseMirrorDirAsync(a.onboarding.obWin, a.obApplyFolder)
}

// obApplyFolder 主线程：校验并记录所选文件夹（覆盖默认路径）。
func (a *app) obApplyFolder(path string) {
	abs, err := a.validateMirrorDir(path)
	if err != nil {
		a.onboarding.dirErr = err.Error()
		return
	}
	a.onboarding.dirDraft = abs
	a.onboarding.dirErr = ""
}

// obNextLabel 底栏主按钮文案。
func (a *app) obNextLabel() string {
	if a.onboarding.step >= onboardingLastStep {
		return locales.OnboardFinish()
	}
	return locales.OnboardNext()
}

// onboardingView 向导窗口每帧构建。
func (a *app) onboardingView(c *ui.Context) {
	dark := themeIsDark(c.Theme())
	style := a.resolveStyle(-1, dark, true)
	t := *c.Theme()
	t.Background, t.Text, t.TextMuted = style.background, style.text, style.textMuted
	t.Surface, t.Border, t.Accent = style.surface, style.border, style.accent
	t.Font, t.FontSize = bodyFontStack("pingfang", ""), 13
	c.SetTheme(&t)

	ob := &a.onboarding
	if ob.step < 1 || ob.step > onboardingLastStep {
		ob.step = 1
	}
	if ob.tabCount < 1 {
		ob.tabCount = 1
	}
	if ob.tabCount > MaxTabCount {
		ob.tabCount = MaxTabCount
	}

	ui.Column(c).Fill().Padding(28, 32).Gap(16).Children(func() {
		ui.Text(c, locales.OnboardStep(ob.step, onboardingLastStep)).
			FontSize(11).Bold().TextColor(tMuted(c))
		switch ob.step {
		case 1:
			a.obLanguage(c, &t)
		case 2:
			a.obFolder(c, &t)
		default:
			a.obTabs(c)
		}
		ui.Box(c).Grow(1)
		if ob.dirErr != "" {
			ui.Text(c, ob.dirErr).FontSize(12).TextColor(t.Danger)
		}
		ui.Row(c).FillWidth().Gap(10).AlignItems(ui.Center).Children(func() {
			if ob.step < onboardingLastStep {
				if ui.Button(c, locales.OnboardSkip()).Clicked() {
					a.finishOnboarding(false)
				}
			}
			ui.Box(c).Grow(1)
			if ob.step > 1 {
				if ui.Button(c, locales.OnboardBack()).Clicked() {
					ob.step--
				}
			}
			if ui.Button(c, a.obNextLabel()).Clicked() {
				if ob.step >= onboardingLastStep {
					a.finishOnboarding(true)
				} else {
					ob.step++
				}
			}
		})
	})
}

// obLanguage 步骤 1：选择界面语言（即时生效，选择会被 MyGo 记住）。
func (a *app) obLanguage(c *ui.Context, t *ui.Theme) {
	ui.Column(c).FillWidth().Gap(12).Children(func() {
		ui.Text(c, locales.OnboardLanguageTitle()).FontSize(22).Bold()
		ui.Text(c, locales.OnboardLanguageSub()).TextColor(tMuted(c))
		ui.Row(c).Gap(10).Children(func() {
			for _, tag := range mygo.App.Languages() {
				tag := tag
				chosen := mygo.App.Locale() == tag
				btn := ui.ButtonBase(c.Key("lang-"+tag)).Label(i18n.Autonym(tag)).
					Padding(14, 10).Radius(10).Cursor(ui.CursorPointer)
				if chosen {
					btn = btn.BorderWidth(2).BorderColor(t.Accent)
				}
				// Label 只是可访问性标题，ButtonBase 不会绘制它；可见文字需要 Text 子元素。
				btn.Children(func() {
					ui.Text(c, i18n.Autonym(tag))
				})
				btn.OnClick(func() {
					// 经统一偏好入口：写 appearance.language 并即时应用，
					// 设置页的语言单选与本步骤保持同一事实来源。
					a.updatePreferences(func(p *preferences) { p.Appearance.Language = tag })
				})
			}
		})
	})
}

// obFolder 步骤 2：备份文件夹。预填默认路径，用户可换；Git/云盘不在向导出现。
func (a *app) obFolder(c *ui.Context, t *ui.Theme) {
	ob := &a.onboarding
	ui.Column(c).FillWidth().Gap(10).Children(func() {
		ui.Text(c, locales.OnboardChooseBackupTitle()).FontSize(17).Bold()
		ui.Text(c, locales.OnboardChooseBackupBody()).
			FontSize(12).TextColor(tMuted(c))
		ui.Column(c).FillWidth().Gap(6).Padding(12, 14).Radius(8).
			Background(t.Surface).BorderWidth(1).BorderColor(t.Border).Children(func() {
			ui.Text(c, locales.OnboardFolderCap()).FontSize(12).TextColor(tMuted(c))
			ui.Text(c, ob.dirDraft).FontSize(11).Font(monoFontStack("sfmono"))
			ui.Row(c).Gap(8).Children(func() {
				label := locales.CommonChange()
				if ob.dirDraft == "" {
					label = locales.ChooseFolder()
				}
				if ui.Button(c, label).Clicked() {
					a.obPickFolder()
				}
			})
		})
	})
}

// obTabs 步骤 4：初始标签数量。
func (a *app) obTabs(c *ui.Context) {
	ob := &a.onboarding
	ui.Column(c).FillWidth().Gap(10).Children(func() {
		ui.Text(c, locales.OnboardTabCountTitle()).FontSize(17).Bold()
		ui.Text(c, locales.OnboardTabCountBody(MaxTabCount)).
			FontSize(12).TextColor(tMuted(c))
		ui.Row(c).Gap(14).AlignItems(ui.Center).Children(func() {
			if ui.Button(c, "−").Clicked() {
				if ob.tabCount > 1 {
					ob.tabCount--
				}
			}
			ui.Text(c, locales.OnboardTabCountN(ob.tabCount)).FontSize(18).Bold()
			if ui.Button(c, "+").Clicked() {
				if ob.tabCount < MaxTabCount {
					ob.tabCount++
				}
			}
		})
		ui.Row(c).Gap(8).Children(func() {
			for i := 0; i < ob.tabCount; i++ {
				ui.Box(c).Width(14).Height(14).Radius(999).Background(ui.Hex(tabPaletteColors[i]))
			}
		})
	})
}
