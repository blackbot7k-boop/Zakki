package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"zakki/internal/fonts"
	"zakki/locales"
)

type fontRuntimeState struct {
	families []ui.FontFamily
	loaded   bool
	scanID   uint64
	pickers  [3]fontPickerState
	// background 静默补齐已有偏好引用、但本机缺失的可下载字体（老用户升级路径）。
	background fontPreparation
	// styles 缓存解析后的样式；键里含字体列表身份，列表替换后自动失效。
	styles styleCache
}

type fontPickerState struct{ query string }

type fontJob struct {
	resource fonts.Resource
	progress fonts.Progress
	err      string
}

type fontPreparation struct {
	cancel     context.CancelFunc
	generation uint64
	running    bool
	jobs       []fontJob
	err        string
}

func (s *fontPreparation) stop() {
	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
	}
	s.generation++
	s.running = false
}

func (a *app) fontStore() fonts.Store {
	return fonts.Store{Dir: filepath.Join(filepath.Dir(a.notes.DataFile), "fonts")}
}

func hasFontFamily(families []ui.FontFamily, name string) bool {
	for _, f := range families {
		if strings.EqualFold(f.Name, name) {
			return true
		}
	}
	return false
}

func fontResource(key string) (fonts.Resource, bool) {
	id := ""
	switch key {
	case "kai", "sonnet", "hand", "hannotate":
		id = "wenkai"
	case "jetbrains", "jbm":
		id = "jetbrains"
	case "fira":
		id = "fira"
	}
	return fonts.Find(id)
}

func (a *app) fontWindowAlive(win *mygo.Window) bool {
	return win != nil && (a.onboarding.obWin == win || a.settingsWin == win || a.win == win)
}

// 系统枚举与缓存校验不占用构建帧；结果只经存活窗口回到主线程。
func (a *app) loadFontFamilies(win *mygo.Window) {
	if a.fonts.loaded || win == nil {
		return
	}
	a.fonts.scanID++
	generation := a.fonts.scanID
	store := a.fontStore()
	go func() {
		families := ui.FontFamilies()
		var issues []string
		for _, r := range fonts.Catalog() {
			if hasFontFamily(families, r.Family) {
				continue
			}
			data, err := store.Read(r)
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			if err == nil {
				err = ui.RegisterFont(data, r.Family)
			}
			if err != nil {
				slog.Warn("字体缓存不可用", "font", r.Family, "error", err)
				issues = append(issues, r.Family)
			}
		}
		families = ui.FontFamilies()
		win.Update(func() {
			if !a.fontWindowAlive(win) || a.fonts.scanID != generation {
				return
			}
			a.fonts.families, a.fonts.loaded = families, true
			if len(issues) > 0 {
				a.notice = locales.FontCacheUnavailable(strings.Join(issues, ", "))
			}
			// 扫描改变字体选项状态：重绑设置页下拉并刷新已开的设置窗口。
			a.settingsUI.syncDraftBindings()
			a.refreshSettingsWindow()
		})
	}()
}

// 不修改持久化选择；已卸载的本机字体只在渲染时回退。
func (a *app) usableFontKey(key string) string {
	if family := fonts.LocalFamily(key); family != "" && !hasFontFamily(a.fonts.families, family) {
		return "system"
	}
	return key
}

func (a *app) selectedFontError(keys []string) string {
	for _, key := range keys {
		if family := fonts.LocalFamily(key); family != "" && !hasFontFamily(a.fonts.families, family) {
			return locales.FontUnavailable(family)
		}
	}
	return ""
}

func (a *app) fontJobs(keys []string) []fontJob {
	seen := map[string]bool{}
	var jobs []fontJob
	for _, key := range keys {
		r, ok := fontResource(key)
		if !ok || seen[r.ID] || hasFontFamily(a.fonts.families, r.Family) {
			continue
		}
		seen[r.ID] = true
		jobs = append(jobs, fontJob{resource: r, progress: fonts.Progress{Total: r.Bytes, Stage: "queued"}})
	}
	return jobs
}

