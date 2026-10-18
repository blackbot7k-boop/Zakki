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
	// SetOnMirrorSettled 安装镜像/Git 结束回调；回调可从后台 goroutine 调用，
	// app 用它回主线程刷新状态栏。
	mu            sync.Mutex
	pending       *pendingSave
	stopped       bool
	saveBlocked   bool
	lastBackup    time.Time
	lastSaveErr   string
	saveTimer     *time.Timer
	saveRunning   bool
	saveRevision  uint64
	saveCompleted uint64
	saveResult    error
	stateWrite    func(string, []byte) error
	statusReadDir func(string) ([]os.DirEntry, error)

	// writeMu 串行笔记写入与写保护切换，不保护 UI 查询的状态。
	writeMu    sync.Mutex
	shutdownMu sync.Mutex

	// backupMu 串行备份状态初始化、本地备份的命名、写入与修剪；不保护可查询状态。
	backupMu       sync.Mutex
	backupLoaded   bool
	backupStamp    string
	backupSequence int
	// mirrorMu 只串行外部镜像与 Git I/O，不保护可查询状态。
	mirrorMu      sync.Mutex
	mirrorRunning bool
	mirrorPending *mirrorJob

	// 外部备份目的地与 Git 自动化（随偏好变更由 SetMirror 更新）。
	mirrorDir       string
	gitMode         string
	lastMirror      time.Time
	mirrorErr       string // 最近一次镜像拷贝失败原因（空 = 正常）
	gitErr          string // 最近一次 Git 提交/推送失败原因（空 = 正常）
	mirrorVersion   uint64 // 配置变更使已启动任务的状态失效
	mirrorRevision  uint64 // 本进程的镜像结果使尚未完成的状态扫描失效
	onMirrorSettled func()
	mirrorWrite     func(string, []byte) error
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
	revision       uint64
	file           File
	backupInterval time.Duration
	backupKeep     int
	after          func()
	ready          bool
}

type mirrorJob struct {
	dir       string
	mode      string
	name      string
	raw       []byte
	names     []string
	stamp     string
	version   uint64
	at        time.Time
	runner    GitRunner
	writeFile func(string, []byte) error
}

