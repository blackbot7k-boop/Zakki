package app

import (
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/egoist/mygo/ui"

	"zakki/internal/markdown"
	"zakki/locales"
)

func TestTimePreviewFocusKeepsObjectAtEnd(t *testing.T) {
	for _, prefix := range []string{"", "- [ ] ", "- ", "> ", "## "} {
		for _, tail := range []string{"", " 附上预算表"} {
			t.Run(prefix+tail, func(t *testing.T) {
				a := newTestApp(t)
				t.Cleanup(a.shutdown)
				raw := prefix + "交报告 @2026-11-01 10:00 ~1h" + tail
				a.tabs[0].Text = raw + "\n\n其他段落"
				a.mdActive = -1
				tt := ui.NewTester(a.view, 440, 600)
				p := a.previewProjection(a.mdBlocks()[0])
				r, ok := tt.Find(p.text)
				if !ok {
					t.Fatalf("找不到带行内对象的预览正文: %q", tt.Texts())
				}
				tt.ClickAt(r.X+1, r.Y+r.H/2)
				tt.Frame()
				if !tt.Focused("内容") || a.mdVisible != p.text || strings.Contains(a.mdVisible, "@2026") {
					t.Fatalf("聚焦必须保持同一投影，不展开源码: visible=%q focused=%v", a.mdVisible, tt.Focused("内容"))
				}
				n := utf8.RuneCountInString(raw)
				if a.mdCaret != n || a.mdSelEnd != n {
					t.Fatalf("默认光标应在整个条目末尾: caret=%d end=%d want=%d", a.mdCaret, a.mdSelEnd, n)
				}
				tt.Type("测")
				want := raw + "测\n\n其他段落"
				if tail == "" {
					want = raw + " 测\n\n其他段落"
				}
				if a.tabs[0].Text != want || !a.mdBlocks()[0].hasTime {
					t.Fatalf("组件后输入不能移动或破坏时间: %q", a.tabs[0].Text)
				}
				tt.Key(ui.Cmd, ui.KeyZ)
				if a.tabs[0].Text != raw+"\n\n其他段落" {
					t.Fatalf("撤销应恢复原文和组件: %q", a.tabs[0].Text)
				}
				tt.Key(0, ui.KeyLeft)
				wantCaret := n - 1
				if tail == "" {
					wantCaret = utf8.RuneCountInString(prefix + "交报告 ")
				}
				if a.mdCaret != wantCaret {
					t.Fatalf("左方向键应跨过整个组件或一个普通字，不反复吸到末尾: got %d want %d", a.mdCaret, wantCaret)
				}
				tt.Type("前")
				if !strings.Contains(a.tabs[0].Text, "前") || !a.mdBlocks()[0].hasTime {
					t.Fatalf("移动后也应能正常输入: %q", a.tabs[0].Text)
				}
				tt.Key(ui.Cmd, ui.KeyZ)
				a.blurLive()
				tt.Frame()
				if got := a.previewProjection(a.mdBlocks()[0]).text; got != p.text || a.tabs[0].Text != raw+"\n\n其他段落" {
					t.Fatalf("失焦应保留相同组件与正文: %q", got)
				}
			})
		}
	}
}

func TestTimeComponentEnterKeepsTimeOnPreviousLine(t *testing.T) {
	for _, prefix := range []string{"", "- [ ] ", "- "} {
		t.Run(prefix, func(t *testing.T) {
			a := newTestApp(t)
			t.Cleanup(a.shutdown)
			raw := prefix + "交报告 @2026-11-01 10:00 ~1h"
			a.tabs[0].Text = raw
			a.mdActive = -1
			a.focusBlock(0, -1, -1)
			tt := ui.NewTester(a.view, 440, 600)
			tt.Key(0, ui.KeyEnter)
			want := raw + "\n" + prefix
			if prefix == "" {
				want = raw + "\n\n"
			}
			if a.tabs[0].Text != want {
				t.Fatalf("组件后回车应保留原条目时间: got %q want %q", a.tabs[0].Text, want)
			}
			if !a.mdBlocks()[0].hasTime {
				t.Fatal("回车后原行时间应保留")
			}
		})
	}
}

