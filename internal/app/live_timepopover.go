package app

// 时间弹层：chip 或「设置时间…」命令的编辑入口。草稿存会话字段（不存 ui 元素），
// 「完成」才写回文档，「清除时间」去掉时间令牌；写回走勾选切换同一条
// 改行 → resync → 撤销事务 → 触发保存管线。

import (
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/egoist/mygo/i18n"
	"github.com/egoist/mygo/ui"

	"zakki/internal/markdown"
	"zakki/locales"
)

// timePopoverState 是时间弹层的会话草稿；wheelDue 只预览滚轮滚动，点击选项才写入 due。
type timePopoverState struct {
	open       bool
	line       int
	due        time.Time
	hasTime    bool
	alert      time.Duration
	hasAlert   bool
	wheelOpen  bool
	wheelDue   time.Time
	wheelAccum [3]float32
}

// openTimePopover 用块现状播种草稿并打开弹层。无时间的块用当前时刻做
// 种子：DateInput/TimeInput 需要非 nil 且可显示的值，是否设置仍由 hasTime 门禁。
func (a *app) openTimePopover(blk liveBlock) {
	a.timePop = timePopoverState{
		open: true, line: blk.start,
		due: blk.due, hasTime: blk.hasTime,
		alert: blk.alert, hasAlert: blk.hasAlert,
	}
	if !blk.hasTime {
		now := a.now()
		h, m, _ := now.Clock()
		a.timePop.due = time.Date(now.Year(), now.Month(), now.Day(), h, m, 0, 0, time.Local)
	}
}

// openTimePopoverAtCaret 为光标所在块打开时间弹层。源码模式没有 chip/编辑器锚点，
// 不响应；未聚焦时先聚焦光标所在块，弹层标记跨帧留存，锚点出现即弹出。
func (a *app) openTimePopoverAtCaret() {
	if a.cfg.Source {
		return
	}
	blocks := a.mdBlocks()
	if len(blocks) == 0 {
		return
	}
	idx := a.mdActive
	if idx < 0 || idx >= len(blocks) {
		start, _ := a.editorCaret()
		idx, _ = blockOfRuneOffset(blocks, start)
		if idx < 0 || idx >= len(blocks) {
			return
		}
		a.focusBlock(idx, -1, -1)
	}
	a.openTimePopover(blocks[idx])
}

// timeAlertLabel 提醒档位的展示名。
func timeAlertLabel(span time.Duration) string {
	switch span {
	case 0:
		return locales.TimeAlertAtTime()
	case time.Hour:
		return locales.TimeAlertHour()
	case 24 * time.Hour:
		return locales.TimeAlertDay()
	default:
		return locales.TimeAlertMin(strconv.Itoa(int(span / time.Minute)))
	}
}

// timePopoverPanelWidth 是弹层外框宽度。ui.Popover 默认把锚点宽度设为面板最小宽度，
// 这里用固定值覆盖它，让日期输入、提醒档位和底部操作共享同一列宽。
const timePopoverPanelWidth float32 = 330
const timeWheelPanelWidth float32 = 152
const timeWheel12PanelWidth float32 = 212

func timeWheelScrollSteps(accum *float32, dy float32) int {
	if dy == 0 {
		return 0
	}
	*accum += dy
	const scrollUnit float32 = 30
	steps := int(*accum / scrollUnit)
	if steps != 0 {
		*accum -= float32(steps) * scrollUnit
	}
	return steps
}

func timeWheelLabels(current, count int, label func(int) string) [3]string {
	previous := (current + count - 1) % count
	next := (current + 1) % count
	return [3]string{label(previous), label(current), label(next)}
}

func setTimePart(value *time.Time, hour, minute int) {
	year, month, day := value.Date()
	_, _, second := value.Clock()
	*value = time.Date(year, month, day, hour, minute, second, value.Nanosecond(), value.Location())
}

func wrapTimeWheelIndex(value, count int) int {
	value %= count
	if value < 0 {
		value += count
	}
	return value
}

func timeWheelNumber(value int) string {
	if value < 10 {
		return "0" + strconv.Itoa(value)
	}
	return strconv.Itoa(value)
}

// timePopover 将时间弹层挂在整行锚点上，同时清除 ui.Popover 按锚点设置的
// 最小宽度。整行锚点用于保留弹层打开期间行内点击不被判定为外部点击。
func (a *app) timePopover(c *ui.Context, anchor ui.Element, blk liveBlock) ui.Element {
	panel := ui.Popover(c, anchor, &a.timePop.open, func() {
		a.timePopoverContent(c, blk)
	})
	return panel.MinWidth(0).Width(timePopoverPanelWidth)
}

