package app

// 外部备份目的地：目录校验、设置页展示与状态栏镜像标签。

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func nextMirrorSettlement(a *app) <-chan struct{} {
	settled := make(chan struct{}, 1)
	a.notes.SetOnMirrorSettled(func() {
		select {
		case settled <- struct{}{}:
		default:
		}
	})
	return settled
}

func waitForMirrorSettlement(t *testing.T, settled <-chan struct{}) {
	t.Helper()
	select {
	case <-settled:
	case <-time.After(2 * time.Second):
		t.Fatal("等待外部镜像完成超时")
	}
}

func TestValidateMirrorDir(t *testing.T) {
	a := newTestApp(t)
	dataDir := filepath.Dir(a.notes.BackupDir)

	// 本地备份目录及其子目录拒绝。
	if _, err := a.validateMirrorDir(a.notes.BackupDir); err == nil {
		t.Fatal("本地备份目录应被拒绝")
	}
	if _, err := a.validateMirrorDir(filepath.Join(a.notes.BackupDir, "sub")); err == nil {
		t.Fatal("本地备份目录的子目录应被拒绝")
	}
	// 应用数据目录及其子目录拒绝。
	if _, err := a.validateMirrorDir(dataDir); err == nil {
		t.Fatal("应用数据目录应被拒绝")
	}
	if _, err := a.validateMirrorDir(filepath.Join(dataDir, "elsewhere", "bak")); err == nil {
		t.Fatal("应用数据目录的子目录应被拒绝")
	}
	// 同级外部目录通过并返回绝对路径。
	ok := filepath.Join(filepath.Dir(dataDir), "external", "bak")
	if err := os.MkdirAll(ok, 0o755); err != nil {
		t.Fatal(err)
	}
	abs, err := a.validateMirrorDir(ok)
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
	if _, err := a.validateMirrorDir(filepath.Join(blocker, "sub")); err == nil {
		t.Fatal("不可写路径应被拒绝")
	}
}

func TestValidateMirrorDirRejectsSymlinkIntoDataDir(t *testing.T) {
	a := newTestApp(t)
	dataDir := filepath.Dir(a.notes.BackupDir)
	link := filepath.Join(dataDir, "mirror-link")
	_ = os.Remove(link)
	if err := os.Symlink(dataDir, link); err != nil {
		t.Skip("当前环境无法创建符号链接:", err)
	}
	t.Cleanup(func() { _ = os.Remove(link) })
	if _, err := a.validateMirrorDir(link); err == nil {
		t.Fatal("指向应用数据目录的符号链接应被拒绝")
	}
}