func TestTimeComponentDeleteUndo(t *testing.T) {
	for _, backward := range []bool{true, false} {
		t.Run(map[bool]string{true: "Backspace", false: "Delete"}[backward], func(t *testing.T) {
			a := newTestApp(t)
			t.Cleanup(a.shutdown)
			raw := "交报告 @2026-11-01 10:00 ~1h 附表"
			a.tabs[0].Text, a.mdActive = raw, -1
			p := projectBlock(a.mdBlocks()[0])
			u := p.units[p.objects[0]]
			caret, key := u.start, ui.KeyDelete
			if backward {
				caret, key = u.end, ui.KeyBackspace
			}
			a.focusBlock(0, caret, caret)
			tt := ui.NewTester(a.view, 400, 540)
			tt.Key(0, key)
			if got := a.tabs[0].Text; got != "交报告  附表" || a.mdBlocks()[0].hasTime || strings.ContainsRune(got, timeObjectRune) {
				t.Fatalf("一次删除必须去掉完整组件但保留前后文字: %q", got)
			}
			tt.Key(ui.Cmd, ui.KeyZ)
			if a.tabs[0].Text != raw || !a.mdBlocks()[0].hasTime {
				t.Fatalf("撤销必须恢复完整组件: %q", a.tabs[0].Text)
			}
			tt.Key(ui.Cmd|ui.Shift, ui.KeyZ)
			if a.mdBlocks()[0].hasTime || a.tabs[0].Text != "交报告  附表" {
				t.Fatalf("重做必须再次删除完整组件: %q", a.tabs[0].Text)
			}
		})
	}
}

func TestTimePopoverUsesCompactPanelLayout(t *testing.T) {
	a := newTestApp(t)
	t.Cleanup(a.shutdown)
	a.timePop = timePopoverState{
		open: true, line: 0,
		due: time.Date(2026, 11, 1, 10, 0, 0, 0, time.Local), hasTime: true,
		alert: time.Hour, hasAlert: true,
	}
	blk := liveBlocks("交报告 @2026-11-01 10:00 ~1h")[0]
	var panelBounds ui.Rect
	tt := ui.NewTester(func(c *ui.Context) {
		anchor := ui.Row(c.Key("wide-anchor")).FillWidth().Height(28)
		a.timePopover(c, anchor, blk).DrawOver(func(_ *ui.Painter, bounds ui.Rect) {
			panelBounds = bounds
		})
	}, 440, 600)
	if panelBounds.W <= 0 {
		t.Fatal("时间弹层应创建面板")
	}
	bounds := panelBounds
	if bounds.W != timePopoverPanelWidth {
		t.Fatalf("弹层宽度 = %.1f，要 %.1f", bounds.W, timePopoverPanelWidth)
	}
	inside := func(label string) {
		t.Helper()
		r, ok := tt.Find(label)
		if !ok {
			t.Fatalf("找不到弹层内容 %q: %q", label, tt.Texts())
		}
		if r.X < bounds.X || r.Y < bounds.Y || r.X+r.W > bounds.X+bounds.W || r.Y+r.H > bounds.Y+bounds.H {
			t.Fatalf("弹层内容 %q 溢出面板: rect=%+v panel=%+v", label, r, bounds)
		}
	}
	inside(locales.TimePopDate())
	inside(locales.TimePopClock())
	inside(locales.TimePopAlert())
	inside(locales.TimePopDone())
}

func TestTimePopoverPreservesRecurAcrossSoftWrap(t *testing.T) {
	a := newTestApp(t)
	t.Cleanup(a.shutdown)
	a.tabs[0].Text = "正文\n续行 @" + a.now().Add(24*time.Hour).Format("2006-01-02 15:04") + " ~10m ^w@1-5 后文"
	blocks := a.mdBlocks()
	if len(blocks) != 1 || blocks[0].recur != "^w@1-5" {
		t.Fatalf("重复段未识别: %+v", blocks)
	}
	a.openTimePopover(blocks[0])
	a.timePop.due = a.timePop.due.Add(time.Hour)
	a.applyTimePopover(blocks[0], &markdown.TimeSuffix{Time: a.timePop.due, Alert: a.timePop.alert, HasAlert: a.timePop.hasAlert})
	got := a.tabs[0].Text
	if strings.Count(got, "@") != 2 || !strings.Contains(got, "\n续行 ") || !strings.Contains(got, "^w@1-5 后文") {
		t.Fatalf("写回软换行与重复段 = %q", got)
	}
}

