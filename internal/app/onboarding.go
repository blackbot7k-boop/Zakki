package app

import (
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"zakki/locales"
)

const onboardingLastStep = 2

var defaultBackupDir = func() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	return filepath.Join(home, "Documents", "Zakki 备份")
}

type onboardingState struct {
	obWin                                *mygo.Window
	step                                 int
	phase                                string
	finished                             bool
	dirDraft                             string
	tabCount                             int
	tabCountText                         string
	dirErr                               string
	appearance                           appearancePrefs
	languageIndex, modeIndex, styleIndex int
	preparation                          fontPreparation
}

func (a *app) firstRun() bool {
	_, err := os.Stat(a.notes.DataFile)
	return os.IsNotExist(err)
}

func (a *app) initOnboarding() {
	ap := a.prefs.Appearance
	ap.BodyFont, ap.LatinFont = "pingfang", ""
	a.onboarding = onboardingState{step: 1, tabCount: DefaultTabCount, tabCountText: strconv.Itoa(DefaultTabCount), dirDraft: defaultBackupDir(), appearance: ap}
	if mygo.App.Locale() == "zh-Hans" {
		a.onboarding.languageIndex = 1
	}
	switch ap.AppearanceMode {
	case "light":
		a.onboarding.modeIndex = 1
	case "dark":
		a.onboarding.modeIndex = 2
	}
	if ap.MarkdownTheme == "sketch" {
		a.onboarding.styleIndex = 1
	}
}

func (a *app) openOnboarding() {
	a.initOnboarding()
	win := mygo.NewWindow(mygo.WindowOptions{Title: locales.OnboardWindowTitle(), Width: 640, Height: 560, MinWidth: 520, MinHeight: 500, StateKey: "onboarding-v2", Content: ui.View(a.onboardingView)})
	a.onboarding.obWin = win
	win.OnClose(func(e *mygo.CloseEvent) {
		if a.onboarding.finished {
			return
		}
		e.PreventDefault()
		if a.onboarding.phase == "saving" {
			return
		}
		a.onboarding.preparation.stop()
		a.finishOnboarding(false)
	})
	win.OnClosed(func() {
		a.onboarding.preparation.stop()
		a.onboarding.obWin = nil
	})
	a.loadFontFamilies(win)
	win.Show()
}

func (a *app) abandonOnboarding() { a.onboarding.preparation.stop(); a.finishOnboarding(false) }

func (a *app) applyDefaultBackupDir() {
	if err := a.updatePreferences(func(p *preferences) { p.Backup.BackupDir = defaultBackupDir() }); err != nil {
		a.notice = locales.OnboardSaveErr(err.Error())
	}
}

// 下载与注册成功只进入就绪页，必须由开始按钮显式提交。
func (a *app) submitOnboarding() {
	ob := &a.onboarding
	if ob.phase == "saving" || ob.preparation.running {
		return
	}
	a.commitTabCountText()
	dir, err := a.validateBackupDir(ob.dirDraft)
	if err != nil {
		ob.dirErr = err.Error()
		return
	}
	ob.dirDraft, ob.dirErr = dir, ""
	ob.phase = "preparing"
	a.prepareFonts(ob.obWin, &ob.preparation, []string{ob.appearance.BodyFont, ob.appearance.LatinFont}, func(ok bool) {
		if ok {
			ob.phase = "ready"
		}
	})
}

func (a *app) editOnboarding() {
	a.onboarding.preparation.stop()
	a.onboarding.phase = ""
	a.onboarding.dirErr = ""
}

func (a *app) useSystemOnboardingFonts() {
	ob := &a.onboarding
	ob.preparation.stop()
	ob.appearance.BodyFont, ob.appearance.LatinFont = "pingfang", ""
	ob.preparation.err = ""
	ob.phase = "ready"
}

