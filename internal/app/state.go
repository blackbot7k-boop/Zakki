package app

// 应用状态、持久化与自动备份。
//
// 数据模型刻意保持简单：7 个点各自是一段 markdown 纯文本，
// 整体存成一个 JSON 文件；备份是文件的滚动副本。

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/egoist/mygo/ui"
)

const appName = "Zakki"

// snapshotFile 在主线程把当前状态拍成磁盘格式（后台只写该快照）。
func (a *app) snapshotFile() savedFile {
	tabs := make([]savedTab, len(a.tabs))
	for i, tb := range a.tabs {
		tabs[i] = savedTab{ID: tb.ID, Name: tb.Name, Text: tb.Text, Color: tb.Color}
	}
	return savedFile{
		Version: currentDataVersion,
		Active:  a.active,
		Tabs:    tabs,
		Settings: settings{
			ThemeSource:   a.cfg.ThemeSource,
			MarkdownTheme: a.cfg.MarkdownTheme,
			BodyFont:      a.cfg.BodyFont,
			MonoFont:      a.cfg.MonoFont,
			BodySize:      a.cfg.BodySize,
			LineHeight:    a.cfg.LineHeight,
			AlwaysOnTop:   a.cfg.AlwaysOnTop,
			HideOnEscape:  a.cfg.HideOnEscape,
			Source:        a.cfg.Source,
			TabTitleWidth: a.cfg.TabTitleWidth,
			TabAnimSpeed:  a.cfg.TabAnimSpeed,
			MonoDots:      a.cfg.MonoDots,
			GlassStrength: a.cfg.GlassStrength,
		},
	}
}

// newApp 建立空状态：先给 7 个默认标签（无数据文件/测试环境都有
// 可用的标签列表），load 时被磁盘数据覆盖。
func newApp() *app {
	s := settings{ThemeSource: "system", HideOnEscape: true}
	now := time.Now
	dir := dataDir()
	a := &app{
		cfg:      s,
		prefs:    defaultPreferences(),
		tabs:     defaultTabs(),
		mdActive: -1, // 启动即全预览态：光标不会让任何一行变成源码
		now:      now,
		notes: newNotesStore(
			filepath.Join(dir, dataFileName),
			filepath.Join(dir, backupDirName),
			now,
		),
		prefsStore: newPrefsStore(filepath.Join(dir, "preferences.json")),
	}
	a.syncNextTabID()
	a.resetSelectionSync()
	return a
}

// resetSelectionSync 把选区同步哨兵复位为「不强制」。
// selSyncStart 等哨兵用 -1 表示不强制；零值 0 会误触强制选区，
// 因此 newApp 显式初始化。
func (a *app) resetSelectionSync() {
	a.selSyncStart, a.selSyncEnd = -1, -1
	a.srcSelSyncStart, a.srcSelSyncEnd = -1, -1
}

// ---- 数据目录与文件 ----

// pendingPath 写保护期间新改动的暂存文件，绝不碰原路径。
func (a *app) pendingPath() string { return a.notes.PendingPath() }

// currentDataPath 当前生效的数据文件：写保护期间是暂存文件。
func (a *app) currentDataPath() string { return a.notes.CurrentDataPath() }

// ---- 载入与保存 ----

// load 从磁盘读入数据。文件不存在用默认值（首次运行）；文件损坏或版本
// 不支持时隔离原件、尽量从备份恢复，绝不覆盖用户数据。返回非 nil 表示
// 数据文件有问题（即使已从备份恢复，也如实上报）。
func (a *app) load() error {
	raw, err := os.ReadFile(a.notes.DataFile)
	switch {
	case errors.Is(err, os.ErrNotExist):
		a.initDefaults()
		return nil
	case err != nil:
		return a.recoverFromBrokenFile(fmt.Errorf("读取数据文件失败: %w", err))
	}
	f, hadTabs, err := a.notes.ParseRaw(raw)
	if err != nil {
		if _, ok := err.(*versionError); ok {
			return a.recoverFromBrokenFile(err)
		}
		return a.recoverFromBrokenFile(fmt.Errorf("解析数据文件失败: %w", err))
	}
	a.applySavedFile(f)
	if !hadTabs {
		a.initDefaults() // 空文件：首次运行的欢迎文本
	}
	a.resyncLive(0)
	return nil
}

