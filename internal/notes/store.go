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

	"zakki/internal/atomicfile"
)

// Store 负责笔记数据文件的读写、防抖保存、滚动备份与写保护协调。
// 路径与时钟在构造时注入，便于测试使用临时目录与固定时钟。
type Store struct {
	DataFile string
	// DefaultBackupDir 是未选择备份文件夹时的内置目的地（应用数据 backups/）。
	DefaultBackupDir string
	Now              func() time.Time
	Migrate          MigrateOptions
	mu               sync.Mutex
	pending          *pendingSave
	stopped          bool
	saveBlocked      bool
	lastBackup       time.Time
	lastSaveErr      string
	saveTimer        *time.Timer
	saveRunning      bool
	saveRevision     uint64
	saveCompleted    uint64
	saveResult       error
	stateWrite       func(string, []byte) error
	statusReadDir    func(string) ([]os.DirEntry, error)

	// writeMu 串行笔记写入与写保护切换，不保护 UI 查询的状态。
	writeMu    sync.Mutex
	shutdownMu sync.Mutex

	// backupMu 串行备份状态初始化、备份的命名、写入与修剪；不保护可查询状态。
	backupMu       sync.Mutex
	backupLoaded   bool
	backupStamp    string
	backupSequence int

	// backupDir 是用户选择的备份文件夹，唯一的备份存储地址；空 = DefaultBackupDir。
	// 随偏好变更由 SetBackupDir 更新（主线程）。
	backupDir string

	// RunGit 执行 git 命令；测试可注入假 runner。
	RunGit GitRunner

	// Git 自动化配置由主线程随偏好变更写入；状态由 git worker 发布。
	gitAutoPush     bool
	gitPushInterval time.Duration
	lastGitCommit   time.Time
	lastGitPush     time.Time
	lastPushAttempt time.Time
	gitErr          string
	// git worker 不持 backupMu：备份入口可能在主线程，git 命令（含网络推送）
	// 不能让它等。worker 只保留最新一次排队请求；gitGen 随备份文件夹变更递增，
	// 旧文件夹的结果不再发布。gitDone 在 worker 退出时关闭。
	gitQueued  *gitRequest
	gitRunning bool
	gitDone    chan struct{}
	gitGen     uint64
}

type gitRequest struct {
	dir, name string
	gen       uint64
}

type pendingSave struct {
	revision       uint64
	file           File
	backupInterval time.Duration
	backupKeep     int
	after          func()
	ready          bool
}

// NewStore 创建笔记持久化 Store。now 为 nil 时使用 time.Now。
func NewStore(dataFile, defaultBackupDir string, now func() time.Time, migrate MigrateOptions) *Store {
	if now == nil {
		now = time.Now
	}
	s := &Store{
		DataFile:         dataFile,
		DefaultBackupDir: defaultBackupDir,
		Now:              now,
		Migrate:          migrate,
		stateWrite:       atomicfile.WriteBytes,
		RunGit:           DefaultGitRunner(),
	}
	return s
}

// SetBackupDir 更新备份文件夹（主线程随偏好变更调用）；空串回到内置目录。
// 文件夹变更后 Git 状态不再属于新目的地，一并作废。
func (s *Store) SetBackupDir(dir string) {
	s.mu.Lock()
	if dir != s.backupDir {
		s.lastGitCommit, s.lastGitPush, s.lastPushAttempt, s.gitErr = time.Time{}, time.Time{}, time.Time{}, ""
		s.gitGen++
		s.gitQueued = nil
	}
	s.backupDir = dir
	s.mu.Unlock()
}

// SetGitAutoPush 更新 Git 自动化配置（主线程随偏好变更调用）。interval 为
// 两次自动 push 尝试之间的最小间隔；enabled 为 false 时备份后不做任何 git 操作。
func (s *Store) SetGitAutoPush(enabled bool, interval time.Duration) {
	s.mu.Lock()
	if !enabled && s.gitAutoPush {
		s.gitErr = "" // 关闭后旧错误不再有意义
	}
	s.gitAutoPush, s.gitPushInterval = enabled, interval
	s.mu.Unlock()
}

// GitCommitTime 最近一次自动提交时间（零值表示尚未提交过）。
func (s *Store) GitCommitTime() time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastGitCommit
}

