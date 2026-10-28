package app

// 实时预览：文档按行切成 liveBlock。编辑状态机在 live_edit.go。

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"zakki/internal/markdown"
)

// liveBlock 是从文档切出的一个可编辑块。
type liveBlock struct {
	kind    blockKind
	start   int      // 文档起始行（含）
	end     int      // 文档结束行（含）
	raw     string   // 原始 markdown（行间以 \n 相连）
	level   int      // 标题级别 1-4
	n       int      // 有序列表序号（0 = 无序）
	checked bool     // 待办勾选
	text    string   // 渲染用文本（去块级标记；段落 = 各行 join " "）
	lines   []string // 代码块内容行（不含围栏）
	info    string   // 代码块开栏 info 串（语言提示）
	spans   []mSpan  // text 的行内标记（渲染用，切分时预计算一次）
	table   markdownTable

	hasTime  bool          // 正文带时间组件（text 已剥离令牌，见 projectBlock）
	due      time.Time     // 令牌时刻（本地时区墙上时间）
	hasAlert bool          // 时间组件带 ~提前量 令牌
	alert    time.Duration // 提醒提前量；0 表示准点
	recur    string        // 原样保留重复规则；桌面暂不消费
}

// liveBlocks 把文档按行切成块。规则（便签子集）：
//   - ``` 围栏：从开栏行到闭栏行（未闭合则到文档尾）为一个块；
//   - 空白行：独立块（可编辑、可点击）；
//   - 标题/引用/分隔线/列表项/待办：单行块；
//   - 其余连续行：合并为一个段落块（软换行段）。
func liveBlocks(src string) []liveBlock {
	lines := strings.Split(src, "\n")
	return parseLines(lines, 0, len(lines))
}

// parseLines 把 lines[from:stop] 切成块（stop 为排他行界）。
// 块不越过 stop：围栏未闭合在 stop-1 截断，段落不吸收 stop 之后的行。
// mdLineSpecial 报告一行是否天然开启新块（段落不能吸收它）。
func mdLineSpecial(line string) bool {
	t := strings.TrimSpace(line)
	s := strings.TrimLeft(line, " \t")
	return t == "" || isFenceOpeningLine(line) || t == "---" || t == "***" || isHeading(t) ||
		strings.HasPrefix(t, ">") || taskPrefixLen(s) > 0 || listMarkerLen(s) > 0
}

// stripTimeSuffix 保留组件前后的全部正文，供通知和纯文本摘要使用。
func stripTimeSuffix(blk *liveBlock, text string) string {
	token, ok := markdown.FindTimeToken(text)
	if !ok {
		return text
	}
	blk.hasTime, blk.due = true, token.Value.Time
	blk.alert, blk.hasAlert, blk.recur = token.Value.Alert, token.Value.HasAlert, token.Value.Recur
	return strings.TrimSpace(markdown.ReplaceTimeToken(text, nil))
}

func parseLines(lines []string, from, stop int) []liveBlock {
	out := make([]liveBlock, 0, stop-from)

	for i := from; i < stop; {
		line := lines[i]
		t := strings.TrimSpace(line)
		// 列表/待办标记需要尾部空格（如 "- "），用仅去左缩进的原文检查。
		s := strings.TrimLeft(line, " \t")
		opening, isFence := parseFenceOpeningLine(line)
		table, isTable := tableStart(lines, i, stop)
		switch {
		case isTable:
			j := i + 2
			for j < stop {
				cells, ok := tableRowCells(lines[j], len(table.align))
				if !ok {
					break
				}
				table.rows = append(table.rows, cells)
				j++
			}
			out = append(out, liveBlock{kind: bkTable, start: i, end: j - 1,
				raw: strings.Join(lines[i:j], "\n"), table: table})
			i = j
		case isFence:
			j := i + 1
			for j < stop && !isFenceClosingLine(lines[j], opening) {
				j++
			}
			closed := j < stop
			end := j
			if !closed {
				end = stop - 1
			}
			blk := liveBlock{kind: bkCode, start: i, end: end, raw: strings.Join(lines[i:end+1], "\n"), info: fenceInfo(line)}
			if closed {
				blk.lines = lines[i+1 : j]
			} else if i+1 < stop {
				blk.lines = lines[i+1 : stop]
			}
			out = append(out, blk)
			i = end + 1
		case t == "":
			out = append(out, liveBlock{kind: bkBlank, start: i, end: i, raw: line})
			i++
		case t == "---" || t == "***":
			out = append(out, liveBlock{kind: bkRule, start: i, end: i, raw: line})
			i++
		case isHeading(t):
			level := 1
			for level < len(t) && t[level] == '#' {
				level++
			}
			if level > 4 {
				level = 4
			}
			blk := liveBlock{kind: bkHeading, start: i, end: i, raw: line, level: level}
			blk.text = stripTimeSuffix(&blk, strings.TrimSpace(t[level:]))
			blk.spans = inlineSpans(blk.text)
			out = append(out, blk)
			i++
		case strings.HasPrefix(t, ">"):
			blk := liveBlock{kind: bkQuote, start: i, end: i, raw: line}
			blk.text = stripTimeSuffix(&blk, strings.TrimSpace(strings.TrimPrefix(t, ">")))
			blk.spans = inlineSpans(blk.text)
			out = append(out, blk)
			i++
		case taskPrefixLen(s) > 0:
			mark := s[:taskPrefixLen(s)]
			blk := liveBlock{kind: bkTask, start: i, end: i, raw: line,
				checked: strings.Contains(mark, "[x]")}
			blk.text = stripTimeSuffix(&blk, strings.TrimSpace(s[len(mark):]))
			blk.spans = inlineSpans(blk.text)
			out = append(out, blk)
			i++
		case listMarkerLen(s) > 0:
			n := listMarkerLen(s)
			blk := liveBlock{kind: bkList, start: i, end: i, raw: line, n: orderedNumber(s)}
			blk.text = stripTimeSuffix(&blk, strings.TrimSpace(s[n:]))
			blk.spans = inlineSpans(blk.text)
			out = append(out, blk)
			i++
		default: // 段落：吸收连续普通行
			j := i
			for j+1 < stop && !mdLineSpecial(lines[j+1]) {
				if _, ok := tableStart(lines, j+1, stop); ok {
					break
				}
				j++
			}
			blk := liveBlock{kind: bkPara, start: i, end: j}
			parts := make([]string, 0, j-i+1)
			for k := i; k <= j; k++ {
				parts = append(parts, strings.TrimSpace(lines[k]))
			}
			blk.raw = strings.Join(lines[i:j+1], "\n")
			blk.text = stripTimeSuffix(&blk, strings.Join(parts, " "))
			blk.spans = inlineSpans(blk.text)
			out = append(out, blk)
			i = j + 1
		}
	}
	return out
}