func TestTimePopoverDateChangeEnablesCompletion(t *testing.T) {
	a := newTestApp(t)
	t.Cleanup(a.shutdown)
	a.tabs[0].Text = "交报告"
	blk := liveBlocks(a.tabs[0].Text)[0]
	a.openTimePopover(blk)
	tt := ui.NewTester(func(c *ui.Context) {
		anchor := ui.Row(c.Key("wide-anchor")).FillWidth().Height(28)
		a.timePopover(c, anchor, blk)
	}, 440, 600)
	tt.SetLocale("zh-Hans")
	var dateLabel string
	for _, label := range tt.Texts() {
		if strings.Contains(label, "/") {
			dateLabel = label
			break
		}
	}
	date, ok := tt.Find(dateLabel)
	if !ok {
		t.Fatalf("日期字段不存在: %q", tt.Texts())
	}
	tt.ClickAt(date.X+date.W/2, date.Y+date.H/2)
	tt.Key(0, ui.KeyRight)
	tt.Key(0, ui.KeyEnter)
	if !a.timePop.hasTime {
		t.Fatalf("只改日期后完成仍禁用: %q", tt.Texts())
	}
	if err := tt.Click(locales.TimePopDone()); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(a.tabs[0].Text, "@") {
		t.Fatalf("完成后未写日期: %q", a.tabs[0].Text)
	}
}

func TestTimePopoverClickConfirmsSeededTime(t *testing.T) {
	a := newTestApp(t)
	t.Cleanup(a.shutdown)
	a.timePop = timePopoverState{
		open: true, line: 0,
		due: time.Date(2026, 11, 1, 10, 5, 0, 0, time.Local), hasTime: false,
	}
	blk := liveBlocks("交报告")[0]
	tt := ui.NewTester(func(c *ui.Context) {
		anchor := ui.Row(c.Key("wide-anchor")).FillWidth().Height(28)
		a.timePopover(c, anchor, blk)
	}, 440, 600)
	tt.SetLocale("zh-Hans")
	if a.timePop.hasTime {
		t.Fatal("打开弹框但未点击时刻时，不应确认临时时刻")
	}
	var dateLabel string
	for _, label := range tt.Texts() {
		if strings.Contains(label, "/") {
			dateLabel = label
			break
		}
	}
	date, ok := tt.Find(dateLabel)
	if !ok {
		t.Fatalf("找不到临时日期字段: %q", tt.Texts())
	}
	tt.ClickAt(date.X+date.W/2, date.Y+date.H/2)
	if a.timePop.hasTime {
		t.Fatal("只点击日期不应确认时刻")
	}
	tt.Key(0, ui.KeyEscape)
	hour, ok := tt.Find("10")
	if !ok {
		t.Fatalf("找不到临时时刻的小时字段: %q", tt.Texts())
	}
	tt.ClickAt(hour.X+hour.W/2, hour.Y+hour.H/2)
	if a.timePop.hasTime || !a.timePop.wheelOpen {
		t.Fatalf("点击时刻输入应只打开滚轮: hasTime=%v wheel=%v", a.timePop.hasTime, a.timePop.wheelOpen)
	}
	minute, ok := tt.Find("06")
	if !ok {
		t.Fatalf("找不到下一分钟: %q", tt.Texts())
	}
	tt.Scroll(minute.X+minute.W/2, minute.Y+minute.H/2, 0, 30)
	if a.timePop.hasTime || a.timePop.due.Minute() != 5 {
		t.Fatalf("滚轮预览不应确认或修改底部时刻: hasTime=%v due=%s", a.timePop.hasTime, a.timePop.due.Format("15:04"))
	}
	if err := tt.Click("06"); err != nil {
		t.Fatal(err)
	}
	if !a.timePop.hasTime || a.timePop.due.Minute() != 6 {
		t.Fatalf("点击滚轮时刻才确认: hasTime=%v due=%s", a.timePop.hasTime, a.timePop.due.Format("15:04"))
	}
}

