package app

// 首启初始化向导：仅在全新安装（数据文件不存在）时出现。
// 六步：语言 → 欢迎 → 备份文件夹（可跳过）→ 初始标签数量 → Git（可选）→ 完成。
// 所有选择先存草稿，确认时才写入偏好与数据文件；关闭窗口等同跳过。
// 语言步骤即时经 App.SetLocale 生效（MyGo 持久化选择），后续步骤随之换语言。

import (
	"log/slog"
	"os"
	"unicode/utf8"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/i18n"
	"github.com/egoist/mygo/ui"

	"zakki/locales"
)

// onboardingLastStep 向导最后一步的序号。
const onboardingLastStep = 6

// onboardingState 向导的草稿状态（主线程）。
type onboardingState struct {
	obWin    *mygo.Window
	step     int // 1..onboardingLastStep
	finished bool

	dirDraft   string // 已选备份文件夹（确认时才写入偏好）
	dirIsGit   bool   // dirDraft 是否检测到 .git
	tabCount   int
	gitMode    string // off / commit / commitPush
	gitModeSel string // Git 档位 Select 绑定的显示文案（持久字段，避免丢编辑）
	dirErr     string // 文件夹校验错误
}

// firstRun 全新安装判定：数据文件不存在（载入时的 initDefaults 只写内存）。
func (a *app) firstRun() bool {
	_, err := os.Stat(a.notes.DataFile)
	return os.IsNotExist(err)
}

