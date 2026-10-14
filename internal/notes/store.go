package notes

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"tot-md-mygo/internal/atomicfile"
)

// Store 负责笔记数据文件的读写、防抖保存、滚动备份与写保护协调。
// 路径与时钟在构造时注入，便于测试使用临时目录与固定时钟。
type Store struct {
	DataFile  string
	BackupDir string
	Now       func() time.Time
	Migrate   MigrateOptions

	mu          sync.Mutex
	pending     *pendingSave
	stopped     bool
	saveBlocked bool
	lastBackup  time.Time
	lastSaveErr string
	saveTimer   *time.Timer
}

type pendingSave struct {
	file           File
	backupInterval time.Duration
	backupKeep     int
	after          func()
}

// NewStore 创建笔记持久化 Store。now 为 nil 时使用 time.Now。
func NewStore(dataFile, backupDir string, now func() time.Time, migrate MigrateOptions) *Store {
	if now == nil {
		now = time.Now
	}
	return &Store{
		DataFile:  dataFile,
		BackupDir: backupDir,
		Now:       now,
		Migrate:   migrate,
	}
}

// PendingPath 写保护期间新改动的暂存文件路径。
func (s *Store) PendingPath() string { return s.DataFile + ".pending" }

// CurrentDataPath 当前生效的数据文件路径（写保护期间为暂存文件）。
func (s *Store) CurrentDataPath() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.currentDataPathLocked()
}

func (s *Store) currentDataPathLocked() string {
	if s.saveBlocked {
		return s.PendingPath()
	}
	return s.DataFile
}

// SaveBlocked 是否处于写保护（损坏且无备份时新改动只写 .pending）。
func (s *Store) SaveBlocked() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.saveBlocked
}

// SetSaveBlocked 设置写保护标志。
func (s *Store) SetSaveBlocked(blocked bool) {
	s.mu.Lock()
	s.saveBlocked = blocked
	s.mu.Unlock()
}

// SetStopped 标记退出后不再接受保存（测试亦可用）。
func (s *Store) SetStopped(v bool) {
	s.mu.Lock()
	s.stopped = v
	s.mu.Unlock()
}

// ParseRaw 解析并迁移磁盘 JSON。
func (s *Store) ParseRaw(raw []byte) (File, bool, error) {
	var f File
	if err := json.Unmarshal(raw, &f); err != nil {
		return File{}, false, err
	}
	if !HasHideOnEscapeKey(raw) {
		f.Settings.HideOnEscape = true
	}
	hadTabs := len(f.Dots) > 0 || len(f.Tabs) > 0
	f, err := Migrate(f, s.Migrate)
	return f, hadTabs, err
}

// QuarantineCorruptFile 把不可用的原文件复制隔离，原件保持原样。
func (s *Store) QuarantineCorruptFile() {
	raw, err := os.ReadFile(s.DataFile)
	if err != nil {
		slog.Warn("损坏文件无法复制隔离，原件保持原样", "error", err)
		return
	}
	dst := s.DataFile + ".corrupt-" + s.Now().Format("20060102-150405")
	if err := os.WriteFile(dst, raw, 0o644); err != nil {
		slog.Warn("隔离副本写入失败，原件保持原样", "path", dst, "error", err)
		return
	}
	slog.Info("损坏文件已隔离", "path", dst)
}

// LoadNewestBackup 从备份目录新→旧找第一个可用备份。
func (s *Store) LoadNewestBackup() (File, string, bool) {
	entries, err := os.ReadDir(s.BackupDir)
	if err != nil {
		return File{}, "", false
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
			names = append(names, e.Name())
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(names)))
	for _, name := range names {
		raw, err := os.ReadFile(filepath.Join(s.BackupDir, name))
		if err != nil {
			continue
		}
		f, _, err := s.ParseRaw(raw)
		if err != nil {
			continue
		}
		return f, name, true
	}
	return File{}, "", false
}

// SaveNow 同步写入快照；直接保存会取代排队的防抖快照。
func (s *Store) SaveNow(f File) error {
	s.mu.Lock()
	stopped := s.stopped
	var err error
	if !stopped {
		s.pending = nil
		err = s.writeFileLocked(f)
	}
	s.mu.Unlock()
	if stopped {
		return nil
	}
	s.ReportPersist("保存失败", err)
	return err
}

// ScheduleDebounced 防抖保存：delay 内无新改动才落盘，必要时顺带周期备份。
func (s *Store) ScheduleDebounced(f File, delay, backupInterval time.Duration, backupKeep int, after func()) {
	s.mu.Lock()
	if s.stopped {
		s.mu.Unlock()
		return
	}
	s.pending = &pendingSave{
		file:           f,
		backupInterval: backupInterval,
		backupKeep:     backupKeep,
		after:          after,
	}
	s.mu.Unlock()
	if s.saveTimer != nil {
		s.saveTimer.Stop()
	}
	s.saveTimer = time.AfterFunc(delay, s.flushPending)
}

