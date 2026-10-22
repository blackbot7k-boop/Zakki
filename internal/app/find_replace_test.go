package app

// 查找替换与跨标签查找：替换进撤销事务、跨标签跳转保留查找状态、
// 跨标签全部替换只更新同一笔记文件，不生成副本。

import (
	"path/filepath"
	"strings"
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
	if !strings.Contains(strings.Join(tt.Texts(), "\n"), "已替换 2 处") {
		t.Fatal("全部替换后应以 toast 提示替换数量")
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

func TestReplaceAllTabsSavesSingleFile(t *testing.T) {
	a := newTestApp(t)
	t.Cleanup(a.shutdown)
	a.tabs[0].Text, a.tabs[1].Text, a.tabs[2].Text = "甲", "无关", "甲甲"
	a.resyncLive(-1)
	a.openFind(true, true)
	if n := a.replaceAllTabsNow("甲", "乙"); n != 3 {
		t.Fatalf("跨标签替换数量 = %d, want 3", n)
	}
	if a.tabs[0].Text != "乙" || a.tabs[1].Text != "无关" || a.tabs[2].Text != "乙乙" {
		t.Fatalf("跨标签替换结果错误: %q %q %q", a.tabs[0].Text, a.tabs[1].Text, a.tabs[2].Text)
	}
	if err := a.saveNow(); err != nil {
		t.Fatal(err)
	}
	f := persistReadSaved(t, a.notes.DataPath())
	if len(f.Tabs) != len(a.tabs) {
		t.Fatalf("标签数量丢失: %d", len(f.Tabs))
	}
	for i := range a.tabs {
		if f.Tabs[i].Text != a.tabs[i].Text {
			t.Fatalf("标签 %d 未写回: %q", i, f.Tabs[i].Text)
		}
	}
	assertOnlyStorageFiles(t, filepath.Dir(a.notes.DataPath()), "zakki.json")
	a.undoLive(false)
	if a.tabs[0].Text != "甲" || a.tabs[1].Text != "无关" || a.tabs[2].Text != "乙乙" {
		t.Fatalf("跨标签替换应保留活动标签撤销事务: %q", tabTexts(a))
	}
}

func TestReplaceAllTabsDoesNotDependOnLegacyBackupDirectory(t *testing.T) {
	a := newTestApp(t)
	t.Cleanup(a.shutdown)
	dir := filepath.Dir(a.notes.DataPath())
	persistWrite(t, filepath.Join(dir, "backups"), "旧备份位置不可用")
	a.tabs[0].Text, a.tabs[2].Text = "甲", "甲"
	a.resyncLive(-1)
	if n := a.replaceAllTabsNow("甲", "乙"); n != 2 {
		t.Fatalf("替换不应依赖旧备份目录: %d", n)
	}
	if a.tabs[0].Text != "乙" || a.tabs[2].Text != "乙" {
		t.Fatalf("替换内容错误: %q", tabTexts(a))
	}
	if err := a.saveNow(); err != nil {
		t.Fatal(err)
	}
	if got := persistReadSaved(t, a.notes.DataPath()); got.Tabs[0].Text != "乙" || got.Tabs[2].Text != "乙" {
		t.Fatalf("替换未保存: %+v", got.Tabs)
	}
	if got := string(persistReadRaw(t, filepath.Join(dir, "backups"))); got != "旧备份位置不可用" {
		t.Fatalf("旧位置不应改动: %q", got)
	}
	assertOnlyStorageFiles(t, dir, "zakki.json", "backups")
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
