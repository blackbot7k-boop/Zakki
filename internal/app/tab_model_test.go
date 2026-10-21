package app

// 标签数据模型、液态动画几何与快捷键/设置页行为测试。
// UI 部分用 ui.NewTester 无窗口驱动；动画经 app.animNow 注入假时钟。

import (
	"fmt"
	"image/color"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"zakki/locales"
)

// ---- 数据模型：新增/删除/排序/命名 ----

func TestTabAddLimitAndFocus(t *testing.T) {
	a := newTestApp(t)
	t.Cleanup(a.shutdown)
	for len(a.tabs) < MaxTabCount {
		if !a.addTab() {
			t.Fatalf("第 %d 个标签新增失败", len(a.tabs)+1)
		}
	}
	if len(a.tabs) != MaxTabCount {
		t.Fatalf("标签数 = %d, want %d", len(a.tabs), MaxTabCount)
	}
	last := a.tabs[len(a.tabs)-1]
	if a.active != MaxTabCount-1 {
		t.Fatalf("新增后应切换到新标签，active=%d", a.active)
	}
	if last.ID == 0 || a.nextTabID <= last.ID {
		t.Fatalf("新标签 ID = %d, nextTabID = %d，应递增且不重复", last.ID, a.nextTabID)
	}
	if last.Text != "" {
		t.Fatalf("新增标签应为空: %q", last.Text)
	}
	// 颜色不与现有标签重复（12 色调色板够 12 个标签各一色）。
	seen := map[string]bool{}
	for i := range a.tabs {
		c := a.tabs[i].Color
		if !validTabColor(c) {
			t.Fatalf("标签 %d 颜色非法: %q", i, c)
		}
		if seen[c] {
			t.Fatalf("新增分配了重复颜色: %q (%+v)", c, a.tabs)
		}
		seen[c] = true
	}
	if a.addTab() {
		t.Fatal("达到上限后 addTab 应失败")
	}
	if !strings.Contains(a.notice, "最多") {
		t.Fatalf("上限提示 = %q", a.notice)
	}
}

func TestTabDeleteMinAndBackup(t *testing.T) {
	t.Run("至少保留一个标签", func(t *testing.T) {
		a := newTestApp(t)
		t.Cleanup(a.shutdown)
		for len(a.tabs) > 1 {
			a.tabs = a.tabs[:len(a.tabs)-1]
		}
		a.active = 0
		a.tabs[0].Text = "唯一"
		if a.deleteTabNow(0) {
			t.Fatal("最后一个标签不应可删除")
		}
		if len(a.tabs) != 1 {
			t.Fatalf("标签被删除: %d", len(a.tabs))
		}
	})

	t.Run("有内容的标签先备份后删除", func(t *testing.T) {
		a := newTestApp(t)
		t.Cleanup(a.shutdown)
		a.tabs[1].Text = "要被删除的内容"
		if !a.deleteTabNow(1) {
			t.Fatal("删除失败")
		}
		if len(a.tabs) != DefaultTabCount-1 {
			t.Fatalf("标签数 = %d, want %d", len(a.tabs), DefaultTabCount-1)
		}
		for i := range a.tabs {
			if a.tabs[i].Text == "要被删除的内容" {
				t.Fatal("内容还在标签里")
			}
		}
		names := persistBackupNames(t, a.notes.BackupDir())
		if len(names) != 1 {
			t.Fatalf("备份 = %v, want 1 份", names)
		}
		if bak := persistReadSaved(t, a.notes.BackupDir()+"/"+names[0]); bak.Tabs[1].Text != "要被删除的内容" {
			t.Fatalf("备份不含被删内容: %+v", bak.Tabs)
		}
	})

	t.Run("备份失败取消删除", func(t *testing.T) {
		a := newTestApp(t)
		t.Cleanup(a.shutdown)
		blocker := a.notes.BackupDir() + "-blocked"
		persistWrite(t, blocker, "普通文件")
		a.notes.SetBackupDir(blocker) // 选择的备份文件夹不可写
		a.tabs[1].Text = "备份失败要保留"
		if a.deleteTabNow(1) {
			t.Fatal("备份失败应取消删除")
		}
		if len(a.tabs) != DefaultTabCount || a.tabs[1].Text != "备份失败要保留" {
			t.Fatalf("删除未被取消: %+v", a.tabs)
		}
	})

	t.Run("删除修正 active", func(t *testing.T) {
		a := newTestApp(t)
		t.Cleanup(a.shutdown)
		a.active = DefaultTabCount - 1
		if !a.deleteTabNow(a.active) {
			t.Fatal("删除失败")
		}
		if a.active != len(a.tabs)-1 {
			t.Fatalf("active = %d, want %d", a.active, len(a.tabs)-1)
		}
	})
}

