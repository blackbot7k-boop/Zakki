package main

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

// 格式命令集成：实时预览与源码模式共用命令入口；每次命令一个撤销事务；
// 无选区切换待输入格式；切点不误改其他笔记。

func TestExecHeadingLiveMode(t *testing.T) {
	a := newTestApp(t)
	a.winOpen = true
	a.tabs[0].Text = "标题内容\n\n下一段"
	a.resyncLive(0)
	a.focusBlock(0, 2, 2)
	a.execFormatCommand(cmdH2)
	if a.tabs[0].Text != "## 标题内容\n\n下一段" {
		t.Fatalf("H2 应用到光标所在行: %q", a.tabs[0].Text)
	}
	// 一次撤销恢复文本与选区。
	a.undoLive(false)
	if a.tabs[0].Text != "标题内容\n\n下一段" {
		t.Fatalf("撤销应恢复文本: %q", a.tabs[0].Text)
	}
	if a.mdActive != 0 || a.selSyncStart != 2 {
		t.Fatalf("撤销应恢复焦点块与选区: %d %d", a.mdActive, a.selSyncStart)
	}
	// 同级标题再次执行还原正文。
	a.execFormatCommand(cmdH2)
	a.execFormatCommand(cmdH2)
	if a.tabs[0].Text != "标题内容\n\n下一段" {
		t.Fatalf("同级标题 toggle: %q", a.tabs[0].Text)
	}
}

func TestExecInlineWithSelectionLive(t *testing.T) {
	a := newTestApp(t)
	a.winOpen = true
	a.tabs[0].Text = "加粗测试"
	a.resyncLive(0)
	a.focusBlock(0, 0, runeLen("加粗"))
	a.execFormatCommand(cmdBold)
	if a.tabs[0].Text != "**加粗**测试" {
		t.Fatalf("选区加粗: %q", a.tabs[0].Text)
	}
	if a.selSyncStart != 2 || a.selSyncEnd != 2+runeLen("加粗") {
		t.Fatalf("选区包住内容: %d..%d", a.selSyncStart, a.selSyncEnd)
	}
	a.undoLive(false)
	if a.tabs[0].Text != "加粗测试" {
		t.Fatalf("一次撤销还原: %q", a.tabs[0].Text)
	}
}

func TestPendingInlineFormatToggleAndApply(t *testing.T) {
	a := newTestApp(t)
	a.winOpen = true
	a.tabs[0].Text = "前文"
	a.resyncLive(0)
	a.focusBlock(0, runeLen("前文"), runeLen("前文"))
	a.mdCaret, a.mdSelEnd = runeLen("前文"), runeLen("前文") // 发布后的光标
	// 无选区 ⌘B：只切换待输入状态，不写空标记。
	a.execFormatCommand(cmdBold)
	if a.tabs[0].Text != "前文" {
		t.Fatalf("待输入状态不得提前写标记: %q", a.tabs[0].Text)
	}
	if !a.mdPending[spanBold] {
		t.Fatal("应记录待输入意图")
	}
	// 第一次输入生成有效 Markdown（走 applyPendingWrap 纯函数路径验证）。
	p := projectBlock(a.mdBlocks()[0])
	pre, post := a.pendingWrapFor(p, runeLen("前文"), runeLen("前文"), a.mdBlocks()[0], runeLen("前文")+2)
	if pre != "**" || post != "**" {
		t.Fatalf("待输入格式应产出包裹标记: %q %q", pre, post)
	}
	raw, caret := applyMarkerWrap("前文新字", runeLen("前文"), runeLen("前文新字"), pre, post)
	if raw != "前文**新字**" || caret != runeLen("前文**新字**") {
		t.Fatalf("包裹插入: %q %d", raw, caret)
	}
	// 光标移动后重新计算（意图清零）。
	a.mdCaret = 0
	a.maybeExpirePending()
	if len(a.mdPending) != 0 {
		t.Fatalf("光标移动后意图应清零: %+v", a.mdPending)
	}
}

func TestPendingInlineToggleOffOpenFormat(t *testing.T) {
	a := newTestApp(t)
	a.winOpen = true
	a.tabs[0].Text = "**已打开"
	a.resyncLive(0)
	a.focusBlock(0, runeLen("**已打开"), runeLen("**已打开"))
	a.mdCaret, a.mdSelEnd = runeLen("**已打开"), runeLen("**已打开") // 发布后的光标
	// 当前位置 bold 已打开：⌘B 关闭后续输入格式。
	a.execFormatCommand(cmdBold)
	p := projectBlock(a.mdBlocks()[0])
	pre, post := a.pendingWrapFor(p, runeLen("**已打开"), runeLen("**已打开"), a.mdBlocks()[0], runeLen("**已打开")+2)
	if pre != "**" || post != "" {
		t.Fatalf("关闭已打开格式应先补闭合标记: %q %q", pre, post)
	}
	raw, _ := applyMarkerWrap("**已打开新", runeLen("**已打开"), runeLen("**已打开新"), pre, post)
	if raw != "**已打开**新" {
		t.Fatalf("闭合后输入落在格式外: %q", raw)
	}
}

