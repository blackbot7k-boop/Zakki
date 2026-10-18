package app

// 手账排版的手绘装饰：抖动铅笔线、马克笔引用条、荧光笔标题底、点阵纸。
// 抖动由种子确定，同一块在每帧画出相同的线条，避免闪烁；绘制回调只负责画。

import (
	"fmt"
	"math"

	"github.com/egoist/mygo/ui"
)

// jitter 返回种子与序号确定的 [-1, 1] 伪随机数。
func jitter(seed uint32, i uint32) float32 {
	h := seed*2654435761 ^ (i+1)*2246822519
	h ^= h >> 15
	h *= 3266489917
	h ^= h >> 13
	return float32(h%2001)/1000 - 1
}

func withAlpha(c ui.Color, a float32) ui.Color {
	return ui.RGBA(c.R, c.G, c.B, a)
}

// sketchLine 画一条带轻微弯曲与出头的铅笔线：两遍不同抖动叠加，模拟手描。
func sketchLine(p *ui.Painter, x0, y0, x1, y1, width float32, col ui.Color, seed uint32) {
	dx, dy := x1-x0, y1-y0
	length := float32(math.Hypot(float64(dx), float64(dy)))
	if length < 1 {
		return
	}
	ux, uy := dx/length, dy/length
	nx, ny := -uy, ux
	amp := min(1.6, 0.6+length*0.004)
	for pass := uint32(0); pass < 2; pass++ {
		s := seed*7 + pass*101
		over0 := 1 + 2*jitter(s, 1)
		over1 := 1 + 2*jitter(s, 2)
		ax, ay := x0-ux*over0+nx*amp*jitter(s, 3), y0-uy*over0+ny*amp*jitter(s, 3)
		bx, by := x1+ux*over1+nx*amp*jitter(s, 4), y1+uy*over1+ny*amp*jitter(s, 4)
		c1, c2 := amp*1.4*jitter(s, 5), amp*1.4*jitter(s, 6)
		var path ui.Path
		path.MoveTo(ax, ay).CubeTo(
			x0+dx/3+nx*c1, y0+dy/3+ny*c1,
			x0+dx*2/3+nx*c2, y0+dy*2/3+ny*c2,
			bx, by)
		alpha := float32(0.9)
		if pass == 1 {
			alpha = 0.45
		}
		p.StrokePath(&path, width, withAlpha(col, alpha*float32(col.A)/255))
	}
}

// sketchRect 用四条铅笔线围出一个略歪的框。
func sketchRect(p *ui.Painter, r ui.Rect, width float32, col ui.Color, seed uint32) {
	in := width
	x0, y0, x1, y1 := r.X+in, r.Y+in, r.X+r.W-in, r.Y+r.H-in
	sketchLine(p, x0, y0, x1, y0, width, col, seed+1)
	sketchLine(p, x1, y0, x1, y1, width, col, seed+2)
	sketchLine(p, x1, y1, x0, y1, width, col, seed+3)
	sketchLine(p, x0, y1, x0, y0, width, col, seed+4)
}

// sketchWave 横向手绘波浪线（分隔线）。
func sketchWave(p *ui.Painter, x0, x1, y, width float32, col ui.Color, seed uint32) {
	var path ui.Path
	step := float32(14)
	path.MoveTo(x0, y+jitter(seed, 0))
	i := uint32(1)
	for x := x0 + step; x <= x1; x += step {
		cy := y + 2.2*float32(1-2*int(i%2)) + 0.8*jitter(seed, i)
		path.QuadTo(x-step/2, cy, x, y+0.6*jitter(seed, i+50))
		i++
	}
	p.StrokePath(&path, width, col)
}

