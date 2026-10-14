package markdown

import (
	"strings"
	"unicode/utf8"
)

type tableCell struct {
	Text       string
	Start, End int // rune 偏移，相对所在行，去掉两侧空白
}

type markdownTable struct {
	Rows  [][]tableCell // 表头 + 数据行，分隔行不在 rows 内
	Align []int         // -1 左、0 默认、1 右、2 居中
}

// pipeCells 只接受带外侧竖线的行；转义的竖线属于单元格内容。
func pipeCells(line string) ([]tableCell, bool) {
	rs := []rune(line)
	lo, hi := 0, len(rs)
	for lo < hi && (rs[lo] == ' ' || rs[lo] == '\t') {
		lo++
	}
	for hi > lo && (rs[hi-1] == ' ' || rs[hi-1] == '\t') {
		hi--
	}
	if hi-lo < 3 || rs[lo] != '|' || rs[hi-1] != '|' {
		return nil, false
	}
	var cells []tableCell
	start := lo + 1
	escaped := false
	for i := start; i < hi; i++ {
		if rs[i] == '|' && !escaped {
			s, e := start, i
			for s < e && (rs[s] == ' ' || rs[s] == '\t') {
				s++
			}
			for e > s && (rs[e-1] == ' ' || rs[e-1] == '\t') {
				e--
			}
			cells = append(cells, tableCell{Text: string(rs[s:e]), Start: s, End: e})
			start = i + 1
		}
		if rs[i] == '\\' {
			escaped = !escaped
		} else {
			escaped = false
		}
	}
	if start != hi || len(cells) == 0 {
		return nil, false
	}
	return cells, true
}

func tableStart(lines []string, i, stop int) (markdownTable, bool) {
	if i+1 >= stop {
		return markdownTable{}, false
	}
	head, ok := pipeCells(lines[i])
	if !ok {
		return markdownTable{}, false
	}
	divider, ok := pipeCells(lines[i+1])
	if !ok || len(divider) != len(head) {
		return markdownTable{}, false
	}
	align := make([]int, len(head))
	for i, cell := range divider {
		s := cell.Text
		left, right := strings.HasPrefix(s, ":"), strings.HasSuffix(s, ":")
		s = strings.TrimSuffix(strings.TrimPrefix(s, ":"), ":")
		if len(s) < 3 || strings.Trim(s, "-") != "" {
			return markdownTable{}, false
		}
		switch {
		case left && right:
			align[i] = 2
		case right:
			align[i] = 1
		case left:
			align[i] = -1
		}
	}
	return markdownTable{Rows: [][]tableCell{head}, Align: align}, true
}

func emptyTableRow(n int) string { return "|" + strings.Repeat("   |", n) }

func tableCellPositions(raw string) []tableCell {
	lines := strings.Split(raw, "\n")
	var out []tableCell
	off := 0
	for row, line := range lines {
		if row != 1 {
			cells, _ := pipeCells(line)
			for _, cell := range cells {
				cell.Start += off
				cell.End += off
				out = append(out, cell)
			}
		}
		off += utf8.RuneCountInString(line) + 1
	}
	return out
}
