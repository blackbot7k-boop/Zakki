package app

import (
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
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

// backupLabel 把最近备份时间转成人性化短句（Tot 底栏的 "Just now"）。
func (a *app) backupLabel() string {
	last := a.lastBackupTime()
	if last.IsZero() {
		return "尚未备份"
	}
	d := a.now().Sub(last)
	switch {
	case d < time.Minute:
		return "刚刚备份"
	case d < time.Hour:
		return fmt.Sprintf("%d 分钟前备份", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%d 小时前备份", int(d.Hours()))
	default:
		return last.Format("01-02 15:04 备份")
	}
}

// mirrorLabel 底栏外部备份状态短句；未配置返回空串，warn 为真时加粗提示。
func (a *app) mirrorLabel() (string, bool) {
	st := a.notes.MirrorStatus()
	if st.Dir == "" {
		return "", false
	}
	if st.Err != "" {
		return "镜像失败", true
	}
	last := st.Last
	if last.IsZero() {
		return "尚未镜像", false
	}
	d := a.now().Sub(last)
	switch {
	case d < time.Minute:
		return "刚刚镜像", false
	case d < time.Hour:
		return fmt.Sprintf("%d 分钟前镜像", int(d.Minutes())), false
	case d < 24*time.Hour:
		return fmt.Sprintf("%d 小时前镜像", int(d.Hours())), false
	default:
		return last.Format("01-02 15:04 镜像"), false
	}
}

// docStats 返回状态栏的行数/词数/字符数。状态栏每帧重建，
// 按文档串缓存，避免对长文档的每帧三次全文扫描。
func (a *app) docStats(doc string) (lines, words, chars int) {
	if a.statsCacheDoc == doc {
		return a.statsCacheLines, a.statsCacheWords, a.statsCacheChars
	}
	lines, words, chars = countLines(doc), countWords(doc), countChars(doc)
	a.statsCacheDoc, a.statsCacheLines, a.statsCacheWords, a.statsCacheChars = doc, lines, words, chars
	return lines, words, chars
}