func TestValidateMirrorDirDoesNotTouchPreexistingProbeFile(t *testing.T) {
	a := newTestApp(t)
	dir := filepath.Join(filepath.Dir(filepath.Dir(a.notes.BackupDir)), "probe-file")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	probe := filepath.Join(dir, ".zakki-backup-probe")
	want := []byte("must survive")
	if err := os.WriteFile(probe, want, 0o640); err != nil {
		t.Fatal(err)
	}
	if _, err := a.validateMirrorDir(dir); err != nil {
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

func TestValidateMirrorDirDoesNotFollowPreexistingProbeSymlink(t *testing.T) {
	a := newTestApp(t)
	dir := filepath.Join(filepath.Dir(filepath.Dir(a.notes.BackupDir)), "probe-link")
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
	if _, err := a.validateMirrorDir(dir); err != nil {
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

func TestApplyMirrorDirLinksGitMode(t *testing.T) {
	a := newTestApp(t)
	// 普通（非 Git）目录：应用为外部备份并联动关闭 Git。
	plain := filepath.Join(filepath.Dir(filepath.Dir(a.notes.BackupDir)), "plain")
	if err := os.MkdirAll(plain, 0o755); err != nil {
		t.Fatal(err)
	}
	a.applyMirrorDir(plain)
	wantPlain, err := canonicalPath(plain)
	if err != nil {
		t.Fatal(err)
	}
	if a.prefs.Backup.MirrorDir != wantPlain {
		t.Fatalf("外部备份目录 = %q, want %q", a.prefs.Backup.MirrorDir, wantPlain)
	}
	if a.prefs.Backup.GitMode != "off" {
		t.Fatalf("非 Git 目录应联动关闭 Git 自动化: %q", a.prefs.Backup.GitMode)
	}
	// Git 仓库目录：保留用户选择的档位（默认 off）。
	repo := filepath.Join(filepath.Dir(filepath.Dir(a.notes.BackupDir)), "repo")
	if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	a.applyMirrorDir(repo)
	wantRepo, err := canonicalPath(repo)
	if err != nil {
		t.Fatal(err)
	}
	if a.prefs.Backup.MirrorDir != wantRepo {
		t.Fatalf("外部备份目录 = %q, want %q", a.prefs.Backup.MirrorDir, wantRepo)
	}
	// 数据目录被拒绝且偏好不变。
	before := a.prefs.Backup.MirrorDir
	a.applyMirrorDir(filepath.Dir(a.notes.BackupDir))
	if a.prefs.Backup.MirrorDir != before {
		t.Fatal("非法目录不应改动偏好")
	}
	if a.settingsUI.keyErr == "" {
		t.Fatal("非法目录应提示错误")
	}
}

func TestPushMirrorGitModeRequiresGitRepo(t *testing.T) {
	a := newTestApp(t)
	plain := filepath.Join(filepath.Dir(filepath.Dir(a.notes.BackupDir)), "plain2")
	if err := os.MkdirAll(plain, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := a.updatePreferences(func(p *preferences) { p.Backup.MirrorDir = plain }); err != nil {
		t.Fatal(err)
	}
	a.settingsUI.syncDraftFromPrefs(a.prefs)
	a.settingsUI.gitModeSel = "自动提交"
	a.pushMirrorGitMode()
	if a.prefs.Backup.GitMode != "off" {
		t.Fatalf("非 Git 目录不应开启 Git 自动化: %q", a.prefs.Backup.GitMode)
	}
	if a.settingsUI.keyErr == "" {
		t.Fatal("应提示目录不是 Git 仓库")
	}
	// Git 仓库目录允许开启。
	repo := filepath.Join(filepath.Dir(a.notes.BackupDir), "repo2")
	if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := a.updatePreferences(func(p *preferences) { p.Backup.MirrorDir = repo }); err != nil {
		t.Fatal(err)
	}
	a.settingsUI.syncDraftFromPrefs(a.prefs)
	a.settingsUI.gitModeSel = "提交并推送"
	a.pushMirrorGitMode()
	if a.prefs.Backup.GitMode != "commitPush" {
		t.Fatalf("Git 仓库应允许开启: %q", a.prefs.Backup.GitMode)
	}
}

func TestMirrorLabel(t *testing.T) {
	a := newTestApp(t)
	if label, warn := a.mirrorLabel(); label != "" || warn {
		t.Fatalf("未配置时不应显示镜像状态: %q %v", label, warn)
	}
	// 配置后尚未备份：尚未镜像。
	mirror := filepath.Join(filepath.Dir(a.notes.BackupDir), "mirror")
	if err := os.MkdirAll(mirror, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := a.updatePreferences(func(p *preferences) { p.Backup.MirrorDir = mirror }); err != nil {
		t.Fatal(err)
	}
	if label, warn := a.mirrorLabel(); label != "尚未镜像" || warn {
		t.Fatalf("配置后未备份 = %q warn=%v, want 尚未镜像 false", label, warn)
	}
	// 备份成功后（固定时钟）：刚刚镜像。
	if err := a.notes.SaveNow(a.snapshotFile()); err != nil {
		t.Fatal(err)
	}
	settled := nextMirrorSettlement(a)
	if err := a.notes.BackupNow(a.prefs.Backup.Keep); err != nil {
		t.Fatal(err)
	}
	waitForMirrorSettlement(t, settled)
	if label, warn := a.mirrorLabel(); label != "刚刚镜像" || warn {
		t.Fatalf("备份后 = %q warn=%v, want 刚刚镜像 false", label, warn)
	}
	// 镜像失败：加粗提示。
	blocker := filepath.Join(filepath.Dir(a.notes.BackupDir), "blocker")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := a.updatePreferences(func(p *preferences) { p.Backup.MirrorDir = blocker }); err != nil {
		t.Fatal(err)
	}
	settled = nextMirrorSettlement(a)
	if err := a.notes.BackupNow(a.prefs.Backup.Keep); err != nil {
		t.Fatalf("镜像失败不得影响本地备份: %v", err)
	}
	waitForMirrorSettlement(t, settled)
	if label, warn := a.mirrorLabel(); label != "镜像失败" || !warn {
		t.Fatalf("镜像失败 = %q warn=%v, want 镜像失败 true", label, warn)
	}
}

func TestSettingsBackupPageShowsMirror(t *testing.T) {
	a := newTestApp(t)
	a.winOpen = true
	mirror := filepath.Join(filepath.Dir(a.notes.BackupDir), "mirror-page")
	if err := os.MkdirAll(filepath.Join(mirror, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := a.updatePreferences(func(p *preferences) {
		p.Backup.MirrorDir = mirror
		p.Backup.GitMode = "commit"
	}); err != nil {
		t.Fatal(err)
	}
	a.settingsPage = pageBackup
	a.settingsUI.syncDraftFromPrefs(a.prefs)
	tt := ui.NewTester(a.settingsView, 760, 600)
	tt.Frame()
	for _, want := range []string{mirror, "打开文件夹", "更换…", "Git 自动化", "Git 仓库"} {
		if !tt.HasText(want) {
			t.Fatalf("备份页缺少 %q: %v", want, tt.Texts())
		}
	}
	// 未配置时显示选择入口。
	if err := a.updatePreferences(func(p *preferences) {
		p.Backup.MirrorDir = ""
		p.Backup.GitMode = "off"
	}); err != nil {
		t.Fatal(err)
	}
	a.settingsUI.syncDraftFromPrefs(a.prefs)
	tt2 := ui.NewTester(a.settingsView, 760, 600)
	tt2.Frame()
	if !tt2.HasText("选择文件夹…") {
		t.Fatalf("未配置时应有选择入口: %v", tt2.Texts())
	}
	if tt2.HasText("打开文件夹") {
		t.Fatalf("未配置时不应有打开入口: %v", tt2.Texts())
	}
}

func TestStatusBarShowsMirrorLabel(t *testing.T) {
	a := newTestApp(t)
	a.winOpen = true
	mirror := filepath.Join(filepath.Dir(a.notes.BackupDir), "mirror-bar")
	if err := os.MkdirAll(mirror, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := a.updatePreferences(func(p *preferences) { p.Backup.MirrorDir = mirror }); err != nil {
		t.Fatal(err)
	}
	if err := a.notes.SaveNow(a.snapshotFile()); err != nil {
		t.Fatal(err)
	}
	settled := nextMirrorSettlement(a)
	if err := a.notes.BackupNow(a.prefs.Backup.Keep); err != nil {
		t.Fatal(err)
	}
	waitForMirrorSettlement(t, settled)
	tt := ui.NewTester(a.view, 400, 540)
	tt.Frame()
	if !tt.HasText("刚刚镜像") || !tt.HasText("刚刚备份") {
		t.Fatalf("状态栏应同时显示备份与镜像状态: %v", tt.Texts())
	}
}

// 断言时序护栏：镜像时间应随固定时钟走，真实 mtime 不参与状态栏文案。
func TestMirrorLastUsesInjectedClock(t *testing.T) {
	a := newTestApp(t)
	mirror := filepath.Join(filepath.Dir(a.notes.BackupDir), "mirror-clock")
	if err := os.MkdirAll(mirror, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := a.updatePreferences(func(p *preferences) { p.Backup.MirrorDir = mirror }); err != nil {
		t.Fatal(err)
	}
	if err := a.notes.SaveNow(a.snapshotFile()); err != nil {
		t.Fatal(err)
	}
	settled := nextMirrorSettlement(a)
	if err := a.notes.BackupNow(a.prefs.Backup.Keep); err != nil {
		t.Fatal(err)
	}
	waitForMirrorSettlement(t, settled)
	if st := a.notes.MirrorStatus(); !st.Last.Equal(time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)) {
		t.Fatalf("lastMirror 应用注入时钟 %v, got %v", time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC), st.Last)
	}
}
