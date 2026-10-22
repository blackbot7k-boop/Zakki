package app

import (
	"fmt"
	"image"
	"math"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/egoist/mygo/ui"
)

func TestSketchFocusPreservesDecorations(t *testing.T) {
	for _, dark := range []bool{false, true} {
		for _, font := range []string{"", "hand"} {
			for _, tc := range []struct {
				name, doc string
				mark      string
			}{
				{"heading-1", "# Heading\n\nTail", "heading"},
				{"heading-2", "## Heading\n\nTail", "heading"},
				{"heading-3", "### Heading\n\nTail", "underline"},
				{"heading-6", "###### Heading\n\nTail", "underline"},
				{"bold", "Text **bold** end\n\nTail", "bold"},
				{"inline-code", "Text `code` end\n\nTail", "code"},
				{"unicode-wrap", "文字😀 **中文高亮带有多个字符用于换行验证的长句子和 emoji😀** 结束\n\nTail", "bold"},
				{"code-wrap", "文字😀 `一二三四五六七八九十一二三四五六七八九十` 结束\n\nTail", "code"},
				{"quote", "> 引用中的 **高亮**\n\nTail", "bold"},
				{"list", "- 列表中的 **高亮**\n\nTail", "bold"},
				{"table-header", "| Head | Col |\n| --- | --- |\n| Data | More |\n\nTail", "bold"},
			} {
				t.Run(fmt.Sprintf("dark=%t/font=%s/%s", dark, font, tc.name), func(t *testing.T) {
					a := newTestApp(t)
					t.Cleanup(a.shutdown)
					a.winOpen = true
					a.prefs.Appearance.Style = styleSketch
					a.prefs.Appearance.BodyFont = font
					a.tabs[0].Text = tc.doc
					a.resyncLive(-1)
					a.mdActive = -1
					tt := ui.NewTester(a.view, 300, 600)
					tt.SetDark(dark)
					md := a.resolveStyle(-1, dark, true)
					color := sketchMark(md)
					switch tc.mark {
					case "heading":
						color = withAlpha(md.quoteBorder, 0.42)
					case "underline":
						color = withAlpha(md.accent, 0.75)
					case "code":
						color = md.codeBackground
					}
					target := sketchComposite(color, md.background)
					tolerance, minPixels := 12, 20
					if tc.mark == "underline" {
						tolerance, minPixels = 400, 5
					}
					before := sketchDecorationPixels(tt.Image(), target, tolerance)
					if before < minPixels {
						t.Fatalf("预览装饰未出现：pixels=%d", before)
					}
					a.focusBlock(0, 0, 0)
					tt.Frame()
					after := sketchDecorationPixels(tt.Image(), target, tolerance)
					if after*10 < before*8 {
						t.Errorf("聚焦后装饰消失：preview=%d editing=%d", before, after)
					}
					if a.tabs[0].Text != tc.doc || !tt.Focused("内容") {
						t.Fatalf("聚焦改变正文或丢失输入焦点：doc=%q", a.tabs[0].Text)
					}
					a.blurLive()
					tt.Frame()
					if got := sketchDecorationPixels(tt.Image(), target, tolerance); got != before {
						t.Errorf("切回预览未恢复原装饰：before=%d after=%d", before, got)
					}
				})
			}
		}
	}
}

func TestSketchDecoratedInputSelectionAndIME(t *testing.T) {
	for _, text := range []string{"**中文😀高亮**", "`中文😀代码`", "# 手账标题"} {
		t.Run(text, func(t *testing.T) {
			a := newTestApp(t)
			t.Cleanup(a.shutdown)
			a.prefs.Appearance.Style = styleSketch
			a.prefs.Appearance.BodyFont = "hand"
			a.tabs[0].Text = text
			a.resyncLive(-1)
			n := utf8.RuneCountInString(text)
			a.focusBlock(0, n, n)
			tt := ui.NewTester(a.view, 300, 600)
			tt.Type("续写")
			if !strings.Contains(a.tabs[0].Text, "续写") {
				t.Fatalf("装饰块不能输入：%q", a.tabs[0].Text)
			}
			tt.Key(ui.Super, ui.KeyZ)
			if a.tabs[0].Text != text {
				t.Fatalf("撤销未恢复原文：%q", a.tabs[0].Text)
			}
			tt.Compose("拼音", 2)
			tt.Frame()
			if a.tabs[0].Text != text {
				t.Fatalf("IME 组合态提前写回：%q", a.tabs[0].Text)
			}
			tt.Type("拼音")
			if !strings.Contains(a.tabs[0].Text, "拼音") {
				t.Fatalf("IME 提交丢失：%q", a.tabs[0].Text)
			}
			tt.Frame()
			tt.Key(ui.Super, ui.KeyA)
			if !a.selDoc {
				t.Fatalf("全选未建立文档选区：composing=%t", a.mdComposing)
			}
			tt.Type("替换")
			if got := projectBlock(a.mdBlocks()[0]).text; got != "替换" || a.selDoc {
				t.Fatalf("全选替换失败：doc=%q visible=%q selection=%t", a.tabs[0].Text, got, a.selDoc)
			}
		})
	}
}

