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

// selectedTableCell also handles short Markdown rows, rather than assuming
// that every row contains as many source cells as the header.
func (a *app) selectedTableCell(b liveBlock) (row, col int) {
	pos := a.mdCaret
	if a.selSyncStart >= 0 {
		pos = a.selSyncStart
	}
	idx := tableCellAt(tableCellPositions(b.raw), pos)
	for r, cells := range b.table.rows {
		if idx < len(cells) {
			return r, min(idx, len(b.table.align)-1)
		}
		idx -= len(cells)
	}
	return 0, 0
}

func tableCellIndex(b liveBlock, row, col int) int {
	idx := col
	for r := 0; r < row; r++ {
		idx += len(b.table.rows[r])
	}
	return idx
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
	old := oldCells[tableCellIndex(b, oldRow, oldCol)]
	start, end := a.mdCaret, a.mdSelEnd
	if a.selSyncStart >= 0 {
		start, end = a.selSyncStart, a.selSyncEnd
	}
	lines := strings.Split(raw, "\n")
	cellRows := append([]string{lines[0]}, lines[2:]...)
	row = clamp(row, 0, len(cellRows)-1)
	cells, _ := pipeCells(cellRows[row])
	col = clamp(col, 0, len(cells)-1)
	idx := col
	for r := 0; r < row; r++ {
		cs, _ := pipeCells(cellRows[r])
		idx += len(cs)
	}
	cell := tableCellPositions(raw)[idx]
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
