package app

import (
	"fmt"
	"image"
	"math"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestMyGoTextBackgroundRanges(t *testing.T) {
	for _, multiline := range []bool{false, true} {
		for _, overlap := range []bool{false, true} {
			t.Run(fmt.Sprintf("multiline=%t/overlap=%t", multiline, overlap), func(t *testing.T) {
				value := "abcd"
				yellow, blue := ui.Hex("#f0d050"), ui.Hex("#4080d0")
				selection := ui.RGBA(220, 40, 100, 0.5)
				start, end, sync := 0, 0, true
				tt := ui.NewTester(func(c *ui.Context) {
					theme := *c.Theme()
					theme.Background, theme.Selection = ui.Hex("#ffffff"), selection
					c.SetTheme(&theme)
					var ta ui.Element
					if multiline {
						ta = ui.TextAreaBase(c, &value).Lines(1, 10)
					} else {
						ta = ui.TextInputBase(c, &value)
					}
					ta.Width(240).Height(40).Font("monospace").FontSize(20).AutoFocus().FocusRing(false)
					ta.TextRanges(ui.TextRange{Start: 0, End: 4, Background: yellow})
					if overlap {
						ta.TextRanges(ui.TextRange{Start: 1, End: 3, Background: blue})
						ta.TextRanges(ui.TextRange{Start: 2, End: 4, Color: ui.Hex("#a02020"), Weight: 700})
					}
					if sync {
						ta.SetTextSelection(start, end)
						sync = false
					}
				}, 260, 100)
				var samples []image.Point
				for i := 0; i < 4; i++ {
					start, end, sync = i, i, true
					tt.Frame()
					caret, ok := tt.TextCaret()
					if !ok {
						t.Fatal("输入框没有原生光标")
					}
					point := image.Pt(int(caret.X)+2, int(math.Ceil(float64(caret.Y)))+1)
					samples = append(samples, point)
					target := yellow
					if overlap && (i == 1 || i == 2) {
						target = blue
					}
					assertTextBackgroundPixel(t, tt.Image(), point, target)
				}
				start, end, sync = 0, 4, true
				tt.Frame()
				for i, point := range samples {
					target := yellow
					if overlap && (i == 1 || i == 2) {
						target = blue
					}
					assertTextBackgroundPixel(t, tt.Image(), point, sketchComposite(selection, target))
				}
			})
		}
	}
}

func TestSketchCompositionMovesBackground(t *testing.T) {
	a := newTestApp(t)
	t.Cleanup(a.shutdown)
	a.prefs.Appearance.Style = styleSketch
	a.tabs[0].Text = "abc **marked** end"
	a.resyncLive(-1)
	a.focusBlock(0, 0, 0)
	tt := ui.NewTester(a.view, 300, 600)
	md := a.resolveStyle(-1, false, true)
	target := sketchComposite(sketchMark(md), md.background)
	bounds := func(img image.Image) image.Rectangle {
		var result image.Rectangle
		for y := 60; y < 400; y++ {
			for x := 28; x < 272; x++ {
				r, g, b, _ := img.At(x, y).RGBA()
				dr, dg, db := int(r>>8)-int(target.R), int(g>>8)-int(target.G), int(b>>8)-int(target.B)
				if dr*dr+dg*dg+db*db <= 12 {
					result = result.Union(image.Rect(x, y, x+1, y+1))
				}
			}
		}
		return result
	}
	before := bounds(tt.Image())
	tt.Compose("一段较长的中文组合文本用于触发换行", 18)
	tt.Frame()
	after := bounds(tt.Image())
	if before.Empty() || after.Empty() || before == after {
		t.Fatalf("组合文本重排时底色未跟随：before=%v after=%v", before, after)
	}
	if a.tabs[0].Text != "abc **marked** end" {
		t.Fatalf("组合文本提前写入正文：%q", a.tabs[0].Text)
	}
	tt.Compose("", 0)
	tt.Frame()
	if got := bounds(tt.Image()); got != before {
		t.Fatalf("取消组合后底色未恢复：before=%v after=%v", before, got)
	}
}

func assertTextBackgroundPixel(t *testing.T, img image.Image, point image.Point, target ui.Color) {
	t.Helper()
	r, g, b, _ := img.At(point.X, point.Y).RGBA()
	dr, dg, db := int(r>>8)-int(target.R), int(g>>8)-int(target.G), int(b>>8)-int(target.B)
	if dr*dr+dg*dg+db*db > 12 {
		t.Errorf("底色/选区覆盖错误：point=%v got=%v want=%v", point, img.At(point.X, point.Y), target)
	}
}

func BenchmarkMyGoNativeTextBackground(b *testing.B) {
	for _, background := range []bool{false, true} {
		b.Run(fmt.Sprintf("background=%t", background), func(b *testing.B) {
			value := strings.Repeat("plain bold text\n", 20000)
			ranges := make([]ui.TextRange, 20000)
			for i := range ranges {
				ranges[i] = ui.TextRange{Start: i*16 + 6, End: i*16 + 10, Weight: 700}
				if background {
					ranges[i].Background = ui.Hex("#f0d050")
				}
			}
			tt := ui.NewTester(func(c *ui.Context) {
				ui.TextAreaBase(c, &value).FillWidth().Height(400).Font("monospace").FontSize(16).TextRanges(ranges...)
			}, 300, 400)
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				tt.Frame()
			}
		})
	}
}