// openOnboarding 创建并显示向导窗口；主窗口等向导结束再创建。
func (a *app) openOnboarding() {
	a.onboarding = onboardingState{
		step:       1,
		tabCount:   DefaultTabCount,
		gitMode:    "off",
		gitModeSel: gitModeDisplay("off"),
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

// abandonOnboarding 放弃向导：保持默认值创建主窗口。
func (a *app) abandonOnboarding() {
	if err := a.persistFirstRunSnapshot(); err != nil {
		a.notice = locales.OnboardDraftPendingNotice()
	}
	a.onboarding.finished = true
	a.enterMainFromOnboarding()
}

// finishOnboarding 结束向导并应用配置。configured=false 表示跳过（全默认）。
func (a *app) finishOnboarding(configured bool) {
	ob := &a.onboarding
	if configured {
		dir, mode := ob.dirDraft, ob.gitMode
		if dir == "" {
			mode = "off"
		}
		n := ob.tabCount
		if n < 1 {
			n = 1
		}
		if n > MaxTabCount {
			n = MaxTabCount
		}
		if err := a.updatePreferences(func(p *preferences) {
			p.Backup.MirrorDir = dir
			p.Backup.GitMode = mode
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
	} else if err := a.persistFirstRunSnapshot(); err != nil {
		ob.dirErr = locales.OnboardWriteErr(err.Error())
		return
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

// obApplyFolder 主线程：校验并记录所选文件夹，非 Git 仓库时联动关闭 Git。
func (a *app) obApplyFolder(path string) {
	abs, err := a.validateMirrorDir(path)
	if err != nil {
		a.onboarding.dirErr = err.Error()
		return
	}
	ob := &a.onboarding
	ob.dirDraft = abs
	ob.dirIsGit = dirIsGitRepo(abs)
	ob.dirErr = ""
	if !ob.dirIsGit {
		ob.gitMode, ob.gitModeSel = "off", gitModeDisplay("off")
	}
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
			a.obWelcome(c, &t)
		case 3:
			a.obFolder(c, &t)
		case 4:
			a.obTabs(c)
		case 5:
			a.obGit(c)
		default:
			a.obDone(c)
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

// obBullet 向导要点条目。
func obBullet(c *ui.Context, title, detail string) {
	ui.Row(c).FillWidth().Gap(10).AlignItems(ui.Center).Children(func() {
		ui.Box(c).Width(7).Height(7).Radius(999).Background(c.Theme().Accent)
		ui.Column(c).Gap(1).Children(func() {
			ui.Text(c, title).Bold()
			ui.Text(c, detail).FontSize(12).TextColor(tMuted(c))
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
				btn.OnClick(func() {
					// 经统一偏好入口：写 appearance.language 并即时应用，
					// 设置页的语言单选与本步骤保持同一事实来源。
					a.updatePreferences(func(p *preferences) { p.Appearance.Language = tag })
				})
			}
		})
	})
}

// obWelcome 步骤 2：欢迎。
func (a *app) obWelcome(c *ui.Context, t *ui.Theme) {
	ui.Column(c).FillWidth().Gap(10).Children(func() {
		ui.Text(c, locales.OnboardWelcomeTitle()).FontSize(22).Bold()
		ui.Text(c, locales.OnboardWelcomeSub()).TextColor(tMuted(c))
		ui.Box(c).Height(6)
		obBullet(c, locales.OnboardBulletLocalTitle(), locales.OnboardBulletLocalBody())
		obBullet(c, locales.OnboardBulletCloudTitle(), locales.OnboardBulletCloudBody())
		obBullet(c, locales.OnboardBulletGitTitle(), locales.OnboardBulletGitBody())
	})
}

// obFolder 步骤 2：备份文件夹（可跳过）。
func (a *app) obFolder(c *ui.Context, t *ui.Theme) {
	ob := &a.onboarding
	ui.Column(c).FillWidth().Gap(10).Children(func() {
		ui.Text(c, locales.OnboardChooseBackupTitle()).FontSize(17).Bold()
		ui.Text(c, locales.OnboardChooseBackupBody()).
			FontSize(12).TextColor(tMuted(c))
		if ob.dirDraft == "" {
			if ui.Button(c, locales.ChooseFolder()).Clicked() {
				a.obPickFolder()
			}
			return
		}
		kind := locales.CommonFolder()
		if ob.dirIsGit {
			kind = locales.CommonGitRepo()
		}
		ui.Column(c).FillWidth().Gap(6).Padding(12, 14).Radius(8).
			Background(t.Surface).BorderWidth(1).BorderColor(t.Border).Children(func() {
			ui.Text(c, ob.dirDraft).FontSize(11).Font(monoFontStack("sfmono")).TextColor(tMuted(c))
			ui.Text(c, locales.MirrorGitDetailLine(kind))
			ui.Row(c).Gap(8).Children(func() {
				if ui.Button(c, locales.CommonChange()).Clicked() {
					a.obPickFolder()
				}
				if ui.Button(c, locales.CommonDisconnect()).Clicked() {
					ob.dirDraft, ob.dirIsGit = "", false
					ob.gitMode, ob.gitModeSel = "off", gitModeDisplay("off")
				}
			})
		})
	})
}

// obTabs 步骤 3：初始标签数量。
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

// obGit 步骤 4：Git 自动化（可选，仅 Git 仓库目录可用）。
func (a *app) obGit(c *ui.Context) {
	ob := &a.onboarding
	ui.Column(c).FillWidth().Gap(10).Children(func() {
		ui.Text(c, locales.OnboardGitTitle()).FontSize(17).Bold()
		if ob.dirDraft == "" || !ob.dirIsGit {
			ui.Text(c, locales.OnboardGitNotRepoBody(gitFolderState(ob))).
				FontSize(12).TextColor(tMuted(c))
			return
		}
		ui.Text(c, locales.OnboardGitRepoBody()).
			FontSize(12).TextColor(tMuted(c))
		if ui.Select(c, &ob.gitModeSel, gitModeOptions()).Changed() {
			ob.gitMode = gitModeOfDisplay(ob.gitModeSel)
		}
	})
}

func gitFolderState(ob *onboardingState) string {
	if ob.dirDraft == "" {
		return locales.CommonNotSet()
	}
	return locales.OnboardGitNotRepo()
}

// obDone 步骤 5：配置汇总。
func (a *app) obDone(c *ui.Context) {
	ob := &a.onboarding
	ui.Column(c).FillWidth().Gap(10).Children(func() {
		ui.Text(c, locales.OnboardDoneTitle()).FontSize(17).Bold()
		obDoneRow(c, locales.OnboardDoneLocal(), locales.OnboardDoneLocalBody())
		if ob.dirDraft != "" {
			kind := locales.CommonFolder()
			if ob.dirIsGit {
				kind = locales.CommonGitRepo()
			}
			obDoneRow(c, locales.OnboardDoneExternal(), locales.OnboardDoneExternalBody(kind, ob.dirDraft))
			obDoneRow(c, locales.OnboardDoneGit(), gitModeDisplay(ob.gitMode))
		} else {
			obDoneRow(c, locales.OnboardDoneExternal(), locales.OnboardDoneNone())
		}
		ui.Text(c, locales.OnboardInitialTabs(ob.tabCount)).FontSize(12).TextColor(tMuted(c))
		if ob.dirDraft != "" {
			if ui.Button(c, locales.OnboardOpenBackupFolder()).Clicked() {
				_ = mygo.Shell.OpenPath(ob.dirDraft)
			}
		}
	})
}

func obDoneRow(c *ui.Context, key, value string) {
	ui.Row(c).FillWidth().Gap(10).Children(func() {
		ui.Text(c, key).TextColor(tMuted(c)).FontSize(12)
		ui.Text(c, "·")
		ui.Text(c, value).FontSize(12).Grow(1)
	})
}