// GitPushTime 最近一次成功自动推送时间（零值表示尚未推送过）。
func (s *Store) GitPushTime() time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastGitPush
}

// GitError 最近一次自动提交/推送的错误描述（空串表示没有）。
func (s *Store) GitError() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.gitErr
}

// BackupDir 返回当前生效的备份目的地：所选文件夹，未选择时是内置目录。
func (s *Store) BackupDir() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.backupDirLocked()
}

func (s *Store) backupDirLocked() string {
	if s.backupDir != "" {
		return s.backupDir
	}
	return s.DefaultBackupDir
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
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
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

// LoadPending 读取并验证写保护期间留下的暂存文件。
// 返回 false 表示文件不存在或不可解析；调用方应保留原文件而非删除。
func (s *Store) LoadPending() (File, bool) {
	raw, err := os.ReadFile(s.PendingPath())
	if err != nil {
		return File{}, false
	}
	f, _, err := s.ParseRaw(raw)
	if err != nil {
		return File{}, false
	}
	return f, true
}

func (s *Store) QuarantineCorruptFile() {
	raw, err := os.ReadFile(s.DataFile)
	if err != nil {
		slog.Warn("损坏文件无法复制隔离，原件保持原样", "error", err)
		return
	}
	base := s.DataFile + ".corrupt-" + s.Now().Format("20060102-150405")
	dst := base
	for i := 2; ; i++ {
		if _, err := os.Lstat(dst); errors.Is(err, os.ErrNotExist) {
			break
		} else if err != nil {
			slog.Warn("隔离副本状态无法读取，原件保持原样", "path", dst, "error", err)
			return
		}
		dst = fmt.Sprintf("%s-%d", base, i)
	}
	if err := os.WriteFile(dst, raw, 0o644); err != nil {
		slog.Warn("隔离副本写入失败，原件保持原样", "path", dst, "error", err)
		return
	}
	slog.Info("损坏文件已隔离", "path", dst)
}

// LoadNewestBackup 从备份文件夹新→旧找第一个可用备份；文件夹里没有时回退
// 内置目录（历史版本的备份都在那里，换文件夹后首次损坏仍可恢复）。
func (s *Store) LoadNewestBackup() (File, string, bool) {
	if f, name, ok := s.loadNewestBackupFrom(s.BackupDir()); ok {
		return f, name, true
	}
	if s.BackupDir() == s.DefaultBackupDir {
		return File{}, "", false
	}
	return s.loadNewestBackupFrom(s.DefaultBackupDir)
}

func (s *Store) loadNewestBackupFrom(dir string) (File, string, bool) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return File{}, "", false
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && isBackupFileName(e.Name()) {
			names = append(names, e.Name())
		}
	}
	sort.Slice(names, func(i, j int) bool { return backupNameNewer(names[i], names[j]) })
	for _, name := range names {
		raw, err := os.ReadFile(filepath.Join(dir, name))
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

// BackupNow 在已保存数据基础上无条件写一份备份。
func (s *Store) BackupNow(keep int) error {
	name, err := s.writeBackup(keep, 0, false)
	if name == "" && err == nil {
		return nil
	}
	s.ReportPersist("备份失败", err)
	return err
}

// MaybeWritePeriodicBackup 距上次备份超过 interval 才写一份。
func (s *Store) MaybeWritePeriodicBackup(interval time.Duration, keep int) error {
	return s.maybeWritePeriodicBackup(interval, keep)
}

func (s *Store) maybeWritePeriodicBackup(interval time.Duration, keep int) error {
	_, err := s.writeBackup(keep, interval, true)
	return err
}

// writeBackup 向备份文件夹写一份备份。所选文件夹可能在云盘同步目录，写入
// 在备份路径（防抖保存 worker / 启动任务 goroutine）上同步进行，不占主线程；
// 用 tmp+rename 原子写，避免云盘同步到半截文件。
func (s *Store) writeBackup(keep int, interval time.Duration, periodic bool) (string, error) {
	s.backupMu.Lock()
	defer s.backupMu.Unlock()

	if err := s.loadLastBackupTimeLocked(); err != nil {
		slog.Warn("读取最近备份时间失败", "error", err)
	}
	s.mu.Lock()
	if s.stopped || periodic && !s.lastBackup.IsZero() && s.Now().Sub(s.lastBackup) <= interval {
		s.mu.Unlock()
		return "", nil
	}
	dataPath := s.currentDataPathLocked()
	dest := s.backupDirLocked()
	s.mu.Unlock()

	raw, err := os.ReadFile(dataPath)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return "", err
	}
	name := s.uniqueBackupName()
	if err := atomicfile.WriteBytes(filepath.Join(dest, name), raw); err != nil {
		return "", err
	}
	if err := s.pruneJSON(dest, keep); err != nil {
		slog.Warn("修剪旧备份失败", "error", err)
	}

	s.mu.Lock()
	s.lastBackup = s.Now()
	if s.gitAutoPush {
		s.queueGitLocked(gitRequest{dir: dest, name: name, gen: s.gitGen})
	}
	s.mu.Unlock()
	return name, nil
}