// applySavedFile 把迁移后的磁盘数据放进内存状态（主线程）。
func (a *app) applySavedFile(f savedFile) {
	a.tabs = a.tabs[:0]
	for _, st := range f.Tabs {
		a.tabs = append(a.tabs, tab{ID: st.ID, Name: st.Name, Text: st.Text, Color: st.Color})
	}
	a.active = f.Active
	a.clampActive()
	a.syncNextTabID()
	a.cfg.ThemeSource = orDefault(f.Settings.ThemeSource, "system")
	a.cfg.MarkdownTheme = f.Settings.MarkdownTheme
	a.cfg.BodyFont, a.cfg.MonoFont = f.Settings.BodyFont, f.Settings.MonoFont
	a.cfg.BodySize, a.cfg.LineHeight = f.Settings.BodySize, f.Settings.LineHeight
	a.cfg.AlwaysOnTop = f.Settings.AlwaysOnTop
	a.cfg.HideOnEscape = f.Settings.HideOnEscape
	a.cfg.Source = f.Settings.Source
	a.cfg.TabTitleWidth, a.cfg.TabAnimSpeed = f.Settings.TabTitleWidth, f.Settings.TabAnimSpeed
	a.cfg.MonoDots, a.cfg.GlassStrength = f.Settings.MonoDots, f.Settings.GlassStrength
}

// recoverFromBrokenFile 数据文件不可用时的兜底：复制隔离原件（绝不删除或
// 覆盖原件），尝试从最近备份恢复；都失败则用默认值并冻结对原路径的写入
// （新改动暂存 .pending），等用户确认后再写回。返回 cause 供调用方上报。
func (a *app) recoverFromBrokenFile(cause error) error {
	a.notes.QuarantineCorruptFile()
	if f, name, ok := a.notes.LoadNewestBackup(); ok {
		a.applySavedFile(f)
		a.resyncLive(0)
		a.notice = "数据文件损坏，已从备份 " + name + " 恢复（原件已隔离）"
		slog.Warn("数据文件损坏，已从备份恢复", "backup", name, "error", cause)
		return cause
	}
	a.initDefaults()
	a.resyncLive(0)
	a.notes.SetSaveBlocked(true)
	a.notice = "数据文件损坏且无可用备份，新改动暂存 .pending；请用菜单“写回数据文件”确认覆盖"
	slog.Error("数据文件损坏且备份不可用，默认内容不写回原路径", "error", cause)
	return cause
}

// welcomeTextBody 欢迎文案中与标签数量无关的段落。
const welcomeTextBody = `- [ ] 试试勾选我：其他块即时排版，点一下就能编辑
- ⌘1 到 ⌘9 切换标签；⌘T 新建；Enter 分块，↑↓←→ 跨块
- ⌘F 搜索本标签内容，⌘⇧P 切换源码模式
- ⌘B / ⌘I / ⌘K 直接加粗、斜体、插链接；⌘⌥1–6 改标题
- 默认 ⌘⇧空格 全局呼出/隐藏窗口（在「设置 → 快捷键」里改）
- 颜色与排版主题、备份策略都在「设置…」⌘, 里，即时生效
- 标签命名、排序与颜色在「设置 → 标签与外观」里，双击胶囊也可重命名
- ⌥⌘T 窗口置顶，Esc 隐藏窗口

支持 **粗体**、*斜体*、` + "`代码`" + `、[链接](https://tot.rocks)、引用和代码块。

> 灵感来自 [Tot](https://tot.rocks)，用 [MyGo](https://github.com/egoist/mygo) 构建。`

// welcomeTextFor 按初始标签数生成标签 1 的欢迎文本。
func welcomeTextFor(tabCount int) string {
	if tabCount < 1 {
		tabCount = 1
	}
	return fmt.Sprintf("# Zakki\n\n%d 个彩色标签，随手记。这里是 **标签 1**。\n\n", tabCount) + welcomeTextBody
}