func TestDeleteConfirmedTabUsesStableIDAndRequiresOpenWindow(t *testing.T) {
	a := newTestApp(t)
	t.Cleanup(a.shutdown)
	targetID := a.tabs[1].ID
	a.tabs[1].Text = ""
	before := len(a.tabs)

	action := a.confirmedDeleteAction(targetID)
	a.winOpen = false
	action()
	if len(a.tabs) != before || a.tabIndexByID(targetID) < 0 {
		t.Fatal("窗口已关闭时不应执行确认删除")
	}

	if !a.moveTab(1, 1) {
		t.Fatal("准备稳定 ID 场景时移动标签失败")
	}
	a.winOpen = true
	action() // 模拟 runOnMain 在 editor lane 执行已投递 action。
	if len(a.tabs) != before-1 {
		t.Fatalf("确认删除后标签数 = %d，要 %d", len(a.tabs), before-1)
	}
	if a.tabIndexByID(targetID) >= 0 {
		t.Fatal("重排后应按稳定 ID 删除原目标")
	}
}

func TestSourceSessionSurvivesAddAndCurrentDelete(t *testing.T) {
	a := newTestApp(t)
	t.Cleanup(a.shutdown)
	a.cfg.Source = true
	a.active = len(a.tabs) - 1
	originalID := a.activeTab().ID
	originalUndo := liveEdit{
		before: liveSnapshot{doc: "before", start: 2, end: 2},
		after:  liveSnapshot{doc: "after", start: 5, end: 5},
		kind:   editInput,
	}
	a.srcCaret, a.srcSelEnd = 2, 5
	a.srcSelSyncStart, a.srcSelSyncEnd = 2, 5
	a.srcPrev = liveSnapshot{doc: "before", start: 2, end: 2}
	a.liveUndo = []liveEdit{originalUndo}

	originalIndex := a.active
	a.switchTab(originalIndex - 1)
	if a.srcCaret != 0 || a.srcSelEnd != 0 || len(a.liveUndo) != 0 {
		t.Fatalf("首次切换目标继承了源码会话: caret=%d end=%d undo=%d", a.srcCaret, a.srcSelEnd, len(a.liveUndo))
	}
	a.srcCaret, a.srcSelEnd = 7, 7
	a.liveUndo = []liveEdit{{before: liveSnapshot{doc: "other"}, kind: editInput}}
	a.switchTab(originalIndex)
	if a.srcCaret != 2 || a.srcSelEnd != 5 || len(a.liveUndo) != 1 || a.liveUndo[0] != originalUndo {
		t.Fatalf("切回原标签未恢复源码会话: caret=%d end=%d undo=%+v", a.srcCaret, a.srcSelEnd, a.liveUndo)
	}

	if !a.addTab() {
		t.Fatal("新增标签失败")
	}
	addedID := a.activeTab().ID
	if addedID == originalID {
		t.Fatal("新增标签未切换到新 ID")
	}
	if a.srcCaret != 0 || a.srcSelEnd != 0 || len(a.liveUndo) != 0 {
		t.Fatalf("新标签继承了源码会话: caret=%d end=%d undo=%d", a.srcCaret, a.srcSelEnd, len(a.liveUndo))
	}
	saved := a.srcSessions[originalID]
	if saved.caret != 2 || saved.selEnd != 5 || len(saved.undo) != 1 || saved.undo[0] != originalUndo {
		t.Fatalf("新增前未保存原标签源码会话: %+v", saved)
	}

	a.srcCaret, a.srcSelEnd = 1, 1
	a.liveUndo = []liveEdit{{before: liveSnapshot{doc: "new"}, kind: editInput}}
	if !a.deleteTabNow(a.active) {
		t.Fatal("删除当前新增标签失败")
	}
	if a.activeTab().ID != originalID {
		t.Fatalf("删除后活动标签 ID = %d, want %d", a.activeTab().ID, originalID)
	}
	if _, ok := a.srcSessions[addedID]; ok {
		t.Fatal("已删除标签的源码会话未清理")
	}
	if a.srcCaret != 2 || a.srcSelEnd != 5 || len(a.liveUndo) != 1 || a.liveUndo[0] != originalUndo {
		t.Fatalf("删除当前标签后未恢复目标会话: caret=%d end=%d undo=%+v", a.srcCaret, a.srcSelEnd, a.liveUndo)
	}
}

func TestTabRenameAndMove(t *testing.T) {
	a := newTestApp(t)
	t.Cleanup(a.shutdown)

	a.renameTab(0, "  工作笔记  ")
	if a.tabs[0].Name != "工作笔记" {
		t.Fatalf("名称未去空白: %q", a.tabs[0].Name)
	}
	if a.displayTitle(0) != "工作笔记" || a.menuTitle(0) != "工作笔记" {
		t.Fatalf("displayTitle/menuTitle = %q/%q", a.displayTitle(0), a.menuTitle(0))
	}
	a.renameTab(0, "   ")
	if a.tabs[0].Name != "" || a.displayTitle(0) != "" {
		t.Fatalf("空白名称应等于未命名: %q", a.tabs[0].Name)
	}
	if a.menuTitle(0) != "标签 1" {
		t.Fatalf("未命名菜单名 = %q", a.menuTitle(0))
	}

	// 排序：active 按 ID 跟随。
	a.active = 1 // 标签 2
	if !a.moveTab(1, -1) {
		t.Fatal("上移失败")
	}
	if a.tabs[0].ID != 2 {
		t.Fatalf("上移后槽位 0 = ID %d, want 2", a.tabs[0].ID)
	}
	if a.active != 0 {
		t.Fatalf("active 应跟随被移动的标签，得到 %d", a.active)
	}
	if a.moveTab(0, -1) {
		t.Fatal("越界上移应失败")
	}
	if a.moveTab(len(a.tabs)-1, 1) {
		t.Fatal("越界下移应失败")
	}
}

