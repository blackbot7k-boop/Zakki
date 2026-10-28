package app

import (
	"strings"
	"testing"
	"time"
)

func reminderToken(at time.Time) string {
	return "@" + at.In(time.Local).Format("2006-01-02 15:04")
}

func TestReminderQuoteCompletedAndImmediatePolicy(t *testing.T) {
	a := newTestApp(t)
	t.Cleanup(a.shutdown)
	future := a.now().Add(2 * time.Hour)
	a.tabs[0].Text = "> - [x] 完成 " + reminderToken(future) + " ~1d\n> - [ ] 未完成 " + reminderToken(future) + " ~1d"
	got := a.collectReminders()
	if len(got) != 1 || !strings.Contains(got[0].text, "未完成") {
		t.Fatalf("引用任务派生 = %+v", got)
	}
	fake := connectFakeNotifier(a)
	a.rebuildReminders() // 首次启动：fireAt 已过，不补。
	expectNoReminder(t, fake, 50*time.Millisecond)
	a.tabs[0].Text = "- [ ] 新事件 " + reminderToken(future) + " ~1d"
	a.rebuildReminders()
	expectReminderBody(t, fake, "新事件")
}

func TestReminderEditedBodyCrossesDueWithoutWindow(t *testing.T) {
	a := newTestApp(t)
	t.Cleanup(a.shutdown)
	fake := connectFakeNotifier(a)
	due := time.Now().In(time.Local).Add(time.Hour).Truncate(time.Minute)
	base := due.Add(-100 * time.Millisecond)
	started := time.Now()
	a.setTestNow(func() time.Time { return base.Add(time.Since(started)) })
	a.tabs[0].Text = "- [ ] 旧正文 " + reminderToken(due) + " ~0m"
	a.rebuildReminders()
	a.tabs[0].Text = "- [ ] 新正文 " + reminderToken(due) + " ~0m"
	a.rescheduleReminders() // 旧正文已排；关窗后仍需投递系统通知，不靠 runOnMain。
	if a.mainWindow.Load() != nil {
		t.Fatal("测试应保持无窗口")
	}
	expectReminderBody(t, fake, "新正文")
	select {
	case n := <-fake.ch:
		t.Fatalf("旧正文或重复通知: %+v", n)
	default:
	}
}

func TestReminderStartupDoesNotBackfillPastDue(t *testing.T) {
	a := newTestApp(t)
	t.Cleanup(a.shutdown)
	fake := connectFakeNotifier(a)
	due := time.Now().In(time.Local).Truncate(time.Minute)
	a.setTestNow(func() time.Time { return due.Add(30 * time.Second) })
	a.tabs[0].Text = "旧提醒 " + reminderToken(due) + " ~0m"
	a.rebuildReminders()
	select {
	case n := <-fake.ch:
		t.Fatalf("首次启动不应补发: %+v", n)
	default:
	}
}

func TestReminderAlertMovedEarlierBeforeDue(t *testing.T) {
	a := newTestApp(t)
	t.Cleanup(a.shutdown)
	fake := connectFakeNotifier(a)
	due := a.now().Add(24 * time.Hour)
	a.tabs[0].Text = "旧正文 " + reminderToken(due) + " ~0m"
	a.rebuildReminders()
	a.tabs[0].Text = "新正文 " + reminderToken(due) + " ~1d"
	a.rescheduleReminders()
	expectReminderBody(t, fake, "新正文")
}

func TestReminderEditCrossesDebounceImmediately(t *testing.T) {
	a := newTestApp(t)
	t.Cleanup(a.shutdown)
	fake := connectFakeNotifier(a)
	due := time.Now().In(time.Local).Add(time.Hour).Truncate(time.Minute)
	base := due.Add(-time.Hour - 100*time.Millisecond)
	started := time.Now()
	a.setTestNow(func() time.Time { return base.Add(time.Since(started)) })
	a.tabs[0].Text = "- [ ] 任务 " + reminderToken(due) + " ~1h"
	a.rebuildReminders()
	a.rescheduleReminders() // 立即撤旧定时器，250ms 后已越过触发时刻。
	expectReminderBody(t, fake, "任务")
	a.reconcileReminders(a.now())
	select {
	case n := <-fake.ch:
		t.Fatalf("重复投递: %+v", n)
	default:
	}
}

