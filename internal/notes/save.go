package notes

import (
	"encoding/json"
	"log/slog"
	"time"

	"zakki/internal/atomicfile"
)

// SaveNow 同步写入快照；直接保存会取代排队的防抖快照。
func (s *Store) SaveNow(f File) error {
	s.mu.Lock()
	if s.stopped {
		s.mu.Unlock()
		return nil
	}
	s.saveRevision++
	revision := s.saveRevision
	s.pending = nil
	s.mu.Unlock()

	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	s.mu.Lock()
	if s.stopped {
		s.mu.Unlock()
		return nil
	}
	// 等写锁时可以接受新请求；不能清掉后来排队的快照或覆盖先完成的新版本。
	if revision <= s.saveCompleted {
		err := s.saveResult
		s.mu.Unlock()
		return err
	}
	target := s.currentDataPathLocked()
	s.mu.Unlock()
	err := s.writeFile(target, f)
	s.completeSave(revision, err)
	s.ReportPersist("保存失败", err)
	return err
}

// ScheduleDebounced 防抖保存：delay 内无新改动才落盘，必要时顺带周期备份。
func (s *Store) ScheduleDebounced(f File, delay, backupInterval time.Duration, backupKeep int, after func()) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stopped {
		return
	}
	s.saveRevision++
	req := &pendingSave{revision: s.saveRevision, file: f, backupInterval: backupInterval, backupKeep: backupKeep, after: after}
	s.pending = req
	if s.saveTimer != nil {
		s.saveTimer.Stop()
	}
	s.saveTimer = time.AfterFunc(delay, func() { s.flushPending(req) })
}

func (s *Store) flushPending(req *pendingSave) {
	s.mu.Lock()
	if s.stopped || s.pending != req {
		s.mu.Unlock()
		return
	}
	req.ready = true
	if s.saveRunning {
		s.mu.Unlock()
		return
	}
	s.saveRunning = true
	s.mu.Unlock()

	// 慢盘期间只有一个保存 worker；到期的新快照由它补跑，不积累等写锁的 goroutine。
	for {
		s.writeMu.Lock()
		s.mu.Lock()
		req = s.pending
		if s.stopped || req == nil || !req.ready {
			s.saveRunning = false
			s.mu.Unlock()
			s.writeMu.Unlock()
			return
		}
		s.pending = nil
		target := s.currentDataPathLocked()
		s.mu.Unlock()
		err := s.writeFile(target, req.file)
		s.completeSave(req.revision, err)
		if err != nil {
			s.ReportPersist("保存失败", err)
		}
		s.writeMu.Unlock()
		if err != nil {
			continue
		}
		if err := s.maybeWritePeriodicBackup(req.backupInterval, req.backupKeep); err != nil {
			s.ReportPersist("备份失败", err)
		}
		if req.after != nil {
			req.after()
		}
	}
}

// StopSaveTimer 取消防抖定时器（退出前调用）。
func (s *Store) StopSaveTimer() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.saveTimer != nil {
		s.saveTimer.Stop()
		s.saveTimer = nil
	}
}

// Shutdown 退出最终保存：拒绝新请求，等待在途写入，再写最终快照。
func (s *Store) Shutdown(f File) error {
	s.shutdownMu.Lock()
	defer s.shutdownMu.Unlock()
	s.mu.Lock()
	if s.stopped {
		s.mu.Unlock()
		return nil
	}
	if s.saveTimer != nil {
		s.saveTimer.Stop()
		s.saveTimer = nil
	}
	s.stopped, s.pending = true, nil
	s.saveRevision++
	revision := s.saveRevision
	s.mu.Unlock()

	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	s.mu.Lock()
	target := s.currentDataPathLocked()
	s.mu.Unlock()
	err := s.writeFile(target, f)
	s.completeSave(revision, err)
	if err != nil {
		slog.Error("退出前最终保存失败", "error", err)
	}
	return err
}

// completeSave 只在持有写锁时发布结果；失败也淘汰更旧请求，避免旧写入掩盖新失败。
func (s *Store) completeSave(revision uint64, err error) {
	s.mu.Lock()
	s.saveCompleted, s.saveResult = revision, err
	s.mu.Unlock()
}

// writeFile 由 writeMu 串行化，调用时不持状态锁，慢磁盘不影响 UI 查询和排队。
func (s *Store) writeFile(target string, f File) error {
	raw, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	write := s.stateWrite
	if write == nil {
		write = atomicfile.WriteBytes
	}
	return write(target, raw)
}
