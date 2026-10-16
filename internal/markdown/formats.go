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
		newFrom := sp.Start + clamp(from-contentStart, 0, contentEnd-contentStart)
		newTo := sp.Start + clamp(to-contentStart, 0, contentEnd-contentStart)
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
	protected := inlineProtectedBlockLines(doc, selStart, selEnd)
	var sb strings.Builder
	mappedStart, mappedEnd := -1, -1
	changedAny := false
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
		if lo < hi && !protected[line] {
			if out, a, b, changed := toggleInlineSpan(lineText, lo, hi, kind); changed {
				newText, nf, nt = out, a, b
				changedAny = true
			}
		}
		sb.WriteString(newText)
		if line < to {
			sb.WriteByte('\n')
		}
		if mappedStart < 0 && lo < hi {
			mappedStart = outRunes + nf
		}
		if lo < hi {
			mappedEnd = outRunes + nt
		}
		outRunes += utf8.RuneCountInString(newText) + 1
	}
	if !changedAny || mappedStart < 0 || mappedEnd < mappedStart {
		return noChange(doc, selStart, selEnd)
	}
	regionStart := utf8.RuneCountInString(head)
	return DocResult{Doc: head + sb.String() + tail, SelStart: regionStart + mappedStart, SelEnd: regionStart + mappedEnd, Changed: true}
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
	protected := protectedBlockLines(doc)
	// toggle：范围内所有可格式化的非空行都已是目标级别 → 还原正文。
	allMatch, anyContent := true, false
	for line := from; line <= to; line++ {
		if protected[line] {
			continue
		}
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
		newText := text
		prefixForLine := ""
		if !protected[line] {
			body, wasHeading := stripHeadingPrefix(text)
			if !wasHeading && strings.TrimSpace(text) != "" {
				body = strings.TrimSpace(text)
			}
			if strings.TrimSpace(text) != "" {
				newText = prefix + body
				prefixForLine = prefix
			}
		}
		sb.WriteString(newText)
		if line < to {
			sb.WriteByte('\n')
		}
		if line == caretLine {
			delta := utf8.RuneCountInString(newText) - utf8.RuneCountInString(text)
			col := caretCol + delta
			// 光标不得落进新标记内部：收敛到标记之后。
			if floor := utf8.RuneCountInString(prefixForLine); col < floor {
				col = floor
			}
			newCaret = outRunes + min(col, utf8.RuneCountInString(newText))
		}
		outRunes += utf8.RuneCountInString(newText) + 1
	}
	newDoc := head + sb.String() + tail
	regionStart := utf8.RuneCountInString(head)
	return DocResult{Doc: newDoc, SelStart: regionStart + newCaret, SelEnd: regionStart + newCaret, Changed: newDoc != doc}
}
