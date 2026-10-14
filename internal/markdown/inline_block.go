package markdown

// 轻量 markdown 引擎：
//   - inlineSpans   解析行内标记（粗体/斜体/代码/链接）
//   - editorRanges  生成编辑器的语法高亮区间
//   - parseBlocks   把文本切成块级元素供预览渲染
//
// 只覆盖便签常用的子集：标题、列表、待办、引用、代码块、分隔线、行内标记。

import "strings"

// ---- 行内标记 ----

type spanKind int

const (
	spanBold spanKind = iota
	spanItalic
	spanCode
	spanLink
)

type mSpan struct {
	Start, End int // rune 区间，含标记符本身
	Kind       spanKind
	Href       string
}

// inlineSpans 找出一段文字里的 **粗体**、*斜体*、`代码` 和 [文本](链接)。
func inlineSpans(s string) []mSpan {
	rs := []rune(s)
	var out []mSpan
	i := 0
	for i < len(rs) {
		switch {
		case rs[i] == '`':
			if j := runeIndexOf(rs, '`', i+1); j > 0 {
				out = append(out, mSpan{Start: i, End: j + 1, Kind: spanCode})
				i = j + 1
				continue
			}
		case rs[i] == '*' && i+1 < len(rs) && rs[i+1] == '*':
			if j := runeSeq(rs, '*', 2, i+2); j > 0 {
				out = append(out, mSpan{Start: i, End: j + 2, Kind: spanBold})
				i = j + 2
				continue
			}
		case rs[i] == '*' || rs[i] == '_':
			if j := runeIndexOf(rs, rs[i], i+1); j > 0 && j > i+1 {
				out = append(out, mSpan{Start: i, End: j + 1, Kind: spanItalic})
				i = j + 1
				continue
			}
		case rs[i] == '[':
			if cb := runeIndexOf(rs, ']', i+1); cb > 0 && cb+1 < len(rs) && rs[cb+1] == '(' {
				if pe := runeIndexOf(rs, ')', cb+2); pe > 0 {
					out = append(out, mSpan{Start: i, End: pe + 1, Kind: spanLink, Href: string(rs[cb+2 : pe])})
					i = pe + 1
					continue
				}
			}
		}
		i++
	}
	return out
}

// spanText 去掉行内标记符后的纯文本。
func spanText(s string, sp mSpan) string {
	rs := []rune(s)
	inner := rs[sp.Start:sp.End]
	switch sp.Kind {
	case spanBold:
		inner = inner[2 : len(inner)-2]
	case spanItalic:
		inner = inner[1 : len(inner)-1]
	case spanCode:
		inner = inner[1 : len(inner)-1]
	case spanLink:
		if cb := runeIndexOf(inner, ']', 0); cb > 0 {
			inner = inner[1:cb]
		}
	}
	return string(inner)
}

func runeIndexOf(rs []rune, target rune, from int) int {
	for i := from; i < len(rs); i++ {
		if rs[i] == target {
			return i
		}
	}
	return -1
}

func runeSeq(rs []rune, target rune, n, from int) int {
	run := 0
	for i := from; i < len(rs); i++ {
		if rs[i] == target {
			run++
			if run == n {
				return i - n + 1
			}
		} else {
			run = 0
		}
	}
	return -1
}

// ---- 块级解析（预览用）----

func orderedNumber(trimmed string) int {
	n := listMarkerLen(trimmed)
	// 标记形如 "1. "，n 含尾部空格，'.' 在 n-2 处。
	if n < 3 || trimmed[n-2] != '.' {
		return 0
	}
	v := 0
	for _, c := range trimmed[:n-2] {
		v = v*10 + int(c-'0')
	}
	return v
}

// runeToByte 把 rune 下标换成字节下标。
func runeToByte(s string, runeIdx int) int {
	i := 0
	for pos := range s {
		if i == runeIdx {
			return pos
		}
		i++
	}
	return len(s)
}

// toggleTaskLine 翻转第 lineNo 行待办的勾选状态。
func toggleTaskLine(src string, lineNo int) string {
	lines := strings.Split(src, "\n")
	if lineNo < 0 || lineNo >= len(lines) {
		return src
	}
	if strings.Contains(lines[lineNo], "[ ]") {
		lines[lineNo] = strings.Replace(lines[lineNo], "[ ]", "[x]", 1)
	} else {
		lines[lineNo] = strings.Replace(lines[lineNo], "[x]", "[ ]", 1)
	}
	return strings.Join(lines, "\n")
}
