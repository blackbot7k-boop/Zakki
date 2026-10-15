package app

import (
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/egoist/mygo/ui"
)

func TestFocusedBlocksKeepRenderedContents(t *testing.T) {
	for _, tc := range []struct{ raw, visible, edited string }{
		{"- [ ] 123", "123", "- [ ] 123中文"},
		{"# 标题", "标题", "# 标题中文"},
		{"- 列表", "列表", "- 列表中文"},
		{"> 引用", "引用", "> 引用中文"},
		{"**粗体**", "粗体", "**粗体中文**"},
		{"[链接](https://example.com)", "链接", "[链接中文](https://example.com)"},
		{"```go\n代码\n```", "代码", "```go\n代码中文\n```"},
	} {
		t.Run(tc.raw, func(t *testing.T) {
			a := newTestApp(t)
			t.Cleanup(a.shutdown)
			a.tabs[0].Text = tc.raw
			tt := ui.NewTester(a.view, 400, 540)
			a.focusBlock(0, utf8.RuneCountInString(tc.raw), utf8.RuneCountInString(tc.raw))
			tt.Frame()
			if a.mdVisible != tc.visible {
				t.Fatalf("focus exposed source: visible=%q want=%q", a.mdVisible, tc.visible)
			}
			if tc.raw != tc.visible && tt.HasText(tc.raw) {
				t.Fatalf("source is still drawn: %q", tt.Texts())
			}
			tt.Type("中文")
			if a.tabs[0].Text != tc.edited {
				t.Fatalf("edit corrupted source: %q want=%q", a.tabs[0].Text, tc.edited)
			}
			tt.Key(ui.Super, ui.KeyZ)
			if a.tabs[0].Text != tc.raw {
				t.Fatalf("undo lost formatting: %q", a.tabs[0].Text)
			}
			tt.Key(ui.Super|ui.Shift, ui.KeyZ)
			if a.tabs[0].Text != tc.edited {
				t.Fatalf("redo lost formatting: %q", a.tabs[0].Text)
			}
			a.blurLive()
			tt.Frame()
			if !tt.HasText(tc.visible + "中文") {
				t.Fatalf("blur differs from focus: %q", tt.Texts())
			}
		})
	}
}

func TestTaskTextAndCheckboxStayIndependent(t *testing.T) {
	a := newTestApp(t)
	t.Cleanup(a.shutdown)
	a.tabs[0].Text = "- [ ] 123"
	a.mdActive = -1
	tt := ui.NewTester(a.view, 400, 540)
	if err := tt.Click("123"); err != nil {
		t.Fatal(err)
	}
	if !tt.Focused("内容") || a.mdVisible != "123" {
		t.Fatalf("click must edit only task text: %q", a.mdVisible)
	}
	tt.Type("X")
	if a.tabs[0].Text != "- [ ] 123X" {
		t.Fatal(a.tabs[0].Text)
	}
	if err := tt.Click("完成：123X"); err != nil {
		t.Fatal(err)
	}
	if a.tabs[0].Text != "- [x] 123X" {
		t.Fatal(a.tabs[0].Text)
	}
	// Repeated focus/blur cycles cannot bring the hidden prefix back.
	for i := 0; i < 3; i++ {
		a.blurLive()
		tt.Frame()
		if err := tt.Click("123X"); err != nil {
			t.Fatal(err)
		}
		if a.mdVisible != "123X" {
			t.Fatal(a.mdVisible)
		}
	}
}

func TestVisibleSelectionPreservesWrappers(t *testing.T) {
	for _, tc := range []struct{ raw, edited string }{
		{"**中文** 尾", "**替换** 尾"},
		{"[中文](https://example.com) 尾", "[替换](https://example.com) 尾"},
	} {
		t.Run(tc.raw, func(t *testing.T) {
			a := newTestApp(t)
			t.Cleanup(a.shutdown)
			a.tabs[0].Text = tc.raw
			tt := ui.NewTester(a.view, 400, 540)
			p := projectBlock(a.mdBlocks()[0])
			a.selSyncStart, a.selSyncEnd = p.sourceOffset(0), p.sourceOffset(2)
			tt.Frame()
			tt.Type("替换")
			if a.tabs[0].Text != tc.edited {
				t.Fatalf("replacement lost wrapper: %q", a.tabs[0].Text)
			}
			tt.Key(ui.Super, ui.KeyZ)
			if a.tabs[0].Text != tc.raw {
				t.Fatalf("undo replacement: %q", a.tabs[0].Text)
			}
		})
	}
}

