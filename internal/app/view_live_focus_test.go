package app

import (
	"fmt"
	"image"
	"image/draw"
	"math"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/egoist/mygo/ui"
)

func TestLiveFocusAppearance(t *testing.T) {
	cases := []struct{ name, text string }{
		{"plain", "普通正文 Normal text 😀"},
		{"bold", "前 **粗体 Bold** 后"},
		{"italic-mixed", "前 *斜体 Italic* 后"},
		{"italic-whole", "*斜体 Italic*"},
		{"link", "前 [链接 Link](https://example.com) 后"},
		{"code", "前 `code iii WWW` 后"},
		{"code-unicode", "前 `中文😀 code` 后"},
		{"combined", "支持 **粗体**、*斜体*、`代码`、[链接](https://example.com)、引用和代码块。"},
		{"quote", "> 前 *Italic* `code` [链接](https://example.com) 后"},
		{"list", "- 前 *Italic* `code` [链接](https://example.com) 后"},
		{"task", "- [ ] 前 *Italic* `code` [链接](https://example.com) 后"},
	}
	for level := 1; level <= 6; level++ {
		cases = append(cases, struct{ name, text string }{fmt.Sprint("heading-", level), strings.Repeat("#", level) + " 中文 Heading"})
	}
	for _, layout := range []string{"sketch", "sonnet"} {
		for _, dark := range []bool{false, true} {
			for _, font := range []string{"", "hand"} {
				for _, tc := range cases {
					t.Run(fmt.Sprintf("%s/dark=%t/font=%s/%s", layout, dark, font, tc.name), func(t *testing.T) {
						a := newFocusAppearanceApp(t, layout, font, tc.text+"\n\n后续段落")
						tt := ui.NewTester(a.view, 1000, 400)
						tt.SetDark(dark)
						next, ok := tt.Find("后续段落")
						if !ok {
							t.Fatal("没有渲染后续段落")
						}
						region := image.Rect(28, 70, 972, int(next.Y)-2)
						before := cloneFocusImage(tt.Image())
						p := a.previewProjection(a.mdBlocks()[0])
						n := utf8.RuneCountInString(a.mdBlocks()[0].raw)
						a.focusBlock(0, n, n)
						tt.Frame()
						caret, ok := tt.TextCaret()
						if !ok || !tt.Focused("内容") {
							t.Fatal("没有编辑焦点或原生光标")
						}
						if after, ok := tt.Find("后续段落"); !ok || after != next {
							t.Errorf("聚焦改变后续段落位置：preview=%+v editing=%+v", next, after)
						}
						if diff := focusPixelDiff(before, tt.Image(), region, caret); diff != 0 {
							t.Errorf("聚焦改变文字外观：%d pixels", diff)
						}
						editing := cloneFocusImage(tt.Image())
						positions := []int{0}
						for _, s := range p.styles {
							positions = append(positions, p.sourceOffset(s.start), p.sourceOffset((s.start+s.end)/2), p.sourceOffset(s.end))
						}
						for _, pos := range positions {
							a.selSyncStart, a.selSyncEnd = pos, pos
							tt.Frame()
							moved, ok := tt.TextCaret()
							if !ok {
								t.Fatal("块内移动后光标消失")
							}
							if diff := focusPixelDiff(editing, tt.Image(), region, caret, moved); diff != 0 {
								t.Errorf("块内移动改变文字：pos=%d pixels=%d", pos, diff)
							}
						}
						a.focusBlock(1, 0, 0)
						tt.Frame()
						// 排除新焦点块的光标：左内边距变化会移动光标列，其行盒
						// 顶部可能探进比较区，与「未恢复预览」无关。
						final, ok := tt.TextCaret()
						if !ok {
							t.Fatal("切走后光标消失")
						}
						if diff := focusPixelDiff(before, tt.Image(), region, final); diff != 0 {
							t.Errorf("切走后未恢复预览：%d pixels", diff)
						}
						if got := a.tabs[0].Text; got != tc.text+"\n\n后续段落" {
							t.Fatalf("仅聚焦就改写了正文：%q", got)
						}
					})
				}
			}
		}
	}
}

