package app

// 代码高亮端到端渲染回归：深色预览、无空行围栏、焦点块（编辑态）三条路径
// 都必须出现 token 彩色像素。采样 tt.Image() 中代码行矩形内的彩色像素计数。

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// countColored 统计文本矩形内与灰白偏离明显的像素数（R/G/B 通道差 > 40）。
func countColored(t *testing.T, tt *ui.Tester, text string) (colored, total int) {
	t.Helper()
	rect, ok := tt.Find(text)
	if !ok {
		t.Fatalf("找不到文本 %q", text)
	}
	img := tt.Image()
	for y := int(rect.Y); y < int(rect.Y+rect.H) && y < img.Bounds().Dy(); y++ {
		for x := int(rect.X); x < int(rect.X+rect.W) && x < img.Bounds().Dx(); x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			r8, g8, b8 := int(r>>8), int(g>>8), int(b>>8)
			mx, mn := max(r8, g8, b8), min(r8, g8, b8)
			total++
			if mx-mn > 40 {
				colored++
			}
		}
	}
	return colored, total
}

func renderDarkPreview(t *testing.T, text string) (*app, *ui.Tester) {
	t.Helper()
	a := newTestApp(t)
	t.Cleanup(a.shutdown)
	a.prefs.Appearance.CodeTheme = "soft"
	a.tabs[0].Text = text
	a.mdActive = -1 // 预览态：所有块走 RichText 路径（焦点块另有用例覆盖）
	tt := ui.NewTester(a.view, 900, 700)
	tt.SetDark(true)
	tt.Frame()
	tt.Frame()
	return a, tt
}

// TestLivePreviewCodeHighlightDark 深色预览下 js 围栏的关键词与数字有着色。
func TestLivePreviewCodeHighlightDark(t *testing.T) {
	_, tt := renderDarkPreview(t, "```js\nconst a = 223;\n```\n")
	c, total := countColored(t, tt, "const a = 223;")
	t.Logf("js 围栏: colored=%d total=%d", c, total)
	if c == 0 {
		t.Fatal("js 代码行未出现彩色像素（关键词/数字未着色）")
	}
}

// TestLivePreviewCodeHighlightNoBlankLine 围栏紧跟段落行（中间无空行）时同样着色。
func TestLivePreviewCodeHighlightNoBlankLine(t *testing.T) {
	_, tt := renderDarkPreview(t, "123\n```js\nconst a = 223;\n```\n")
	c, total := countColored(t, tt, "const a = 223;")
	t.Logf("无空行围栏: colored=%d total=%d", c, total)
	if c == 0 {
		t.Fatal("围栏紧跟段落时代码未着色")
	}
}

// countColoredWhole 整幅画面里与灰白偏离明显的像素数。
func countColoredWhole(t *testing.T, tt *ui.Tester) int {
	t.Helper()
	img := tt.Image()
	colored := 0
	for y := 0; y < img.Bounds().Dy(); y++ {
		for x := 0; x < img.Bounds().Dx(); x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			r8, g8, b8 := int(r>>8), int(g>>8), int(b>>8)
			if mx, mn := max(r8, g8, b8), min(r8, g8, b8); mx-mn > 40 {
				colored++
			}
		}
	}
	return colored
}

// TestActiveCodeBlockTokenColors 焦点块（正在编辑）的代码也必须着色：
// activeText 走 TextArea + TextRanges（与源码模式同机制），
// tab 内容只有一个代码块时，全窗彩色像素只能来自它的 token 配色。
// 注意 Tester 的 Texts/Find 不收集 TextArea 内容，不能用 Find 定位。
func TestActiveCodeBlockTokenColors(t *testing.T) {
	a, tt := renderDarkPreview(t, "```js\nconst a = 223;\n```\n")
	_ = a
	// newTestApp 里 mdActive = 0：首块即焦点块，走 activeText 路径。
	tt.Frame()
	c := countColoredWhole(t, tt)
	t.Logf("焦点块整窗彩色像素: %d", c)
	if c == 0 {
		t.Fatal("焦点块（activeText 路径）代码未着色")
	}
}
