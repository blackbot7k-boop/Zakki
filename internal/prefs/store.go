package prefs

import (
	"log/slog"
	"sync"
	"time"

	"zakki/internal/atomicfile"
)

// IssueKind 偏好文件异常状态（影响是否允许写盘）。
type IssueKind int

const (
	IssueNone IssueKind = iota
	IssuePrefsBroken
)

// Issue 描述偏好载入/迁移异常，供设置页展示与写盘门禁。
type Issue struct {
	Kind    IssueKind
	Message string
}

// Snapshot 交给后台写盘的不可变偏好快照。
type Snapshot struct {
	Rev   int
	Prefs Preferences
}

// Store 串行化 preferences.json 写入，修订号防止旧快照覆盖新值。
type Store struct {
	Path     string
	Debounce time.Duration

	mu          sync.Mutex
	writeMu     sync.Mutex
	rev         int
	writtenRev  int
	lastErrRev  int
	lastSaveErr string
	stopped     bool
	saveTimer   *time.Timer
	writeJSON   func(string, any) error
	issue       Issue
}

// NewStore 创建偏好 Store。debounce 为 0 时使用 500ms。
func NewStore(path string, debounce time.Duration) *Store {
	if debounce <= 0 {
		debounce = 500 * time.Millisecond
	}
	return &Store{Path: path, Debounce: debounce, writeJSON: atomicfile.WriteJSON}
}

// SetIssue 设置异常状态（主线程）。
func (s *Store) SetIssue(issue Issue) {
	s.mu.Lock()
	s.issue = issue
	s.mu.Unlock()
}

// Issue 当前异常状态。
func (s *Store) Issue() Issue {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.issue
}

// TakeSnapshot 取最新快照；待恢复状态返回 nil（不写盘）。
func (s *Store) TakeSnapshot(p Preferences) *Snapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.issue.Kind != IssueNone {
		return nil
	}
	s.rev++
	return &Snapshot{Rev: s.rev, Prefs: p.Clone()}
}

// SaveNow 同步写盘。
func (s *Store) SaveNow(p Preferences) error {
	snap := s.TakeSnapshot(p)
	if snap == nil {
		return nil
	}
	return s.WriteSnapshot(snap)
}

// ScheduleDebounced 防抖写盘（快照在主线程生成）。
func (s *Store) ScheduleDebounced(snap *Snapshot) {
	if snap == nil {
		return
	}
	s.mu.Lock()
	stopped := s.stopped
	s.mu.Unlock()
	if stopped {
		return
	}
	if s.saveTimer != nil {
		s.saveTimer.Stop()
	}
	s.saveTimer = time.AfterFunc(s.Debounce, func() { s.flush(snap) })
}

func (s *Store) flush(snap *Snapshot) {
	s.mu.Lock()
	if s.stopped {
		s.mu.Unlock()
		return
	}
	if s.writtenRev >= snap.Rev {
		s.mu.Unlock()
		return
	}
	s.mu.Unlock()
	s.WriteSnapshot(snap)
}

// WriteSnapshot 串行化写盘：同一时刻只有一个写入者，原子替换。
func (s *Store) WriteSnapshot(snap *Snapshot) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	s.mu.Lock()
	if s.stopped || s.writtenRev >= snap.Rev || s.issue.Kind != IssueNone {
		s.mu.Unlock()
		return nil
	}
	s.mu.Unlock()

	err := s.writeJSON(s.Path, snap.Prefs)
	committed := err == nil || atomicfile.IsCommitted(err)

	s.mu.Lock()
	if committed && snap.Rev > s.writtenRev {
		s.writtenRev = snap.Rev
	}
	if err == nil {
		s.lastSaveErr = ""
	} else if snap.Rev > s.lastErrRev {
		s.lastErrRev = snap.Rev
		s.lastSaveErr = "偏好保存失败：" + err.Error()
	}
	s.mu.Unlock()
	if err != nil {
		slog.Error("偏好保存失败", "error", err)
	}
	return err
}

// WriteInitial 首次迁移写盘（不经修订号门禁）。
func (s *Store) WriteInitial(p Preferences) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	err := s.writeJSON(s.Path, p)
	s.mu.Lock()
	if err == nil {
		s.lastSaveErr = ""
	} else {
		s.lastSaveErr = "偏好保存失败：" + err.Error()
	}
	s.mu.Unlock()
	return err
}

// SetSaveError 记录最近一次偏好保存错误（设置页「未保存」）。
func (s *Store) SetSaveError(err error) {
	s.mu.Lock()
	s.lastSaveErr = "偏好保存失败：" + err.Error()
	s.mu.Unlock()
}

// LastSaveError 最近一次偏好保存错误（空串表示已保存）。
func (s *Store) LastSaveError() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastSaveErr
}

// BumpRev 用户确认恢复后递增修订号（下一次保存使用新 rev）。
func (s *Store) BumpRev() {
	s.mu.Lock()
	s.rev++
	s.mu.Unlock()
}

// StopSaveTimer 取消防抖定时器。
func (s *Store) StopSaveTimer() {
	if s.saveTimer != nil {
		s.saveTimer.Stop()
		s.saveTimer = nil
	}
}

// Shutdown 退出最终保存；待恢复状态绝不覆盖原文件。
func (s *Store) Shutdown(p Preferences) error {
	s.StopSaveTimer()
	if s.Issue().Kind != IssueNone {
		s.mu.Lock()
		s.stopped = true
		s.mu.Unlock()
		return nil
	}
	s.mu.Lock()
	snap := &Snapshot{Rev: s.rev + 1, Prefs: p.Clone()}
	s.mu.Unlock()
	if err := s.WriteSnapshot(snap); err != nil {
		return err
	}
	s.mu.Lock()
	s.stopped = true
	s.mu.Unlock()
	return nil
}

// ResetRevsForTest 测试重置修订号（仅测试）。
func (s *Store) ResetRevsForTest() {
	s.mu.Lock()
	s.rev = 0
	s.writtenRev = 0
	s.mu.Unlock()
}
