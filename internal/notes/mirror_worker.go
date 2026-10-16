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
		if err := s.pruneJSON(job.dir, job.keep); err != nil {
			slog.Warn("修剪外部备份失败", "dir", job.dir, "error", err)
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

func (s *Store) mirrorJobCurrentLocked(job mirrorJob) bool {
	return job.version == s.mirrorVersion && job.dir == s.mirrorDir && job.mode == s.gitMode
}
