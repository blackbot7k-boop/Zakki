package app

import "testing"

// 行尾键入的 @ 若属于 ^ 重复规则（如 ^w@1-5 的周几集），是规则字符而不是
// 时间弹层的占位符；普通行尾 @ 仍照常触发（R13）。
func TestTryTimeTriggerSkipsRecurRule(t *testing.T) {
	a := newTestApp(t)
	t.Cleanup(a.shutdown)
	blk := liveBlocks("交房租")[0]

	for _, visible := range []string{"交房租 ^w@", "交房租 @2026-10-26 09:00 ^w@", "^w@"} {
		a.timePop.open = false
		a.mdVisible = visible
		n := len([]rune(visible))
		cs, ce := n, n
		if a.tryTimeTrigger(blk, &cs, &ce) {
			t.Fatalf("%q：规则内的 @ 不应触发弹层", visible)
		}
		if a.mdVisible != visible || cs != n || a.timePop.open {
			t.Fatalf("%q：未触发时不得改写文本或打开弹层: %q cs=%d open=%v", visible, a.mdVisible, cs, a.timePop.open)
		}
	}

	// 继续输入周几集不会触发（@ 已不在行尾）。
	a.mdVisible = "交房租 ^w@1-5"
	cs, ce := len([]rune(a.mdVisible)), len([]rune(a.mdVisible))
	if a.tryTimeTrigger(blk, &cs, &ce) {
		t.Fatal("^w@1-5 不应触发")
	}

	// 普通行尾 @ 照常触发，包括 ^ 出现在更早的片段时。
	for _, visible := range []string{"交房租 @", "2 ^ 3 @", "x^w@"} {
		a.timePop.open = false
		a.mdVisible = visible
		n := len([]rune(visible))
		cs, ce := n, n
		if !a.tryTimeTrigger(blk, &cs, &ce) {
			t.Fatalf("%q：普通行尾 @ 应触发", visible)
		}
	}

	// 行内代码里的 @ 后面还有闭合反引号，不在行尾，不触发。
	a.timePop.open = false
	a.mdVisible = "`a@`"
	cs, ce = 3, 3
	if a.tryTimeTrigger(blk, &cs, &ce) {
		t.Fatal("行内代码内的 @ 不应触发")
	}
}
