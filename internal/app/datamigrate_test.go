package app

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
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
	writeFile(t, filepath.Join(oldDir, "config.toml"), "[appearance]\nbodySize = 18\n")
	writeFile(t, filepath.Join(oldDir, "config.json"), `{"appearance":{"bodySize":17}}`)
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
	mustExist(t, filepath.Join(newDir, "config.toml"))
	mustExist(t, filepath.Join(newDir, "config.json"))
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

func TestMigrateDataDirReportsLegacyProbeFailure(t *testing.T) {
	probeErr := errors.New("probe denied")
	original := legacyDataDirLstat
	legacyDataDirLstat = func(string) (os.FileInfo, error) { return nil, probeErr }
	t.Cleanup(func() { legacyDataDirLstat = original })

	err := migrateDataDir(t.TempDir())
	if !errors.Is(err, probeErr) {
		t.Fatalf("旧目录探测失败必须阻止迁移后继续启动: %v", err)
	}
}

func TestMigrateDataDirReportsLegacySourceProbeFailure(t *testing.T) {
	base := t.TempDir()
	oldDir := filepath.Join(base, legacyDataDirectoryName)
	newDir := filepath.Join(base, dataDirectoryName)
	source := filepath.Join(oldDir, legacyDataFileName)
	writeFile(t, source, `{"version":2}`)
	if err := os.MkdirAll(newDir, 0o755); err != nil {
		t.Fatal(err)
	}

	probeErr := errors.New("source probe denied")
	original := legacySourceLstat
	legacySourceLstat = func(path string) (os.FileInfo, error) {
		if path == source {
			return nil, probeErr
		}
		return os.Lstat(path)
	}
	t.Cleanup(func() { legacySourceLstat = original })

	err := migrateDataDir(base)
	if !errors.Is(err, probeErr) {
		t.Fatalf("legacy 源文件探测失败必须阻止迁移: %v", err)
	}
	mustExist(t, source)
	mustNotExist(t, filepath.Join(newDir, dataFileName))
}

