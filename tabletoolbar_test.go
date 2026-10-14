package main

import (
	"fmt"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

const toolbarTable = "| 名称 | 数量 |\n| :--- | ---: |\n| **中文** | 2 |"

func tableToolbarApp(t *testing.T, doc string) (*app, *ui.Tester) {
	t.Helper()
	a := newTestApp(t)
	t.Cleanup(a.shutdown)
	a.tabs[0].Text, a.mdActive = doc, -1
	tt := ui.NewTester(a.view, 400, 540)
	return a, tt
}

func tableToolClick(t *testing.T, tt *ui.Tester, label string) {
	t.Helper()
	if err := tt.Click(label); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
}

func TestTableToolbarAlignment(t *testing.T) {
	for _, tc := range []struct {
		label, divider string
		align          int
	}{
		{"表格左对齐", ":---", -1},
		{"表格居中对齐", ":---:", 2},
		{"表格右对齐", "---:", 1},
	} {
		t.Run(tc.label, func(t *testing.T) {
			a, tt := tableToolbarApp(t, toolbarTable)
			if _, ok := tt.Find("调整表格大小"); ok {
				t.Fatal("preview-only table must not show tools")
			}
			tableToolClick(t, tt, "单元格 2,1")
			tableToolClick(t, tt, tc.label)
			want := strings.Replace(toolbarTable, "| :--- | ---: |", "| "+tc.divider+" | "+tc.divider+" |", 1)
			if a.tabs[0].Text != want || !tableAllAligned(a.mdBlocks()[0].table.align, tc.align) {
				t.Fatalf("alignment must apply to every column: %q", a.tabs[0].Text)
			}
			if !tt.Focused("内容") || a.mdVisible != "中文" {
				t.Fatalf("toolbar lost cell focus: %q", a.mdVisible)
			}
			tt.Type("好")
			if !strings.Contains(a.tabs[0].Text, "**中文好**") {
				t.Fatalf("toolbar lost caret or wrappers: %q", a.tabs[0].Text)
			}
			tt.Key(ui.Super, ui.KeyZ)
			if a.tabs[0].Text != want {
				t.Fatalf("typing undo: %q", a.tabs[0].Text)
			}
			tt.Key(ui.Super, ui.KeyZ)
			if a.tabs[0].Text != toolbarTable {
				t.Fatalf("alignment undo: %q", a.tabs[0].Text)
			}
			tt.Key(ui.Super|ui.Shift, ui.KeyZ)
			if a.tabs[0].Text != want {
				t.Fatalf("alignment redo: %q", a.tabs[0].Text)
			}
			// Clicking the already-active alignment restores focus but must not
			// create a redundant undo entry.
			count := len(a.liveUndo)
			tableToolClick(t, tt, tc.label)
			if len(a.liveUndo) != count || a.tabs[0].Text != want || !tt.Focused("内容") {
				t.Fatal("same alignment added a history entry or lost focus")
			}
		})
	}
}

func TestTableToolbarAllColumnsAndSelection(t *testing.T) {
	a, tt := tableToolbarApp(t, toolbarTable)
	tableToolClick(t, tt, "单元格 2,2")
	tt.Key(ui.Super, ui.KeyA)
	tableToolClick(t, tt, "表格居中对齐")
	if !strings.Contains(a.tabs[0].Text, "| :---: | :---: |") {
		t.Fatal(a.tabs[0].Text)
	}
	tt.Type("替换")
	if a.mdBlocks()[0].table.rows[1][1].text != "替换" {
		t.Fatalf("selection was not preserved: %q", a.tabs[0].Text)
	}
}

func TestTableToolbarGrowHoverAndUndo(t *testing.T) {
	doc := "前文\n\n" + toolbarTable + "\n\n后文"
	a, tt := tableToolbarApp(t, doc)
	tableToolClick(t, tt, "单元格 2,2")
	tableToolClick(t, tt, "调整表格大小")
	r, ok := tt.Find("4 列 3 行")
	if !ok {
		t.Fatal("missing size picker")
	}
	tt.Move(r.X+r.W/2, r.Y+r.H/2)
	if !tt.HasText("4 列 × 3 行") {
		t.Fatalf("hover did not update dimensions: %v", tt.Texts())
	}
	tableToolClick(t, tt, "4 列 3 行")
	b := a.mdBlocks()[a.mdActive]
	if len(b.table.rows) != 3 || len(b.table.align) != 4 || b.table.align[0] != -1 || b.table.align[1] != 1 {
		t.Fatalf("resize dimensions/alignment: %+v", b.table)
	}
	if b.table.rows[1][0].text != "**中文**" || b.table.rows[1][1].text != "2" || !strings.HasPrefix(a.tabs[0].Text, "前文\n\n") || !strings.HasSuffix(a.tabs[0].Text, "\n\n后文") {
		t.Fatal(a.tabs[0].Text)
	}
	if a.tableTools.open || !tt.Focused("内容") || a.mdVisible != "2" {
		t.Fatalf("resize lost selected cell: %q", a.mdVisible)
	}
	resized := a.tabs[0].Text
	tt.Key(ui.Super, ui.KeyZ)
	if a.tabs[0].Text != doc {
		t.Fatalf("resize should undo in one step: %q", a.tabs[0].Text)
	}
	tt.Key(ui.Super|ui.Shift, ui.KeyZ)
	if a.tabs[0].Text != resized {
		t.Fatalf("resize redo: %q", a.tabs[0].Text)
	}
	tt.Type("X")
	if a.mdBlocks()[a.mdActive].table.rows[1][1].text != "2X" {
		t.Fatalf("resize lost caret: %q", a.tabs[0].Text)
	}
}

func TestTableToolbarShrinkConfirmation(t *testing.T) {
	a, tt := tableToolbarApp(t, toolbarTable)
	tableToolClick(t, tt, "单元格 2,2")
	tableToolClick(t, tt, "调整表格大小")
	tableToolClick(t, tt, "1 列 1 行")
	if !a.tableTools.confirm || a.tabs[0].Text != toolbarTable || !tt.HasText("缩小会删除有内容的单元格") {
		t.Fatalf("content must not be silently removed: %q", a.tabs[0].Text)
	}
	tableToolClick(t, tt, "取消")
	if a.tabs[0].Text != toolbarTable || a.tableTools.open || !tt.Focused("内容") {
		t.Fatal("cancel changed data or lost focus")
	}
	tableToolClick(t, tt, "调整表格大小")
	tableToolClick(t, tt, "1 列 1 行")
	tableToolClick(t, tt, "确认缩小")
	want := "| 名称 |\n| :--- |"
	if a.tabs[0].Text != want || a.mdVisible != "名称" || !tt.Focused("内容") {
		t.Fatalf("shrink did not clamp removed-cell focus: %q visible=%q", a.tabs[0].Text, a.mdVisible)
	}
	tt.Key(ui.Super, ui.KeyZ)
	if a.tabs[0].Text != toolbarTable {
		t.Fatalf("shrink must undo all deleted content: %q", a.tabs[0].Text)
	}
}

func TestTableToolbarEmptyShrinkAndEscapes(t *testing.T) {
	doc := "| a\\|b | c |  |\n| --- | :---: | ---: |\n| **值** | x\\\\y |  |\n|  |  |  |"
	a, tt := tableToolbarApp(t, doc)
	tableToolClick(t, tt, "单元格 2,2")
	tableToolClick(t, tt, "调整表格大小")
	tableToolClick(t, tt, "2 列 2 行")
	if a.tableTools.confirm || a.tableTools.open {
		t.Fatal("empty trimmed cells should not need confirmation")
	}
	want := "| a\\|b | c |\n| --- | :---: |\n| **值** | x\\\\y |"
	if a.tabs[0].Text != want {
		t.Fatalf("resize double-escaped source: %q", a.tabs[0].Text)
	}
	tt.Key(ui.Super, ui.KeyZ)
	if a.tabs[0].Text != doc {
		t.Fatal(a.tabs[0].Text)
	}
}

func TestTableToolbarDeleteAndUndo(t *testing.T) {
	for _, doc := range []string{
		toolbarTable,
		"前文\n\n" + toolbarTable,
		toolbarTable + "\n\n后文",
		"前文\n\n" + toolbarTable + "\n\n后文",
	} {
		t.Run(doc, func(t *testing.T) {
			a, tt := tableToolbarApp(t, doc)
			tableToolClick(t, tt, "单元格 2,1")
			tableToolClick(t, tt, "删除表格")
			if strings.Contains(a.tabs[0].Text, "| 名称 |") || !tt.Focused("内容") {
				t.Fatalf("delete did not restore paragraph focus: %q", a.tabs[0].Text)
			}
			deleted := a.tabs[0].Text
			tt.Key(ui.Super, ui.KeyZ)
			if a.tabs[0].Text != doc || a.mdVisible != "中文" || !tt.Focused("内容") {
				t.Fatalf("delete undo lost table/cell focus: %q visible=%q", a.tabs[0].Text, a.mdVisible)
			}
			tt.Key(ui.Super|ui.Shift, ui.KeyZ)
			if a.tabs[0].Text != deleted {
				t.Fatal(a.tabs[0].Text)
			}
			tt.Type("X")
			if !strings.Contains(a.tabs[0].Text, "X") {
				t.Fatal("cannot type after deleting table")
			}
		})
	}
}

func TestTableToolbarPopupDismissAndScope(t *testing.T) {
	a, tt := tableToolbarApp(t, toolbarTable+"\n\n后文")
	tableToolClick(t, tt, "单元格 2,1")
	for i := 0; i < 3; i++ {
		tableToolClick(t, tt, "调整表格大小")
		tt.Key(0, ui.KeyEscape)
		if a.tableTools.open || a.tabs[0].Text != toolbarTable+"\n\n后文" {
			t.Fatal("Escape must close only popup")
		}
	}
	tableToolClick(t, tt, "调整表格大小")
	// The panel covers the paragraph's left side. Click its unobscured right
	// side: a label-center click would hit the popup, not the paragraph.
	r, ok := tt.Find("后文")
	if !ok {
		t.Fatal("missing following paragraph")
	}
	tt.ClickAt(r.X+r.W-2, r.Y+r.H/2)
	tt.Frame()
	if a.tableTools.open || a.mdVisible != "后文" {
		t.Fatalf("outside click did not focus its target: open=%v visible=%q rect=%+v", a.tableTools.open, a.mdVisible, r)
	}
	tableToolClick(t, tt, "单元格 2,1")
	tableToolClick(t, tt, "调整表格大小")
	tableToolClick(t, tt, "标签 2")
	if a.tableTools.open || a.mdActive != -1 {
		t.Fatal("picker leaked across dots")
	}
	a.switchTab(0)
	tt.Frame()
	tableToolClick(t, tt, "单元格 2,1")
	tableToolClick(t, tt, "调整表格大小")
	a.cfg.Source = true
	tt.Frame()
	if a.tableTools.open {
		t.Fatal("picker leaked into source mode")
	}
}

func TestTableToolbarStaleConfirmation(t *testing.T) {
	a, tt := tableToolbarApp(t, toolbarTable)
	tableToolClick(t, tt, "单元格 2,1")
	tableToolClick(t, tt, "调整表格大小")
	tableToolClick(t, tt, "1 列 1 行")
	a.tabs[0].Text += "\n\n新内容"
	a.confirmTableResize()
	if !strings.HasPrefix(a.tabs[0].Text, toolbarTable) || a.tableTools.open {
		t.Fatal("stale confirmation deleted current content")
	}
}

func TestTableToolbarSmallWindowAndThemes(t *testing.T) {
	for _, dark := range []bool{false, true} {
		t.Run(fmt.Sprint(dark), func(t *testing.T) {
			a := newTestApp(t)
			t.Cleanup(a.shutdown)
			a.tabs[0].Text, a.mdActive = toolbarTable, -1
			tt := ui.NewTester(a.view, 300, 340)
			tt.SetDark(dark)
			tableToolClick(t, tt, "单元格 2,1")
			tableToolClick(t, tt, "调整表格大小")
			r, ok := tt.Find("8 列 10 行")
			if !ok || r.X < 0 || r.Y < 0 || r.X+r.W > 300 || r.Y+r.H > 340 {
				t.Fatalf("picker clipped at minimum window size: %+v", r)
			}
			tableToolClick(t, tt, "8 列 10 行")
			if a.tableTools.open || len(a.mdBlocks()[a.mdActive].table.rows) != 10 {
				t.Fatal("minimum-window picker is not clickable")
			}
		})
	}
}
