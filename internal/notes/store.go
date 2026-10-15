package notes

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
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
	DataFile  string
	BackupDir string
	Now       func() time.Time
	Migrate   MigrateOptions
	// RunGit 在镜像目录执行 git 命令；生产用 DefaultGitRunner，测试注入假 runner。
	// 为 nil 时禁用 Git 自动化。
	RunGit GitRunner
	// OnMirrorSettled 镜像/Git 结束后的回调（任意 goroutine 调用；app 用它回主线程
	// 刷新状态栏）。可为 nil。
	OnMirrorSettled func()

	mu          sync.Mutex
	pending     *pendingSave
	stopped     bool
	saveBlocked bool
	lastBackup  time.Time
	lastSaveErr string
	saveTimer   *time.Timer

	// 外部备份目的地与 Git 自动化（随偏好变更由 SetMirror 更新）。
	mirrorDir  string
	gitMode    string
	lastMirror time.Time
	mirrorErr  string // 最近一次镜像拷贝失败原因（空 = 正常）
	gitErr     string // 最近一次 Git 提交/推送失败原因（空 = 正常）
	gitBusy    bool   // 后台 Git 任务执行中
	gitPending bool   // 忙碌期间又发生了备份，结束后需补一次
}

// GitRunner 在目录 dir 上执行 git 参数并返回合并输出。
type GitRunner func(dir string, args ...string) (string, error)

// MirrorStatus 是外部备份的当前状态快照（主线程每帧读取）。
type MirrorStatus struct {
	Dir  string    // 目的地，空 = 未配置
	Mode string    // off / commit / commitPush
	Last time.Time // 最近一次成功镜像；零值 = 尚未镜像
	Err  string    // 最近一次镜像或 Git 错误（空 = 正常）
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
		_, err = s.writeBackupLocked(keep)
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
	_, err := s.writeBackupLocked(keep)
	return err
}

// writeBackupLocked 写本地备份并顺带镜像到外部目录。
// 返回刚写入的备份文件名。必须持有 s.mu。
func (s *Store) writeBackupLocked(keep int) (string, error) {
	raw, err := os.ReadFile(s.currentDataPathLocked())
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(s.BackupDir, 0o755); err != nil {
		return "", err
	}
	name := s.uniqueBackupNameLocked()
	if err := os.WriteFile(filepath.Join(s.BackupDir, name), raw, 0o644); err != nil {
		return "", err
	}
	s.lastBackup = s.Now()
	if err := s.pruneJSONLocked(s.BackupDir, keep); err != nil {
		slog.Warn("修剪旧备份失败", "error", err)
	}
	s.writeMirrorLocked(name, raw, keep)
	return name, nil
}

// writeMirrorLocked 把刚写入的备份镜像到外部目录并按需 Git 提交。
// 失败只记录状态与日志，绝不向上返回错误：外部目的地（云盘、Git）不可用
// 不能拖垮本地备份。目录可能是云盘同步文件夹，拷贝耗时略长，可接受。
// 必须持有 s.mu。
func (s *Store) writeMirrorLocked(name string, raw []byte, keep int) {
	dir := s.mirrorDir
	if dir == "" {
		return
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		s.mirrorErr = err.Error()
		slog.Warn("外部备份目录不可用", "dir", dir, "error", err)
		return
	}
	if err := atomicfile.WriteBytes(filepath.Join(dir, name), raw); err != nil {
		s.mirrorErr = err.Error()
		slog.Warn("外部备份镜像写入失败", "dir", dir, "error", err)
		return
	}
	s.lastMirror, s.mirrorErr = s.Now(), ""
	if err := s.pruneJSONLocked(dir, keep); err != nil {
		slog.Warn("修剪外部备份失败", "dir", dir, "error", err)
	}
	s.kickGitLocked()
}

// kickGitLocked 在需要时起后台 goroutine 做 Git 提交（可选推送）；
// 忙碌期间只记待办，当前任务结束后补一次。必须持有 s.mu。
func (s *Store) kickGitLocked() {
	if s.gitMode == "" || s.gitMode == "off" || s.RunGit == nil {
		return
	}
	if s.gitBusy {
		s.gitPending = true
		return
	}
	s.gitBusy = true
	dir, mode, runner, settled := s.mirrorDir, s.gitMode, s.RunGit, s.OnMirrorSettled
	stamp := s.Now().Format("20060102-150405")
	go func() {
		err := runGitJob(runner, dir, mode, stamp)
		s.mu.Lock()
		s.gitBusy = false
		if err != nil {
			s.gitErr = err.Error()
		} else {
			s.gitErr = ""
		}
		re := s.gitPending
		s.gitPending = false
		s.mu.Unlock()
		if err != nil {
			slog.Warn("Git 自动备份失败", "dir", dir, "error", err)
		}
		if settled != nil {
			settled()
		}
		if re {
			s.mu.Lock()
			s.kickGitLocked()
			s.mu.Unlock()
		}
	}()
}