func TestExecCodeBlockInCodeDisabled(t *testing.T) {
	a := newTestApp(t)
	a.winOpen = true
	a.tabs[0].Text = "```\ncode\n```"
	a.resyncLive(0)
	a.focusBlock(0, 4, 4)
	before := a.tabs[0].Text
	a.execFormatCommand(cmdBold) // 代码块内禁用行内格式
	if a.tabs[0].Text != before {
		t.Fatalf("代码块内行内格式应禁用: %q", a.tabs[0].Text)
	}
	a.execFormatCommand(cmdH1) // 标题也禁用
	if a.tabs[0].Text != before {
		t.Fatalf("代码块内标题应禁用: %q", a.tabs[0].Text)
	}
	// 正文/代码块切换除外。
	a.execFormatCommand(cmdCodeBlock)
	if strings.Contains(a.tabs[0].Text, "```") {
		t.Fatalf("代码块内执行代码块命令应还原正文: %q", a.tabs[0].Text)
	}
	a.undoLive(false)
	if a.tabs[0].Text != before {
		t.Fatalf("撤销还原代码块: %q", a.tabs[0].Text)
	}
}

func TestExecInsertRuleAndTable(t *testing.T) {
	a := newTestApp(t)
	a.winOpen = true
	a.tabs[0].Text = "第一段"
	a.resyncLive(0)
	a.focusBlock(0, 0, 0)
	a.execFormatCommand(cmdRule)
	if a.tabs[0].Text != "第一段\n\n---" {
		t.Fatalf("分隔线插入当前块之后: %q", a.tabs[0].Text)
	}
	a.execFormatCommand(cmdTable)
	if !strings.Contains(a.tabs[0].Text, "| 表头 1 | 表头 2 |") {
		t.Fatalf("表格插入: %q", a.tabs[0].Text)
	}
	var sawRule, sawTable bool
	for _, blk := range a.mdBlocks() {
		if blk.kind == bkRule {
			sawRule = true
		}
		if blk.kind == bkTable {
			sawTable = true
		}
	}
	if !sawRule || !sawTable {
		t.Fatalf("分隔线与表格都应成为独立块: %q", a.tabs[0].Text)
	}
}

func TestExecCommandSourceMode(t *testing.T) {
	a := newTestApp(t)
	a.winOpen = true
	a.tabs[0].Text = "# 标题\n正文第一行\n正文第二行"
	a.cfg.Source = true
	a.srcPrev = a.liveSnapshot()
	// 选区覆盖两行正文（rune 偏移）。
	s := runeLen("# 标题\n")
	a.srcCaret, a.srcSelEnd = s, s+runeLen("正文第一行\n正文第二行")
	a.execFormatCommand(cmdH1) // 标题命令作用于选区覆盖的行（1..2 行）
	if a.tabs[0].Text != "# 标题\n# 正文第一行\n# 正文第二行" {
		t.Fatalf("源码选区标题: %q", a.tabs[0].Text)
	}
	a.undoLive(false)
	if a.tabs[0].Text != "# 标题\n正文第一行\n正文第二行" {
		t.Fatalf("撤销: %q", a.tabs[0].Text)
	}
	if a.srcSelSyncStart != s || a.srcSelSyncEnd != s+runeLen("正文第一行\n正文第二行") {
		t.Fatalf("撤销应恢复源码选区: %d..%d", a.srcSelSyncStart, a.srcSelSyncEnd)
	}
	// 行内格式跨行。
	a.execFormatCommand(cmdBold)
	if !strings.Contains(a.tabs[0].Text, "**正文第一行**") {
		t.Fatalf("源码行内跨行: %q", a.tabs[0].Text)
	}
}

func TestSwitchDotDoesNotTouchOtherNotes(t *testing.T) {
	a := newTestApp(t)
	a.winOpen = true
	a.tabs[0].Text = "点一内容"
	a.tabs[1].Text = "点二内容"
	a.resyncLive(0)
	a.focusBlock(0, 0, 0)
	a.execFormatCommand(cmdBold) // 无选区 → 待输入状态
	a.switchTab(1)
	if a.tabs[0].Text != "点一内容" {
		t.Fatalf("切点不得改写原点: %q", a.tabs[0].Text)
	}
	if a.tabs[1].Text != "点二内容" {
		t.Fatalf("切点不得改写新点: %q", a.tabs[1].Text)
	}
	if len(a.mdPending) != 0 {
		t.Fatal("切点应清零待输入意图")
	}
}

