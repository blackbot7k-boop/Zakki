package app

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

// 键入围栏后按 Enter 新建代码块；退化未闭合围栏的键入重定向到内容行。

// AX 整值替换不推进原生光标：纯追加而光标仍停在前缀内时必须吸附到末尾，
// 否则后续字符会插进文本中间（复现“```+g → ```g`”的退化路径）。
func TestNativeCaretAfterEdit(t *testing.T) {
	cases := []struct {
		name        string
		old, neu    string
		caret, want int
	}{
		{"尾部追加但光标未推进吸附到末尾", "```", "````", 3, 4},
		{"空块首字符且光标未推进", "", "g", 0, 1},
		{"追加且光标已推进保持原位", "```", "````", 4, 4},
		{"中部插入不是前缀不吸附", "abc", "aXbc", 2, 2},
		{"替换首字符不是前缀不吸附", "abc", "Abc", 1, 1},
		{"文本未变不吸附", "abc", "abc", 1, 1},
		{"删除变短不吸附", "abc", "ab", 2, 2},
		{"多字节追加按字节前缀判定", "代码", "代码块", 2, 3},
		{"光标越界交给 clamp 原样返回", "abc", "abcd", 7, 7},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := nativeCaretAfterEdit(tc.old, tc.neu, tc.caret); got != tc.want {
				t.Fatalf("nativeCaretAfterEdit(%q, %q, %d) = %d, 期望 %d", tc.old, tc.neu, tc.caret, got, tc.want)
			}
		})
	}
}

func typeInto(t *testing.T, a *app, doc string, focus func(), typed ...string) *ui.Tester {
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
	return tt
}

// 空笔记逐键键入 ```，按 Enter 后内容必须落在围栏之间。
func TestTypedFenceConvertsToCodeBlock(t *testing.T) {
	a := newTestApp(t)
	tt := typeInto(t, a, "", func() { a.focusBlockEnd(0) }, "`", "`", "`")
	if got := a.tabs[0].Text; got != "```" {
		t.Fatalf("按 Enter 前围栏必须保持可编辑: %q", got)
	}
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	tt.Type("hello")
	tt.Frame()
	if a.tabs[0].Text != "```\nhello\n```" {
		t.Fatalf("键入 ```、Enter 与内容后应得到闭合代码块: %q", a.tabs[0].Text)
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
	tt := typeInto(t, a, "第一段\n\n\n第二段", func() { a.focusBlock(2, 0, 0) }, "`", "`", "`")
	if got := a.tabs[0].Text; got != "第一段\n\n```\n第二段" {
		t.Fatalf("按 Enter 前不得提前闭合围栏: %q", got)
	}
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	tt.Type("go")
	tt.Frame()
	want := "第一段\n\n```\ngo\n```\n第二段"
	if a.tabs[0].Text != want {
		t.Fatalf("后续段落应保持正文: %q", a.tabs[0].Text)
	}
}

// 带语言标识的键入（```go）：闭合时保留信息串。
func TestTypedFenceWithLanguage(t *testing.T) {
	a := newTestApp(t)
	tt := typeInto(t, a, "", func() { a.focusBlockEnd(0) }, "`", "`", "`", "g", "o")
	if got := a.tabs[0].Text; got != "```go" {
		t.Fatalf("语言标识必须在 Enter 前保持在开栏: %q", got)
	}
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	tt.Type("x=1")
	tt.Frame()
	if a.tabs[0].Text != "```go\nx=1\n```" {
		t.Fatalf("语言标识应保留: %q", a.tabs[0].Text)
	}
}

func TestTypedVariableFenceUsesMatchingCloser(t *testing.T) {
	tests := []struct {
		opening string
		want    string
	}{
		{opening: "````go", want: "````go\nx\n````"},
		{opening: "~~~~go", want: "~~~~go\nx\n~~~~"},
	}
	for _, tc := range tests {
		t.Run(tc.opening, func(t *testing.T) {
			a := newTestApp(t)
			tt := typeInto(t, a, "", func() { a.focusBlockEnd(0) })
			for _, r := range tc.opening {
				tt.Type(string(r))
				tt.Frame()
			}
			if got := a.tabs[0].Text; got != tc.opening {
				t.Fatalf("逐键开栏在 Enter 前被改写: %q", got)
			}
			tt.Key(0, ui.KeyEnter)
			tt.Frame()
			tt.Type("x")
			tt.Frame()
			if a.tabs[0].Text != tc.want {
				t.Fatalf("闭栏必须匹配开栏: got %q want %q", a.tabs[0].Text, tc.want)
			}
		})
	}
}

// 键入 ``` 的转换与后续输入分属两个撤销事务：撤「go」、再撤转换。
func TestTypedFenceUndoRestoresDoc(t *testing.T) {
	a := newTestApp(t)
	tt := typeInto(t, a, "第一段\n\n\n第二段", func() { a.focusBlock(2, 0, 0) }, "`", "`", "`")
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	tt.Type("go")
	tt.Frame()
	a.undoLive(false)
	if a.tabs[0].Text != "第一段\n\n```\n\n```\n第二段" {
		t.Fatalf("第一次撤销应只撤内容输入: %q", a.tabs[0].Text)
	}
	a.undoLive(false)
	if a.tabs[0].Text != "第一段\n\n\n第二段" {
		t.Fatalf("第二次撤销应还原整个转换: %q", a.tabs[0].Text)
	}
}

func TestFenceDraftFocusRemapsAfterParagraphMerge(t *testing.T) {
	a := newTestApp(t)
	typeInto(t, a, "\nnext", func() { a.focusBlock(0, 0, 0) }, "`")
	if !a.fenceDraft || len(a.mdBlocks()) != 2 {
		t.Fatalf("逐键围栏草稿应暂时隔离当前行: draft=%v blocks=%+v", a.fenceDraft, a.mdBlocks())
	}
	a.focusBlock(1, 0, 0)
	if a.fenceDraft || a.mdActive != 0 || a.mdDraft != "`\nnext" || a.mdCaret != 2 || a.mdSelEnd != 2 {
		t.Fatalf("草稿并回段落后焦点必须保持原文档位置: active=%d draft=%q sel=%d:%d fence=%v", a.mdActive, a.mdDraft, a.mdCaret, a.mdSelEnd, a.fenceDraft)
	}
}

func TestSourceTypedFenceRoundTripsWithoutAutoPairing(t *testing.T) {
	a := newTestApp(t)
	a.winOpen = true
	a.cfg.Source = true
	tt := ui.NewTester(a.view, 400, 540)
	tt.Frame()
	for _, r := range "````go" {
		tt.Type(string(r))
		tt.Frame()
	}
	if got := a.tabs[0].Text; got != "````go" {
		t.Fatalf("源码逐键输入不得自动补闭栏: %q", got)
	}
	a.toggleSourceMode()
	tt.Frame()
	if got := a.tabs[0].Text; got != "````go" {
		t.Fatalf("源码切回预览必须逐字往返: %q", got)
	}
}

func TestPreviewPasteFenceAndBlurPreservesExactText(t *testing.T) {
	a := newTestApp(t)
	tt := typeInto(t, a, "", func() { a.focusBlockEnd(0) })
	tt.SetClipboard("````go")
	tt.Command("paste")
	tt.Frame()
	if got := a.tabs[0].Text; got != "````go" {
		t.Fatalf("粘贴开栏不得自动补闭栏: %q", got)
	}
	a.blurLive()
	if got := a.tabs[0].Text; got != "````go" {
		t.Fatalf("失焦不得改写粘贴的开栏: %q", got)
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
