package app

import (
	"zakki/internal/markdown"
)

// 格式变换桥接：main 保留原有 docResult 与函数名，实现委托给 internal/markdown。

type docResult struct {
	doc      string
	selStart int
	selEnd   int
	changed  bool
}

func fromMD(r markdown.DocResult) docResult {
	return docResult{doc: r.Doc, selStart: r.SelStart, selEnd: r.SelEnd, changed: r.Changed}
}

func applyInlineToLines(doc string, from, to, selStart, selEnd int, kind spanKind) docResult {
	return fromMD(markdown.ApplyInlineToLines(doc, from, to, selStart, selEnd, markdown.SpanKind(kind)))
}

func applyHeadingToLines(doc string, from, to, caret int, level int) docResult {
	return fromMD(markdown.ApplyHeadingToLines(doc, from, to, caret, level))
}

func applyPrefixToLines(doc string, from, to, caret int, kind prefixKind) docResult {
	return fromMD(markdown.ApplyPrefixToLines(doc, from, to, caret, markdown.PrefixKind(kind)))
}

func applyCodeBlockToggle(doc string, from, to, caret int) docResult {
	return fromMD(markdown.ApplyCodeBlockToggle(doc, from, to, caret))
}

func applyInsertAfterLine(doc string, afterLine int, block string, caretInBlock int) docResult {
	return fromMD(markdown.ApplyInsertAfterLine(doc, afterLine, block, caretInBlock))
}

func replaceLineWith(doc string, line int, block string, caretInBlock int) docResult {
	return fromMD(markdown.ReplaceLineWith(doc, line, block, caretInBlock))
}

func defaultTableBlock() string { return markdown.DefaultTableBlock() }

func docLineCount(doc string) int { return markdown.DocLineCount(doc) }

func caretLineCol(doc string, caret int) (int, int) { return markdown.CaretLineCol(doc, caret) }

func inlineMarkers(kind spanKind) (string, string) {
	return markdown.InlineMarkers(markdown.SpanKind(kind))
}

func runeIndexOf(rs []rune, target rune, from int) int { return markdown.RuneIndexOf(rs, target, from) }

func runeSeq(rs []rune, target rune, n, from int) int { return markdown.RuneSeq(rs, target, n, from) }

func caretFromLineCol(doc string, line, col int) int {
	return markdown.CaretFromLineCol(doc, line, col)
}

type spanKind = markdown.SpanKind

const (
	spanBold   = markdown.SpanBold
	spanItalic = markdown.SpanItalic
	spanCode   = markdown.SpanCode
	spanLink   = markdown.SpanLink
)

type mSpan struct {
	start, end int
	kind       spanKind
	href       string
}

func mdSpan(sp markdown.Span) mSpan {
	return mSpan{start: sp.Start, end: sp.End, kind: spanKind(sp.Kind), href: sp.Href}
}

func inlineSpans(s string) []mSpan {
	xs := markdown.InlineSpans(s)
	out := make([]mSpan, len(xs))
	for i, sp := range xs {
		out[i] = mdSpan(sp)
	}
	return out
}

func spanText(s string, sp mSpan) string {
	return markdown.SpanText(s, markdown.Span{Start: sp.start, End: sp.end, Kind: markdown.SpanKind(sp.kind), Href: sp.href})
}

func toggleTaskLine(src string, lineNo int) string { return markdown.ToggleTaskLine(src, lineNo) }

func runeToByte(s string, runeIdx int) int { return markdown.RuneToByte(s, runeIdx) }

type tableCell struct {
	text       string
	start, end int
}

type markdownTable struct {
	rows  [][]tableCell
	align []int
}

func fromMDCell(c markdown.TableCell) tableCell {
	return tableCell{text: c.Text, start: c.Start, end: c.End}
}

func fromMDTable(t markdown.Table) markdownTable {
	rows := make([][]tableCell, len(t.Rows))
	for i, row := range t.Rows {
		rows[i] = make([]tableCell, len(row))
		for j, c := range row {
			rows[i][j] = fromMDCell(c)
		}
	}
	return markdownTable{rows: rows, align: t.Align}
}

func toMDCells(cells []tableCell) []markdown.TableCell {
	out := make([]markdown.TableCell, len(cells))
	for i, c := range cells {
		out[i] = markdown.TableCell{Text: c.text, Start: c.start, End: c.end}
	}
	return out
}

func pipeCells(line string) ([]tableCell, bool) {
	cells, ok := markdown.PipeCells(line)
	if !ok {
		return nil, false
	}
	out := make([]tableCell, len(cells))
	for i, c := range cells {
		out[i] = fromMDCell(c)
	}
	return out, true
}

func tableStart(lines []string, i, stop int) (markdownTable, bool) {
	t, ok := markdown.TableStart(lines, i, stop)
	if !ok {
		return markdownTable{}, false
	}
	return fromMDTable(t), true
}

func emptyTableRow(n int) string { return markdown.EmptyTableRow(n) }

func tableCellPositions(raw string) []tableCell {
	cells := markdown.TableCellPositions(raw)
	out := make([]tableCell, len(cells))
	for i, c := range cells {
		out[i] = fromMDCell(c)
	}
	return out
}

type prefixKind = markdown.PrefixKind

const (
	prefixNone    = markdown.PrefixNone
	prefixQuote   = markdown.PrefixQuote
	prefixBullet  = markdown.PrefixBullet
	prefixOrdered = markdown.PrefixOrdered
	prefixTask    = markdown.PrefixTask
)
