package core

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Rule 是 ^ 段的解析结果。
type Rule struct {
	Every    int    `json:"every"`
	Unit     string `json:"unit"`
	Weekdays []int  `json:"weekdays,omitempty"`
	End      string `json:"end,omitempty"`
}

// ParseRule 严格解析重复规则。
func ParseRule(s string) (string, error) {
	r, err := parseRule(s)
	if err != nil {
		return "", err
	}
	return marshal(r), nil
}

func parseRule(s string) (Rule, error) {
	var r Rule
	if !strings.HasPrefix(s, "^") {
		return r, fmt.Errorf("规则须以 ^ 开头")
	}
	body := s[1:]
	if i := strings.IndexByte(body, '<'); i >= 0 {
		r.End = body[i+1:]
		if r.End == "" {
			return r, fmt.Errorf("结束日期为空")
		}
		body = body[:i]
	}
	if i := strings.IndexByte(body, '@'); i >= 0 {
		var err error
		r.Weekdays, err = parseWeekdays(body[i+1:])
		if err != nil {
			return r, err
		}
		body = body[:i]
	}
	i := 0
	for i < len(body) && body[i] >= '0' && body[i] <= '9' {
		i++
	}
	r.Every = 1
	if i > 0 {
		if body[0] == '0' {
			return r, fmt.Errorf("次数不可前导零")
		}
		n, err := strconv.Atoi(body[:i])
		if err != nil {
			return r, fmt.Errorf("次数非法: %w", err)
		}
		r.Every = n
	}
	r.Unit = body[i:]
	return r, validateRule(r)
}

func parseWeekdays(s string) ([]int, error) {
	if s == "" {
		return nil, fmt.Errorf("周几集为空")
	}
	seen := [8]bool{}
	for _, part := range strings.Split(s, ",") {
		bounds := strings.Split(part, "-")
		if len(bounds) > 2 {
			return nil, fmt.Errorf("周几区间非法")
		}
		a, err := weekdayNumber(bounds[0])
		if err != nil {
			return nil, err
		}
		b := a
		if len(bounds) == 2 {
			b, err = weekdayNumber(bounds[1])
			if err != nil {
				return nil, err
			}
		}
		if a > b {
			return nil, fmt.Errorf("周几区间倒序")
		}
		for d := a; d <= b; d++ {
			seen[d] = true
		}
	}
	out := make([]int, 0, 7)
	for d := 1; d <= 7; d++ {
		if seen[d] {
			out = append(out, d)
		}
	}
	return out, nil
}

func weekdayNumber(s string) (int, error) {
	if len(s) != 1 || s[0] < '1' || s[0] > '7' {
		return 0, fmt.Errorf("周几须为 1-7: %q", s)
	}
	return int(s[0] - '0'), nil
}

func validateRule(r Rule) error {
	if r.Every < 1 || r.Every > 999 {
		return fmt.Errorf("次数须为 1..999")
	}
	switch r.Unit {
	case "d", "w", "mo", "y":
	default:
		return fmt.Errorf("单位须为 d/w/mo/y")
	}
	if len(r.Weekdays) > 0 {
		if r.Unit != "w" || r.Every != 1 {
			return fmt.Errorf("周几集仅支持每周")
		}
		prev := 0
		for _, d := range r.Weekdays {
			if d < 1 || d > 7 || d <= prev {
				return fmt.Errorf("周几集须升序去重 1..7")
			}
			prev = d
		}
	}
	if r.End != "" {
		if len(r.End) != 10 || r.End[4] != '-' || r.End[7] != '-' {
			return fmt.Errorf("结束日期格式非法")
		}
		for i, c := range r.End {
			if i == 4 || i == 7 {
				continue
			}
			if c < '0' || c > '9' {
				return fmt.Errorf("结束日期格式非法")
			}
		}
		d, err := time.ParseInLocation("2006-01-02", r.End, location())
		if err != nil || d.Format("2006-01-02") != r.End {
			return fmt.Errorf("结束日期非法")
		}
	}
	return nil
}

func decodedRule(s string) (Rule, error) {
	var r Rule
	if err := unmarshal(s, &r); err != nil {
		return r, err
	}
	if err := validateRule(r); err != nil {
		return r, err
	}
	return r, nil
}