func TestTabColor(t *testing.T) {
	a := newTestApp(t)
	t.Cleanup(a.shutdown)
	if got := a.tabColor(0); got != ui.Hex(tabPaletteColors[0]) {
		t.Fatalf("默认色 = %+v", got)
	}
	a.setTabColor(0, "#00A3B4")
	if a.tabs[0].Color != "#00a3b4" {
		t.Fatalf("颜色未规范化小写: %q", a.tabs[0].Color)
	}
	if got := a.tabColor(0); got != ui.Hex("#00a3b4") {
		t.Fatalf("自定义色未生效: %+v", got)
	}
	a.setTabColor(0, "not-a-color")
	if a.tabs[0].Color != "#00a3b4" {
		t.Fatal("非法颜色不应写入")
	}
	// 无效颜色回退调色板。
	a.tabs[1].Color = "bogus"
	if got := a.tabColor(1); got != ui.Hex(tabPaletteColors[1]) {
		t.Fatalf("无效颜色应回退调色板: %+v", got)
	}
	if a.tabAccent(0, false) != a.tabColor(0).Mix(ui.RGB(0, 0, 0), 0.30) {
		t.Fatalf("自定义色 accent 应向黑收敛: %+v", a.tabAccent(0, false))
	}
}

// ---- 快照深拷贝 ----

func TestTabSnapshotDeepCopy(t *testing.T) {
	a := newTestApp(t)
	t.Cleanup(a.shutdown)
	a.tabs[0].Text = "快照时的内容"
	a.tabs[0].Name = "快照"
	a.active = 2
	snap := a.snapshotFile()

	// 后台保存期间主线程继续改状态：已排队快照不受影响。
	a.tabs[0].Text = "保存期间的新改动"
	a.tabs[0].Name = "改过的名字"
	a.tabs = append(a.tabs, tab{ID: 999, Text: "新增"})
	a.active = 0

	if snap.Tabs[0].Text != "快照时的内容" || snap.Tabs[0].Name != "快照" {
		t.Fatalf("快照被后续修改污染: %+v", snap.Tabs[0])
	}
	if len(snap.Tabs) != DefaultTabCount || snap.Active != 2 {
		t.Fatalf("快照结构被污染: len=%d active=%d", len(snap.Tabs), snap.Active)
	}
	if snap.Tabs[0].Text != "快照时的内容" || snap.Active != 2 || snap.Version != currentDataVersion {
		t.Fatalf("磁盘格式转换丢失: %+v", snap)
	}
	if snap.Dots != nil {
		t.Fatalf("新快照不应携带 v1 dots 字段: %v", snap.Dots)
	}
}

// ---- 液态动画几何（纯函数） ----

