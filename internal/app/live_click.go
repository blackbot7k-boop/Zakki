package app

import (
	"unicode/utf16"

	"github.com/egoist/mygo/ui"
)

// liveHit 是预览里可被点击定位的一段文字。点击落在元素上时用原生排版的
// 光标；落在行间距或页边时，用同一元素的上一帧盒子把点击换算进这段文字。
// 元素只活一帧，这里留 Handle，点击回调里再 Resolve。
type liveHit struct {
	block int
	ref   ui.Handle
	proj  liveProjection
	font  ui.Font
}

func bodyLiveFont(md resolvedStyle) ui.Font {
	return ui.Font{Family: md.bodyFont, Size: md.bodySize}
}

func headingLiveFont(md resolvedStyle, level int) ui.Font {
	level = clamp(level, 1, 6)
	return ui.Font{Family: md.headingFont, Size: md.headingSizes[level-1], Weight: 700}
}

func codeLiveFont(md resolvedStyle) ui.Font {
	return ui.Font{Family: md.monoFont, Size: md.codeSize}
}

// bindPreviewClick 让预览文字自己接收点击，并把原生光标映射回源码偏移。
// 文字铺满行宽，点在第 n 行右侧空白时，光标停在这一行末尾，而不是整块末尾。
func recordLiveHit(el ui.Element, block int, proj liveProjection, font ui.Font, hits *[]liveHit) {
	if hits == nil || !el.Valid() {
		return
	}
	var ref ui.Handle
	el.Bind(&ref)
	*hits = append(*hits, liveHit{block: block, ref: ref, proj: proj, font: font})
}

func (a *app) bindPreviewClick(el ui.Element, block int, proj liveProjection, font ui.Font, hits *[]liveHit) {
	recordLiveHit(el, block, proj, font, hits)
	el.Selectable().OnClick(func() {
		a.liveClickHandled = true
		s, e := el.TextSelection()
		a.focusBlock(block, proj.sourceOffset(s), proj.sourceOffset(e))
	})
}

// focusNearestHit 把指针落进垂直方向最近的一段文字。
// belowIsDocumentEnd 为真时，点在最后一段下方、距离超过 slack 的空白仍去文末，
// 短笔记可以点纸面末尾继续写。行与行之间的空隙归到较近的那一行。
func (a *app) focusNearestHit(c *ui.Context, hits []liveHit, slack float32, belowIsDocumentEnd bool) {
	best, last := -1, -1
	bestDist := float32(1e9)
	bestBelow := false
	for i, h := range hits {
		el := c.Resolve(h.ref)
		if !el.Valid() {
			continue
		}
		b := el.Bounds()
		if b.H <= 0 || b.W <= 0 {
			continue
		}
		_, y, _ := el.PointerPosition()
		dist, below := float32(0), false
		switch {
		case y < 0:
			dist = -y
		case y >= b.H:
			dist, below = y-b.H, true
		}
		last = i
		if dist < bestDist {
			best, bestDist, bestBelow = i, dist, below
		}
	}
	if best < 0 {
		a.focusBlockEnd(len(a.mdBlocks()) - 1)
		return
	}
	if belowIsDocumentEnd && best == last && bestBelow && bestDist > slack {
		a.focusBlockEnd(len(a.mdBlocks()) - 1)
		return
	}
	a.focusHit(c, hits[best])
}

func (a *app) focusHit(c *ui.Context, h liveHit) {
	caret := 0
	if el := c.Resolve(h.ref); h.proj.text != "" && el.Valid() {
		x, y, _ := el.PointerPosition()
		b := el.Bounds()
		caret = visibleCaretAt(h.proj.text, h.font, b.W, b.H, x, y)
	}
	src := h.proj.sourceOffset(caret)
	a.focusBlock(h.block, src, src)
}

// visibleCaretAt 用与预览相同的字体和换行宽度做一次排版，把点击换算成可见文字偏移。
// ShapeText 用字体的自然行高，预览行高是字号的倍数；两者行数一致时，
// 用元素高度和排版高度的比把点击的纵坐标映过去。
func visibleCaretAt(text string, font ui.Font, width, height, x, y float32) int {
	if text == "" || width <= 1 {
		return 0
	}
	if font.Size <= 0 {
		font.Size = 16
	}
	layout := ui.ShapeText(text, font, width)
	if _, shapedH := layout.Size(); shapedH > 1 && height > 1 {
		y *= shapedH / height
	}
	if y < 0 {
		y = 0
	}
	return utf16ToRuneIndex(text, layout.IndexAt(ui.Point{X: x, Y: y}))
}

func utf16ToRuneIndex(s string, u16 int) int {
	if u16 <= 0 {
		return 0
	}
	n, u := 0, 0
	for _, r := range s {
		if u >= u16 {
			return n
		}
		u += utf16.RuneLen(r)
		n++
	}
	return n
}
