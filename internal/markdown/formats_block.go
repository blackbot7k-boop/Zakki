package markdown

// 块级格式变换：列表、引用、代码块，以及分隔线/表格插入。

import (
	"strings"
	"unicode/utf8"
)

// ---- 列表 / 引用 ----

type PrefixKind int

const (
	PrefixNone PrefixKind = iota
	PrefixQuote
	PrefixBullet
	PrefixOrdered
	PrefixTask
)

// lineMarkerKind 判断一行当前的块标记类别（-1 = 无）。
func lineMarkerKind(text string) PrefixKind {
	t := strings.TrimSpace(text)
	switch {
	case strings.HasPrefix(t, ">"):
		return PrefixQuote
	case taskPrefixLen(t) > 0:
		return PrefixTask
	case listMarkerLen(t) > 0:
		marker := t[:listMarkerLen(t)]
		if strings.HasSuffix(marker, ". ") {
			return PrefixOrdered
		}
		return PrefixBullet
	}
	return PrefixNone
}

// stripBlockMarker 去掉一行的块标记（标题/引用/列表/待办），返回正文。
func stripBlockMarker(text string) string {
	t := strings.TrimLeft(text, " \t")
	if body, ok := stripHeadingPrefix(text); ok {
		return body
	}
	switch {
	case strings.HasPrefix(t, ">"):
		body := strings.TrimSpace(strings.TrimPrefix(t, ">"))
		return stripBlockMarker(body) // 嵌套引用逐层剥
	case taskPrefixLen(t) > 0:
		return strings.TrimSpace(t[taskPrefixLen(t):])
	case listMarkerLen(t) > 0:
		return strings.TrimSpace(t[listMarkerLen(t):])
	}
	return strings.TrimSpace(text)
}

