package main

// 应用状态、持久化与自动备份。
//
// 数据模型刻意保持简单：7 个点各自是一段 markdown 纯文本，
// 整体存成一个 JSON 文件；备份是文件的滚动副本。

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// DotCount 是 Tot 风格便签的分区数量：7 个彩色“点”。
const DotCount = 7

// appName 决定数据目录名（~/Library/Application Support/<appName>）。
const appName = "Tot MD"

// dotTheme 是从本机安装的 Tot 2.1.1 应用包提取的每点主题
// （/Applications/Tot.app/Contents/Resources/theme.json，成品工件证据）。
type dotTheme struct {
	dot  string // 点本身的颜色
	accL string // 浅色模式 accent（高亮/链接）
	accD string // 深色模式 accent
	bgL  string // 浅色模式内容区背景
	bgD  string // 深色模式内容区背景
	stL  string // 浅色模式顶栏/底栏背景（status_background）
	stD  string // 深色模式顶栏/底栏背景
}

var dotThemes = [DotCount]dotTheme{
	{"#d4253a", "#883341", "#ffbdc0", "#feeeef", "#211414", "#f5d5d5", "#3d282b"}, // 1 红
	{"#f67500", "#7e3f13", "#fac198", "#fff2e9", "#241c12", "#f2dac9", "#453222"}, // 2 橙
	{"#ffd01c", "#776200", "#f4dea3", "#fff9ea", "#211f0b", "#f4e9ce", "#3e3c1f"}, // 3 黄
	{"#65c802", "#58752e", "#d3efac", "#f9fff0", "#1b2015", "#dee8cb", "#313724"}, // 4 绿
	{"#0f75e9", "#3857a2", "#94bfff", "#e7eeff", "#191d26", "#cdd8f8", "#2c354e"}, // 5 蓝
	{"#a40bd8", "#754993", "#dfb5fc", "#f8effe", "#1e1723", "#e3d3f1", "#372943"}, // 6 紫
	{"#98989d", "#3a3a3c", "#d1d1d6", "#ffffff", "#0a0a0c", "#f2f2f4", "#0e0e10"}, // 7 灰（Tot 的 default 单色主题）
}

// dotColor 返回第 i 个点的颜色。
func dotColor(i int) ui.Color { return ui.Hex(dotThemes[i].dot) }

// dotAccent 返回第 i 个点的 accent 色（随深浅色模式）。
func dotAccent(i int, dark bool) ui.Color {
	if dark {
		return ui.Hex(dotThemes[i].accD)
	}
	return ui.Hex(dotThemes[i].accL)
}

// dotBackground 返回第 i 个点的内容区背景色（随深浅色模式）。
func dotBackground(i int, dark bool) ui.Color {
	if dark {
		return ui.Hex(dotThemes[i].bgD)
	}
	return ui.Hex(dotThemes[i].bgL)
}

// dotStatusBackground 返回第 i 个点的底栏背景色（Tot 的 status_background，
// 与 theme.json 逐字节核对过：teal 点 #D1E9EF 完全一致）。
func dotStatusBackground(i int, dark bool) ui.Color {
	if dark {
		return ui.Hex(dotThemes[i].stD)
	}
	return ui.Hex(dotThemes[i].stL)
}

// headerBackground 顶栏固定色：Tot 的顶栏不随点变，浅色 #F2F2F2（采样自真机截图）。
func headerBackground(dark bool) ui.Color {
	if dark {
		return ui.Hex("#2a2a2c")
	}
	return ui.Hex("#f2f2f2")
}

// themeIsDark 用背景亮度判断当前是否深色主题。
func themeIsDark(t *ui.Theme) bool {
	c := t.Background
	lum := (0.299*float64(c.R) + 0.587*float64(c.G) + 0.114*float64(c.B)) / 255
	return lum < 0.5
}

// dotLabel 返回点的展示名，如“点 3”。
func dotLabel(i int) string { return fmt.Sprintf("点 %d", i+1) }

type settings struct {
	ThemeSource  string `json:"themeSource"` // system | light | dark
	AlwaysOnTop  bool   `json:"alwaysOnTop"`
	HideOnEscape bool   `json:"hideOnEscape"`
	Preview      bool   `json:"preview"`
}

