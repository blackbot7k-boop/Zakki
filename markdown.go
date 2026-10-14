package main

// 块级结构与源码高亮辅助（行内解析与格式变换见 internal/markdown）。

import "strings"

type blockKind int

const (
	bkPara blockKind = iota
	bkHeading
	bkQuote
	bkCode
	bkRule
	bkList
	bkTask
	bkBlank
	bkTable
)

type block struct {
	kind    blockKind
	level   int
	text    string
	lines   []string
	checked bool
	lineNo  int
	n       int
}

func parseBlocks(src string) []block {
	var out []block
	for _, lb := range liveBlocks(src) {
		if lb.kind == bkBlank {
			continue
		}
		out = append(out, block{
			kind:    lb.kind,
			level:   lb.level,
			text:    lb.text,
			lines:   lb.lines,
			checked: lb.checked,
			lineNo:  lb.start,
			n:       lb.n,
		})
	}
	return out
}

func orderedNumber(trimmed string) int {
	n := listMarkerLen(trimmed)
	if n < 3 || trimmed[n-2] != '.' {
		return 0
	}
	v := 0
	for _, c := range trimmed[:n-2] {
		v = v*10 + int(c-'0')
	}
	return v
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

func taskPrefixLen(trimmed string) int {
	for _, mark := range []string{"- [ ] ", "* [ ] ", "- [x] ", "* [x] "} {
		if strings.HasPrefix(trimmed, mark) {
			return len(mark)
		}
	}
	return 0
}

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