func TestCellClickKeepsGridAndEscapesText(t *testing.T) {
	a := newTestApp(t)
	t.Cleanup(a.shutdown)
	a.tabs[0].Text = "| a | b | c |\n| --- | --- | --- |\n|   |   |   |"
	a.mdActive = -1
	tt := ui.NewTester(a.view, 600, 540)
	if err := tt.Click("单元格 2,2"); err != nil {
		t.Fatal(err)
	}
	if !tt.Focused("内容") || !tt.HasText("a") || !tt.HasText("c") || a.mdVisible != "" {
		t.Fatalf("grid disappeared on focus: visible=%q texts=%q", a.mdVisible, tt.Texts())
	}
	tt.Type("中文|值")
	if !strings.Contains(a.tabs[0].Text, `中文\|值`) {
		t.Fatalf("cell pipes not escaped: %q", a.tabs[0].Text)
	}
	if len(a.mdBlocks()) != 1 || a.mdBlocks()[0].kind != bkTable || len(a.mdBlocks()[0].table.rows[1]) != 3 {
		t.Fatal("cell input broke table")
	}
	tt.Key(0, ui.KeyTab)
	tt.Type("末格")
	if a.mdBlocks()[0].table.rows[1][2].text != "末格" {
		t.Fatalf("Tab input in wrong cell: %q", a.tabs[0].Text)
	}
	tt.Key(ui.Shift, ui.KeyTab)
	if a.mdVisible != "中文|值" {
		t.Fatalf("Shift+Tab must show visible text: %q", a.mdVisible)
	}
	tt.Type("替换")
	if a.mdBlocks()[0].table.rows[1][1].text != "替换" {
		t.Fatalf("selected cell replacement: %q", a.tabs[0].Text)
	}
	// Keyboard movement/extra frames keep the grid and input alive.
	for _, key := range []ui.Key{ui.KeyLeft, ui.KeyRight, ui.KeyUp, ui.KeyDown} {
		tt.Key(0, key)
		tt.Frame()
	}
	if !tt.HasText("a") || !tt.HasText("c") || !tt.Focused("内容") {
		t.Fatal("navigation dropped grid/focus")
	}
}

func TestProjectedChineseComposition(t *testing.T) {
	for _, raw := range []string{"- [ ] 中文", "| a | b |\n| --- | --- |\n| 中文 | 2 |"} {
		t.Run(fmt.Sprintf("%s", raw), func(t *testing.T) {
			a := newTestApp(t)
			t.Cleanup(a.shutdown)
			a.tabs[0].Text = raw
			tt := ui.NewTester(a.view, 500, 540)
			if a.mdBlocks()[0].kind == bkTable {
				a.focusTableCell(0, 1, 0)
			} else {
				n := utf8.RuneCountInString(raw)
				a.focusBlock(0, n, n)
			}
			tt.Frame()
			old := a.tabs[0].Text
			tt.Compose("ni", 2)
			tt.Frame()
			tt.Compose("你", 1)
			tt.Frame()
			if a.tabs[0].Text != old {
				t.Fatalf("preedit leaked to document: %q", a.tabs[0].Text)
			}
			tt.Type("你好")
			if !strings.Contains(a.tabs[0].Text, "中文你好") {
				t.Fatalf("Chinese commit lost text: %q", a.tabs[0].Text)
			}
			if strings.Contains(a.tabs[0].Text, "ni") {
				t.Fatal("preedit retained")
			}
		})
	}
}

func TestProjectedHeadingKeepsHeight(t *testing.T) {
	a := newTestApp(t)
	t.Cleanup(a.shutdown)
	a.tabs[0].Text = "# 标题\n\n正文"
	a.mdActive = -1
	tt := ui.NewTester(a.view, 400, 540)
	r, ok := tt.Find("标题")
	if !ok {
		t.Fatal("missing heading")
	}
	if err := tt.Click("标题"); err != nil {
		t.Fatal(err)
	}
	input, ok := tt.Find("内容")
	if !ok {
		t.Fatal("missing formatted input")
	}
	if input.H < r.H-2 || input.H > r.H+2 {
		t.Fatalf("heading jumps on focus: preview=%v editor=%v", r, input)
	}
}
