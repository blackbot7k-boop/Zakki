package app

// 设置页布局与预览样式的测试：previewStyle 的覆盖/跟随语义，
// 以及新版卡片式设置页的关键内容渲染。

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"zakki/locales"
)

func TestResolveStyleAppliesOverrides(t *testing.T) {
	a := newTestApp(t)
	a.prefs.Appearance.MarkdownTheme = "book"
	a.prefs.Appearance.ColorTheme = "paper"
	a.prefs.Appearance.BodyFont = "songti"
	a.prefs.Appearance.MonoFont = "menlo"
	a.prefs.Appearance.BodySize = 21
	a.prefs.Appearance.LineHeight = 2.0

	s := a.resolveStyle(-1, false, true)
	if s.bodySize != 21 || s.lineHeight != 2.0 {
		t.Fatalf("排版微调应作用于样式: size=%v lh=%v", s.bodySize, s.lineHeight)
	}
	if s.bodyFont != bodyFontStack("songti", a.prefs.Appearance.LatinFont) || s.monoFont != monoFontStack("menlo") {
		t.Fatalf("字体覆盖应作用于样式: %q %q", s.bodyFont, s.monoFont)
	}
	if s.headingSizes[1] != roundHalf(21*markdownLayoutByID("book").headingScale[1]) {
		t.Fatalf("标题字号应随字号覆盖缩放: %+v", s.headingSizes)
	}
	if s.background != colorThemes[0].light.background { // paper
		t.Fatalf("颜色主题应作用于样式: %v", s.background)
	}
}

func TestResolveStyleFollowsLayoutWithoutOverrides(t *testing.T) {
	a := newTestApp(t)
	a.prefs.Appearance.MarkdownTheme = "book"
	a.prefs.Appearance.BodySize, a.prefs.Appearance.LineHeight = 0, 0
	a.prefs.Appearance.BodyFont, a.prefs.Appearance.LatinFont, a.prefs.Appearance.MonoFont = "", "", ""

	layout := markdownLayoutByID("book")
	s := a.resolveStyle(-1, false, true)
	if s.bodySize != layout.bodySize || s.lineHeight != layout.lineHeight {
		t.Fatalf("空覆盖应跟随排版主题: %+v", s)
	}
	// 字体与排版主题解耦：book 主题不再带宋体，空键始终是默认栈。
	if s.bodyFont != sonnetBodyFont || s.monoFont != sonnetMonoFont {
		t.Fatalf("空字体键应为默认栈: %q %q", s.bodyFont, s.monoFont)
	}
}

func TestSettingsViewCardLayout(t *testing.T) {
	a := newTestApp(t)
	a.winOpen = true

	a.settingsPage = pageAppearance
	a.settingsUI.syncDraftFromPrefs(a.prefs)
	tt := ui.NewTester(a.settingsView, 900, 620)
	tt.Frame()
	for _, want := range []string{
		"界面外观", "风格", "颜色主题",
		"排版主题", "中文字体", "英文字体", "代码字体",
		"主题", "排版", "语法配色", "恢复本页默认值",
	} {
		if !tt.HasText(want) {
			t.Fatalf("外观页缺少 %q: %v", want, tt.Texts())
		}
	}
	// 页面级常驻预览已移除：不应再出现「预览」栏标题。
	if tt.HasText("预览") {
		t.Fatalf("外观页不应再有页面级预览栏: %v", tt.Texts())
	}

	a.settingsPage = pageGeneral
	a.settingsUI.syncDraftFromPrefs(a.prefs)
	tt = ui.NewTester(a.settingsView, 900, 620)
	tt.Frame()
	if !tt.HasText("窗口") || !tt.HasText("恢复本页默认值") {
		t.Fatalf("常规页应为卡片分组且重置按钮在卡片内: %v", tt.Texts())
	}

	a.settingsPage = pageBackup
	a.settingsUI.syncDraftFromPrefs(a.prefs)
	tt = ui.NewTester(a.settingsView, 900, 620)
	tt.Frame()
	if !tt.HasText(a.notes.BackupDir()) || !tt.HasText("打开文件夹") {
		t.Fatalf("备份页缺少路径: %v", tt.Texts())
	}
	if tt.HasText("立即备份") || tt.HasText("Git 自动化") {
		t.Fatalf("备份页不应再有立即备份或 Git 自动化: %v", tt.Texts())
	}
}