// runGitJob 在镜像目录执行 Git 提交（可选推送）。无变更时静默跳过。
// 只 add 本应用的备份文件（含更名前的旧前缀，使旧文件改名/淘汰进入历史），
// 不碰目录里的其他内容。
func runGitJob(runner GitRunner, dir, mode, stamp string) error {
	if _, err := runner(dir, "add", "--", "zakki-*.json", "tot-md-*.json"); err != nil {
		return err
	}
	out, err := runner(dir, "status", "--porcelain")
	if err != nil {
		return err
	}
	if strings.TrimSpace(out) == "" {
		return nil // 没有变更：跳过提交
	}
	if _, err := runner(dir, "commit", "-m", "backup: zakki-"+stamp); err != nil {
		return err
	}
	if mode == "commitPush" {
		if _, err := runner(dir, "push"); err != nil {
			return err
		}
	}
	return nil
}

// backupFilePrefix 是当前备份与镜像文件的命名前缀；legacyBackupFilePrefix
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

// DefaultGitRunner 返回调用本机 git CLI 的 runner。
func DefaultGitRunner() GitRunner {
	return func(dir string, args ...string) (string, error) {
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		var out bytes.Buffer
		cmd.Stdout, cmd.Stderr = &out, &out
		err := cmd.Run()
		if err != nil {
			return strings.TrimSpace(out.String()), fmt.Errorf("git %s: %w（%s）",
				strings.Join(args, " "), err, strings.TrimSpace(out.String()))
		}
		return out.String(), nil
	}
}

// SetMirror 更新外部备份目的地与 Git 模式（主线程随偏好变更调用）。
// dir 变更后旧的错误状态随之作废。
func (s *Store) SetMirror(dir, gitMode string) {
	s.mu.Lock()
	if dir != s.mirrorDir {
		s.mirrorErr, s.gitErr, s.lastMirror = "", "", time.Time{}
	}
	s.mirrorDir, s.gitMode = dir, gitMode
	s.mu.Unlock()
}

// MirrorStatus 外部备份当前状态。
func (s *Store) MirrorStatus() MirrorStatus {
	s.mu.Lock()
	defer s.mu.Unlock()
	err := s.mirrorErr
	if err == "" {
		err = s.gitErr
	}
	return MirrorStatus{Dir: s.mirrorDir, Mode: s.gitMode, Last: s.lastMirror, Err: err}
}

// LoadMirrorTime 启动时从镜像目录恢复最近镜像时间（未配置或目录不存在时静默）。
func (s *Store) LoadMirrorTime() error {
	s.mu.Lock()
	dir := s.mirrorDir
	s.mu.Unlock()
	if dir == "" {
		return nil
	}
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	var newest time.Time
	for _, e := range entries {
		if e.IsDir() || !isBackupFileName(e.Name()) {
			continue
		}
		if info, err := e.Info(); err == nil && info.ModTime().After(newest) {
			newest = info.ModTime()
		}
	}
	s.mu.Lock()
	s.lastMirror = newest
	s.mu.Unlock()
	return nil
}

func (s *Store) uniqueBackupNameLocked() string {
	base := backupFilePrefix + s.Now().Format("20060102-150405")
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
	_, err := s.writeBackupLocked(keep)
	return err
}

// pruneBackupsLocked 修剪本地备份目录，只保留最新 keep 份。
func (s *Store) pruneBackupsLocked(keep int) error {
	return s.pruneJSONLocked(s.BackupDir, keep)
}

// pruneJSONLocked 按文件名降序只保留目录里最新 keep 份备份文件
// （zakki-*.json 与旧前缀 tot-md-*.json 一起排序）；只动本应用的备份命名
// 空间，不碰目录里的其他文件。
func (s *Store) pruneJSONLocked(dir string, keep int) error {
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
	sort.Slice(files, func(i, j int) bool { return files[i].Name() > files[j].Name() })
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