// timePopoverContent 弹层内容：日期+时刻、提醒档位、清除与完成。
func (a *app) timePopoverContent(c *ui.Context, blk liveBlock) {
	tp := &a.timePop
	theme := c.Theme()
	col := ui.Column(c.Key("time-pop")).Gap(12).Padding(12).FillWidth()
	col.Children(func() {
		fields := ui.Row(c.Key("time-fields")).Gap(8).FillWidth().AlignItems(ui.End)
		fields.Children(func() {
			dateField := ui.Column(c.Key("time-date-field")).Gap(6).Grow(1).MinWidth(0)
			dateField.Children(func() {
				ui.Text(c, locales.TimePopDate()).FontSize(11.5).TextColor(theme.TextMuted)
				if ui.DateInput(c, &tp.due).FillWidth().Changed() {
					tp.hasTime = true
				}
			})
			clockField := ui.Column(c.Key("time-clock-field")).Gap(6).Shrink(0)
			clockField.Children(func() {
				ui.Text(c, locales.TimePopClock()).FontSize(11.5).TextColor(theme.TextMuted)
				ti := ui.TimeInput(c, &tp.due).HandleInput(func(ev ui.InputEvent) bool {
					switch ev.Kind {
					case ui.InputPointerDown:
						if ev.Button == 0 {
							tp.wheelOpen = !tp.wheelOpen
							if tp.wheelOpen {
								tp.wheelDue = tp.due
							}
							tp.wheelAccum = [3]float32{}
							return true
						}
					case ui.InputPointerUp:
						return ev.Button == 0
					}
					return false
				})
				if ti.Changed() {
					tp.hasTime = true
					if tp.wheelOpen {
						tp.wheelDue = tp.due
					}
				}
				wheel := ui.Popover(c, ti, &tp.wheelOpen, func() {
					a.timeWheelContent(c)
				})
				wheel.AttachTo(ti, ui.AnchorCenter, ui.AnchorCenter).
					MinWidth(0).Width(timeWheelWidth(c))
			})
		})
		alertSection := ui.Column(c.Key("time-alert-section")).Gap(8).FillWidth()
		alertSection.Children(func() {
			ui.Text(c, locales.TimePopAlert()).FontSize(11.5).TextColor(theme.TextMuted)
			pills := ui.Row(c.Key("time-alert-pills")).Gap(6).FillWidth().Wrap()
			pills.Children(func() {
				selected := func() bool { return tp.hasTime && !tp.hasAlert }
				selNone := selected()
				none := ui.ButtonBase(c.Key("alert-none")).Radius(999).
					Cursor(ui.CursorPointer).FocusRing(true).Padding(5, 12).Disabled(!tp.hasTime)
				if selNone {
					none.Background(c.Theme().Accent)
				} else {
					none.Background(c.Theme().Surface)
				}
				none.Children(func() {
					txt := ui.Text(c, locales.TimeAlertNone()).FontSize(12)
					if selNone {
						txt = txt.TextColor(c.Theme().AccentText)
					}
				})
				if none.Clicked() {
					tp.hasAlert = false
				}
				for _, tier := range markdown.AlertTiers() {
					tier := tier
					sel := tp.hasAlert && tp.alert == tier.Span
					pill := ui.ButtonBase(c.Key("alert-"+tier.Suffix)).Radius(999).
						Cursor(ui.CursorPointer).FocusRing(true).Padding(5, 12).Disabled(!tp.hasTime)
					if sel {
						pill.Background(c.Theme().Accent)
					} else {
						pill.Background(c.Theme().Surface)
					}
					pill.Children(func() {
						txt := ui.Text(c, timeAlertLabel(tier.Span)).FontSize(12)
						if sel {
							txt = txt.TextColor(c.Theme().AccentText)
						}
					})
					if pill.Clicked() {
						tp.alert, tp.hasAlert = tier.Span, true
					}
				}
			})
		})
		foot := ui.Row(c.Key("time-actions")).Gap(8).FillWidth().AlignItems(ui.Center)
		foot.Children(func() {
			clear := ui.ButtonBase(c.Key("time-clear")).Radius(6).
				Cursor(ui.CursorPointer).FocusRing(true).Padding(5, 10).Disabled(!tp.hasTime)
			clear.Children(func() {
				ui.Text(c, locales.TimePopClear()).FontSize(12.5).TextColor(c.Theme().Danger)
			})
			if clear.Clicked() {
				a.applyTimePopover(blk, nil)
				tp.open = false
				tp.wheelOpen = false
			}
			ui.Box(c).Grow(1)
			done := ui.ButtonBase(c.Key("time-done")).Radius(6).
				Cursor(ui.CursorPointer).FocusRing(true).Padding(5, 14).Disabled(!tp.hasTime).
				Background(c.Theme().Accent)
			done.Children(func() {
				ui.Text(c, locales.TimePopDone()).FontSize(12.5).TextColor(c.Theme().AccentText)
			})
			if done.Clicked() {
				a.applyTimePopover(blk, &markdown.TimeSuffix{Time: tp.due, Alert: tp.alert, HasAlert: tp.hasAlert, Recur: blk.recur})
				tp.open = false
				tp.wheelOpen = false
			}
		})
	})
}