// prepareFonts 统一准备入口：下载缺失字体并注册，完成后在主线程 settle(ok)。
// 每批任务只接触资源快照；返回修改、关窗或重试后，旧回调不得推进新批次。
func (a *app) prepareFonts(win *mygo.Window, state *fontPreparation, keys []string, settle func(ok bool)) {
	if settle == nil {
		settle = func(bool) {}
	}
	state.stop()
	state.err = a.selectedFontError(keys)
	if state.err != "" {
		return
	}
	state.jobs = a.fontJobs(keys)
	if len(state.jobs) == 0 {
		settle(true)
		return
	}
	if win == nil {
		state.err = locales.FontPrepareFailed()
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	state.cancel, state.running = cancel, true
	generation := state.generation
	jobs := append([]fontJob(nil), state.jobs...)
	store := a.fontStore()
	go func() {
		failed := false
		for i, job := range jobs {
			if ctx.Err() != nil {
				return
			}
			index := i
			report := func(p fonts.Progress, message string) {
				win.Update(func() {
					if ctx.Err() != nil || !a.fontWindowAlive(win) || generation != state.generation {
						return
					}
					state.jobs[index].progress = p
					state.jobs[index].err = message
				})
			}
			data, err := store.Ensure(ctx, job.resource, func(p fonts.Progress) {
				if p.Stage == "ready" {
					p.Stage = "registering"
				}
				report(p, "")
			})
			if err == nil && ctx.Err() == nil {
				err = ui.RegisterFont(data, job.resource.Family)
			}
			if ctx.Err() != nil {
				return
			}
			if err != nil {
				failed = true
				report(fonts.Progress{Total: job.resource.Bytes, Stage: "error"}, err.Error())
			} else {
				report(fonts.Progress{Downloaded: job.resource.Bytes, Total: job.resource.Bytes, Stage: "ready"}, "")
			}
		}
		families := ui.FontFamilies()
		win.Update(func() {
			if ctx.Err() != nil || !a.fontWindowAlive(win) || generation != state.generation {
				return
			}
			state.running = false
			state.cancel = nil
			cancel()
			a.fonts.families = families
			if failed {
				state.err = locales.FontPrepareFailed()
				settle(false)
				return
			}
			state.err = ""
			settle(true)
		})
	}()
}

// preparePreferredFontInBackground 老用户升级后首次开窗：偏好引用的可下载
// 字体缺失时静默补齐并注册；失败只记日志，不改偏好、渲染按缺失回退。
func (a *app) preparePreferredFontInBackground() {
	keys := []string{a.prefs.Appearance.BodyFont, a.prefs.Appearance.LatinFont, a.prefs.Appearance.MonoFont}
	if len(a.fontJobs(keys)) == 0 {
		return
	}
	a.prepareFonts(a.mainWindow.Load(), &a.fonts.background, keys, nil)
}

// applySettingsFont 设置页字体变更：本机可用立即生效；缺失的可下载字体
// 先准备、成功后才写入偏好，失败保留旧偏好并提示，草稿不落未应用值。
func (a *app) applySettingsFont(field string, key string) {
	apply := func() {
		_ = a.updatePreferences(func(p *preferences) {
			switch field {
			case "bodyFont":
				p.Appearance.BodyFont = key
			case "latinFont":
				p.Appearance.LatinFont = key
			default:
				p.Appearance.MonoFont = key
			}
		})
		a.settingsUI.syncDraftBindings()
		a.refreshSettingsWindow()
	}
	if len(a.fontJobs([]string{key})) == 0 {
		apply()
		return
	}
	// 换字体即取消上一批未完成的准备，避免旧回调写入过期选择。
	a.prepareFonts(a.settingsWin, &a.settingsUI.fontPreparation, []string{key}, func(ok bool) {
		if ok {
			apply()
		} else {
			a.settingsUI.syncDraftBindings()
			a.refreshSettingsWindow()
		}
	})
}

func fontBytes(n int64) string {
	if n < 1000000 {
		return fmt.Sprintf("%d KB", n/1000)
	}
	return fmt.Sprintf("%.1f MB", float64(n)/1000000)
}

func fontStage(stage string) string {
	switch stage {
	case "downloading":
		return locales.FontDownloading()
	case "verifying":
		return locales.FontVerifying()
	case "registering":
		return locales.FontRegistering()
	case "ready":
		return locales.FontReady()
	case "error":
		return locales.FontFailed()
	default:
		return locales.FontQueued()
	}
}

func fontProgressView(c *ui.Context, state *fontPreparation) {
	var got, total int64
	for _, j := range state.jobs {
		got += j.progress.Downloaded
		total += j.resource.Bytes
	}
	if total > 0 {
		ui.Row(c).FillWidth().Children(func() {
			ui.Text(c, locales.FontDownloadProgress()).Grow(1)
			ui.Text(c, fontBytes(got)+" / "+fontBytes(total)).FontSize(12).TextColor(tMuted(c))
		})
		ui.Progress(c, float64(got)/float64(total)).FillWidth().Height(4).Label(locales.FontDownloadProgress())
	}
	for _, job := range state.jobs {
		ui.Column(c.Key(job.resource.ID)).FillWidth().Gap(4).PaddingY(6).Children(func() {
			ui.Row(c).FillWidth().Children(func() {
				ui.Text(c, job.resource.Family).Grow(1)
				ui.Text(c, fontStage(job.progress.Stage)).FontSize(11).TextColor(tMuted(c))
			})
			if job.err != "" {
				ui.Text(c, job.err).FontSize(11).TextColor(c.Theme().Danger)
			} else {
				ui.Progress(c, float64(job.progress.Downloaded)/float64(job.resource.Bytes)).FillWidth().Height(2).Label(job.resource.Family)
			}
		})
	}
	if state.err != "" {
		ui.Text(c, state.err).FontSize(12).TextColor(c.Theme().Danger)
	}
}
