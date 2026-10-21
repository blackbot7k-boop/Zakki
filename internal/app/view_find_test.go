package app

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// 菜单「查找下一个/上一个」与查找条按钮、Enter 共用 findStep：
// 在命中集合里循环跳匹配；无命中不动；查找条未开时菜单入口先开条。
func TestFindStepCyclesAndOpenOrStep(t *testing.T) {
	a := newTestApp(t)
	a.winOpen = true
	a.tabs[0].Text = "甲 one\n甲 two\n甲 three"
	a.query = "甲"
	a.findOpen = true
	a.matchIdx = 0

	a.findStep(1)
	if a.matchIdx != 1 || !a.jumpPending {
		t.Fatalf("findStep(1) 应跳下一个命中: idx=%d pending=%v", a.matchIdx, a.jumpPending)
	}
	a.findStep(1)
	a.findStep(1) // 2 → 循环回 0
	if a.matchIdx != 0 {
		t.Fatalf("应循环回首个命中: %d", a.matchIdx)
	}
	a.findStep(-1) // 0 → 循环到末尾
	if a.matchIdx != 2 {
		t.Fatalf("findStep(-1) 应循环到末尾命中: %d", a.matchIdx)
	}
	// 无命中：不动索引、不产生跳转。
	a.query = "不存在"
	a.jumpPending = false
	a.findStep(1)
	if a.matchIdx != 2 || a.jumpPending {
		t.Fatalf("无命中应无操作: idx=%d pending=%v", a.matchIdx, a.jumpPending)
	}
	// 菜单入口在查找条关闭时先开条并重置查询。
	a.findOpen = false
	a.findOpenOrStep(1)
	if !a.findOpen || a.query != "" || a.matchIdx != 0 {
		t.Fatalf("查找条未开时应先开条: open=%v query=%q idx=%d", a.findOpen, a.query, a.matchIdx)
	}
}

// TestFindBarShiftEnterStepsBack 查找框里 Enter 跳下一个、Shift+Enter 跳上一个；
// 关闭查找条后高亮查询清空，残留的查询词不再着色正文。
func TestFindBarShiftEnterStepsBack(t *testing.T) {
	a := newTestApp(t)
	t.Cleanup(a.shutdown)
	a.winOpen = true
	a.tabs[0].Text = "甲 one\n甲 two\n甲 three"
	tt := ui.NewTester(a.view, 500, 600)
	a.runFixedShortcut("find")
	tt.Frame()
	tt.Type("甲")
	tt.Frame()
	if a.highlightQuery() != "甲" {
		t.Fatalf("查找条打开时应高亮查询: %q", a.highlightQuery())
	}
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	if a.matchIdx != 1 {
		t.Fatalf("Enter 应跳到下一个命中: %d", a.matchIdx)
	}
	tt.Key(ui.Shift, ui.KeyEnter)
	tt.Frame()
	if a.matchIdx != 0 {
		t.Fatalf("Shift+Enter 应跳回上一个命中: %d", a.matchIdx)
	}
	if err := tt.Click("关闭查找"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.findOpen || a.highlightQuery() != "" {
		t.Fatalf("关闭后不应再高亮: open=%v query=%q", a.findOpen, a.highlightQuery())
	}
}
