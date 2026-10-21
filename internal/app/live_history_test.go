package app

import (
	"math/rand"
	"strings"
	"testing"
	"unicode/utf8"
	"unsafe"
)

func historyTestApp(t *testing.T, doc string) *app {
	t.Helper()
	a := newTestApp(t)
	a.notes.SetStopped(true)
	a.cfg.Source = true
	a.tabs[0].Text = doc
	return a
}

func historyTestEdit(a *app, doc string, start, end int, kind editKind) {
	before := a.liveSnapshot()
	a.activeTab().Text = doc
	a.srcSelSyncStart, a.srcSelSyncEnd = start, end
	a.rememberLiveEdit(before, kind)
}

func TestLiveHistoryCurrentAnchors(t *testing.T) {
	for _, size := range []int{0, 1, 4095, 4096, 4097, 65536, 1048576} {
		doc := strings.Repeat("甲🙂x", size/8+1)[:size]
		a := historyTestApp(t, doc)
		for range 3 {
			historyTestEdit(a, a.tabs[0].Text+"字", 0, 0, editIME)
		}
		for _, redo := range []bool{false, false, true, false, false, true} {
			a.undoLive(redo)
			for _, stack := range [][]liveEdit{a.liveUndo, a.liveRedo} {
				if len(stack) == 0 {
					continue
				}
				top := stack[len(stack)-1]
				if top.delta != nil {
					t.Fatalf("size=%d: 栈顶未保留完整校验快照", size)
				}
			}
			if len(a.liveUndo) > 0 && a.liveUndo[len(a.liveUndo)-1].after.doc != a.tabs[0].Text {
				t.Fatal("撤销栈顶与当前文档脱节")
			}
			if len(a.liveRedo) > 0 && a.liveRedo[len(a.liveRedo)-1].before.doc != a.tabs[0].Text {
				t.Fatal("重做栈顶与当前文档脱节")
			}
		}
	}
}

func TestLiveHistoryUnicodeRandomRoundTrip(t *testing.T) {
	rng := rand.New(rand.NewSource(7321))
	a := historyTestApp(t, strings.Repeat("甲🙂é\n", 9000))
	docs := []string{a.tabs[0].Text}
	selections := [][2]int{{0, 0}}
	for i := range 200 {
		rs := []rune(a.tabs[0].Text)
		lo := rng.Intn(len(rs) + 1)
		hi := min(len(rs), lo+rng.Intn(9))
		fragment := []string{"中", "🙂é", "\n新\n", "", "乙", "丙"}[rng.Intn(6)]
		next := string(rs[:lo]) + fragment + string(rs[hi:])
		if next == a.tabs[0].Text {
			next += "尾"
		}
		end := lo + utf8.RuneCountInString(fragment)
		historyTestEdit(a, next, lo, end, editIME)
		docs = append(docs, next)
		selections = append(selections, [2]int{lo, end})
		if len(a.liveUndo) != i+1 {
			t.Fatalf("step %d: history=%d", i, len(a.liveUndo))
		}
	}
	for i := 199; i >= 0; i-- {
		a.undoLive(false)
		if a.tabs[0].Text != docs[i] || a.srcSelSyncStart != selections[i][0] || a.srcSelSyncEnd != selections[i][1] {
			t.Fatalf("undo %d: document/selection mismatch", i)
		}
	}
	if len(a.liveUndo) != 0 || len(a.liveRedo) != 200 {
		t.Fatal("200 undo did not move all transactions")
	}
	for i := 1; i <= 200; i++ {
		a.undoLive(true)
		if a.tabs[0].Text != docs[i] || a.srcSelSyncStart != selections[i][0] || a.srcSelSyncEnd != selections[i][1] {
			t.Fatalf("redo %d: document/selection mismatch", i)
		}
	}
}