func timeWheelWidth(c *ui.Context) float32 {
	region := c.Region()
	if region.HourCycle != 12 && region.HourCycle != 24 {
		locale := region.Locale
		if locale == "" {
			locale = c.Locale()
		}
		region = i18n.RegionFor(locale)
	}
	if region.HourCycle == 12 {
		return timeWheel12PanelWidth
	}
	return timeWheelPanelWidth
}

func (a *app) timeWheelContent(c *ui.Context) {
	tp := &a.timePop
	theme := c.Theme()
	region := c.Region()
	locale := region.Locale
	if locale == "" {
		locale = c.Locale()
	}
	if region.HourCycle != 12 && region.HourCycle != 24 {
		region = i18n.RegionFor(locale)
	}
	twelve := region.HourCycle == 12
	am, pm := i18n.DayPeriods(locale, i18n.Abbreviated)

	wheelColumn := func(key string, part int, labels [3]string, step func(int), choose func(int)) ui.Element {
		column := ui.Column(c.Key("time-wheel-" + key)).Gap(6).Width(52).AlignItems(ui.Center).
			HandleInput(func(ev ui.InputEvent) bool {
				if ev.Kind != ui.InputScroll {
					return false
				}
				if steps := timeWheelScrollSteps(&tp.wheelAccum[part], ev.DY); steps != 0 {
					step(steps)
				}
				return true
			})
		column.Children(func() {
			for index, label := range labels {
				index, label := index, label
				item := ui.ButtonBase(c.Key("time-wheel-" + key + "-" + strconv.Itoa(index))).
					Width(52).Height(34).Radius(6).Cursor(ui.CursorPointer).FocusRing(true)
				if index == 1 {
					item.Background(theme.SurfaceHover)
				} else {
					item.Background(theme.Surface)
				}
				item.Children(func() {
					txt := ui.Text(c, label).FontSize(13).FontFeatures("tnum")
					if index != 1 {
						txt = txt.TextColor(theme.TextMuted)
					}
				})
				if item.Clicked() {
					choose(index - 1)
					setTimePart(&tp.due, tp.wheelDue.Hour(), tp.wheelDue.Minute())
					tp.hasTime = true
					tp.wheelAccum[part] = 0
				}
			}
		})
		return column
	}

	currentHour := tp.wheelDue.Hour()
	hourCount := 24
	hourIndex := currentHour
	if twelve {
		hourCount = 12
		hourIndex = (currentHour + 11) % 12
	}
	setHour := func(index int) {
		index = wrapTimeWheelIndex(index, hourCount)
		hour := index
		if twelve {
			hour = (index + 1) % 12
			if currentHour >= 12 {
				hour += 12
			}
		}
		setTimePart(&tp.wheelDue, hour, tp.wheelDue.Minute())
	}
	hourLabels := timeWheelLabels(hourIndex, hourCount, func(index int) string {
		if twelve {
			return timeWheelNumber(index + 1)
		}
		return timeWheelNumber(index)
	})

	minuteIndex := tp.wheelDue.Minute()
	setMinute := func(index int) {
		setTimePart(&tp.wheelDue, tp.wheelDue.Hour(), wrapTimeWheelIndex(index, 60))
	}
	minuteLabels := timeWheelLabels(minuteIndex, 60, timeWheelNumber)

	periodIndex := 0
	if currentHour >= 12 {
		periodIndex = 1
	}
	setPeriod := func(index int) {
		hour := tp.wheelDue.Hour() % 12
		if index == 1 {
			hour += 12
		}
		setTimePart(&tp.wheelDue, hour, tp.wheelDue.Minute())
	}
	periodLabels := timeWheelLabels(periodIndex, 2, func(index int) string {
		if index == 0 {
			return am
		}
		return pm
	})

	panel := ui.Column(c.Key("time-wheel-panel")).Gap(8).Padding(10).FillWidth()
	panel.Children(func() {
		ui.Text(c, locales.TimePopClock()).FontSize(11.5).TextColor(theme.TextMuted)
		row := ui.Row(c.Key("time-wheel-values")).Gap(4).AlignItems(ui.Center)
		row.Children(func() {
			wheelColumn("hour", 0, hourLabels, func(steps int) {
				index := tp.wheelDue.Hour()
				if twelve {
					index = (index + 11) % 12
				}
				setHour(index + steps)
			}, func(offset int) {
				index := tp.wheelDue.Hour()
				if twelve {
					index = (index + 11) % 12
				}
				setHour(index + offset)
			})
			ui.Text(c, ":").FontSize(13).TextColor(theme.TextMuted)
			wheelColumn("minute", 1, minuteLabels, func(steps int) {
				setMinute(tp.wheelDue.Minute() + steps)
			}, func(offset int) {
				setMinute(tp.wheelDue.Minute() + offset)
			})
			if twelve {
				wheelColumn("period", 2, periodLabels, func(steps int) {
					setPeriod(wrapTimeWheelIndex(periodIndex+steps, 2))
				}, func(offset int) {
					setPeriod(wrapTimeWheelIndex(periodIndex+offset, 2))
				})
			}
		})
	})
}