func TestMigrateDataDirReportsNewDataProbeFailure(t *testing.T) {
	base := t.TempDir()
	oldDir := filepath.Join(base, legacyDataDirectoryName)
	newDir := filepath.Join(base, dataDirectoryName)
	writeFile(t, filepath.Join(oldDir, legacyDataFileName), `{"version":2}`)
	if err := os.MkdirAll(newDir, 0o755); err != nil {
		t.Fatal(err)
	}

	probePath := filepath.Join(newDir, dataFileName)
	probeErr := errors.New("destination probe denied")
	original := newDataPathLstat
	newDataPathLstat = func(path string) (os.FileInfo, error) {
		if path == probePath {
			return nil, probeErr
		}
		return os.Lstat(path)
	}
	t.Cleanup(func() { newDataPathLstat = original })

	err := migrateDataDir(base)
	if !errors.Is(err, probeErr) {
		t.Fatalf("新目录探测失败必须阻止迁移: %v", err)
	}
	mustExist(t, filepath.Join(oldDir, legacyDataFileName))
	mustNotExist(t, probePath)
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

// 新目录只是空壳（无任何数据/偏好文件）：整批复制并改名，旧目录保留供人工核对和失败重试。
func TestMigrateDataDirMovesIntoEmptyNewDirectory(t *testing.T) {
	base := t.TempDir()
	oldDir := filepath.Join(base, legacyDataDirectoryName)
	newDir := filepath.Join(base, dataDirectoryName)
	writeFile(t, filepath.Join(oldDir, legacyDataFileName), `{"version":2}`)
	writeFile(t, filepath.Join(oldDir, "preferences.json"), `{"version":3}`)
	writeFile(t, filepath.Join(oldDir, "config.toml"), "[appearance]\nbodySize = 18\n")
	writeFile(t, filepath.Join(oldDir, "config.json"), `{"appearance":{"bodySize":17}}`)
	writeFile(t, filepath.Join(oldDir, backupDirName, "tot-md-20240601-120000.json"), "{}")
	if err := os.MkdirAll(newDir, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := migrateDataDir(base); err != nil {
		t.Fatal(err)
	}
	mustExist(t, filepath.Join(newDir, dataFileName))
	mustExist(t, filepath.Join(newDir, "preferences.json"))
	mustExist(t, filepath.Join(newDir, "config.toml"))
	mustExist(t, filepath.Join(newDir, "config.json"))
	mustExist(t, filepath.Join(newDir, backupDirName, "zakki-20240601-120000.json"))
	mustExist(t, filepath.Join(oldDir, legacyDataFileName))
	mustExist(t, filepath.Join(oldDir, "config.toml"))
	mustExist(t, filepath.Join(oldDir, "config.json"))
	mustNotExist(t, filepath.Join(newDir, legacyDataFileName))
}

// 旧路径被文件占用时必须报错，不能静默显示一套空白数据。
func TestMigrateDataDirRejectsFileAtLegacyPath(t *testing.T) {
	base := t.TempDir()
	oldPath := filepath.Join(base, legacyDataDirectoryName)
	writeFile(t, oldPath, "not a dir")
	if err := migrateDataDir(base); err == nil {
		t.Fatal("旧数据路径不是目录时必须阻止启动")
	}
	mustExist(t, oldPath)
	mustNotExist(t, filepath.Join(base, dataDirectoryName))
}

func TestMigrateDataDirReportsDestinationProbeFailure(t *testing.T) {
	base := t.TempDir()
	oldDir := filepath.Join(base, legacyDataDirectoryName)
	newDir := filepath.Join(base, dataDirectoryName)
	writeFile(t, filepath.Join(oldDir, legacyDataFileName), `{"version":1}`)
	if err := os.MkdirAll(newDir, 0o755); err != nil {
		t.Fatal(err)
	}
	probeErr := errors.New("destination probe denied")
	original := newDataPathLstat
	newDataPathLstat = func(path string) (os.FileInfo, error) {
		if path == filepath.Join(newDir, dataFileName) {
			return nil, probeErr
		}
		return os.Lstat(path)
	}
	t.Cleanup(func() { newDataPathLstat = original })

	err := migrateDataDir(base)
	if !errors.Is(err, probeErr) {
		t.Fatalf("目标探测失败未上报: %v", err)
	}
	mustExist(t, filepath.Join(oldDir, legacyDataFileName))
	mustNotExist(t, filepath.Join(newDir, dataFileName))
}

func TestMigrateDataDirFollowsRootSymlinkAndPreservesOriginal(t *testing.T) {
	base := t.TempDir()
	target := filepath.Join(base, "legacy-target")
	oldDir := filepath.Join(base, legacyDataDirectoryName)
	newDir := filepath.Join(base, dataDirectoryName)
	source := filepath.Join(target, legacyDataFileName)
	writeFile(t, source, `{"version":2}`)
	if err := os.Symlink(target, oldDir); err != nil {
		t.Fatal(err)
	}

	if err := migrateDataDir(base); err != nil {
		t.Fatal(err)
	}
	if got := string(persistReadRaw(t, filepath.Join(newDir, dataFileName))); got != `{"version":2}` {
		t.Fatalf("符号链接目录迁移内容=%q", got)
	}
	if st, err := os.Lstat(oldDir); err != nil || st.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("旧目录链接必须保留: mode=%v err=%v", st, err)
	}
	mustExist(t, source)
}

func TestMigrateDataDirPartialFailurePreservesSourcesAndRetries(t *testing.T) {
	base := t.TempDir()
	oldDir := filepath.Join(base, legacyDataDirectoryName)
	newDir := filepath.Join(base, dataDirectoryName)
	writeFile(t, filepath.Join(oldDir, legacyDataFileName), `{"version":2}`)
	sentinel := filepath.Join(base, "sentinel.toml")
	writeFile(t, sentinel, "preserve")
	if err := os.Symlink(sentinel, filepath.Join(oldDir, "config.toml")); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(newDir, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := migrateDataDir(base); err == nil {
		t.Fatal("非普通 legacy 配置必须让整批迁移失败")
	}
	mustExist(t, filepath.Join(oldDir, legacyDataFileName))
	mustNotExist(t, filepath.Join(newDir, dataFileName))
	if raw, err := os.ReadFile(sentinel); err != nil || string(raw) != "preserve" {
		t.Fatalf("迁移不得跟随或改写 symlink 目标: %q, %v", raw, err)
	}

	if err := os.Remove(filepath.Join(oldDir, "config.toml")); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(oldDir, "config.toml"), "[appearance]\nbodySize = 18\n")
	if err := migrateDataDir(base); err != nil {
		t.Fatalf("修复源文件后应可重试: %v", err)
	}
	mustExist(t, filepath.Join(newDir, dataFileName))
	mustExist(t, filepath.Join(newDir, "config.toml"))
	mustExist(t, filepath.Join(oldDir, legacyDataFileName))
}

func TestResolveDataDirCachesMigrationErrorAndNextProcessRetries(t *testing.T) {
	base := t.TempDir()
	oldDir := filepath.Join(base, legacyDataDirectoryName)
	newDir := filepath.Join(base, dataDirectoryName)
	writeFile(t, filepath.Join(oldDir, legacyDataFileName), `{"version":2}`)
	if err := os.Symlink(filepath.Join(base, "missing"), filepath.Join(oldDir, "config.toml")); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(newDir, 0o755); err != nil {
		t.Fatal(err)
	}

	oldBase := mygoAppDataBase
	oldOnce, oldDirCache, oldErrCache := migrateDataDirOnce, resolvedDataDir, resolvedDataDirErr
	t.Cleanup(func() {
		mygoAppDataBase = oldBase
		migrateDataDirOnce, resolvedDataDir, resolvedDataDirErr = oldOnce, oldDirCache, oldErrCache
	})
	mygoAppDataBase = func() (string, error) { return base, nil }
	migrateDataDirOnce, resolvedDataDir, resolvedDataDirErr = new(sync.Once), "", nil

	dir, firstErr := resolveDataDir()
	if dir != newDir || firstErr == nil {
		t.Fatalf("目录和迁移错误必须同时可见: dir=%q err=%v", dir, firstErr)
	}
	startupDir, gotApp, startupErr := startupApp()
	if startupDir != newDir || startupErr == nil || gotApp != nil {
		t.Fatalf("启动必须在 Store 构造前停止: dir=%q app=%v err=%v", startupDir, gotApp, startupErr)
	}
	if err := os.Remove(filepath.Join(oldDir, "config.toml")); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(oldDir, "config.toml"), "[appearance]\nbodySize = 18\n")
	if _, err := resolveDataDir(); err == nil || err.Error() != firstErr.Error() {
		t.Fatalf("同一进程必须稳定返回缓存错误: first=%v second=%v", firstErr, err)
	}

	// 新进程会重新初始化缓存，因此能够在源问题修复后重试。
	migrateDataDirOnce, resolvedDataDir, resolvedDataDirErr = new(sync.Once), "", nil
	if _, err := resolveDataDir(); err != nil {
		t.Fatalf("下一次进程启动应重试成功: %v", err)
	}
	mustExist(t, filepath.Join(newDir, dataFileName))
	mustExist(t, filepath.Join(newDir, "config.toml"))
}