func TestTimePopoverTimeWheel(t *testing.T) {
	a := newTestApp(t)
	t.Cleanup(a.shutdown)
	a.timePop = timePopoverState{
		open: true, line: 0,
		due: time.Date(2026, 11, 1, 10, 5, 0, 0, time.Local), hasTime: true,
	}
	blk := liveBlocks("交报告 @2026-11-01 10:05")[0]
	var panelBounds ui.Rect
	tt := ui.NewTester(func(c *ui.Context) {
		anchor := ui.Row(c.Key("wide-anchor")).FillWidth().Height(28)
		a.timePopover(c, anchor, blk).DrawOver(func(_ *ui.Painter, bounds ui.Rect) {
			panelBounds = bounds
		})
	}, 440, 600)
	tt.SetLocale("zh-Hans")
	baseHeight := panelBounds.H
	if tt.HasText("AM/PM") {
		t.Fatalf("24 小时制不应出现上午/下午字段: %q", tt.Texts())
	}
	clock, ok := tt.Find("10")
	if !ok {
		t.Fatalf("找不到时刻输入: %q", tt.Texts())
	}
	tt.ClickAt(clock.X+clock.W/2, clock.Y+clock.H/2)
	if !a.timePop.wheelOpen || !tt.HasText("04") || !tt.HasText("06") {
		t.Fatalf("点击时刻应打开滚轮: open=%v texts=%q", a.timePop.wheelOpen, tt.Texts())
	}
	if panelBounds.H != baseHeight {
		t.Fatalf("浮动滚轮不应撑高主弹层: got %.1f want %.1f", panelBounds.H, baseHeight)
	}
	previousHour, ok := tt.Find("09")
	if !ok {
		t.Fatalf("找不到滚轮上一小时: %q", tt.Texts())
	}
	nextHour, ok := tt.Find("11")
	if !ok {
		t.Fatalf("找不到滚轮下一小时: %q", tt.Texts())
	}
	wheelCenterY := (previousHour.Y + previousHour.H/2 + nextHour.Y + nextHour.H/2) / 2
	if centerX := previousHour.X + previousHour.W/2; centerX < clock.X-72 || centerX > clock.X+clock.W+72 ||
		wheelCenterY < clock.Y-48 || wheelCenterY > clock.Y+clock.H+48 {
		t.Fatalf("滚轮选中行应靠近时刻控件: clock=%+v previous=%+v next=%+v", clock, previousHour, nextHour)
	}
	if pitch := nextHour.Y + nextHour.H/2 - previousHour.Y - previousHour.H/2; pitch < 76 {
		t.Fatalf("滚轮选项纵向空间不足: %.1f", pitch)
	}
	minute, ok := tt.Find("06")
	if !ok {
		t.Fatalf("找不到滚轮下一分钟: %q", tt.Texts())
	}
	tt.Scroll(minute.X+minute.W/2, minute.Y+minute.H/2, 0, 30)
	if _, gotMinute, _ := a.timePop.due.Clock(); gotMinute != 5 || !tt.HasText("07") {
		t.Fatalf("滚轮滚动只能预览下一分钟，底部仍应为 05: due=%s wheel=%q", a.timePop.due.Format("15:04"), tt.Texts())
	}
	tt.Scroll(previousHour.X+previousHour.W/2, previousHour.Y+previousHour.H/2, 0, -30)
	if got := a.timePop.due.Format("15:04"); got != "10:05" {
		t.Fatalf("滚动多个滚轮列也不应修改底部时刻: %s", got)
	}
	if err := tt.Click("06"); err != nil {
		t.Fatal(err)
	}
	if got := a.timePop.due.Format("15:04"); got != "09:06" {
		t.Fatalf("点击滚轮时刻应一起确认小时和分钟: %s", got)
	}
	if err := tt.Click("07"); err != nil {
		t.Fatal(err)
	}
	if _, gotMinute, _ := a.timePop.due.Clock(); gotMinute != 7 || !a.timePop.open || !a.timePop.wheelOpen {
		t.Fatalf("选择滚轮项后应更新草稿并保持弹框: minute=%02d outer=%v wheel=%v", gotMinute, a.timePop.open, a.timePop.wheelOpen)
	}
	if err := tt.Click(locales.TimePopDone()); err != nil {
		t.Fatal(err)
	}
	if a.timePop.wheelOpen {
		t.Fatalf("点击完成应关闭时刻滚轮: outer=%v wheel=%v", a.timePop.open, a.timePop.wheelOpen)
	}
	if a.timePop.open || !strings.Contains(a.tabs[0].Text, "09:07") {
		t.Fatalf("完成后应提交滚轮选择: open=%v wheel=%v text=%q", a.timePop.open, a.timePop.wheelOpen, a.tabs[0].Text)
	}
}

