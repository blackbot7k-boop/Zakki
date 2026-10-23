package notes

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"zakki/internal/atomicfile"
)

// Store 管理唯一笔记文件；状态锁不覆盖磁盘或网络操作。
type Store struct {
	mu                                     sync.Mutex
	writeMu                                sync.Mutex
	shutdownMu                             sync.Mutex
	dataPath                               string
	now                                    func() time.Time
	migrate                                MigrateOptions
	pending                                *pendingSave
	stopped, closed, saveBlocked, invalid  bool
	saveTimer                              *time.Timer
	timerGen                               uint64
	saveRunning                            bool
	saveRevision, saveCompleted, saveSaved uint64
	saveResult                             error
	lastSaveErr                            string
	lastSaveTime                           time.Time
	diskKnown, diskExists, syncPending     bool
	diskHash                               [32]byte
	savedContent                           []byte
	stateWrite                             func(string, []byte) error
	stateRemove                            func(string) error

	// RunGit 仅供构造后、启动后台任务前注入；nil 使用系统 CLI。
	RunGit                           GitRunner
	gitGate                          sync.Mutex
	gitConfig                        GitConfig
	gitStatus                        GitState
	gitGen                           uint64
	gitTimer                         *time.Timer
	gitRunning, gitQueued            bool
	gitDone                          chan struct{}
	gitCancel                        func()
	gitAfter                         func()
	gitCommitAttempt, gitPushAttempt time.Time
}

// NewStore 创建单文件存储；缺省时钟使用 time.Now。
func NewStore(dataFile string, now func() time.Time, migrate MigrateOptions) *Store {
	if now == nil {
		now = time.Now
	}
	path, err := canonicalPath(dataFile)
	if err != nil {
		path, _ = filepath.Abs(dataFile)
	}
	return &Store{dataPath: path, now: now, migrate: migrate, stateWrite: atomicfile.WriteBytes, stateRemove: os.Remove}
}

// SetClock 替换状态时间戳时钟；测试应在启动后台任务前注入，nil 恢复真实时钟。
func (s *Store) SetClock(now func() time.Time) {
	if now == nil {
		now = time.Now
	}
	s.mu.Lock()
	s.now = now
	s.mu.Unlock()
}

// DataPath 返回当前唯一权威文件的路径。
func (s *Store) DataPath() string { s.mu.Lock(); defer s.mu.Unlock(); return s.dataPath }

// ParseRaw 保留磁盘格式迁移；不修改文件及写保护状态。
func (s *Store) ParseRaw(raw []byte) (File, bool, error) {
	var f File
	if err := json.Unmarshal(raw, &f); err != nil {
		return File{}, false, err
	}
	if !HasHideOnEscapeKey(raw) {
		f.Settings.HideOnEscape = true
	}
	hadTabs := len(f.Dots) > 0 || len(f.Tabs) > 0
	f, err := Migrate(f, s.migrate)
	return f, hadTabs, err
}

// Load 读取并记住磁盘指纹；不存在的全新文件允许首次保存。
// hadTabs 报告原始 JSON 是否带标签（dots 或 tabs），供空文件欢迎文案判断。
func (s *Store) Load() (f File, exists, hadTabs bool, err error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	path := s.DataPath()
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		s.mu.Lock()
		if s.diskExists {
			err = ErrExternalChange
		} else if _, e := os.Lstat(path); errors.Is(e, os.ErrNotExist) {
			s.diskKnown = true
			err = nil
		}
		s.mu.Unlock()
		if err != nil {
			s.ReportPersist("读取失败", err)
		}
		return File{}, false, false, err
	}
	if err == nil {
		f, hadTabs, err = s.ParseRaw(raw)
	}
	if err != nil {
		s.mu.Lock()
		s.invalid = true
		s.mu.Unlock()
		s.ReportPersist("读取失败", err)
		return File{}, false, false, err
	}
	content, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return File{}, false, false, err
	}
	s.mu.Lock()
	s.diskKnown, s.diskExists, s.invalid, s.syncPending = true, true, false, false
	s.diskHash, s.savedContent = sha256.Sum256(raw), content
	s.lastSaveErr = ""
	s.mu.Unlock()
	return f, true, hadTabs, nil
}