func TestLiquidGeometry(t *testing.T) {
	// 未命名：每个标记一样宽。有名字：当前槽是色条，其余仍是圆点。
	widths := equalTabWidths(3)
	slots, total := layoutTabSlots(widths)
	if total != dotDiameter*3+dotGap*2 {
		t.Fatalf("总宽 = %g", total)
	}
	named := tabSlotWidths(pillWidthOf(100), 3, 1)
	slots, total = layoutTabSlots(named)
	if total != pillWidthOf(100)+dotDiameter*2+dotGap*2 {
		t.Fatalf("命名后总宽 = %g", total)
	}
	if slots[1].w != pillWidthOf(100) || slots[0].w != dotDiameter {
		t.Fatalf("命名槽 = %+v", slots)
	}
	if d := slots[1].cx - slots[0].cx; d != dotDiameter/2+dotGap+pillWidthOf(100)/2 {
		t.Fatalf("相邻中心距 = %g", d)
	}

	// 缓动：起点 0、终点 1、中段轻微回弹（>1）。
	if e := easeLiquid(0); e != 0 {
		t.Fatalf("easeLiquid(0) = %g", e)
	}
	if e := easeLiquid(1); e != 1 {
		t.Fatalf("easeLiquid(1) = %g", e)
	}
	if e := easeLiquid(0.7); e <= 1 || e > 1.1 {
		t.Fatalf("easeLiquid(0.7) = %g, want 轻微回弹 (1, 1.1]", e)
	}
	if e := easeLiquid(1.5); e != 1 {
		t.Fatalf("进度超界应收敛到 1: %g", e)
	}

	// 插值：线性中点。
	from := []tabSlotGeom{{w: 18, cx: 9}, {w: 18, cx: 41}}
	to := []tabSlotGeom{{w: 18, cx: 41}, {w: 153, cx: 136.5}}
	cur := interpSlots(from, to, 0.5)
	if cur[0].cx != 25 || cur[1].cx != 88.75 || cur[1].w != 85.5 {
		t.Fatalf("插值中点 = %+v", cur)
	}

	// 桥接：间隙越大桥越细，静止间距不粘连、贴合时最粗。
	if w := bridgeWidthFor(bridgeReach); w != 0 {
		t.Fatalf("达到触达距离应无桥: %g", w)
	}
	if w := bridgeWidthFor(14); w != 0 {
		t.Fatalf("静止间距 14 不应粘连: %g", w)
	}
	w0, w5 := bridgeWidthFor(0), bridgeWidthFor(5)
	if w0 <= w5 || w5 <= 0 || w0 > pillHeight {
		t.Fatalf("桥厚单调性错误: %g vs %g", w0, w5)
	}
	if g := edgeGap(0, 11, 32, 9); g != 12 {
		t.Fatalf("edgeGap = %g, want 12", g)
	}

	// 标题淡出/淡入：起点全显旧标题，中段都淡，终点全显新标题。
	fa, ta := titleAlphas(0)
	if fa != 1 || ta != 0 {
		t.Fatalf("titleAlphas(0) = %g/%g", fa, ta)
	}
	fa, ta = titleAlphas(0.45)
	if fa != 0 || ta != 0 {
		t.Fatalf("桥接阶段标题应都淡出: %g/%g", fa, ta)
	}
	fa, ta = titleAlphas(1)
	if fa != 0 || ta != 1 {
		t.Fatalf("titleAlphas(1) = %g/%g", fa, ta)
	}

	// 胶囊宽度：未命名 = 圆点；命名含标题区。
	if pillWidthOf(0) != pillHeight {
		t.Fatalf("未命名胶囊宽 = %g", pillWidthOf(0))
	}
	if pillWidthOf(120) != 120+2*pillPadX {
		t.Fatalf("命名胶囊宽 = %g", pillWidthOf(120))
	}

	// 胶囊上的文字按对比度选色：黄色配深字，蓝色配白字。
	if got := pillTextColor(ui.Hex("#ffd01c")); got != ui.RGB(28, 28, 30) {
		t.Fatalf("浅色胶囊文字 = %+v, want 深色", got)
	}
	if got := pillTextColor(ui.Hex("#0f75e9")); got != ui.RGB(255, 255, 255) {
		t.Fatalf("深色胶囊文字 = %+v, want 白色", got)
	}
}

// ---- 切换动画（UI + 假时钟） ----

type fakeAnimClock struct {
	cur time.Time
}

func (c *fakeAnimClock) now() time.Time { return c.cur }
func (c *fakeAnimClock) advance(d time.Duration) {
	c.cur = c.cur.Add(d)
}

func newAnimTestApp(t *testing.T) (*app, *ui.Tester, *fakeAnimClock) {
	t.Helper()
	a := newTestApp(t)
	t.Cleanup(a.shutdown)
	a.tabs[1].Text = "内容乙"
	clk := &fakeAnimClock{cur: time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)}
	a.animNow = clk.now
	tt := ui.NewTester(a.view, 460, 540)
	return a, tt, clk
}

func TestTabMorphAnimation(t *testing.T) {
	a, tt, clk := newAnimTestApp(t)
	dur := a.tabAnimDuration()

	// 相邻右切：动画开始，目标是标签 2。
	if err := tt.Click(fmt.Sprintf("标签 %d", 2)); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !a.morph.active || a.morph.toID != a.tabs[1].ID {
		t.Fatalf("切换应开始动画: %+v", a.morph)
	}
	if a.active != 1 {
		t.Fatalf("状态应立即切换: active=%d", a.active)
	}
	if a.morph.fromID != a.tabs[0].ID {
		t.Fatalf("动画起点 = ID %d", a.morph.fromID)
	}

	// 结束后：动画清空、目标胶囊唯一。
	clk.advance(dur + time.Millisecond)
	tt.Frame()
	if a.morph.active {
		t.Fatal("动画应结束")
	}
	_ = a.morph // 结束后 morph 归零，当前状态只剩目标胶囊
	if a.active != 1 {
		t.Fatalf("结束后 active = %d", a.active)
	}
}

func TestTabMorphTakeoverMidFlight(t *testing.T) {
	a, tt, clk := newAnimTestApp(t)
	dur := a.tabAnimDuration()

	if err := tt.Click(fmt.Sprintf("标签 %d", 2)); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	clk.advance(dur / 4)
	tt.Frame() // 动画进行到 1/4
	sampledBody := a.morph.bodyAt(clk.now())
	sampledSlots := interpSlots(a.morph.from, a.morph.to, easeLiquid(a.morph.progress(clk.now())))

	// 快速连续切换：从当前显示几何接管到新目标。
	if err := tt.Click(fmt.Sprintf("标签 %d", 3)); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !a.morph.active || a.morph.toID != a.tabs[2].ID {
		t.Fatalf("接管失败: %+v", a.morph)
	}
	if a.morph.fromBody != sampledBody {
		t.Fatalf("接管移动体跳变: from=%+v sampled=%+v", a.morph.fromBody, sampledBody)
	}
	for i, slot := range sampledSlots {
		if a.morph.from[i] != slot {
			t.Fatalf("接管槽位 %d 跳变", i)
		}
	}
	clk.advance(dur + time.Millisecond)
	tt.Frame()
	if a.morph.active {
		t.Fatal("接管后的动画应正常结束")
	}
	if a.active != 2 {
		t.Fatalf("active = %d, want 2", a.active)
	}
}

