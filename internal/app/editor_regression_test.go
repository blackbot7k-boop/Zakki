package app

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func TestSourceToPreviewKeepsAuthoritativeSourceText(t *testing.T) {
	a := newTestApp(t)
	t.Cleanup(a.shutdown)
	a.tabs[0].Text = "甲"
	tt := ui.NewTester(a.view, 400, 540)
	a.focusBlock(0, 1, 1)
	tt.Frame()

	a.toggleSourceMode()
	tt.Frame()
	tt.Command("selectAll")
	tt.Type("新内容")
	if got := a.tabs[0].Text; got != "新内容" {
		t.Fatalf("源码输入后=%q, want 新内容", got)
	}
	a.toggleSourceMode()
	tt.Frame()
	if got := a.tabs[0].Text; got != "新内容" {
		t.Fatalf("切回预览后源码修改被旧草稿覆盖: %q", got)
	}
}

func TestBlankEnterDoesNotOverwriteFollowingParagraph(t *testing.T) {
	for _, tc := range []struct {
		name  string
		text  string
		block int
		want  string
	}{
		{"单个中间空段", "甲\n\n乙", 1, "甲\n\n\n乙"},
		{"连续中间空段", "甲\n\n\n乙", 2, "甲\n\n\n\n乙"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := newTestApp(t)
			t.Cleanup(a.shutdown)
			a.tabs[0].Text = tc.text
			tt := ui.NewTester(a.view, 400, 540)
			a.focusBlock(tc.block, 0, 0)
			tt.Frame()
			tt.Key(0, ui.KeyEnter)
			if got := a.tabs[0].Text; got != tc.want {
				t.Fatalf("空段回车后=%q, want %q", got, tc.want)
			}
		})
	}
}

func TestRunOnMainDropsCallbackWithoutLiveWindow(t *testing.T) {
	a := newTestApp(t)
	t.Cleanup(a.shutdown)
	called := false
	a.runOnMain(func() { called = true })
	if called {
		t.Fatal("无存活窗口时 runOnMain 不应在调用 goroutine 直接执行")
	}
}

// TestSourceSessionsIsolateSelectionNotHistory 源码会话按标签恢复光标与选区；
// 撤销历史只属于当前这段编辑，切换标签即清空，切回也不带回旧历史。
func TestSourceSessionsIsolateSelectionNotHistory(t *testing.T) {
	a := newTestApp(t)
	t.Cleanup(a.shutdown)
	a.cfg.Source = true
	a.tabs[0].Text, a.tabs[1].Text = "AAA", "BBB"
	a.srcCaret, a.srcSelEnd = 1, 3
	a.srcSelSyncStart, a.srcSelSyncEnd = 1, 3
	a.liveUndo = []liveEdit{{
		before: liveSnapshot{doc: "AA", start: 2, end: 2},
		after:  liveSnapshot{doc: "AAA", start: 3, end: 3},
		kind:   editInput,
	}}

	a.switchTab(1)
	if a.srcCaret != 0 || a.srcSelEnd != 0 || len(a.liveUndo) != 0 {
		t.Fatalf("新标签继承了源码会话: caret=%d end=%d undo=%d", a.srcCaret, a.srcSelEnd, len(a.liveUndo))
	}
	a.srcCaret, a.srcSelEnd = 2, 2
	a.liveUndo = []liveEdit{{before: liveSnapshot{doc: "B"}, after: liveSnapshot{doc: "BBB"}, kind: editInput}}

	a.switchTab(0)
	if a.srcCaret != 1 || a.srcSelEnd != 3 {
		t.Fatalf("标签一选区未恢复: %d..%d", a.srcCaret, a.srcSelEnd)
	}
	if len(a.liveUndo) != 0 || len(a.liveRedo) != 0 {
		t.Fatalf("切回标签不应带回撤销历史: undo=%+v redo=%+v", a.liveUndo, a.liveRedo)
	}
	a.undoLive(false)
	if a.tabs[0].Text != "AAA" {
		t.Fatalf("无历史时撤销不得改动文本: %q", a.tabs[0].Text)
	}
}

