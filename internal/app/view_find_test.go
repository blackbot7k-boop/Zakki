package app

import "testing"

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
