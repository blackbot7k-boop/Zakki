package notes

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
	"time"
)

func TestBackupNameOrderUsesNumericSequence(t *testing.T) {
	names := []string{
		"zakki-20240601-120000-10.json",
		"tot-md-20240601-120000-9.json",
		"zakki-20240601-120000.json",
		"zakki-20240531-235959.json",
	}
	sortBackupNames(names)
	want := []string{
		"zakki-20240601-120000-10.json",
		"tot-md-20240601-120000-9.json",
		"zakki-20240601-120000.json",
		"zakki-20240531-235959.json",
	}
	if !reflect.DeepEqual(names, want) {
		t.Fatalf("备份排序=%v，要 %v", names, want)
	}
}

func sortBackupNames(names []string) {
	sort.Slice(names, func(i, j int) bool { return backupNameNewer(names[i], names[j]) })
}

func TestSameSecondBackupPruningKeepsNumericNewest(t *testing.T) {
	clk := newTestClock()
	s := newTestStore(t, clk.now)
	if err := s.SaveNow(File{Version: CurrentDataVersion, Tabs: defaultTabs("同秒备份"), Settings: Settings{HideOnEscape: true}}); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 11; i++ {
		if err := s.BackupNow(1); err != nil {
			t.Fatal(err)
		}
	}
	if got, want := backupNames(t, s.BackupDir()), []string{"zakki-20240601-120000-11.json"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("同秒第 11 份后的保留结果=%v，要 %v", got, want)
	}
}

// 备份文件夹是唯一存储地址：选择后备份直写该文件夹，内置目录不再产生新备份；
// 断开后回到内置目录。
func TestSetBackupDirSingleDestination(t *testing.T) {
	clk := newTestClock()
	s := newTestStore(t, clk.now)
	dest := filepath.Join(filepath.Dir(s.DataFile), "cloud")
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveNow(File{Version: CurrentDataVersion, Tabs: defaultTabs("单一目的地"), Settings: Settings{HideOnEscape: true}}); err != nil {
		t.Fatal(err)
	}
	s.SetBackupDir(dest)
	if got := s.BackupDir(); got != dest {
		t.Fatalf("BackupDir() = %q, want %q", got, dest)
	}
	if err := s.BackupNow(2); err != nil {
		t.Fatal(err)
	}
	if got := backupNames(t, dest); len(got) != 1 {
		t.Fatalf("备份应直写所选文件夹: %v", got)
	}
	if got := backupNames(t, s.DefaultBackupDir); len(got) != 0 {
		t.Fatalf("选择后内置目录不应再写入: %v", got)
	}

	// 保留策略在所选文件夹生效。
	for i := 0; i < 2; i++ {
		clk.advance(time.Second)
		if err := s.BackupNow(2); err != nil {
			t.Fatal(err)
		}
	}
	if got := backupNames(t, dest); len(got) != 2 {
		t.Fatalf("所选文件夹应按 keep 修剪: %v", got)
	}

	// 断开后回到内置目录。
	s.SetBackupDir("")
	clk.advance(time.Second)
	if err := s.BackupNow(2); err != nil {
		t.Fatal(err)
	}
	if got := backupNames(t, s.DefaultBackupDir); len(got) != 1 {
		t.Fatalf("断开后应回到内置目录: %v", got)
	}
}

// 切换备份文件夹后，新备份只写新文件夹，旧文件夹保持原样。
func TestSetBackupDirSwitchLeavesOldDestAlone(t *testing.T) {
	clk := newTestClock()
	s := newTestStore(t, clk.now)
	if err := s.SaveNow(File{Version: CurrentDataVersion, Tabs: defaultTabs("切换"), Settings: Settings{HideOnEscape: true}}); err != nil {
		t.Fatal(err)
	}
	old := filepath.Join(filepath.Dir(s.DataFile), "old")
	if err := os.MkdirAll(old, 0o755); err != nil {
		t.Fatal(err)
	}
	s.SetBackupDir(old)
	if err := s.BackupNow(20); err != nil {
		t.Fatal(err)
	}
	before := backupNames(t, old)

	fresh := filepath.Join(filepath.Dir(s.DataFile), "fresh")
	if err := os.MkdirAll(fresh, 0o755); err != nil {
		t.Fatal(err)
	}
	s.SetBackupDir(fresh)
	clk.advance(time.Second)
	if err := s.BackupNow(20); err != nil {
		t.Fatal(err)
	}
	if got := backupNames(t, old); !reflect.DeepEqual(got, before) {
		t.Fatalf("旧文件夹应保持原样: got %v, want %v", got, before)
	}
	if got := backupNames(t, fresh); len(got) != 1 {
		t.Fatalf("新备份应写进新文件夹: %v", got)
	}
}

// LoadNewestBackup 先看备份文件夹；里面没有可用备份时回退内置目录
// （历史版本的备份都在那里，换文件夹后首次损坏仍可恢复）。
func TestLoadNewestBackupFallsBackToDefaultDir(t *testing.T) {
	s := newTestStore(t, nil)
	if err := s.SaveNow(File{Version: CurrentDataVersion, Tabs: defaultTabs("恢复"), Settings: Settings{HideOnEscape: true}}); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(s.DataFile)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(s.DefaultBackupDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(s.DefaultBackupDir, "zakki-20240601-110000.json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}

	cloud := filepath.Join(filepath.Dir(s.DataFile), "cloud")
	if err := os.MkdirAll(cloud, 0o755); err != nil {
		t.Fatal(err)
	}
	s.SetBackupDir(cloud)
	if _, _, ok := s.LoadNewestBackup(); !ok {
		t.Fatal("备份文件夹为空时应回退内置目录")
	}
	// 备份文件夹里有可用备份后不再回退。
	if err := os.WriteFile(filepath.Join(cloud, "zakki-20240601-120000.json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	f, name, ok := s.LoadNewestBackup()
	if !ok || name != "zakki-20240601-120000.json" {
		t.Fatalf("应取备份文件夹里的最新一份: %q %q %v", name, f.Tabs[0].Text, ok)
	}
}

// 最近备份状态只认本应用的备份命名：用户自选文件夹里混入的其他文件
// 不得记成备份时间。
func TestLoadLastBackupTimeIgnoresStrayFiles(t *testing.T) {
	clk := newTestClock()
	s := newTestStore(t, clk.now)
	dest := filepath.Join(filepath.Dir(s.DataFile), "cloud")
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatal(err)
	}
	writeRaw(t, filepath.Join(dest, "随手放的笔记.txt"), "not a backup")
	s.SetBackupDir(dest)
	if err := s.LoadLastBackupTime(); err != nil {
		t.Fatal(err)
	}
	if got := s.LastBackupTime(); !got.IsZero() {
		t.Fatalf("无关文件不应记为备份时间: %v", got)
	}
	writeRaw(t, filepath.Join(dest, "zakki-20240601-120000.json"), "{}")
	// 新实例重新扫描：backupLoaded 只在首次扫描后置位。
	s2 := NewStore(s.DataFile, s.DefaultBackupDir, clk.now, testMigrateOpt())
	s2.SetBackupDir(dest)
	if err := s2.LoadLastBackupTime(); err != nil {
		t.Fatal(err)
	}
	if s2.LastBackupTime().IsZero() {
		t.Fatal("备份文件应记为最近备份时间")
	}
}
