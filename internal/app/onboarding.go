package app

// 首启初始化向导：仅在全新安装（数据文件不存在）时出现。
// 五步：欢迎 → 备份文件夹（可跳过）→ 初始标签数量 → Git（可选）→ 完成。
// 所有选择先存草稿，确认时才写入偏好与数据文件；关闭窗口等同跳过。

import (
	"log/slog"
	"os"
	"unicode/utf8"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// onboardingLastStep 向导最后一步的序号。
const onboardingLastStep = 5

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
		Title:     "欢迎使用 Zakki",
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
		a.notice = "首启数据未能保存，下次启动可能再次显示初始化向导"
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
			ob.dirErr = "保存设置失败：" + err.Error()
			return // 留在向导，让用户重试或跳过
		}
		if err := a.commitFirstRunData(n); err != nil {
			ob.dirErr = "数据写入失败：" + err.Error()
			return // 数据未落盘时不关向导，避免偏好已改但下次仍首启
		}
		if dir != "" {
			if err := a.backupNow(); err != nil {
				slog.Warn("向导完成后首次备份失败", "error", err)
				a.notice = "数据已保存，但首次外部备份失败：" + err.Error()
			}
		}
	} else if err := a.persistFirstRunSnapshot(); err != nil {
		ob.dirErr = "数据写入失败：" + err.Error()
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
		return "完成，进入 Zakki"
	}
	return "下一步"
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
		ui.Textf(c, "初始设置 · 步骤 %d / %d", ob.step, onboardingLastStep).
			FontSize(11).Bold().TextColor(tMuted(c))
		switch ob.step {
		case 1:
			a.obWelcome(c, &t)
		case 2:
			a.obFolder(c, &t)
		case 3:
			a.obTabs(c)
		case 4:
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
				if ui.Button(c, "跳过，使用默认设置").Clicked() {
					a.finishOnboarding(false)
				}
			}
			ui.Box(c).Grow(1)
			if ob.step > 1 {
				if ui.Button(c, "上一步").Clicked() {
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

// obWelcome 步骤 1：欢迎。
func (a *app) obWelcome(c *ui.Context, t *ui.Theme) {
	ui.Column(c).FillWidth().Gap(10).Children(func() {
		ui.Text(c, "欢迎使用 Zakki").FontSize(22).Bold()
		ui.Text(c, "一分钟完成初始设置；笔记数据始终优先保存在本机。").TextColor(tMuted(c))
		ui.Box(c).Height(6)
		obBullet(c, "本地优先", "数据与本地备份永远先落盘，外部副本失败不影响使用。")
		obBullet(c, "云盘备份", "备份目录选在 iCloud、坚果云等同步文件夹内即自动上云。")
		obBullet(c, "Git 备份（可选）", "目录选在 Git 仓库里，可开启自动提交甚至推送；不配置也完全可用。")
	})
}

// obFolder 步骤 2：备份文件夹（可跳过）。
func (a *app) obFolder(c *ui.Context, t *ui.Theme) {
	ob := &a.onboarding
	ui.Column(c).FillWidth().Gap(10).Children(func() {
		ui.Text(c, "选择备份文件夹").FontSize(17).Bold()
		ui.Text(c, "每次备份会镜像一份到这里：云盘同步文件夹即云备份；GitHub 仓库的本地 clone 可版本化。跳过则仅保留本地备份，之后可在设置里配置。").
			FontSize(12).TextColor(tMuted(c))
		if ob.dirDraft == "" {
			if ui.Button(c, "选择文件夹…").Clicked() {
				a.obPickFolder()
			}
			return
		}
		kind := "文件夹"
		if ob.dirIsGit {
			kind = "Git 仓库"
		}
		ui.Column(c).FillWidth().Gap(6).Padding(12, 14).Radius(8).
			Background(t.Surface).BorderWidth(1).BorderColor(t.Border).Children(func() {
			ui.Text(c, ob.dirDraft).FontSize(11).Font(monoFontStack("sfmono")).TextColor(tMuted(c))
			ui.Textf(c, "%s · 每次备份时镜像一份到这里", kind)
			ui.Row(c).Gap(8).Children(func() {
				if ui.Button(c, "更换…").Clicked() {
					a.obPickFolder()
				}
				if ui.Button(c, "断开").Clicked() {
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
		ui.Text(c, "初始标签数量").FontSize(17).Bold()
		ui.Textf(c, "决定首次打开时的标签页个数（1–%d 个）；以后随时可在设置里增删。", MaxTabCount).
			FontSize(12).TextColor(tMuted(c))
		ui.Row(c).Gap(14).AlignItems(ui.Center).Children(func() {
			if ui.Button(c, "−").Clicked() {
				if ob.tabCount > 1 {
					ob.tabCount--
				}
			}
			ui.Textf(c, "%d 个", ob.tabCount).FontSize(18).Bold()
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
		ui.Text(c, "Git 自动备份（可选）").FontSize(17).Bold()
		if ob.dirDraft == "" || !ob.dirIsGit {
			ui.Text(c, "Git 自动化仅对 Git 仓库目录可用。当前所选文件夹"+gitFolderState(ob)+
				"；作为云盘同步文件夹使用没有问题，想用 Git 备份请返回上一步更换目录。").
				FontSize(12).TextColor(tMuted(c))
			return
		}
		ui.Text(c, "检测到备份文件夹是 Git 仓库。可让应用在每次备份写入后自动提交（可选推送）；需要本机已安装 git，失败只提示、不影响备份本身。").
			FontSize(12).TextColor(tMuted(c))
		if ui.Select(c, &ob.gitModeSel, gitModeOptions).Changed() {
			ob.gitMode = gitModeOfDisplay(ob.gitModeSel)
		}
	})
}

func gitFolderState(ob *onboardingState) string {
	if ob.dirDraft == "" {
		return "尚未选择"
	}
	return "不是 Git 仓库"
}

// obDone 步骤 5：配置汇总。
func (a *app) obDone(c *ui.Context) {
	ob := &a.onboarding
	ui.Column(c).FillWidth().Gap(10).Children(func() {
		ui.Text(c, "准备就绪").FontSize(17).Bold()
		obDoneRow(c, "本地备份", "启动 + 编辑节流 + ⌘S 立即备份")
		if ob.dirDraft != "" {
			kind := "文件夹"
			if ob.dirIsGit {
				kind = "Git 仓库"
			}
			obDoneRow(c, "外部备份", kind+" · "+ob.dirDraft)
			obDoneRow(c, "Git 自动化", gitModeDisplay(ob.gitMode))
		} else {
			obDoneRow(c, "外部备份", "未配置（仅本地备份，可随时在设置里补配）")
		}
		ui.Textf(c, "初始标签 %d 个", ob.tabCount).FontSize(12).TextColor(tMuted(c))
		if ob.dirDraft != "" {
			if ui.Button(c, "打开备份文件夹").Clicked() {
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
