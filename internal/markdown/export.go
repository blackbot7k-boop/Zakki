package markdown

// 对外导出：根目录 UI 层通过桥接调用，避免在 main 重复格式变换逻辑。

// SpanKind 行内标记类别。
type SpanKind = spanKind

const (
	SpanBold   = spanBold
	SpanItalic = spanItalic
	SpanCode   = spanCode
	SpanLink   = spanLink
)

// Span 是行内标记区间。
type Span struct {
	Start, End int
	Kind       SpanKind
	Href       string
}

func toSpan(sp mSpan) Span {
	return Span{Start: sp.Start, End: sp.End, Kind: SpanKind(sp.Kind), Href: sp.Href}
}

// InlineSpans 解析行内标记。
func InlineSpans(s string) []Span {
	xs := inlineSpans(s)
	out := make([]Span, len(xs))
	for i, sp := range xs {
		out[i] = toSpan(sp)
	}
	return out
}

// SpanText 去掉标记符后的纯文本。
func SpanText(s string, sp Span) string {
	return spanText(s, mSpan{Start: sp.Start, End: sp.End, Kind: spanKind(sp.Kind), Href: sp.Href})
}

// ToggleTaskLine 翻转待办勾选。
func ToggleTaskLine(src string, lineNo int) string { return toggleTaskLine(src, lineNo) }

// RuneToByte 把 rune 下标换成字节下标。
func RuneToByte(s string, runeIdx int) int { return runeToByte(s, runeIdx) }

// TableCell 表格单元格在源行内的 rune 区间。
type TableCell struct {
	Text       string
	Start, End int
	Missing    bool
}

// Table 是解析后的 Markdown 表格。
type Table struct {
	Rows  [][]TableCell
	Align []int
}

func toTableCell(c tableCell) TableCell {
	return TableCell{Text: c.Text, Start: c.Start, End: c.End, Missing: c.Missing}
}

func PipeCells(line string) ([]TableCell, bool) {
	cells, ok := pipeCells(line)
	if !ok {
		return nil, false
	}
	out := make([]TableCell, len(cells))
	for i, c := range cells {
		out[i] = toTableCell(c)
	}
	return out, true
}

func TableStart(lines []string, i, stop int) (Table, bool) {
	t, ok := tableStart(lines, i, stop)
	if !ok {
		return Table{}, false
	}
	rows := make([][]TableCell, len(t.Rows))
	for i, row := range t.Rows {
		rows[i] = make([]TableCell, len(row))
		for j, c := range row {
			rows[i][j] = toTableCell(c)
		}
	}
	return Table{Rows: rows, Align: t.Align}, true
}

func EmptyTableRow(n int) string { return emptyTableRow(n) }

func TableRowCells(line string, columns int) ([]TableCell, bool) {
	cells, ok := tableRowCells(line, columns)
	if !ok {
		return nil, false
	}
	out := make([]TableCell, len(cells))
	for i, c := range cells {
		out[i] = toTableCell(c)
	}
	return out, true
}

func NormalizeTableRow(line string, columns int) (string, bool) {
	return normalizeTableRow(line, columns)
}

func TableCellPositions(raw string) []TableCell {
	cells := tableCellPositions(raw)
	out := make([]TableCell, len(cells))
	for i, c := range cells {
		out[i] = toTableCell(c)
	}
	return out
}

// FenceMarker 标识代码围栏使用的字符与长度。
type FenceMarker struct {
	Character byte
	Size      int
}

// ParseFenceOpeningLine 解析至少三个反引号或波浪号组成的开栏。
func ParseFenceOpeningLine(line string) (FenceMarker, bool) {
	marker, ok := parseFenceOpeningLine(line)
	return FenceMarker{Character: marker.char, Size: marker.size}, ok
}

// IsFenceOpeningLine 报告该行是否是有效开栏。
func IsFenceOpeningLine(line string) bool { return isFenceOpeningLine(line) }

// IsFenceClosingLine 按开栏字符与最短长度判断闭栏。
func IsFenceClosingLine(line string, opening FenceMarker) bool {
	return isFenceClosingLine(line, fenceMarker{char: opening.Character, size: opening.Size})
}

// FenceClosingMarker 返回与开栏匹配的闭栏文本。
func FenceClosingMarker(opening FenceMarker) string {
	return fenceClosingMarker(fenceMarker{char: opening.Character, size: opening.Size})
}

func DocLineCount(doc string) int { return docLineCount(doc) }

func CaretLineCol(doc string, caret int) (int, int) { return caretLineCol(doc, caret) }

func InlineMarkers(kind SpanKind) (open, close string) { return inlineMarkers(spanKind(kind)) }

func RuneIndexOf(rs []rune, target rune, from int) int { return runeIndexOf(rs, target, from) }

func RuneSeq(rs []rune, target rune, n, from int) int { return runeSeq(rs, target, n, from) }

func CaretFromLineCol(doc string, line, col int) int { return caretFromLineCol(doc, line, col) }
