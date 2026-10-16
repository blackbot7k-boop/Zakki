package notes

import (
	"log/slog"
	"os"
	"path/filepath"
)

// enqueueMirrorJob 保证同时只有一个镜像 worker；忙碌时只保留最新快照。
func (s *Store) enqueueMirrorJob(job mirrorJob) {
	s.mu.Lock()
	if s.mirrorRunning {
		copy := job
		s.mirrorPending = &copy
		s.mu.Unlock()
		return
	}
	s.mirrorRunning = true
	s.mu.Unlock()
	go s.runMirrorJobs(job)
}

func (s *Store) runMirrorJobs(job mirrorJob) {
	for {
		s.runMirrorJob(job)

		s.mu.Lock()
		if s.mirrorPending == nil {
			s.mirrorRunning = false
			s.mu.Unlock()
			return
		}
		job = *s.mirrorPending
		s.mirrorPending = nil
		s.mu.Unlock()
	}
}

// runMirrorJob 串行执行外部镜像与 Git，并仅在短锁区发布仍有效的结果。
func (s *Store) runMirrorJob(job mirrorJob) {
	s.mirrorMu.Lock()
	s.mu.Lock()
	current := s.mirrorJobCurrentLocked(job)
	s.mu.Unlock()
	if !current {
		s.mirrorMu.Unlock()
		return
	}

	mirrorErr := error(nil)
	gitErr := error(nil)
	if err := os.MkdirAll(job.dir, 0o755); err != nil {
		mirrorErr = err
	} else if err := job.writeFile(filepath.Join(job.dir, job.name), job.raw); err != nil {
		mirrorErr = err
	} else {
		if err := s.reconcileMirrorLocked(job); err != nil {
			slog.Warn("对账外部备份镜像失败", "dir", job.dir, "error", err)
		}
		if job.mode != "" && job.mode != "off" && job.runner != nil {
			gitErr = runGitJob(job.runner, job.dir, job.mode, job.stamp)
		}
	}
	s.mirrorMu.Unlock()

	s.mu.Lock()
	current = s.mirrorJobCurrentLocked(job)
	var settled func()
	if current {
		if mirrorErr != nil {
			s.mirrorErr = mirrorErr.Error()
		} else {
			s.lastMirror, s.mirrorErr = job.at, ""
			if job.mode != "" && job.mode != "off" && job.runner != nil {
				if gitErr != nil {
					s.gitErr = gitErr.Error()
				} else {
					s.gitErr = ""
				}
			}
		}
		settled = s.onMirrorSettled
	}
	s.mu.Unlock()

	if mirrorErr != nil {
		slog.Warn("外部备份镜像写入失败", "dir", job.dir, "error", mirrorErr)
	}
	if gitErr != nil && current {
		slog.Warn("Git 自动备份失败", "dir", job.dir, "error", gitErr)
	}
	if settled != nil {
		settled()
	}
}

// listBackupNames 返回本地备份目录中的备份文件名清单。在无锁的备份路径上调用；
// 目录不存在或读失败时返回 nil，由任务运行期的对账失败路径如实上报。
func listBackupNames(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var names []string
	for _, entry := range entries {
		if !entry.IsDir() && isBackupFileName(entry.Name()) {
			names = append(names, entry.Name())
		}
	}
	return names
}

func (s *Store) mirrorJobCurrentLocked(job mirrorJob) bool {
	return job.version == s.mirrorVersion && job.dir == s.mirrorDir && job.mode == s.gitMode
}

// reconcileMirrorLocked 清除镜像目录中本地已修剪的残留备份：合并待办会跳过
// 中间快照，仅按 keep 修剪会让滞留的旧备份永久发散（本地清单即真相源，随
// 任务代际捕获，配置切换后的在途任务只维护自己捕获的清单）。快照本身允许
// 缺席（合并跳过是有意设计），不允许滞留。调用方持有 mirrorMu。
func (s *Store) reconcileMirrorLocked(job mirrorJob) error {
	present := make(map[string]bool, len(job.names))
	for _, name := range job.names {
		present[name] = true
	}
	entries, err := os.ReadDir(job.dir)
	if err != nil {
		return err
	}
	var firstErr error
	for _, entry := range entries {
		if entry.IsDir() || !isBackupFileName(entry.Name()) || present[entry.Name()] {
			continue
		}
		if err := os.Remove(filepath.Join(job.dir, entry.Name())); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}
