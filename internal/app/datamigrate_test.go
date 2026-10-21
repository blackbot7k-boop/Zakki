package app

import (
	"path/filepath"
	"testing"
)

// resolveDataDir 全程 sync.Once 缓存，测试直接驱动纯解析主体。
func TestResolveDataDirAt(t *testing.T) {
	dir, err := resolveDataDirAt(filepath.Join(t.TempDir(), "base"), false)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(dir) != dataDirectoryName {
		t.Fatalf("正式目录 = %q, want %q", dir, dataDirectoryName)
	}
	devDir, err := resolveDataDirAt(filepath.Join(t.TempDir(), "base"), true)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(devDir) != devDataDirectoryName {
		t.Fatalf("开发目录 = %q, want %q", devDir, devDataDirectoryName)
	}
	if _, err := resolveDataDirAt("", false); err == nil {
		t.Fatal("空基础目录应返回错误")
	}
}