func TestSourceSessionSurvivesStructuralTabChanges(t *testing.T) {
	a := newTestApp(t)
	t.Cleanup(a.shutdown)
	a.cfg.Source = true
	a.tabs[0].Text, a.tabs[1].Text, a.tabs[2].Text = "AAA", "BBB", "CCC"
	activeID := a.tabs[0].ID
	wantEdit := liveEdit{
		before: liveSnapshot{doc: "AA", start: 2, end: 2},
		after:  liveSnapshot{doc: "AAA", start: 3, end: 3},
		kind:   editInput,
	}
	a.srcCaret, a.srcSelEnd = 1, 3
	a.srcSelSyncStart, a.srcSelSyncEnd = 1, 3
	a.liveUndo = []liveEdit{wantEdit}

	assertSession := func(step string) {
		t.Helper()
		if a.tabs[a.active].ID != activeID {
			t.Fatalf("%s 后活动标签改变: got %d want %d", step, a.tabs[a.active].ID, activeID)
		}
		if a.srcCaret != 1 || a.srcSelEnd != 3 || len(a.liveUndo) != 1 || a.liveUndo[0] != wantEdit {
			t.Fatalf("%s 后源码会话丢失: caret=%d end=%d undo=%+v", step, a.srcCaret, a.srcSelEnd, a.liveUndo)
		}
	}

	if !a.deleteTabNow(1) {
		t.Fatal("删除非活动标签失败")
	}
	assertSession("删除非活动标签")
	if !a.moveTab(1, 1) {
		t.Fatal("移动非活动标签失败")
	}
	assertSession("移动非活动标签")
	if !a.moveTab(a.active, 1) {
		t.Fatal("移动活动标签失败")
	}
	assertSession("移动活动标签")
}

func TestSourceSelectionDoesNotLeakAcrossTabSwitch(t *testing.T) {
	a := newTestApp(t)
	t.Cleanup(a.shutdown)
	a.cfg.Source = true
	a.tabs[0].Text, a.tabs[1].Text = "AAA", "BBB"
	tt := ui.NewTester(a.view, 500, 600)
	a.menuSelectAll()
	tt.Frame()
	tt.Key(ui.Super, ui.Key2)
	tt.Frame()
	tt.Type("X")
	if got := a.tabs[1].Text; got != "BBBX" {
		t.Fatalf("源码选区跨标签泄漏，标签二=%q, want BBBX", got)
	}
}

func TestBlockStartBackspaceSchedulesAutosave(t *testing.T) {
	a := newTestApp(t)
	t.Cleanup(a.shutdown)
	a.tabs[0].Text = "甲\n\n乙"
	if err := a.saveNow(); err != nil {
		t.Fatal(err)
	}
	tt := ui.NewTester(a.view, 400, 540)
	a.focusBlock(2, 0, 0)
	tt.Frame()
	tt.Key(0, ui.KeyBackspace)
	if got := a.tabs[0].Text; got != "甲乙" {
		t.Fatalf("段首退格后=%q, want 甲乙", got)
	}
	waitForSavedText(t, a, "甲乙")
}

func TestLongPasteRestoresVirtualEditorFocus(t *testing.T) {
	for _, viaMenu := range []bool{false, true} {
		name := map[bool]string{false: "输入命令", true: "菜单"}[viaMenu]
		t.Run(name, func(t *testing.T) {
			a := newTestApp(t)
			t.Cleanup(a.shutdown)
			a.tabs[0].Text = "甲"
			tt := ui.NewTester(a.view, 400, 540)
			a.focusBlock(0, 1, 1)
			tt.Frame()
			pasted := strings.Repeat("段\n\n", 100) + "尾"
			tt.SetClipboard(pasted)
			if viaMenu {
				chooseEditMenu(t, a, tt, "paste")
			} else {
				tt.Command("paste")
			}
			for range 3 {
				tt.Frame()
			}
			tt.Type("X")
			if got, want := a.tabs[0].Text, "甲"+pasted+"X"; got != want {
				t.Fatalf("长粘贴后的输入丢失，末尾=%q", got[max(0, len(got)-20):])
			}
		})
	}
}

func TestSourcePasteAndIMEAreIndependentUndoTransactions(t *testing.T) {
	t.Run("粘贴", func(t *testing.T) {
		a := newTestApp(t)
		t.Cleanup(a.shutdown)
		a.cfg.Source = true
		a.tabs[0].Text = ""
		tt := ui.NewTester(a.view, 400, 540)
		tt.Type("甲")
		tt.SetClipboard("乙")
		tt.Command("paste")
		tt.Key(ui.Cmd, ui.KeyZ)
		if got := a.tabs[0].Text; got != "甲" {
			t.Fatalf("撤销粘贴后=%q, want 甲", got)
		}
	})
	t.Run("输入法确认", func(t *testing.T) {
		a := newTestApp(t)
		t.Cleanup(a.shutdown)
		a.cfg.Source = true
		a.tabs[0].Text = ""
		tt := ui.NewTester(a.view, 400, 540)
		tt.Type("甲")
		tt.Compose("ni", 2)
		tt.Compose("你", 1)
		tt.Type("你")
		if got := a.tabs[0].Text; got != "甲你" {
			t.Fatalf("输入法确认后=%q, want 甲你", got)
		}
		tt.Key(ui.Cmd, ui.KeyZ)
		if got := a.tabs[0].Text; got != "甲" {
			t.Fatalf("撤销输入法确认后=%q, want 甲", got)
		}
	})
}