// NewStore 创建笔记持久化 Store。now 为 nil 时使用 time.Now。
func NewStore(dataFile, backupDir string, now func() time.Time, migrate MigrateOptions) *Store {
	if now == nil {
		now = time.Now
	}
	return &Store{
		DataFile:    dataFile,
		BackupDir:   backupDir,
		Now:         now,
		Migrate:     migrate,
		mirrorWrite: atomicfile.WriteBytes,
		stateWrite:  atomicfile.WriteBytes,
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

// LoadNewestBackup 从备份目录新→旧找第一个可用备份。
func (s *Store) LoadNewestBackup() (File, string, bool) {
	entries, err := os.ReadDir(s.BackupDir)
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

// BackupNow 在已保存数据基础上无条件写一份本地备份，并在后台镜像。
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

// writeBackup 写本地备份并提交不可变镜像任务。外部 I/O 不占用 s.mu。
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
	s.mu.Unlock()

	raw, err := os.ReadFile(dataPath)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(s.BackupDir, 0o755); err != nil {
		return "", err
	}
	name := s.uniqueBackupName()
	if err := os.WriteFile(filepath.Join(s.BackupDir, name), raw, 0o644); err != nil {
		return "", err
	}
	if err := s.pruneJSON(s.BackupDir, keep); err != nil {
		slog.Warn("修剪旧备份失败", "error", err)
	}

	at := s.Now()
	s.mu.Lock()
	s.lastBackup = at
	job, ok := s.mirrorJobLocked(name, raw, at)
	s.mu.Unlock()
	if ok {
		job.names = listBackupNames(s.BackupDir)
		s.enqueueMirrorJob(job)
	}
	return name, nil
}

func (s *Store) mirrorJobLocked(name string, raw []byte, at time.Time) (mirrorJob, bool) {
	if s.mirrorDir == "" {
		return mirrorJob{}, false
	}
	writeFile := s.mirrorWrite
	if writeFile == nil {
		writeFile = atomicfile.WriteBytes
	}
	return mirrorJob{
		dir:       s.mirrorDir,
		mode:      s.gitMode,
		name:      name,
		raw:       raw,
		stamp:     at.Format("20060102-150405"),
		version:   s.mirrorVersion,
		at:        at,
		runner:    s.RunGit,
		writeFile: writeFile,
	}, true
}

// runGitJob 在镜像目录执行 Git 提交（可选推送）。无变更时静默跳过。
// 精确路径来自当前文件和已跟踪文件，既能记录删除，也不会把不存在的旧前缀
// 作为 pathspec 交给 add/commit。
func runGitJob(runner GitRunner, dir, mode, stamp string) error {
	paths, err := gitBackupPaths(runner, dir)
	if err != nil {
		return err
	}
	if len(paths) == 0 {
		return nil
	}
	args := append([]string{"add", "--"}, paths...)
	if _, err := runner(dir, args...); err != nil {
		return err
	}
	args = append([]string{"status", "--porcelain", "--"}, paths...)
	out, err := runner(dir, args...)
	if err != nil {
		return err
	}
	if strings.TrimSpace(out) == "" {
		return nil // 没有本应用备份变更：跳过提交
	}
	args = append([]string{"commit", "--only", "-m", "backup: zakki-" + stamp, "--"}, paths...)
	if _, err := runner(dir, args...); err != nil {
		return err
	}
	if mode == "commitPush" {
		if _, err := runner(dir, "push"); err != nil {
			return err
		}
	}
	return nil
}

func gitBackupPaths(runner GitRunner, dir string) ([]string, error) {
	paths := make(map[string]struct{})
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if !entry.IsDir() && isBackupFileName(entry.Name()) {
			paths[entry.Name()] = struct{}{}
		}
	}
	tracked, err := runner(dir, "ls-files", "-z", "--", "zakki-*.json", "tot-md-*.json")
	if err != nil {
		return nil, err
	}
	for _, path := range strings.Split(tracked, "\x00") {
		if path == "" || filepath.Clean(path) != filepath.Base(path) || !isBackupFileName(path) {
			continue
		}
		paths[path] = struct{}{}
	}
	out := make([]string, 0, len(paths))
	for path := range paths {
		out = append(out, path)
	}
	sort.Strings(out)
	return out, nil
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

// SetOnMirrorSettled 原子替换后台镜像/Git 完成通知回调。
func (s *Store) SetOnMirrorSettled(fn func()) {
	s.mu.Lock()
	s.onMirrorSettled = fn
	s.mu.Unlock()
}

// SetMirror 更新外部备份目的地与 Git 模式（主线程随偏好变更调用）。
// 目录变更后全部旧状态作废；Git 模式变更后旧 Git 错误作废。
func (s *Store) SetMirror(dir, gitMode string) {
	s.mu.Lock()
	if dir != s.mirrorDir || gitMode != s.gitMode {
		s.mirrorVersion++
		s.mirrorPending = nil
	}
	if dir != s.mirrorDir {
		s.mirrorErr, s.gitErr, s.lastMirror = "", "", time.Time{}
	} else if gitMode != s.gitMode {
		s.gitErr = ""
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

func (s *Store) uniqueBackupName() string {
	stamp := s.Now().Format("20060102-150405")
	sequence := 1
	if s.backupStamp == stamp {
		sequence = s.backupSequence + 1
	} else if entries, err := os.ReadDir(s.BackupDir); err == nil {
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
		_, err := os.Stat(filepath.Join(s.BackupDir, name))
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

// pruneBackupsLocked 修剪本地备份目录，只保留最新 keep 份。
func (s *Store) pruneBackupsLocked(keep int) error {
	return s.pruneJSON(s.BackupDir, keep)
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
