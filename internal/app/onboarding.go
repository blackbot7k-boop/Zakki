package app

import (
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
	if a.notes.SaveBlocked() {
		return false
	}
	_, err := os.Stat(a.notes.DataPath())
	return os.IsNotExist(err)
}

func (a *app) initOnboarding() {
	ap := a.prefs.Appearance
	ap.BodyFont, ap.LatinFont = "pingfang", "jetbrains"
	a.onboarding = onboardingState{step: 1, tabCount: DefaultTabCount, tabCountText: strconv.Itoa(DefaultTabCount), dirDraft: filepath.Dir(a.notes.DataPath()), appearance: ap}
	if mygo.App.Locale() == "zh-Hans" {
		a.onboarding.languageIndex = 1
	}
	switch ap.AppearanceMode {
	case "light":
		a.onboarding.modeIndex = 1
	case "dark":
		a.onboarding.modeIndex = 2
	}
	if ap.Style == styleSketch {
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

// 下载与注册成功只进入就绪页，必须由开始按钮显式提交。
func (a *app) submitOnboarding() {
	ob := &a.onboarding
	if ob.phase == "saving" || ob.preparation.running {
		return
	}
	a.commitTabCountText()
	dir, err := a.validateStorageDir(ob.dirDraft)
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
	n, ap, dir := DefaultTabCount, a.prefs.Appearance, filepath.Dir(a.notes.DataPath())
	if configured {
		n, ap, dir = ob.tabCount, ob.appearance, ob.dirDraft
	}
	defaults := defaultPreferences().Appearance
	if ap.AppearanceMode == "" {
		ap.AppearanceMode = defaults.AppearanceMode
	}
	if ap.Style == "" {
		ap.Style = defaults.Style
	}
	if ap.ColorTheme == "" {
		ap.ColorTheme = defaults.ColorTheme
	}
	if ap.MarkdownTheme == "" {
		ap.MarkdownTheme = defaults.MarkdownTheme
	}
	if ap.CodeTheme == "" {
		ap.CodeTheme = defaults.CodeTheme
	}
	if ap.CodeIndent == "" {
		ap.CodeIndent = defaults.CodeIndent
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
	next := clonePrefs(a.prefs)
	next.Appearance = ap
	snapshot, store, prefStore := a.snapshotFile(), a.notes, a.prefsStore
	ob.phase, a.storageMoving = "saving", true
	persist := func() error {
		abs, err := validateStorageDir(dir)
		if err != nil {
			return err
		}
		next.Storage.DataFile = filepath.Join(abs, dataFileName)
		if err := validatePrefs(next); err != nil {
			return err
		}
		if next.Storage.DataFile == store.DataPath() {
			if err := store.SaveNow(snapshot); err != nil {
				return err
			}
			return prefStore.SaveNow(next)
		}
		return store.Relocate(next.Storage.DataFile, snapshot, func() error { return prefStore.SaveNow(next) })
	}
	complete := func(err error) {
		a.storageMoving = false
		if err != nil {
			ob.phase = "ready"
			ob.dirErr = locales.OnboardWriteErr(err.Error())
			return
		}
		a.prefs = next
		a.applyPreferencesToRuntime()
		ob.finished = true
		if win := ob.obWin; win != nil {
			win.Close()
		}
		a.enterMainFromOnboarding()
	}
	if win := ob.obWin; win != nil && a.ready {
		go func() {
			err := persist()
			win.Update(func() {
				if a.onboarding.obWin == win {
					complete(err)
				}
			})
		}()
	} else {
		complete(persist())
	}
}

func (a *app) enterMainFromOnboarding() {
	if a.ready {
		a.createWindow()
	}
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

func (a *app) obPickFolder() { a.chooseStorageDirAsync(a.onboarding.obWin, a.obApplyFolder) }
func (a *app) obApplyFolder(path string) {
	abs, err := a.validateStorageDir(path)
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
