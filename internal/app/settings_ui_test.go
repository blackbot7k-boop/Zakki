package app

// 设置页布局与预览样式的测试：previewStyle 的覆盖/跟随语义，
// 以及新版卡片式设置页的关键内容渲染。

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestPreviewStyleAppliesOverrides(t *testing.T) {
	a := newTestApp(t)
	ap := a.prefs.Appearance
	ap.MarkdownTheme = "book"
	ap.ColorTheme = "paper"
	ap.BodyFont = "songti"
	ap.MonoFont = "menlo"
	ap.BodySize = 21
	ap.LineHeight = 2.0

	s := a.previewStyle(ap, false)
	if s.bodySize != 21 || s.lineHeight != 2.0 {
		t.Fatalf("排版微调应作用于预览: size=%v lh=%v", s.bodySize, s.lineHeight)
	}
	if s.bodyFont != bodyFontStack("songti", "") || s.monoFont != monoFontStack("menlo") {
		t.Fatalf("字体覆盖应作用于预览: %q %q", s.bodyFont, s.monoFont)
	}
	if s.headingSizes[1] != roundHalf(21*markdownLayoutByID("book").headingScale[1]) {
		t.Fatalf("标题字号应随字号覆盖缩放: %+v", s.headingSizes)
	}
	if s.background != colorThemes[0].light.background { // paper
		t.Fatalf("颜色主题应作用于预览: %v", s.background)
	}
	if a.prefs.Appearance.BodySize == 21 || a.prefs.Appearance.ColorTheme == "paper" {
		t.Fatal("previewStyle 不应修改已保存偏好")
	}
}

func TestPreviewStyleFollowsThemeWithoutOverrides(t *testing.T) {
	a := newTestApp(t)
	ap := a.prefs.Appearance
	ap.MarkdownTheme = "book"
	ap.BodySize, ap.LineHeight = 0, 0
	ap.BodyFont, ap.MonoFont = "", ""

	layout := markdownLayoutByID("book")
	s := a.previewStyle(ap, false)
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
		"明暗模式", "颜色主题", "Markdown 排版主题", "排版微调",
		"中文字体", "西文字体", "代码字体",
		"预览", "排版主题", "恢复本页默认值",
	} {
		if !tt.HasText(want) {
			t.Fatalf("外观页缺少 %q: %v", want, tt.Texts())
		}
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
	if !tt.HasText("自动备份") || !tt.HasText("手动操作") || !tt.HasText("备份目录：") {
		t.Fatalf("备份页缺少卡片或路径: %v", tt.Texts())
	}
}

func TestSettingsWideLayoutFallback(t *testing.T) {
	a := newTestApp(t)
	a.winOpen = true
	a.settingsPage = pageAppearance
	a.settingsUI.syncDraftFromPrefs(a.prefs)

	// 窄视口：预览与控制项在同一滚动列（单栏退化），内容完整。
	tt := ui.NewTester(a.settingsView, 800, 620)
	tt.Frame()
	for _, want := range []string{"预览", "颜色主题", "Markdown 排版主题", "排版微调"} {
		if !tt.HasText(want) {
			t.Fatalf("窄视口单栏缺少 %q: %v", want, tt.Texts())
		}
	}
	if err := tt.Click("颜色主题 石墨"); err != nil {
		t.Fatal(err)
	}
	if a.prefs.Appearance.ColorTheme != "graphite" {
		t.Fatalf("窄视口下主题卡片应可点击生效: %+v", a.prefs.Appearance)
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
	tt := ui.NewTester(a.settingsView, 900, 900)
	tt.Frame()

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
	if a.prefs.Appearance.LatinFont != "" {
		t.Fatalf("选中文字体不应改动西文设置: %q", a.prefs.Appearance.LatinFont)
	}
	if err := tt.Click("跟随中文字体"); err != nil {
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
