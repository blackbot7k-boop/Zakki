package app

import (
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/egoist/mygo/ui"
)

// editorRanges 为编辑器生成互不重叠的着色区间：搜索命中优先，
// 其次是行级结构（标题/引用/代码/列表/待办），再次是行内标记。
func editorRanges(text, query string, t *ui.Theme, accent ui.Color) []ui.TextRange {
	var accepted []ui.TextRange
	total := utf8.RuneCountInString(text)

	add := func(tr ui.TextRange) bool {
		if tr.Start >= tr.End || tr.Start < 0 || tr.End > total {
			return false
		}
		for _, a := range accepted {
			if tr.Start < a.End && a.Start < tr.End {
				return false
			}
		}
		accepted = append(accepted, tr)
		return true
	}

	for _, m := range findMatches(text, query) {
		add(ui.TextRange{Start: m.Start, End: m.End, Color: accent, Weight: 700})
	}

	runeOff := 0
	inCode := false
	for _, line := range strings.Split(text, "\n") {
		lineRunes := utf8.RuneCountInString(line)
		trimmed := strings.TrimSpace(line)
		styledLine := false
		switch {
		case strings.HasPrefix(trimmed, "```"):
			inCode = !inCode
			add(ui.TextRange{Start: runeOff, End: runeOff + lineRunes, Color: t.TextMuted})
			styledLine = true
		case inCode:
			add(ui.TextRange{Start: runeOff, End: runeOff + lineRunes, Color: t.TextMuted})
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
