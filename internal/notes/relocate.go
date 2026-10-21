package notes

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Relocate 串行保存当前快照、校验新文件、持久化偏好，再切换权威路径。
// persist 只能操作调用方已拍下的 prefs.Store 快照，不得回调本 Store。
// 偏好成功后即使旧文件删除失败，DataPath 也必须反映已提交的新路径。
func (s *Store) Relocate(target string, f File, persist func() error) error {
	target, err := canonicalPath(target)
	if err != nil {
		s.ReportPersist("迁址失败", err)
		return err
	}
	s.mu.Lock()
	if target == s.dataPath {
		s.mu.Unlock()
		return nil
	}
	if s.stopped || s.closed {
		s.mu.Unlock()
		return errors.New("存储已停止")
	}
	req := s.acceptLocked(cloneFile(f))
	s.pending = nil
	s.stopTimerLocked()
	s.mu.Unlock()
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	old := s.DataPath()
	if target == old {
		return nil
	}
	err = s.relocateLocked(old, target, req, persist)
	s.ReportPersist("迁址失败", err)
	return err
}

func (s *Store) relocateLocked(old, target string, req *pendingSave, persist func() error) error {
	if _, err := os.Lstat(target); !errors.Is(err, os.ErrNotExist) {
		if err == nil {
			return fmt.Errorf("目标文件已存在：%s", target)
		}
		return err
	}
	if err := s.saveRequest(req); err != nil {
		return err
	}
	s.mu.Lock()
	raw := bytes.Clone(s.savedContent)
	s.mu.Unlock()
	if err := writeExclusive(target, raw); err != nil {
		return err
	}
	cleanup := func(cause error) error {
		if err := os.Remove(target); err != nil {
			return errors.Join(cause, fmt.Errorf("清理未采用目标失败：%w", err))
		}
		return cause
	}
	check := func() error {
		resolved, err := canonicalPath(target)
		if err != nil {
			return err
		}
		if resolved != target {
			return ErrExternalChange
		}
		got, err := os.ReadFile(target)
		if err != nil {
			return err
		}
		if !bytes.Equal(got, raw) {
			return ErrExternalChange
		}
		return s.verifyDisk()
	}
	if err := check(); err != nil {
		return cleanup(err)
	}
	// 本地 Git 命令与切换互斥；网络推送不占此门闩，旧推送由取消上下文中止。
	s.gitGate.Lock()
	defer s.gitGate.Unlock()
	if err := check(); err != nil {
		return cleanup(err)
	}
	if persist != nil {
		if err := persist(); err != nil {
			return cleanup(err)
		}
	}
	s.mu.Lock()
	s.dataPath = target
	s.diskKnown, s.diskExists = true, true
	s.diskHash = sha256.Sum256(raw)
	s.invalidateGitLocked(true)
	s.armGitLocked()
	s.mu.Unlock()
	// 偏好已提交，不再回退路径；外部修改过的旧文件不得删除。
	oldRaw, err := os.ReadFile(old)
	if err != nil {
		return fmt.Errorf("已切换，读取旧文件失败：%w", err)
	}
	if !bytes.Equal(oldRaw, raw) {
		return fmt.Errorf("已切换，旧文件发生外部修改：%w", ErrExternalChange)
	}
	if err := s.stateRemove(old); err != nil {
		return fmt.Errorf("已切换，删除旧文件失败：%w", err)
	}
	return nil
}

// writeExclusive 通过同目录临时文件和排他发布拒绝覆盖竞态中新建的目标。
func writeExclusive(path string, raw []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".zakki-relocate-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err = tmp.Write(raw); err != nil {
		tmp.Close()
		return err
	}
	if err = tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	if err = os.Link(tmp.Name(), path); err != nil {
		return err
	}
	return nil
}
