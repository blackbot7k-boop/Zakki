package app

// 数据目录一次性迁移：应用更名（Tot MD → Zakki）后，把旧目录整体迁到新目录，
// 数据与备份文件按新命名规则改名。只认旧命名模式的文件，其余原样保留。

import (
	"bytes"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/egoist/mygo"

	"zakki/internal/atomicfile"
)

const (
	// dataDirectoryName 数据目录名；旧安装的目录名是 legacyDataDirectoryName，
	// 首次启动时整体迁移过来。
	dataDirectoryName       = "Zakki"
	legacyDataDirectoryName = "Tot MD"
	// devDataDirectoryName 开发运行（go tool mygo dev / go run）的数据目录：
	// 与正式安装共处一台机器时互不污染，开发目录全新、不做旧目录迁移。
	devDataDirectoryName    = "Zakki Dev"
	dataFileName            = "zakki.json"
	legacyDataFileName      = "tot-md.json"
	legacyDataPendingSuffix = ".pending"
	backupDirName           = "backups"
	legacyBackupFilePrefix  = "tot-md-"
	backupFilePrefix        = "zakki-"
)

var (
	migrateDataDirOnce = new(sync.Once)
	resolvedDataDir    string
	resolvedDataDirErr error
	// devMode 判定开发运行；测试可替换。
	devMode = mygo.IsDev
)

// mygoAppDataBase 返回系统应用数据根目录；测试可替换。
var mygoAppDataBase = func() (string, error) { return mygo.App.Path(mygo.PathAppData) }

// legacyDataDirLstat 读取旧数据目录元信息；测试可注入探测失败。
var legacyDataDirLstat = os.Lstat

// legacySourceLstat 与 newDataPathLstat 只用于迁移预检，测试可注入
// 无法由当前平台权限稳定复现的探测错误。
var (
	legacySourceLstat = os.Lstat
	newDataPathLstat  = os.Lstat
)

// resolveDataDir 只解析和迁移一次，同时缓存目录与错误。生产启动必须检查
// error；迁移失败时本进程不得继续构造 Store，下一次进程启动会重新尝试。
func resolveDataDir() (string, error) {
	migrateDataDirOnce.Do(func() {
		base, err := mygoAppDataBase()
		if err != nil || base == "" {
			home, _ := os.UserHomeDir()
			base = filepath.Join(home, "Library", "Application Support")
		}
		// 开发运行的数据目录与正式安装分开：正式目录 Zakki 留给日常使用的
		// 安装版（含本地 make build 产物），首启向导与迁移语义在两边各自
		// 完整生效；开发目录全新，不迁移旧目录、不触碰正式目录。
		if devMode() {
			resolvedDataDir = filepath.Join(base, devDataDirectoryName)
			return
		}
		resolvedDataDir = filepath.Join(base, dataDirectoryName)
		if err := migrateDataDir(base); err != nil {
			resolvedDataDirErr = fmt.Errorf("迁移旧数据目录 %s: %w", filepath.Join(base, legacyDataDirectoryName), err)
		}
	})
	return resolvedDataDir, resolvedDataDirErr
}

// dataDir 是旧调用点的兼容包装。生产 Run 应改用 resolveDataDir 并在错误时
// 立即返回，随后把同一个目录传给 newAppAt。
func dataDir() string {
	dir, err := resolveDataDir()
	if err != nil {
		slog.Error("旧数据目录迁移未完成，启动必须停止", "error", err)
	}
	return dir
}

// migrateDataDir 把 <base>/Tot MD 迁到 <base>/Zakki：
//   - 新目录不存在：整个目录改名（同卷原子），再按新命名规则改内部文件名；
//   - 新目录已存在且为空壳（无 zakki.json / preferences.json）：逐文件搬入后保留旧目录；
//   - 新目录已有数据：不动旧目录，等待人工处理。
//
// 返回 error 表示迁移未完成；Run 必须停止启动，修复后由下一进程重试。
func migrateDataDir(base string) error {
	oldDir := filepath.Join(base, legacyDataDirectoryName)
	newDir := filepath.Join(base, dataDirectoryName)
	if oldDir == newDir {
		return nil
	}
	st, err := legacyDataDirLstat(oldDir)
	if errors.Is(err, os.ErrNotExist) {
		return nil // 没有旧目录：全新安装或已迁移
	}
	if err != nil {
		return fmt.Errorf("检查旧数据目录 %s: %w", oldDir, err)
	}
	sourceDir, removeSource := oldDir, true
	if st.Mode()&os.ModeSymlink != 0 {
		resolved, err := filepath.EvalSymlinks(oldDir)
		if err != nil {
			return fmt.Errorf("解析旧数据目录链接 %s: %w", oldDir, err)
		}
		resolvedInfo, err := os.Stat(resolved)
		if err != nil {
			return fmt.Errorf("检查旧数据目录链接目标 %s: %w", resolved, err)
		}
		if !resolvedInfo.IsDir() {
			return fmt.Errorf("旧数据目录链接目标不是目录: %s", resolved)
		}
		sourceDir, removeSource = resolved, false
	} else if !st.IsDir() {
		return fmt.Errorf("旧数据路径不是目录: %s", oldDir)
	}
	newInfo, err := os.Lstat(newDir)
	if errors.Is(err, os.ErrNotExist) {
		return migrateWholeDataDir(sourceDir, newDir, removeSource)
	}
	if err != nil {
		return fmt.Errorf("检查新数据目录 %s: %w", newDir, err)
	}
	if !newInfo.IsDir() {
		return fmt.Errorf("新数据路径不是目录: %s", newDir)
	}
	// 两目录并存：仅当新目录还没有任何数据文件时复制迁移。旧目录在
	// 整批成功前后都保留，任何部分失败都可在下次启动安全重试。
	exists, err := newDataExists(newDir)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	return copyLegacyFiles(sourceDir, newDir)
}