// highlighter 荧光笔涂抹：一块两端略歪、上下边微弯的半透明色带。
func highlighter(p *ui.Painter, x, y, w, h float32, col ui.Color, seed uint32) {
	if w <= 0 || h <= 0 {
		return
	}
	var path ui.Path
	t := h * 0.18
	path.MoveTo(x+2*jitter(seed, 1), y+t*jitter(seed, 2)).
		QuadTo(x+w/2, y-t+t*jitter(seed, 3), x+w+3*jitter(seed, 4), y+t*jitter(seed, 5)).
		LineTo(x+w+3*jitter(seed, 6), y+h+t*jitter(seed, 7)).
		QuadTo(x+w/2, y+h+t+t*jitter(seed, 8), x+2*jitter(seed, 9), y+h+t*jitter(seed, 10)).
		Close()
	p.FillPath(&path, col)
}

// dotGrid 子弹笔记式点阵纸，铺满 r；scrollY 是内容滚动量，点阵随内容移动。
// 只画可见区域，长文档也不会每帧铺满全部点。
func dotGrid(p *ui.Painter, r ui.Rect, scrollY float32, col ui.Color) {
	const gap = 22
	vis := p.Visible()
	top := max(r.Y, vis.Y)
	bottom := min(r.Y+r.H, vis.Y+vis.H)
	// 内容坐标中第一行点落在 gap/2 处；换算到视口后再从 top 起对齐。
	origin := r.Y + gap/2 - scrollY
	start := origin + float32(math.Ceil(float64((top-origin)/gap)))*gap
	var path ui.Path
	for y := start; y <= bottom; y += gap {
		for x := r.X + gap/2; x < r.X+r.W; x += gap {
			path.Circle(x, y, 0.9)
		}
	}
	p.FillPath(&path, col)
}

// quoteBar 引用竖条：手账排版画马克笔笔触，其余排版保持圆角色条。
func quoteBar(c *ui.Context, md resolvedStyle, seed uint32) {
	if !md.sketch {
		ui.Box(c).Width(md.quoteBar).FillHeight().Radius(md.quoteBar / 2).Background(md.quoteBorder)
		return
	}
	ui.Box(c).Width(md.quoteBar + 4).FillHeight().Shrink(0).Draw(func(p *ui.Painter, r ui.Rect) {
		x := r.X + r.W/2
		sketchLine(p, x, r.Y+2, x+jitter(seed, 9), r.Y+r.H-2, md.quoteBar, withAlpha(md.quoteBorder, 0.85), seed)
	})
}

// codeCard 代码块容器：手账排版用铅笔框，其余排版用圆角描边。
func codeCard(c *ui.Context, md resolvedStyle, t *ui.Theme, seed uint32) ui.Element {
	card := ui.Column(c).Padding(md.codePadX, md.codePadY).Radius(md.codeRadius).Background(t.Surface)
	if md.sketch {
		card.Draw(func(p *ui.Painter, r ui.Rect) {
			sketchRect(p, r, md.codeBorder, md.textMuted, seed)
		})
	} else {
		card.BorderWidth(md.codeBorder).BorderColor(md.rule)
	}
	return card
}

// ruleLine 分隔线：手账排版画铅笔波浪线。
func ruleLine(c *ui.Context, md resolvedStyle, seed uint32) {
	if !md.sketch {
		ui.Box(c).Height(1).FillWidth().Background(md.rule)
		return
	}
	ui.Box(c).Height(8).FillWidth().Draw(func(p *ui.Painter, r ui.Rect) {
		sketchWave(p, r.X+4, r.X+r.W-4, r.Y+r.H/2, 1.4, md.textMuted, seed)
	})
}

// paperTheme 当前颜色主题是否为不透明纸张（手绘）：此时不透出窗口毛玻璃。
func (a *app) paperTheme() bool { return a.prefs.Appearance.ColorTheme == "sketch" }

// paperBackground 内容区背景：纸张主题保持不透明底色，其余沿用毛玻璃规则。
func (a *app) paperBackground(base ui.Color, dark, vibrancy bool) ui.Color {
	if a.paperTheme() {
		return base
	}
	return contentBackground(base, dark, vibrancy)
}

