package app

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func mustExist(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Lstat(path); err != nil {
		t.Fatalf("%s 应存在: %v", path, err)
	}
}

func mustNotExist(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Lstat(path); err == nil {
		t.Fatalf("%s 不应存在", path)
	}
}

// 旧目录整体改名：内部数据、暂存与备份文件全部换用新命名，旧目录消失。
func TestMigrateDataDirRenamesWholeDirectory(t *testing.T) {
	base := t.TempDir()
	oldDir := filepath.Join(base, legacyDataDirectoryName)
	writeFile(t, filepath.Join(oldDir, legacyDataFileName), `{"version":2}`)
	writeFile(t, filepath.Join(oldDir, legacyDataFileName+legacyDataPendingSuffix), `{"version":2}`)
	writeFile(t, filepath.Join(oldDir, "preferences.json"), `{"version":3}`)
	writeFile(t, filepath.Join(oldDir, backupDirName, "tot-md-20240601-120000.json"), "{}")
	writeFile(t, filepath.Join(oldDir, backupDirName, "tot-md-20240601-120001.json"), "{}")
	writeFile(t, filepath.Join(oldDir, backupDirName, "other.txt"), "x")

	if err := migrateDataDir(base); err != nil {
		t.Fatal(err)
	}
	newDir := filepath.Join(base, dataDirectoryName)
	mustExist(t, filepath.Join(newDir, dataFileName))
	mustExist(t, filepath.Join(newDir, dataFileName+legacyDataPendingSuffix))
	mustExist(t, filepath.Join(newDir, "preferences.json"))
	mustExist(t, filepath.Join(newDir, backupDirName, "zakki-20240601-120000.json"))
	mustExist(t, filepath.Join(newDir, backupDirName, "zakki-20240601-120001.json"))
	mustExist(t, filepath.Join(newDir, backupDirName, "other.txt"))
	mustNotExist(t, oldDir)
	// 再跑一次应是无操作。
	if err := migrateDataDir(base); err != nil {
		t.Fatal(err)
	}
	mustExist(t, filepath.Join(newDir, dataFileName))
}

// 无旧目录（全新安装）：不动文件系统。
func TestMigrateDataDirWithoutLegacyDirectory(t *testing.T) {
	base := t.TempDir()
	if err := migrateDataDir(base); err != nil {
		t.Fatal(err)
	}
	mustNotExist(t, filepath.Join(base, dataDirectoryName))
}

// 新目录已有数据：不迁移、不覆盖，旧目录原样保留等人工处理。
func TestMigrateDataDirKeepsBothWhenNewHasData(t *testing.T) {
	base := t.TempDir()
	oldDir := filepath.Join(base, legacyDataDirectoryName)
	newDir := filepath.Join(base, dataDirectoryName)
	writeFile(t, filepath.Join(oldDir, legacyDataFileName), `{"version":2}`)
	writeFile(t, filepath.Join(newDir, dataFileName), `{"version":3}`)

	if err := migrateDataDir(base); err != nil {
		t.Fatal(err)
	}
	mustExist(t, filepath.Join(oldDir, legacyDataFileName))
	mustNotExist(t, filepath.Join(newDir, legacyDataFileName))
	if raw, err := os.ReadFile(filepath.Join(newDir, dataFileName)); err != nil || string(raw) != `{"version":3}` {
		t.Fatalf("新目录数据被改动: %q err=%v", raw, err)
	}
}

// 新目录只是空壳（无任何数据/偏好文件）：逐文件搬入并改名，旧目录保留。
func TestMigrateDataDirMovesIntoEmptyNewDirectory(t *testing.T) {
	base := t.TempDir()
	oldDir := filepath.Join(base, legacyDataDirectoryName)
	newDir := filepath.Join(base, dataDirectoryName)
	writeFile(t, filepath.Join(oldDir, legacyDataFileName), `{"version":2}`)
	writeFile(t, filepath.Join(oldDir, "preferences.json"), `{"version":3}`)
	writeFile(t, filepath.Join(oldDir, backupDirName, "tot-md-20240601-120000.json"), "{}")
	if err := os.MkdirAll(newDir, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := migrateDataDir(base); err != nil {
		t.Fatal(err)
	}
	mustExist(t, filepath.Join(newDir, dataFileName))
	mustExist(t, filepath.Join(newDir, "preferences.json"))
	mustExist(t, filepath.Join(newDir, backupDirName, "zakki-20240601-120000.json"))
	mustNotExist(t, filepath.Join(oldDir, legacyDataFileName))
	mustNotExist(t, filepath.Join(newDir, legacyDataFileName))
}

// 旧路径被文件占用时按无旧目录处理。
func TestMigrateDataDirIgnoresFileAtLegacyPath(t *testing.T) {
	base := t.TempDir()
	writeFile(t, filepath.Join(base, legacyDataDirectoryName), "not a dir")
	if err := migrateDataDir(base); err != nil {
		t.Fatal(err)
	}
	mustExist(t, filepath.Join(base, legacyDataDirectoryName))
	mustNotExist(t, filepath.Join(base, dataDirectoryName))
}
