package app

import (
	"testing"
	"time"
	"zakki/locales"
)

func TestRecurringChipStateMatrix(t *testing.T) {
	now := time.Date(2024, 6, 10, 12, 0, 0, 0, time.Local)
	for _, tc := range []struct {
		raw   string
		state timeChipState
		label string
	}{
		{"- [ ] 今天 @2024-06-10 09:00 ^1d", chipToday, locales.TimeChipToday("09:00")},
		{"- [ ] 未来 @2024-06-11 09:00 ^1mo", chipFuture, ""},
		{"- [ ] 过去 @2024-06-01 09:00 ^1mo", chipNeutral, ""},
		{"- [ ] 结束 @2024-06-01 09:00 ^1mo<2024-06-01", chipNeutral, ""},
		{"- [x] 完成 @2024-06-10 09:00 ^1d", chipNeutral, "09:00"},
		{"- [ ] 普通 @2024-06-10 09:00", chipOverdue, ""},
		{"正文 @2024-06-10 09:00 ^1d", chipNeutral, ""},
	} {
		t.Run(tc.raw, func(t *testing.T) {
			blk := liveBlocks(tc.raw)[0]
			if got := timeChipStateFor(blk, now); got != tc.state {
				t.Fatalf("state=%v want %v", got, tc.state)
			}
			if tc.label != "" {
				if got := timeBlockLabel(blk, now); got != tc.label {
					t.Fatalf("label=%q want %q", got, tc.label)
				}
			}
		})
	}
}

func TestRecurringReminderPerInstanceLeadAndHorizon(t *testing.T) {
	now := time.Date(2024, 6, 10, 9, 0, 0, 0, time.Local)
	entries := collectReminderTab(reminderTab{id: 1, text: "- [ ] 每天 @2024-06-10 10:00 ~1d ^1d"}, now)
	if len(entries) != 60 {
		t.Fatalf("instances=%d want 60", len(entries))
	}
	seen := map[string]bool{}
	for _, e := range entries {
		if !e.fireAt.Equal(e.due.Add(-24 * time.Hour)) {
			t.Fatalf("lead=%v", e)
		}
		if seen[reminderKey(e)] {
			t.Fatal("实例去重键重复")
		}
		seen[reminderKey(e)] = true
	}
	if !entries[0].fireAt.Before(now) || !entries[0].due.After(now) {
		t.Fatal("提前量已过的未来实例必须收集")
	}
}

func TestRecurringReminderLateEligibilitySurvivesReparse(t *testing.T) {
	a := newTestApp(t)
	t.Cleanup(a.shutdown)
	fake := connectFakeNotifier(a)
	start := time.Date(2024, 6, 10, 9, 0, 0, 0, time.Local)
	a.now = func() time.Time { return start }
	a.tabs[0].Text = "- [ ] 原文 @2024-06-10 10:00 ~0m ^1d<2024-06-10"
	a.rebuildReminders()
	start = start.Add(time.Hour + 30*time.Second)
	a.tabs[0].Text = "- [ ] 新文 @2024-06-10 10:00 ~0m ^1d<2024-06-10"
	a.rebuildReminders()
	expectReminderBody(t, fake, "新文")
	a.rebuildReminders()
	expectNoReminder(t, fake, 10*time.Millisecond)
}

func TestRecurringReminderCacheRollsWindow(t *testing.T) {
	a := newTestApp(t)
	t.Cleanup(a.shutdown)
	connectFakeNotifier(a)
	now := time.Date(2024, 6, 10, 9, 0, 0, 0, time.Local)
	a.now = func() time.Time { return now }
	a.tabs[0].Text = "- [ ] 每天 @2024-06-10 10:00 ~0m ^1d"
	a.rebuildReminders()
	now = now.AddDate(0, 0, 2)
	a.rebuildReminders()
	a.reminders.mu.Lock()
	defer a.reminders.mu.Unlock()
	if len(a.reminders.pending) != 60 {
		t.Fatalf("pending=%d", len(a.reminders.pending))
	}
	for _, e := range a.reminders.pending {
		if e.due.Before(now) {
			t.Fatalf("过期实例残留: %v", e.due)
		}
	}
}