func TestTabMorphLeftAndCross(t *testing.T) {
	a, tt, clk := newAnimTestApp(t)
	dur := a.tabAnimDuration()

	// 先切到 5，再向左切回 2（跨 2 项）。
	if err := tt.Click(fmt.Sprintf("标签 %d", 5)); err != nil {
		t.Fatal(err)
	}
	clk.advance(dur + time.Millisecond)
	tt.Frame()
	if err := tt.Click(fmt.Sprintf("标签 %d", 2)); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !a.morph.active {
		t.Fatal("左切换应开始动画")
	}
	slotOf := func(id uint64, slots []tabSlotGeom, ids []uint64) float32 {
		for i, mid := range ids {
			if mid == id {
				return slots[i].cx
			}
		}
		return -1
	}
	fromCX := slotOf(a.morph.fromID, a.morph.from, a.morph.ids)
	toCX := slotOf(a.morph.toID, a.morph.to, a.morph.ids)
	if fromCX <= toCX {
		t.Fatalf("向左切换目标应更靠左: from=%g to=%g", fromCX, toCX)
	}
	clk.advance(dur + time.Millisecond)
	tt.Frame()
	if a.morph.active || a.active != 1 {
		t.Fatalf("动画未正常收尾: active=%d morph=%+v", a.active, a.morph)
	}
}

// 回归：回弹缓动会把飞行胶囊推过目标约 5%，绘制被裁剪在标签栏元素矩形内，
// 没有端部余量时胶囊会在边缘标签处消失（切换到第 1/最后一个标签时最明显）。
func TestTabMorphStaysVisibleAtOvershoot(t *testing.T) {
	a := newTestApp(t)
	t.Cleanup(a.shutdown)
	for i := range a.tabs {
		a.tabs[i].Text = ""
	}
	clk := &fakeAnimClock{cur: time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)}
	a.animNow = clk.now
	tt := ui.NewTester(a.view, 460, 540)
	dur := a.tabAnimDuration()

	// 先切到 7 并让动画结束，再从 7 向左跨切回 1：全程 192px，
	// 最大回弹（t≈0.64，e≈1.053）时胶囊中心越过目标槽中心约 10px，
	// 超出标签栏元素左缘——没有端部余量时胶囊左半被裁剪消失。
	if err := tt.Click(fmt.Sprintf("标签 %d", 7)); err != nil {
		t.Fatal(err)
	}
	clk.advance(dur + time.Millisecond)
	tt.Frame()
	if err := tt.Click(fmt.Sprintf("标签 %d", 1)); err != nil {
		t.Fatal(err)
	}
	clk.advance(dur * 64 / 100)
	tt.Frame()
	if !a.morph.active {
		t.Fatal("应处于动画中")
	}

	background := a.chromeColor(headerBackground(false), false, false)
	r0, ok := tt.Find(fmt.Sprintf("标签 %d", 1))
	if !ok {
		t.Fatal("missing tab 1 slot")
	}
	// 采样飞行中的圆点中心：回弹会越过目标槽，裁掉后这里只剩背景。
	restCX := r0.X + r0.W/2
	flyCX := restCX + (a.morph.bodyAt(clk.now()).cx - a.morph.toBody.cx)
	pix := color.NRGBAModel.Convert(tt.Image().At(int(flyCX), int(r0.Y+r0.H/2))).(color.NRGBA)
	isBackground := pix.R == background.R && pix.G == background.G && pix.B == background.B
	if isBackground {
		t.Fatalf("回弹点胶囊被裁剪消失: pixel=%+v background=%+v", pix, background)
	}

	// 动画正常收尾：目标槽恢复静态实心圆。
	clk.advance(dur)
	tt.Frame()
	if a.morph.active {
		t.Fatal("动画应结束")
	}
}

func TestTabMorphRespectsReduceMotion(t *testing.T) {
	a := newTestApp(t)
	t.Cleanup(a.shutdown)
	a.tabs[1].Text = "乙"
	tt := ui.NewTester(a.view, 460, 540)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	if err := tt.Click(fmt.Sprintf("标签 %d", 2)); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.morph.active {
		t.Fatal("减少动态效果时不应开始动画")
	}
	if a.active != 1 {
		t.Fatalf("active = %d", a.active)
	}
}

// ---- 快捷键 ----

