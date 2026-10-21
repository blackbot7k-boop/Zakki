package app

import (
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"zakki/locales"
)

// statusWarn 状态栏警示文案：持久化错误优先，其次载入提示。
func (a *app) statusWarn() string {
	msg := a.saveError()
	if msg == "" {
		msg = a.notice
	}
	if r := []rune(msg); len(r) > 40 {
		msg = string(r[:40]) + "…"
	}
	return msg
}

// countWords 统计词数（以空白分隔；中文连续文本按 1 词计）。
// 零分配：状态栏每帧都调，不用 strings.Fields（会拷出全部词）。
func countWords(s string) int {
	n, inWord := 0, false
	for _, r := range s {
		if unicode.IsSpace(r) {
			inWord = false
			continue
		}
		if !inWord {
			n++
			inWord = true
		}
	}
	return n
}

// countChars 统计字符数（按 rune）。
func countChars(s string) int { return utf8.RuneCountInString(s) }

// countLines 统计行数（空文本为 0 行，与 Tot 一致）。
func countLines(s string) int {
	if s == "" {
		return 0
	}
	return strings.Count(s, "\n") + 1
}

// backupLabel 把最近备份时间转成人性化短句。
func (a *app) backupLabel() string {
	last := a.lastBackupTime()
	if last.IsZero() {
		return locales.StatusNeverBackedUp()
	}
	d := a.now().Sub(last)
	switch {
	case d < time.Minute:
		return locales.StatusBackedUpJustNow()
	case d < time.Hour:
		return locales.StatusBackedUpMinutes(int(d.Minutes()))
	case d < 24*time.Hour:
		return locales.StatusBackedUpHours(int(d.Hours()))
	default:
		return locales.StatusBackedUpAt(last.Format("01-02 15:04"))
	}
}

// statsDebounce 长文档连续输入时底栏统计的最短重算间隔；停手后补一帧算准。
const statsDebounce = 250 * time.Millisecond

// statsDebounceBytes 以下的文档全文扫描只需微秒级，每次改动都即时重算。
const statsDebounceBytes = 32 << 10

// docStats 返回状态栏的行数/词数/字符数。状态栏每帧重建，按文档串缓存；
// 同一标签的长文档在连续输入时按 statsDebounce 节流，期间沿用上次结果，
// 并预约一次刷新，停手后数字仍会追上。切换标签总是立即重算。
func (a *app) docStats(doc string) (lines, words, chars int) {
	if a.statsCacheDoc == doc {
		return a.statsCacheLines, a.statsCacheWords, a.statsCacheChars
	}
	now, tab := a.now(), a.activeTab().ID
	if tab == a.statsCacheTab && len(doc) >= statsDebounceBytes && now.Sub(a.statsCacheAt) < statsDebounce {
		a.armStatsRefresh(now)
		return a.statsCacheLines, a.statsCacheWords, a.statsCacheChars
	}
	lines, words, chars = countLines(doc), countWords(doc), countChars(doc)
	a.statsCacheDoc, a.statsCacheLines, a.statsCacheWords, a.statsCacheChars = doc, lines, words, chars
	a.statsCacheTab, a.statsCacheAt = tab, now
	return lines, words, chars
}

// armStatsRefresh 节流期内预约一次重建，每个节流窗口至多一个定时器。
func (a *app) armStatsRefresh(now time.Time) {
	if a.win == nil || now.Before(a.statsRefreshAt) {
		return
	}
	a.statsRefreshAt = now.Add(statsDebounce)
	win := a.win
	time.AfterFunc(statsDebounce, func() { win.Update(func() {}) })
}
