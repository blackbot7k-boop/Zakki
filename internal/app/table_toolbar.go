package app

import (
	"fmt"

	"github.com/egoist/mygo/ui"

	"zakki/locales"
)

const (
	tablePickerCols = 8
	tablePickerRows = 10
)

func (a *app) tableToolButton(c *ui.Context, t *ui.Theme, key, label string, icon *ui.SVG, selected bool) ui.Element {
	btn := ui.ButtonBase(c.Key(key)).Label(label).Tooltip(label).
		Size(28, 28).Radius(4).Cursor(ui.CursorPointer).FocusRing(true).
		Disabled(a.mdComposing)
	ink := t.TextMuted
	if selected {
		ink = t.Accent
		btn.Background(t.Accent.Mix(t.Background, 0.90))
	} else if btn.Hovered() {
		btn.Background(t.Surface)
	}
	btn.Children(func() {
		ui.Icon(c, icon).Size(17, 17).TextColor(ink)
	})
	return btn
}

func (a *app) tableToolbar(c *ui.Context, t *ui.Theme, blk liveBlock) {
	if a.tableTools.dot != a.active || a.tableTools.line != blk.start {
		a.tableTools = tableTools{dot: a.active, line: blk.start}
	}
	ui.Row(c.Key("table-tools")).FillWidth().Gap(2).AlignItems(ui.Center).
		Label(locales.TableToolbar()).Children(func() {
		anchor := a.tableToolButton(c, t, "table-size", locales.TableResize(), iconTable, a.tableTools.open)
		anchor.OnClick(func() {
			a.liveClickHandled = true
			a.tableTools.open = !a.tableTools.open
			a.tableTools.confirm = false
			a.tableTools.pendingDoc = ""
			a.tableTools.hoverRows, a.tableTools.hoverCols = len(blk.table.rows), len(blk.table.align)
		})
		for _, action := range []struct {
			key, label string
			icon       *ui.SVG
			align      int
		}{
			{"table-left", locales.TableAlignLeft(), iconAlignLeft, -1},
			{"table-center", locales.TableAlignCenter(), iconAlignCenter, 2},
			{"table-right", locales.TableAlignRight(), iconAlignRight, 1},
		} {
			selected := tableAllAligned(blk.table.align, action.align)
			a.tableToolButton(c, t, action.key, action.label, action.icon, selected).OnClick(func() {
				a.liveClickHandled = true
				a.alignTable(action.align)
			})
		}
		ui.Box(c).Grow(1)
		a.tableToolButton(c, t, "table-delete", locales.TableDelete(), iconTrash, false).OnClick(func() {
			a.liveClickHandled = true
			a.deleteTable()
		})
		popover := ui.Popover(c.Key("table-size-popup"), anchor, &a.tableTools.open, func() {
			ui.Column(c).Gap(8).Children(func() {
				if a.tableTools.confirm {
					ui.Text(c, locales.TableShrinkWarn()).FontSize(12).Bold()
					ui.Text(c, shortcutText(locales.TableResizeConfirm(a.tableTools.cols, a.tableTools.rows))).FontSize(11).TextColor(t.TextMuted)
					ui.Row(c).Gap(8).Children(func() {
						ui.Button(c.Key("cancel-resize"), locales.CommonCancel()).OnClick(func() {
							a.liveClickHandled = true
							a.tableTools = tableTools{}
							a.liveFocus = 1
						})
						ui.Button(c.Key("confirm-resize"), locales.TableConfirmShrink()).OnClick(func() {
							a.liveClickHandled = true
							a.confirmTableResize()
						})
					})
					return
				}
				a.tableSizePicker(c, t)
			})
		})
		if popover.Dismissed() {
			a.tableTools.confirm = false
			a.tableTools.pendingDoc = ""
		}
	}).OnShortcut(ui.Super, ui.KeyZ, func() { a.undoLive(false) }).
		OnShortcut(ui.Super|ui.Shift, ui.KeyZ, func() { a.undoLive(true) }).
		OnShortcut(ui.Ctrl, ui.KeyZ, func() { a.undoLive(false) }).
		OnShortcut(ui.Ctrl|ui.Shift, ui.KeyZ, func() { a.undoLive(true) })
}

func (a *app) tableSizePicker(c *ui.Context, t *ui.Theme) {
	ui.Text(c, locales.TableSizeAt(a.tableTools.hoverCols, a.tableTools.hoverRows)).FontSize(12).Bold()
	// 先构建全部命中目标，让每格使用同一帧的悬停矩形；元素不跨帧保存。
	var cells []ui.Element
	ui.Scroll(c.Key("table-size-grid")).MaxHeight(210).Children(func() {
		ui.Column(c).Gap(2).Children(func() {
			for row := 1; row <= tablePickerRows; row++ {
				ui.Row(c.Key(fmt.Sprintf("size-row-%d", row))).Gap(2).Children(func() {
					for col := 1; col <= tablePickerCols; col++ {
						btn := ui.ButtonBase(c.Key(fmt.Sprintf("size-col-%d", col))).
							Size(28, 28).Radius(2).Border(1, t.Border).FocusRing(true).
							Label(locales.TablePickSize(col, row)).Cursor(ui.CursorPointer)
						if btn.Focused() {
							btn.ScrollIntoView()
						}
						btn.OnClick(func() {
							a.liveClickHandled = true
							a.requestTableResize(row, col)
						})
						cells = append(cells, btn)
					}
				})
			}
		})
	})
	target := -1
	for i, cell := range cells {
		if cell.Focused() {
			target = i
		}
	}
	for i, cell := range cells {
		if cell.Hovered() {
			target = i
			break
		}
	}
	if target >= 0 {
		rows, cols := target/tablePickerCols+1, target%tablePickerCols+1
		if rows != a.tableTools.hoverRows || cols != a.tableTools.hoverCols {
			a.tableTools.hoverRows, a.tableTools.hoverCols = rows, cols
			c.Invalidate()
		}
	}
	for i, cell := range cells {
		if i/tablePickerCols < a.tableTools.hoverRows && i%tablePickerCols < a.tableTools.hoverCols {
			cell.Background(t.Accent.Mix(t.Background, 0.78)).BorderColor(t.Accent)
		}
	}
	ui.Text(c, locales.TableSizeHint()).FontSize(10).TextColor(t.TextMuted)
}