func TestLiveHistoryLimitAndSlots(t *testing.T) {
	a := historyTestApp(t, strings.Repeat("x", 1<<16))
	for range 250 {
		historyTestEdit(a, a.tabs[0].Text+"字", 0, 0, editIME)
	}
	if len(a.liveUndo) != 200 {
		t.Fatalf("history=%d, want 200", len(a.liveUndo))
	}
	for i, e := range a.liveUndo[:199] {
		if e.delta == nil || e.before.doc != "" || e.after.doc != "" || len(e.delta.removed)+len(e.delta.inserted) != len("字") {
			t.Fatalf("history %d retains a full snapshot or wrong fragment", i)
		}
	}
	base := strings.Repeat("x", 1<<16) + strings.Repeat("字", 50)
	for range 200 {
		backing := a.liveUndo[:cap(a.liveUndo)]
		index := len(a.liveUndo) - 1
		a.undoLive(false)
		if backing[index] != (liveEdit{}) {
			t.Fatal("undo pop retains bottom slot")
		}
	}
	if a.tabs[0].Text != base {
		t.Fatal("oldest retained transaction was lost")
	}
	for i, e := range a.liveRedo[:199] {
		if e.delta == nil || e.before.doc != "" || e.after.doc != "" {
			t.Fatalf("redo %d retains a full snapshot", i)
		}
	}
	for range 200 {
		backing := a.liveRedo[:cap(a.liveRedo)]
		index := len(a.liveRedo) - 1
		a.undoLive(true)
		if backing[index] != (liveEdit{}) {
			t.Fatal("redo pop retains bottom slot")
		}
	}
}

func TestLiveHistoryBranchMergesCompressedInput(t *testing.T) {
	for _, kind := range []editKind{editInput, editFencePair} {
		t.Run(map[editKind]string{editInput: "input", editFencePair: "fence"}[kind], func(t *testing.T) {
			base := strings.Repeat("x", 1<<16)
			a := historyTestApp(t, base)
			a.srcSelSyncStart, a.srcSelSyncEnd = len(base), len(base)
			historyTestEdit(a, base+"`", len(base)+1, len(base)+1, editInput)
			historyTestEdit(a, base+"`甲", len(base)+2, len(base)+2, editIME)
			if a.liveUndo[0].delta == nil {
				t.Fatal("previous input is not compressed")
			}
			a.undoLive(false)
			next := base + "``"
			if kind == editFencePair {
				next = base + "```\n\n```"
			}
			historyTestEdit(a, next, len(base)+2, len(base)+2, kind)
			if len(a.liveUndo) != 1 || len(a.liveRedo) != 0 || a.liveUndo[0].delta != nil {
				t.Fatal("branch failed to merge/materialize or discard redo")
			}
			if kind == editFencePair && a.liveUndo[0].kind != editCommand {
				t.Fatal("fence transaction did not become a command")
			}
			a.undoLive(false)
			if a.tabs[0].Text != base {
				t.Fatal("merged branch lost original before")
			}
			a.undoLive(true)
			if a.tabs[0].Text != next {
				t.Fatal("merged branch failed redo")
			}
		})
	}
}

func TestLiveHistoryExternalRewriteRejected(t *testing.T) {
	for _, redo := range []bool{false, true} {
		a := historyTestApp(t, strings.Repeat("x", 1<<16))
		historyTestEdit(a, a.tabs[0].Text+"甲", 1, 1, editIME)
		historyTestEdit(a, a.tabs[0].Text+"乙", 2, 2, editPaste)
		if redo {
			a.undoLive(false)
			a.undoLive(false)
		} else {
			a.undoLive(false)
		}
		changed := "y" + a.tabs[0].Text[1:]
		a.tabs[0].Text = changed
		a.undoLive(redo)
		if a.tabs[0].Text != changed || len(a.liveUndo)+len(a.liveRedo) != 0 {
			t.Fatalf("redo=%v accepted a rewrite outside replacement region", redo)
		}
	}
}

func TestLiveHistoryRememberRejectsExternalRewrite(t *testing.T) {
	base := strings.Repeat("x", 1<<16)
	a := historyTestApp(t, base)
	historyTestEdit(a, base+"甲", 1, 1, editIME)
	historyTestEdit(a, base+"甲乙", 2, 2, editIME)
	a.undoLive(false)
	a.tabs[0].Text = "y" + a.tabs[0].Text[1:]
	changed := a.tabs[0].Text
	historyTestEdit(a, changed+"丙", 2, 2, editInput)
	if len(a.liveUndo) != 1 || len(a.liveRedo) != 0 {
		t.Fatal("remember accepted stale compressed history")
	}
	a.undoLive(false)
	if a.tabs[0].Text != changed {
		t.Fatal("new history crossed the external rewrite")
	}
}

func TestLiveHistoryContinuityAfterCompressedTop(t *testing.T) {
	base := strings.Repeat("x", 1<<16)
	a := historyTestApp(t, base)
	historyTestEdit(a, base+"甲", 1, 1, editIME)
	historyTestEdit(a, base+"甲乙", 2, 2, editPaste)
	a.undoLive(false)
	historyTestEdit(a, base+"甲丙", 2, 2, editCommand)
	if len(a.liveUndo) != 2 {
		t.Fatal("compressed after broke continuity")
	}
	a.undoLive(false)
	a.undoLive(false)
	if a.tabs[0].Text != base {
		t.Fatal("branch dropped old history")
	}
}

