package app

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// 模拟用户完整操作流：多块输入 → 全选替换 → 撤销 → 重做。
func TestUndoFlowAfterSelectAll(t *testing.T) {
	a := newTestApp(t)
	t.Cleanup(a.shutdown)
	a.tabs[0].Text = ""
	tt := ui.NewTester(a.view, 400, 540)
	tt.Frame()
	a.focusBlock(0, 0, 0)
	tt.Frame()

	tt.Type("甲乙")
	tt.Frame()
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	tt.Type("丙丁")
	tt.Frame()
	if got, want := a.tabs[0].Text, "甲乙\n\n丙丁"; got != want {
		t.Fatalf("输入阶段 doc=%q, want %q", got, want)
	}

	tt.Key(ui.Cmd, ui.KeyA)
	for range 2 {
		tt.Frame()
	}
	if !a.selDoc {
		t.Fatal("Cmd+A 未进入文档级全选")
	}
	tt.Type("全")
	tt.Frame()
	if got, want := a.tabs[0].Text, "全"; got != want {
		t.Fatalf("全选替换 doc=%q, want %q", got, want)
	}

	tt.Key(ui.Cmd, ui.KeyZ)
	tt.Frame()
	if got, want := a.tabs[0].Text, "甲乙\n\n丙丁"; got != want {
		t.Fatalf("撤销 doc=%q, want %q", got, want)
	}

	tt.Key(ui.Cmd|ui.Shift, ui.KeyZ)
	tt.Frame()
	if got, want := a.tabs[0].Text, "全"; got != want {
		t.Fatalf("重做 doc=%q, want %q", got, want)
	}
}

// 纯输入的多步撤销：一步一个字符块，撤销应逐块回退。
func TestUndoPlainTypingSteps(t *testing.T) {
	a := newTestApp(t)
	t.Cleanup(a.shutdown)
	a.tabs[0].Text = ""
	tt := ui.NewTester(a.view, 400, 540)
	tt.Frame()
	a.focusBlock(0, 0, 0)
	tt.Frame()

	tt.Type("甲")
	tt.Frame()
	tt.Type("乙")
	tt.Frame()
	if got := a.tabs[0].Text; got != "甲乙" {
		t.Fatalf("输入 doc=%q", got)
	}
	tt.Key(ui.Cmd, ui.KeyZ)
	tt.Frame()
	if got := a.tabs[0].Text; got != "甲" {
		t.Fatalf("一次撤销 doc=%q, want 甲", got)
	}
	tt.Key(ui.Cmd, ui.KeyZ)
	tt.Frame()
	if got := a.tabs[0].Text; got != "" {
		t.Fatalf("两次撤销 doc=%q, want 空", got)
	}
}
