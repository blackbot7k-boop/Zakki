package markdown

import (
	"strings"
	"time"
)

// 行尾时间令牌：`@YYYY-MM-DD HH:mm` 是时刻，`~<提前量>` 是提醒。
// 令牌只认行尾，提前量限定在档位白名单里，保证每个可解析值都有对应的弹层档位；
// 手打白名单之外的值一律按普通文本处理，不做部分识别。

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

// timeLayout 是时刻令牌的布局；加上 @ 前缀后定长 17 字节，解析按定长从行尾反扫。
const timeLayout = "2006-01-02 15:04"

const timeTokenLen = len("@" + timeLayout)

// TimeSuffix 是一行行尾解析出的时间与提醒。
type TimeSuffix struct {
	Time     time.Time     // 时刻（本地时区墙上时间）
	Alert    time.Duration // 提醒提前量；0 表示准点
	HasAlert bool          // 是否设置了提醒（区分"没提醒"与"准点"）
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

// SplitTimeSuffix 剥离 line 行尾的时间令牌，返回正文与解析结果；行尾没有合法
// 令牌时原样返回 line 与 nil。识别是全有或全无：时刻合法但提前量不合法时整体不算令牌。
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

// splitTimeToken 在 s 末尾匹配「时刻 [空格 提醒]」令牌串；正文是令牌前的部分，
// 保留其原有结尾空白（写回时由 JoinTimeSuffix 收敛）。
func splitTimeToken(s string) (rest string, lt *TimeSuffix, ok bool) {
	lt = &TimeSuffix{}
	timeEnd := len(s)
	// 行尾是提醒档位时先剥掉；时刻令牌紧随其前，且与提醒之间恰好一个空格。
	for _, tier := range alertTiers {
		if strings.HasSuffix(s, "~"+tier.suffix) {
			cut := len(s) - len(tier.suffix) - 1 // '~' 加档位之后的位置
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

// JoinTimeSuffix 把时间令牌按规范格式写回行尾，正文右侧空白收敛为一个空格；
// 期望传入 SplitTimeSuffix 剥出的正文或本就不含令牌的正文。lt 为 nil 表示去掉时间。
// Alert 不在档位表内时按无提醒写出（档位来自白名单，正常不会走到）。
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
	return s
}
