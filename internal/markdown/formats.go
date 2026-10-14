package markdown

// Markdown 格式变换：源文档 + rune 选区 → 新文档 + 新选区的纯函数。
// 菜单、Keymap 与编辑器经命令注册表调用；实时预览经既有投影/切块机制接入。
//
// 语义（与计划一致）：
//   - 行内格式：选区完整具有该格式则移除，否则应用；行内代码与其他
//     行内样式互斥，不生成错误嵌套；保留已有链接。
//   - 标题/正文：修改覆盖的正文行；同级标题再次执行还原正文。
//   - 列表/引用：处理所选行并保留文本，移除自身标记时不丢内容。
//   - 代码块：包裹所选内容（行），无选区插入空代码块；
//     代码块内再执行则还原为正文。
//   - 分隔线/表格：在当前行所在块之后插入。

import (
	"strings"
	"unicode/utf8"
)

// lineByteBounds 返回文档第 line 行的字节范围 [start, end)（不含换行符）。
func lineByteBounds(doc string, line int) (int, int) {
	start := lineByteOffset(doc, line)
	rest := doc[start:]
	end := start + len(rest)
	if i := strings.IndexByte(rest, '\n'); i >= 0 {
		end = start + i
	}
	return start, end
}

// docLineCount 文档行数（空文档算 1 行）。
func docLineCount(doc string) int {
	if doc == "" {
		return 1
	}
	return strings.Count(doc, "\n") + 1
}

// byteToRune 字节偏移 → rune 偏移。
func byteToRune(s string, byteOff int) int { return utf8.RuneCountInString(s[:byteOff]) }

// docResult 是一次变换的输出：新文档与 rune 选区。
type DocResult struct {
	Doc      string
	SelStart int
	SelEnd   int
	Changed  bool
}

func noChange(doc string, selStart, selEnd int) DocResult {
	return DocResult{Doc: doc, SelStart: selStart, SelEnd: selEnd}
}

// ---- 行内格式 ----

func inlineMarkers(kind spanKind) (open, close string) {
	switch kind {
	case spanBold:
		return "**", "**"
	case spanItalic:
		return "*", "*"
	case spanCode:
		return "`", "`"
	}
	return "", ""
}

// spanPrefixLen 行内标记的开标记长度（用于内容定位；链接取 "[" 后）。
func spanPrefixLen(kind spanKind) int {
	if kind == spanBold {
		return 2
	}
	return 1
}

// toggleInlineSpan 在单行文本上切换 [from,to) 的行内格式。
// 返回新文本与映射后的选区（行内 rune 偏移），changed=false 表示无法应用
// （例如会生成歧义标记 run）。
func toggleInlineSpan(text string, from, to int, kind spanKind) (string, int, int, bool) {
	open, closer := inlineMarkers(kind)
	if open == "" {
		return text, from, to, false
	}
	rs := []rune(text)
	if from < 0 || to > len(rs) || from > to {
		return text, from, to, false
	}
	spans := inlineSpans(text)

	// 1) 选区正好落在（或包含于）同格式 span 内：移除该格式（unwrap）。
	for _, sp := range spans {
		if sp.Kind != kind {
			continue
		}
		exact := from == sp.Start && to == sp.End
		inside := sp.Start <= from && to <= sp.End && to > from
		if !exact && !inside {
			continue
		}
		prefix := spanPrefixLen(kind)
		contentStart, contentEnd := sp.Start+prefix, sp.End-prefix
		if contentStart > contentEnd {
			return text, from, to, false
		}
		newText := string(rs[:sp.Start]) + string(rs[contentStart:contentEnd]) + string(rs[sp.End:])
		newFrom := clamp(from, contentStart, contentEnd) - sp.Start
		newTo := clamp(to, contentStart, contentEnd) - sp.Start
		return newText, newFrom, newTo, true
	}

	// 2) 应用目标格式前清除冲突 span：代码清除相交的粗/斜体；
	//    粗/斜体清除相交的代码。链接保留（不与标记互斥）。
	//    只处理「完全含于选区」或「完整包含选区」的冲突 span；
	//    部分相交会导致无法配对，直接放弃（不破坏文档）。
	var removals []mSpan
	for _, sp := range spans {
		overlaps := sp.Start < to && from < sp.End
		if !overlaps {
			continue
		}
		conflict := (kind == spanCode && sp.Kind != spanCode && sp.Kind != spanLink) ||
			(kind != spanCode && sp.Kind == spanCode)
		if !conflict {
			continue
		}
		fullyInside := sp.Start >= from && sp.End <= to
		contains := sp.Start <= from && to <= sp.End
		if !fullyInside && !contains {
			return text, from, to, false
		}
		removals = append(removals, sp)
	}

	// 3) 相同标记相邻会生成歧义 run（如 **** ）：跳过。
	markerRune := rune(open[0])
	if from > 0 && rs[from-1] == markerRune || to < len(rs) && rs[to] == markerRune {
		return text, from, to, false
	}
	for _, sp := range spans {
		if sp.Kind == kind && sp.Start <= from && from <= sp.End {
			return text, from, to, false // 插入点在同类 span 边界上
		}
	}

	// 4) 从右向左移除冲突 span，再包上标记；选区经 mapOffset 映射。
	newText := text
	for i := len(removals) - 1; i >= 0; i-- {
		sp := removals[i]
		sr := []rune(newText)
		newText = string(sr[:sp.Start]) + spanText(text, sp) + string(sr[sp.End:])
	}
	mappedFrom, mappedTo := mapOffset(from, removals, text), mapOffset(to, removals, text)
	nr := []rune(newText)
	if mappedFrom > mappedTo {
		mappedFrom, mappedTo = mappedTo, mappedFrom
	}
	out := string(nr[:mappedFrom]) + open + string(nr[mappedFrom:mappedTo]) + closer + string(nr[mappedTo:])
	n := utf8.RuneCountInString(open)
	return out, mappedFrom + n, mappedTo + n, true
}

