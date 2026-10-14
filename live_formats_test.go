package main

import "testing"

func runeLen(s string) int { return len([]rune(s)) }

// 默认表格必须能被实时预览切成一张表。格式变换本身的断言在 internal/markdown。
func TestDefaultTableBlockIsLiveTable(t *testing.T) {
	blocks := liveBlocks(defaultTableBlock())
	if len(blocks) != 1 || blocks[0].kind != bkTable || len(blocks[0].table.rows) != 3 {
		t.Fatalf("默认表格两列三行（表头+两行正文）: %+v", blocks)
	}
}
