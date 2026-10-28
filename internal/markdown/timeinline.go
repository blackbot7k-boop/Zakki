package markdown

import (
	"unicode/utf8"
	"zakki/core"
)

// 行内时间令牌的语义实现在共享内核 zakki/core；本文件只保留桌面的调用面，
// 并把本包的行内扫描（inlineSpans）换算成 core 认识的抽象保护区——
// 行内代码与链接中的时间示例保持普通文字，这条桌面知识不进内核。

// TimeToken 是正文内一个时间组件对应的源码字节区间。
type TimeToken = core.TimeToken

// FindTimeToken 识别任意位置的固定格式时间及可选提醒与重复段；行内代码与
// 链接中的示例保持普通文字，转义的 @ 不识别。
func FindTimeToken(text string) (TimeToken, bool) {
	return core.FindTimeTokenEx(text, protectedSpans(text))
}

// ReplaceTimeToken 原位更新或清除时间；没有时间时按原有规则追加到正文末尾。
// 组件后面的文字不移动，清除时仅合并组件两侧重复的分隔空白。
func ReplaceTimeToken(text string, value *TimeSuffix) string {
	return core.ReplaceTimeTokenEx(text, value, protectedSpans(text))
}

// protectedSpans 把行内代码与链接的 rune 区间转换成 core 的字节区间。
func protectedSpans(text string) [][2]int {
	offsets := make([]int, 0, utf8.RuneCountInString(text)+1)
	for i := range text {
		if utf8.RuneStart(text[i]) {
			offsets = append(offsets, i)
		}
	}
	offsets = append(offsets, len(text))
	var out [][2]int
	for _, sp := range inlineSpans(text) {
		if sp.Kind == spanCode || sp.Kind == spanLink {
			out = append(out, [2]int{offsets[sp.Start], offsets[sp.End]})
		}
	}
	return out
}
