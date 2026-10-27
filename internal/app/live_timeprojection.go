package app

import (
	"unicode/utf8"

	"zakki/internal/markdown"
)

const timeObjectRune = '\ufffc'

func (p liveProjection) visibleSelection(start, end int) (int, int) {
	s, e := p.visibleOffset(start), p.visibleOffset(end)
	if start < end {
		for _, at := range p.objects {
			u := p.units[at]
			if start < u.end && end > u.start {
				s, e = min(s, at), max(e, at+1)
			}
		}
	}
	return s, e
}

// withTimeObject 用单个对象字符映射完整时间源码，让原生选区与方向键
// 只停在组件两侧；文档仍保存原始 Markdown，不保存占位符。
func (p liveProjection) withTimeObject() liveProjection {
	token, ok := markdown.FindTimeToken(p.raw)
	if !ok {
		return p
	}
	start := utf8.RuneCountInString(p.raw[:token.Start])
	end := start + utf8.RuneCountInString(p.raw[token.Start:token.End])
	if start < p.start || end > p.end {
		return p
	}
	lo, hi := p.visibleOffset(start), p.visibleOffset(end)
	if hi <= lo {
		return p
	}
	rs := []rune(p.text)
	p.text = string(rs[:lo]) + string(timeObjectRune) + string(rs[hi:])
	units := make([]projectedRune, 0, len(p.units)-(hi-lo)+1)
	units = append(units, p.units[:lo]...)
	units = append(units, projectedRune{start, end})
	p.units = append(units, p.units[hi:]...)
	p.objects = []int{lo}
	mapOffset := func(at int) int {
		if at <= lo {
			return at
		}
		if at < hi {
			return lo + 1
		}
		return at - (hi - lo) + 1
	}
	for i := range p.styles {
		p.styles[i].start = mapOffset(p.styles[i].start)
		p.styles[i].end = mapOffset(p.styles[i].end)
	}
	return p
}