type savedFile struct {
	Version  int      `json:"version"`
	Active   int      `json:"active"`
	Dots     []string `json:"dots"`
	Settings settings `json:"settings"`
}

// app 是窗口展示的全部状态。它只能在主线程被改动；
// 其他 goroutine 通过 Window.Update 回到主线程。
type app struct {
	dots   [DotCount]string
	active int
	cfg    settings

	// 界面状态
	findOpen     bool
	query        string
	matchIdx     int
	jumpPending  bool
	insertBullet bool

	// 窗口与后台
	win     *mygo.Window
	winOpen bool
	hidden  bool

	// 菜单引用（同步勾选状态用）
	menu      *mygo.Menu
	miPreview *mygo.MenuItem
	miFind    *mygo.MenuItem
	miTop     *mygo.MenuItem
	miEsc     *mygo.MenuItem

	saveTimer  *time.Timer
	lastBackup time.Time
}

// persistOff 由测试置位，避免测试写真实数据目录。
var persistOff bool

func newApp() *app {
	return &app{cfg: settings{ThemeSource: "system", HideOnEscape: true}}
}

// ---- 数据目录与文件 ----

func dataDir() string {
	base, err := mygo.App.Path(mygo.PathAppData)
	if err != nil || base == "" {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, "Library", "Application Support")
	}
	return filepath.Join(base, appName)
}

func dataFile() string    { return filepath.Join(dataDir(), "tot-md.json") }
func backupDir() string   { return filepath.Join(dataDir(), "backups") }

// ---- 载入与保存 ----

// load 从磁盘读入数据；文件不存在或损坏时用默认值。
func (a *app) load() {
	raw, err := os.ReadFile(dataFile())
	if err != nil {
		a.initDefaults()
		return
	}
	var f savedFile
	if err := json.Unmarshal(raw, &f); err != nil {
		a.initDefaults()
		return
	}
	for i := 0; i < DotCount && i < len(f.Dots); i++ {
		a.dots[i] = f.Dots[i]
	}
	if a.dots[0] == "" && len(f.Dots) == 0 {
		a.initDefaults()
	}
	a.active = clamp(f.Active, 0, DotCount-1)
	a.cfg.ThemeSource = orDefault(f.Settings.ThemeSource, "system")
	a.cfg.AlwaysOnTop = f.Settings.AlwaysOnTop
	a.cfg.HideOnEscape = f.Settings.HideOnEscape
}

// initDefaults 在首次运行时给点 1 一段欢迎文本。
func (a *app) initDefaults() {
	a.dots[0] = `# Tot MD

7 个彩色点，随手记。这里是 **点 1**。

- [ ] 试试点前面的方框，勾选待办
- [ ] 按 ⌘⇧B 在当前行插入/取消圆点
- ⌘1 到 ⌘7 切换点
- ⌘F 搜索本点内容，⌘⇧P 切换 Markdown 预览
- ⌘⇧空格 全局呼出/隐藏窗口（后台也有效）
- ⌥⌘T 窗口置顶，Esc 隐藏窗口

支持 **粗体**、*斜体*、` + "`代码`" + `、[链接](https://tot.rocks)、引用和代码块。

> 灵感来自 [Tot](https://tot.rocks)，用 [MyGo](https://github.com/egoist/mygo) 构建。`
}

// saveNow 立即把状态写入磁盘（原子替换），必要时顺带做一次备份。
func (a *app) saveNow() {
	if persistOff {
		return
	}
	f := savedFile{
		Version: 1,
		Active:  a.active,
		Dots:    a.dots[:],
		Settings: settings{
			ThemeSource:  a.cfg.ThemeSource,
			AlwaysOnTop:  a.cfg.AlwaysOnTop,
			HideOnEscape: a.cfg.HideOnEscape,
		},
	}
	raw, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return
	}
	dir := dataDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return
	}
	tmp := dataFile() + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return
	}
	_ = os.Rename(tmp, dataFile())

	// 每 5 分钟至多一次滚动备份；距上次备份足够久才做。
	if a.lastBackup.IsZero() || time.Since(a.lastBackup) > 5*time.Minute {
		a.writeBackup()
	}
}

