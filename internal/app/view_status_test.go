package app

import (
	"strings"
	"testing"
	"time"
)

func TestDocStatsDebouncesLongDocs(t *testing.T) {
	a := newTestApp(t)
	t.Cleanup(a.shutdown)
	now := time.Date(2026, 1, 1, 9, 0, 0, 0, time.UTC)
	a.setTestNow(func() time.Time { return now })

	long := strings.Repeat("词 ", statsDebounceBytes)
	a.activeTab().Text = long
	if _, w, _ := a.docStats(long); w != statsDebounceBytes {
		t.Fatalf("首次统计 = %d", w)
	}
	typed := long + "新"
	if _, w, _ := a.docStats(typed); w != statsDebounceBytes {
		t.Fatalf("节流期内应沿用上次结果，得到 %d", w)
	}
	now = now.Add(statsDebounce)
	if _, w, _ := a.docStats(typed); w != statsDebounceBytes+1 {
		t.Fatalf("节流期过后应重算，得到 %d", w)
	}

	// 短文档每次改动都即时重算。
	now = now.Add(time.Millisecond)
	if _, w, _ := a.docStats("甲 乙"); w != 2 {
		t.Fatalf("短文档应即时重算，得到 %d", w)
	}

	// 切换标签总是立即重算，不沿用上一个标签的数字。
	a.activeTab().Text = long
	a.docStats(long)
	a.switchTab(1)
	other := strings.Repeat("字 ", statsDebounceBytes+5)
	if _, w, _ := a.docStats(other); w != statsDebounceBytes+5 {
		t.Fatalf("切换标签后应立即重算，得到 %d", w)
	}
}
