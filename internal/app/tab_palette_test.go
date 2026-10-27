package app

import (
	"fmt"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"zakki/locales"
)

func TestTabPillColorsDefaultPalette(t *testing.T) {
	for i, hex := range tabPaletteColors {
		t.Run(hex, func(t *testing.T) {
			base := ui.Hex(hex)
			got := tabPillColors(base)
			wantText := ui.RGB(36, 36, 38)
			if i == 0 || i == 4 || i == 5 || i == 10 {
				wantText = ui.RGB(248, 248, 250)
			}
			if got.text != wantText {
				t.Fatalf("文字 = %+v, want %+v", got.text, wantText)
			}
			if ratio := contrastRatio(got.text, got.background); ratio < 5 {
				t.Fatalf("默认配色对比度 = %.6f, want >= 5", ratio)
			}
			if contrastRatio(wantText, base) >= 5 && got.background != base {
				t.Fatalf("已足够清楚的背景不应改变: got %+v, want %+v", got.background, base)
			}
			if i == 4 && got.background == base {
				t.Fatal("蓝底应压暗以保留柔白字，而不是直接换成黑字")
			}
		})
	}
}

func TestTabContrastMixRespectsBudget(t *testing.T) {
	base, ink := ui.Hex("#0f75e9"), ui.RGB(248, 248, 250)
	black := ui.RGB(0, 0, 0)
	if got := tabContrastMix(base, black, ink, 3, 0.12); got != base {
		t.Fatalf("达标时应保留原色: got %+v, want %+v", got, base)
	}
	if got := tabContrastMix(base, black, ink, 5, 0.01); got != base.Mix(black, 0.01) {
		t.Fatalf("预算不足时不得越界: got %+v", got)
	}
	got := tabContrastMix(base, black, ink, 5, 0.12)
	if ratio := contrastRatio(got, ink); ratio < 5 {
		t.Fatalf("调整后对比度 = %.9f, want >= 5", ratio)
	}
	if got == base.Mix(black, 0.12) {
		t.Fatal("可提前达标时不应耗尽背景调整预算")
	}
	fallback := tabContrastMix(ui.RGB(36, 36, 38), black, base, 4.5, 1)
	if ratio := contrastRatio(fallback, base); ratio < 4.5 {
		t.Fatalf("文字向纯黑兜底后对比度 = %.9f, want >= 4.5", ratio)
	}
}

func TestTabPillColorsCustomColorGrid(t *testing.T) {
	for r := 0; r <= 255; r += 17 {
		for g := 0; g <= 255; g += 17 {
			for b := 0; b <= 255; b += 17 {
				base := ui.RGB(uint8(r), uint8(g), uint8(b))
				got := tabPillColors(base)
				if ratio := contrastRatio(got.text, got.background); ratio < 4.5 {
					t.Fatalf("背景 %+v 的配色 %+v 对比度 = %.9f, want >= 4.5", base, got, ratio)
				}
				if got.background.A != 255 || got.text.A != 255 {
					t.Fatalf("稳定态不得用透明度降低对比度: %+v", got)
				}
				lo, hi := base.Mix(ui.RGB(0, 0, 0), 0.12), base.Mix(ui.RGB(255, 255, 255), 0.12)
				if got.background.R < lo.R || got.background.R > hi.R ||
					got.background.G < lo.G || got.background.G > hi.G ||
					got.background.B < lo.B || got.background.B > hi.B {
					t.Fatalf("背景调整超过 12%%: base %+v, got %+v", base, got.background)
				}
			}
		}
	}
}

func TestTabPillColorsThresholds(t *testing.T) {
	for v := 0; v <= 255; v++ {
		base := ui.RGB(uint8(v), uint8(v), uint8(v))
		got := tabPillColors(base)
		if ratio := contrastRatio(got.text, got.background); ratio < 4.5 {
			t.Fatalf("灰阶 %d 对比度 %.9f, want >= 4.5", v, ratio)
		}
	}
}