// scheduleSave 防抖保存：800ms 内没有新的改动才落盘。
func (a *app) scheduleSave() {
	if persistOff {
		return
	}
	if a.saveTimer != nil {
		a.saveTimer.Stop()
	}
	a.saveTimer = time.AfterFunc(800*time.Millisecond, func() {
		if a.winOpen && a.win != nil {
			a.win.Update(func() { a.saveNow() })
		} else {
			a.saveNow()
		}
	})
}

// writeBackup 把当前数据文件复制到备份目录，只保留最新 20 份。
func (a *app) writeBackup() {
	raw, err := os.ReadFile(dataFile())
	if err != nil {
		return
	}
	if err := os.MkdirAll(backupDir(), 0o755); err != nil {
		return
	}
	name := "tot-md-" + time.Now().Format("20060102-150405") + ".json"
	if err := os.WriteFile(filepath.Join(backupDir(), name), raw, 0o644); err != nil {
		return
	}
	a.lastBackup = time.Now()
	a.pruneBackups(20)
}

// backupOnLaunch 每次启动时做一份备份（数据文件存在时）。
func (a *app) backupOnLaunch() {
	if _, err := os.Stat(dataFile()); err == nil {
		a.writeBackup()
	}
}

func (a *app) pruneBackups(keep int) {
	entries, err := os.ReadDir(backupDir())
	if err != nil {
		return
	}
	var files []os.DirEntry
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
			files = append(files, e)
		}
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Name() > files[j].Name() })
	for i := keep; i < len(files); i++ {
		_ = os.Remove(filepath.Join(backupDir(), files[i].Name()))
	}
}

// ---- 桌面集成 ----

// applyAlwaysOnTop 把置顶设置同步到窗口。
func (a *app) applyAlwaysOnTop() {
	if a.win != nil && a.winOpen {
		a.win.SetAlwaysOnTop(a.cfg.AlwaysOnTop)
	}
}

// showWindow 显示并聚焦窗口（必要时新建）。
func (a *app) showWindow() {
	if a.win == nil || !a.winOpen {
		a.createWindow()
		return
	}
	a.win.Restore()
	a.win.Show()
	a.win.Focus()
}

// toggleWindow 全局热键与托盘点击：显示/隐藏窗口。
func (a *app) toggleWindow() {
	if a.hidden || a.win == nil || !a.winOpen {
		a.showWindow()
		return
	}
	a.win.Hide()
}

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

// countWords 统计词数（以空白分隔；中文连续文本按 1 词计）。
func countWords(s string) int { return len(strings.Fields(s)) }

// countChars 统计字符数（按 rune）。
func countChars(s string) int { return utf8.RuneCountInString(s) }

// countLines 统计行数（空文本为 0 行，与 Tot 一致）。
func countLines(s string) int {
	if s == "" {
		return 0
	}
	return strings.Count(s, "\n") + 1
}

// backupLabel 把最近备份时间转成人性化短句（Tot 底栏的 "Just now"）。
func (a *app) backupLabel() string {
	if a.lastBackup.IsZero() {
		return "尚未备份"
	}
	d := time.Since(a.lastBackup)
	switch {
	case d < time.Minute:
		return "刚刚备份"
	case d < time.Hour:
		return fmt.Sprintf("%d 分钟前备份", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%d 小时前备份", int(d.Hours()))
	default:
		return a.lastBackup.Format("01-02 15:04 备份")
	}
}

// loadLastBackupTime 启动时从备份目录恢复最近备份时间。
func (a *app) loadLastBackupTime() {
	entries, err := os.ReadDir(backupDir())
	if err != nil {
		return
	}
	var newest time.Time
	for _, e := range entries {
		if info, err := e.Info(); err == nil && info.ModTime().After(newest) {
			newest = info.ModTime()
		}
	}
	a.lastBackup = newest
}

// findMatches 找出 query 在 text 中的全部出现位置（大小写不敏感），
// 返回按 rune 计的区间。
func findMatches(text, query string) []ui.TextRange {
	if query == "" {
		return nil
	}
	lt, lq := strings.ToLower(text), strings.ToLower(query)
	var out []ui.TextRange
	for off := 0; off <= len(lt)-len(lq); {
		idx := strings.Index(lt[off:], lq)
		if idx < 0 {
			break
		}
		b := off + idx
		start := utf8.RuneCountInString(lt[:b])
		out = append(out, ui.TextRange{Start: start, End: start + utf8.RuneCountInString(lq)})
		off = b + len(lq)
	}
	return out
}