// newDataExists 报告目录里是否已有新版命名或沿用旧版同名的数据/偏好文件。
// 无法确定路径是否存在时返回错误，避免在不明状态下覆盖目标目录。
func newDataExists(dir string) (bool, error) {
	for _, name := range []string{
		dataFileName, legacyDataFileName,
		dataFileName + legacyDataPendingSuffix, legacyDataFileName + legacyDataPendingSuffix,
		"preferences.json", "config.toml", "config.json",
	} {
		path := filepath.Join(dir, name)
		if _, err := newDataPathLstat(path); err == nil {
			return true, nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return false, fmt.Errorf("检查新数据路径 %s: %w", path, err)
		}
	}
	path := filepath.Join(dir, backupDirName)
	entries, err := os.ReadDir(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("检查新备份目录 %s: %w", path, err)
	}
	return len(entries) > 0, nil
}

// legacyFilePairs 旧→新命名的顶层文件对（迁移到新目录时另有 preferences.json）。
func legacyFilePairs() [][2]string {
	return [][2]string{
		{legacyDataFileName, dataFileName},
		{legacyDataFileName + legacyDataPendingSuffix, dataFileName + legacyDataPendingSuffix},
	}
}

// migrateWholeDataDir 先在同级临时目录复制并完成内部更名，再原子发布。
// 原目录在所有步骤成功前保持不变，避免半迁移破坏下次重试。
// removeSource 仅用于普通旧目录；根目录符号链接及其目标始终保留。
func migrateWholeDataDir(oldDir, newDir string, removeSource bool) error {
	stage, err := os.MkdirTemp(filepath.Dir(newDir), ".zakki-migrate-*")
	if err != nil {
		return fmt.Errorf("创建迁移暂存目录: %w", err)
	}
	if err := os.Remove(stage); err != nil {
		return fmt.Errorf("准备迁移暂存目录: %w", err)
	}
	defer os.RemoveAll(stage)
	if err := copyTree(oldDir, stage); err != nil {
		return fmt.Errorf("复制旧数据目录: %w", err)
	}
	if err := renameDataFilesIn(stage); err != nil {
		return err
	}
	if err := os.Rename(stage, newDir); err != nil {
		return fmt.Errorf("发布新数据目录: %w", err)
	}
	if removeSource {
		if err := os.RemoveAll(oldDir); err != nil {
			slog.Warn("新数据目录已发布，但旧目录未能删除", "dir", oldDir, "error", err)
		}
	}
	return nil
}

func copyTree(from, to string) error {
	st, err := os.Lstat(from)
	if err != nil {
		return err
	}
	switch mode := st.Mode(); {
	case mode.IsDir():
		if err := os.Mkdir(to, mode.Perm()); err != nil {
			return err
		}
		entries, err := os.ReadDir(from)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if err := copyTree(filepath.Join(from, entry.Name()), filepath.Join(to, entry.Name())); err != nil {
				return err
			}
		}
		return nil
	case mode.IsRegular():
		raw, err := os.ReadFile(from)
		if err != nil {
			return err
		}
		return os.WriteFile(to, raw, mode.Perm())
	case mode&os.ModeSymlink != 0:
		target, err := os.Readlink(from)
		if err != nil {
			return err
		}
		return os.Symlink(target, to)
	default:
		return fmt.Errorf("不支持迁移的文件类型: %s", from)
	}
}

type legacyCopy struct {
	to  string
	raw []byte
}

