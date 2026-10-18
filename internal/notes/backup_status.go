package notes

import (
	"errors"
	"os"
	"time"
)

// LoadLastBackupTime 恢复本进程尚未初始化的备份时间；与本地备份串行，I/O 不占状态锁。
func (s *Store) LoadLastBackupTime() error {
	s.backupMu.Lock()
	defer s.backupMu.Unlock()
	return s.loadLastBackupTimeLocked()
}

func (s *Store) loadLastBackupTimeLocked() error {
	if s.backupLoaded {
		return nil
	}
	s.mu.Lock()
	stopped := s.stopped
	s.mu.Unlock()
	if stopped {
		return nil
	}
	newest, err := s.readStatusTime(s.BackupDir, false)
	if err != nil {
		return err
	}
	s.mu.Lock()
	if !s.stopped {
		s.lastBackup = newest
		s.backupLoaded = true
	}
	s.mu.Unlock()
	return nil
}

// LoadMirrorTime 恢复镜像时间；配置切换、新镜像完成或退出后丢弃过时的扫描。
func (s *Store) LoadMirrorTime() error {
	s.mu.Lock()
	dir, version, revision, stopped := s.mirrorDir, s.mirrorVersion, s.mirrorRevision, s.stopped
	s.mu.Unlock()
	if stopped || dir == "" {
		return nil
	}
	newest, err := s.readStatusTime(dir, true)
	if err != nil {
		return err
	}
	s.mu.Lock()
	if !s.stopped && dir == s.mirrorDir && version == s.mirrorVersion && revision == s.mirrorRevision {
		s.lastMirror = newest
		s.mirrorRevision++
	}
	s.mu.Unlock()
	return nil
}

func (s *Store) readStatusTime(dir string, mirror bool) (time.Time, error) {
	entries, err := s.readStatusDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return time.Time{}, nil
	}
	if err != nil {
		return time.Time{}, err
	}
	var newest time.Time
	for _, entry := range entries {
		if mirror && (entry.IsDir() || !isBackupFileName(entry.Name())) {
			continue
		}
		if info, err := entry.Info(); err == nil && info.ModTime().After(newest) {
			newest = info.ModTime()
		}
	}
	return newest, nil
}

func (s *Store) readStatusDir(dir string) ([]os.DirEntry, error) {
	read := s.statusReadDir
	if read == nil {
		read = os.ReadDir
	}
	return read(dir)
}