func TestContinuousTypingMergesUndo(t *testing.T) {
	a := newTestApp(t)
	a.winOpen = true
	a.tabs[0].Text = ""
	a.resyncLive(0)
	a.focusBlock(0, 0, 0)
	// 模拟连续输入：文档已变后记 editInput 事务。
	before := a.liveSnapshot()
	a.tabs[0].Text = "你"
	a.rememberLiveEdit(before, editInput)
	before = a.liveSnapshot()
	a.tabs[0].Text = "你好"
	a.rememberLiveEdit(before, editInput)
	before = a.liveSnapshot()
	a.tabs[0].Text = "你好世"
	a.rememberLiveEdit(before, editInput)
	if len(a.liveUndo) != 1 {
		t.Fatalf("连续输入应合并为一个事务: %d", len(a.liveUndo))
	}
	// 粘贴是独立事务。
	before = a.liveSnapshot()
	a.tabs[0].Text = "你好世界，粘贴内容"
	a.rememberLiveEdit(before, editPaste)
	if len(a.liveUndo) != 2 {
		t.Fatalf("粘贴应独立成事务: %d", len(a.liveUndo))
	}
	// 撤销：先撤粘贴，再撤整个输入串。
	a.undoLive(false)
	if a.tabs[0].Text != "你好世" {
		t.Fatalf("撤销粘贴: %q", a.tabs[0].Text)
	}
	a.undoLive(false)
	if a.tabs[0].Text != "" {
		t.Fatalf("撤销连续输入一步到位: %q", a.tabs[0].Text)
	}
	a.undoLive(true)
	if a.tabs[0].Text != "你好世" {
		t.Fatalf("重做: %q", a.tabs[0].Text)
	}
}

func TestCommandRunsOncePerKey(t *testing.T) {
	a := newTestApp(t)
	a.winOpen = true
	a.tabs[0].Text = "文本"
	a.resyncLive(0)
	a.focusBlock(0, 0, runeLen("文本"))
	a.execFormatCommand(cmdBold)
	a.execFormatCommand(cmdBold) // 第二次：选区已是 bold span → 移除
	if a.tabs[0].Text != "文本" {
		t.Fatalf("同一选区两次执行应切换回来: %q", a.tabs[0].Text)
	}
}

func TestLinkToolApply(t *testing.T) {
	a := newTestApp(t)
	a.winOpen = true
	a.tabs[0].Text = "看看这个链接"
	a.resyncLive(0)
	sel := "这个链接"
	a.focusBlock(0, runeLen("看看"), runeLen("看看")+runeLen(sel))
	a.openLinkEditor()
	if !a.linkTool.open || a.linkTool.text != sel {
		t.Fatalf("浮层应预填选区文字: %+v", a.linkTool)
	}
	a.linkTool.url = "https://example.com"
	a.applyLinkTool()
	if a.tabs[0].Text != "看看[这个链接](https://example.com)" {
		t.Fatalf("确认生成链接: %q", a.tabs[0].Text)
	}
	// 编辑已有链接：填入当前值。
	a.openLinkEditor()
	if a.linkTool.text != "这个链接" || a.linkTool.url != "https://example.com" {
		t.Fatalf("已有链接应填入当前值: %+v", a.linkTool)
	}
	a.linkTool.url = "https://tot.rocks"
	a.applyLinkTool()
	if a.tabs[0].Text != "看看[这个链接](https://tot.rocks)" {
		t.Fatalf("编辑链接: %q", a.tabs[0].Text)
	}
	// 无选区且无文字：要求填写，不读剪贴板。
	a.focusBlock(0, runeLen("看看[这个链接](https://tot.rocks)"), runeLen("看看[这个链接](https://tot.rocks)"))
	a.openLinkEditor()
	a.linkTool.text = ""
	a.applyLinkTool()
	if !a.linkTool.open || a.linkTool.err == "" {
		t.Fatal("缺链接文字应提示且浮层保持打开")
	}
	a.closeLinkTool()
}

func TestFormatCommandWindowFocusGuard(t *testing.T) {
	a := newTestApp(t)
	a.winOpen = true
	a.tabs[0].Text = "文本"
	a.resyncLive(0)
	a.focusBlock(0, 0, 1)
	// 无焦点窗口（菜单来源）或焦点在主窗口：执行。
	a.execFormatCommand(cmdBold)
	if a.tabs[0].Text != "**文**本" {
		t.Fatalf("命令应执行: %q", a.tabs[0].Text)
	}
}

func TestRecordSourceKeyUndoRouting(t *testing.T) {
	a := newTestApp(t)
	a.winOpen = true
	a.tabs[0].Text = "源码"
	a.cfg.Source = true
	a.srcPrev = a.liveSnapshot()
	if a.recordSourceKey(ui.InputEvent{Kind: ui.InputKeyDown, Key: ui.KeyZ, Mods: ui.Super}) != true {
		t.Fatal("⌘Z 应被拦截路由到文档级撤销")
	}
	if a.liveUndoPending != 1 {
		t.Fatal("应记撤销待执行")
	}
	if a.recordSourceKey(ui.InputEvent{Kind: ui.InputKeyDown, Key: ui.KeyZ, Mods: ui.Super | ui.Shift}) != true {
		t.Fatal("⌘⇧Z 应被拦截")
	}
	if a.liveUndoPending != -1 {
		t.Fatal("应记重做待执行")
	}
	if a.recordSourceKey(ui.InputEvent{Kind: ui.InputCommand, Text: "paste"}) != false {
		t.Fatal("粘贴应交给原生处理，仅记录事务类型")
	}
	if !a.nextEditIsPaste {
		t.Fatal("粘贴应标记独立事务")
	}
}
