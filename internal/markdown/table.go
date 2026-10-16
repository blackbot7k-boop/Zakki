package markdown

import (
	"strings"
	"unicode/utf8"
)

type tableCell struct {
	Text       string
	Start, End int  // rune 偏移，相对所在行，去掉两侧空白
	Missing    bool // 源行缺少该语义列；编辑前需要先补成真实单元格
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

// tableRowCells 按表头列数返回可见单元格。宽行截掉不可见的额外列，
// 短行补虚拟单元格；虚拟位置落在闭合竖线前，调用方不能直接回写该范围。
func tableRowCells(line string, columns int) ([]tableCell, bool) {
	cells, ok := pipeCells(line)
	if !ok || columns <= 0 {
		return nil, false
	}
	if len(cells) > columns {
		cells = cells[:columns]
	}
	at := tableRowInsertOffset(line)
	for len(cells) < columns {
		cells = append(cells, tableCell{Start: at, End: at, Missing: true})
	}
	return cells, true
}

func tableRowInsertOffset(line string) int {
	rs := []rune(line)
	i := len(rs)
	for i > 0 && (rs[i-1] == ' ' || rs[i-1] == '\t') {
		i--
	}
	if i > 0 && rs[i-1] == '|' {
		return i - 1
	}
	return i
}

// normalizeTableRow 把短行补到表头列数。它只在用户开始编辑缺失格时使用；
// 已经足够宽的源行原样保留，避免无关的 Markdown 格式化。
func normalizeTableRow(line string, columns int) (string, bool) {
	cells, ok := pipeCells(line)
	if !ok || columns <= 0 {
		return line, false
	}
	if len(cells) >= columns {
		return line, true
	}
	values := make([]string, columns)
	for i, cell := range cells {
		values[i] = cell.Text
	}
	return "| " + strings.Join(values, " | ") + " |", true
}

func tableCellPositions(raw string) []tableCell {
	lines := strings.Split(raw, "\n")
	table, ok := tableStart(lines, 0, len(lines))
	if !ok {
		return nil
	}
	columns := len(table.Align)
	out := make([]tableCell, 0, (len(lines)-1)*columns)
	off := 0
	for row, line := range lines {
		if row != 1 {
			cells, ok := tableRowCells(line, columns)
			if !ok {
				break
			}
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