func (s *Store) flushPending() {
	s.mu.Lock()
	req := s.pending
	s.pending = nil
	stopped := s.stopped
	var err, bakErr error
	var after func()
	if !stopped && req != nil {
		after = req.after
		if err = s.writeFileLocked(req.file); err == nil {
			bakErr = s.maybeWritePeriodicBackupLocked(req.backupInterval, req.backupKeep)
		}
	}
	s.mu.Unlock()
	if stopped || req == nil {
		return
	}
	if err != nil {
		s.ReportPersist("保存失败", err)
		return
	}
	if bakErr != nil {
		s.ReportPersist("备份失败", bakErr)
	}
	if after != nil {
		after()
	}
}

// StopSaveTimer 取消防抖定时器（退出前调用）。
func (s *Store) StopSaveTimer() {
	if s.saveTimer != nil {
		s.saveTimer.Stop()
		s.saveTimer = nil
	}
}

// Shutdown 退出最终保存：停止接受新请求并写入最终快照。
func (s *Store) Shutdown(f File) error {
	s.StopSaveTimer()
	s.mu.Lock()
	if s.stopped {
		s.mu.Unlock()
		return nil
	}
	s.stopped = true
	s.pending = nil
	err := s.writeFileLocked(f)
	s.mu.Unlock()
	if err != nil {
		slog.Error("退出前最终保存失败", "error", err)
		return err
	}
	return nil
}

func (s *Store) writeFileLocked(f File) error {
	raw, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	target := s.currentDataPathLocked()
	return atomicfile.WriteBytes(target, raw)
}

// BackupNow 在已保存数据基础上无条件写一份备份。
func (s *Store) BackupNow(keep int) error {
	s.mu.Lock()
	stopped := s.stopped
	var err error
	if !stopped {
		err = s.writeBackupLocked(keep)
	}
	s.mu.Unlock()
	if stopped {
		return nil
	}
	s.ReportPersist("备份失败", err)
	return err
}

// MaybeWritePeriodicBackup 距上次备份超过 interval 才写一份。
func (s *Store) MaybeWritePeriodicBackup(interval time.Duration, keep int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.maybeWritePeriodicBackupLocked(interval, keep)
}

func (s *Store) maybeWritePeriodicBackupLocked(interval time.Duration, keep int) error {
	if !s.lastBackup.IsZero() && s.Now().Sub(s.lastBackup) <= interval {
		return nil
	}
	return s.writeBackupLocked(keep)
}

func (s *Store) writeBackupLocked(keep int) error {
	raw, err := os.ReadFile(s.currentDataPathLocked())
	if err != nil {
		return err
	}
	if err := os.MkdirAll(s.BackupDir, 0o755); err != nil {
		return err
	}
	name := s.uniqueBackupNameLocked()
	if err := os.WriteFile(filepath.Join(s.BackupDir, name), raw, 0o644); err != nil {
		return err
	}
	s.lastBackup = s.Now()
	if err := s.pruneBackupsLocked(keep); err != nil {
		slog.Warn("修剪旧备份失败", "error", err)
	}
	return nil
}

func (s *Store) uniqueBackupNameLocked() string {
	base := "tot-md-" + s.Now().Format("20060102-150405")
	name := base + ".json"
	for i := 2; ; i++ {
		_, err := os.Stat(filepath.Join(s.BackupDir, name))
		if errors.Is(err, os.ErrNotExist) {
			return name
		}
		if err != nil {
			return name
		}
		name = fmt.Sprintf("%s-%d.json", base, i)
	}
}

// BackupOnLaunch 启用时在数据文件存在时写一份启动备份。
func (s *Store) BackupOnLaunch(enabled bool, keep int) error {
	if !enabled {
		return nil
	}
	if _, err := os.Stat(s.CurrentDataPath()); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.writeBackupLocked(keep)
}

func (s *Store) pruneBackupsLocked(keep int) error {
	entries, err := os.ReadDir(s.BackupDir)
	if err != nil {
		return err
	}
	var files []os.DirEntry
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
			files = append(files, e)
		}
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Name() > files[j].Name() })
	var firstErr error
	for i := keep; i < len(files); i++ {
		if err := os.Remove(filepath.Join(s.BackupDir, files[i].Name())); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// RemovePending 删除写保护暂存文件（用户确认写回后）。
func (s *Store) RemovePending() error {
	return os.Remove(s.PendingPath())
}

// ReportPersist 记录持久化结果到 lastSaveErr 并打日志。
func (s *Store) ReportPersist(op string, err error) {
	s.mu.Lock()
	if err == nil {
		s.lastSaveErr = ""
		s.mu.Unlock()
		return
	}
	s.lastSaveErr = op + "：" + err.Error()
	s.mu.Unlock()
	slog.Error(op, "error", err)
}

// LastSaveError 最近一次持久化错误（空串表示没有）。
func (s *Store) LastSaveError() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastSaveErr
}

// LastBackupTime 最近备份时间。
func (s *Store) LastBackupTime() time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastBackup
}

// LoadLastBackupTime 启动时从备份目录恢复最近备份时间。
func (s *Store) LoadLastBackupTime() error {
	entries, err := os.ReadDir(s.BackupDir)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	var newest time.Time
	for _, e := range entries {
		if info, err := e.Info(); err == nil && info.ModTime().After(newest) {
			newest = info.ModTime()
		}
	}
	s.mu.Lock()
	s.lastBackup = newest
	s.mu.Unlock()
	return nil
}
