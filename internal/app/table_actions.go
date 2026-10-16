package app

import (
	"strings"
	"unicode/utf8"
)

// tableTools is transient window state, never part of the saved Markdown.
// A destructive resize is tied to the exact document it was proposed for.
type tableTools struct {
	open, confirm        bool
	dot, line            int
	hoverRows, hoverCols int
	rows, cols           int
	pendingDoc           string
}

func (a *app) activeTable() (liveBlock, bool) {
	blocks := a.mdBlocks()
	if a.cfg.Source || a.mdComposing || a.mdActive < 0 || a.mdActive >= len(blocks) {
		return liveBlock{}, false
	}
	b := blocks[a.mdActive]
	return b, b.kind == bkTable
}

// selectedTableCell 用表头列数解释语义单元格，短行和宽行都与渲染模型一致。
func (a *app) selectedTableCell(b liveBlock) (row, col int) {
	cols := len(b.table.align)
	cells := tableCellPositions(b.raw)
	if cols == 0 || len(cells) == 0 {
		return 0, 0
	}
	pos := a.mdCaret
	if a.selSyncStart >= 0 {
		pos = a.selSyncStart
	}
	idx := clamp(tableCellAt(cells, pos), 0, len(cells)-1)
	return idx / cols, idx % cols
}

func tableCellIndex(b liveBlock, row, col int) int {
	return row*len(b.table.align) + col
}

// materializeMissingTableCell 把短行的虚拟单元格补成真实 Markdown 单元格。
// 点击命中仍由视图负责，这里统一源行规范化与结构化回写。
func (a *app) materializeMissingTableCell(b liveBlock, row, col int) bool {
	cols := len(b.table.align)
	if cols == 0 || row < 0 || row >= len(b.table.rows) || col < 0 || col >= cols {
		return false
	}
	cells := tableCellPositions(b.raw)
	index := tableCellIndex(b, row, col)
	if index >= len(cells) || !cells[index].missing {
		return false
	}
	lines := strings.Split(b.raw, "\n")
	line := row
	if row > 0 {
		line++ // Markdown 分隔行不是可见行。
	}
	normalized, ok := normalizeTableRow(lines[line], cols)
	if !ok {
		return false
	}
	lines[line] = normalized
	a.replaceTable(strings.Join(lines, "\n"), row, col)
	return true
}

// replaceTable keeps the caret/selection within the same surviving cell,
// commits one structural edit, and returns keyboard focus to the cell editor.
func (a *app) replaceTable(raw string, row, col int) {
	b, ok := a.activeTable()
	if !ok {
		return
	}
	before := a.liveSnapshot()
	oldCells := tableCellPositions(b.raw)
	oldRow, oldCol := a.selectedTableCell(b)
	oldIndex := clamp(tableCellIndex(b, oldRow, oldCol), 0, len(oldCells)-1)
	old := oldCells[oldIndex]
	start, end := a.mdCaret, a.mdSelEnd
	if a.selSyncStart >= 0 {
		start, end = a.selSyncStart, a.selSyncEnd
	}
	lines := strings.Split(raw, "\n")
	table, ok := tableStart(lines, 0, len(lines))
	if !ok {
		return
	}
	cells := tableCellPositions(raw)
	cols := len(table.align)
	rows := len(cells) / cols
	row = clamp(row, 0, rows-1)
	col = clamp(col, 0, cols-1)
	cell := cells[row*cols+col]
	s, e := cell.start, cell.start
	if row == oldRow && col == oldCol {
		s += clamp(start-old.start, 0, cell.end-cell.start)
		e += clamp(end-old.start, 0, cell.end-cell.start)
	}
	a.mdDraft = raw
	a.commitDraft(s)
	a.selSyncStart, a.selSyncEnd = s, e
	a.liveFocus = 1
	a.tableTools = tableTools{}
	a.rememberLiveEdit(before, editCommand)
	if before.doc != a.activeTab().Text {
		a.onEdited()
	}
}

func tableDivider(align int) string {
	switch align {
	case -1:
		return ":---"
	case 1:
		return "---:"
	case 2:
		return ":---:"
	default:
		return "---"
	}
}

// alignTable applies one alignment to every column, preserving cell contents
// and the current cell's caret/selection. Mixed source alignments are supported
// by the parser but the toolbar's commands intentionally act on the whole table.
func (a *app) alignTable(align int) {
	b, ok := a.activeTable()
	if !ok || (align != -1 && align != 1 && align != 2) {
		return
	}
	row, col := a.selectedTableCell(b)
	lines := strings.Split(b.raw, "\n")
	cells, _ := pipeCells(lines[1])
	rs := []rune(lines[1])
	var divider strings.Builder
	off := 0
	for _, cell := range cells {
		divider.WriteString(string(rs[off:cell.start]))
		divider.WriteString(tableDivider(align))
		off = cell.end
	}
	divider.WriteString(string(rs[off:]))
	lines[1] = divider.String()
	a.replaceTable(strings.Join(lines, "\n"), row, col)
}

