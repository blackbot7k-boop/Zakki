package app

import (
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"zakki/internal/markdown"
	"zakki/locales"
)

func TestLiveBlockTimeSuffixParse(t *testing.T) {
	due := time.Date(2026, 6, 1, 10, 0, 0, 0, time.Local)
	cases := []struct {
		name     string
		raw      string
		index    int
		text     string
		hasTime  bool
		hasAlert bool
		alert    time.Duration
	}{
		{"待办行", "- [ ] 交报告 @2026-06-01 10:00 ~1h", 0, "交报告", true, true, time.Hour},
		{"列表行", "- 团建 @2026-06-01 10:00 ~1d", 0, "团建", true, true, 24 * time.Hour},
		{"引用行", "> 和 Bob 通电话 @2026-06-01 10:00", 0, "和 Bob 通电话", true, false, 0},
		{"标题行", "## 计划 @2026-06-01 10:00", 0, "计划", true, false, 0},
		{"段落单行", "牙医复诊 @2026-06-01 10:00 ~10m", 0, "牙医复诊", true, true, 10 * time.Minute},
		{"段落末行", "甲\n乙 @2026-06-01 10:00", 0, "甲 乙", true, false, 0},
		{"无令牌待办", "- [ ] 普通任务", 0, "普通任务", false, false, 0},
		{"非法日期不识别", "x @2026-02-30 10:00", 0, "x @2026-02-30 10:00", false, false, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			blocks := liveBlocks(tc.raw)
			if tc.index >= len(blocks) {
				t.Fatalf("块数不足: %d", len(blocks))
			}
			blk := blocks[tc.index]
			if blk.text != tc.text {
				t.Errorf("text = %q，要 %q", blk.text, tc.text)
			}
			if blk.hasTime != tc.hasTime {
				t.Fatalf("hasTime = %v，要 %v", blk.hasTime, tc.hasTime)
			}
			if !tc.hasTime {
				return
			}
			if !blk.due.Equal(due) {
				t.Errorf("due = %v，要 %v", blk.due, due)
			}
			if blk.hasAlert != tc.hasAlert || blk.alert != tc.alert {
				t.Errorf("alert = (%v,%v)，要 (%v,%v)", blk.hasAlert, blk.alert, tc.hasAlert, tc.alert)
			}
		})
	}
}

// TestProjectBlockHidesTimeSuffix 两态共用对象投影，完整源码仅存在于映射中。
func TestProjectBlockHidesTimeSuffix(t *testing.T) {
	cases := []struct {
		name string
		raw  string
	}{
		{"待办", "- [ ] 交报告 @2026-06-01 10:00 ~1h"},
		{"列表", "- 团建 @2026-06-01 10:00"},
		{"引用", "> 电话 @2026-06-01 10:00"},
		{"标题", "## 计划 @2026-06-01 10:00"},
		{"段落", "牙医复诊 @2026-06-01 10:00"},
		{"多行段落", "甲\n乙 @2026-06-01 10:00"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			blk := liveBlocks(tc.raw)[0]
			proj := projectBlock(blk)
			if strings.Contains(proj.text, "@2026") {
				t.Errorf("可见文本泄漏令牌: %q", proj.text)
			}
			if proj.end > len([]rune(blk.raw)) {
				t.Fatalf("可编辑区越界: %d", proj.end)
			}
			if proj.end != utf8.RuneCountInString(blk.raw) || len(proj.objects) != 1 || !strings.ContainsRune(proj.text, timeObjectRune) {
				t.Fatalf("组件后必须保留可编辑边界: %+v", proj)
			}
		})
	}
}

func TestProjectBlockKeepsTokenlessBlocks(t *testing.T) {
	blk := liveBlocks("- [ ] 普通任务")[0]
	proj := projectBlock(blk)
	if proj.text != "普通任务" {
		t.Errorf("无令牌块投影 = %q，要 %q", proj.text, "普通任务")
	}
}