// mapOffset 把旧偏移映射到移除 removals 之后的文本。
func mapOffset(off int, removals []mSpan, oldText string) int {
	sortSpans(removals)
	result := off
	for _, sp := range removals {
		inner := utf8.RuneCountInString(spanText(oldText, sp))
		markerLen := sp.End - sp.Start - inner
		if off <= sp.Start {
			break
		}
		if off >= sp.End {
			result -= markerLen
			continue
		}
		// 偏移在 span 内：收敛到内容区间。
		contentStart := sp.Start + spanPrefixLen(sp.Kind)
		innerOff := clamp(off-contentStart, 0, inner)
		return sp.Start + innerOff
	}
	return result
}

func sortSpans(spans []mSpan) {
	for i := 1; i < len(spans); i++ {
		for j := i; j > 0 && spans[j].Start < spans[j-1].Start; j-- {
			spans[j], spans[j-1] = spans[j-1], spans[j]
		}
	}
}

// applyInlineToLines 对选区覆盖的每一行应用行内格式切换（源码选区可跨行）。
// 返回新文档与新选区（文档 rune 偏移）。
func ApplyInlineToLines(doc string, from, to, selStart, selEnd int, kind spanKind) DocResult {
	if selStart >= selEnd {
		return noChange(doc, selStart, selEnd)
	}
	total := docLineCount(doc)
	if from < 0 {
		from = 0
	}
	if to > total-1 {
		to = total - 1
	}
	headEnd, _ := lineByteBounds(doc, from)
	_, tailStart := lineByteBounds(doc, to) // 行尾（含其后的换行符）归 tail
	head, tail := doc[:headEnd], ""
	if tailStart <= len(doc) {
		tail = doc[tailStart:]
	}
	var sb strings.Builder
	mappedStart, mappedEnd := -1, -1
	outRunes := 0 // 已拼接输出的 rune 数（含换行）
	for line := from; line <= to; line++ {
		ls, le := lineByteBounds(doc, line)
		lineText := doc[ls:le]
		lineRunes := utf8.RuneCountInString(lineText)
		lineStart := byteToRune(doc, ls)
		lo := max(selStart-lineStart, 0)
		hi := min(selEnd-lineStart, lineRunes)
		if lo > lineRunes {
			lo = lineRunes
		}
		newText, nf, nt := lineText, lo, hi
		if lo < hi {
			if out, a, b, changed := toggleInlineSpan(lineText, lo, hi, kind); changed {
				newText, nf, nt = out, a, b
			}
		}
		sb.WriteString(newText)
		if line < to {
			sb.WriteByte('\n')
		}
		if mappedStart < 0 && (nf != lo || nt != hi || newText != lineText) && (lo < hi) {
			mappedStart = outRunes + nf
		}
		if lo < hi {
			mappedEnd = outRunes + nt
		}
		outRunes += utf8.RuneCountInString(newText) + 1
	}
	if mappedStart < 0 || mappedEnd < mappedStart {
		return noChange(doc, selStart, selEnd)
	}
	return DocResult{Doc: head + sb.String() + tail, SelStart: mappedStart, SelEnd: mappedEnd, Changed: true}
}