func TestStaleLinkOverlayCannotEditChangedDocument(t *testing.T) {
	a := newTestApp(t)
	t.Cleanup(a.shutdown)
	a.cfg.Source = true
	a.tabs[0].Text = "原文"
	a.srcCaret, a.srcSelEnd = 0, 2
	a.openLinkEditor()
	if !a.linkTool.open || a.linkTool.tabID != a.tabs[0].ID || a.linkTool.doc != "原文" {
		t.Fatalf("链接浮层未保存标签与文档快照: %+v", a.linkTool)
	}
	a.linkTool.text, a.linkTool.url = "链接", "https://example.com"
	a.tabs[0].Text = "前缀原文"
	a.applyLinkTool()
	if got := a.tabs[0].Text; got != "前缀原文" {
		t.Fatalf("陈旧链接浮层修改了新文档: %q", got)
	}
	if a.linkTool.open {
		t.Fatal("陈旧链接浮层确认后未关闭")
	}
}

func TestFindMenuFocusesQueryInsteadOfDocument(t *testing.T) {
	for _, source := range []bool{false, true} {
		t.Run(map[bool]string{false: "实时预览", true: "源码"}[source], func(t *testing.T) {
			a := newTestApp(t)
			t.Cleanup(a.shutdown)
			a.cfg.Source = source
			a.tabs[0].Text = "AAA"
			a.resyncLive(0)
			tt := ui.NewTester(a.view, 500, 600)
			if !source {
				a.focusBlock(0, 3, 3)
				tt.Frame()
			}
			a.miFind = nil
			menu := a.createMenu()
			item := menu.ItemByID("find")
			if item == nil || item.Click == nil {
				t.Fatal("找不到可执行的查找菜单项")
			}
			item.Click(item, nil)
			tt.Frame()
			tt.Type("search")
			if a.query != "search" || a.tabs[0].Text != "AAA" {
				t.Fatalf("查找菜单焦点错误: query=%q document=%q", a.query, a.tabs[0].Text)
			}
		})
	}
}

func TestUnfocusedPreviewMenuPasteAppendsAndRemainsEditable(t *testing.T) {
	a := newTestApp(t)
	t.Cleanup(a.shutdown)
	a.tabs[0].Text, a.tabs[1].Text = "AAA", "BBB"
	tt := ui.NewTester(a.view, 500, 600)
	a.switchTab(1)
	tt.Frame()
	pasted := "## 标题\n\n正文"
	tt.SetClipboard(pasted)
	chooseEditMenu(t, a, tt, "paste")
	if got, want := a.tabs[1].Text, "BBB\n"+pasted; got != want {
		t.Fatalf("未聚焦预览菜单粘贴=%q, want %q", got, want)
	}
	if got := a.tabs[0].Text; got != "AAA" {
		t.Fatalf("菜单粘贴修改了非当前标签: %q", got)
	}
	for range 3 {
		tt.Frame()
	}
	tt.Type("X")
	wantWithTyping := "BBB\n" + pasted + "X"
	if got := a.tabs[1].Text; got != wantWithTyping {
		t.Fatalf("菜单粘贴后焦点未恢复: %q, want %q", got, wantWithTyping)
	}
	waitForSavedTextAt(t, a, 1, wantWithTyping)

	chooseEditMenu(t, a, tt, "undo")
	if got := a.tabs[1].Text; got != "BBB\n"+pasted {
		t.Fatalf("第一次撤销应只撤后续输入: %q", got)
	}
	chooseEditMenu(t, a, tt, "undo")
	if got := a.tabs[1].Text; got != "BBB" {
		t.Fatalf("第二次撤销应只撤菜单粘贴: %q", got)
	}
}

func waitForSavedText(t *testing.T, a *app, want string) {
	t.Helper()
	waitForSavedTextAt(t, a, 0, want)
}

func waitForSavedTextAt(t *testing.T, a *app, tab int, want string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		raw, err := os.ReadFile(a.notes.DataFile)
		if err == nil {
			var saved savedFile
			if json.Unmarshal(raw, &saved) == nil && tab < len(saved.Tabs) && saved.Tabs[tab].Text == want {
				return
			}
		}
		time.Sleep(25 * time.Millisecond)
	}
	raw, err := os.ReadFile(a.notes.DataFile)
	if err != nil {
		t.Fatal(err)
	}
	var saved savedFile
	if err := json.Unmarshal(raw, &saved); err != nil {
		t.Fatal(err)
	}
	if tab >= len(saved.Tabs) {
		t.Fatalf("保存结果缺少标签 %d: %+v", tab, saved.Tabs)
	}
	t.Fatalf("标签 %d 保存内容=%q, want %q", tab, saved.Tabs[tab].Text, want)
}
