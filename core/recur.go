package core

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

// Rule 是 ^ 段的解析结果。对外一律走 JSON（gomobile 绑定形状：只跨字符串与数值）。
type Rule struct {
	Every    int    `json:"every"`              // 间隔数，缺省 1
	Unit     string `json:"unit"`               // d | w | mo | y
	Weekdays []int  `json:"weekdays,omitempty"` // ISO 周几 1-7（1=周一），仅 Unit=w 且 Every=1
	End      string `json:"end,omitempty"`      // YYYY-MM-DD，含当日，可空
}

// ParseRule 解析 ^ 段文本（如 "^w@1-5"、"^3d"、"^1mo<2025-12-31"），返回 Rule 的 JSON。
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
		return r, fmt.Errorf("规则须以 ^ 开头: %q", s)
	}
	body := s[1:]
	if i := strings.Index(body, "<"); i >= 0 { // 结束日期
		r.End = strings.TrimSpace(body[i+1:])
		if len(r.End) != 10 {
			return r, fmt.Errorf("结束日期须为 YYYY-MM-DD: %q", r.End)
		}
		body = body[:i]
	}
	if i := strings.Index(body, "@"); i >= 0 { // 周几集
		wd, err := parseWeekdays(body[i+1:])
		if err != nil {
			return r, err
		}
		r.Weekdays = wd
		body = body[:i]
	}
	n := 1
	unit := body
	if len(body) > 0 && body[0] >= '0' && body[0] <= '9' {
		j := 0
		for j < len(body) && body[j] >= '0' && body[j] <= '9' {
			j++
		}
		if _, err := fmt.Sscanf(body[:j], "%d", &n); err != nil || n <= 0 {
			return r, fmt.Errorf("次数须为正整数: %q", body[:j])
		}
		unit = body[j:]
	}
	switch unit {
	case "d", "w", "mo", "y":
		r.Every, r.Unit = n, unit
	default:
		return r, fmt.Errorf("单位须为 d/w/mo/y: %q", unit)
	}
	if len(r.Weekdays) > 0 && (r.Unit != "w" || r.Every != 1) {
		return r, fmt.Errorf("周几集仅支持 ^w（v1）: %q", s)
	}
	return r, nil
}

func parseWeekdays(s string) ([]int, error) {
	var out []int
	seen := map[int]bool{}
	for _, part := range strings.Split(s, ",") {
		if i := strings.Index(part, "-"); i > 0 { // 区间 a-b
			var a, b int
			if _, err := fmt.Sscanf(part, "%d-%d", &a, &b); err != nil || a < 1 || b > 7 || a > b {
				return nil, fmt.Errorf("周几区间非法: %q", part)
			}
			for d := a; d <= b; d++ {
				if !seen[d] {
					seen[d] = true
					out = append(out, d)
				}
			}
			continue
		}
		var d int
		if _, err := fmt.Sscanf(part, "%d", &d); err != nil || d < 1 || d > 7 {
			return nil, fmt.Errorf("周几须为 1-7: %q", part)
		}
		if !seen[d] {
			seen[d] = true
			out = append(out, d)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("周几集为空: %q", s)
	}
	sort.Ints(out)
	return out, nil
}

// Expand 从锚点展开未来实例：返回 [now, now+horizonDays] 内、不超过 maxCount 个的
// 时刻（Unix 毫秒 JSON 数组，升序）。语义以 ADR-0002 为准：错过不补、月末/闰日跳过、
// 结束日期含当日、补勾不追账由调用方（勾选改写锚点）负责。
func Expand(ruleJSON string, anchorMs, nowMs int64, horizonDays, maxCount int) (string, error) {
	var r Rule
	if err := unmarshal(ruleJSON, &r); err != nil {
		return "", err
	}
	anchor := time.UnixMilli(anchorMs)
	now := time.UnixMilli(nowMs)
	horizon := now.AddDate(0, 0, horizonDays)
	var endDate time.Time
	if r.End != "" {
		t, err := time.ParseInLocation("2006-01-02", r.End, time.Local)
		if err != nil {
			return "", fmt.Errorf("结束日期非法: %w", err)
		}
		endDate = t.AddDate(0, 0, 1).Add(-time.Nanosecond) // 含当日全天
	}
	var out []int64
	// emit 返回 false 表示序列应终止（越过结束日期或达到配额）
	emit := func(t time.Time) bool {
		if !endDate.IsZero() && t.After(endDate) {
			return false
		}
		if t.After(now) && !t.After(horizon) {
			out = append(out, t.UnixMilli())
		}
		return len(out) < maxCount
	}
	clock := func(day time.Time) time.Time {
		return time.Date(day.Year(), day.Month(), day.Day(), anchor.Hour(), anchor.Minute(), 0, 0, time.Local)
	}
	cont := true
	switch {
	case r.Unit == "d":
		for t := anchor; cont; t = t.AddDate(0, 0, r.Every) {
			if t.After(horizon) {
				break
			}
			cont = emit(t)
		}
	case r.Unit == "w" && len(r.Weekdays) > 0: // 周几集（Every=1）
		anchorDay := time.Date(anchor.Year(), anchor.Month(), anchor.Day(), 0, 0, 0, 0, time.Local)
		weekStart := anchorDay.AddDate(0, 0, -(int(anchorDay.Weekday())+6)%7) // 锚点 ISO 周的周一
		for ws := weekStart; cont; ws = ws.AddDate(0, 0, 7) {
			for _, wd := range r.Weekdays {
				day := ws.AddDate(0, 0, wd-1)
				if day.Before(anchorDay) {
					continue
				}
				if day.After(horizon) {
					return finish(out)
				}
				if cont = emit(clock(day)); !cont {
					return finish(out)
				}
			}
		}
		return finish(out)
	case r.Unit == "w":
		for t := anchor; cont; t = t.AddDate(0, 0, 7*r.Every) {
			if t.After(horizon) {
				break
			}
			cont = emit(t)
		}
	case r.Unit == "mo":
		for k, y, m := 0, anchor.Year(), anchor.Month(); ; {
			if k%r.Every == 0 {
				t, ok := monthDay(y, m, anchor.Day(), anchor)
				if ok {
					if t.After(horizon) {
						break
					}
					if cont = emit(t); !cont {
						break
					}
				}
			}
			k++
			y, m = stepMonth(y, m, 1)
		}
	case r.Unit == "y":
		for k, y := 0, anchor.Year(); ; {
			if k%r.Every == 0 {
				t, ok := monthDay(y, anchor.Month(), anchor.Day(), anchor)
				if ok {
					if t.After(horizon) {
						break
					}
					if cont = emit(t); !cont {
						break
					}
				}
			}
			k++
			y++
		}
	default:
		return "", fmt.Errorf("未知规则: %+v", r)
	}
	return finish(out)
}

func finish(out []int64) (string, error) { return marshal(out), nil }

// monthDay 构造 y-m-d 保持锚点钟点的时刻；日期不存在（月末/闰日跳过语义）返回 ok=false。
func monthDay(y int, m time.Month, d int, anchor time.Time) (time.Time, bool) {
	first := time.Date(y, m, 1, 0, 0, 0, 0, time.Local)
	if d > first.AddDate(0, 1, -1).Day() {
		return time.Time{}, false
	}
	return time.Date(y, m, d, anchor.Hour(), anchor.Minute(), 0, 0, time.Local), true
}

func stepMonth(y int, m time.Month, n int) (int, time.Month) {
	total := int(m) - 1 + n
	return y + total/12, time.Month(total%12 + 1)
}

func marshal(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func unmarshal(s string, v any) error { return json.Unmarshal([]byte(s), v) }
