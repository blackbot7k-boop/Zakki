package app

import (
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/egoist/mygo/ui"
	"zakki/internal/codehl"
)

// 保留优化前的逐项扫描实现，独立校验完整输出与逐次加入的优先级。
// 不与生产位图实现共享判交逻辑。
func editorRangesScanOracle(text, query string, t *ui.Theme, accent ui.Color, cp ...codePalette) []ui.TextRange {
	codeOn := len(cp) > 0 && cp[0] != (codePalette{})
	var palette codePalette
	if codeOn {
		palette = cp[0]
	}
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
		add(searchHit(m.Start, m.End, accent))
	}

	runeOff := 0
	inCode := false
	var opening fenceMarker
	codeLang := ""
	for _, line := range strings.Split(text, "\n") {
		lineRunes := utf8.RuneCountInString(line)
		trimmed := strings.TrimSpace(line)
		fence, opens := parseFenceOpeningLine(line)
		styledLine := false
		switch {
		case inCode && isFenceClosingLine(line, opening):
			inCode = false
			codeLang = ""
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
		case opens:
			inCode = true
			opening = fence
			codeLang = codehl.Lang(fenceInfo(line))
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