// Expand 返回 (now, now+horizonDays] 内至多 maxCount 个实例的 Unix 毫秒 JSON 数组。
func Expand(ruleJSON string, anchorMs, nowMs int64, horizonDays, maxCount int) (string, error) {
	r, err := decodedRule(ruleJSON)
	if err != nil {
		return "", err
	}
	if horizonDays > 3660 || maxCount > 1000 {
		return "", fmt.Errorf("地平线或数量非法")
	}
	out := make([]int64, 0)
	if horizonDays <= 0 || maxCount <= 0 {
		return marshal(out), nil
	}
	anchor, now := recurrenceAnchor(anchorMs), time.UnixMilli(nowMs).In(location())
	limit := now.AddDate(0, 0, horizonDays)
	start := now
	if anchor.After(start) {
		start = anchor.AddDate(0, 0, -1)
	}
	scanDays(r, anchor, start, limit, func(t time.Time) bool {
		if t.After(now) {
			out = append(out, t.UnixMilli())
		}
		return len(out) < maxCount
	})
	return marshal(out), nil
}

// Occurrence 返回当前实例（不晚于 now 的最近一个；若尚未开始则取首个）。
func Occurrence(ruleJSON string, anchorMs, nowMs int64) (string, error) {
	r, err := decodedRule(ruleJSON)
	if err != nil {
		return "", err
	}
	anchor, now := recurrenceAnchor(anchorMs), time.UnixMilli(nowMs).In(location())
	if r.End != "" {
		end, _ := time.ParseInLocation("2006-01-02", r.End, location())
		end = end.AddDate(0, 0, 1).Add(-time.Nanosecond)
		if now.After(end) {
			now = end
		}
	}
	if current, ok := nearestOccurrence(r, anchor, now, false); ok {
		return marshal(occurrence{DueMs: current.UnixMilli()}), nil
	}
	if first, ok := nearestOccurrence(r, anchor, anchor.Add(-time.Nanosecond), true); ok {
		return marshal(occurrence{DueMs: first.UnixMilli()}), nil
	}
	return marshal(occurrence{Ended: true}), nil
}

// Advance 返回当前实例之后的首次实例；序列终止时 ended 为 true。
func Advance(ruleJSON string, anchorMs, nowMs int64) (string, error) {
	r, err := decodedRule(ruleJSON)
	if err != nil {
		return "", err
	}
	currentJSON, err := Occurrence(ruleJSON, anchorMs, nowMs)
	if err != nil {
		return "", err
	}
	var current occurrence
	_ = unmarshal(currentJSON, &current)
	if current.Ended {
		return currentJSON, nil
	}
	anchor := recurrenceAnchor(anchorMs)
	after := time.UnixMilli(current.DueMs).In(location())
	if next, ok := nearestOccurrence(r, anchor, after, true); ok {
		return marshal(occurrence{DueMs: next.UnixMilli()}), nil
	}
	return marshal(occurrence{Ended: true}), nil
}

func recurrenceAnchor(ms int64) time.Time {
	t := time.UnixMilli(ms).In(location())
	return t.Add(-time.Duration(t.Second())*time.Second - time.Duration(t.Nanosecond()))
}

type occurrence struct {
	DueMs int64 `json:"dueMs"`
	Ended bool  `json:"ended"`
}

func firstOccurrence(r Rule, anchor time.Time) (time.Time, bool) {
	limit := anchor.AddDate(0, 0, 3660)
	var first time.Time
	scanDays(r, anchor, anchor.AddDate(0, 0, -1), limit, func(t time.Time) bool { first = t; return false })
	return first, !first.IsZero()
}

// scanDays 以有限日历窗口扫描，月份/年份的间隔由月份/年份差直接判定，不按锚点年龄迭代。
func scanDays(r Rule, anchor, start, limit time.Time, emit func(time.Time) bool) {
	loc := location()
	day := time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, loc)
	anchorDay := time.Date(anchor.Year(), anchor.Month(), anchor.Day(), 0, 0, 0, 0, loc)
	var endDay time.Time
	if r.End != "" {
		endDay, _ = time.ParseInLocation("2006-01-02", r.End, loc)
	}
	for !day.After(limit) {
		if !day.Before(anchorDay) && (endDay.IsZero() || !day.After(endDay)) && matchesDay(r, anchorDay, day) {
			t := time.Date(day.Year(), day.Month(), day.Day(), anchor.Hour(), anchor.Minute(), 0, 0, loc)
			// 每次从锚点钟点重建，DST gap 沿用 time.Date 归一化，后续日期恢复原钟点。
			if t.Year() == day.Year() && t.Month() == day.Month() && t.Day() == day.Day() && !t.Before(anchor) && !t.Before(start) && !t.After(limit) {
				if !emit(t) {
					return
				}
			}
		}
		if !endDay.IsZero() && day.After(endDay) {
			return
		}
		day = day.AddDate(0, 0, 1)
	}
}

