package main

import (
	"strings"
	"unicode/utf8"
)

// createTableOnEnter 将完整、独立的表头行转为标准 Markdown 表格。
func (a *app) createTableOnEnter(caret, selEnd int, blk liveBlock) bool {
	if blk.kind != bkPara || strings.Contains(a.mdDraft, "\n") || caret != selEnd || caret != utf8.RuneCountInString(a.mdDraft) {
		return false
	}
	head, ok := pipeCells(a.mdDraft)
	if !ok || len(head) < 2 {
		return false
	}
	// 分隔线自己不是候选表头。
	allSeparators := true
	for _, cell := range head {
		if strings.Trim(cell.text, "-:") != "" {
			allSeparators = false
		}
	}
	if allSeparators {
		return false
	}
	a.autoTableBefore = a.activeTab().Text
	a.autoTableLine, a.autoTableCol = blk.start, caret
	a.mdDraft += "\n|" + strings.Repeat(" --- |", len(head)) + "\n" + emptyTableRow(len(head)) + "\n"
	a.commitDraft(-1)
	// Keep the grid on screen and focus its first data cell immediately.
	a.autoTableAfter = a.activeTab().Text
	cells, _ := pipeCells(emptyTableRow(len(head)))
	a.blockFromLineCol(blk.start+2, cells[0].start)
	a.autoTableCaret = a.selSyncStart
	return true
}

// navigateTable selects adjacent cells while the grid stays visible;
// Tab from the final cell adds a data row.
func (a *app) navigateTable(backward bool, caret int) {
	cells := tableCellPositions(a.mdDraft)
	if len(cells) == 0 {
		return
	}
	idx := 0
	for i, cell := range cells {
		if cell.start <= caret {
			idx = i
		}
	}
	if backward {
		idx--
	} else {
		idx++
	}
	if idx < 0 {
		return
	}
	if idx >= len(cells) {
		table, ok := tableStart(strings.Split(a.mdDraft, "\n"), 0, strings.Count(a.mdDraft, "\n")+1)
		if !ok {
			return
		}
		a.mdDraft += "\n" + emptyTableRow(len(table.align))
		cells = tableCellPositions(a.mdDraft)
		idx = len(cells) - len(table.align)
		if idx < 0 {
			idx = 0
		}
		a.publishDraft(cells[idx].start)
		a.onEdited()
	}
	a.selSyncStart, a.selSyncEnd = cells[idx].start, cells[idx].end
}

func (a *app) tableEnter(caret int) {
	lines := strings.Split(a.mdDraft, "\n")
	table, ok := tableStart(lines, 0, len(lines))
	if !ok {
		return
	}
	line := strings.Count(a.mdDraft[:runeToByte(a.mdDraft, caret)], "\n")
	if line <= 1 && len(lines) > 2 {
		cells := tableCellPositions(a.mdDraft)
		a.selSyncStart, a.selSyncEnd = cells[len(table.align)].start, cells[len(table.align)].end
		return
	}
	if line == len(lines)-1 && line >= 2 {
		cells, _ := pipeCells(lines[line])
		empty := true
		for _, cell := range cells {
			if cell.text != "" {
				empty = false
			}
		}
		if empty {
			a.mdDraft = strings.Join(lines[:line], "\n") + "\n\n"
			a.commitDraft(utf8.RuneCountInString(a.mdDraft))
			return
		}
	}
	row := emptyTableRow(len(table.align))
	insert := min(line+1, len(lines))
	lines = append(lines[:insert], append([]string{row}, lines[insert:]...)...)
	a.mdDraft = strings.Join(lines, "\n")
	cells, _ := pipeCells(row)
	pos := utf8.RuneCountInString(strings.Join(lines[:insert], "\n")) + 1 + cells[0].start
	a.commitDraft(pos)
}