// welcomeText 默认标签数下的欢迎文案（测试与固定引用）。
var welcomeText = welcomeTextFor(DefaultTabCount)

// commitFirstRunData 写入首启数据快照。tabCount>0 时按数量重建标签；为 0 时
// 保留当前标签数并刷新欢迎文案。非首启时静默成功。
func (a *app) commitFirstRunData(tabCount int) error {
	if !a.firstRun() {
		return nil
	}
	if tabCount > 0 {
		if tabCount < 1 {
			tabCount = 1
		}
		if tabCount > MaxTabCount {
			tabCount = MaxTabCount
		}
		a.rebuildInitialTabs(tabCount)
	} else if len(a.tabs) > 0 {
		text := welcomeTextFor(len(a.tabs))
		a.tabs[0].Text = text
		a.resyncLive(utf8.RuneCountInString(text))
	}
	err := a.notes.SaveNow(a.snapshotFile())
	if err != nil {
		slog.Warn("首启数据写入失败", "error", err)
	}
	return err
}

// persistFirstRunSnapshot 跳过/关窗路径落盘（保持当前默认标签数）。
func (a *app) persistFirstRunSnapshot() error {
	return a.commitFirstRunData(0)
}

// initDefaults 首次运行/空文件：保证 7 个默认标签存在，标签 1 是
// 欢迎文本。不改 active（空文件里 active 跟随磁盘数据）。
func (a *app) initDefaults() {
	if len(a.tabs) == 0 {
		a.tabs = defaultTabs()
		a.syncNextTabID()
	}
	if len(a.tabs) > 0 {
		a.tabs[0].Text = welcomeTextFor(len(a.tabs))
	}
	a.clampActive()
	a.resyncLive(utf8.RuneCountInString(a.tabs[0].Text))
}

// saveNow 把当前状态快照同步写入磁盘（原子替换）。只保存数据，不做备份。
func (a *app) saveNow() error {
	return a.notes.SaveNow(a.snapshotFile())
}

// scheduleSave 防抖保存：配置延迟（默认 800ms）内没有新的改动才落盘。快照在主线程生成，
// 后台定时器只把快照写盘，绝不读写 app 状态（MyGo：UI 状态只在主线程改）。
func (a *app) scheduleSave() {
	win := a.win
	after := func() {
		if win != nil {
			win.Update(func() {}) // 回主线程刷新状态栏（备份时间/错误提示）
		}
	}
	a.notes.ScheduleDebounced(
		a.snapshotFile(),
		a.autosaveDelay(),
		a.backupIntervalDuration(),
		a.prefs.Backup.Keep,
		after,
	)
}

// shutdown 退出流程（OnWillQuit，主线程）：取消防抖定时器，做一次最终快照
// 保存并等它完成；此后不再接受任何保存请求，也不会与定时保存并发写两份。
func (a *app) shutdown() {
	if err := a.shutdownForQuit(); err != nil {
		slog.Error("最终保存失败", "error", err)
	}
}

func (a *app) shutdownForQuit() error {
	if err := a.notes.Shutdown(a.snapshotFile()); err != nil {
		a.notes.SetStopped(false)
		return err
	}
	if err := a.shutdownPreferences(); err != nil {
		a.notes.SetStopped(false)
		return err
	}
	slog.Info("退出前已最终保存")
	return nil
}

// backupNow 保存当前数据并无条件写一份备份（手动备份入口，绕过周期节流）。
func (a *app) backupNow() error {
	if err := a.saveNow(); err != nil {
		return err
	}
	return a.notes.BackupNow(a.prefs.Backup.Keep)
}

// backupInterval 自动滚动备份的默认最小间隔。
const backupInterval = 5 * time.Minute

// maybeWritePeriodicBackup 自动备份：距上次备份超过配置间隔才写一份。
func (a *app) maybeWritePeriodicBackup() error {
	return a.notes.MaybeWritePeriodicBackup(a.backupIntervalDuration(), a.prefs.Backup.Keep)
}

