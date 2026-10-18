package app

import (
	"runtime"
	"strings"
	"testing"
)

func BenchmarkUndoRetainedMemory(b *testing.B) {
	var retained int64
	b.ReportAllocs()
	for b.Loop() {
		b.StopTimer()
		a := newTestApp(b)
		a.notes.SetStopped(true)
		a.cfg.Source = true
		a.tabs[0].Text = strings.Repeat("x", 1<<20)
		runtime.GC()
		var before, after runtime.MemStats
		runtime.ReadMemStats(&before)
		b.StartTimer()
		for i := 0; i < 250; i++ {
			previous := a.liveSnapshot()
			a.tabs[0].Text += "字"
			a.rememberLiveEdit(previous, editIME)
		}
		b.StopTimer()
		runtime.GC()
		runtime.ReadMemStats(&after)
		retained += int64(after.HeapAlloc) - int64(before.HeapAlloc)
		if len(a.liveUndo) != 200 {
			b.Fatalf("撤销事务数量=%d，要 200", len(a.liveUndo))
		}
		runtime.KeepAlive(a)
		b.StartTimer()
	}
	b.ReportMetric(float64(retained)/float64(b.N), "retained-B/op")
}
