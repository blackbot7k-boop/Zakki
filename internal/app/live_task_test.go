package app

import (
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"zakki/locales"
)

func TestRecurringTaskToggleAdvanceUndoEnd(t *testing.T) {
	for _, tc := range []struct{ raw, want string }{
		{"- [ ] 月度 @2024-06-01 09:00 ~1h ^1mo", "- [ ] 月度 @2024-07-01 09:00 ~1h ^1mo"},
		{"- [ ] 最后 @2024-06-01 09:00 ~1h ^1mo<2024-06-01", "- [x] 最后 @2024-06-01 09:00 ~1h ^1mo<2024-06-01"},
		{"- [x] 重开 @2024-06-01 09:00 ~1h ^1mo", "- [ ] 重开 @2024-06-01 09:00 ~1h ^1mo"},
		{"- [ ] 普通 @2024-06-01 09:00 ~1h", "- [x] 普通 @2024-06-01 09:00 ~1h"},
		{"正文 @2024-06-01 09:00 ^1mo", "正文 @2024-06-01 09:00 ^1mo"},
	} {
		t.Run(tc.raw, func(t *testing.T) {
			a := newTestApp(t)
			t.Cleanup(a.shutdown)
			a.tabs[0].Text = tc.raw
			a.toggleTask(a.mdBlocks()[0])
			if got := a.tabs[0].Text; got != tc.want {
				t.Fatalf("got %q want %q", got, tc.want)
			}
			if tc.raw == tc.want {
				return
			}
			a.undoLive(false)
			if a.tabs[0].Text != tc.raw {
				t.Fatalf("undo=%q", a.tabs[0].Text)
			}
			a.undoLive(true)
			if a.tabs[0].Text != tc.want {
				t.Fatalf("redo=%q", a.tabs[0].Text)
			}
		})
	}
}

func TestRecurringTaskCheckboxBothStyles(t *testing.T) {
	for _, sketch := range []bool{false, true} {
		a := newTestApp(t)
		t.Cleanup(a.shutdown)
		a.now = func() time.Time { return time.Date(2024, 6, 10, 12, 0, 0, 0, time.Local) }
		a.tabs[0].Text = "- [ ] 周一 @2024-06-10 09:00 ~1h ^w@1-5 尾部\n续行"
		blk := a.mdBlocks()[0]
		label := locales.EditorTaskDone(a.previewProjection(blk).text)
		tt := ui.NewTester(func(c *ui.Context) {
			md := a.resolveStyle(-1, false, true)
			md.sketch = sketch
			a.taskCheckbox(c, blk, md)
		}, 440, 600)
		if err := tt.Click(label); err != nil {
			t.Fatal(err)
		}
		if got := a.tabs[0].Text; !strings.Contains(got, "@2024-06-11 09:00 ~1h ^w@1-5 尾部") || !strings.HasSuffix(got, "尾部\n续行") {
			t.Fatalf("sketch=%v got=%q", sketch, got)
		}
		a.undoLive(false)
		if a.tabs[0].Text != blk.raw+"\n续行" {
			t.Fatalf("undo=%q", a.tabs[0].Text)
		}
	}
}