func TestTimeChipStateFor(t *testing.T) {
	now := time.Date(2026, 6, 10, 12, 0, 0, 0, time.Local)
	mk := func(kind blockKind, checked bool, due time.Time, hasTime bool) liveBlock {
		return liveBlock{kind: kind, checked: checked, due: due, hasTime: hasTime}
	}
	past := now.Add(-time.Hour)
	today := time.Date(2026, 6, 10, 18, 0, 0, 0, time.Local)
	future := now.Add(48 * time.Hour)
	cases := []struct {
		name string
		blk  liveBlock
		want timeChipState
	}{
		{"过期任务", mk(bkTask, false, past, true), chipOverdue},
		{"完成不再过期", mk(bkTask, true, past, true), chipNeutral},
		{"今天任务", mk(bkTask, false, today, true), chipToday},
		{"未来任务", mk(bkTask, false, future, true), chipFuture},
		{"普通行过去只是过去", mk(bkList, false, past, true), chipNeutral},
		{"普通行今天", mk(bkList, false, today, true), chipToday},
		{"无时间不参与", mk(bkTask, false, past, false), chipFuture},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := timeChipStateFor(tc.blk, now); got != tc.want {
				t.Errorf("state = %v，要 %v", got, tc.want)
			}
		})
	}
}

// TestTimeChipLabel 与 locale 目录同源断言，不绑定具体语言。
func TestTimeChipLabel(t *testing.T) {
	now := time.Date(2026, 6, 10, 12, 0, 0, 0, time.Local)
	today := time.Date(2026, 6, 10, 18, 0, 0, 0, time.Local)
	tomorrow := time.Date(2026, 6, 11, 12, 0, 0, 0, time.Local)
	inWeek := time.Date(2026, 6, 12, 9, 30, 0, 0, time.Local)
	sameYear := time.Date(2026, 8, 20, 15, 5, 0, 0, time.Local)
	otherYear := time.Date(2027, 3, 1, 9, 0, 0, 0, time.Local)

	if got := timeChipLabel(now, today, false); got != locales.TimeChipToday("18:00") {
		t.Errorf("今天 = %q", got)
	}
	if got := timeChipLabel(now, tomorrow, false); got != locales.TimeChipTomorrow(tomorrow.Format("15:04")) {
		t.Errorf("明天 = %q", got)
	}
	if got := timeChipLabel(now, inWeek, false); got != locales.TimeChipWeekday(timeChipWeekdayName(inWeek.Weekday()), "09:30") {
		t.Errorf("周内 = %q", got)
	}
	if got := timeChipLabel(now, sameYear, false); got != locales.TimeChipDate(timeChipMonthName(8), "20", "15:05") {
		t.Errorf("同年 = %q", got)
	}
	if got := timeChipLabel(now, otherYear, false); got != locales.TimeChipDateYear(timeChipMonthName(3), "1", "2027", "09:00") {
		t.Errorf("跨年 = %q", got)
	}
	if got := timeChipLabel(now, today, true); got != "18:00" {
		t.Errorf("已完成 = %q", got)
	}
	// 已过去（非完成）走绝对日期而不是星期几。
	past := time.Date(2026, 6, 9, 8, 0, 0, 0, time.Local)
	if got := timeChipLabel(now, past, false); got != locales.TimeChipDate(timeChipMonthName(6), "9", "08:00") {
		t.Errorf("过去 = %q", got)
	}
}

// TestSplitSuffixRoundTripThroughBlocks 块层往返：liveBlocks 剥离的正文拼回令牌后应复原。
func TestSplitSuffixRoundTripThroughBlocks(t *testing.T) {
	raw := "- [ ] 交报告 @2026-06-01 10:00 ~1h"
	blk := liveBlocks(raw)[0]
	if !blk.hasTime {
		t.Fatal("应识别出时间令牌")
	}
	restored := "- [ ] " + markdown.JoinTimeSuffix(blk.text, &markdown.TimeSuffix{
		Time: blk.due, Alert: blk.alert, HasAlert: blk.hasAlert,
	})
	if restored != raw {
		t.Errorf("拼回 = %q，要 %q", restored, raw)
	}
}

// TestEnterSplitKeepsTimeOnTask 光标在可见末尾回车：令牌留在原任务行，新续行不带时间。
func TestEnterSplitKeepsTimeOnTask(t *testing.T) {
	a := newTestApp(t)
	t.Cleanup(a.shutdown)
	raw := "- [ ] 交报告 @2026-06-01 10:00 ~1h"
	a.tabs[0].Text = raw
	a.mdActive, a.mdDraft = 0, raw
	// 可见文本 "交报告" 末尾的原始偏移 = 9（"- [ ] 交报告"）。
	a.afterEnter(9, 9)
	want := raw + "\n- [ ] "
	if a.tabs[0].Text != want {
		t.Fatalf("拆分后文档 = %q, 要 %q", a.tabs[0].Text, want)
	}
	blocks := liveBlocks(a.tabs[0].Text)
	if !blocks[0].hasTime || blocks[1].hasTime {
		t.Errorf("令牌应留在原行: %v / %v", blocks[0].hasTime, blocks[1].hasTime)
	}
	if blocks[1].text != "" {
		t.Errorf("新续行应为空任务: %q", blocks[1].text)
	}
}