func TestSketchInputBackgroundGeometry(t *testing.T) {
	for _, tc := range []struct {
		name, text string
		cell       int
	}{
		{"wrapped", "前😀 **中文高亮带有多个字符用于换行验证的长句子和 emoji😀** 后", -1},
		{"newlines", "前😀 **首行\n次行\n末行** 后", -1},
		{"center-cell", "| 前😀 **中文高亮** 后 | 列 |\n| :---: | ---: |\n| 行文 | **右侧高亮** |", 0},
		{"right-cell", "| 标题 | 列 |\n| :---: | ---: |\n| 行文 | **右侧高亮** 后 |", 3},
	} {
		for _, lineHeight := range []float32{1, 1.75, 3} {
			t.Run(fmt.Sprintf("%s/line-height=%.2f", tc.name, lineHeight), func(t *testing.T) {
				a := newTestApp(t)
				t.Cleanup(a.shutdown)
				a.prefs.Appearance.Style = styleSketch
				a.prefs.Appearance.BodyFont = "hand"
				a.prefs.Appearance.LineHeight = lineHeight
				a.tabs[0].Text = tc.text
				a.resyncLive(-1)
				blk := a.mdBlocks()[0]
				projection := a.previewProjection(blk)
				if tc.cell >= 0 {
					table := a.previewTable(blk)
					projection = table.tableEditor(table.positions[tc.cell])
				}
				style := projection.styles[0]
				at := projection.sourceOffset(style.start)
				a.focusBlock(0, at, at)
				tt := ui.NewTester(a.view, 300, 700)
				md := a.resolveStyle(-1, false, true)
				target := sketchComposite(sketchMark(md), md.background)
				for _, end := range []bool{false, true} {
					if end {
						at = projection.sourceOffset(style.end)
						a.selSyncStart, a.selSyncEnd = at, at
						tt.Frame()
					}
					caret, ok := tt.TextCaret()
					if !ok {
						t.Fatal("编辑器没有光标")
					}
					x := caret.X + 2
					if end {
						x = caret.X - 2
					}
					// 光标内侧 3×3 邻域应至少一像素落在纸底（或装饰叠纸底）：
					// 字形亚像素渲染与手账荧光笔的双笔叠边随内容水平位置浮动，
					// 单像素可能恰好落进笔画；而原生底色一旦画出，邻域内任何
					// 像素都不可能命中纸底，回归仍会被抓住。
					base := int(math.Floor(float64(x)))
					row := int(math.Ceil(float64(caret.Y))) + 1
					covered := false
					for dx := -1; dx <= 1 && !covered; dx++ {
						for dy := -1; dy <= 1 && !covered; dy++ {
							r, g, b, _ := tt.Image().At(base+dx, row+dy).RGBA()
							dr, dg, db := int(r>>8)-int(target.R), int(g>>8)-int(target.G), int(b>>8)-int(target.B)
							covered = dr*dr+dg*dg+db*db <= 12
						}
					}
					if !covered {
						t.Errorf("底色没有覆盖原生文字边界：end=%t caret=%+v want=%v", end, caret, target)
					}
				}
			})
		}
	}
}

func sketchComposite(top, bottom ui.Color) ui.Color {
	blend := func(a, b uint8) uint8 {
		return uint8((int(a)*int(top.A) + int(b)*(255-int(top.A)) + 127) / 255)
	}
	return ui.RGB(blend(top.R, bottom.R), blend(top.G, bottom.G), blend(top.B, bottom.B))
}

func sketchDecorationPixels(img image.Image, target ui.Color, tolerance int) int {
	n := 0
	for y := 70; y < img.Bounds().Dy()-60; y++ {
		for x := 28; x < img.Bounds().Dx()-28; x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			dr, dg, db := int(r>>8)-int(target.R), int(g>>8)-int(target.G), int(b>>8)-int(target.B)
			if dr*dr+dg*dg+db*db <= tolerance {
				n++
			}
		}
	}
	return n
}