func TestNewTestAppBypassesGlobalDataDirectoryResolution(t *testing.T) {
	base := t.TempDir()
	oldDir := filepath.Join(base, legacyDataDirectoryName)
	writeFile(t, filepath.Join(oldDir, legacyDataFileName), `{"version":2}`)

	oldBase := mygoAppDataBase
	oldOnce, oldDirCache, oldErrCache := migrateDataDirOnce, resolvedDataDir, resolvedDataDirErr
	t.Cleanup(func() {
		mygoAppDataBase = oldBase
		migrateDataDirOnce, resolvedDataDir, resolvedDataDirErr = oldOnce, oldDirCache, oldErrCache
	})
	probes := 0
	mygoAppDataBase = func() (string, error) {
		probes++
		return base, nil
	}
	migrateDataDirOnce, resolvedDataDir, resolvedDataDirErr = new(sync.Once), "", nil

	a := newTestApp(t)
	if probes != 0 {
		t.Fatalf("newTestApp 不得探测全局目录，实际调用 %d 次", probes)
	}
	if filepath.Dir(a.notes.DataFile) == filepath.Join(base, dataDirectoryName) {
		t.Fatal("测试 Store 不得落入全局数据目录")
	}
	mustExist(t, filepath.Join(oldDir, legacyDataFileName))
	mustNotExist(t, filepath.Join(base, dataDirectoryName))
}
