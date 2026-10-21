package app

// 查找替换与跨标签查找：替换进撤销事务、跨标签跳转保留查找状态、
// 跨标签全部替换先做安全备份，备份失败不动任何标签。

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestReplaceMatchesUnicode(t *testing.T) {
	text := "Ab甲ab 😀AB"
	got := replaceMatches(text, findMatches(text, "ab"), "乙")
	if got != "乙甲乙 😀乙" {
		t.Fatalf("大小写不敏感的全部替换 = %q", got)
	}
	if got := replaceMatches(text, nil, "x"); got != text {
		t.Fatalf("无命中应原样返回: %q", got)
	}
}

func TestReplaceCurrentStepsAndUndoes(t *testing.T) {
	a := newTestApp(t)
	t.Cleanup(a.shutdown)
	a.cfg.Source = true
	a.tabs[0].Text = "甲 one\n甲 two\n甲 three"
	a.openFind(true, false)
	a.query, a.replaceText = "甲", "乙"
	a.matchIdx = 1

	a.replaceCurrent()
	if got := a.activeTab().Text; got != "甲 one\n乙 two\n甲 three" {
		t.Fatalf("应只替换当前命中: %q", got)
	}
	if a.matchIdx != 1 || !a.jumpPending {
		t.Fatalf("替换后应跳到其后的命中: idx=%d pending=%v", a.matchIdx, a.jumpPending)
	}
	a.undoLive(false)
	if got := a.activeTab().Text; got != "甲 one\n甲 two\n甲 three" {
		t.Fatalf("单次替换应可撤销: %q", got)
	}

	if n := a.replaceAllCurrentTab("甲", "乙"); n != 3 {
		t.Fatalf("全部替换数量 = %d, want 3", n)
	}
	if got := a.activeTab().Text; got != "乙 one\n乙 two\n乙 three" {
		t.Fatalf("当前标签全部替换: %q", got)
	}
	a.undoLive(false)
	if got := a.activeTab().Text; got != "甲 one\n甲 two\n甲 three" {
		t.Fatalf("全部替换应是一个撤销事务: %q", got)
	}
}

func TestReplaceInLivePreviewCommitsDraft(t *testing.T) {
	a := newTestApp(t)
	t.Cleanup(a.shutdown)
	a.tabs[0].Text = "甲甲\n\n乙"
	tt := ui.NewTester(a.view, 500, 600)
	tt.Frame()
	a.openFind(true, false)
	a.query, a.replaceText = "甲", "丙"
	a.replaceAll()
	tt.Frame()
	if got := a.activeTab().Text; got != "丙丙\n\n乙" {
		t.Fatalf("实时预览下全部替换: %q", got)
	}
	if a.notice == "" {
		t.Fatal("全部替换后应提示替换数量")
	}
}

func TestFindAllTabsStepsAcrossTabs(t *testing.T) {
	a := newTestApp(t)
	t.Cleanup(a.shutdown)
	for i := range a.tabs {
		a.tabs[i].Text = ""
	}
	a.tabs[0].Text, a.tabs[2].Text = "甲", "甲 甲"
	a.openFind(false, true)
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

func TestReplaceAllTabsBacksUpFirst(t *testing.T) {
	t.Run("成功", func(t *testing.T) {
		a := newTestApp(t)
		t.Cleanup(a.shutdown)
		a.tabs[0].Text, a.tabs[1].Text, a.tabs[2].Text = "甲", "无关", "甲甲"
		a.resyncLive(-1) // 直接改了文本：焦点块草稿跟上，替换前提交草稿才不会写回旧内容
		a.openFind(true, true)
		if n := a.replaceAllTabsNow("甲", "乙"); n != 3 {
			t.Fatalf("跨标签替换数量 = %d, want 3", n)
		}
		if a.tabs[0].Text != "乙" || a.tabs[1].Text != "无关" || a.tabs[2].Text != "乙乙" {
			t.Fatalf("跨标签替换结果错误: %q %q %q", a.tabs[0].Text, a.tabs[1].Text, a.tabs[2].Text)
		}
		if a.lastBackupTime().IsZero() {
			t.Fatal("跨标签替换前应先做安全备份")
		}
	})
	t.Run("备份失败不替换", func(t *testing.T) {
		a := newTestApp(t)
		t.Cleanup(a.shutdown)
		blocker := a.notes.BackupDir() + "-blocked"
		persistWrite(t, blocker, "普通文件")
		a.notes.SetBackupDir(blocker)
		a.tabs[0].Text, a.tabs[2].Text = "甲", "甲"
		a.resyncLive(-1)
		if n := a.replaceAllTabsNow("甲", "乙"); n != 0 {
			t.Fatalf("备份失败不应替换: %d", n)
		}
		if a.tabs[0].Text != "甲" || a.tabs[2].Text != "甲" || a.notice == "" {
			t.Fatalf("备份失败应保留原文并提示: %q %q notice=%q", a.tabs[0].Text, a.tabs[2].Text, a.notice)
		}
	})
}

func TestFindBarReplaceRowAndEscape(t *testing.T) {
	a := newTestApp(t)
	t.Cleanup(a.shutdown)
	a.winOpen = true
	a.tabs[0].Text = "甲"
	tt := ui.NewTester(a.view, 600, 600)
	tt.Frame()
	if _, ok := tt.Find("全部替换"); ok {
		t.Fatal("查找条关闭时不应出现替换行")
	}
	a.runFixedShortcut("findReplace")
	tt.Frame()
	tt.Frame()
	if _, ok := tt.Find("全部替换"); !ok || !a.replaceOpen {
		t.Fatal("⌥⌘F 应打开带替换行的查找条")
	}
	if !tt.Focused("替换为") {
		t.Fatal("打开替换时焦点应在替换框")
	}
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if a.findOpen {
		t.Fatal("Esc 应关闭查找条")
	}
}
