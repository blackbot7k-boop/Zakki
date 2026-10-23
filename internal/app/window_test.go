package app

import (
	"reflect"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestMinimumWindowSizeFollowsEffectiveTypography(t *testing.T) {
	p := defaultPreferences()
	baseW, baseH := minimumWindowSize(p)
	if baseW != 300 || baseH != 340 {
		t.Fatalf("默认排版应保留基础最小尺寸: %dx%d", baseW, baseH)
	}
	p.Appearance.BodySize = 40
	p.Appearance.LineHeight = 3
	p.Appearance.PadX = 120
	p.Appearance.PadY = 120
	largeW, largeH := minimumWindowSize(p)
	if largeW < 640 || largeH < 839 {
		t.Fatalf("极端合法排版下未保留 10 列 4 行正文: %dx%d", largeW, largeH)
	}
}

func TestWindowUIRebuildPreservesDocument(t *testing.T) {
	a := newTestApp(t)
	a.notes.SetStopped(true)
	a.tabs[0].Text = "第一段 **内容**\n\n第二段"
	a.resyncLive(0)
	a.liveUndo = []liveEdit{{before: liveSnapshot{doc: "旧内容"}, after: liveSnapshot{doc: a.tabs[0].Text}}}
	beforeTabs := append([]tab(nil), a.tabs...)
	beforeUndo := append([]liveEdit(nil), a.liveUndo...)
	beforeCaret, beforeActive := a.mdCaret, a.mdActive

	for i := 0; i < 5; i++ {
		a.resetWindowUI()
		if a.clipSvcOK {
			t.Fatal("重建窗口不能复用旧窗口的剪贴板服务")
		}
		tt := ui.NewTester(a.view, 400, 540)
		if !tt.HasText("第二段") {
			t.Fatal("重建窗口后没有渲染笔记")
		}
		a.clipSvcOK = true
	}
	if !reflect.DeepEqual(a.tabs, beforeTabs) || !reflect.DeepEqual(a.liveUndo, beforeUndo) || a.mdCaret != beforeCaret || a.mdActive != beforeActive {
		t.Fatalf("重建窗口改变了状态：tabs=%v undo=%v caret=%d/%d active=%d/%d", reflect.DeepEqual(a.tabs, beforeTabs), reflect.DeepEqual(a.liveUndo, beforeUndo), a.mdCaret, beforeCaret, a.mdActive, beforeActive)
	}
}
