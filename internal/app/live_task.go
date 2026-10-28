package app

import (
	"encoding/json"
	"log/slog"
	"time"

	"zakki/internal/markdown"
)

// toggleTask 统一普通与手账复选框的文档事务，推进与勾选一起撤销。
func (a *app) toggleTask(blk liveBlock) {
	if blk.kind != bkTask {
		return
	}
	before := a.liveSnapshot()
	updated := toggleTaskLine(a.activeTab().Text, blk.start)
	if !blk.checked && blk.hasTime && blk.recur != "" {
		rule, err := markdown.ParseRule(blk.recur)
		if err == nil {
			value, err := markdown.Advance(rule, blk.due.UnixMilli(), a.now().UnixMilli())
			var next timeOccurrence
			if err != nil || json.Unmarshal([]byte(value), &next) != nil {
				slog.Error("重复任务推进失败", "rule", blk.recur, "err", err)
				return
			}
			if !next.Ended {
				raw := replaceBlockTimeToken(blk.raw, &markdown.TimeSuffix{Time: time.UnixMilli(next.DueMs).In(blk.due.Location()), Alert: blk.alert, HasAlert: blk.hasAlert, Recur: blk.recur})
				updated = spliceDocLines(a.activeTab().Text, blk.start, blk.end, raw)
			}
		}
	}
	a.activeTab().Text = updated
	a.resyncLive(a.mdCaret)
	a.rememberLiveEdit(before, editCommand)
	a.onEdited()
}