// backupOnLaunch 启用启动备份时做一份备份（当前数据文件存在时）。
// 参数由调用方从偏好预取，可在后台 goroutine 调用而不与主线程的偏好变更竞争。
func (a *app) backupOnLaunchAsync(enabled bool, keep int) error {
	return a.notes.BackupOnLaunch(enabled, keep)
}

// confirmWriteBack 用户确认后解除写保护，把当前状态写回数据文件
// （不可用的原件此前已隔离为 .corrupt-<时间戳>，不会丢）。
func (a *app) confirmWriteBack() error {
	blocked := a.notes.SaveBlocked()
	a.notes.SetSaveBlocked(false)
	if err := a.saveNow(); err != nil {
		a.notes.SetSaveBlocked(blocked) // 写回失败维持写保护
		return err
	}
	if blocked {
		if err := a.notes.RemovePending(); err != nil && !errors.Is(err, os.ErrNotExist) {
			slog.Warn("清理暂存文件失败", "error", err)
		}
		a.notice = ""
		slog.Info("已按用户确认把数据写回原路径")
	}
	return nil
}

// clearActiveTab 清空当前标签内容。危险操作：先无条件备份，备份失败则
// 取消清空。
func (a *app) clearActiveTab() {
	if err := a.backupNow(); err != nil {
		slog.Error("清空当前标签前备份失败，已取消清空", "error", err)
		return
	}
	a.activeTab().Text = ""
	a.resyncLive(0)
	a.liveFocus = 1 // 焦点回到编辑器（点击按钮会把焦点带走）
	a.onEdited()
}

// reportPersist 上报持久化结果：失败记 ERROR 日志并让状态栏显示，成功清错误。
func (a *app) reportPersist(op string, err error) { a.notes.ReportPersist(op, err) }

// saveError 最近一次持久化错误（空串表示没有）。
func (a *app) saveError() string { return a.notes.LastSaveError() }

// ---- 小工具 ----

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

// lastBackupTime 最近备份时间（后台备份也会更新 Store）。
func (a *app) lastBackupTime() time.Time { return a.notes.LastBackupTime() }

// loadLastBackupTime 启动时从备份目录恢复最近备份时间。
func (a *app) loadLastBackupTime() error { return a.notes.LoadLastBackupTime() }

// findMatches 找出 query 在 text 中的全部出现位置（大小写不敏感），
// 返回按 rune 计的区间。
func findMatches(text, query string) []ui.TextRange {
	if query == "" {
		return nil
	}
	return matchLower(strings.ToLower(text), strings.ToLower(query))
}

// matchLower 在已小写的文本里找全部出现位置，O(n)：
// rune 偏移随扫描递增，不对每个命中重扫前缀。
func matchLower(lt, lq string) []ui.TextRange {
	if lq == "" || len(lq) > len(lt) {
		return nil
	}
	var out []ui.TextRange
	runes, off := 0, 0
	for {
		idx := strings.Index(lt[off:], lq)
		if idx < 0 {
			break
		}
		b := off + idx
		runes += utf8.RuneCountInString(lt[off:b])
		out = append(out, ui.TextRange{Start: runes, End: runes + utf8.RuneCountInString(lq)})
		runes += utf8.RuneCountInString(lq)
		off = b + len(lq)
	}
	return out
}

// loweredDoc 返回 doc 的小写副本（缓存；全文 ToLower 每帧做太浪费）。
func (a *app) loweredDoc(doc string) string {
	if doc != a.lowerDoc {
		a.lowerDoc, a.lowerCache = doc, strings.ToLower(doc)
	}
	return a.lowerCache
}

// docMatches 与 findMatches 相同，但复用当前点文本的小写缓存
// （查找条打开时每帧都会调用）。
func (a *app) docMatches(doc, query string) []ui.TextRange {
	if query == "" {
		return nil
	}
	// 查找条打开时每帧都会调用，按文档串与查询串缓存全文扫描结果。
	if a.matchesCacheDoc == doc && a.matchesCacheQuery == query {
		return a.matchesCache
	}
	ms := matchLower(a.loweredDoc(doc), strings.ToLower(query))
	a.matchesCacheDoc, a.matchesCacheQuery, a.matchesCache = doc, query, ms
	return ms
}