// isLiveSeparator 区分源码分隔符与真正的空段落。
// 非空块后的第一个空行分隔下一段；多出的空行仍是可编辑空段落。
// 文档开头没有上一非空块：至少两个前导空行时，最后一个分隔后面的正文。
// 单个末尾空行是光标所在的空段落，不能当成分隔符跳过。
func isLiveSeparator(blocks []liveBlock, i int) bool {
	if i <= 0 || i >= len(blocks)-1 || blocks[i].kind != bkBlank || blocks[i].raw != "" {
		return false
	}
	if blocks[i-1].kind != bkBlank {
		return true
	}
	if blocks[i+1].kind == bkBlank {
		return false
	}
	for _, b := range blocks[:i] {
		if b.kind != bkBlank {
			return false
		}
	}
	return true
}

// bareListMarker 返回整行恰好是列表/待办标记（"- "、"1. "、"- [ ] " 等）时的标记文本。
func bareListMarker(line string) string {
	s := strings.TrimLeft(line, " \t")
	for _, m := range []string{"- [ ] ", "- [x] ", "* [ ] ", "* [x] ", "- ", "* "} {
		if s == m {
			return m
		}
	}
	if n := listMarkerLen(s); n > 0 && n == len(s) {
		return s
	}
	return ""
}

// listContinuePrefix 返回列表/待办块的续行标记（Enter 空项自动补全用），非列表返回 ""。
func listContinuePrefix(b liveBlock) string {
	t := strings.TrimSpace(b.raw)
	switch b.kind {
	case bkTask:
		if n := taskPrefixLen(t); n > 0 {
			return strings.Replace(t[:n], "[x]", "[ ]", 1)
		}
	case bkList:
		if n := listMarkerLen(t); n > 0 {
			if strings.HasSuffix(t[:n], ". ") {
				if v := orderedNumber(t); v > 0 {
					return fmt.Sprintf("%d. ", v+1)
				}
			}
			return t[:n]
		}
	}
	return ""
}

// blockOfRuneOffset 把文档 rune 偏移映射到所在块索引及块首的 rune 偏移。
// 文档 = 各块 raw 依次相连（块间由换行分隔，末尾无换行）。
func blockOfRuneOffset(blocks []liveBlock, runeOff int) (idx, blockStart int) {
	off := 0
	for k, b := range blocks {
		w := utf8.RuneCountInString(b.raw) + 1 // 块内容 + 其后的换行符
		if runeOff < off+w {
			return k, off
		}
		off += w
	}
	if len(blocks) == 0 {
		return 0, 0
	}
	last := len(blocks) - 1
	return last, off - utf8.RuneCountInString(blocks[last].raw) - 1
}

// sameBlock 比较两个块（liveBlock 含切片字段，不能直接 ==）。
func sameBlock(a, b liveBlock) bool {
	if a.kind != b.kind || a.start != b.start || a.end != b.end || a.raw != b.raw ||
		a.level != b.level || a.n != b.n || a.checked != b.checked || a.text != b.text || a.recur != b.recur ||
		len(a.lines) != len(b.lines) || len(a.spans) != len(b.spans) {
		return false
	}
	for i := range a.lines {
		if a.lines[i] != b.lines[i] {
			return false
		}
	}
	return true
}
