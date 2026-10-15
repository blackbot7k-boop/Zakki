package app

// 数据目录一次性迁移：应用更名（Tot MD → Zakki）后，把旧目录整体迁到新目录，
// 数据与备份文件按新命名规则改名。只认旧命名模式的文件，其余原样保留。

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/egoist/mygo"
)

const (
	// dataDirectoryName 数据目录名；旧安装的目录名是 legacyDataDirectoryName，
	// 首次启动时整体迁移过来。
	dataDirectoryName       = "Zakki"
	legacyDataDirectoryName = "Tot MD"
	dataFileName            = "zakki.json"
	legacyDataFileName      = "tot-md.json"
	legacyDataPendingSuffix = ".pending"
	backupDirName           = "backups"
	legacyBackupFilePrefix  = "tot-md-"
	backupFilePrefix        = "zakki-"
)

var migrateDataDirOnce sync.Once

// mygoAppDataBase 返回系统应用数据根目录；测试可替换。
var mygoAppDataBase = func() (string, error) { return mygo.App.Path(mygo.PathAppData) }

// dataDir 返回数据目录（绝对路径），并保证旧目录已迁移。
func dataDir() string {
	base, err := mygoAppDataBase()
	if err != nil || base == "" {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, "Library", "Application Support")
	}
	dir := filepath.Join(base, dataDirectoryName)
	migrateDataDirOnce.Do(func() {
		if err := migrateDataDir(base); err != nil {
			slog.Warn("旧数据目录迁移未完成（旧目录保持原样，不影响新目录使用）",
				"from", filepath.Join(base, legacyDataDirectoryName), "error", err)
		}
	})
	return dir
}

// migrateDataDir 把 <base>/Tot MD 迁到 <base>/Zakki：
//   - 新目录不存在：整个目录改名（同卷原子），再按新命名规则改内部文件名；
//   - 新目录已存在且为空壳（无 zakki.json / preferences.json）：逐文件搬入后保留旧目录；
//   - 新目录已有数据：不动旧目录，等待人工处理。
//
// 返回 error 只代表迁移没完成，不影响应用用新目录继续运行。
func migrateDataDir(base string) error {
	oldDir := filepath.Join(base, legacyDataDirectoryName)
	newDir := filepath.Join(base, dataDirectoryName)
	if oldDir == newDir {
		return nil
	}
	st, err := os.Lstat(oldDir)
	if err != nil {
		return nil // 没有旧目录：全新安装或已迁移
	}
	if !st.IsDir() {
		return nil // 旧路径被文件占用：不碰它
	}
	if _, err := os.Lstat(newDir); err != nil {
		if err := os.Rename(oldDir, newDir); err != nil {
			return err
		}
		renameDataFilesIn(newDir)
		return nil
	}
	// 两目录并存：仅当新目录还没有任何数据文件时逐文件搬入。
	if newDataExists(newDir) {
		return nil
	}
	moveLegacyFiles(oldDir, newDir)
	return nil
}

// newDataExists 报告目录里是否已有新版命名或沿用旧版同名的数据/偏好文件。
func newDataExists(dir string) bool {
	for _, name := range []string{dataFileName, legacyDataFileName, "preferences.json"} {
		if _, err := os.Lstat(filepath.Join(dir, name)); err == nil {
			return true
		}
	}
	return false
}

// moveLegacyFiles 把旧目录中的数据文件、偏好与备份搬进新目录并改用新命名。
// 单个文件失败只记日志，继续处理其余文件；旧目录保留（用户可自行删除）。
func moveLegacyFiles(oldDir, newDir string) {
	for _, pair := range [][2]string{
		{legacyDataFileName, dataFileName},
		{legacyDataFileName + legacyDataPendingSuffix, dataFileName + legacyDataPendingSuffix},
		{"preferences.json", "preferences.json"},
	} {
		renameOrLog(filepath.Join(oldDir, pair[0]), filepath.Join(newDir, pair[1]))
	}
	oldBackups := filepath.Join(oldDir, backupDirName)
	entries, err := os.ReadDir(oldBackups)
	if err != nil {
		return
	}
	newBackups := filepath.Join(newDir, backupDirName)
	if err := os.MkdirAll(newBackups, 0o755); err != nil {
		slog.Warn("迁移备份目录失败", "dir", newBackups, "error", err)
		return
	}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasPrefix(name, legacyBackupFilePrefix) || !strings.HasSuffix(name, ".json") {
			continue
		}
		renameOrLog(filepath.Join(oldBackups, name),
			filepath.Join(newBackups, backupFilePrefix+strings.TrimPrefix(name, legacyBackupFilePrefix)))
	}
}

// renameDataFilesIn 把已整体改名的目录内部文件改成新命名。
func renameDataFilesIn(dir string) {
	for _, pair := range [][2]string{
		{legacyDataFileName, dataFileName},
		{legacyDataFileName + legacyDataPendingSuffix, dataFileName + legacyDataPendingSuffix},
	} {
		renameOrLog(filepath.Join(dir, pair[0]), filepath.Join(dir, pair[1]))
	}
	backups := filepath.Join(dir, backupDirName)
	entries, err := os.ReadDir(backups)
	if err != nil {
		return
	}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasPrefix(name, legacyBackupFilePrefix) || !strings.HasSuffix(name, ".json") {
			continue
		}
		renameOrLog(filepath.Join(backups, name),
			filepath.Join(backups, backupFilePrefix+strings.TrimPrefix(name, legacyBackupFilePrefix)))
	}
}

func renameOrLog(from, to string) {
	if _, err := os.Lstat(from); err != nil {
		return // 源不存在，无需处理
	}
	if _, err := os.Lstat(to); err == nil {
		slog.Warn("迁移跳过已存在的目标文件", "from", from, "to", to)
		return
	}
	if err := os.Rename(from, to); err != nil {
		slog.Warn("迁移文件失败", "from", from, "to", to, "error", err)
	}
}