func TestTimePopoverScrollWithoutClickKeepsSavedTime(t *testing.T) {
	a := newTestApp(t)
	t.Cleanup(a.shutdown)
	a.timePop = timePopoverState{
		open: true, line: 0,
		due: time.Date(2026, 11, 1, 10, 5, 0, 0, time.Local), hasTime: true,
	}
	blk := liveBlocks("交报告 @2026-11-01 10:05")[0]
	tt := ui.NewTester(func(c *ui.Context) {
		anchor := ui.Row(c.Key("wide-anchor")).FillWidth().Height(28)
		a.timePopover(c, anchor, blk)
	}, 440, 600)
	tt.SetLocale("zh-Hans")
	clock, ok := tt.Find("10")
	if !ok {
		t.Fatalf("找不到时刻输入: %q", tt.Texts())
	}
	tt.ClickAt(clock.X+clock.W/2, clock.Y+clock.H/2)
	minute, ok := tt.Find("06")
	if !ok {
		t.Fatalf("找不到下一分钟: %q", tt.Texts())
	}
	tt.Scroll(minute.X+minute.W/2, minute.Y+minute.H/2, 0, 30)
	if err := tt.Click(locales.TimePopDone()); err != nil {
		t.Fatal(err)
	}
	if a.timePop.open || !strings.Contains(a.tabs[0].Text, "10:05") {
		t.Fatalf("未点击滚轮时完成应保留原时刻: open=%v text=%q", a.timePop.open, a.tabs[0].Text)
	}
}

func TestTimePopoverWheelInTwelveHourRegion(t *testing.T) {
	a := newTestApp(t)
	t.Cleanup(a.shutdown)
	a.timePop = timePopoverState{
		open: true, line: 0,
		due: time.Date(2026, 11, 1, 10, 5, 0, 0, time.Local), hasTime: true,
	}
	blk := liveBlocks("交报告 @2026-11-01 10:05")[0]
	tt := ui.NewTester(func(c *ui.Context) {
		anchor := ui.Row(c.Key("wide-anchor")).FillWidth().Height(28)
		a.timePopover(c, anchor, blk)
	}, 440, 600)
	tt.SetLocale("en-US")
	hour, ok := tt.Find("10")
	if !ok {
		t.Fatalf("找不到 12 小时制时刻输入: %q", tt.Texts())
	}
	tt.ClickAt(hour.X+hour.W/2, hour.Y+hour.H/2)
	pm, ok := tt.Find("PM")
	if !ok {
		t.Fatalf("12 小时制滚轮应显示时段: %q", tt.Texts())
	}
	if pm.X+pm.W > 440 {
		t.Fatalf("时段滚轮不应越过窗口: %+v", pm)
	}
	tt.Scroll(pm.X+pm.W/2, pm.Y+pm.H/2, 0, 30)
	if got := a.timePop.due.Hour(); got != 10 {
		t.Fatalf("滚动时段只预览，原时刻仍应为上午十点: %d", got)
	}
	pm, ok = tt.Find("PM")
	if !ok {
		t.Fatalf("滚动后找不到选中的下午时段: %q", tt.Texts())
	}
	tt.ClickAt(pm.X+pm.W/2, pm.Y+pm.H/2)
	if got := a.timePop.due.Hour(); got != 22 || !a.timePop.wheelOpen {
		t.Fatalf("选择下午应得到 22:05 并保持滚轮: hour=%d wheel=%v", got, a.timePop.wheelOpen)
	}
	if err := tt.Click(locales.TimePopDone()); err != nil {
		t.Fatal(err)
	}
	if a.timePop.open || !strings.Contains(a.tabs[0].Text, "22:05") {
		t.Fatalf("12 小时制滚轮浮出时完成应能直接提交: open=%v text=%q", a.timePop.open, a.tabs[0].Text)
	}
}