func TestLiveFocusCodeWrap(t *testing.T) {
	for _, width := range []int{240, 280, 320} {
		for _, code := range []string{"code iii WWW ", "中文😀 iii WWW", "code iii WWW code iii WWW"} {
			t.Run(fmt.Sprintf("width=%d/%s", width, code), func(t *testing.T) {
				a := newFocusAppearanceApp(t, "sketch", "hand", "支持 `"+code+"`，尾文。\n\n后续段落")
				tt := ui.NewTester(a.view, width, 500)
				before, ok := tt.Find("后续段落")
				if !ok {
					t.Fatal("没有后续段落")
				}
				n := utf8.RuneCountInString(a.mdBlocks()[0].raw)
				a.focusBlock(0, n, n)
				tt.Frame()
				if after, ok := tt.Find("后续段落"); !ok || before != after {
					t.Fatalf("仅聚焦行内代码就改变换行：preview=%+v editing=%+v", before, after)
				}
			})
		}
	}
}

func TestLiveContentThemeTextColor(t *testing.T) {
	for _, dark := range []bool{false, true} {
		for level := 0; level <= 6; level++ {
			t.Run(fmt.Sprintf("dark=%t/heading=%d", dark, level), func(t *testing.T) {
				prefix := ""
				if level > 0 {
					prefix = strings.Repeat("#", level) + " "
				}
				a := newFocusAppearanceApp(t, "sketch", "hand", prefix+"中文 Heading\n\n后续段落")
				tt := ui.NewTester(a.view, 500, 400)
				tt.SetDark(dark)
				md := a.resolveStyle(-1, dark, true)
				for _, focus := range []bool{false, true} {
					if focus {
						n := utf8.RuneCountInString(a.mdBlocks()[0].raw)
						a.focusBlock(0, n, n)
						tt.Frame()
					}
					next, ok := tt.Find("后续段落")
					if !ok {
						t.Fatal("没有后续段落")
					}
					pixels := 0
					img := tt.Image()
					for y := 70; y < int(next.Y)-2; y++ {
						for x := 28; x < 472; x++ {
							r, g, b, _ := img.At(x, y).RGBA()
							if uint8(r>>8) == md.text.R && uint8(g>>8) == md.text.G && uint8(b>>8) == md.text.B {
								pixels++
							}
						}
					}
					if pixels == 0 {
						t.Errorf("正文未使用主题墨色：focus=%t want=%v", focus, md.text)
					}
				}
			})
		}
	}
}

func TestLiveInlineCompositionAndReplacement(t *testing.T) {
	for _, raw := range []string{"前 *中文😀 italic* 后", "前 `中文😀 code` 后", "前 [中文😀 link](https://example.com) 后"} {
		t.Run(raw, func(t *testing.T) {
			a := newFocusAppearanceApp(t, "sketch", "hand", raw)
			p := a.previewProjection(a.mdBlocks()[0])
			s := p.styles[0]
			pos := p.sourceOffset(s.start + 1)
			a.focusBlock(0, pos, pos)
			tt := ui.NewTester(a.view, 500, 400)
			before := cloneFocusImage(tt.Image())
			caret, ok := tt.TextCaret()
			if !ok {
				t.Fatal("没有原生光标")
			}
			tt.Compose("拼音😀", 3)
			tt.Frame()
			if a.tabs[0].Text != raw {
				t.Fatal("组合文本提前写回文档")
			}
			tt.Compose("", 0)
			tt.Frame()
			region := image.Rect(28, 70, 472, 140)
			if diff := focusPixelDiff(before, tt.Image(), region, caret); diff != 0 {
				t.Errorf("取消组合后样式未恢复：%d pixels", diff)
			}
			tt.Type("续")
			want := string([]rune(raw)[:pos]) + "续" + string([]rune(raw)[pos:])
			if a.tabs[0].Text != want {
				t.Fatalf("格式内输入丢失包装：got=%q want=%q", a.tabs[0].Text, want)
			}
			tt.Key(ui.Super, ui.KeyZ)
			if a.tabs[0].Text != raw {
				t.Fatal("撤销未恢复带格式的文档")
			}
			a.selSyncStart, a.selSyncEnd = s.content, s.close
			tt.Frame()
			tt.Type("替换")
			want = string([]rune(raw)[:s.content]) + "替换" + string([]rune(raw)[s.close:])
			if a.tabs[0].Text != want {
				t.Fatalf("格式内选区替换丢失包装：got=%q want=%q", a.tabs[0].Text, want)
			}
		})
	}
}