func (a *app) finishOnboarding(configured bool) {
	ob := &a.onboarding
	if ob.phase == "saving" || (configured && ob.phase != "ready") {
		return
	}
	n := DefaultTabCount
	ap := a.prefs.Appearance
	dir := defaultBackupDir()
	if configured {
		n = ob.tabCount
		ap = ob.appearance
		dir = ob.dirDraft
		// 未经过 initOnboarding 的零值草稿（测试直调）按默认枚举补齐，
		// 字体键保持空串语义（默认栈 / 跟随中文字体）。
		if ap.AppearanceMode == "" {
			ap.AppearanceMode = "system"
		}
		if ap.ColorTheme == "" {
			ap.ColorTheme = "classic"
		}
		if ap.MarkdownTheme == "" {
			ap.MarkdownTheme = "sonnet"
		}
		if ap.CodeTheme == "" {
			ap.CodeTheme = "soft"
		}
		if ap.CodeIndent == "" {
			ap.CodeIndent = "space2"
		}
	}
	if err := a.updatePreferences(func(p *preferences) { p.Backup.BackupDir = dir; p.Appearance = ap }); err != nil {
		ob.dirErr = locales.OnboardSaveErr(err.Error())
		return
	}
	if n < 1 {
		n = 1
	}
	if n > MaxTabCount {
		n = MaxTabCount
	}
	if a.firstRun() {
		a.rebuildInitialTabs(n)
	}
	ob.phase = "saving"
	prefsSnapshot := clonePrefs(a.prefs)
	notesSnapshot := a.snapshotFile()
	prefStore, noteStore := a.prefsStore, a.notes
	persist := func() (error, error) {
		if err := prefStore.SaveNow(prefsSnapshot); err != nil {
			return err, nil
		}
		if err := noteStore.SaveNow(notesSnapshot); err != nil {
			return err, nil
		}
		if dir != "" {
			return nil, noteStore.BackupNow(prefsSnapshot.Backup.Keep)
		}
		return nil, nil
	}
	savingPhase := ob.phase
	complete := func(err, backupErr error) {
		if err != nil {
			ob.phase = savingPhase
			ob.dirErr = locales.OnboardWriteErr(err.Error())
			return
		}
		if backupErr != nil {
			slog.Warn("向导首次备份失败", "error", backupErr)
			a.notice = locales.OnboardFirstBackupErr(backupErr.Error())
		}
		ob.finished = true
		if win := ob.obWin; win != nil {
			win.Close()
		}
		a.enterMainFromOnboarding()
	}
	if win := ob.obWin; win != nil && a.ready {
		go func() {
			err, backupErr := persist()
			win.Update(func() {
				if a.onboarding.obWin == win {
					complete(err, backupErr)
				}
			})
		}()
	} else {
		err, backupErr := persist()
		complete(err, backupErr)
	}
}

func (a *app) enterMainFromOnboarding() {
	if a.ready {
		a.createWindow()
	}
	a.syncMenus()
}

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

func (a *app) obPickFolder() { a.chooseBackupDirAsync(a.onboarding.obWin, a.obApplyFolder) }
func (a *app) obApplyFolder(path string) {
	abs, err := a.validateBackupDir(path)
	if err != nil {
		a.onboarding.dirErr = err.Error()
		return
	}
	a.onboarding.dirDraft, a.onboarding.dirErr = abs, ""
}

func (a *app) obNextLabel() string {
	if a.onboarding.step >= onboardingLastStep {
		if len(a.fontJobs([]string{a.onboarding.appearance.BodyFont, a.onboarding.appearance.LatinFont})) > 0 {
			return locales.OnboardDownloadFonts()
		}
		return locales.OnboardCompleteSetup()
	}
	return locales.OnboardNext()
}

func onAccentColor(t ui.Theme) ui.Color {
	if themeIsDark(&t) {
		return t.Background
	}
	return ui.Hex("#ffffff")
}

func (a *app) setTabCount(n int) {
	if n < 1 {
		n = 1
	}
	if n > MaxTabCount {
		n = MaxTabCount
	}
	a.onboarding.tabCount, a.onboarding.tabCountText = n, strconv.Itoa(n)
}
func (a *app) commitTabCountText() {
	n, err := strconv.Atoi(strings.TrimSpace(a.onboarding.tabCountText))
	if err != nil {
		a.onboarding.tabCountText = strconv.Itoa(a.onboarding.tabCount)
		return
	}
	a.setTabCount(n)
}
