package ui

import "github.com/egoist/mygo/internal/text"

// TextObjects 返回最近实际布局中的占位符，不触发布局。应在布局后的 Draw 回调中，
// 或输入回调通过 Context.Resolve 的当前元素查询；构建期可能尚无布局。
// TextArea 仅返回已布局的可见段落，矩形未裁切；支持 Text、RichText 和输入控件。
func (e *node) TextObjects() []TextObject {
	var out []TextObject
	add := func(l *text.Layout, x, y float32, start int) {
		e.eachTextObject(l, x, y, start, func(o TextObject) { out = append(out, o) })
	}
	if ed := e.st.editor; e.kind == kindInput && ed != nil {
		if a := ed.area; a != nil {
			if a.version == 0 {
				return nil
			}
			for i := a.first; i <= min(a.last, len(ed.buf.paras)-1); i++ {
				add(ed.buf.paras[i].layout, ed.originX, ed.originY+float32(a.hs.top(i)-a.scroll), ed.buf.start(i))
			}
		} else {
			add(ed.layout, ed.originX-ed.scrollX, ed.originY, 0)
		}
	} else if e.kind == kindText {
		if e.tl != nil {
			add(e.tl, e.contentX(), e.contentY(), 0)
		} else if ed := e.st.editor; ed != nil {
			add(ed.layout, ed.originX, ed.originY, 0)
		}
	}
	return out
}

// DrawTextObjects 在真实文字绘制过程中绘制占位符，背景之后、选区和文字之前。
// 回调沿用文字的裁剪、变换和透明度；Rect 是可直接传给 Painter 的完整对象矩形。
// 回调只绘制，不修改状态；可能每帧运行多次。点击应由元素输入回调结合 TextObjects
// 命中测试处理。支持 Text、RichText 和输入控件，不创建子元素或抢占编辑焦点。
func (e *node) DrawTextObjects(fn func(p *Painter, object TextObject)) *node {
	e.textObjectsFn = fn
	return e
}

func (e *node) eachTextObject(l *text.Layout, x, y float32, start int, fn func(TextObject)) {
	if l == nil {
		return
	}
	var spans []text.Span
	for _, line := range l.Lines {
		for _, g := range line.Glyphs {
			if !g.Object {
				continue
			}
			if spans == nil {
				spans = text.DecodeSpans(l.Params.Spans)
			}
			for _, sp := range spans {
				if g.Cluster >= sp.End {
					continue
				}
				index := start + g.Cluster
				if ed := e.st.editor; e.kind == kindInput && ed != nil {
					if ed.area == nil || start == ed.buf.start(ed.buf.para(ed.caret)) {
						index = ed.textIndex(index)
					}
				}
				fn(TextObject{Index: index, Rect: Rect{x + g.X, y + line.Baseline - sp.ObjectHeight, g.Advance, sp.ObjectHeight}})
				break
			}
		}
	}
}

func (e *node) paintTextObjects(p *Painter, l *text.Layout, x, y float32, start int) {
	if e.textObjectsFn == nil {
		return
	}
	e.eachTextObject(l, x, y, start, func(o TextObject) {
		if r := intersect(p.clip, transformRect(p.transform, o.Rect)); r.W > 0 && r.H > 0 {
			e.textObjectsFn(p, o)
		}
	})
}