// copyLegacyFiles 把旧目录的已知文件复制到日志等创建的空壳目录。
// 先完整预检，再逐项写入；写入失败会回滚本轮新增目标，旧文件始终不删。
func copyLegacyFiles(oldDir, newDir string) error {
	pairs := append(legacyFilePairs(),
		[2]string{"preferences.json", "preferences.json"},
		[2]string{"config.toml", "config.toml"},
		[2]string{"config.json", "config.json"},
	)
	var copies []legacyCopy
	var errs []error
	for _, pair := range pairs {
		copy, err := prepareLegacyCopy(filepath.Join(oldDir, pair[0]), filepath.Join(newDir, pair[1]))
		if err != nil {
			errs = append(errs, err)
		} else if copy != nil {
			copies = append(copies, *copy)
		}
	}
	backupCopies, err := prepareLegacyBackups(filepath.Join(oldDir, backupDirName), filepath.Join(newDir, backupDirName))
	if err != nil {
		errs = append(errs, err)
	}
	copies = append(copies, backupCopies...)
	if err := errors.Join(errs...); err != nil {
		return err
	}
	var created []string
	for _, copy := range copies {
		if err := atomicfile.WriteBytes(copy.to, copy.raw); err != nil {
			for _, path := range created {
				if removeErr := os.Remove(path); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
					err = errors.Join(err, fmt.Errorf("回滚迁移目标 %s: %w", path, removeErr))
				}
			}
			return fmt.Errorf("写入迁移目标 %s: %w", copy.to, err)
		}
		created = append(created, copy.to)
	}
	return nil
}

func prepareLegacyCopy(from, to string) (*legacyCopy, error) {
	st, err := legacySourceLstat(from)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("检查迁移源 %s: %w", from, err)
	}
	if !st.Mode().IsRegular() {
		return nil, fmt.Errorf("迁移源不是普通文件: %s", from)
	}
	raw, err := os.ReadFile(from)
	if err != nil {
		return nil, fmt.Errorf("读取迁移源 %s: %w", from, err)
	}
	if existing, err := os.ReadFile(to); err == nil {
		if bytes.Equal(existing, raw) {
			return nil, nil
		}
		return nil, fmt.Errorf("迁移目标已存在且内容不同: %s", to)
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("检查迁移目标 %s: %w", to, err)
	}
	return &legacyCopy{to: to, raw: raw}, nil
}

func prepareLegacyBackups(oldBackups, newBackups string) ([]legacyCopy, error) {
	entries, err := os.ReadDir(oldBackups)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("读取旧备份目录: %w", err)
	}
	var copies []legacyCopy
	var errs []error
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasPrefix(name, legacyBackupFilePrefix) || !strings.HasSuffix(name, ".json") {
			continue
		}
		toName := backupFilePrefix + strings.TrimPrefix(name, legacyBackupFilePrefix)
		copy, err := prepareLegacyCopy(filepath.Join(oldBackups, name), filepath.Join(newBackups, toName))
		if err != nil {
			errs = append(errs, err)
		} else if copy != nil {
			copies = append(copies, *copy)
		}
	}
	return copies, errors.Join(errs...)
}

// renameLegacyBackups 把旧备份目录里旧前缀的备份改成新命名放进 newBackups
// （同一路径即目录内改名）。
func renameLegacyBackups(oldBackups, newBackups string) error {
	entries, err := os.ReadDir(oldBackups)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("读取旧备份目录: %w", err)
	}
	var errs []error
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasPrefix(name, legacyBackupFilePrefix) || !strings.HasSuffix(name, ".json") {
			continue
		}
		if err := renameLegacyFile(filepath.Join(oldBackups, name),
			filepath.Join(newBackups, backupFilePrefix+strings.TrimPrefix(name, legacyBackupFilePrefix))); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// renameDataFilesIn 把已复制到暂存目录的文件改成新命名。
func renameDataFilesIn(dir string) error {
	var errs []error
	for _, pair := range legacyFilePairs() {
		if err := renameLegacyFile(filepath.Join(dir, pair[0]), filepath.Join(dir, pair[1])); err != nil {
			errs = append(errs, err)
		}
	}
	if err := renameLegacyBackups(filepath.Join(dir, backupDirName), filepath.Join(dir, backupDirName)); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

func renameLegacyFile(from, to string) error {
	if _, err := os.Lstat(from); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return fmt.Errorf("检查迁移源 %s: %w", from, err)
	}
	if _, err := os.Lstat(to); err == nil {
		return fmt.Errorf("迁移目标已存在: %s", to)
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("检查迁移目标 %s: %w", to, err)
	}
	if err := os.Rename(from, to); err != nil {
		return fmt.Errorf("迁移文件 %s: %w", from, err)
	}
	return nil
}
