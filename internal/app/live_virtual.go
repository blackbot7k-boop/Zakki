package app

import (
	"github.com/egoist/mygo/ui"
)

// 块内虚拟化：超长代码块与表格只构建视口附近的行，其余行用占位撑开。
// 位置取上一帧布局（Element.Bounds）并按其后的滚动量校正；行高按构建过的
// 行实测缓存，未测过的行用实测均值估计。窗口依据过期时再要一帧校正。

const (
	// virtMinRows 行数达到它才虚拟化：小块全量构建更简单，也没有估高误差。
	virtMinRows = 200
	// virtFirstRows 没有上一帧布局时先建的行数（下一帧按实际位置校正）。
	virtFirstRows = 60
)

// virtKey 区分块内虚拟化的缓存条目：同一块内容在同一种排版下行高一致。
type virtKey struct {
	raw   string
	kind  blockKind
	style virtStyleKey
}

// virtStyleKey 只记录实际参与行布局的值；纯颜色变化不影响行高。
type virtStyleKey struct {
	bodyFont, monoFont string
	size, lineHeight   float32
	padX, padY         float32
	sketch             bool
}

func virtualRowsKey(raw string, kind blockKind, md resolvedStyle) virtKey {
	style := virtStyleKey{size: md.codeSize}
	switch kind {
	case bkCode:
		style.monoFont = md.monoFont
		style.lineHeight = md.codeLineHeigh
	case bkTable:
		style.bodyFont = md.bodyFont
		style.monoFont = md.monoFont
		style.lineHeight = md.lineHeight
		style.padX, style.padY = md.tablePadX, md.tablePadY
		style.sketch = md.sketch
	}
	return virtKey{raw: raw, kind: kind, style: style}
}

type virtRows struct {
	width   float32
	heights []float32 // 0 = 未实测
	sum     float32   // 已实测行高之和
	count   int
	used    uint64
}

// liveVirtState 块内虚拟化的帧间状态（主线程）。
type liveVirtState struct {
	frame    uint64
	viewport ui.Rect // 上一帧实时预览视口（窗口坐标）
	scrollY  float32 // 上一帧构建时的滚动量，用来校正上一帧布局的位置
	scrolled float32 // 本帧相对上一帧的滚动量
	rows     map[virtKey]*virtRows
}

// beginFrame 每帧构建实时预览前调用：记下自上一帧以来的滚动量。
func (v *liveVirtState) beginFrame(scrollY float32) {
	v.frame++
	v.scrolled = scrollY - v.scrollY
	v.scrollY = scrollY
}

// endFrame 记下本帧视口，丢弃本帧未用到的条目（块不可见或内容已变）。
func (v *liveVirtState) endFrame(viewport ui.Rect) {
	v.viewport = viewport
	for k, e := range v.rows {
		if e.used != v.frame {
			delete(v.rows, k)
		}
	}
}

func (v *liveVirtState) entry(key virtKey, n int) *virtRows {
	if v.rows == nil {
		v.rows = map[virtKey]*virtRows{}
	}
	e := v.rows[key]
	if e == nil || len(e.heights) != n {
		e = &virtRows{heights: make([]float32, n)}
		v.rows[key] = e
	}
	e.used = v.frame
	return e
}

func (e *virtRows) reset(width float32) {
	e.width, e.sum, e.count = width, 0, 0
	clear(e.heights)
}

func (e *virtRows) record(i int, h float32) {
	if h <= 0 {
		return
	}
	if old := e.heights[i]; old > 0 {
		e.sum -= old
		e.count--
	}
	e.heights[i] = h
	e.sum += h
	e.count++
}

// height 第 i 行的行高：实测值优先，否则用实测均值，没有实测时用 est。
func (e *virtRows) height(i int, est float32) float32 {
	if h := e.heights[i]; h > 0 {
		return h
	}
	if e.count > 0 {
		return e.sum / float32(e.count)
	}
	return est
}

// virtWindow 返回与内容坐标区间 [top, bottom) 相交的行 [first, last)。
func virtWindow(n int, height func(int) float32, top, bottom float32) (first, last int) {
	y := float32(0)
	first, last = n, n
	for i := range n {
		h := height(i)
		if first == n && y+h > top {
			first = i
		}
		if y >= bottom {
			last = i
			break
		}
		y += h
	}
	if first > last {
		first = last
	}
	return first, last
}

// virtualRows 在一个纵向容器里构建 n 行，row(i) 构建第 i 行并返回该行元素
// （行元素需带稳定 Key，才能读到上一帧的实测高度）。行数不足 virtMinRows
// 时全量构建。pin >= 0 的行总会构建（表格里正在编辑的单元格）。
func (a *app) virtualRows(c *ui.Context, key virtKey, n int, est float32, pin int, row func(i int) ui.Element) ui.Element {
	box := ui.Column(c.Key("virt")).FillWidth()
	if n < virtMinRows {
		box.Children(func() {
			for i := range n {
				row(i)
			}
		})
		return box
	}
	v := &a.virt
	e := v.entry(key, n)
	b := box.Bounds()
	if b.W != e.width {
		e.reset(b.W)
	}
	height := func(i int) float32 { return e.height(i, est) }
	first, last := 0, min(n, virtFirstRows)
	if vp := v.viewport; b.H > 0 && vp.H > 0 {
		top := vp.Y - (b.Y - v.scrolled)
		first, last = virtWindow(n, height, top-vp.H, top+2*vp.H)
	}
	if b.H == 0 || v.scrolled != 0 {
		// 上一帧布局缺失或已被滚动挪动：本帧按估计构建，再要一帧按新布局校正。
		c.Invalidate()
	}
	box.Children(func() {
		spacer := func(from, to int) {
			var h float32
			for i := from; i < to; i++ {
				h += height(i)
			}
			if h > 0 {
				ui.Box(c).FillWidth().Height(h)
			}
		}
		build := func(i int) {
			if bb := row(i).Bounds(); bb.W > 0 && bb.W == b.W {
				e.record(i, bb.H)
			}
		}
		at := 0
		if pin >= 0 && pin < first {
			spacer(0, pin)
			build(pin)
			at = pin + 1
		}
		spacer(at, first)
		for i := first; i < last; i++ {
			build(i)
		}
		at = last
		if pin >= last && pin < n {
			spacer(last, pin)
			build(pin)
			at = pin + 1
		}
		spacer(at, n)
	})
	return box
}