// TestEnterSplitKeepsTimeOnPara 段落末尾回车：令牌留在原段。
func TestEnterSplitKeepsTimeOnPara(t *testing.T) {
	a := newTestApp(t)
	t.Cleanup(a.shutdown)
	raw := "甲乙 @2026-06-01 10:00"
	a.tabs[0].Text = raw
	a.mdActive, a.mdDraft = 0, raw
	a.afterEnter(utf8.RuneCountInString("甲乙"), 0)
	want := raw + "\n\n"
	if a.tabs[0].Text != want {
		t.Fatalf("拆分后文档 = %q, 要 %q", a.tabs[0].Text, want)
	}
	if !liveBlocks(a.tabs[0].Text)[0].hasTime {
		t.Error("令牌应留在原段")
	}
}

// TestEnterSplitMidTextKeepsTimeRight 段落中间回车：令牌本就属于右侧文本，不移动。
func TestEnterSplitMidTextKeepsTimeRight(t *testing.T) {
	a := newTestApp(t)
	t.Cleanup(a.shutdown)
	raw := "甲乙 @2026-06-01 10:00"
	a.tabs[0].Text = raw
	a.mdActive, a.mdDraft = 0, raw
	a.afterEnter(1, 1)
	want := "甲\n\n乙 @2026-06-01 10:00"
	if a.tabs[0].Text != want {
		t.Fatalf("拆分后文档 = %q, 要 %q", a.tabs[0].Text, want)
	}
	var right *liveBlock
	for i := range a.mdBlocks() {
		b := a.mdBlocks()[i]
		if b.kind == bkPara && strings.HasPrefix(b.text, "乙") {
			right = &a.mdBlocks()[i]
		}
	}
	if right == nil || !right.hasTime {
		t.Fatalf("令牌应留在右侧文本块上")
	}
	var leftHas bool
	for _, b := range a.mdBlocks() {
		if b.kind == bkPara && strings.HasPrefix(b.text, "甲") {
			leftHas = b.hasTime
		}
	}
	if leftHas {
		t.Error("左侧段不应带时间")
	}
}

// TestBackspaceMergeListKeepsTime 列表+列表合并是摘标记换行语义：令牌仍在原行尾，继续生效。
func TestBackspaceMergeListKeepsTime(t *testing.T) {
	a := newTestApp(t)
	t.Cleanup(a.shutdown)
	a.tabs[0].Text = "- 甲 @2026-06-01 10:00\n- 乙"
	a.mdActive, a.mdDraft = 1, "- 乙"
	a.backspaceAtBlockStart()
	want := "- 甲 @2026-06-01 10:00\n乙"
	if a.tabs[0].Text != want {
		t.Fatalf("合块后文档 = %q, 要 %q", a.tabs[0].Text, want)
	}
	if !liveBlocks(a.tabs[0].Text)[0].hasTime {
		t.Error("令牌仍在行尾，应继续生效")
	}
}

// TestBackspaceMergeTaskKeepsInlineTime 合并后的同一条内容仍保留行内时间，后文不丢失。
func TestBackspaceMergeTaskKeepsInlineTime(t *testing.T) {
	a := newTestApp(t)
	t.Cleanup(a.shutdown)
	a.tabs[0].Text = "- [ ] 甲 @2026-06-01 10:00\n- [ ] 乙"
	a.mdActive, a.mdDraft = 1, "- [ ] 乙"
	a.backspaceAtBlockStart()
	want := "- [ ] 甲 @2026-06-01 10:00- [ ] 乙"
	if a.tabs[0].Text != want {
		t.Fatalf("合块后文档 = %q, 要 %q", a.tabs[0].Text, want)
	}
	blk := liveBlocks(a.tabs[0].Text)[0]
	if !blk.hasTime {
		t.Error("行中令牌应继续识别为时间")
	}
	if strings.Contains(blk.text, "@2026-06-01 10:00") || !strings.Contains(blk.text, "乙") {
		t.Errorf("可读正文应去令牌但保留合并后文: %q", blk.text)
	}
}