// queueGitLocked 排队一次 git 步骤并在需要时启动 worker；调用方持有 mu。
// 忙碌时只保留最新请求：一次提交会带上此前所有未提交的备份变更。
func (s *Store) queueGitLocked(req gitRequest) {
	s.gitQueued = &req
	if s.gitRunning {
		return
	}
	s.gitRunning = true
	s.gitDone = make(chan struct{})
	go s.gitWorker()
}

func (s *Store) gitWorker() {
	for {
		s.mu.Lock()
		req := s.gitQueued
		s.gitQueued = nil
		if req == nil {
			s.gitRunning = false
			close(s.gitDone)
			s.mu.Unlock()
			return
		}
		s.mu.Unlock()
		s.runGitBackupStep(*req)
	}
}

// WaitGitIdle 等待已排队的 git 步骤全部完成（测试与需要确定状态的调用方用）。
func (s *Store) WaitGitIdle() {
	for {
		s.mu.Lock()
		if !s.gitRunning {
			s.mu.Unlock()
			return
		}
		done := s.gitDone
		s.mu.Unlock()
		<-done
	}
}

// runGitBackupStep 把备份文件夹里的备份变更（含修剪产生的删除）提交到 git
// 仓库，并在距上次推送尝试超过配置间隔时 push。在 git worker 上运行，与备份
// 写入并行；命令耗时受 gitCommandTimeout 约束。仓库缺失时静默跳过；
// 身份/远端等失败如实记录，下次备份自动重试。备份文件夹已更换时丢弃结果。
func (s *Store) runGitBackupStep(req gitRequest) {
	if !DirIsGitRepo(req.dir) {
		return
	}
	message := "backup: " + strings.TrimSuffix(req.name, ".json")
	committed, err := runGitJob(s.RunGit, req.dir, message)

	s.mu.Lock()
	if s.gitGen != req.gen {
		s.mu.Unlock()
		return
	}
	if err != nil {
		s.gitErr = err.Error()
	} else {
		s.gitErr = ""
		if committed {
			s.lastGitCommit = s.Now()
		}
	}
	interval := s.gitPushInterval
	s.mu.Unlock()
	if err != nil {
		slog.Warn("Git 自动备份失败", "dir", req.dir, "error", err)
		return // 提交失败不推送，等下次备份重试
	}

	// 推送按尝试时间节流：失败也计一次尝试，避免无网时每次备份都硬撞。
	now := s.Now()
	s.mu.Lock()
	due := s.gitGen == req.gen && interval > 0 &&
		(s.lastPushAttempt.IsZero() || now.Sub(s.lastPushAttempt) >= interval)
	if due {
		s.lastPushAttempt = now
	}
	s.mu.Unlock()
	if !due {
		return
	}
	_, err = s.runGit(req.dir, "push")
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.gitGen != req.gen {
		return
	}
	if err != nil {
		s.gitErr = err.Error()
		slog.Warn("Git 自动推送失败", "dir", req.dir, "error", err)
		return
	}
	s.gitErr = ""
	s.lastGitPush = s.Now()
}

// runGit 执行一条 git 命令；runner 未注入时视为未配置，返回哨兵错误。
func (s *Store) runGit(dir string, args ...string) (string, error) {
	if s.RunGit == nil {
		return "", fmt.Errorf("git runner 未配置")
	}
	return s.RunGit(dir, args...)
}

