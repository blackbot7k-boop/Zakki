package app

// 备份页双形态：备份文件夹探测到 .git 时出现 Git 区块；开关与推送间隔经
// 草稿应用到偏好与 Store。普通文件夹不出现 Git 区块。

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestBackupPageGitSectionOnlyForRepo(t *testing.T) {
	a := newTestApp(t)
	// 内置目录不是仓库：不出现 Git 区块，未选择时说明当前写入位置。
	a.settingsPage = pageBackup
	a.settingsUI.syncDraftFromPrefs(a.prefs)
	tt := ui.NewTester(a.settingsView, 900, 620)
	tt.Frame()
	if tt.HasText("Git 仓库") {
		t.Fatalf("普通文件夹不应出现 Git 区块: %v", tt.Texts())
	}
	if !tt.HasText("未连接文件夹，当前备份写入：") {
		t.Fatalf("未选择时应显示内置目录说明: %v", tt.Texts())
	}
	if !tt.HasText("数据文件") || !tt.HasText("在访达中显示") {
		t.Fatalf("数据文件区块文案缺失: %v", tt.Texts())
	}

	// 换成 git 仓库目录：出现 Git 区块、开关与推送间隔。
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := a.updatePreferences(func(p *preferences) { p.Backup.BackupDir = repo }); err != nil {
		t.Fatal(err)
	}
	tt = ui.NewTester(a.settingsView, 900, 620)
	tt.Frame()
	if !tt.HasText("Git 仓库") || !tt.HasText("自动提交与推送") {
		t.Fatalf("仓库文件夹应出现 Git 区块: %v", tt.Texts())
	}
	// 自动推送默认关闭：只显示开关，打开后才出现推送间隔。
	if tt.HasText("推送间隔（分钟）") {
		t.Fatalf("默认关闭时不应显示推送间隔: %v", tt.Texts())
	}
	if err := a.updatePreferences(func(p *preferences) { p.Backup.GitAutoPush = true }); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !tt.HasText("推送间隔（分钟）") {
		t.Fatalf("开关开启时应显示推送间隔: %v", tt.Texts())
	}
}

func TestBackupPageGitDraftAppliesWithValidation(t *testing.T) {
	a := newTestApp(t)
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := a.updatePreferences(func(p *preferences) { p.Backup.BackupDir, p.Backup.GitAutoPush = repo, true }); err != nil {
		t.Fatal(err)
	}
	// 与真实打开设置页一致：先同步草稿，draftReady 后 merge 才生效。
	a.settingsUI.syncDraftFromPrefs(a.prefs)
	// 草稿关闭开关并改间隔：经 pushBackup 应用到偏好。
	a.settingsUI.draft.Backup.GitAutoPush = false
	a.settingsUI.draft.Backup.GitPushMinutes = 30
	a.pushBackup()
	if a.prefs.Backup.GitAutoPush || a.prefs.Backup.GitPushMinutes != 30 {
		t.Fatalf("草稿应应用到偏好: %+v", a.prefs.Backup)
	}
	// 越界间隔被校验拒绝：偏好不变，错误进本页错误行。
	a.settingsUI.draft.Backup.GitPushMinutes = 0
	a.pushBackup()
	if a.settingsUI.keyErr == "" {
		t.Fatal("间隔越界应产生错误提示")
	}
	if a.prefs.Backup.GitPushMinutes != 30 {
		t.Fatalf("越界值不得应用: %d", a.prefs.Backup.GitPushMinutes)
	}
}

func TestBackupPageGitStatusLine(t *testing.T) {
	a := newTestApp(t)
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := a.updatePreferences(func(p *preferences) { p.Backup.BackupDir, p.Backup.GitAutoPush = repo, true }); err != nil {
		t.Fatal(err)
	}
	// 备份建立在已保存的数据文件上（生产环境由启动/首存写入）。
	if err := os.WriteFile(a.notes.DataFile, []byte(`{"version":3,"tabs":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	// 假 runner：无变更（不产生提交），推送成功。
	a.notes.RunGit = func(dir string, args ...string) (string, error) {
		if args[0] == "ls-files" || args[0] == "status" {
			return "", nil
		}
		return "", nil
	}
	if err := a.notes.BackupNow(20); err != nil {
		t.Fatal(err)
	}
	a.notes.WaitGitIdle()
	a.settingsPage = pageBackup
	a.settingsUI.syncDraftFromPrefs(a.prefs)
	tt := ui.NewTester(a.settingsView, 900, 620)
	tt.Frame()
	if !tt.HasText("推送于 06-01 12:00") {
		t.Fatalf("状态行应显示最近推送时间: %v", tt.Texts())
	}
}
