package markdown

import (
	"strings"
	"time"
	"unicode/utf8"
)

// TimeToken 是正文内一个时间组件对应的源码字节区间。
// 同一条内容只取第一个合法时间；代码和链接中的示例保持为普通文字。
type TimeToken struct {
	Start, End int
	Value      TimeSuffix
}

// FindTimeToken 识别任意位置的固定格式时间及可选提醒，不要求令牌位于行尾。
// 提醒仍须紧跟时刻且只用一个空格分隔；不支持的档位使整组保持普通文字。
func FindTimeToken(text string) (TimeToken, bool) {
	spans := inlineSpans(text)
	protected := func(at int) bool {
		pos := utf8.RuneCountInString(text[:at])
		for _, sp := range spans {
			if (sp.Kind == spanCode || sp.Kind == spanLink) && sp.Start <= pos && pos < sp.End {
				return true
			}
		}
		return false
	}
	for from := 0; from < len(text); {
		rel := strings.IndexByte(text[from:], '@')
		if rel < 0 {
			break
		}
		at := from + rel
		from = at + 1
		if at+timeTokenLen > len(text) || protected(at) {
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
		return token, true
	}
	return TimeToken{}, false
}

// ReplaceTimeToken 原位更新或清除时间；没有时间时按原有规则追加到正文末尾。
// 组件后面的文字不移动，清除时仅合并组件两侧重复的分隔空白。
func ReplaceTimeToken(text string, value *TimeSuffix) string {
	token, ok := FindTimeToken(text)
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
