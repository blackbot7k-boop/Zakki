package app

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

// 键入 ``` 新建代码块：提交出单行围栏时闭合成对围栏（与空块上执行
// 「代码块」命令一致），退化未闭合围栏的键入重定向到内容行。

func typeInto(t *testing.T, a *app, doc string, focus func(), typed ...string) {
	t.Helper()
	a.winOpen = true
	a.tabs[0].Text = doc
	a.resyncLive(0)
	tt := ui.NewTester(a.view, 400, 540)
	tt.Frame()
	focus()
	tt.Frame()
	for _, s := range typed {
		tt.Type(s)
		tt.Frame()
	}
}

// 空笔记键入 ``` 后直接输入：内容必须落在围栏之间，不得拼到围栏行上。
func TestTypedFenceConvertsToCodeBlock(t *testing.T) {
	a := newTestApp(t)
	typeInto(t, a, "", func() { a.focusBlockEnd(0) }, "```", "hello")
	if a.tabs[0].Text != "```\nhello\n```" {
		t.Fatalf("键入 ``` 与内容后应得到闭合代码块: %q", a.tabs[0].Text)
	}
	if a.mdBlocks()[a.mdActive].kind != bkCode {
		t.Fatalf("焦点块应为代码块: %v", a.mdBlocks()[a.mdActive].kind)
	}
	if a.mdVisible != "hello" {
		t.Fatalf("可见文本应为代码内容: %q", a.mdVisible)
	}
}

// 中文档空行键入 ```：闭合成对围栏，后续段落保持正文不被吞进代码块。
// 空行本身就是被转换的块，转换后仅剩前一个分隔空行。
func TestTypedFenceMidDocKeepsFollowingText(t *testing.T) {
	a := newTestApp(t)
	typeInto(t, a, "第一段\n\n\n第二段", func() { a.focusBlock(2, 0, 0) }, "```", "go")
	want := "第一段\n\n```\ngo\n```\n第二段"
	if a.tabs[0].Text != want {
		t.Fatalf("后续段落应保持正文: %q", a.tabs[0].Text)
	}
}

// 带语言标识的键入（```go）：闭合时保留信息串。
func TestTypedFenceWithLanguage(t *testing.T) {
	a := newTestApp(t)
	typeInto(t, a, "", func() { a.focusBlockEnd(0) }, "```go", "x=1")
	if a.tabs[0].Text != "```go\nx=1\n```" {
		t.Fatalf("语言标识应保留: %q", a.tabs[0].Text)
	}
}

// 键入 ``` 的转换与后续输入分属两个撤销事务：撤「go」、再撤转换。
func TestTypedFenceUndoRestoresDoc(t *testing.T) {
	a := newTestApp(t)
	typeInto(t, a, "第一段\n\n\n第二段", func() { a.focusBlock(2, 0, 0) }, "```", "go")
	a.undoLive(false)
	if a.tabs[0].Text != "第一段\n\n```\n\n```\n第二段" {
		t.Fatalf("第一次撤销应只撤内容输入: %q", a.tabs[0].Text)
	}
	a.undoLive(false)
	if a.tabs[0].Text != "第一段\n\n\n第二段" {
		t.Fatalf("第二次撤销应还原整个转换: %q", a.tabs[0].Text)
	}
}

// 存量的孤立未闭合围栏（如历史文档残留）：点击后键入应生成内容行，
// 而不是改写围栏行。
func TestTypingIntoStrayFenceCreatesContent(t *testing.T) {
	a := newTestApp(t)
	typeInto(t, a, "前文\n\n```", func() { a.focusBlock(2, 3, 3) }, "x")
	if a.tabs[0].Text != "前文\n\n```\nx" {
		t.Fatalf("键入应落到围栏后的内容行: %q", a.tabs[0].Text)
	}
	if !strings.HasPrefix(a.mdBlocks()[a.mdActive].raw, "```\n") {
		t.Fatalf("围栏行不得被改写: %q", a.mdBlocks()[a.mdActive].raw)
	}
}

// 代码块内容里键入 ``` 是正文（记代码示例），不触发新建转换。
func TestTypedFenceInsideCodeBlockStaysLiteral(t *testing.T) {
	a := newTestApp(t)
	typeInto(t, a, "```\nabc\n```", func() {
		a.focusBlock(0, 4, 7) // 选中内容 abc
	}, "```")
	if a.tabs[0].Text != "```\n```\n```" {
		t.Fatalf("代码块内键入 ``` 应作为内容原样替换: %q", a.tabs[0].Text)
	}
}