func TestSettingsWideLayoutFallback(t *testing.T) {
	a := newTestApp(t)
	a.winOpen = true
	a.settingsPage = pageAppearance
	a.settingsUI.syncDraftFromPrefs(a.prefs)

	// 窄视口：单栏滚动列，内容完整且无页面级预览。
	tt := ui.NewTester(a.settingsView, 800, 620)
	tt.Frame()
	for _, want := range []string{"颜色主题", "排版主题", "中文字体"} {
		if !tt.HasText(want) {
			t.Fatalf("窄视口单栏缺少 %q: %v", want, tt.Texts())
		}
	}
	if tt.HasText("预览") {
		t.Fatalf("窄视口不应出现页面级预览栏: %v", tt.Texts())
	}
	if err := tt.Click("经典"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("石墨"); err != nil {
		t.Fatal(err)
	}
	if a.prefs.Appearance.ColorTheme != "graphite" {
		t.Fatalf("窄视口下主题下拉应可点击生效: %+v", a.prefs.Appearance)
	}
}

// TestSettingsFontSplitPersists 排版微调里中西文字体分开设置：分别选中后各自落到
// 偏好并持久化，中文选择不受西文选择影响。
func TestSettingsFontSplitPersists(t *testing.T) {
	a := newTestApp(t)
	if err := a.loadPreferences(); err != nil {
		t.Fatal(err)
	}
	a.settingsPage = pageAppearance
	a.settingsUI.syncDraftFromPrefs(a.prefs)
	// 视口要够高，排版微调卡片与下拉选项才在可视区内。
	tt := ui.NewTester(a.settingsView, 900, 1200)
	tt.Frame()
	latin := a.prefs.Appearance.LatinFont

	// 下拉：先点开当前值，再点选项。
	if err := tt.Click("霞鹜文楷（默认）"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("宋体"); err != nil {
		t.Fatal(err)
	}
	if a.prefs.Appearance.BodyFont != "songti" {
		t.Fatalf("中文字体应即时生效: %+v", a.prefs.Appearance)
	}
	if a.prefs.Appearance.LatinFont != latin {
		t.Fatalf("选中文字体不应改动西文设置: %q", a.prefs.Appearance.LatinFont)
	}
	// 英文字体行在代码字体行之前：同名显示时先点到的是英文字体下拉。
	if err := tt.Click(fontDisplayOf(1, latin)); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("Noteworthy"); err != nil {
		t.Fatal(err)
	}
	if a.prefs.Appearance.LatinFont != "noteworthy" {
		t.Fatalf("西文字体应即时生效: %+v", a.prefs.Appearance)
	}
	if a.prefs.Appearance.BodyFont != "songti" {
		t.Fatalf("选西文字体不应改动中文设置: %q", a.prefs.Appearance.BodyFont)
	}
	// 解析后的样式栈：西文在前，中文仍在。
	style := a.resolveStyle(-1, false, true)
	if !strings.HasPrefix(style.bodyFont, "Noteworthy, ") || !strings.Contains(style.bodyFont, "Songti SC") {
		t.Fatalf("正文栈应为西文在前中文在后: %q", style.bodyFont)
	}
	// 持久化。
	a.shutdownPreferences()
	saved, err := decodePreferences(prefsReadRaw(t, a.preferencesPath()))
	if err != nil {
		t.Fatal(err)
	}
	if saved.Appearance.BodyFont != "songti" || saved.Appearance.LatinFont != "noteworthy" {
		t.Fatalf("中西文字体应分别落盘: %+v", saved.Appearance)
	}
}

func TestSettingsDraftMergesExternalUpdatesByField(t *testing.T) {
	a := newTestApp(t)
	a.settingsUI.syncDraftFromPrefs(a.prefs)

	// 非模态窗口打开期间，其他入口更新同一 section 的另一个字段。
	if err := a.updatePreferences(func(p *preferences) {
		p.Window.AlwaysOnTop = true
		p.Appearance.Language = "en"
	}); err != nil {
		t.Fatal(err)
	}
	if !a.settingsUI.draft.Window.AlwaysOnTop || a.settingsUI.draft.Appearance.Language != "en" {
		t.Fatalf("未编辑字段应同步外部更新: %+v %+v", a.settingsUI.draft.Window, a.settingsUI.draft.Appearance)
	}

	// 用户随后只编辑 RememberBounds 和 ColorTheme，不得用陈旧草稿覆盖外部值。
	a.settingsUI.draft.Window.RememberBounds = false
	a.pushWindow()
	a.settingsUI.draft.Appearance.ColorTheme = "paper"
	a.pushAppearance()
	if !a.prefs.Window.AlwaysOnTop || a.prefs.Window.RememberBounds {
		t.Fatalf("窗口字段级合并失败: %+v", a.prefs.Window)
	}
	if a.prefs.Appearance.Language != "en" || a.prefs.Appearance.ColorTheme != "paper" {
		t.Fatalf("外观字段级合并失败: %+v", a.prefs.Appearance)
	}
}

func TestSettingsInvalidNumericTextAndErrorSurviveFrames(t *testing.T) {
	a := newTestApp(t)
	a.settingsPage = pageAppearance
	a.settingsUI.syncDraftFromPrefs(a.prefs)
	a.settingsUI.bodySizeText = "99"
	a.commitFloatField("bodySize")
	if a.settingsUI.bodySizeErr == "" || !a.settingsUI.bodySizeDirty {
		t.Fatalf("越界值必须保持 dirty 与行内错误: %+v", a.settingsUI)
	}
	if a.prefs.Appearance.BodySize != 0 {
		t.Fatalf("越界值不得提交: %v", a.prefs.Appearance.BodySize)
	}

	tt := ui.NewTester(a.settingsView, 900, 1200)
	for frame := 1; frame <= 2; frame++ {
		tt.Frame()
		if a.settingsUI.bodySizeText != "99" || a.settingsUI.bodySizeErr == "" || !a.settingsUI.bodySizeDirty {
			t.Fatalf("第 %d 帧清掉了非法输入: text=%q err=%q dirty=%v", frame,
				a.settingsUI.bodySizeText, a.settingsUI.bodySizeErr, a.settingsUI.bodySizeDirty)
		}
	}

	if err := a.updatePreferences(func(p *preferences) { p.Appearance.ColorTheme = "paper" }); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.settingsUI.bodySizeText != "99" || a.settingsUI.bodySizeErr == "" || !a.settingsUI.bodySizeDirty {
		t.Fatalf("外部更新不得清掉非法数字草稿: text=%q err=%q dirty=%v",
			a.settingsUI.bodySizeText, a.settingsUI.bodySizeErr, a.settingsUI.bodySizeDirty)
	}

	a.settingsUI.bodySizeText = "18"
	a.commitFloatField("bodySize")
	tt.Frame()
	if a.settingsUI.bodySizeDirty || a.settingsUI.bodySizeErr != "" {
		t.Fatalf("合法修正后才应清 dirty/error: err=%q dirty=%v", a.settingsUI.bodySizeErr, a.settingsUI.bodySizeDirty)
	}
	if a.prefs.Appearance.BodySize != 18 || a.settingsUI.bodySizeText != "18" {
		t.Fatalf("合法修正未提交或未规范化: prefs=%v text=%q", a.prefs.Appearance.BodySize, a.settingsUI.bodySizeText)
	}
}

func TestSettingsDefaultWindowSize(t *testing.T) {
	a := newTestApp(t)
	a.openSettings(pageGeneral)
	tt := ui.NewTester(a.settingsView, 900, 620)
	if _, ok := tt.Find("默认宽度"); !ok {
		t.Fatalf("常规页应有默认宽度: %v", tt.Texts())
	}
	if _, ok := tt.Find("默认高度"); !ok {
		t.Fatalf("常规页应有默认高度: %v", tt.Texts())
	}
	if err := a.updatePreferences(func(p *preferences) {
		p.Advanced.WindowWidth = 480
		p.Advanced.WindowHeight = 640
	}); err != nil {
		t.Fatal(err)
	}
	if a.prefs.Advanced.WindowWidth != 480 || a.settingsUI.draft.Advanced.WindowWidth != 480 || a.settingsUI.draft.Advanced.WindowHeight != 640 {
		t.Fatalf("默认尺寸未写入: prefs=%+v draft=%+v", a.prefs.Advanced, a.settingsUI.draft.Advanced)
	}
	a.resetPageDefaults(pageGeneral)
	if a.prefs.Advanced.WindowWidth != 400 || a.prefs.Advanced.WindowHeight != 540 {
		t.Fatalf("恢复本页默认应回到 400×540: %+v", a.prefs.Advanced)
	}
}

func TestResetPageAfterConfirmationDropsWhenWindowClosed(t *testing.T) {
	a := newTestApp(t)
	a.prefs.Window.AlwaysOnTop = true

	a.resetPageAfterConfirmation(pageGeneral)

	if !a.prefs.Window.AlwaysOnTop {
		t.Fatal("后台对话框结果不得在没有存活窗口时直接修改偏好")
	}
}

// TestSettingsCodeThemePersists 代码配色下拉：从默认柔和改选明快，落到偏好并持久化；
// 显式「跟随文本」也能选中并原样落盘（空串不可被 omitempty 吞掉）。
func TestSettingsCodeThemePersists(t *testing.T) {
	a := newTestApp(t)
	if err := a.loadPreferences(); err != nil {
		t.Fatal(err)
	}
	if a.prefs.Appearance.CodeTheme != "soft" {
		t.Fatalf("新安装默认应为柔和: %+v", a.prefs.Appearance)
	}
	a.settingsPage = pageAppearance
	a.settingsUI.syncDraftFromPrefs(a.prefs)
	tt := ui.NewTester(a.settingsView, 900, 1200)
	tt.Frame()

	if err := tt.Click(locales.CodeThemeSoft()); err != nil {
		t.Fatalf("当前值应可见: %v", err)
	}
	if err := tt.Click(locales.CodeThemeVivid()); err != nil {
		t.Fatal(err)
	}
	if a.prefs.Appearance.CodeTheme != "vivid" {
		t.Fatalf("代码配色应即时生效: %+v", a.prefs.Appearance)
	}
	// 显式选「跟随文本」：空串必须即时生效且落盘保留。
	if err := tt.Click(locales.CodeThemeVivid()); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click(locales.CodeThemeNone()); err != nil {
		t.Fatal(err)
	}
	if a.prefs.Appearance.CodeTheme != "" {
		t.Fatalf("跟随文本应即时生效: %+v", a.prefs.Appearance)
	}
	a.shutdownPreferences()
	saved, err := decodePreferences(prefsReadRaw(t, a.preferencesPath()))
	if err != nil {
		t.Fatal(err)
	}
	if saved.Appearance.CodeTheme != "" {
		t.Fatalf("跟随文本应已落盘为空串: %+v", saved.Appearance)
	}
}
