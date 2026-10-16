package markdown

import "testing"

func TestIrregularTableCellsUseHeaderWidth(t *testing.T) {
	raw := "| a | b |\n| --- | --- |\n| only |\n| 1 | 2 | hidden |"
	cells := tableCellPositions(raw)
	if len(cells) != 6 {
		t.Fatalf("语义单元格数=%d, want 6", len(cells))
	}
	if cells[2].Text != "only" || cells[2].Missing || !cells[3].Missing {
		t.Fatalf("短行规范化模型错误: %+v", cells[2:4])
	}
	if cells[4].Text != "1" || cells[5].Text != "2" {
		t.Fatalf("宽行不应暴露隐藏列: %+v", cells[4:])
	}
}

func TestNormalizeTableRowMaterializesMissingCells(t *testing.T) {
	got, ok := normalizeTableRow("  | only |  ", 3)
	if !ok || got != "| only |  |  |" {
		t.Fatalf("短行补格=%q ok=%v", got, ok)
	}
	cells, ok := tableRowCells(got, 3)
	if !ok || len(cells) != 3 {
		t.Fatalf("补格后解析失败: %+v ok=%v", cells, ok)
	}
	for i, cell := range cells {
		if cell.Missing {
			t.Fatalf("第 %d 格仍是虚拟格: %+v", i, cell)
		}
	}

	wide := "| 1 | 2 | hidden |"
	if preserved, ok := normalizeTableRow(wide, 2); !ok || preserved != wide {
		t.Fatalf("宽行不应被无关格式化: %q ok=%v", preserved, ok)
	}
}