var (
	// ErrExternalChange 表示文件已被外部改写、移动或删除。
	ErrExternalChange = errors.New("笔记文件已被外部修改或移除，请重新读取后再保存")
	// ErrSaveBlocked 表示原文件必须保持不变。
	ErrSaveBlocked = errors.New("笔记文件处于写保护状态")
)

// verifyDisk 在写锁内核对字节指纹；相同内容的保存也不能绕过核对。
func (s *Store) verifyDisk() error {
	s.mu.Lock()
	path, known, exists, hash := s.dataPath, s.diskKnown, s.diskExists, s.diskHash
	blocked := s.saveBlocked || s.invalid
	s.mu.Unlock()
	if blocked {
		return ErrSaveBlocked
	}
	resolved, err := canonicalPath(path)
	if err != nil {
		return err
	}
	if resolved != path {
		return ErrExternalChange
	}
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		if exists {
			return ErrExternalChange
		}
		if _, e := os.Lstat(path); !errors.Is(e, os.ErrNotExist) {
			return ErrExternalChange
		}
		s.mu.Lock()
		s.diskKnown = true
		s.mu.Unlock()
		return nil
	}
	if err != nil {
		return err
	}
	if !known || !exists || sha256.Sum256(raw) != hash {
		return ErrExternalChange
	}
	return nil
}

// canonicalPath 解析已存在的文件及父目录，缺失尾部保持为绝对路径。
func canonicalPath(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	probe := abs
	var tail []string
	for {
		resolved, e := filepath.EvalSymlinks(probe)
		if e == nil {
			for i := len(tail) - 1; i >= 0; i-- {
				resolved = filepath.Join(resolved, tail[i])
			}
			return resolved, nil
		}
		if !errors.Is(e, os.ErrNotExist) {
			return "", e
		}
		if _, le := os.Lstat(probe); le == nil {
			return "", fmt.Errorf("无法解析符号链接 %s: %w", probe, e)
		}
		parent := filepath.Dir(probe)
		if parent == probe {
			return "", e
		}
		tail = append(tail, filepath.Base(probe))
		probe = parent
	}
}

// SaveBlocked 报告用户暂停或载入失败造成的写保护。
func (s *Store) SaveBlocked() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.saveBlocked || s.invalid
}

// SetSaveBlocked 不能解除损坏或高版本文件的保护，必须成功重新载入。
func (s *Store) SetSaveBlocked(v bool) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	s.gitGate.Lock()
	defer s.gitGate.Unlock()
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.saveBlocked != v {
		s.saveBlocked = v
		s.invalidateGitLocked(false)
		s.armGitLocked()
	}
}

// SetStopped 控制是否接受新快照，退出失败时可恢复。
func (s *Store) SetStopped(v bool) { s.mu.Lock(); s.stopped = v; s.mu.Unlock() }

// ReportPersist 如实发布最近持久化结果。
func (s *Store) ReportPersist(op string, err error) {
	s.mu.Lock()
	if err == nil {
		s.lastSaveErr = ""
	} else {
		s.lastSaveErr = op + "：" + err.Error()
	}
	s.mu.Unlock()
	if err != nil {
		slog.Error(op, "error", err)
	}
}

// LastSaveError 返回最近保存错误。
func (s *Store) LastSaveError() string { s.mu.Lock(); defer s.mu.Unlock(); return s.lastSaveErr }

// LastSaveTime 返回最近实际写入成功时间。
func (s *Store) LastSaveTime() time.Time { s.mu.Lock(); defer s.mu.Unlock(); return s.lastSaveTime }

// Unsaved 报告最新接受的快照是否尚未成功保存。
func (s *Store) Unsaved() bool { s.mu.Lock(); defer s.mu.Unlock(); return s.saveRevision > s.saveSaved }
