package notes

import (
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"zakki/internal/atomicfile"
)

func pauseStateWriter(s *Store) (<-chan struct{}, func()) {
	started, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	s.stateWrite = func(path string, raw []byte) error {
		once.Do(func() {
			close(started)
			<-release
		})
		return atomicfile.WriteBytes(path, raw)
	}
	var resume sync.Once
	return started, func() { resume.Do(func() { close(release) }) }
}

func TestSlowSaveLeavesStateResponsive(t *testing.T) {
	for _, operation := range []string{"status", "schedule"} {
		t.Run(operation, func(t *testing.T) {
			s := newTestStore(t, nil)
			started, release := pauseStateWriter(s)
			saved := make(chan struct{})
			file := File{Version: CurrentDataVersion, Tabs: defaultTabs("待保存内容")}
			s.ScheduleSave(file, 0, func() { close(saved) })
			<-started
			defer func() {
				release()
				<-saved
				s.StopSaveTimer()
			}()
			done := make(chan struct{})
			go func() {
				if operation == "status" {
					s.LastSaveError()
					s.LastSaveTime()
				} else {
					s.ScheduleSave(file, time.Hour, nil)
				}
				close(done)
			}()
			select {
			case <-done:
			case <-time.After(200 * time.Millisecond):
				release()
				<-done
				t.Error("后台写盘阻塞了主线程使用的状态接口")
			}
		})
	}
}

func BenchmarkSaveContention(b *testing.B) {
	for _, operation := range []string{"status", "schedule"} {
		b.Run(operation, func(b *testing.B) {
			s := newTestStore(b, nil)
			file := File{Version: CurrentDataVersion, Tabs: defaultTabs("性能测试内容")}
			b.ReportAllocs()
			for b.Loop() {
				b.StopTimer()
				started, release := pauseStateWriter(s)
				saved := make(chan struct{})
				s.ScheduleSave(file, 0, func() { close(saved) })
				<-started
				timer := time.AfterFunc(150*time.Millisecond, release)
				b.StartTimer()
				if operation == "status" {
					s.LastSaveError()
					s.LastSaveTime()
				} else {
					s.ScheduleSave(file, time.Hour, nil)
				}
				b.StopTimer()
				<-saved
				timer.Stop()
				s.StopSaveTimer()
				b.StartTimer()
			}
			b.ReportMetric(150, "injected-ms")
		})
	}
}

func BenchmarkSaveSnapshot(b *testing.B) {
	for _, size := range []int{1 << 20, 10 << 20} {
		b.Run(fmt.Sprintf("%dMiB", size>>20), func(b *testing.B) {
			s := newTestStore(b, nil)
			file := File{Version: CurrentDataVersion, Tabs: defaultTabs(strings.Repeat("x", size))}
			b.SetBytes(int64(size))
			b.ReportAllocs()
			for b.Loop() {
				if err := s.SaveNow(file); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