func markerPrefix(kind PrefixKind, lineIndex int) string {
	switch kind {
	case PrefixQuote:
		return "> "
	case PrefixBullet:
		return "- "
	case PrefixOrdered:
		return itoa(lineIndex+1) + ". "
	case PrefixTask:
		return "- [ ] "
	}
	return ""
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

// applyPrefixToLines 对 [from,to] 行切换块标记。全部行已有该标记时移除；
// 否则先把每行的既有块标记剥掉，再加目标标记（互斥，保留文本）。
func ApplyPrefixToLines(doc string, from, to, caret int, kind PrefixKind) DocResult {
	total := docLineCount(doc)
	from, to = clamp(from, 0, total-1), clamp(to, 0, total-1)
	if to < from {
		from, to = to, from
	}
	protected := protectedBlockLines(doc)
	allMatch, anyContent := true, false
	for line := from; line <= to; line++ {
		if protected[line] {
			continue
		}
		ls, le := lineByteBounds(doc, line)
		text := doc[ls:le]
		if strings.TrimSpace(stripBlockMarker(text)) == "" && strings.TrimSpace(text) == "" {
			continue // 空行跳过
		}
		anyContent = true
		if lineMarkerKind(text) != kind {
			allMatch = false
		}
	}
	if !anyContent {
		return noChange(doc, caret, caret)
	}
	removing := allMatch

	headEnd, _ := lineByteBounds(doc, from)
	_, tailStart := lineByteBounds(doc, to)
	head, tail := doc[:headEnd], ""
	if tailStart <= len(doc) {
		tail = doc[tailStart:]
	}
	var sb strings.Builder
	caretLine, caretCol := caretLineCol(doc, caret)
	outRunes := 0
	newCaret := caret
	seq := 0
	for line := from; line <= to; line++ {
		ls, le := lineByteBounds(doc, line)
		text := doc[ls:le]
		newText := text
		if !protected[line] && strings.TrimSpace(text) != "" {
			body := stripBlockMarker(text)
			if removing {
				newText = body
			} else {
				pfx := markerPrefix(kind, seq)
				if kind != PrefixQuote {
					seq++
				}
				// 引用内保留行内标记文本；列表/待办剥掉旧标记后加新前缀。
				newText = pfx + body
			}
		}
		sb.WriteString(newText)
		if line < to {
			sb.WriteByte('\n')
		}
		if line == caretLine {
			delta := utf8.RuneCountInString(newText) - utf8.RuneCountInString(text)
			col := caretCol + delta
			if col < 0 {
				col = 0
			}
			newCaret = outRunes + min(col, utf8.RuneCountInString(newText))
		}
		outRunes += utf8.RuneCountInString(newText) + 1
	}
	newDoc := head + sb.String() + tail
	regionStart := utf8.RuneCountInString(head)
	return DocResult{Doc: newDoc, SelStart: regionStart + newCaret, SelEnd: regionStart + newCaret, Changed: newDoc != doc}
}

// ---- 代码块 ----

// applyCodeBlockToggle 若 [from,to] 是围栏代码块则还原为正文；
// 否则用围栏包裹这些行，光标落在开栏之后的内容首。
func ApplyCodeBlockToggle(doc string, from, to, caret int) DocResult {
	total := docLineCount(doc)
	from, to = clamp(from, 0, total-1), clamp(to, 0, total-1)
	if to < from {
		from, to = to, from
	}
	ls, _ := lineByteBounds(doc, from)
	_, le := lineByteBounds(doc, to)
	region := doc[ls:le]
	lines := strings.Split(region, "\n")
	base := byteToRune(doc, ls)
	opening, isFenced := parseFenceOpeningLine(lines[0])
	if len(lines) >= 2 && isFenced && isFenceClosingLine(lines[len(lines)-1], opening) {
		// 还原为正文：只移除围栏行，围栏之外的内容逐字保留。
		inner := strings.Join(lines[1:len(lines)-1], "\n")
		out := doc[:ls] + inner + doc[le:]
		openingLen := utf8.RuneCountInString(lines[0]) + 1
		innerLen := utf8.RuneCountInString(inner)
		newCaret := clamp(caret-openingLen, base, base+innerLen)
		return DocResult{Doc: out, SelStart: newCaret, SelEnd: newCaret, Changed: true}
	}
	// 包裹：只在所选行两侧插入围栏，原有后缀（含换行）逐字保留。
	const fence = "```\n"
	wrapped := fence + region + "\n```"
	out := doc[:ls] + wrapped + doc[le:]
	newCaret := base + len(fence)
	return DocResult{Doc: out, SelStart: newCaret, SelEnd: newCaret, Changed: true}
}

// ---- 插入 ----

// applyInsertAfterLine 在 afterLine 行之后插入一个块（前置空行分隔），
// 光标落到插入内容内 caretInBlock rune 处（相对插入块首）。
func ApplyInsertAfterLine(doc string, afterLine int, block string, caretInBlock int) DocResult {
	total := docLineCount(doc)
	afterLine = clamp(afterLine, -1, total-1)
	var head string
	if afterLine < 0 {
		head = ""
	} else {
		_, le := lineByteBounds(doc, afterLine)
		head = doc[:le]
	}
	tail := ""
	if len(head) < len(doc) {
		tail = doc[len(head):] // 以 \n 开头（若后面还有内容）
	}
	inserted := "\n\n" + block
	out := head + inserted + tail
	caret := byteToRune(out, len(head)) + utf8.RuneCountInString(inserted) - utf8.RuneCountInString(block) + clamp(caretInBlock, 0, utf8.RuneCountInString(block))
	return DocResult{Doc: out, SelStart: caret, SelEnd: caret, Changed: true}
}

// replaceLineWith 用 block 替换第 line 行，光标落在块内 caretInBlock rune 处。
func ReplaceLineWith(doc string, line int, block string, caretInBlock int) DocResult {
	total := docLineCount(doc)
	line = clamp(line, 0, total-1)
	ls, le := lineByteBounds(doc, line)
	out := doc[:ls] + block
	if le < len(doc) {
		out += doc[le:]
	}
	caret := byteToRune(out, ls) + clamp(caretInBlock, 0, utf8.RuneCountInString(block))
	return DocResult{Doc: out, SelStart: caret, SelEnd: caret, Changed: true}
}

// defaultTableBlock 新表格：两列、一个表头行、两行正文，由已有表格工具调整。
func DefaultTableBlock() string {
	return "| 表头 1 | 表头 2 |\n| --- | --- |\n|  |  |\n|  |  |"
}

// caretLineCol 把文档 rune 偏移拆成 (行, 行内 rune 列)。
func caretLineCol(doc string, caret int) (int, int) {
	caret = clamp(caret, 0, utf8.RuneCountInString(doc))
	line, col := 0, 0
	i := 0
	for _, r := range doc {
		if i >= caret {
			break
		}
		if r == '\n' {
			line++
			col = 0
		} else {
			col++
		}
		i++
	}
	return line, col
}

// caretFromLineCol 把 (行, 列) 合回文档 rune 偏移（越界收敛）。
func caretFromLineCol(doc string, line, col int) int {
	total := docLineCount(doc)
	line = clamp(line, 0, total-1)
	ls, le := lineByteBounds(doc, line)
	runes := utf8.RuneCountInString(doc[ls:le])
	return byteToRune(doc, ls) + clamp(col, 0, runes)
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// LineByteOffset 返回文档第 line 行（0-based）的字节起始偏移。
func LineByteOffset(doc string, line int) int {
	return lineByteOffset(doc, line)
}

func lineByteOffset(doc string, line int) int {
	if line <= 0 {
		return 0
	}
	off := 0
	for l := 0; l < line; l++ {
		i := strings.IndexByte(doc[off:], '\n')
		if i < 0 {
			return len(doc)
		}
		off += i + 1
	}
	return off
}