func TestTableCellFocusInlineAppearance(t *testing.T) {
	for _, layout := range []string{"sketch", "sonnet"} {
		for _, divider := range []string{":---", ":---:", "---:"} {
			for _, row := range []int{0, 1} {
				t.Run(fmt.Sprintf("%s/%s/row=%d", layout, divider, row), func(t *testing.T) {
					cell := "前 *Italic* `iii WWW` [链接](https://example.com) 后"
					a := newFocusAppearanceApp(t, layout, "hand", "| "+cell+" |\n| "+divider+" |\n| "+cell+" |")
					editing := false
					// 单独渲染真实表格网格，固定工具栏引起的外部位移。
					tt := ui.NewTester(func(c *ui.Context) {
						md := a.resolveStyle(-1, false, true)
						theme := md.apply(*c.Theme())
						c.SetTheme(&theme)
						ui.Column(c).Fill().Font(md.bodyFont).FontSize(md.bodySize).LineHeight(md.lineHeight).
							TextColor(md.text).Children(func() {
							a.tableGrid(c, &theme, md, a.mdBlocks()[0], editing)
						})
					}, 600, 250)
					before := cloneFocusImage(tt.Image())
					a.focusTableCell(0, row, 0)
					editing = true
					tt.Frame()
					caret, ok := tt.TextCaret()
					if !ok {
						t.Fatal("单元格没有原生光标")
					}
					if diff := focusPixelDiff(before, tt.Image(), before.Bounds(), caret); diff != 0 {
						t.Errorf("表格单元格聚焦改变外观：%d pixels", diff)
					}
				})
			}
		}
	}
}

func newFocusAppearanceApp(t *testing.T, layout, font, doc string) *app {
	t.Helper()
	a := newTestApp(t)
	t.Cleanup(a.shutdown)
	a.winOpen = true
	if layout == styleSketch {
		a.prefs.Appearance.Style = styleSketch
	} else {
		a.prefs.Appearance.ColorTheme = layout
		a.prefs.Appearance.MarkdownTheme = layout
	}
	a.prefs.Appearance.BodyFont = font
	a.tabs[0].Text = doc
	a.resyncLive(-1)
	a.mdActive = -1
	return a
}

func cloneFocusImage(src image.Image) *image.RGBA {
	dst := image.NewRGBA(src.Bounds())
	draw.Draw(dst, dst.Bounds(), src, src.Bounds().Min, draw.Src)
	return dst
}

func focusPixelDiff(a, b image.Image, region image.Rectangle, carets ...ui.Rect) int {
	region = region.Intersect(a.Bounds()).Intersect(b.Bounds())
	diff := 0
	for y := region.Min.Y; y < region.Max.Y; y++ {
		for x := region.Min.X; x < region.Max.X; x++ {
			masked := false
			for _, r := range carets {
				if x >= int(math.Floor(float64(r.X)))-2 && x <= int(math.Ceil(float64(r.X+r.W)))+2 &&
					y >= int(math.Floor(float64(r.Y)))-2 && y <= int(math.Ceil(float64(r.Y+r.H)))+2 {
					masked = true
				}
			}
			if !masked && a.At(x, y) != b.At(x, y) {
				diff++
			}
		}
	}
	return diff
}
