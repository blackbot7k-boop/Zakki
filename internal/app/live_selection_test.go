package app

import (
	"testing"
	"unicode/utf8"

	"github.com/egoist/mygo/ui"
)

// 聚焦块的全选必须覆盖整块可见文字，且随后的输入替换整个选区。
func focusFirstBlock(t *testing.T, a *app, tt *ui.Tester, text string) int {
	t.Helper()
	a.tabs[0].Text = text
	n := utf8.RuneCountInString(text)
	a.selSyncStart, a.selSyncEnd = n, n
	tt.Frame()
	return n
}

func TestLiveSelectAllByKey(t *testing.T) {
	const text = "甲乙丙丁"
	a := newTestApp(t)
	t.Cleanup(a.shutdown)
	tt := ui.NewTester(a.view, 400, 540)
	n := focusFirstBlock(t, a, tt, text)
	tt.Key(ui.Cmd, ui.KeyA)
	for range 3 {
		tt.Frame()
	}
	if !a.selDoc || a.selDocStart != 0 || a.selDocEnd != n {
		t.Fatalf("Cmd+A 后文档选区=%v %d..%d, want 0..%d", a.selDoc, a.selDocStart, a.selDocEnd, n)
	}
	tt.Type("X")
	if got, want := a.tabs[0].Text, "X"; got != want {
		t.Fatalf("全选后输入未替换整块: doc=%q, want %q", got, want)
	}
}

func TestLiveSelectAllByMenuCommand(t *testing.T) {
	const text = "甲乙丙丁"
	a := newTestApp(t)
	t.Cleanup(a.shutdown)
	tt := ui.NewTester(a.view, 400, 540)
	n := focusFirstBlock(t, a, tt, text)
	tt.Command("selectAll")
	for range 3 {
		tt.Frame()
	}
	if a.mdSelEnd-a.mdCaret != n {
		t.Fatalf("菜单全选后文档选区=%d，want %d", a.mdSelEnd-a.mdCaret, n)
	}
	tt.Type("X")
	if got, want := a.tabs[0].Text, "X"; got != want {
		t.Fatalf("菜单全选后输入未替换整块: doc=%q, want %q", got, want)
	}
}

func TestLiveSelectAllByMouseDrag(t *testing.T) {
	const text = "甲乙丙丁"
	a := newTestApp(t)
	t.Cleanup(a.shutdown)
	tt := ui.NewTester(a.view, 400, 540)
	n := focusFirstBlock(t, a, tt, text)
	rect, ok := tt.Find("内容")
	if !ok {
		t.Fatal("找不到聚焦块输入框")
	}
	tt.Press(rect.X+2, rect.Y+rect.H/2)
	tt.Move(rect.X+rect.W-2, rect.Y+rect.H/2)
	tt.Release(rect.X+rect.W-2, rect.Y+rect.H/2)
	for range 3 {
		tt.Frame()
	}
	if a.mdSelEnd-a.mdCaret != n {
		t.Fatalf("拖选后文档选区=%d，want %d (caret=%d selEnd=%d)", a.mdSelEnd-a.mdCaret, n, a.mdCaret, a.mdSelEnd)
	}
	tt.Type("X")
	if got, want := a.tabs[0].Text, "X"; got != want {
		t.Fatalf("拖选后输入未替换整块: doc=%q, want %q", got, want)
	}
}

// 文档级全选：Cmd+A 覆盖所有块；后续输入/删除/复制按文档区间语义执行。
func TestLiveSelectAllDocument(t *testing.T) {
	const text = "甲\n\n乙\n\n丙"
	a := newTestApp(t)
	t.Cleanup(a.shutdown)
	tt := ui.NewTester(a.view, 400, 540)
	a.tabs[0].Text = text
	tt.Frame()
	a.focusBlock(1, 1, 1)
	tt.Frame()
	tt.Key(ui.Cmd, ui.KeyA)
	for range 3 {
		tt.Frame()
	}
	if !a.selDoc {
		t.Fatal("Cmd+A 未进入文档级全选")
	}
	if a.selDocStart != 0 || a.selDocEnd != utf8.RuneCountInString(text) {
		t.Fatalf("文档选区=%d..%d, want 0..%d", a.selDocStart, a.selDocEnd, utf8.RuneCountInString(text))
	}
	// 输入替换整个文档
	tt.Type("X")
	if got, want := a.tabs[0].Text, "X"; got != want {
		t.Fatalf("全选后输入未替换整个文档: %q", got)
	}
	if a.selDoc {
		t.Fatal("替换后应退出文档级选区")
	}
}

