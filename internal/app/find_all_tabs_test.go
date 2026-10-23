package app

// 跨标签查找：命中计数、跳转与查找状态在标签切换后保持一致。

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestFindAllTabsStepsAcrossTabs(t *testing.T) {
	a := newTestApp(t)
	t.Cleanup(a.shutdown)
	for i := range a.tabs {
		a.tabs[i].Text = ""
	}
	a.tabs[0].Text, a.tabs[2].Text = "甲", "甲 甲"
	a.openFind(true)
	a.query = "甲"

	if total, tabs := a.allTabHits(); total != 3 || tabs != 2 {
		t.Fatalf("跨标签命中 = %d/%d, want 3/2", total, tabs)
	}
	a.findStep(1)
	if a.active != 2 || a.matchIdx != 0 || !a.findOpen || a.query != "甲" || !a.findAllTabs {
		t.Fatalf("应跳到下一个有命中的标签并保留查找: active=%d idx=%d open=%v q=%q", a.active, a.matchIdx, a.findOpen, a.query)
	}
	a.findStep(1)
	if a.active != 2 || a.matchIdx != 1 {
		t.Fatalf("同标签内先走完命中: active=%d idx=%d", a.active, a.matchIdx)
	}
	a.findStep(1)
	if a.active != 0 || a.matchIdx != 0 {
		t.Fatalf("越过末尾应循环回第一个标签: active=%d idx=%d", a.active, a.matchIdx)
	}
	a.findStep(-1)
	if a.active != 2 || a.matchIdx != 1 {
		t.Fatalf("向上越过开头应落到上一个标签的最后命中: active=%d idx=%d", a.active, a.matchIdx)
	}
	a.findAllTabs = false
	a.findStep(1)
	if a.active != 2 || a.matchIdx != 0 {
		t.Fatalf("关闭跨标签后只在当前标签循环: active=%d idx=%d", a.active, a.matchIdx)
	}
}

func TestFindBarOnlyShowsSearchControls(t *testing.T) {
	a := newTestApp(t)
	t.Cleanup(a.shutdown)
	a.winOpen = true
	a.tabs[0].Text = "甲"
	tt := ui.NewTester(a.view, 600, 600)
	a.runFixedShortcut("find")
	tt.Frame()

	text := strings.Join(tt.Texts(), "\n")
	if strings.Contains(text, "替换") || strings.Contains(strings.ToLower(text), "replace") {
		t.Fatalf("查找条不应再暴露替换控件: %q", text)
	}
	if _, ok := tt.Find("在所有标签中查找"); !ok {
		t.Fatal("查找条应保留跨标签查找入口")
	}
	if !tt.Focused("查找") {
		t.Fatal("打开查找时焦点应在查找框")
	}
}
