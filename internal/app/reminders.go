package app

// 提醒调度：从文档派生应触发的提醒（术语表：提醒存在 && 时刻在未来 &&
// （非任务或未完成）），到点投递系统通知并弹应用内 toast。
//
// 状态在 reminderScheduler 内部 mutex 中：主线程 reschedule/stop，触发闭包在
// 后台 goroutine 读代号防陈旧（模式同 notes/git_schedule）。应用退出后不投递，
// 错过的提醒不补发（只显示过期状态，见 docs/adr/0001）。

import (
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"zakki/internal/sysnotify"
)

type reminderEntry struct {
	tabID  uint64
	line   int
	title  string
	text   string
	due    time.Time
	alert  time.Duration
	fireAt time.Time
}

// reminderScheduler 保存当前全部在途定时器；reschedule 全量重建（便签规模
// 行数有限，全量重建比增量对账简单且无对账 bug 面）。mutex 保护代号与
// 定时器表：主线程 reschedule/stop，触发闭包在后台 goroutine 读代号防陈旧
// （模式同 notes/git_schedule）。应用退出后不投递，错过的提醒不补发
// （只显示过期状态，见 docs/adr/0001）。
type reminderScheduler struct {
	mu        sync.Mutex
	gen       int
	timers    []*time.Timer
	debounce  *time.Timer
	notifier  sysnotify.Notifier
	connected bool // Run 装配后为 true；未装配时不调度
}

// connectReminders 注入投递器并做首次调度（Run 装配期调用一次）。
func (a *app) connectReminders() {
	a.reminders.mu.Lock()
	a.reminders.notifier = sysnotify.New()
	a.reminders.connected = true
	a.reminders.mu.Unlock()
	a.rescheduleReminders()
}

// stopReminders 停掉全部在途定时器（退出流程调用）。
func (a *app) stopReminders() {
	a.reminders.mu.Lock()
	defer a.reminders.mu.Unlock()
	a.reminders.invalidateLocked()
	a.reminders.connected = false
}

// invalidateLocked 在编辑当下作废旧代，而不是等防抖结束后才作废。
func (s *reminderScheduler) invalidateLocked() {
	s.gen++
	for _, t := range s.timers {
		t.Stop()
	}
	s.timers = nil
	if s.debounce != nil {
		s.debounce.Stop()
		s.debounce = nil
	}
}

// rescheduleReminders 在主线程采集快照；防抖回调不再访问文档。
func (a *app) rescheduleReminders() {
	a.reminders.mu.Lock()
	if !a.reminders.connected {
		a.reminders.mu.Unlock()
		return
	}
	a.reminders.invalidateLocked()
	gen := a.reminders.gen
	a.reminders.mu.Unlock()

	entries := a.collectReminders()
	now, started := a.now(), time.Now()
	a.reminders.mu.Lock()
	defer a.reminders.mu.Unlock()
	if gen != a.reminders.gen || !a.reminders.connected {
		return
	}
	a.reminders.debounce = time.AfterFunc(250*time.Millisecond, func() {
		a.scheduleReminderSnapshot(entries, gen, now.Add(time.Since(started)))
	})
}

// collectReminders 派生当前全部待触发提醒（主线程调用，读文档与标签名）。
func (a *app) collectReminders() []reminderEntry {
	now := a.now()
	var out []reminderEntry
	for ti := range a.tabs {
		tab := &a.tabs[ti]
		for _, blk := range liveBlocks(tab.Text) {
			if !blk.hasTime || !blk.hasAlert {
				continue
			}
			// 派生规则：未完成任务或非任务行，且触发时刻（截止减提前量）在未来。
			if blk.kind == bkTask && blk.checked {
				continue
			}
			fireAt := blk.due.Add(-blk.alert)
			if !fireAt.After(now) {
				continue // 错过的不补发
			}
			text := projectRegion(blk.text, 0, utf8.RuneCountInString(blk.text), true, false).text
			out = append(out, reminderEntry{
				tabID: tab.ID, line: blk.start,
				title: a.tabTitle(ti), text: strings.Join(strings.Fields(text), " "),
				due: blk.due, alert: blk.alert, fireAt: fireAt,
			})
		}
	}
	return out
}

// rebuildReminders 由主线程直接采集并重建，保留测试与即时调度入口。
func (a *app) rebuildReminders() {
	a.reminders.mu.Lock()
	if !a.reminders.connected {
		a.reminders.mu.Unlock()
		return
	}
	a.reminders.invalidateLocked()
	gen := a.reminders.gen
	a.reminders.mu.Unlock()
	entries := a.collectReminders()
	a.scheduleReminderSnapshot(entries, gen, a.now())
}

// scheduleReminderSnapshot 只消费不可变快照；迟到的防抖回调不能复活旧代。
func (a *app) scheduleReminderSnapshot(entries []reminderEntry, gen int, now time.Time) {
	a.reminders.mu.Lock()
	defer a.reminders.mu.Unlock()
	if gen != a.reminders.gen || !a.reminders.connected {
		return
	}
	a.reminders.debounce = nil
	for _, e := range entries {
		e := e
		delay := e.fireAt.Sub(now)
		if delay < 0 {
			delay = 0
		}
		a.reminders.timers = append(a.reminders.timers, time.AfterFunc(delay, func() {
			a.fireReminder(e, gen)
		}))
	}
}

// fireReminder 到点投递：系统通知在后台 goroutine 同步执行（外部命令），
// 应用内 toast 经主线程投递。代号不符说明调度已重建，本条作废。
func (a *app) fireReminder(e reminderEntry, gen int) {
	a.reminders.mu.Lock()
	stale := gen != a.reminders.gen || !a.reminders.connected
	notifier := a.reminders.notifier
	a.reminders.mu.Unlock()
	if stale {
		return
	}
	body := fmt.Sprintf("%s · %s", e.title, e.text)
	if notifier != nil {
		go func() {
			a.reminders.mu.Lock()
			stale := gen != a.reminders.gen || !a.reminders.connected
			a.reminders.mu.Unlock()
			if stale {
				return
			}
			if err := notifier.Notify(sysnotify.Notification{
				ID:    fmt.Sprintf("zakki-reminder-%d-%d", e.tabID, e.line),
				Title: "Zakki", Body: body,
			}); err != nil {
				slog.Warn("系统通知投递失败", "err", err)
			}
		}()
	}
	a.runOnMain(func() {
		a.reminders.mu.Lock()
		stale := gen != a.reminders.gen || !a.reminders.connected
		a.reminders.mu.Unlock()
		if !stale {
			a.showToast(body)
		}
	})
}