// TestLiveHistoryClearedOnTabSwitch 撤销历史不跨标签保留：切走再切回后
// 撤销不再回到切换前的编辑，文本保持原样。
func TestLiveHistoryClearedOnTabSwitch(t *testing.T) {
	base := strings.Repeat("x", 1<<16)
	a := historyTestApp(t, base)
	historyTestEdit(a, base+"甲", 1, 1, editInput)
	historyTestEdit(a, base+"甲乙", 2, 2, editIME)
	a.switchTab(1)
	a.switchTab(0)
	if len(a.liveUndo) != 0 || len(a.liveRedo) != 0 {
		t.Fatalf("切回后不应保留撤销历史: undo=%d redo=%d", len(a.liveUndo), len(a.liveRedo))
	}
	a.undoLive(false)
	if a.tabs[0].Text != base+"甲乙" {
		t.Fatal("无历史时撤销不得改动文本")
	}
	historyTestEdit(a, base+"甲乙丙", 3, 3, editInput)
	a.undoLive(false)
	if a.tabs[0].Text != base+"甲乙" {
		t.Fatal("切回后的新编辑应能正常撤销")
	}
}

func TestLiveHistoryFullReplacementKeepsSharing(t *testing.T) {
	before := strings.Repeat("x", 1<<16)
	for _, after := range []string{strings.Repeat("y", 1<<16), ""} {
		e := liveEdit{before: liveSnapshot{doc: before}, after: liveSnapshot{doc: after}}
		if e.compressed() != e {
			t.Fatal("全文替换不应克隆两个完整差分而放大内存")
		}
	}
}

func TestLiveHistoryClonesByteFragments(t *testing.T) {
	before := strings.Repeat("前", 30000) + "甲" + strings.Repeat("后", 30000)
	after := strings.Repeat("前", 30000) + "申" + strings.Repeat("后", 30000)
	e := liveEdit{before: liveSnapshot{doc: before}, after: liveSnapshot{doc: after}}.compressed()
	if e.delta == nil || e.before.doc != "" || e.after.doc != "" {
		t.Fatal("compression retains snapshot")
	}
	// 甲/申 共用前两个字节，差分落在 rune 内也必须原样恢复。
	if utf8.ValidString(e.delta.removed) || utf8.ValidString(e.delta.inserted) {
		t.Fatal("fixture did not put byte boundary inside rune")
	}
	for _, fragment := range []string{e.delta.removed, e.delta.inserted} {
		if len(fragment) > 3 {
			t.Fatal("small edit retained large fragment")
		}
		p := uintptr(unsafe.Pointer(unsafe.StringData(fragment)))
		for _, doc := range []string{before, after} {
			start := uintptr(unsafe.Pointer(unsafe.StringData(doc)))
			if p >= start && p < start+uintptr(len(doc)) {
				t.Fatal("fragment aliases full document backing storage")
			}
		}
	}
	expanded, ok := e.expanded(after, false)
	if !ok || expanded.before.doc != before {
		t.Fatal("byte delta failed undo")
	}
	expanded, ok = e.expanded(before, true)
	if !ok || expanded.after.doc != after {
		t.Fatal("byte delta failed redo")
	}
	small := liveEdit{before: liveSnapshot{doc: "甲"}, after: liveSnapshot{doc: "乙"}}
	if small.compressed() != small {
		t.Fatal("small snapshot changed representation")
	}
}

// TestLiveHistoryKeptWhenOtherTabsChange 删除其他标签或排序不改正在编辑的
// 文档，撤销历史照常可用。
func TestLiveHistoryKeptWhenOtherTabsChange(t *testing.T) {
	a := historyTestApp(t, "甲")
	a.cfg.Source = false
	historyTestEdit(a, "甲乙", 2, 2, editInput)
	if !a.deleteTabNow(len(a.tabs) - 1) {
		t.Fatal("删除其他标签失败")
	}
	if !a.moveTab(0, 1) {
		t.Fatal("移动当前标签失败")
	}
	if len(a.liveUndo) != 1 {
		t.Fatalf("其他标签变化不应清空当前历史: %d", len(a.liveUndo))
	}
	a.undoLive(false)
	if a.activeTab().Text != "甲" {
		t.Fatalf("撤销应回到编辑前: %q", a.activeTab().Text)
	}
}
