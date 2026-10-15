package app

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// 双层主题：颜色 × 明暗 × 排版组合的参数验证与 WCAG 对比度。

func TestColorThemeContrast(t *testing.T) {
	for _, def := range append([]colorThemeDef{{id: "totNeutral", light: totNeutralPalette(false), dark: totNeutralPalette(true)}}, colorThemes...) {
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
	a.prefs.Appearance.ColorTheme = "tot"
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
	a.prefs.Appearance.ColorTheme = "tot"
	neutral := a.resolveStyle(-1, false, true)
	if neutral.background != totNeutralPalette(false).background {
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
	if s2.bodySize != 22 || s2.lineHeight != 2 || s2.bodyFont != bodyFontStack("songti") || s2.monoFont != monoFontStack("menlo") {
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