func TestTabPillRenderedAndRenameShareColors(t *testing.T) {
	for _, dark := range []bool{false, true} {
		for _, hex := range []string{"#d4253a", "#ffd01c", "#0f75e9", "#808080"} {
			t.Run(fmt.Sprintf("dark=%t/%s", dark, hex), func(t *testing.T) {
				a := newTestApp(t)
				a.tabs = a.tabs[:1]
				a.tabs[0].Name, a.tabs[0].Color = "项目 Project", hex
				tt := ui.NewTester(func(c *ui.Context) {
					c.Root().Background(headerBackground(dark))
					a.tabStrip(c, dark)
				}, 400, 38)
				want := tabPillColors(ui.Hex(hex))
				check := func() {
					t.Helper()
					if a.lastBody.color != want.background || a.lastBody.textColor != want.text {
						t.Fatalf("显示与重命名应共用配色: body %+v, want %+v", a.lastBody, want)
					}
					p := tabPixel(tt, a.lastBody.cx, 19+pillHeight/2-2)
					if p.R != want.background.R || p.G != want.background.G || p.B != want.background.B {
						t.Fatalf("实际胶囊背景 = %+v, want %+v", p, want.background)
					}
					found := false
					for y := 10; y < 27; y++ {
						for x := int(a.lastBody.cx-a.lastBody.w/2+pillPadX) + 3; x < int(a.lastBody.cx+a.lastBody.w/2-pillPadX); x++ {
							p := tabPixel(tt, float32(x), float32(y))
							if absByte(p.R, want.text.R)+absByte(p.G, want.text.G)+absByte(p.B, want.text.B) <= 6 {
								found = true
							}
						}
					}
					if !found {
						t.Fatalf("实际标题未使用配套文字色 %+v", want.text)
					}
				}
				check()
				a.beginRename(0)
				tt.Frame()
				if !tt.HasText(locales.RenameTabField()) {
					t.Fatal("未进入重命名输入框")
				}
				check()
				a.cancelRename()
				tt.Frame()
				check()
				if a.tabs[0].Color != hex || a.snapshotFile().Tabs[0].Color != hex {
					t.Fatal("显示配色不得改写标签原色或持久快照")
				}
			})
		}
	}
}

func TestTabPillMorphChangesColorsWhileTitleIsHidden(t *testing.T) {
	start := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)
	for _, from := range tabPaletteColors {
		for _, to := range tabPaletteColors {
			fp, tp := tabPillColors(ui.Hex(from)), tabPillColors(ui.Hex(to))
			m := tabMorphState{active: true, start: start, duration: time.Second,
				fromBody: tabBody{w: 100, h: pillHeight, color: fp.background, textColor: fp.text, title: "旧标题"},
				toBody:   tabBody{w: 100, h: pillHeight, color: tp.background, textColor: tp.text, title: "新标题"},
			}
			for step := 0; step <= 100; step++ {
				body := m.bodyAt(start.Add(time.Duration(step) * 10 * time.Millisecond))
				fa, ta := titleAlphas(float32(step) / 100)
				if fa > 0 && (body.color != fp.background || body.textColor != fp.text) {
					t.Fatalf("%s → %s 旧标题可见时提前切色: step %d body %+v", from, to, step, body)
				}
				if ta > 0 && (body.color != tp.background || body.textColor != tp.text) {
					t.Fatalf("%s → %s 新标题可见时配色未就位: step %d body %+v", from, to, step, body)
				}
				if fa == 0 && ta == 0 && body.title != "" {
					t.Fatal("中途再次切换时不应复活已淡出的标题")
				}
			}
		}
	}
}

func TestTabPillRetargetDuringHiddenPhase(t *testing.T) {
	a, tt, clk := newAnimTestApp(t)
	a.tabs[0].Name, a.tabs[2].Name, a.tabs[4].Name = "红色", "黄色", "蓝色"
	tt.Frame()
	a.switchTab(2)
	tt.Frame()
	clk.advance(a.tabAnimDuration() * 45 / 100)
	tt.Frame()
	middle := a.morph.bodyAt(clk.now())
	a.switchTab(4)
	tt.Frame()
	if !a.morph.active || a.morph.fromBody != middle || a.morph.fromBody.title != "" {
		t.Fatalf("连续切换应接续当前底色且不复活已淡出的文字: %+v", a.morph)
	}
	clk.advance(a.tabAnimDuration())
	tt.Frame()
	if a.morph.active || a.lastBody.title != "蓝色" {
		t.Fatalf("连续切换后未在目标标签就位: %+v", a.lastBody)
	}
}

func TestTabPillRenameStopsInFlightMorph(t *testing.T) {
	a, tt, clk := newAnimTestApp(t)
	a.tabs[0].Name, a.tabs[4].Name = "红色", "蓝色"
	tt.Frame()
	a.switchTab(4)
	tt.Frame()
	clk.advance(a.tabAnimDuration() / 4)
	tt.Frame()
	if !a.morph.active {
		t.Fatal("测试应在动画期间进入重命名")
	}
	a.beginRename(4)
	tt.Frame()
	if a.morph.active {
		t.Fatal("重命名输入应在目标胶囊稳定就位，不跟随旧动画")
	}
	want := tabPillColors(a.tabColor(4))
	if a.lastBody.color != want.background || a.lastBody.textColor != want.text {
		t.Fatalf("重命名背景与文字配色不一致: %+v", a.lastBody)
	}
}
