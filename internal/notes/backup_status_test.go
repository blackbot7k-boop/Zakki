package notes

import (
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func blockStatusRead(t *testing.T, s *Store, dir string) (<-chan struct{}, func()) {
	t.Helper()
	started, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	t.Cleanup(unblock)
	s.statusReadDir = func(path string) ([]os.DirEntry, error) {
		entries, err := os.ReadDir(path)
		if path != dir {
			return entries, err
		}
		// 冻结这次扫描读到的元数据，让备份完成与扫描发布的交错可重复。
		for i, entry := range entries {
			if info, infoErr := entry.Info(); infoErr == nil {
				entries[i] = fs.FileInfoToDirEntry(info)
			}
		}
		close(started)
		<-release
		return entries, err
	}
	return started, unblock
}

func awaitStatusRead[T any](t *testing.T, ch <-chan T) T {
	t.Helper()
	select {
	case result := <-ch:
		return result
	case <-time.After(2 * time.Second):
		t.Fatal("等待备份状态操作超时")
		var zero T
		return zero
	}
}

func writeStatusBackup(t *testing.T, dir string, at time.Time) {
	t.Helper()
	path := filepath.Join(dir, "zakki-20240601-110000.json")
	writeRaw(t, path, `{}`)
	if err := os.Chtimes(path, at, at); err != nil {
		t.Fatal(err)
	}
}

func TestPeriodicBackupInitializesHistoricalTime(t *testing.T) {
	s := newTestStore(t, nil)
	writeRaw(t, s.DataFile, `{}`)
	at := s.Now().Add(-time.Minute)
	writeStatusBackup(t, s.BackupDir(), at)
	if err := s.MaybeWritePeriodicBackup(5*time.Minute, 20); err != nil {
		t.Fatal(err)
	}
	if names := backupNames(t, s.BackupDir()); len(names) != 1 {
		t.Fatalf("状态恢复完成前创建了额外周期备份: %v", names)
	}
	if got := s.LastBackupTime(); !got.Equal(at) {
		t.Fatalf("历史节流时间 = %v, want %v", got, at)
	}
}

func TestLoadLastBackupTimeAfterNewBackup(t *testing.T) {
	s := newTestStore(t, nil)
	writeRaw(t, s.DataFile, `{}`)
	writeStatusBackup(t, s.BackupDir(), s.Now().Add(-time.Hour))
	if err := s.BackupOnLaunch(true, 20); err != nil {
		t.Fatal(err)
	}
	if err := s.LoadLastBackupTime(); err != nil {
		t.Fatal(err)
	}
	if got := s.LastBackupTime(); !got.Equal(s.Now()) {
		t.Fatalf("扫描覆盖刚完成的启动备份: got %v, want %v", got, s.Now())
	}
}

func TestLocalStatusScanDoesNotBlockSaveOrShutdown(t *testing.T) {
	s := newTestStore(t, nil)
	writeStatusBackup(t, s.BackupDir(), s.Now().Add(-time.Minute))
	started, unblock := blockStatusRead(t, s, s.BackupDir())
	done := make(chan error, 1)
	go func() { done <- s.LoadLastBackupTime() }()
	awaitStatusRead(t, started)
	final := File{Version: CurrentDataVersion, Tabs: []Tab{{ID: 1, Text: "退出最终快照"}}}
	s.ScheduleDebounced(final, time.Hour, time.Minute, 20, nil)
	quit := make(chan error, 1)
	go func() { quit <- s.Shutdown(final) }()
	if err := awaitStatusRead(t, quit); err != nil {
		t.Fatal(err)
	}
	unblock()
	if err := awaitStatusRead(t, done); err != nil {
		t.Fatal(err)
	}
	if got := s.LastBackupTime(); !got.IsZero() {
		t.Fatalf("退出后的扫描仍发布了状态: %v", got)
	}
	if got := readFile(t, s.DataFile).Tabs[0].Text; got != final.Tabs[0].Text {
		t.Fatalf("最终快照 = %q", got)
	}
}
