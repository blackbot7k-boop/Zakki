package main

import (
	"fmt"
	"strconv"
	"unicode/utf8"

	"github.com/egoist/mygo/ui"
)

func styleHeading(h ui.Element, level int, md resolvedStyle) {
	level = clamp(level, 1, 6)
	h.FontSize(md.headingSizes[level-1]).LineHeight(1.25).Bold()
}

// Separate the checkbox hit target from the editable text. Clicking the text
// edits the task; clicking the square toggles it, even while its text has focus.
func (a *app) taskCheckbox(c *ui.Context, blk liveBlock) {
	checked := blk.checked
	ui.Checkbox(c.Key("task-"+strconv.Itoa(blk.start)), &checked, "").
		Label("完成：" + projectBlock(blk).text).OnChange(func() {
		before := a.liveSnapshot()
		a.activeTab().Text = toggleTaskLine(a.activeTab().Text, blk.start)
		a.resyncLive(a.mdCaret)
		a.rememberLiveEdit(before, editCommand)
		a.onEdited()
	})
}

func (a *app) recordProjectedKey(ev ui.InputEvent, p liveProjection) bool {
	if ev.Kind == ui.InputCommand && ev.Text == "paste" {
		a.nextEditIsPaste = true // 粘贴是独立撤销事务
	}
	if a.mdComposing {
		return false
	}
	if ev.Kind == ui.InputCommand && (ev.Text == "undo" || ev.Text == "redo") {
		a.liveUndoPending = 1
		if ev.Text == "redo" {
			a.liveUndoPending = -1
		}
		return true
	}
	if ev.Kind == ui.InputKeyDown && ev.Key == ui.KeyZ &&
		(ev.Mods == ui.Super || ev.Mods == ui.Ctrl || ev.Mods == ui.Super|ui.Shift || ev.Mods == ui.Ctrl|ui.Shift) {
		a.liveUndoPending = 1
		if ev.Mods&ui.Shift != 0 {
			a.liveUndoPending = -1
		}
		return true
	}
	if ev.Kind == ui.InputKeyDown && ev.Key == ui.KeyTab && p.cell && (ev.Mods == 0 || ev.Mods == ui.Shift) {
		a.tableBackward = ev.Mods&ui.Shift != 0
		a.keyHint = ui.KeyTab
		return true
	}
	if ev.Kind == ui.InputKeyDown && ev.Mods == 0 && ev.Key == ui.KeyBackspace &&
		p.visibleOffset(a.mdCaret) == 0 && a.mdSelEnd <= a.mdCaret {
		// Backspace cannot delete invisible syntax or a table border.
		if p.cell && a.activeTab().Text == a.autoTableAfter && a.mdCaret == a.autoTableCaret {
			a.tableUndoPending = true
		} else if !p.cell {
			a.keyHint = ui.KeyBackspace
		}
		return true
	}
	return a.recordKeyHint(ev)
}

func (a *app) applyProjectedHint(key ui.Key, p liveProjection, start, end int) {
	s, e := p.sourceOffset(start), p.sourceOffset(end)
	if p.cell {
		// Table Enter and Tab retain the existing row/navigation semantics;
		// other keys remain within the cell instead of escaping through pipes.
		if key == ui.KeyEnter || key == ui.KeyTab {
			a.applyKeyHint(key, s, e)
		}
		return
	}
	if key == ui.KeyEnter {
		blk := a.mdBlocks()[a.mdActive]
		if blk.kind == bkPara || blk.kind == bkHeading || blk.kind == bkList || blk.kind == bkTask {
			if len(p.units) == 0 {
				a.afterEnter(p.sourceOffset(start), p.sourceOffset(end))
				return
			}
			if a.createTableOnEnter(s, e, blk) {
				return
			}
			left, right := p.fragment(0, start), p.fragment(end, len(p.units))
			prefix := p.raw[:runeToByte(p.raw, p.start)]
			a.mdDraft = prefix + left
			caret := utf8.RuneCountInString(a.mdDraft)
			if blk.kind == bkList || blk.kind == bkTask {
				if start == 0 && start == end {
					a.mdDraft += "\n" + prefix + right
				} else {
					continuation := listContinuePrefix(blk)
					a.mdDraft += "\n" + continuation + right
					caret += 1 + utf8.RuneCountInString(continuation)
				}
			} else {
				a.mdDraft += "\n\n" + right
				caret += 2
			}
			a.publishDraft(caret)
			return
		}
	}
	switch key {
	case ui.KeyUp, ui.KeyLeft, ui.KeyBackspace:
		if start == 0 {
			s = 0
		}
	case ui.KeyDown, ui.KeyRight:
		if start == len(p.units) {
			s = utf8.RuneCountInString(p.raw)
		}
	case ui.KeyEnter:
		if start == 0 && (a.mdBlocks()[a.mdActive].kind == bkList || a.mdBlocks()[a.mdActive].kind == bkTask) {
			s = 0
		}
	}
	if start == end {
		e = s
	}
	a.applyKeyHint(key, s, e)
}

