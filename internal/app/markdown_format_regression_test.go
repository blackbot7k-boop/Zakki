package app

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/egoist/mygo/ui"
)

func TestIrregularTableMissingCellClickMaterializesCell(t *testing.T) {
	a := newTestApp(t)
	t.Cleanup(a.shutdown)
	a.tabs[0].Text = "| a | b |\n| --- | --- |\n| only |"
	tt := ui.NewTester(a.view, 400, 540)

	if err := tt.Click("单元格 2,2"); err != nil {
		t.Fatal(err)
	}
	tt.Type("X")

	lines := strings.Split(a.tabs[0].Text, "\n")
	cells, ok := pipeCells(lines[2])
	if !ok || len(cells) != 2 || cells[0].text != "only" || cells[1].text != "X" {
		t.Fatalf("点击短行缺失格后文本=%q, cells=%+v", a.tabs[0].Text, cells)
	}
	b, ok := a.activeTable()
	if !ok {
		t.Fatal("输入后表格失去活动状态")
	}
	row, col := a.selectedTableCell(b)
	if row != 1 || col != 1 {
		t.Fatalf("焦点格=(%d,%d), want (1,1)", row, col)
	}
}

func TestIrregularTableTabSkipsHiddenWideCell(t *testing.T) {
	a := newTestApp(t)
	t.Cleanup(a.shutdown)
	a.tabs[0].Text = "| a | b |\n| --- | --- |\n| 1 | 2 | hidden |"
	tt := ui.NewTester(a.view, 400, 540)

	if err := tt.Click("单元格 2,2"); err != nil {
		t.Fatal(err)
	}
	tt.Key(0, ui.KeyTab)
	tt.Frame()
	b, ok := a.activeTable()
	if !ok {
		t.Fatal("Tab 后表格失去活动状态")
	}
	row, col := a.selectedTableCell(b)
	if row != 2 || col != 0 {
		t.Fatalf("Tab 后焦点格=(%d,%d), want 新行 (2,0)", row, col)
	}
	tt.Type("X")

	lines := strings.Split(a.tabs[0].Text, "\n")
	if len(lines) != 4 || !strings.Contains(lines[2], "hidden") || !strings.Contains(lines[3], "X") {
		t.Fatalf("宽行 Tab 后文本=%q", a.tabs[0].Text)
	}
}

func TestTaskCheckboxClickOnlyChangesPrefix(t *testing.T) {
	a := newTestApp(t)
	t.Cleanup(a.shutdown)
	a.tabs[0].Text = "- [ ] done [ ]"
	tt := ui.NewTester(a.view, 400, 540)

	if err := tt.Click("完成：done [ ]"); err != nil {
		t.Fatal(err)
	}

	if got := a.tabs[0].Text; got != "- [x] done [ ]" {
		t.Fatalf("切换任务后=%q", got)
	}
}

func TestUnclosedFenceLastLineCanBeFocusedAndEdited(t *testing.T) {
	a := newTestApp(t)
	t.Cleanup(a.shutdown)
	a.tabs[0].Text = "```\nx\ny"
	tt := ui.NewTester(a.view, 400, 540)
	tt.Frame()
	blocks := a.mdBlocks()
	if len(blocks) != 1 || len(blocks[0].lines) != 2 || blocks[0].lines[1] != "y" {
		t.Fatalf("未闭合围栏末行不可见: %+v", blocks)
	}
	end := utf8.RuneCountInString(blocks[0].raw)
	a.focusBlock(0, end, end)
	tt.Frame()
	if a.mdVisible != "x\ny" {
		t.Fatalf("未闭合围栏可编辑投影=%q", a.mdVisible)
	}

	tt.Type("Z")

	if got := a.tabs[0].Text; got != "```\nx\nyZ" {
		t.Fatalf("编辑未闭合围栏末行后=%q", got)
	}
}

func TestDocWideFormatsSkipFencesAndTables(t *testing.T) {
	doc := "first\n\n```go\nfmt.Println(`code`)\n```\n\n| head | value |\n| --- | --- |\n| **cell** | 1 |\n\nlast"
	protected := "```go\nfmt.Println(`code`)\n```\n\n| head | value |\n| --- | --- |\n| **cell** | 1 |"
	tests := []struct {
		name  string
		cmd   string
		first string
		last  string
	}{
		{name: "bold", cmd: cmdBold, first: "**first**", last: "**last**"},
		{name: "heading", cmd: cmdH2, first: "## first", last: "## last"},
		{name: "list", cmd: cmdBullet, first: "- first", last: "- last"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			a := newTestApp(t)
			t.Cleanup(a.shutdown)
			a.winOpen = true
			a.tabs[0].Text = doc
			a.resyncLive(0)
			a.focusBlock(len(a.mdBlocks())-1, 0, 0)
			a.selectDocWide()
			a.execFormatCommand(tc.cmd)
			got := a.tabs[0].Text
			if !strings.Contains(got, protected) || !strings.HasPrefix(got, tc.first) || !strings.HasSuffix(got, tc.last) {
				t.Fatalf("文档级格式不得改写围栏或表格: %q", got)
			}
		})
	}
}
