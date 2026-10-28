package app

import (
	"testing"
	"time"
)

func TestReminderRenameDoesNotRedeliverSameOccurrence(t *testing.T) {
	a := newTestApp(t)
	t.Cleanup(a.shutdown)
	fake := connectFakeNotifier(a)
	a.rebuildReminders()
	e := reminderEntry{tabID: 1, title: "旧标题", text: "正文", due: a.now().Add(time.Hour)}
	a.fireReminder(e, reminderGeneration(a))
	expectReminderBody(t, fake, "旧标题 · 正文")
	e.title = "新标题"
	a.fireReminder(e, reminderGeneration(a))
	expectNoReminder(t, fake, 50*time.Millisecond)
}