// TestApplyTimePopover 弹层写回：设置与清除都走行级令牌管线并进入撤销。
func TestApplyTimePopover(t *testing.T) {
	a := newTestApp(t)
	t.Cleanup(a.shutdown)
	a.tabs[0].Text = "- [ ] 交报告"
	a.mdActive, a.mdDraft = 0, a.tabs[0].Text
	blk := a.mdBlocks()[0]

	a.openTimePopover(blk)
	if !a.timePop.open || a.timePop.line != 0 {
		t.Fatalf("弹层应打开并绑定行 0: %+v", a.timePop)
	}
	// 无时间块用当前时刻做种子，但 hasTime 仍为 false。
	if a.timePop.hasTime {
		t.Error("无时间块播种后不应标记 hasTime")
	}
	due := time.Date(2026, 6, 1, 10, 0, 0, 0, time.Local)
	a.timePop.due, a.timePop.hasTime, a.timePop.alert, a.timePop.hasAlert = due, true, time.Hour, true
	a.applyTimePopover(blk, &markdown.TimeSuffix{Time: due, Alert: time.Hour, HasAlert: true})
	if want := "- [ ] 交报告 @2026-06-01 10:00 ~1h"; a.tabs[0].Text != want {
		t.Fatalf("写回后 = %q, 要 %q", a.tabs[0].Text, want)
	}
	if !liveBlocks(a.tabs[0].Text)[0].hasTime {
		t.Error("写回后应识别出时间")
	}
	// 撤销恢复到无时间状态。
	a.undoLive(false)
	if want := "- [ ] 交报告"; a.tabs[0].Text != want {
		t.Fatalf("撤销后 = %q, 要 %q", a.tabs[0].Text, want)
	}

	// 清除时间。
	a.tabs[0].Text = "- [ ] 交报告 @2026-06-01 10:00 ~1h"
	a.mdActive, a.mdDraft = 0, a.tabs[0].Text
	a.applyTimePopover(a.mdBlocks()[0], nil)
	if want := "- [ ] 交报告"; a.tabs[0].Text != want {
		t.Fatalf("清除后 = %q, 要 %q", a.tabs[0].Text, want)
	}
}

// TestTimeAlertLabel 提醒档位文案与 locale 目录同源。
func TestTimeAlertLabel(t *testing.T) {
	if got := timeAlertLabel(0); got != locales.TimeAlertAtTime() {
		t.Errorf("准点 = %q", got)
	}
	if got := timeAlertLabel(10 * time.Minute); got != locales.TimeAlertMin("10") {
		t.Errorf("10 分 = %q", got)
	}
	if got := timeAlertLabel(time.Hour); got != locales.TimeAlertHour() {
		t.Errorf("1 小时 = %q", got)
	}
	if got := timeAlertLabel(24 * time.Hour); got != locales.TimeAlertDay() {
		t.Errorf("1 天 = %q", got)
	}
}

// TestTryTimeTrigger 行尾键入 @ 的占位符转化：吃掉 @、打开弹层；
// 行中、代码块、非 @ 不触发。
func TestTryTimeTrigger(t *testing.T) {
	a := newTestApp(t)
	t.Cleanup(a.shutdown)
	blk := liveBlocks("买牛奶")[0]

	a.mdVisible = "买牛奶@"
	cs, ce := 4, 4
	if !a.tryTimeTrigger(blk, &cs, &ce) {
		t.Fatal("行尾 @ 应触发")
	}
	if a.mdVisible != "买牛奶" || cs != 3 || ce != 3 {
		t.Errorf("占位符未正确移除: %q cs=%d", a.mdVisible, cs)
	}
	if !a.timePop.open || a.timePop.line != blk.start {
		t.Errorf("弹层应打开并绑定该块: %+v", a.timePop)
	}

	a.mdVisible = "甲@乙"
	cs, ce = 2, 2
	if a.tryTimeTrigger(blk, &cs, &ce) {
		t.Error("行中 @ 不应触发")
	}
	a.mdVisible = "买牛奶"
	cs, ce = 3, 3
	if a.tryTimeTrigger(blk, &cs, &ce) {
		t.Error("非 @ 不应触发")
	}
	codeBlk := liveBlocks("```\nx\n```\n")[0]
	a.mdVisible = "x@"
	cs, ce = 2, 2
	if a.tryTimeTrigger(codeBlk, &cs, &ce) {
		t.Error("代码块内不应触发")
	}
}
