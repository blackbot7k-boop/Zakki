package app

import (
	"testing"
	"time"
)

func TestTimeTokenCannotCrossSourceSoftLine(t *testing.T) {
	for _, raw := range []string{
		"正文 @2026-11-01\n10:00 ~10m",
		"正文 @2026-11-01\n10:00 ~10m\n续行",
	} {
		blk := liveBlocks(raw)[0]
		if blk.hasTime || len(projectBlock(blk).objects) != 0 {
			t.Fatalf("跨源码行不应产生组件: %+v", blk)
		}
		entries := collectReminderTabs([]reminderTab{{id: 1, text: raw}}, time.Date(2026, 10, 28, 0, 0, 0, 0, time.Local))
		if len(entries) != 0 {
			t.Fatalf("跨源码行不应调度: %+v", entries)
		}
	}
	blk := liveBlocks("正文\n后续 @2026-11-01 10:00 ~10m")[0]
	if !blk.hasTime || blk.text != "正文 后续" {
		t.Fatalf("完整落在后续行的令牌应保留: %+v", blk)
	}
}
