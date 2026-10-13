package main

// 轻量 markdown 引擎：
//   - inlineSpans   解析行内标记（粗体/斜体/代码/链接）
//   - editorRanges  生成编辑器的语法高亮区间
//   - parseBlocks   把文本切成块级元素供预览渲染
//
// 只覆盖便签常用的子集：标题、列表、待办、引用、代码块、分隔线、行内标记。

import (
	"strings"
	"unicode/utf8"

	"github.com/egoist/mygo/ui"
)

// ---- 行内标记 ----

type spanKind int

const (
	spanBold spanKind = iota
	spanItalic
	spanCode
	spanLink
)

type mSpan struct {
	start, end int // rune 区间，含标记符本身
	kind       spanKind
	href       string
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
				out = append(out, mSpan{start: i, end: j + 1, kind: spanCode})
				i = j + 1
				continue
			}
		case rs[i] == '*' && i+1 < len(rs) && rs[i+1] == '*':
			if j := runeSeq(rs, '*', 2, i+2); j > 0 {
				out = append(out, mSpan{start: i, end: j + 2, kind: spanBold})
				i = j + 2
				continue
			}
		case rs[i] == '*' || rs[i] == '_':
			if j := runeIndexOf(rs, rs[i], i+1); j > 0 && j > i+1 {
				out = append(out, mSpan{start: i, end: j + 1, kind: spanItalic})
				i = j + 1
				continue
			}
		case rs[i] == '[':
			if cb := runeIndexOf(rs, ']', i+1); cb > 0 && cb+1 < len(rs) && rs[cb+1] == '(' {
				if pe := runeIndexOf(rs, ')', cb+2); pe > 0 {
					out = append(out, mSpan{start: i, end: pe + 1, kind: spanLink, href: string(rs[cb+2 : pe])})
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
	inner := rs[sp.start:sp.end]
	switch sp.kind {
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

// ---- 编辑器高亮 ----

// editorRanges 为编辑器生成互不重叠的着色区间：搜索命中优先，
// 其次是行级结构（标题/引用/代码/列表/待办），再次是行内标记。
// accent 用当前点的主题色（与 Tot 一致，每个点有自己的强调色）。
func editorRanges(text, query string, t *ui.Theme, accent ui.Color) []ui.TextRange {
	var accepted []ui.TextRange

	add := func(tr ui.TextRange) bool {
		if tr.Start >= tr.End || tr.Start < 0 || tr.End > utf8.RuneCountInString(text) {
			return false
		}
		for _, a := range accepted {
			if tr.Start < a.End && a.Start < tr.End {
				return false // 与已接受的区间重叠
			}
		}
		accepted = append(accepted, tr)
		return true
	}

	// 搜索命中（最高优先级）
	for _, m := range findMatches(text, query) {
		add(ui.TextRange{Start: m.Start, End: m.End, Color: accent, Weight: 700})
	}

	// 行级与行内样式
	runeOff := 0
	inCode := false
	for _, line := range strings.Split(text, "\n") {
		lineRunes := utf8.RuneCountInString(line)
		trimmed := strings.TrimSpace(line)
		styledLine := false
		switch {
		case strings.HasPrefix(trimmed, "```"):
			inCode = !inCode
			add(ui.TextRange{Start: runeOff, End: runeOff + lineRunes, Color: t.TextMuted})
			styledLine = true
		case inCode:
			add(ui.TextRange{Start: runeOff, End: runeOff + lineRunes, Color: t.TextMuted})
			styledLine = true
		case isHeading(trimmed):
			add(ui.TextRange{Start: runeOff, End: runeOff + lineRunes, Color: accent, Weight: 700})
			styledLine = true
		case strings.HasPrefix(trimmed, ">"):
			add(ui.TextRange{Start: runeOff, End: runeOff + lineRunes, Color: t.TextMuted})
			styledLine = true
		}

		if !styledLine {
			// 待办前缀 [ ] / [x]
			if bi := strings.Index(line, "[ ]"); bi >= 0 {
				mark := runeOff + utf8.RuneCountInString(line[:bi])
				add(ui.TextRange{Start: mark, End: mark + 3, Color: accent, Weight: 600})
			} else if bi := strings.Index(line, "[x]"); bi >= 0 {
				mark := runeOff + utf8.RuneCountInString(line[:bi])
				add(ui.TextRange{Start: mark, End: mark + 3, Color: accent, Weight: 600})
			}
			// 列表标记 - * 1.
			if n := listMarkerLen(trimmed); n > 0 {
				lead := len(line) - len(strings.TrimLeft(line, " \t"))
				off := utf8.RuneCountInString(line[:lead])
				add(ui.TextRange{Start: runeOff + off, End: runeOff + off + n, Color: t.TextMuted})
			}
			// 行内标记
			for _, sp := range inlineSpans(line) {
				tr := ui.TextRange{Start: runeOff + sp.start, End: runeOff + sp.end}
				switch sp.kind {
				case spanBold:
					tr.Weight = 600
				case spanCode:
					tr.Color = t.TextMuted
				case spanLink:
					tr.Color = accent
				}
				if tr.Color != (ui.Color{}) || tr.Weight != 0 {
					add(tr)
				}
			}
		}

		runeOff += lineRunes + 1
	}

	// 区间必须不重叠且按顺序出现。
	sortRanges(accepted)
	return accepted
}

func sortRanges(rs []ui.TextRange) {
	for i := 1; i < len(rs); i++ {
		for j := i; j > 0 && rs[j].Start < rs[j-1].Start; j-- {
			rs[j], rs[j-1] = rs[j-1], rs[j]
		}
	}
}

func isHeading(trimmed string) bool {
	if trimmed == "" || trimmed[0] != '#' {
		return false
	}
	n := 0
	for n < len(trimmed) && trimmed[n] == '#' {
		n++
	}
	return n <= 6 && n < len(trimmed) && trimmed[n] == ' '
}

// taskPrefixLen 返回 "- [ ] " 类待办前缀的字节长度，0 表示不是待办。
func taskPrefixLen(trimmed string) int {
	for _, mark := range []string{"- [ ] ", "* [ ] ", "- [x] ", "* [x] "} {
		if strings.HasPrefix(trimmed, mark) {
			return len(mark)
		}
	}
	return 0
}

// listMarkerLen 返回 "- "、"・ "、"1. " 类列表标记的 rune 长度。
func listMarkerLen(trimmed string) int {
	if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") {
		return 2
	}
	for i := 0; i < len(trimmed); i++ {
		if trimmed[i] == '.' && i+1 < len(trimmed) && trimmed[i+1] == ' ' && i > 0 {
			for _, c := range trimmed[:i] {
				if c < '0' || c > '9' {
					return 0
				}
			}
			return i + 2
		}
		if trimmed[i] < '0' || trimmed[i] > '9' {
			return 0
		}
	}
	return 0
}

// ---- 块级解析（预览用）----

type blockKind int

const (
	bkPara blockKind = iota
	bkHeading
	bkQuote
	bkCode
	bkRule
	bkList
	bkTask
)

type block struct {
	kind    blockKind
	level   int    // 标题级别 1-4
	text    string // 段落/标题/引用/列表项/待办的文字
	lines   []string
	checked bool   // 待办是否勾选
	lineNo  int    // 待办在源文本中的行号
	n       int    // 有序列表序号
}

// parseBlocks 把 markdown 文本切成块。
func parseBlocks(src string) []block {
	lines := strings.Split(src, "\n")
	var blocks []block
	var para []string
	paraStart := 0
	codeOpen := false

	flushPara := func() {
		if len(para) > 0 {
			blocks = append(blocks, block{kind: bkPara, text: strings.Join(para, " "), lineNo: paraStart})
			para = nil
		}
	}

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, "```"):
			flushPara()
			if codeOpen {
				codeOpen = false
			} else {
				codeOpen = true
				blocks = append(blocks, block{kind: bkCode})
			}
		case codeOpen:
			blocks[len(blocks)-1].lines = append(blocks[len(blocks)-1].lines, line)
		case trimmed == "":
			flushPara()
		case trimmed == "---" || trimmed == "***":
			flushPara()
			blocks = append(blocks, block{kind: bkRule, lineNo: i})
		case isHeading(trimmed):
			flushPara()
			level := 1
			for level < len(trimmed) && trimmed[level] == '#' {
				level++
			}
			if level > 4 {
				level = 4
			}
			blocks = append(blocks, block{kind: bkHeading, level: level, text: strings.TrimSpace(trimmed[level:]), lineNo: i})
		case strings.HasPrefix(trimmed, ">"):
			flushPara()
			blocks = append(blocks, block{kind: bkQuote, text: strings.TrimSpace(strings.TrimPrefix(trimmed, ">")), lineNo: i})
		case taskPrefixLen(trimmed) > 0:
			flushPara()
			mark := trimmed[:taskPrefixLen(trimmed)]
			blocks = append(blocks, block{
				kind:    bkTask,
				checked: strings.Contains(mark, "[x]"),
				text:    trimmed[len(mark):],
				lineNo:  i,
			})
		case listMarkerLen(trimmed) > 0:
			flushPara()
			n := listMarkerLen(trimmed)
			blocks = append(blocks, block{kind: bkList, text: trimmed[n:], lineNo: i,
				n: orderedNumber(trimmed)})
		default:
			if len(para) == 0 {
				paraStart = i
			}
			para = append(para, trimmed)
		}
	}
	if codeOpen {
		// 未闭合的代码块：保留已收集内容
	}
	flushPara()
	return blocks
}

func orderedNumber(trimmed string) int {
	n := listMarkerLen(trimmed)
	if n == 0 || trimmed[n-1] != '.' {
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

// toggleBulletPrefix 在第 caretRune 个 rune 所在行行首插入或移除 "- "，
// 返回新文本和插入后光标应处的 rune 位置。
func toggleBulletPrefix(s string, caretRune int) (string, int) {
	if caretRune > utf8.RuneCountInString(s) {
		caretRune = utf8.RuneCountInString(s)
	}
	b := runeToByte(s, caretRune)
	lineStart := 0
	if j := strings.LastIndexByte(s[:b], '\n'); j >= 0 {
		lineStart = j + 1
	}
	rest := s[lineStart:]
	if strings.HasPrefix(rest, "- ") {
		return s[:lineStart] + rest[2:], caretRune - 2
	}
	return s[:lineStart] + "- " + rest, caretRune + 2
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