// sketchHeading 一、二级标题在文字下半部垫荧光笔，三级及以下画一道铅笔下划线；
// 宽度跟随文字。
func (a *app) sketchHeading(c *ui.Context, t *ui.Theme, md resolvedStyle, blk liveBlock) {
	projection := a.previewProjection(blk)
	sketchHeadingFrame(c, md, blk, projection.text).Children(func() {
		h := projection.content(c, t, md).TextColor(t.Text)
		styleHeading(h, blk.level, md)
	})
}

func sketchHeadingFrame(c *ui.Context, md resolvedStyle, blk liveBlock, text string) ui.Element {
	size := md.headingSizes[clamp(blk.level, 1, 6)-1]
	mark := withAlpha(md.quoteBorder, 0.42)
	under := withAlpha(md.accent, 0.75)
	seed := uint32(blk.start)
	return ui.Box(c).FillWidth().Draw(func(p *ui.Painter, r ui.Rect) {
		w, _ := p.MeasureText(r.W, ui.Span{Text: text, Font: md.headingFont, Size: size, Weight: 700})
		w = min(w+8, r.W)
		if blk.level <= 2 {
			highlighter(p, r.X-3, r.Y+r.H*0.5, w, r.H*0.36, mark, seed)
			return
		}
		sketchLine(p, r.X, r.Y+r.H-1.5, r.X+w-6, r.Y+r.H-0.5, 1.5, under, seed)
	})
}

// sketchMark 粗体的荧光笔底色。
func sketchMark(md resolvedStyle) ui.Color { return withAlpha(md.quoteBorder, 0.38) }

// listMarker 列表符号：手账排版无序列表画一枚手绘小圈，有序列表保留数字；
// 其余排版沿用圆点。
func listMarker(c *ui.Context, md resolvedStyle, blk liveBlock) {
	if blk.n > 0 {
		ui.Text(c, fmt.Sprintf("%d.", blk.n)).TextColor(md.accent)
		return
	}
	if !md.sketch {
		ui.Text(c, "•").TextColor(md.accent)
		return
	}
	size := md.bodySize * md.lineHeight
	seed := uint32(blk.start)
	ui.Box(c).Width(12).Height(size).Shrink(0).Draw(func(p *ui.Painter, r ui.Rect) {
		cx, cy := r.X+r.W/2, r.Y+r.H/2
		var path ui.Path
		rad := float32(3.6)
		path.MoveTo(cx+rad, cy+0.6*jitter(seed, 1))
		path.CubeTo(cx+rad, cy-rad*1.3, cx-rad*1.3, cy-rad, cx-rad, cy+0.4*jitter(seed, 2))
		path.CubeTo(cx-rad, cy+rad*1.3, cx+rad*1.2, cy+rad*1.1, cx+rad+0.6, cy-1+jitter(seed, 3))
		p.StrokePath(&path, 1.6, md.accent)
	})
}

// sketchCheckbox 手绘勾选框：铅笔方框，完成时画一笔朱红对勾；保持复选框语义。
func sketchCheckbox(c *ui.Context, key *ui.Context, label string, checked bool, md resolvedStyle, seed uint32, toggle func()) {
	size := md.bodySize * md.lineHeight
	box := ui.ButtonBase(key).Label(label).Role(ui.RoleCheckBox).Checked(checked).
		Width(26).Height(size).Shrink(0).Cursor(ui.CursorPointer).FocusRing(false)
	box.Draw(func(p *ui.Painter, r ui.Rect) {
		s := float32(14)
		x, y := r.X+(r.W-s)/2, r.Y+(r.H-s)/2
		sketchRect(p, ui.Rect{X: x, Y: y, W: s, H: s}, 1.4, md.text, seed)
		if !checked {
			return
		}
		var path ui.Path
		path.MoveTo(x+2, y+s*0.5).
			QuadTo(x+s*0.3, y+s*0.7, x+s*0.42, y+s+1).
			QuadTo(x+s*0.65, y+s*0.25, x+s+4, y-3)
		p.StrokePath(&path, 2.4, md.accent)
	})
	if box.Clicked() {
		toggle()
	}
}
