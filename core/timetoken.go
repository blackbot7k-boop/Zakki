// Package core 是 Zakki 的共享数据内核：时间令牌（@时刻、~提前量、^重复）与
// 重复实例展开。它是叶子模块——只依赖标准库，禁止依赖 MyGo、第三方包或
// zakki/internal/*；桌面的 internal/markdown 与未来的移动端（gomobile bind）
// 都从这一份实现消费语义。分层铁律见 docs/engineering-standard.md。
package core

import (
	"strings"
	"time"
	"unicode/utf8"
)

// 时间令牌：`@YYYY-MM-DD HH:mm` 是时刻，`~<提前量>` 是提醒，`^<规则>` 是重复
// （语法见 docs/adr/0002-recurring-time-tokens.md）。提醒档位限定在白名单里，
// 保证每个可解析值都有对应的弹层档位；手打白名单之外的值一律按普通文本处理，
// 不做部分识别。`^` 段不同：紧跟簇时若整体合法则纳入令牌，非法则保持普通文本。

// alertTiers 是提醒提前量档位，顺序即弹层展示顺序，也是解析白名单。
var alertTiers = []struct {
	suffix string
	span   time.Duration
}{
	{"0m", 0},
	{"5m", 5 * time.Minute},
	{"10m", 10 * time.Minute},
	{"30m", 30 * time.Minute},
	{"1h", time.Hour},
	{"1d", 24 * time.Hour},
}

// timeLayout 是时刻令牌的布局；加上 @ 前缀后定长 17 字节。
const timeLayout = "2006-01-02 15:04"

const timeTokenLen = len("@" + timeLayout)

// TimeSuffix 是解析出的时间与提醒（含可选重复规则）。
type TimeSuffix struct {
	Time     time.Time     // 时刻（本地时区墙上时间）
	Alert    time.Duration // 提醒提前量；0 表示准点
	HasAlert bool          // 是否设置了提醒（区分"没提醒"与"准点"）
	Recur    string        // ^ 重复规则原文（如 "^w@1-5"）；空表示不重复
}

// AlertTier 是提醒提前量档位（弹层展示与解析共用一份白名单）。
type AlertTier struct {
	Suffix string        // 令牌后缀，如 "10m"
	Span   time.Duration // 提前量
}

// AlertTiers 返回全部提醒档位，顺序即弹层展示顺序。
func AlertTiers() []AlertTier {
	out := make([]AlertTier, len(alertTiers))
	for i, tier := range alertTiers {
		out[i] = AlertTier{Suffix: tier.suffix, Span: tier.span}
	}
	return out
}

// SplitTimeSuffix 剥离 line 行尾的时间令牌簇，返回正文与解析结果；行尾没有
// 合法令牌时原样返回 line 与 nil。识别是全有或全无：时刻合法但提前量非法时
// 整体不算令牌；`^` 段不同——非法时仅忽略该段，令牌仍成立。
func SplitTimeSuffix(line string) (string, *TimeSuffix) {
	body := strings.TrimSuffix(line, "\r")
	end := len(body)
	for end > 0 && (body[end-1] == ' ' || body[end-1] == '\t') {
		end--
	}
	rest, lt, ok := splitTimeToken(body[:end])
	if !ok {
		return line, nil
	}
	return rest, lt
}

// splitTimeToken 在 s 末尾匹配「时刻 [空格 提醒] [空格 重复]」令牌串（规范顺序
// 时刻→提醒→重复）。正文是令牌前的部分，保留其原有结尾空白（写回时由
// JoinTimeSuffix 收敛）。
func splitTimeToken(s string) (rest string, lt *TimeSuffix, ok bool) {
	lt = &TimeSuffix{}
	timeEnd := len(s)
	// 行尾是 ^ 重复段时先剥掉（须整体合法；否则视为普通文本不剥）。
	if i := strings.LastIndex(s[:timeEnd], " ^"); i >= 0 {
		seg := s[i+1 : timeEnd]
		if _, err := ParseRule(seg); err == nil {
			lt.Recur = seg
			timeEnd = i
		}
	}
	// 行尾是提醒档位时先剥掉；时刻令牌紧随其前，且与提醒之间恰好一个空格。
	for _, tier := range alertTiers {
		if strings.HasSuffix(s[:timeEnd], "~"+tier.suffix) {
			cut := timeEnd - len(tier.suffix) - 1 // '~' 加档位之后的位置
			if cut > 0 && s[cut-1] == ' ' {
				lt.Alert, lt.HasAlert = tier.span, true
				timeEnd = cut - 1
			}
			break
		}
	}
	start := timeEnd - timeTokenLen
	if start < 0 || s[start] != '@' {
		return s, nil, false
	}
	due, err := time.ParseInLocation(timeLayout, s[start+1:timeEnd], time.Local)
	if err != nil {
		return s, nil, false
	}
	lt.Time = due
	return s[:start], lt, true
}