func (a *app) editTable(c *ui.Context, t *ui.Theme, blk liveBlock) {
	md := a.resolveStyle(-1, themeIsDark(t), true)
	ui.Column(c.Key("editable-table")).FillWidth().Gap(4).Children(func() {
		a.tableToolbar(c, t, blk)
		a.tableGrid(c, t, md, blk, true)
	})
}

func tableCellAt(cells []tableCell, caret int) int {
	idx := 0
	for i, cell := range cells {
		if cell.start <= caret {
			idx = i
		}
	}
	return idx
}

func (a *app) tableGrid(c *ui.Context, t *ui.Theme, md resolvedStyle, blk liveBlock, editing bool) {
	accent := md.accent
	cols := len(blk.table.align)
	if cols == 0 {
		return
	}
	positions := tableCellPositions(blk.raw)
	pos := a.mdCaret
	if a.selSyncStart >= 0 {
		pos = a.selSyncStart
	}
	target := tableCellAt(positions, pos)
	blockIndex := a.mdActive
	if !editing {
		for i, b := range a.mdBlocks() {
			if b.start == blk.start {
				blockIndex = i
				break
			}
		}
	}
	index := 0
	ui.Column(c).FillWidth().Border(1, t.Border).Radius(6).Clip().Children(func() {
		for row, cells := range blk.table.rows {
			ui.Row(c.Key(fmt.Sprintf("row-%d", row))).FillWidth().Children(func() {
				for col := 0; col < cols; col++ {
					ci := index + col
					cell := ui.Box(c.Key(fmt.Sprintf("cell-%d", col))).Grow(1).Basis(0).MinWidth(0).Padding(md.tablePadY, md.tablePadX).
						BorderWidth(0, 0, 1, 1).BorderColor(t.Border).
						Label(fmt.Sprintf("单元格 %d,%d", row+1, col+1))
					if row == 0 {
						cell.Background(t.Surface).Bold()
					}
					cell.Children(func() {
						if editing && ci == target && col < len(cells) {
							p := projectRegion(blk.raw, positions[ci].start, positions[ci].end, true, true)
							// Alignment and header weight are inherited by the input.
							ui.Box(c).FillWidth().TextAlign(tableAlignment(blk.table.align[col])).Children(func() {
								a.activeText(c, t, blk, p)
							})
						} else {
							p := liveProjection{text: " "}
							if col < len(cells) {
								p = projectRegion(cells[col].text, 0, utf8.RuneCountInString(cells[col].text), true, true)
							}
							p.content(c, t, accent).FillWidth().FontSize(md.codeSize).TextAlign(tableAlignment(blk.table.align[col]))
						}
					})
					cell.OnClick(func() {
						a.liveClickHandled = true
						a.focusTableCell(blockIndex, row, col)
					})
				}
			})
			index += len(cells)
		}
	})
}

func tableAlignment(align int) ui.Align {
	switch align {
	case 1:
		return ui.End
	case 2:
		return ui.Center
	default:
		return ui.Start
	}
}

func (a *app) focusTableCell(block, row, col int) {
	a.focusBlock(block, 0, 0)
	b := a.mdBlocks()[a.mdActive]
	cells := tableCellPositions(b.raw)
	index := col
	for i := 0; i < row; i++ {
		index += len(b.table.rows[i])
	}
	if index >= len(cells) || col >= len(b.table.rows[row]) {
		return
	}
	cell := cells[index]
	a.selSyncStart, a.selSyncEnd = cell.end, cell.end
	a.liveFocus = 1
}