// tryTimeTrigger 行尾键入 @ 时把占位符就地转化为时间弹层（像表格输入的即时转换）：
// 占位符从可见文本移除后继续走常规提交（草稿内容不变，光标落回原位），
// 弹层跨帧弹出。代码块内、光标后还有字符（行中邮箱等）不触发。
func (a *app) tryTimeTrigger(blk liveBlock, cs, ce *int) bool {
	if blk.kind == bkCode || *cs != *ce || *cs == 0 {
		return false
	}
	rs := []rune(a.mdVisible)
	if *cs > len(rs) || rs[*cs-1] != '@' {
		return false
	}
	if *cs != len(rs) && rs[*cs] != '\n' {
		return false
	}
	a.mdVisible = string(rs[:*cs-1]) + string(rs[*cs:])
	*cs--
	*ce--
	a.openTimePopover(blk)
	return true
}

// replaceBlockTimeToken 保留软换行结构，只在实际包含令牌的源码行更新。
func replaceBlockTimeToken(raw string, lt *markdown.TimeSuffix) string {
	lines := strings.Split(raw, "\n")
	for i, line := range lines {
		if _, ok := markdown.FindTimeToken(line); ok {
			lines[i] = markdown.ReplaceTimeToken(line, lt)
			return strings.Join(lines, "\n")
		}
	}
	return markdown.ReplaceTimeToken(raw, lt)
}

// applyTimePopover 把弹层草稿写回绑定行；lt 为 nil 表示清除时间。
func (a *app) applyTimePopover(blk liveBlock, lt *markdown.TimeSuffix) {
	lines := strings.Split(a.activeTab().Text, "\n")
	if blk.start >= len(lines) {
		return
	}
	before := a.liveSnapshot()
	if lt != nil && lt.Recur == "" && blk.recur != "" {
		copy := *lt
		copy.Recur = blk.recur
		lt = &copy
	}
	updated := replaceBlockTimeToken(blk.raw, lt)
	caret := a.mdCaret
	blocks := a.mdBlocks()
	if a.mdActive >= 0 && a.mdActive < len(blocks) && blocks[a.mdActive].start == blk.start {
		if token, ok := markdown.FindTimeToken(blk.raw); ok {
			end := utf8.RuneCountInString(blk.raw[:token.End])
			if caret >= end {
				caret += utf8.RuneCountInString(updated) - utf8.RuneCountInString(blk.raw)
			}
		} else if caret >= utf8.RuneCountInString(blk.raw) {
			caret = utf8.RuneCountInString(updated)
		}
	}
	a.activeTab().Text = spliceDocLines(a.activeTab().Text, blk.start, blk.end, updated)
	a.resyncLive(caret)
	a.rememberLiveEdit(before, editCommand)
	a.onEdited()
}
