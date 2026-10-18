package notes

import (
	"encoding/json"
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"

	"zakki/internal/atomicfile"
)

func waitSaveState(t *testing.T, s *Store, check func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		s.mu.Lock()
		ok := check()
		s.mu.Unlock()
		if ok {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("等待保存状态超时")
}

func TestSlowSaveCoalescesLatestReadySnapshot(t *testing.T) {
	s := newTestStore(t, nil)
	started, release := make(chan struct{}), make(chan struct{})
	var released sync.Once
	resume := func() { released.Do(func() { close(release) }) }
	defer resume()
	var once sync.Once
	var writes []string
	s.stateWrite = func(path string, raw []byte) error {
		once.Do(func() { close(started); <-release })
		var f File
		if err := json.Unmarshal(raw, &f); err != nil {
			return err
		}
		writes = append(writes, f.Tabs[0].Text)
		return atomicfile.WriteBytes(path, raw)
	}
	first := File{Version: CurrentDataVersion, Tabs: defaultTabs("first")}
	s.ScheduleDebounced(first, 0, time.Hour, 20, nil)
	<-started
	last := File{Version: CurrentDataVersion, Tabs: defaultTabs("last")}
	for i := 0; i < 100; i++ {
		s.ScheduleDebounced(first, 0, time.Hour, 20, nil)
	}
	done := make(chan struct{})
	s.ScheduleDebounced(last, 0, time.Hour, 20, func() { close(done) })
	waitSaveState(t, s, func() bool { return s.pending != nil && s.pending.ready })
	resume()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("慢写恢复后没有保存最新快照")
	}
	s.StopSaveTimer()
	if !reflect.DeepEqual(writes, []string{"first", "last"}) {
		t.Fatalf("慢盘写入序列=%q，要 first,last", writes)
	}
	if got := readFile(t, s.DataFile).Tabs[0].Text; got != "last" {
		t.Fatalf("最终内容=%q，要 last", got)
	}
}

func TestShutdownWaitsForSlowSaveAndRejectsLaterRequests(t *testing.T) {
	s := newTestStore(t, nil)
	started, release := pauseStateWriter(s)
	defer release()
	first := File{Version: CurrentDataVersion, Tabs: defaultTabs("in-flight")}
	s.ScheduleDebounced(first, 0, time.Hour, 20, nil)
	<-started
	final := File{Version: CurrentDataVersion, Tabs: defaultTabs("final")}
	done := make(chan error, 1)
	go func() { done <- s.Shutdown(final) }()
	waitSaveState(t, s, func() bool { return s.stopped })
	s.ScheduleDebounced(first, 0, time.Hour, 20, nil)
	select {
	case err := <-done:
		t.Fatalf("在途写入未完成就退出了: %v", err)
	default:
	}
	release()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if err := s.SaveNow(first); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, s.DataFile).Tabs[0].Text; got != "final" {
		t.Fatalf("最终保存被旧快照覆盖: %q", got)
	}
}

func TestDirectSavePreservesLaterDebouncedRequest(t *testing.T) {
	s := newTestStore(t, nil)
	started, release := pauseStateWriter(s)
	defer release()
	first := File{Version: CurrentDataVersion, Tabs: defaultTabs("in-flight")}
	s.ScheduleDebounced(first, 0, time.Hour, 20, nil)
	<-started
	direct := File{Version: CurrentDataVersion, Tabs: defaultTabs("older-sync")}
	done := make(chan error, 1)
	go func() { done <- s.SaveNow(direct) }()
	waitSaveState(t, s, func() bool { return s.saveRevision == 2 })
	latest := File{Version: CurrentDataVersion, Tabs: defaultTabs("latest")}
	saved := make(chan struct{})
	s.ScheduleDebounced(latest, 0, time.Hour, 20, func() { close(saved) })
	waitSaveState(t, s, func() bool { return s.pending != nil && s.pending.ready })
	release()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	select {
	case <-saved:
	case <-time.After(2 * time.Second):
		t.Fatal("直接保存丢弃了后来接受的自动保存")
	}
	if got := readFile(t, s.DataFile).Tabs[0].Text; got != "latest" {
		t.Fatalf("较旧直接保存覆盖新请求: %q", got)
	}
}

func TestStaleDirectSaveReturnsLatestFailure(t *testing.T) {
	s := newTestStore(t, nil)
	// 控制写锁，模拟新版本先完成但失败，旧同步请求仍在排队。
	s.writeMu.Lock()
	done := make(chan error, 1)
	go func() { done <- s.SaveNow(File{Tabs: defaultTabs("old")}) }()
	waitSaveState(t, s, func() bool { return s.saveRevision == 1 })
	want := errors.New("newer save failed")
	s.mu.Lock()
	s.saveRevision++
	revision := s.saveRevision
	s.mu.Unlock()
	s.completeSave(revision, want)
	s.writeMu.Unlock()
	if err := <-done; !errors.Is(err, want) {
		t.Fatalf("旧请求掩盖了新版本保存失败: %v", err)
	}
}

func TestStaleSaveTimerDoesNotFlushNewerSnapshotEarly(t *testing.T) {
	s := newTestStore(t, nil)
	first := File{Version: CurrentDataVersion, Tabs: defaultTabs("stale")}
	s.ScheduleDebounced(first, time.Hour, time.Hour, 20, nil)
	s.mu.Lock()
	stale := s.pending
	s.mu.Unlock()
	newer := File{Version: CurrentDataVersion, Tabs: defaultTabs("newer")}
	s.ScheduleDebounced(newer, time.Hour, time.Hour, 20, nil)
	s.flushPending(stale)
	s.mu.Lock()
	ready := s.pending.ready
	s.mu.Unlock()
	s.StopSaveTimer()
	if ready {
		t.Fatal("旧定时回调提前触发了新快照的防抖保存")
	}
}
