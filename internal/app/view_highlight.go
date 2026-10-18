package app

import (
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/egoist/mygo/ui"
	"zakki/internal/codehl"
)

// 每个 rune 占一位；判交只访问候选覆盖的字，不扫描已接受区间。
// 必须先完整判交再标记，重叠候选整段拒绝，不占用其余空闲位置。
type editorRangeOccupancy struct {
	total int
	words []uint64
}

func (o editorRangeOccupancy) add(start, end int) bool {
	if start < 0 || start >= end || end > o.total {
		return false
	}
	first, last := start/64, (end-1)/64
	firstMask := ^uint64(0) << uint(start%64)
	lastMask := ^uint64(0) >> uint(63-(end-1)%64)
	if first == last {
		mask := firstMask & lastMask
		if o.words[first]&mask != 0 {
			return false
		}
		o.words[first] |= mask
		return true
	}
	if o.words[first]&firstMask != 0 || o.words[last]&lastMask != 0 {
		return false
	}
	for i := first + 1; i < last; i++ {
		if o.words[i] != 0 {
			return false
		}
	}
	o.words[first] |= firstMask
	for i := first + 1; i < last; i++ {
		o.words[i] = ^uint64(0)
	}
	o.words[last] |= lastMask
	return true
}

// editorRanges 为编辑器生成互不重叠的着色区间：搜索命中优先，
// 其次是行级结构（标题/引用/代码/列表/待办），再次是行内标记。
// 可选 cp 非零时，围栏代码行按 token 上色，其余代码部分沿用 muted。
func editorRanges(text, query string, t *ui.Theme, accent ui.Color, cp ...codePalette) []ui.TextRange {
	codeOn := len(cp) > 0 && cp[0] != (codePalette{})
	var palette codePalette
	if codeOn {
		palette = cp[0]
	}
	var accepted []ui.TextRange
	total := utf8.RuneCountInString(text)
	occupied := editorRangeOccupancy{total: total, words: make([]uint64, (total+63)/64)}

	add := func(tr ui.TextRange) bool {
		if !occupied.add(tr.Start, tr.End) {
			return false
		}
		accepted = append(accepted, tr)
		return true
	}

	for _, m := range findMatches(text, query) {
		add(ui.TextRange{Start: m.Start, End: m.End, Color: accent, Weight: 700})
	}

	runeOff := 0
	inCode := false
	codeLang := ""
	for _, line := range strings.Split(text, "\n") {
		lineRunes := utf8.RuneCountInString(line)
		trimmed := strings.TrimSpace(line)
		styledLine := false
		switch {
		case strings.HasPrefix(trimmed, "```"):
			inCode = !inCode
			codeLang = ""
			if inCode {
				codeLang = codehl.Lang(fenceInfo(trimmed))
			}
			add(ui.TextRange{Start: runeOff, End: runeOff + lineRunes, Color: t.TextMuted})
			styledLine = true
		case inCode:
			if codeOn {
				for _, tr := range codeSourceRanges(line, codehl.Line(codeLang, line), palette, t.TextMuted) {
					tr.Start += runeOff
					tr.End += runeOff
					add(tr)
				}
			} else {
				add(ui.TextRange{Start: runeOff, End: runeOff + lineRunes, Color: t.TextMuted})
			}
			styledLine = true
		case isHeading(trimmed):
			add(ui.TextRange{Start: runeOff, End: runeOff + lineRunes, Color: accent, Weight: 700})
			styledLine = true
		case strings.HasPrefix(trimmed, ">"):
			add(ui.TextRange{Start: runeOff, End: runeOff + lineRunes, Color: t.TextMuted})
			styledLine = true
		}

		if !styledLine {
			if bi := strings.Index(line, "[ ]"); bi >= 0 {
				mark := runeOff + utf8.RuneCountInString(line[:bi])
				add(ui.TextRange{Start: mark, End: mark + 3, Color: accent, Weight: 600})
			} else if bi := strings.Index(line, "[x]"); bi >= 0 {
				mark := runeOff + utf8.RuneCountInString(line[:bi])
				add(ui.TextRange{Start: mark, End: mark + 3, Color: accent, Weight: 600})
			}
			if n := listMarkerLen(trimmed); n > 0 {
				lead := len(line) - len(strings.TrimLeft(line, " \t"))
				off := utf8.RuneCountInString(line[:lead])
				add(ui.TextRange{Start: runeOff + off, End: runeOff + off + n, Color: t.TextMuted})
			}
			for _, sp := range inlineSpans(line) {
				tr := ui.TextRange{Start: runeOff + sp.start, End: runeOff + sp.end}
				switch sp.kind {
				case spanBold:
					tr.Weight = 600
				case spanCode:
					tr.Color = t.TextMuted
				case spanLink:
					tr.Color = accent
				}
				if tr.Color != (ui.Color{}) || tr.Weight != 0 {
					add(tr)
				}
			}
		}

		runeOff += lineRunes + 1
	}

	sort.Slice(accepted, func(i, j int) bool { return accepted[i].Start < accepted[j].Start })
	return accepted
}