func tableAllAligned(alignments []int, align int) bool {
	if len(alignments) == 0 {
		return false
	}
	for _, value := range alignments {
		if value != align && !(align == -1 && value == 0) {
			return false
		}
	}
	return true
}

func tableResizeDropsContent(b liveBlock, rows, cols int) bool {
	for r, cells := range b.table.rows {
		for c, cell := range cells {
			if (r >= rows || c >= cols) && cell.text != "" {
				return true
			}
		}
	}
	return false
}

func resizedTable(b liveBlock, rows, cols int) string {
	oldLines := strings.Split(b.raw, "\n")
	out := make([]string, 0, rows+1)
	for r := 0; r < rows; r++ {
		// Preserve original whitespace and escapes when the column count and
		// source row width are unchanged.
		if r < len(b.table.rows) && cols == len(b.table.align) && len(b.table.rows[r]) == cols {
			line := r
			if r > 0 {
				line++
			}
			out = append(out, oldLines[line])
		} else {
			cells := make([]string, cols)
			if r < len(b.table.rows) {
				for c := 0; c < min(cols, len(b.table.rows[r])); c++ {
					// Already raw Markdown: don't double-escape pipes or wrappers.
					cells[c] = b.table.rows[r][c].text
				}
			}
			out = append(out, "| "+strings.Join(cells, " | ")+" |")
		}
		if r == 0 {
			divider := oldLines[1]
			if cols != len(b.table.align) {
				cells := make([]string, cols)
				newAlign := 0
				for _, align := range []int{-1, 1, 2} {
					if tableAllAligned(b.table.align, align) {
						newAlign = align
						break
					}
				}
				for c := range cells {
					align := newAlign
					if c < len(b.table.align) {
						align = b.table.align[c]
					}
					cells[c] = tableDivider(align)
				}
				divider = "| " + strings.Join(cells, " | ") + " |"
			}
			out = append(out, divider)
		}
	}
	return strings.Join(out, "\n")
}

// rows includes the header (the Markdown divider is not a visible row).
func (a *app) requestTableResize(rows, cols int) {
	b, ok := a.activeTable()
	if !ok || rows < 1 || cols < 1 || rows > tablePickerRows || cols > tablePickerCols {
		return
	}
	if tableResizeDropsContent(b, rows, cols) {
		a.tableTools.confirm = true
		a.tableTools.rows, a.tableTools.cols = rows, cols
		a.tableTools.pendingDoc = a.activeTab().Text
		return
	}
	row, col := a.selectedTableCell(b)
	a.replaceTable(resizedTable(b, rows, cols), min(row, rows-1), min(col, cols-1))
}

func (a *app) confirmTableResize() {
	b, ok := a.activeTable()
	s := a.tableTools
	if !ok || !s.confirm || s.dot != a.active || s.line != b.start || s.pendingDoc != a.activeTab().Text {
		a.tableTools = tableTools{}
		return
	}
	row, col := a.selectedTableCell(b)
	a.replaceTable(resizedTable(b, s.rows, s.cols), min(row, s.rows-1), min(col, s.cols-1))
}

func (a *app) deleteTable() {
	b, ok := a.activeTable()
	if !ok {
		return
	}
	before := a.liveSnapshot()
	doc := a.activeTab().Text
	start, end := lineByteOffset(doc, b.start), lineByteOffset(doc, b.end+1)
	newDoc := doc[:start] + doc[end:]
	blocks := liveBlocks(newDoc)
	// Prefer the following paragraph, or the preceding one at document end.
	idx := -1
	for i, block := range blocks {
		if block.kind != bkBlank {
			idx = i
			if block.start >= b.start {
				break
			}
		}
	}
	if idx < 0 {
		idx = 0
	}
	target := blocks[idx]
	line, col := target.start, 0
	if target.start < b.start {
		lastLine := strings.Split(target.raw, "\n")
		line = target.end
		col = utf8.RuneCountInString(lastLine[len(lastLine)-1])
	}
	a.applyDocEdit(newDoc, line, col)
	a.tableTools = tableTools{}
	a.liveFocus, a.scrollActive = 1, true
	a.rememberLiveEdit(before, editCommand)
	a.onEdited()
}