// backupFilePrefix 是备份文件的命名前缀；legacyBackupFilePrefix
// 是应用更名前旧版本写入的前缀。识别、修剪与 Git 路径规范继续匹配旧前缀，
// 让外部目录里遗留的旧命名文件随保留策略自然淘汰，不额外改写用户目录。
const (
	backupFilePrefix       = "zakki-"
	legacyBackupFilePrefix = "tot-md-"
)

// isBackupFileName 报告 name 是否本应用的备份文件（新旧前缀都认）。
func isBackupFileName(name string) bool {
	if !strings.HasSuffix(name, ".json") {
		return false
	}
	return strings.HasPrefix(name, backupFilePrefix) || strings.HasPrefix(name, legacyBackupFilePrefix)
}

// backupNameNewer 按备份时间戳、再按同秒序号数值排序。字符串排序会把 -10
// 放到 -9 前，导致恢复与裁剪选择错误；两条路径必须使用同一比较规则。
func backupNameNewer(left, right string) bool {
	lt, ln, lok := backupNameParts(left)
	rt, rn, rok := backupNameParts(right)
	if lok && rok {
		if lt != rt {
			return lt > rt
		}
		return ln > rn
	}
	return left > right
}

func backupNameParts(name string) (timestamp string, sequence int, ok bool) {
	name = strings.TrimSuffix(name, ".json")
	for _, prefix := range []string{backupFilePrefix, legacyBackupFilePrefix} {
		if !strings.HasPrefix(name, prefix) {
			continue
		}
		rest := strings.TrimPrefix(name, prefix)
		if len(rest) < len("20060102-150405") {
			return "", 0, false
		}
		timestamp = rest[:len("20060102-150405")]
		if _, err := time.Parse("20060102-150405", timestamp); err != nil {
			return "", 0, false
		}
		sequence = 1
		if len(rest) == len(timestamp) {
			return timestamp, sequence, true
		}
		if rest[len(timestamp)] != '-' {
			return "", 0, false
		}
		var parsed int
		if _, err := fmt.Sscanf(rest[len(timestamp)+1:], "%d", &parsed); err != nil || parsed < 2 {
			return "", 0, false
		}
		return timestamp, parsed, true
	}
	return "", 0, false
}

// uniqueBackupName 在当前备份文件夹里挑下一个未占用的名字。同秒序号缓存
// 跨文件夹复用是安全的：候选名总会再对新目录 Stat 确认。
func (s *Store) uniqueBackupName() string {
	stamp := s.Now().Format("20060102-150405")
	sequence := 1
	if s.backupStamp == stamp {
		sequence = s.backupSequence + 1
	} else if entries, err := os.ReadDir(s.backupDirLocked()); err == nil {
		for _, entry := range entries {
			entryStamp, entrySequence, ok := backupNameParts(entry.Name())
			if ok && entryStamp == stamp && entrySequence >= sequence {
				sequence = entrySequence + 1
			}
		}
	}
	base := backupFilePrefix + stamp
	for {
		name := base + ".json"
		if sequence > 1 {
			name = fmt.Sprintf("%s-%d.json", base, sequence)
		}
		_, err := os.Stat(filepath.Join(s.backupDirLocked(), name))
		if errors.Is(err, os.ErrNotExist) || err != nil {
			s.backupStamp, s.backupSequence = stamp, sequence
			return name
		}
		sequence++
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
	_, err := s.writeBackup(keep, 0, false)
	return err
}

// pruneJSON 按文件名降序只保留目录里最新 keep 份备份文件
// （zakki-*.json 与旧前缀 tot-md-*.json 一起排序）；只动本应用的备份命名
// 空间，不碰目录里的其他文件。
func (s *Store) pruneJSON(dir string, keep int) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	var files []os.DirEntry
	for _, e := range entries {
		if !e.IsDir() && isBackupFileName(e.Name()) {
			files = append(files, e)
		}
	}
	sort.Slice(files, func(i, j int) bool { return backupNameNewer(files[i].Name(), files[j].Name()) })
	var firstErr error
	for i := keep; i < len(files); i++ {
		if err := os.Remove(filepath.Join(dir, files[i].Name())); err != nil && firstErr == nil {
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