func matchesDay(r Rule, anchor, day time.Time) bool {
	switch r.Unit {
	case "d":
		return day.Sub(anchor).Hours()/24 >= 0 && calendarDays(anchor, day)%r.Every == 0
	case "w":
		if len(r.Weekdays) > 0 {
			wd := int(day.Weekday())
			if wd == 0 {
				wd = 7
			}
			return sort.SearchInts(r.Weekdays, wd) < len(r.Weekdays) && r.Weekdays[sort.SearchInts(r.Weekdays, wd)] == wd
		}
		return calendarDays(anchor, day)%(7*r.Every) == 0
	case "mo":
		return day.Day() == anchor.Day() && ((day.Year()-anchor.Year())*12+int(day.Month()-anchor.Month()))%r.Every == 0
	case "y":
		return day.Month() == anchor.Month() && day.Day() == anchor.Day() && (day.Year()-anchor.Year())%r.Every == 0
	}
	return false
}
func calendarDays(a, b time.Time) int {
	ordinal := func(t time.Time) int { y := t.Year() - 1; return 365*y + y/4 - y/100 + y/400 + t.YearDay() }
	return ordinal(b) - ordinal(a)
}

// nearestOccurrence 按周期直接定位，不沿锚点到现在逐日推进。
func nearestOccurrence(r Rule, anchor, ref time.Time, forward bool) (time.Time, bool) {
	loc := location()
	anchorDay := time.Date(anchor.Year(), anchor.Month(), anchor.Day(), 0, 0, 0, 0, loc)
	refDay := time.Date(ref.Year(), ref.Month(), ref.Day(), 0, 0, 0, 0, loc)
	if len(r.Weekdays) > 0 {
		start := refDay
		if forward {
			start = start.AddDate(0, 0, -1)
		}
		for i := 0; i < 8; i++ {
			day := start
			if forward {
				day = day.AddDate(0, 0, i+1)
			} else {
				day = day.AddDate(0, 0, -i)
			}
			if t, ok := candidate(r, anchor, day, ref, forward); ok {
				return t, true
			}
		}
		return time.Time{}, false
	}
	var diff int
	switch r.Unit {
	case "d":
		diff = calendarDays(anchorDay, refDay) / r.Every
	case "w":
		diff = calendarDays(anchorDay, refDay) / (7 * r.Every)
	case "mo":
		diff = ((ref.Year()-anchor.Year())*12 + int(ref.Month()-anchor.Month())) / r.Every
	case "y":
		diff = (ref.Year() - anchor.Year()) / r.Every
	}
	if diff < 0 {
		diff = 0
	}
	// 月末和闰日最多按 400 年公历周期寻找；较大间隔也仅遍历周期候选。
	for step := 0; step < 402; step++ {
		k := diff
		if forward {
			k += step
		} else {
			k -= step
		}
		if k < 0 {
			if forward {
				continue
			}
			break
		}
		var day time.Time
		switch r.Unit {
		case "d":
			day = anchorDay.AddDate(0, 0, k*r.Every)
		case "w":
			day = anchorDay.AddDate(0, 0, k*7*r.Every)
		case "mo":
			day = anchorDay.AddDate(0, k*r.Every, 0)
		case "y":
			day = anchorDay.AddDate(k*r.Every, 0, 0)
		}
		if (r.Unit == "mo" || r.Unit == "y") && day.Day() != anchor.Day() || r.Unit == "y" && day.Month() != anchor.Month() {
			continue
		}
		if t, ok := candidate(r, anchor, day, ref, forward); ok {
			return t, true
		}
	}
	return time.Time{}, false
}
func candidate(r Rule, anchor, day, ref time.Time, forward bool) (time.Time, bool) {
	t := time.Date(day.Year(), day.Month(), day.Day(), anchor.Hour(), anchor.Minute(), 0, 0, location())
	if t.Year() != day.Year() || t.Month() != day.Month() || t.Day() != day.Day() || t.Before(anchor) {
		return time.Time{}, false
	}
	if forward && !t.After(ref) || !forward && t.After(ref) {
		return time.Time{}, false
	}
	if r.End != "" && day.Format("2006-01-02") > r.End {
		return time.Time{}, false
	}
	if len(r.Weekdays) > 0 && !matchesDay(r, anchor, day) {
		return time.Time{}, false
	}
	return t, true
}
func marshal(v any) string            { b, _ := json.Marshal(v); return string(b) }
func unmarshal(s string, v any) error { return json.Unmarshal([]byte(s), v) }