func TestTimeComponentClickOpensPopover(t *testing.T) {
	a := newTestApp(t)
	t.Cleanup(a.shutdown)
	a.tabs[0].Text = "交报告 @2026-11-01 10:00 ~1h 附表"
	a.mdActive = -1
	a.focusBlock(0, -1, -1)
	var target ui.Rect
	tt := ui.NewTester(func(c *ui.Context) {
		a.view(c)
		el := c.Resolve(a.editorHandle)
		el.DrawOver(func(_ *ui.Painter, bounds ui.Rect) {
			for _, object := range el.TextObjects() {
				target = ui.Rect{X: bounds.X + object.Rect.X, Y: bounds.Y + object.Rect.Y, W: object.Rect.W, H: object.Rect.H}
			}
		})
	}, 440, 600)
	if target.W <= 0 || target.H <= 0 {
		t.Fatal("组件必须具有原生布局矩形")
	}
	tt.ClickAt(target.X+target.W/2, target.Y+target.H/2)
	tt.Frame()
	if !a.timePop.open {
		t.Fatal("点击聚焦中的时间组件应打开弹层")
	}
	a.timePop.open = false
	a.blurLive()
	tt = ui.NewTester(a.view, 440, 600)
	tt.ClickAt(target.X+target.W/2, target.Y+target.H/2)
	tt.Frame()
	if !a.timePop.open || !tt.HasText(locales.TimePopDate()) {
		t.Fatalf("点击未聚焦的时间组件也应显示弹层内容: open=%v active=%d target=%+v texts=%q", a.timePop.open, a.mdActive, target, tt.Texts())
	}
	count := 0
	for _, text := range tt.Texts() {
		if text == locales.TimePopDate() {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("同一时间行只应创建一个弹层: count=%d texts=%q", count, tt.Texts())
	}
}

func TestTimeRowFocusAfterPicker(t *testing.T) {
	for _, raw := range []string{"交报告 @2026-11-01 10:00 ~1h", "@2026-11-01 10:00 ~1h", "- [ ] 交报告 @2026-11-01 10:00 ~1h"} {
		t.Run(raw, func(t *testing.T) {
			a := newTestApp(t)
			t.Cleanup(a.shutdown)
			a.tabs[0].Text, a.mdActive = raw+"\n\n其他段落", -1
			tt := ui.NewTester(a.view, 440, 600)
			p := a.previewProjection(a.mdBlocks()[0])
			r, ok := tt.Find(p.text)
			if !ok {
				t.Fatalf("找不到时间行: %q", tt.Texts())
			}
			md := a.resolveStyle(-1, false, true)
			var objects []ui.TextRange
			ui.NewTester(func(c *ui.Context) { objects = a.timeObjectRanges(c, md, a.mdBlocks()[0], p) }, 440, 600)
			layout := ui.ShapeRichText(timeObjectSpans(p, nil, objects), bodyLiveFont(md), r.W)
			chip := layout.Objects()[0].Rect
			tt.ClickAt(r.X+chip.X+chip.W/2, r.Y+r.H/2)
			tt.Frame()
			if !a.timePop.open || !tt.HasText(locales.TimePopDate()) {
				t.Fatalf("点击时间应打开选择器: open=%v active=%d texts=%q", a.timePop.open, a.mdActive, tt.Texts())
			}
			if err := tt.Click(locales.TimePopDone()); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			r, ok = tt.Find(p.text)
			if !ok {
				t.Fatalf("关闭选择器后找不到时间行: active=%d texts=%q", a.mdActive, tt.Texts())
			}
			tt.ClickAt(r.X+r.W-2, r.Y+r.H/2)
			tt.Frame()
			if !tt.Focused("内容") || a.mdActive != 0 || a.timePop.open {
				t.Fatalf("关闭选择器后应能点击行尾空白聚焦: active=%d popover=%v focused=%v texts=%q", a.mdActive, a.timePop.open, tt.Focused("内容"), tt.Texts())
			}
			tt.Type("后续")
			if want := raw + " 后续\n\n其他段落"; a.tabs[0].Text != want {
				t.Fatalf("聚焦后输入错误: got %q want %q", a.tabs[0].Text, want)
			}
		})
	}
}

func TestTimeComponentExplicitSelectionIsAtomic(t *testing.T) {
	a := newTestApp(t)
	t.Cleanup(a.shutdown)
	raw := "交报告 @2026-11-01 10:00 ~1h 附表"
	a.tabs[0].Text = raw
	a.mdActive = -1
	start := utf8.RuneCountInString("交报告 @2026-")
	a.focusBlock(0, start, start+2)
	tt := ui.NewTester(a.view, 440, 600)
	p := projectBlock(a.mdBlocks()[0])
	u := p.units[p.objects[0]]
	if a.mdCaret != u.start || a.mdSelEnd != u.end {
		t.Fatalf("搜索选区触及源码时应完整选中组件: %d..%d want %d..%d", a.mdCaret, a.mdSelEnd, u.start, u.end)
	}
	tt.Type("替代")
	if want := "交报告 替代 附表"; a.tabs[0].Text != want {
		t.Fatalf("输入应替换整组件，不留下部分令牌: got %q want %q", a.tabs[0].Text, want)
	}
	tt.Key(ui.Cmd, ui.KeyZ)
	if a.tabs[0].Text != raw || !a.mdBlocks()[0].hasTime {
		t.Fatalf("撤销应完整恢复组件与后文: %q", a.tabs[0].Text)
	}
}
