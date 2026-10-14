package main

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestTableToolbarRowOnlyResizePreservesSource(t *testing.T) {
	doc := "  |  名称  |数量|\n| :---- | ----: |\n |  a\\|b | **值**  |\n|  |  |"
	a, tt := tableToolbarApp(t, doc)
	tableToolClick(t, tt, "单元格 2,2")
	tableToolClick(t, tt, "调整表格大小")
	tableToolClick(t, tt, "2 列 2 行")
	want := strings.TrimSuffix(doc, "\n|  |  |")
	if a.tabs[0].Text != want {
		t.Fatalf("row-only resize reformatted source: %q", a.tabs[0].Text)
	}
	tt.Key(ui.Super, ui.KeyZ)
	if a.tabs[0].Text != doc {
		t.Fatal(a.tabs[0].Text)
	}
}

func TestTableToolbarNormalizesShortRows(t *testing.T) {
	a, tt := tableToolbarApp(t, "| a | b | c |\n| --- | :---: | ---: |\n| 短行 |")
	tableToolClick(t, tt, "单元格 2,1")
	tableToolClick(t, tt, "调整表格大小")
	tableToolClick(t, tt, "4 列 3 行")
	b := a.mdBlocks()[a.mdActive]
	if len(b.table.rows) != 3 || len(b.table.rows[1]) != 4 || b.table.rows[1][0].text != "短行" || a.mdVisible != "短行" {
		t.Fatalf("short row normalization lost content/focus: %q", a.tabs[0].Text)
	}
	tt.Type("好")
	if a.mdBlocks()[a.mdActive].table.rows[1][0].text != "短行好" {
		t.Fatal(a.tabs[0].Text)
	}
}

func TestTableToolbarUndoWhilePopupOpen(t *testing.T) {
	a, tt := tableToolbarApp(t, toolbarTable)
	tableToolClick(t, tt, "单元格 2,2")
	tableToolClick(t, tt, "表格居中对齐")
	tableToolClick(t, tt, "调整表格大小")
	tt.Key(ui.Super, ui.KeyZ)
	if a.tabs[0].Text != toolbarTable || a.tableTools.open || !tt.Focused("内容") {
		t.Fatalf("toolbar-focused undo failed: %q open=%v", a.tabs[0].Text, a.tableTools.open)
	}
}

func TestTableToolbarNewColumnsInheritWholeTableAlignment(t *testing.T) {
	a, tt := tableToolbarApp(t, toolbarTable)
	tableToolClick(t, tt, "单元格 2,2")
	tableToolClick(t, tt, "表格右对齐")
	tableToolClick(t, tt, "调整表格大小")
	tableToolClick(t, tt, "4 列 3 行")
	b := a.mdBlocks()[a.mdActive]
	if len(b.table.align) != 4 || !tableAllAligned(b.table.align, 1) {
		t.Fatalf("new columns lost whole-table alignment: %v", b.table.align)
	}
}

func TestTableAllAlignedMixedState(t *testing.T) {
	if tableAllAligned([]int{-1, 1, 2}, -1) || tableAllAligned([]int{-1, 1, 2}, 1) || tableAllAligned([]int{-1, 1, 2}, 2) {
		t.Fatal("mixed table must not mark one alignment active")
	}
	if !tableAllAligned([]int{-1, 0, -1}, -1) || tableAllAligned(nil, -1) {
		t.Fatal("default-left and explicit-left should share the active state")
	}
}

func TestTableToolbarCompositionCannotBeDestroyed(t *testing.T) {
	a, tt := tableToolbarApp(t, toolbarTable)
	tableToolClick(t, tt, "单元格 2,2")
	tt.Compose("ni", 2)
	tt.Frame()
	if !a.mdComposing {
		t.Fatal("composition not active")
	}
	a.deleteTable()
	a.alignTable(2)
	a.requestTableResize(1, 1)
	if a.tabs[0].Text != toolbarTable || a.tableTools.confirm {
		t.Fatal("structural operations destroyed IME preedit")
	}
	tt.Type("你")
	if a.mdBlocks()[a.mdActive].table.rows[1][1].text != "2你" {
		t.Fatalf("IME commit after disabled tools failed: %q", a.tabs[0].Text)
	}
}