// JoinTimeSuffix 把时间令牌簇按规范格式（时刻→提醒→重复）写回行尾，正文右侧
// 空白收敛为一个空格；期望传入 SplitTimeSuffix 剥出的正文或本就不含令牌的正文。
// lt 为 nil 表示去掉时间。Recur 非法时按无重复写出（合法值只会来自解析）。
func JoinTimeSuffix(text string, lt *TimeSuffix) string {
	text = strings.TrimRight(text, " \t")
	if lt == nil {
		return text
	}
	s := text + " @" + lt.Time.Format(timeLayout)
	if lt.HasAlert {
		for _, tier := range alertTiers {
			if tier.span == lt.Alert {
				s += " ~" + tier.suffix
				break
			}
		}
	}
	if lt.Recur != "" {
		if _, err := ParseRule(lt.Recur); err == nil {
			s += " " + lt.Recur
		}
	}
	return s
}

// TimeToken 是正文内一个时间组件对应的源码字节区间。
// 同一条内容只取第一个合法时间；代码和链接中的示例保持为普通文字。
type TimeToken struct {
	Start, End int
	Value      TimeSuffix
}

// FindTimeToken 识别任意位置的时间令牌簇，不要求令牌位于行尾，不做保护区
// 排除（等价于 FindTimeTokenEx(text, nil)）。
func FindTimeToken(text string) (TimeToken, bool) {
	return FindTimeTokenEx(text, nil)
}

// FindTimeTokenEx 同 FindTimeToken，但 protected 给出 rune 区间列表（如行内
// 代码与链接），落在其中的 @ 不识别为令牌。
func FindTimeTokenEx(text string, protected [][2]int) (TimeToken, bool) {
	for from := 0; from < len(text); {
		rel := strings.IndexByte(text[from:], '@')
		if rel < 0 {
			break
		}
		at := from + rel
		from = at + 1
		if at+timeTokenLen > len(text) || isProtected(text, at, protected) {
			continue
		}
		backslashes := 0
		for i := at - 1; i >= 0 && text[i] == '\\'; i-- {
			backslashes++
		}
		if backslashes%2 != 0 {
			continue
		}
		end := at + timeTokenLen
		dueText := text[at+1 : end]
		due, err := time.ParseInLocation(timeLayout, dueText, time.Local)
		if err != nil || due.Format(timeLayout) != dueText {
			continue
		}
		if end < len(text) && (text[end] >= '0' && text[end] <= '9' || text[end] == ':' || text[end] == '~') {
			continue
		}
		token := TimeToken{Start: at, End: end, Value: TimeSuffix{Time: due}}
		spaceEnd := end
		for spaceEnd < len(text) && (text[spaceEnd] == ' ' || text[spaceEnd] == '\t') {
			spaceEnd++
		}
		if spaceEnd < len(text) && text[spaceEnd] == '~' {
			if spaceEnd != end+1 || text[end] != ' ' {
				continue
			}
			matched := false
			for _, tier := range alertTiers {
				suffix := "~" + tier.suffix
				if strings.HasPrefix(text[spaceEnd:], suffix) {
					token.End = spaceEnd + len(suffix)
					token.Value.Alert, token.Value.HasAlert = tier.span, true
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}
		// 可选 ^ 重复段：恰好一个空格分隔；段内不允许空格。整体非法时忽略该段，
		// 令牌仍成立（^ 段文本保持普通文字，等待移动端或桌面弹层重写）。
		if ruleEnd, seg, ok := scanRecurSegment(text, token.End); ok {
			token.End = ruleEnd
			token.Value.Recur = seg
		}
		return token, true
	}
	return TimeToken{}, false
}

// scanRecurSegment 在 at 处尝试读取「空格 + ^段」。段须通过 ParseRule 校验；
// 返回段的结束偏移与原文。
func scanRecurSegment(text string, at int) (end int, seg string, ok bool) {
	if at >= len(text) || text[at] != ' ' || at+1 >= len(text) || text[at+1] != '^' {
		return at, "", false
	}
	j := at + 2
	for j < len(text) && text[j] != ' ' && text[j] != '~' {
		j++
	}
	cand := text[at+1 : j]
	if _, err := ParseRule(cand); err != nil {
		return at, "", false
	}
	return j, cand, true
}

// isProtected 报告字节偏移 at（换算为 rune 下标）是否落在保护区间的半开区间内。
func isProtected(text string, at int, protected [][2]int) bool {
	pos := utf8.RuneCountInString(text[:at])
	for _, p := range protected {
		if p[0] <= pos && pos < p[1] {
			return true
		}
	}
	return false
}

// ReplaceTimeToken 原位更新或清除时间；没有时间时按原有规则追加到正文末尾。
// 组件后面的文字不移动，清除时仅合并组件两侧重复的分隔空白。不做保护区排除。
func ReplaceTimeToken(text string, value *TimeSuffix) string {
	return ReplaceTimeTokenEx(text, value, nil)
}

// ReplaceTimeTokenEx 同 ReplaceTimeToken，但用 protected 排除保护区内的 @。
func ReplaceTimeTokenEx(text string, value *TimeSuffix, protected [][2]int) string {
	token, ok := FindTimeTokenEx(text, protected)
	if !ok {
		return JoinTimeSuffix(text, value)
	}
	before, after := text[:token.Start], text[token.End:]
	if value != nil {
		return before + strings.TrimPrefix(JoinTimeSuffix("", value), " ") + after
	}
	if strings.TrimRight(before, " \t") != before && strings.TrimLeft(after, " \t") != after {
		before = strings.TrimRight(before, " \t")
	}
	return strings.TrimRight(before+after, " \t")
}