func TestTabShortcuts(t *testing.T) {
	a := newTestApp(t)
	t.Cleanup(a.shutdown)
	for i := range a.tabs {
		a.tabs[i].Text = fmt.Sprintf("内容 %d", i+1)
	}
	tt := ui.NewTester(a.view, 460, 540)

	// ⌘1–⌘9 切换（主修饰键+数字，两平台同形）。
	tt.Key(ui.Super, ui.Key3)
	tt.Frame()
	if a.active != 2 {
		t.Fatalf("⌘3 后 active=%d", a.active)
	}
	tt.Key(ui.Super, ui.Key1)
	tt.Frame()
	if a.active != 0 {
		t.Fatalf("⌘1 后 active=%d", a.active)
	}

	// ⌘⌥←/→ 循环。
	tt.Key(ui.Super|ui.Alt, ui.KeyRight)
	tt.Frame()
	if a.active != 1 {
		t.Fatalf("⌘⌥→ 后 active=%d", a.active)
	}
	tt.Key(ui.Super|ui.Alt, ui.KeyLeft)
	tt.Frame()
	if a.active != 0 {
		t.Fatalf("⌘⌥← 后 active=%d", a.active)
	}
	// 循环：第 1 个再往左回到最后一个。
	tt.Key(ui.Super|ui.Alt, ui.KeyLeft)
	tt.Frame()
	if a.active != len(a.tabs)-1 {
		t.Fatalf("循环左应到最后一个，active=%d", a.active)
	}

	// ⌘T 新增并切换。
	before := len(a.tabs)
	tt.Key(ui.Super, ui.KeyT)
	tt.Frame()
	if len(a.tabs) != before+1 || a.active != before {
		t.Fatalf("⌘T 后 len=%d active=%d", len(a.tabs), a.active)
	}

	// ⌘R 打开重命名，输入后 Enter 提交。
	tt.Key(ui.Super, ui.Key1)
	tt.Frame()
	tt.Key(ui.Super, ui.KeyR)
	tt.Frame()
	if a.renaming != a.tabs[0].ID {
		t.Fatalf("⌘R 应进入重命名: renaming=%d", a.renaming)
	}
	tt.Type("新名字")
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	if a.renaming != 0 || a.tabs[0].Name != "新名字" {
		t.Fatalf("Enter 应提交重命名: renaming=%d name=%q", a.renaming, a.tabs[0].Name)
	}

	// Escape 取消。
	tt.Key(ui.Super, ui.KeyR)
	tt.Frame()
	t.Logf("rename focused=%v renaming=%d", tt.Focused("重命名标签"), a.renaming)
	tt.Type("草稿")
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if a.renaming != 0 || a.tabs[0].Name != "新名字" {
		t.Fatalf("Escape 应取消: renaming=%d name=%q", a.renaming, a.tabs[0].Name)
	}
	// 取消后内容编辑不受影响。
	tt.ClickAt(200, 300)
	tt.Type("X")
	if !strings.Contains(a.tabs[0].Text, "X") {
		t.Fatalf("取消重命名后编辑失效: %q", a.tabs[0].Text)
	}
}

// ---- 标签与外观设置页 ----

func TestTabsSettingsPage(t *testing.T) {
	a := newTestApp(t)
	t.Cleanup(a.shutdown)
	a.tabs[1].Name = "乙名"
	a.settingsPage = pageTabs
	tt := ui.NewTester(a.settingsView, 760, 600)

	if !tt.HasText("新建标签") {
		t.Fatalf("应有新建入口，texts=%q", tt.Texts())
	}
	for i := range a.tabs {
		if !tt.HasText(a.menuTitle(i)) {
			t.Fatalf("列表缺少 %s，texts=%q", a.menuTitle(i), tt.Texts())
		}
	}
	// 外观区控件已迁到「外观」页，标签页只剩列表。
	for _, gone := range []string{"标签标题最大宽度", "切换动画速度", "毛玻璃回退强度"} {
		if tt.HasText(gone) {
			t.Fatalf("标签页不应再有 %q（已迁外观页）, texts=%q", gone, tt.Texts())
		}
	}

	// 重命名输入：即时保存（走设置页行内重命名会话）。
	a.editSettingsTabName(t, tt, 2, "改名乙")
	if a.tabs[1].Name != "改名乙" {
		t.Fatalf("设置页重命名未保存: %q", a.tabs[1].Name)
	}

	// 删除空标签（无确认）。
	before := len(a.tabs)
	a.deleteTabNow(5)
	if len(a.tabs) != before-1 {
		t.Fatalf("删除后标签数 = %d", len(a.tabs))
	}

	// 上移/下移：active 跟随。
	a.active = 1
	if !a.moveTab(1, -1) {
		t.Fatal("上移失败")
	}
	if a.tabs[0].Name != "改名乙" || a.active != 0 {
		t.Fatalf("移动后 %+v active=%d", a.tabs[0], a.active)
	}

	// 新增达到上限时按钮仍可用但 addTab 拒绝。
	for len(a.tabs) < MaxTabCount {
		a.addTab()
	}
	if a.addTab() {
		t.Fatal("上限应拒绝新增")
	}
}