// ---- 标题与正文 ----

// headingLevelOf 返回一行的标题级别（0=非标题）。
func headingLevelOf(line string) int {
	t := strings.TrimSpace(line)
	if !isHeading(t) {
		return 0
	}
	n := 0
	for n < len(t) && t[n] == '#' {
		n++
	}
	return n
}

// stripHeadingPrefix 去掉一行开头的标题标记，返回 (正文, 是否标题行)。
func stripHeadingPrefix(line string) (string, bool) {
	level := headingLevelOf(line)
	if level == 0 {
		return line, false
	}
	t := strings.TrimLeft(line, " \t")
	body := strings.TrimSpace(t[level:])
	return body, true
}

// applyHeadingToLines 把 [from,to] 行设为 level 级标题（0=正文）。
// 全部行已是该级别时还原为正文（toggle）。空行保持空行。
func ApplyHeadingToLines(doc string, from, to, caret int, level int) DocResult {
	total := docLineCount(doc)
	from, to = clamp(from, 0, total-1), clamp(to, 0, total-1)
	if to < from {
		from, to = to, from
	}
	// toggle：范围内所有非空行都已是目标级别 → 还原正文。
	allMatch, anyContent := true, false
	for line := from; line <= to; line++ {
		ls, le := lineByteBounds(doc, line)
		text := doc[ls:le]
		if strings.TrimSpace(text) == "" {
			continue
		}
		anyContent = true
		if headingLevelOf(text) != level {
			allMatch = false
		}
	}
	if !anyContent {
		return noChange(doc, caret, caret)
	}
	if level > 0 && allMatch {
		level = 0
	}

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
	prefix := ""
	if level > 0 {
		prefix = strings.Repeat("#", level) + " "
	}
	for line := from; line <= to; line++ {
		ls, le := lineByteBounds(doc, line)
		text := doc[ls:le]
		body, wasHeading := stripHeadingPrefix(text)
		if !wasHeading && strings.TrimSpace(text) != "" {
			body = strings.TrimSpace(text)
		}
		var newText string
		if strings.TrimSpace(text) == "" {
			newText = text // 空行不动
		} else {
			newText = prefix + body
		}
		sb.WriteString(newText)
		if line < to {
			sb.WriteByte('\n')
		}
		if line == caretLine {
			delta := utf8.RuneCountInString(newText) - utf8.RuneCountInString(text)
			col := caretCol + delta
			// 光标不得落进新标记内部：收敛到标记之后。
			if floor := utf8.RuneCountInString(prefix); col < floor {
				col = floor
			}
			newCaret = outRunes + min(col, utf8.RuneCountInString(newText))
		}
		outRunes += utf8.RuneCountInString(newText) + 1
	}
	newDoc := head + sb.String() + tail
	return DocResult{Doc: newDoc, SelStart: newCaret, SelEnd: newCaret, Changed: newDoc != doc}
}

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
	allMatch, anyContent := true, false
	for line := from; line <= to; line++ {
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
		if strings.TrimSpace(text) != "" {
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
	return DocResult{Doc: newDoc, SelStart: newCaret, SelEnd: newCaret, Changed: newDoc != doc}
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
	if len(lines) >= 2 && isFenceLine(strings.TrimSpace(lines[0])) &&
		isFenceLine(strings.TrimSpace(lines[len(lines)-1])) {
		// 还原为正文：去掉两行围栏，光标跟随上移一行。
		// 闭栏行自身后面的换行属于围栏结构，一并去掉。
		inner := strings.Join(lines[1:len(lines)-1], "\n")
		out := doc[:ls] + inner
		if rest := doc[le:]; strings.Trim(rest, "\n") != "" {
			out += rest // 闭栏之后还有正文：原样保留
		}
		newCaret := caret - (base + utf8.RuneCountInString(lines[0]) + 1)
		return DocResult{Doc: out, SelStart: max(newCaret, 0), SelEnd: max(newCaret, 0), Changed: true}
	}
	// 包裹：插入前后围栏，光标落到内容首。
	const fence = "```\n"
	wrapped := fence + region + "\n```\n"
	var out string
	if le >= len(doc) {
		out = doc[:ls] + wrapped
	} else {
		out = doc[:ls] + wrapped + doc[le:]
	}
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
