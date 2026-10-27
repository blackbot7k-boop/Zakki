package app

// 时间 chip：预览与聚焦共用的行内时间组件文案和状态；
// 点击编辑入口在 timepopover。状态语义见 GLOSSARY：只有未完成任务行会过期。

import (
	"strconv"
	"time"

	"zakki/locales"
)

type timeChipState int

const (
	chipFuture  timeChipState = iota // 时刻在今天之后
	chipToday                        // 时刻就在今天（未过）
	chipOverdue                      // 任务行的截止已过且未完成
	chipNeutral                      // 已完成任务，或非任务行已过去的时刻
)

// timeChipStateFor 分类 chip 状态；now 注入以便测试。无时间的块没有状态，
// 统一归为 chipFuture（调用方本就不会为它们渲染 chip）。
func timeChipStateFor(blk liveBlock, now time.Time) timeChipState {
	if !blk.hasTime {
		return chipFuture
	}
	if blk.kind == bkTask && blk.checked {
		return chipNeutral
	}
	if blk.due.Before(now) {
		if blk.kind == bkTask {
			return chipOverdue
		}
		return chipNeutral
	}
	if sameWallDay(blk.due, now) {
		return chipToday
	}
	return chipFuture
}

func sameWallDay(a, b time.Time) bool {
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	return ay == by && am == bm && ad == bd
}

// timeChipLabel 智能文案：今天/明天/周X/M月D日，跨年带年份；已完成只留时刻，
// 已过去的时刻用绝对日期（过去的星期几没有信息量）。
func timeChipLabel(now, due time.Time, done bool) string {
	hm := due.Format("15:04")
	if done {
		return hm
	}
	if !due.Before(now) {
		if sameWallDay(due, now) {
			return locales.TimeChipToday(hm)
		}
		if sameWallDay(due, now.AddDate(0, 0, 1)) {
			return locales.TimeChipTomorrow(hm)
		}
		if due.Year() == now.Year() && due.Sub(now) < 7*24*time.Hour {
			return locales.TimeChipWeekday(timeChipWeekdayName(due.Weekday()), hm)
		}
	}
	if due.Year() == now.Year() {
		return locales.TimeChipDate(timeChipMonthName(due.Month()), strconv.Itoa(due.Day()), hm)
	}
	return locales.TimeChipDateYear(timeChipMonthName(due.Month()), strconv.Itoa(due.Day()), strconv.Itoa(due.Year()), hm)
}

// timeChipWeekdayName 周一为 1，对齐 locale 键 timeWeekday1..7。
func timeChipWeekdayName(w time.Weekday) string {
	switch (int(w) + 6) % 7 {
	case 0:
		return locales.TimeWeekday1()
	case 1:
		return locales.TimeWeekday2()
	case 2:
		return locales.TimeWeekday3()
	case 3:
		return locales.TimeWeekday4()
	case 4:
		return locales.TimeWeekday5()
	case 5:
		return locales.TimeWeekday6()
	default:
		return locales.TimeWeekday7()
	}
}

func timeChipMonthName(m time.Month) string {
	switch m {
	case time.February:
		return locales.TimeMonth2()
	case time.March:
		return locales.TimeMonth3()
	case time.April:
		return locales.TimeMonth4()
	case time.May:
		return locales.TimeMonth5()
	case time.June:
		return locales.TimeMonth6()
	case time.July:
		return locales.TimeMonth7()
	case time.August:
		return locales.TimeMonth8()
	case time.September:
		return locales.TimeMonth9()
	case time.October:
		return locales.TimeMonth10()
	case time.November:
		return locales.TimeMonth11()
	case time.December:
		return locales.TimeMonth12()
	default:
		return locales.TimeMonth1()
	}
}
