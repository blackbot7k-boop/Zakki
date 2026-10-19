package app

// 备份文件夹：目录校验、设置页展示。备份文件夹是唯一存储地址。

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestValidateBackupDir(t *testing.T) {
	a := newTestApp(t)
	dataDir := filepath.Dir(a.notes.DataFile)

	// 应用数据目录及其子目录（含内置 backups/）拒绝：不能把笔记数据
	// 自我嵌套进备份。
	if _, err := a.validateBackupDir(dataDir); err == nil {
		t.Fatal("应用数据目录应被拒绝")
	}
	if _, err := a.validateBackupDir(a.notes.BackupDir()); err == nil {
		t.Fatal("内置备份目录（应用数据子目录）应被拒绝")
	}
	if _, err := a.validateBackupDir(filepath.Join(dataDir, "elsewhere", "bak")); err == nil {
		t.Fatal("应用数据目录的子目录应被拒绝")
	}
	// 同级外部目录通过并返回绝对路径。
	ok := filepath.Join(filepath.Dir(dataDir), "external", "bak")
	if err := os.MkdirAll(ok, 0o755); err != nil {
		t.Fatal(err)
	}
	abs, err := a.validateBackupDir(ok)
	if err != nil {
		t.Fatalf("正常目录不应拒绝: %v", err)
	}
	wantOK, err := canonicalPath(ok)
	if err != nil {
		t.Fatal(err)
	}
	if abs != wantOK {
		t.Fatalf("应返回规范绝对路径 %q, got %q", wantOK, abs)
	}
	// 不可写目录拒绝（用文件冒充目录）。
	blocker := filepath.Join(dataDir, "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := a.validateBackupDir(filepath.Join(blocker, "sub")); err == nil {
		t.Fatal("不可写路径应被拒绝")
	}
}

func TestValidateBackupDirRejectsSymlinkIntoDataDir(t *testing.T) {
	a := newTestApp(t)
	dataDir := filepath.Dir(a.notes.DataFile)
	link := filepath.Join(dataDir, "backup-link")
	_ = os.Remove(link)
	if err := os.Symlink(dataDir, link); err != nil {
		t.Skip("当前环境无法创建符号链接:", err)
	}
	t.Cleanup(func() { _ = os.Remove(link) })
	if _, err := a.validateBackupDir(link); err == nil {
		t.Fatal("指向应用数据目录的符号链接应被拒绝")
	}
}

func TestValidateBackupDirDoesNotTouchPreexistingProbeFile(t *testing.T) {
	a := newTestApp(t)
	dir := filepath.Join(filepath.Dir(filepath.Dir(a.notes.DataFile)), "probe-file")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	probe := filepath.Join(dir, ".zakki-backup-probe")
	want := []byte("must survive")
	if err := os.WriteFile(probe, want, 0o640); err != nil {
		t.Fatal(err)
	}
	if _, err := a.validateBackupDir(dir); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(probe)
	if err != nil {
		t.Fatalf("预存探针文件不得被删除: %v", err)
	}
	if string(got) != string(want) {
		t.Fatalf("预存探针文件不得被截断: %q", got)
	}
}

func TestValidateBackupDirDoesNotFollowPreexistingProbeSymlink(t *testing.T) {
	a := newTestApp(t)
	dir := filepath.Join(filepath.Dir(filepath.Dir(a.notes.DataFile)), "probe-link")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(filepath.Dir(dir), "probe-target")
	want := []byte("target must survive")
	if err := os.WriteFile(target, want, 0o640); err != nil {
		t.Fatal(err)
	}
	probe := filepath.Join(dir, ".zakki-backup-probe")
	if err := os.Symlink(target, probe); err != nil {
		t.Skip("当前环境无法创建符号链接:", err)
	}
	if _, err := a.validateBackupDir(dir); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(target)
	if err != nil || string(got) != string(want) {
		t.Fatalf("探针不得跟随 symlink 改写目标: %q, %v", got, err)
	}
	info, err := os.Lstat(probe)
	if err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("预存探针 symlink 不得被删除或替换: info=%v err=%v", info, err)
	}
}

func TestApplyBackupDirWritesPreference(t *testing.T) {
	a := newTestApp(t)
	base := filepath.Dir(filepath.Dir(a.notes.DataFile))

	// 合法目录：写入偏好并同步草稿。
	dir := filepath.Join(base, "chosen")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	a.applyBackupDir(dir)
	want, err := canonicalPath(dir)
	if err != nil {
		t.Fatal(err)
	}
	if a.prefs.Backup.BackupDir != want {
		t.Fatalf("备份文件夹 = %q, want %q", a.prefs.Backup.BackupDir, want)
	}
	if a.notes.BackupDir() != want {
		t.Fatalf("运行时备份目的地应跟随偏好: %q", a.notes.BackupDir())
	}
	// 数据目录被拒绝且偏好不变。
	a.applyBackupDir(filepath.Dir(a.notes.DataFile))
	if a.prefs.Backup.BackupDir != want {
		t.Fatal("非法目录不应改动偏好")
	}
	if a.settingsUI.keyErr == "" {
		t.Fatal("非法目录应提示错误")
	}
	// 断开：回到内置目录。
	a.disconnectBackupFolder()
	if a.prefs.Backup.BackupDir != "" {
		t.Fatalf("断开后应清空备份文件夹: %q", a.prefs.Backup.BackupDir)
	}
	if a.notes.BackupDir() != a.notes.DefaultBackupDir {
		t.Fatalf("断开后应回到内置目录: %q", a.notes.BackupDir())
	}
}

func TestSettingsBackupPageShowsFolder(t *testing.T) {
	a := newTestApp(t)
	a.winOpen = true
	chosen := filepath.Join(filepath.Dir(filepath.Dir(a.notes.DataFile)), "page-folder")
	if err := os.MkdirAll(chosen, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := a.updatePreferences(func(p *preferences) {
		p.Backup.BackupDir = chosen
	}); err != nil {
		t.Fatal(err)
	}
	a.settingsPage = pageBackup
	a.settingsUI.syncDraftFromPrefs(a.prefs)
	tt := ui.NewTester(a.settingsView, 760, 600)
	tt.Frame()
	for _, want := range []string{chosen, "打开文件夹", "更换…", "断开"} {
		if !tt.HasText(want) {
			t.Fatalf("备份页缺少 %q: %v", want, tt.Texts())
		}
	}
	// 未选择时显示内置目录与选择入口，无断开。
	if err := a.updatePreferences(func(p *preferences) {
		p.Backup.BackupDir = ""
	}); err != nil {
		t.Fatal(err)
	}
	a.settingsUI.syncDraftFromPrefs(a.prefs)
	tt2 := ui.NewTester(a.settingsView, 760, 600)
	tt2.Frame()
	if !tt2.HasText(a.notes.BackupDir()) || !tt2.HasText("选择文件夹…") {
		t.Fatalf("未选择时应显示内置目录与选择入口: %v", tt2.Texts())
	}
	if tt2.HasText("断开") {
		t.Fatalf("未选择时不应有断开: %v", tt2.Texts())
	}
}