func TestTabsSettingsDeleteEmptyTabThroughUI(t *testing.T) {
	a := newTestApp(t)
	t.Cleanup(a.shutdown)
	a.settingsPage = pageTabs
	target := 1
	a.tabs[target].Text = ""
	targetID := a.tabs[target].ID
	before := len(a.tabs)
	tt := ui.NewTester(a.settingsView, 760, 600)

	// 删除入口是行尾的图标按钮，可达名仍是「删除 〈标签名〉」。
	if err := tt.Click(locales.TabDelete(a.menuTitle(target))); err != nil {
		t.Fatal(err)
	}
	if len(a.tabs) != before-1 {
		t.Fatalf("点击删除后标签数 = %d，要 %d", len(a.tabs), before-1)
	}
	if a.tabIndexByID(targetID) >= 0 {
		t.Fatal("点击删除后目标标签仍存在")
	}
}

func TestTabsSettingsRowUIState(t *testing.T) {
	a := newTestApp(t)
	t.Cleanup(a.shutdown)
	a.settingsPage = pageTabs
	a.tabs[0].Color = tabPaletteColors[0]
	tt := ui.NewTester(a.settingsView, 760, 600)
	tt.Frame()

	// 旧控件退场：颜色按钮、上/下移与 ⌘N 提示都不再渲染（用元素精确查找，
	// 避免命中提示文案里的子串）。
	for _, gone := range []string{"颜色", "↑", "↓"} {
		if _, ok := tt.Find(gone); ok {
			t.Fatalf("设置页不应再有 %q 元素, texts=%q", gone, tt.Texts())
		}
	}
	if tt.HasText("⌘1") {
		t.Fatalf("设置页不应再有 ⌘N 提示, texts=%q", tt.Texts())
	}
	// 取色器色块以颜色为可达名，草稿每帧从标签色播种。
	if _, ok := tt.Find(locales.TabColorOf(a.tabs[0].Color)); !ok {
		t.Fatalf("找不到颜色色块 %q, texts=%q", locales.TabColorOf(a.tabs[0].Color), tt.Texts())
	}
	if a.settingsUI.tabWellColors[0] != ui.Hex(tabPaletteColors[0]) {
		t.Fatalf("取色器草稿未播种: %+v", a.settingsUI.tabWellColors[0])
	}

	// 双击名称重命名：会话进入后输入框出现，提交/取消走同一存储。
	a.beginSettingsRename(0)
	tt.Frame()
	if a.settingsUI.tabRenaming != a.tabs[0].ID || !tt.HasText(locales.RenameTabField()) {
		t.Fatalf("应进入行内重命名: renaming=%d", a.settingsUI.tabRenaming)
	}
	a.settingsUI.tabRenameDraft = "改名甲" // 与输入框同一存储，等价键入
	a.commitSettingsRename()
	tt.Frame()
	if a.tabs[0].Name != "改名甲" || a.settingsUI.tabRenaming != 0 {
		t.Fatalf("提交重命名失败: name=%q renaming=%d", a.tabs[0].Name, a.settingsUI.tabRenaming)
	}
	a.beginSettingsRename(0)
	a.settingsUI.tabRenameDraft = "草稿"
	a.cancelSettingsRename()
	if a.tabs[0].Name != "改名甲" || a.settingsUI.tabRenaming != 0 {
		t.Fatalf("取消不应落盘: name=%q renaming=%d", a.tabs[0].Name, a.settingsUI.tabRenaming)
	}
}

// TestReorderTab 拖拽排序语义：to 是「移到哪一行之前」，目标在后时
// 剔除被移行再折算插入点；active 按稳定 ID 跟随。
func TestReorderTab(t *testing.T) {
	a := newTestApp(t)
	t.Cleanup(a.shutdown)
	for len(a.tabs) < 4 {
		a.addTab()
	}
	ids := []uint64{a.tabs[0].ID, a.tabs[1].ID, a.tabs[2].ID, a.tabs[3].ID}

	// 中间行拖到末尾：to=4 → 新下标 3。
	a.active = 1
	if !a.reorderTab(1, 4) {
		t.Fatal("拖到末尾应成功")
	}
	if a.tabs[3].ID != ids[1] || a.active != 3 {
		t.Fatalf("拖到末尾失败: order=%v active=%d", a.tabs, a.active)
	}
	// 末尾行拖回队首：to=0。
	if !a.reorderTab(3, 0) {
		t.Fatal("拖到队首应成功")
	}
	if a.tabs[0].ID != ids[1] || a.active != 0 {
		t.Fatalf("拖回队首失败: order=%v active=%d", a.tabs, a.active)
	}
	// 原位与越界：静默拒绝。
	if a.reorderTab(0, 1) || a.reorderTab(-1, 0) || a.reorderTab(0, len(a.tabs)+1) {
		t.Fatal("原位/越界拖拽应拒绝")
	}
}

