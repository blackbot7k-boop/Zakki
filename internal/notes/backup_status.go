package notes

import (
	"errors"
	"os"
	"time"
)

// LoadLastBackupTime 恢复本进程尚未初始化的备份时间；与备份写入串行，I/O 不占状态锁。
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
	dir := s.backupDirLocked()
	s.mu.Unlock()
	if stopped {
		return nil
	}
	newest, err := s.readStatusTime(dir)
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

// readStatusTime 取目录里最新一份备份文件的修改时间；目录不存在不算错误。
// 只认本应用的备份命名，用户自选文件夹里可能混有其他文件。
func (s *Store) readStatusTime(dir string) (time.Time, error) {
	entries, err := s.readStatusDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return time.Time{}, nil
	}
	if err != nil {
		return time.Time{}, err
	}
	var newest time.Time
	for _, entry := range entries {
		if entry.IsDir() || !isBackupFileName(entry.Name()) {
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