func TestReminderPollsLateOnce(t *testing.T) {
	a := newTestApp(t)
	t.Cleanup(a.shutdown)
	fake := connectFakeNotifier(a)
	base := a.now()
	due := base.Add(2 * time.Hour)
	e := reminderEntry{tabID: a.tabs[0].ID, text: "迟到", due: due, fireAt: base.Add(100 * time.Millisecond)}
	a.scheduleReminderSnapshot([]reminderEntry{e}, reminderGeneration(a), base)
	// 模拟合盖后巡检发现旧快照已到点，必须按同一 key 只发一次。
	a.reconcileReminders(base.Add(200 * time.Millisecond))
	expectReminderBody(t, fake, "迟到")
	a.reconcileReminders(base.Add(300 * time.Millisecond))
	expectNoReminder(t, fake, 50*time.Millisecond)
}

func TestReminderPollWaitsForEditedBody(t *testing.T) {
	a := newTestApp(t)
	t.Cleanup(a.shutdown)
	fake := connectFakeNotifier(a)
	due := time.Now().In(time.Local).Add(time.Hour).Truncate(time.Minute)
	base := due.Add(-time.Hour - 100*time.Millisecond)
	started := time.Now()
	a.setTestNow(func() time.Time { return base.Add(time.Since(started)) })
	a.tabs[0].Text = "旧正文 " + reminderToken(due) + " ~1h"
	a.rebuildReminders()
	a.tabs[0].Text = "新正文 " + reminderToken(due) + " ~1h"
	a.rescheduleReminders()
	a.reconcileReminders(base.Add(200 * time.Millisecond))
	select {
	case n := <-fake.ch:
		t.Fatalf("巡检不应投递编辑前正文: %+v", n)
	default:
	}
	expectReminderBody(t, fake, "新正文")
}

func TestReminderEmptyTitleAndStableIdentity(t *testing.T) {
	a := newTestApp(t)
	t.Cleanup(a.shutdown)
	fake := connectFakeNotifier(a)
	a.rebuildReminders()
	e := reminderEntry{tabID: 1, line: 1, text: "正文", due: a.now().Add(time.Hour)}
	a.fireReminder(e, reminderGeneration(a))
	expectReminderBody(t, fake, "正文")
	e.line = 100
	a.fireReminder(e, reminderGeneration(a))
	expectNoReminder(t, fake, 50*time.Millisecond)
}

func TestReminderReloadReplacesSchedule(t *testing.T) {
	a := newTestApp(t)
	t.Cleanup(a.shutdown)
	connectFakeNotifier(a)
	future := reminderToken(a.now().Add(24 * time.Hour))
	a.tabs[0].Text = "旧 " + future + " ~10m"
	a.rebuildReminders()
	old := a.collectReminders()[0]
	gen := reminderGeneration(a)
	file := a.snapshotFile()
	file.Tabs[0].Text = "新 " + future + " ~10m"
	a.applySavedFile(file)
	if reminderGeneration(a) == gen {
		t.Fatal("载入成功应作废旧代")
	}
	a.fireReminder(old, gen)
	a.rebuildReminders()
	got := a.collectReminders()
	if len(got) != 1 || got[0].text != "新" {
		t.Fatalf("重读未更新排程: %+v", got)
	}
}

func TestReminderTabChangesAndQuitResume(t *testing.T) {
	a := newTestApp(t)
	t.Cleanup(a.shutdown)
	fake := connectFakeNotifier(a)
	future := reminderToken(a.now().Add(24 * time.Hour))
	a.tabs[0].Text = "一 " + future + " ~10m"
	a.rebuildReminders()
	a.renameTab(0, "新标题")
	a.rebuildReminders()
	renamed := a.collectReminders()
	if len(renamed) != 1 || renamed[0].title != "新标题" {
		t.Fatalf("重命名后通知标题 = %+v", renamed)
	}
	oldGen := reminderGeneration(a)
	if !a.deleteTabNow(0) {
		t.Fatal("删除失败")
	}
	a.fireReminder(renamed[0], oldGen)
	select {
	case n := <-fake.ch:
		t.Fatalf("删除后旧提醒仍投递: %+v", n)
	default:
	}
	// 迁址阻止退出后必须恢复调度能力。
	a.storageMoving = true
	if err := a.shutdownForQuit(); err == nil {
		t.Fatal("迁址中退出应失败")
	}
	a.storageMoving = false
	a.reminders.mu.Lock()
	connected := a.reminders.connected
	a.reminders.mu.Unlock()
	if !connected {
		t.Fatal("退出取消后提醒未恢复")
	}
	old := reminderEntry{tabID: 1, title: "旧标题", text: "一", due: a.now().Add(time.Hour)}
	a.fireReminder(old, reminderGeneration(a)-1)
	expectNoReminder(t, fake, 50*time.Millisecond)
}