// editSettingsTabName 在设置页里重命名某标签：走行内重命名会话
// （双击名称进入后的同一存储与提交路径）。
func (a *app) editSettingsTabName(t *testing.T, tt *ui.Tester, index1 int, name string) {
	t.Helper()
	a.beginSettingsRename(index1 - 1)
	tt.Frame()
	a.settingsUI.tabRenameDraft = name // 与输入框同一存储，等价键入
	a.commitSettingsRename()
	tt.Frame()
}

func TestTabsSettingsPageOpensFromState(t *testing.T) {
	a := newTestApp(t)
	t.Cleanup(a.shutdown)
	// 事件循环未启动时 openSettings 只路由状态（不创建原生窗口）。
	a.openSettings(pageTabs)
	if a.settingsPage != pageTabs {
		t.Fatalf("settingsPage = %q", a.settingsPage)
	}
	a.openSettings(pageGeneral)
	if a.settingsPage != pageGeneral {
		t.Fatalf("settingsPage = %q", a.settingsPage)
	}
}

// ---- 重命名会话与编辑共存 ----

func TestTabRenameSessionKeepsEditor(t *testing.T) {
	a := newTestApp(t)
	t.Cleanup(a.shutdown)
	a.tabs[0].Text = "第一段"
	tt := ui.NewTester(a.view, 460, 540)

	a.beginRename(0)
	tt.Frame()
	tt.Type("甲")
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	if a.tabs[0].Name != "甲" {
		t.Fatalf("重命名未提交: %q", a.tabs[0].Name)
	}
	if a.renaming != 0 {
		t.Fatal("提交后应退出重命名")
	}
	// 提交后点击内容继续编辑。
	tt.ClickAt(200, 300)
	tt.Type("续")
	if !strings.HasPrefix(a.tabs[0].Text, "第一段") || !strings.Contains(a.tabs[0].Text, "续") {
		t.Fatalf("重命名后编辑异常: %q", a.tabs[0].Text)
	}
	// 再次进入重命名并取消，编辑仍正常。
	a.beginRename(0)
	tt.Frame()
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	tt.ClickAt(200, 300)
	tt.Type("Y")
	if !strings.Contains(a.tabs[0].Text, "Y") {
		t.Fatalf("取消重命名后编辑异常: %q", a.tabs[0].Text)
	}
}

// ---- 切换后既有行为保持 ----

func TestTabSwitchResetsSessionState(t *testing.T) {
	a := newTestApp(t)
	t.Cleanup(a.shutdown)
	a.tabs[0].Text = "甲\n\n乙"
	a.tabs[1].Text = "丙"
	tt := ui.NewTester(a.view, 460, 540)
	tt.ClickAt(200, 120) // 进入焦点块
	tt.Frame()
	if a.mdActive < 0 {
		t.Fatal("应进入焦点块")
	}
	a.findOpen, a.query, a.matchIdx = true, "甲", 0
	a.tableTools.open = true
	a.switchTab(1)
	tt.Frame()
	if a.findOpen || a.query != "" || a.tableTools.open {
		t.Fatalf("切换未重置会话状态: find=%v query=%q tools=%+v", a.findOpen, a.query, a.tableTools)
	}
	if a.mdActive != -1 {
		t.Fatalf("切换后应回预览态: mdActive=%d", a.mdActive)
	}
	// 切回后编辑正常。
	tt.ClickAt(200, 300)
	tt.Type("丁")
	if !strings.Contains(a.tabs[1].Text, "丁") {
		t.Fatalf("切换后编辑异常: %q", a.tabs[1].Text)
	}
}

// TestTabRenameCommitsOnBlur 重命名中点击内容区让输入框失焦：等同回车，
// 名称提交、退出重命名，且不把焦点强行拉回内容编辑。
func TestTabRenameCommitsOnBlur(t *testing.T) {
	a := newTestApp(t)
	t.Cleanup(a.shutdown)
	tt := ui.NewTester(a.view, 460, 540)

	a.beginRename(0)
	tt.Frame()
	tt.Type("新名字")
	tt.Frame()
	if !a.renameSeenFocus {
		t.Fatal("自动聚焦落定后应置位 renameSeenFocus")
	}
	tt.ClickAt(200, 300) // 焦点到内容编辑器
	tt.Frame()
	if a.renaming != 0 {
		t.Fatal("失焦后应退出重命名")
	}
	if a.tabs[0].Name != "新名字" {
		t.Fatalf("失焦应提交名称: %q", a.tabs[0].Name)
	}
}

// TestBeginRenameCommitsPrevious 重命名 A 时直接对 B 开始重命名：
// A 的草稿先提交，不被静默覆盖。
func TestBeginRenameCommitsPrevious(t *testing.T) {
	a := newTestApp(t)
	t.Cleanup(a.shutdown)
	a.beginRename(0)
	a.renameDraft = "甲"
	a.beginRename(1)
	if a.renaming != a.tabs[1].ID {
		t.Fatalf("应进入标签 2 的重命名: renaming=%d", a.renaming)
	}
	if a.tabs[0].Name != "甲" {
		t.Fatalf("切换前应提交上一个草稿: %q", a.tabs[0].Name)
	}
}
