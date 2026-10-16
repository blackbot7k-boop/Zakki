package markdown

import (
	"strings"
	"unicode/utf8"
)

// protectedBlockLines 标记不能被行级格式命令改写的结构行。
// 围栏未闭合时保护到文档末尾；表格范围沿用编辑器的有效表格模型。
func protectedBlockLines(doc string) []bool {
	lines := strings.Split(doc, "\n")
	protected := make([]bool, len(lines))
	var opening fenceMarker
	for i := 0; i < len(lines); {
		if opening.size > 0 {
			protected[i] = true
			if isFenceClosingLine(lines[i], opening) {
				opening = fenceMarker{}
			}
			i++
			continue
		}
		if marker, ok := parseFenceOpeningLine(lines[i]); ok {
			protected[i] = true
			opening = marker
			i++
			continue
		}
		table, ok := tableStart(lines, i, len(lines))
		if !ok {
			i++
			continue
		}
		columns := len(table.Align)
		protected[i], protected[i+1] = true, true
		i += 2
		for i < len(lines) {
			if _, ok := tableRowCells(lines[i], columns); !ok {
				break
			}
			protected[i] = true
			i++
		}
	}
	return protected
}

// inlineProtectedBlockLines 仅在非空选区完全落入一个真实表格单元格时，
// 允许行内命令处理该行；围栏、表格结构和跨单元格选区仍受保护。
func inlineProtectedBlockLines(doc string, selStart, selEnd int) []bool {
	protected := protectedBlockLines(doc)
	if selStart >= selEnd {
		return protected
	}
	lines := strings.Split(doc, "\n")
	lineStarts := make([]int, len(lines))
	for i := 1; i < len(lines); i++ {
		lineStarts[i] = lineStarts[i-1] + utf8.RuneCountInString(lines[i-1]) + 1
	}
	var opening fenceMarker
	for i := 0; i < len(lines); {
		if opening.size > 0 {
			if isFenceClosingLine(lines[i], opening) {
				opening = fenceMarker{}
			}
			i++
			continue
		}
		if marker, ok := parseFenceOpeningLine(lines[i]); ok {
			opening = marker
			i++
			continue
		}
		table, ok := tableStart(lines, i, len(lines))
		if !ok {
			i++
			continue
		}
		end := i + 1
		columns := len(table.Align)
		for end+1 < len(lines) {
			if _, ok := tableRowCells(lines[end+1], columns); !ok {
				break
			}
			end++
		}
		cells := tableCellPositions(strings.Join(lines[i:end+1], "\n"))
		for _, cell := range cells {
			start := lineStarts[i] + cell.Start
			endOffset := lineStarts[i] + cell.End
			if selStart >= start && selEnd <= endOffset {
				for line := i; line <= end; line++ {
					protected[line] = false
				}
				return protected
			}
		}
		i = end + 1
	}
	return protected
}
