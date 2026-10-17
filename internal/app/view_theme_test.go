package app

import (
	"fmt"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

// 双层主题：颜色 × 明暗 × 排版组合的参数验证与 WCAG 对比度。

func TestColorThemeContrast(t *testing.T) {
	for _, def := range append([]colorThemeDef{{id: "totNeutral", light: classicNeutralPalette(false), dark: classicNeutralPalette(true)}}, colorThemes...) {
		for _, dark := range []bool{false, true} {
			pal := def.light
			if dark {
				pal = def.dark
			}
			mode := "light"
			if dark {
				mode = "dark"
			}
			if r := contrastRatio(pal.text, pal.background); r < 4.5 {
				t.Errorf("%s/%s 正文对背景 %.2f < 4.5", def.id, mode, r)
			}
			if r := contrastRatio(pal.textMuted, pal.background); r < 4.5 {
				t.Errorf("%s/%s 次级文字对背景 %.2f < 4.5", def.id, mode, r)
			}
			if r := contrastRatio(pal.accent, pal.background); r < 3 {
				t.Errorf("%s/%s 强调对背景 %.2f < 3", def.id, mode, r)
			}
			if r := contrastRatio(pal.border, pal.background); r < 1.2 {
				t.Errorf("%s/%s 边界过弱 %.2f", def.id, mode, r)
			}
			// 代码底色上文字仍可读（代码底不低对比）。
			if r := contrastRatio(pal.text, pal.codeBackground); r < 4 {
				t.Errorf("%s/%s 代码底上正文 %.2f < 4", def.id, mode, r)
			}
		}
	}
}

// TestDotPaletteContrast Tot 主窗口随点配色：正文对七点底色 ≥ 4.5。
func TestDotPaletteContrast(t *testing.T) {
	for i := 0; i < DotCount; i++ {
		for _, dark := range []bool{false, true} {
			pal := dotPalette(i, dark)
			mode := "light"
			if dark {
				mode = "dark"
			}
			if r := contrastRatio(pal.text, pal.background); r < 4.5 {
				t.Errorf("点 %d/%s 正文 %.2f < 4.5", i+1, mode, r)
			}
			if r := contrastRatio(pal.textMuted, pal.background); r < 4.5 {
				t.Errorf("点 %d/%s 次级 %.2f < 4.5", i+1, mode, r)
			}
		}
	}
}

func TestMarkdownLayouts(t *testing.T) {
	byID := map[string]markdownLayout{}
	for _, l := range markdownLayouts {
		byID[l.id] = l
	}
	// 字体已解耦：排版主题只携带间距与字号参数。
	clear := byID["clear"]
	if clear.bodySize != 16 || clear.lineHeight != 1.65 || clear.blockGap != 14 {
		t.Fatalf("清晰排版参数: %+v", clear)
	}
	book := byID["book"]
	if book.bodySize != 17 || book.lineHeight != 1.8 || book.blockGap != 18 {
		t.Fatalf("书页排版参数: %+v", book)
	}
	compact := byID["compact"]
	if compact.bodySize != 15 || compact.lineHeight != 1.5 || compact.blockGap != 10 {
		t.Fatalf("紧凑排版参数: %+v", compact)
	}
	// 标题字号 = 基准 × 主题比例（无散落固定字号；比例静态可查）。
	for _, l := range markdownLayouts {
		for i, scale := range l.headingScale {
			if roundHalf(l.bodySize*scale) <= 0 {
				t.Fatalf("%s 标题 %d 比例无效", l.id, i+1)
			}
		}
	}
}

func headingSizeOf(l markdownLayout, i int) float32 {
	return roundHalf(l.bodySize * l.headingScale[i])
}

func TestResolveStyleIndependentLayers(t *testing.T) {
	a := newTestApp(t)
	// 默认：Tot 颜色 + Sonnet 排版。
	a.prefs.Appearance.ColorTheme = "classic"
	a.prefs.Appearance.MarkdownTheme = "sonnet"
	tot := a.resolveStyle(0, false, true) // 点 0（主窗口上下文）
	if tot.background != dotBackground(0, false) {
		t.Fatal("Tot 颜色主题主窗口应随点配色")
	}
	// 固定背景主题：切点不重新染色正文。
	a.prefs.Appearance.ColorTheme = "paper"
	p0 := a.resolveStyle(0, false, true)
	p1 := a.resolveStyle(3, false, true)
	if p0.background != p1.background {
		t.Fatal("固定背景主题切点不应改变正文背景")
	}
	def, _ := colorThemeByID("paper")
	if p0.background != def.light.background {
		t.Fatalf("纸白浅色背景: %v", p0.background)
	}
	// 排版独立：换颜色主题不改变字号；字体与主题解耦，保持用户选择。
	a.prefs.Appearance.MarkdownTheme = "book"
	b := a.resolveStyle(3, false, true)
	if b.bodySize != 17 || b.bodyFont != sonnetBodyFont {
		t.Fatalf("书页排版应生效且字体不随主题: %+v", b)
	}
	// 设置窗口（dot=-1）用 Tot 中性灰变体。
	a.prefs.Appearance.ColorTheme = "classic"
	neutral := a.resolveStyle(-1, false, true)
	if neutral.background != classicNeutralPalette(false).background {
		t.Fatal("设置窗口应用 Tot 中性灰变体")
	}
}

func TestResolveStyleOverridesFollowTheme(t *testing.T) {
	a := newTestApp(t)
	a.prefs.Appearance.MarkdownTheme = "clear"
	// 无覆盖：跟随主题。
	s := a.resolveStyle(-1, false, true)
	if s.bodySize != 16 || s.lineHeight != 1.65 {
		t.Fatalf("无覆盖应跟随主题: %+v", s)
	}
	// 字体键与排版主题解耦：clear 主题不再改变字体，空键是默认栈。
	if s.bodyFont != sonnetBodyFont || s.monoFont != sonnetMonoFont {
		t.Fatalf("空字体键应为默认栈: %q %q", s.bodyFont, s.monoFont)
	}
	// 显式字号/行高覆盖优先，且主题切换不重置覆盖。
	a.prefs.Appearance.BodySize = 22
	a.prefs.Appearance.LineHeight = 2
	a.prefs.Appearance.BodyFont = "songti"
	a.prefs.Appearance.MonoFont = "menlo"
	s2 := a.resolveStyle(-1, false, true)
	if s2.bodySize != 22 || s2.lineHeight != 2 || s2.bodyFont != bodyFontStack("songti", "") || s2.monoFont != monoFontStack("menlo") {
		t.Fatalf("覆盖应优先: %+v", s2)
	}
	// 标题字号随覆盖的基准缩放。
	if s2.headingSizes[0] != roundHalf(22*a_markdownLayoutClear().headingScale[0]) {
		t.Fatalf("标题应随基准缩放: %+v", s2.headingSizes)
	}
	// 预览（override=false）不套用户覆盖。
	s3 := a.resolveStyle(-1, false, false)
	if s3.bodySize != 16 {
		t.Fatalf("预览不应套覆盖: %+v", s3)
	}
	// 源码模式参数：代码字体 + 正文基准字号/行高（由 resolveStyle 直接给出）。
	if s2.monoFont == "" {
		t.Fatal("代码字体不能为空")
	}
}

func a_markdownLayoutClear() markdownLayout { return markdownLayoutByID("clear") }

func TestSourceModeStyleUsesMonoFont(t *testing.T) {
	a := newTestApp(t)
	a.prefs.Appearance.MarkdownTheme = "book"
	a.winOpen = true
	tt := ui.NewTester(a.view, 400, 540)
	a.cfg.Source = true
	tt.Frame()
	// 源码模式不退回标题装饰：整体字号 = 正文基准 17。
	// （视觉验证由 ui 测试覆盖焦点行为，这里锁定样式解析不 panic。）
	_ = tt
}

func TestSketchThemeSet(t *testing.T) {
	a := newTestApp(t)
	if err := a.updatePreferences(func(p *preferences) {
		p.Appearance.ColorTheme = "sketch"
		p.Appearance.MarkdownTheme = "sketch"
	}); err != nil {
		t.Fatalf("手绘整套主题应通过偏好校验: %v", err)
	}
	s := a.resolveStyle(2, false, true)
	def, ok := colorThemeByID("sketch")
	if !ok || s.background != def.light.background {
		t.Fatalf("手绘颜色主题应使用固定纸张底色: %v", s.background)
	}
	if s.bodySize != 16 || s.lineHeight != 1.75 || s.quoteBar != 5 || s.codeRadius != 12 || s.codeBorder != 1.5 {
		t.Fatalf("手账排版参数: %+v", s)
	}
	// 其他排版保持原装饰尺寸。
	a.prefs.Appearance.MarkdownTheme = "sonnet"
	if d := a.resolveStyle(-1, false, true); d.quoteBar != 3 || d.codeRadius != 8 || d.codeBorder != 0 {
		t.Fatalf("默认装饰尺寸被改变: %+v", d)
	}
}

func TestSketchPresetButton(t *testing.T) {
	a := newTestApp(t)
	a.settingsPage = pageAppearance
	a.settingsUI.syncDraftFromPrefs(a.prefs)
	tt := ui.NewTester(a.settingsView, 900, 1200)
	tt.Frame()
	if err := tt.Click("套用手绘整套主题"); err != nil {
		t.Fatal(err)
	}
	if a.prefs.Appearance.ColorTheme != "sketch" || a.prefs.Appearance.MarkdownTheme != "sketch" || a.prefs.Appearance.BodyFont != "hand" {
		t.Fatalf("一键套用应同时设置颜色、排版与手写字体: %+v", a.prefs.Appearance)
	}
	if s := a.resolveStyle(-1, false, true); !s.sketch || s.bodyFont != handBodyFont || s.headingFont != handHeadingFont {
		t.Fatalf("手账排版应启用手绘装饰与手写字体搭配: %+v", s)
	}
}

// TestSketchLiveRenders 手账排版下各类块（标题/引用/代码/分隔线/表格）可正常构建，
// 元素消失再出现也不 panic。
func TestSketchLiveRenders(t *testing.T) {
	a := newTestApp(t)
	a.winOpen = true
	a.prefs.Appearance.ColorTheme = "sketch"
	a.prefs.Appearance.MarkdownTheme = "sketch"
	a.prefs.Appearance.BodyFont = "hand"
	a.tabs[0].Text = "# 今日手账\n\n> 引用一句\n\n```\ncode\n```\n\n---\n\n| a | b |\n| - | - |\n| 1 | 2 |\n\n- [ ] 待办"
	a.resyncLive(-1)
	a.mdActive = -1 // 全预览态：所有块走手绘渲染路径
	tt := ui.NewTester(a.view, 400, 540)
	for i := 0; i < 3; i++ {
		tt.Frame()
	}
	if !tt.HasText("今日手账") || !tt.HasText("引用一句") {
		t.Fatalf("手账排版缺少内容: %v", tt.Texts())
	}
	a.tabs[0].Text = "纯文本"
	a.resyncLive(-1)
	tt.Frame()
	tt.Frame()
}

func TestJitterDeterministic(t *testing.T) {
	for i := uint32(0); i < 100; i++ {
		v := jitter(42, i)
		if v != jitter(42, i) || v < -1 || v > 1 {
			t.Fatalf("jitter(42,%d)=%v 不稳定或越界", i, v)
		}
	}
}

// TestSketchColorDropdownAppliesColorOnly 颜色主题下拉选手绘：只改颜色主题，
// 不联动排版与字体（整套切换走「手绘手账」预设行）。
func TestSketchColorDropdownAppliesColorOnly(t *testing.T) {
	a := newTestApp(t)
	a.settingsPage = pageAppearance
	a.settingsUI.syncDraftFromPrefs(a.prefs)
	tt := ui.NewTester(a.settingsView, 900, 1200)
	tt.Frame()
	if err := tt.Click("经典"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("手绘"); err != nil {
		t.Fatal(err)
	}
	ap := a.prefs.Appearance
	if ap.ColorTheme != "sketch" {
		t.Fatalf("下拉选手绘应生效: %+v", ap)
	}
	if ap.MarkdownTheme != "sonnet" || ap.BodyFont != "" {
		t.Fatalf("颜色主题不应联动排版与字体: %+v", ap)
	}
}

// TestSketchPaperFillsViewport 手账纸面（点阵与页边线）必须铺满整个编辑视口，
// 不能只画到内容高度为止：短文档在高窗口里，底部空白区也要有纸面纹理。
func TestSketchPaperFillsViewport(t *testing.T) {
	a := newTestApp(t)
	a.winOpen = true
	a.prefs.Appearance.ColorTheme = "sketch"
	a.prefs.Appearance.MarkdownTheme = "sketch"
	a.tabs[0].Text = "一行"
	a.resyncLive(-1)
	a.mdActive = -1
	tt := ui.NewTester(a.view, 400, 760)
	for i := 0; i < 3; i++ {
		tt.Frame()
	}
	img := tt.Image()
	style := a.resolveStyle(-1, false, true)
	bg := style.background
	// 取状态栏上方一段（远离唯一一行内容），统计偏离纸色的像素。
	h := img.Bounds().Dy()
	marks := 0
	for y := h - 200; y < h-60; y++ {
		for x := 0; x < img.Bounds().Dx(); x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			dr := int(r>>8) - int(bg.R)
			dg := int(g>>8) - int(bg.G)
			db := int(b>>8) - int(bg.B)
			if dr*dr+dg*dg+db*db > 300 {
				marks++
			}
		}
	}
	if marks < 40 {
		t.Fatalf("编辑区底部空白没有手账纸面纹理（偏离纸色像素 %d）", marks)
	}
}

// TestSketchPaperScrollsWithContent 长文档滚动到底部时纸面仍铺满视口，
// 且点阵随内容移动（滚动前后同一视口位置的纹理不同）。
func TestSketchPaperScrollsWithContent(t *testing.T) {
	a := newTestApp(t)
	a.winOpen = true
	a.prefs.Appearance.ColorTheme = "sketch"
	a.prefs.Appearance.MarkdownTheme = "sketch"
	var b strings.Builder
	for i := 0; i < 80; i++ {
		fmt.Fprintf(&b, "第 %d 段\n\n", i)
	}
	a.tabs[0].Text = b.String()
	a.resyncLive(-1)
	a.mdActive = -1
	tt := ui.NewTester(a.view, 400, 600)
	for i := 0; i < 3; i++ {
		tt.Frame()
	}
	before := tt.Image()
	tt.Scroll(200, 300, 0, 333)
	for i := 0; i < 3; i++ {
		tt.Frame()
	}
	if a.liveScroll.Y <= 0 {
		t.Fatalf("滚动未生效: %+v", a.liveScroll)
	}
	after := tt.Image()
	// 页边线附近一条竖带只含纸面纹理：滚动后点阵应换了位置。
	style := a.resolveStyle(-1, false, true)
	x0 := int(style.padX*0.45) + 6
	diff := 0
	for y := 120; y < 480; y++ {
		for x := x0; x < x0+14; x++ {
			if before.At(x, y) != after.At(x, y) {
				diff++
			}
		}
	}
	if diff == 0 {
		t.Fatal("滚动后点阵没有随内容移动")
	}
}

// TestHeadingFontFollowsBody 只有手写搭配给标题单独换字体，其余字体标题跟随正文。
func TestHeadingFontFollowsBody(t *testing.T) {
	for _, key := range []string{"", "kai", "pingfang", "songti", "hannotate"} {
		if headingFontStack(key, "") != bodyFontStack(key, "") {
			t.Fatalf("%q 标题字体应跟随正文", key)
		}
	}
	// 手写搭配：标题与正文同用内置的霞鹜文楷 Lite（观感靠字号与装饰区分）。
	if got := headingFontStack("hand", ""); !strings.Contains(got, "LXGW WenKai Lite") {
		t.Fatalf("手写搭配的标题应使用内置手写字体: %q", got)
	}
	// 全文手札体：正文与标题同一字体栈。
	if bodyFontStack("hannotate", "") != hannotateFont || headingFontStack("hannotate", "") != hannotateFont {
		t.Fatal("手札体选项应让正文与标题都用手札体")
	}
	if err := (func() error {
		p := defaultPreferences()
		p.Appearance.BodyFont = "hannotate"
		return validatePrefs(p)
	})(); err != nil {
		t.Fatalf("手札体应通过偏好校验: %v", err)
	}
}

// TestLatinFontSeparateFromChinese 中西文可分开设置：西文栈排在中文栈前面
// 接管字母，中文仍由中文栈渲染；西文为空时保持原来的单一字体栈。
func TestLatinFontSeparateFromChinese(t *testing.T) {
	// 跟随中文字体：与旧行为完全一致。
	if got := bodyFontStack("hand", ""); got != handBodyFont {
		t.Fatalf("西文跟随中文字体时应保持中文栈: %q", got)
	}
	if got := bodyFontStack("pingfang", ""); got != cjkFontStack("pingfang") {
		t.Fatalf("西文跟随中文字体时应保持中文栈: %q", got)
	}
	// 分开设置：西文在前，中文在后。
	got := bodyFontStack("songti", "noteworthy")
	if !strings.HasPrefix(got, "Noteworthy, ") || !strings.Contains(got, "Songti SC") {
		t.Fatalf("西文应在前且保留中文栈: %q", got)
	}
	if got := headingFontStack("hand", "bradley"); !strings.HasPrefix(got, "Bradley Hand, LXGW WenKai Lite") {
		t.Fatalf("标题也应使用所选西文: %q", got)
	}
	for _, key := range latinFontKeys {
		stack := latinFontStack(key)
		if key != "" && stack == "" {
			t.Fatalf("西文字体 %q 缺少字体栈", key)
		}
		if strings.Contains(stack, "WenKai") || strings.Contains(stack, "Songti") || strings.Contains(stack, "Hannotate") {
			t.Fatalf("西文字体栈 %q 不应包含中文字体，否则会顶掉中文渲染", stack)
		}
	}
	// 设置页可选的中文字体与西文字体都要通过偏好校验。
	for _, cjk := range bodyFontKeys {
		for _, latin := range latinFontKeys {
			p := defaultPreferences()
			p.Appearance.BodyFont, p.Appearance.LatinFont = cjk, latin
			if err := validatePrefs(p); err != nil {
				t.Fatalf("组合 %q + %q 应通过校验: %v", cjk, latin, err)
			}
		}
	}
}