func TestLiveSelectAllDocumentCopyAndCut(t *testing.T) {
	const text = "甲\n\n乙"
	a := newTestApp(t)
	t.Cleanup(a.shutdown)
	tt := ui.NewTester(a.view, 400, 540)
	a.tabs[0].Text = text
	tt.Frame()
	a.focusBlock(0, 0, 0)
	tt.Frame()
	// 菜单「全选」入口（真实菜单由 Run 注册，测试直接调 handler）。
	a.menuSelectAll()
	tt.Frame()
	if !a.selDoc {
		t.Fatal("菜单全选未进入文档级选区")
	}
	tt.Key(ui.Cmd, ui.KeyC)
	tt.Frame()
	if got := tt.Clipboard(); got != text {
		t.Fatalf("全选复制=%q, want %q", got, text)
	}
	tt.Key(ui.Cmd, ui.KeyX)
	tt.Frame()
	if got := a.tabs[0].Text; got != "" {
		t.Fatalf("全选剪切后文档=%q, want 空", got)
	}
	if got := tt.Clipboard(); got != text {
		t.Fatalf("剪切后剪贴板=%q, want %q", got, text)
	}
}

func TestLiveSelectAllDocumentDeleteAndCollapse(t *testing.T) {
	const text = "甲\n\n乙\n\n丙"
	a := newTestApp(t)
	t.Cleanup(a.shutdown)
	tt := ui.NewTester(a.view, 400, 540)
	a.tabs[0].Text = text
	tt.Frame()
	a.focusBlock(2, 0, 0)
	for range 2 {
		tt.Frame()
	}
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Frame()
	// 方向键收缩到选区末尾（末块丙内，文档偏移 7 = 块内 1）
	tt.Key(0, ui.KeyRight)
	for range 3 {
		tt.Frame()
	}
	if a.selDoc {
		t.Fatal("方向键后应退出文档级选区")
	}
	last := len(a.mdBlocks()) - 1
	if a.mdActive != last || a.mdCaret != a.mdSelEnd {
		t.Fatalf("收缩后应在末块末尾: active=%d caret=%d..%d", a.mdActive, a.mdCaret, a.mdSelEnd)
	}
	// 重新全选后 Backspace 清空文档，且可撤销
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Frame()
	tt.Key(0, ui.KeyBackspace)
	tt.Frame()
	if got := a.tabs[0].Text; got != "" {
		t.Fatalf("全选删除后文档=%q, want 空", got)
	}
	a.undoLive(false)
	if got, want := a.tabs[0].Text, text; got != want {
		t.Fatalf("撤销恢复=%q, want %q", got, want)
	}
}

func TestLiveSelectAllEscapeCollapsesWithoutHiding(t *testing.T) {
	const text = "甲\n\n乙"
	a := newTestApp(t)
	t.Cleanup(a.shutdown)
	tt := ui.NewTester(a.view, 400, 540)
	a.tabs[0].Text = text
	a.focusBlock(0, 0, 0)
	tt.Frame()
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Frame()
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if a.selDoc {
		t.Fatal("Esc 应退出文档级选区")
	}
	if a.mdCaret != 0 || a.mdSelEnd != 0 {
		t.Fatalf("Esc 后光标应收缩到选区起点: %d..%d", a.mdCaret, a.mdSelEnd)
	}
}

// 表格单元格内 Cmd+A 保持单元格原生全选，不进入文档级全选。
func TestLiveSelectAllInsideTableCellStaysLocal(t *testing.T) {
	const text = "|甲|乙|\n| --- | --- |\n|  1|  2|\n"
	a := newTestApp(t)
	t.Cleanup(a.shutdown)
	tt := ui.NewTester(a.view, 400, 540)
	a.tabs[0].Text = text
	tt.Frame()
	a.focusBlock(0, 1, 1)
	for range 2 {
		tt.Frame()
	}
	// 进入第一行第一列单元格
	a.focusTableCell(0, 0, 0)
	for range 3 {
		tt.Frame()
	}
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Frame()
	if a.selDoc {
		t.Fatal("单元格内 Cmd+A 不应进入文档级全选")
	}
}
