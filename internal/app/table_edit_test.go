package app

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/egoist/mygo/ui"
)

// TestCreateTableOnEnter 输入完整表头行按回车自动补分隔行和空数据行，光标进第一个单元格。
func TestCreateTableOnEnter(t *testing.T) {
	a := newTestApp(t)
	t.Cleanup(a.shutdown)
	tt := ui.NewTester(a.view, 400, 540)
	a.tabs[0].Text = "| 名称 | 数量 |"
	n := utf8.RuneCountInString(a.tabs[0].Text)
	a.selSyncStart, a.selSyncEnd = n, n
	tt.Frame()
	tt.Key(0, ui.KeyEnter)
	for frame := 0; frame < 2; frame++ {
		tt.Frame()
	}
	want := "| 名称 | 数量 |\n| --- | --- |\n|   |   |\n"
	if a.tabs[0].Text != want {
		t.Fatalf("自动建表: %q, 要 %q", a.tabs[0].Text, want)
	}
	// 建表后仍显示网格，光标直接进入第一个数据格，源码只留在文档中。
	if a.mdActive != 0 || a.mdVisible != "" {
		t.Fatalf("建表后应编辑空数据格: active=%d visible=%q", a.mdActive, a.mdVisible)
	}
	if !tt.Focused("内容") || !tt.HasText("名称") || !tt.HasText("数量") {
		t.Fatal("建表后应保持表头渲染并聚焦单元格")
	}
	tt.Type("中文值")
	if !strings.Contains(a.tabs[0].Text, "|   中文值|   |") {
		t.Fatalf("输入应写入第一格: %q", a.tabs[0].Text)
	}
	blocks := a.mdBlocks()
	if len(blocks) < 2 || blocks[0].kind != bkTable {
		t.Fatalf("建表后应有独立表格块: %v", blocks)
	}
}

// TestCreateTableUndo 退格 / ⌘Z 撤销自动建表，恢复原文并保证可继续输入。
func TestCreateTableUndo(t *testing.T) {
	for _, via := range []string{"backspace", "undo"} {
		t.Run(via, func(t *testing.T) {
			a := newTestApp(t)
			t.Cleanup(a.shutdown)
			tt := ui.NewTester(a.view, 400, 540)
			text := "| a | b |"
			a.tabs[0].Text = text
			n := utf8.RuneCountInString(text)
			a.selSyncStart, a.selSyncEnd = n, n
			tt.Frame()
			tt.Key(0, ui.KeyEnter)
			for frame := 0; frame < 2; frame++ {
				tt.Frame()
			}
			// First data cell remains focused: undo requires no extra click.
			switch via {
			case "backspace":
				tt.Key(0, ui.KeyBackspace)
			case "undo":
				tt.Key(ui.Super, ui.KeyZ)
			}
			for frame := 0; frame < 2; frame++ {
				tt.Frame()
			}
			if a.tabs[0].Text != text {
				t.Fatalf("撤销建表失败: %q, 要 %q", a.tabs[0].Text, text)
			}
			tt.Type("X")
			if want := text + "X"; a.tabs[0].Text != want {
				t.Fatalf("撤销后输入位置错误: %q, 要 %q", a.tabs[0].Text, want)
			}
		})
	}
}

// TestTableTabNavigation Tab 选中下一单元格，末单元格 Tab 增加数据行。
func TestTableTabNavigation(t *testing.T) {
	a := newTestApp(t)
	t.Cleanup(a.shutdown)
	tt := ui.NewTester(a.view, 400, 540)
	a.tabs[0].Text = "| a | b |\n| --- | --- |\n| 1 | 2 |"
	tt.Frame()            // 先同步草稿，再定位
	a.focusBlock(0, 2, 2) // 光标在表头第一个单元格
	tt.Frame()
	tt.Key(0, ui.KeyTab)
	tt.Frame()
	if a.mdCaret != 6 || a.mdSelEnd != 7 {
		t.Fatalf("Tab 应选中表头第 2 格（caret=%d sel=%d）", a.mdCaret, a.mdSelEnd)
	}
	// 数据行最后一个单元格：Tab 加新行并选中其首格。
	a.focusBlock(0, 0, 0)
	tt.Frame()
	cells := tableCellPositions(a.mdDraft)
	last := cells[len(cells)-1]
	a.selSyncStart, a.selSyncEnd = last.start, last.end
	tt.Frame()
	tt.Key(0, ui.KeyTab)
	tt.Frame()
	rows := strings.Split(a.tabs[0].Text, "\n")
	if len(rows) != 4 {
		t.Fatalf("末格 Tab 应新增数据行: %q", a.tabs[0].Text)
	}
	if cells := tableCellPositions(a.mdDraft); len(cells) != 6 {
		t.Fatalf("新行单元格: %d", len(cells))
	}
	if got := tableCellPositions(a.mdDraft); a.mdCaret != got[4].start {
		t.Fatalf("Tab 后应落新行首格: caret=%d\n", a.mdCaret)
	}
}

// TestTableEnterAppendsRow 表格内回车新增当前列宽的数据行；空数据行回车退出表格。
func TestTableEnterAppendsRow(t *testing.T) {
	a := newTestApp(t)
	t.Cleanup(a.shutdown)
	tt := ui.NewTester(a.view, 400, 540)
	a.tabs[0].Text = "| a | b |\n| --- | --- |\n| 1 | 2 |"
	tt.Frame() // 先同步草稿，再定位
	n := utf8.RuneCountInString(a.tabs[0].Text)
	a.focusBlock(0, n-1, n-1) // 末数据行内
	tt.Frame()
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	rows := strings.Split(a.tabs[0].Text, "\n")
	if len(rows) != 4 || rows[3] != "|   |   |" {
		t.Fatalf("表格内回车应新增空数据行: %q", a.tabs[0].Text)
	}
	// 再反向：空数据行末格 Tab 再增一行。
	p := tableCellPositions(a.mdDraft)
	pos := p[len(p)-1].start
	a.selSyncStart, a.selSyncEnd = pos, pos
	tt.Frame()
	tt.Key(0, ui.KeyTab)
	tt.Frame()
	if got := strings.Split(a.tabs[0].Text, "\n"); len(got) != 5 {
		t.Fatalf("空行末格 Tab 应再增行: %q", a.tabs[0].Text)
	}
}

// TestPipeLineNotTableIncomplete 只有表头没有分隔行时，回车仍是普通段落分段。
func TestPipeLineNotTableIncomplete(t *testing.T) {
	a := newTestApp(t)
	t.Cleanup(a.shutdown)
	tt := ui.NewTester(a.view, 400, 540)
	a.tabs[0].Text = "| a | b "
	n := utf8.RuneCountInString(a.tabs[0].Text)
	a.selSyncStart, a.selSyncEnd = n, n
	tt.Frame()
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	if a.tabs[0].Text != "| a | b \n\n" {
		t.Fatalf("非完整表头行应保持普通分段: %q", a.tabs[0].Text)
	}
}
