package app

// 首启向导：全新安装判定、配置应用、跳过语义与视图渲染。

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/i18n"
	"github.com/egoist/mygo/ui"
)

func TestFirstRunDetection(t *testing.T) {
	a := newTestApp(t)
	if !a.firstRun() {
		t.Fatal("数据文件不存在应判定为全新安装")
	}
	if err := a.notes.SaveNow(a.snapshotFile()); err != nil {
		t.Fatal(err)
	}
	if a.firstRun() {
		t.Fatal("数据文件已存在不应判定为全新安装")
	}
}

// TestDefaultMirrorDir 默认外部备份目录固定在 ~/Documents/Zakki 备份，
// 且永远非空（取不到主目录时由向导退化成"必须手选"，但取值函数本身有兜底）。
func TestDefaultMirrorDir(t *testing.T) {
	got := defaultMirrorDir()
	if got == "" {
		t.Fatal("默认备份目录不应为空")
	}
	if filepath.Base(got) != "Zakki 备份" || filepath.Base(filepath.Dir(got)) != "Documents" {
		t.Fatalf("默认备份目录应为 ~/Documents/Zakki 备份: %q", got)
	}
}

func TestOnboardingFinishAppliesConfig(t *testing.T) {
	a := newTestApp(t)
	a.notes.RunGit = nil // 本测试只验镜像文件，不验 git，避免异步噪音
	mirror := filepath.Join(filepath.Dir(a.notes.BackupDir), "wizard-mirror")
	if err := os.MkdirAll(mirror, 0o755); err != nil {
		t.Fatal(err)
	}
	a.onboarding.dirDraft = mirror
	a.onboarding.tabCount = 3
	a.onboarding.step = onboardingLastStep

	settled := nextMirrorSettlement(a)
	a.finishOnboarding(true)
	waitForMirrorSettlement(t, settled)

	// 偏好：外部备份落盘；Git 档位不在向导出现，保持默认 off。
	if a.prefs.Backup.MirrorDir != mirror {
		t.Fatalf("向导配置未写入偏好: %+v", a.prefs.Backup)
	}
	if a.prefs.Backup.GitMode != "off" {
		t.Fatalf("向导不应改动 Git 档位: %+v", a.prefs.Backup)
	}
	// 数据：3 个标签，第 1 个含欢迎文本，其余为空；文件已落盘。
	if len(a.tabs) != 3 {
		t.Fatalf("标签数 = %d, want 3", len(a.tabs))
	}
	if a.tabs[0].Text == "" {
		t.Fatal("首个标签应含欢迎文本")
	}
	if want := welcomeTextFor(3); a.tabs[0].Text != want {
		t.Fatalf("欢迎文案应匹配标签数: got %q", a.tabs[0].Text)
	}
	for i := 1; i < len(a.tabs); i++ {
		if a.tabs[i].Text != "" {
			t.Fatalf("标签 %d 应为空: %q", i+1, a.tabs[i].Text)
		}
	}
	// ID 单调唯一：收缩后允许跳号，但不得与现有标签冲突。
	seen := map[uint64]bool{}
	for _, tb := range a.tabs {
		if tb.ID == 0 || seen[tb.ID] {
			t.Fatalf("标签 ID 应唯一非零: %+v", a.tabs)
		}
		seen[tb.ID] = true
	}
	if a.nextTabID <= 3 {
		t.Fatalf("nextTabID = %d, 应大于现有最大 ID", a.nextTabID)
	}
	if _, err := os.Stat(a.notes.DataFile); err != nil {
		t.Fatalf("数据文件应已写入: %v", err)
	}
	// 外部备份目录应立刻出现第一份镜像备份。
	names := dirJSONNames(t, mirror)
	if len(names) != 1 {
		t.Fatalf("向导完成后镜像目录应有 1 份备份: %v", names)
	}
	// 向导标记结束（OnClosed 在真实窗口关闭时清理状态）。
	if !a.onboarding.finished {
		t.Fatal("向导应标记为已结束")
	}
}

func TestOnboardingSkipBlocksWhenDataSaveFails(t *testing.T) {
	a := newTestApp(t)
	// 跳过会把默认备份文件夹写进偏好，注入临时路径保证测试不触达真实主目录。
	oldDef := defaultMirrorDir
	t.Cleanup(func() { defaultMirrorDir = oldDef })
	tmpDef := filepath.Join(t.TempDir(), "默认备份")
	defaultMirrorDir = func() string { return tmpDef }

	dir := filepath.Dir(a.notes.DataFile)
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Skip("无法 chmod 测试目录:", err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })

	a.finishOnboarding(false)
	if a.onboarding.finished {
		t.Fatal("落盘失败时不应结束向导")
	}
	if a.onboarding.dirErr == "" {
		t.Fatal("应提示数据写入失败")
	}
	if _, err := os.Stat(a.notes.DataFile); !os.IsNotExist(err) {
		t.Fatalf("落盘失败时不应产生数据文件: %v", err)
	}
}

func TestOnboardingSkipAppliesDefaultMirror(t *testing.T) {
	a := newTestApp(t)
	// 跳过等同接受默认：备份文件夹落默认路径，Git 档位保持 off。
	oldDef := defaultMirrorDir
	t.Cleanup(func() { defaultMirrorDir = oldDef })
	tmpDef := filepath.Join(t.TempDir(), "默认备份")
	defaultMirrorDir = func() string { return tmpDef }

	a.finishOnboarding(false)
	if a.prefs.Backup.MirrorDir != tmpDef {
		t.Fatalf("跳过应写入默认备份文件夹: %+v", a.prefs.Backup)
	}
	if a.prefs.Backup.GitMode != "off" {
		t.Fatalf("跳过不应改动 Git 档位: %+v", a.prefs.Backup)
	}
	if len(a.tabs) != DefaultTabCount {
		t.Fatalf("跳过后应保持默认标签数 %d, got %d", DefaultTabCount, len(a.tabs))
	}
	if _, err := os.Stat(a.notes.DataFile); err != nil {
		t.Fatalf("跳过向导后应写入初始数据文件，避免下次启动重复向导: %v", err)
	}
	if a.firstRun() {
		t.Fatal("数据文件落盘后不应再判定为全新安装")
	}
}

func TestObApplyFolderValidates(t *testing.T) {
	a := newTestApp(t)
	base := filepath.Dir(filepath.Dir(a.notes.BackupDir))

	plain := filepath.Join(base, "plain")
	if err := os.MkdirAll(plain, 0o755); err != nil {
		t.Fatal(err)
	}
	a.obApplyFolder(plain)
	wantPlain, err := canonicalPath(plain)
	if err != nil {
		t.Fatal(err)
	}
	if a.onboarding.dirDraft != wantPlain || a.onboarding.dirErr != "" {
		t.Fatalf("合法目录应原样记录: %+v", a.onboarding)
	}
	// 非法目录：记录错误且不改草稿。
	a.obApplyFolder(filepath.Dir(a.notes.BackupDir))
	if a.onboarding.dirErr == "" || a.onboarding.dirDraft != wantPlain {
		t.Fatalf("非法目录应报错且不改草稿: %+v", a.onboarding)
	}
}

func TestOnboardingViewRendersSteps(t *testing.T) {
	a := newTestApp(t)
	a.onboarding.tabCount = 7
	folder := filepath.Join(filepath.Dir(filepath.Dir(a.notes.BackupDir)), "view-mirror")
	if err := os.MkdirAll(folder, 0o755); err != nil {
		t.Fatal(err)
	}
	a.onboarding.dirDraft = folder

	steps := map[int][]string{
		1: {"选择语言", "English", "简体中文"},
		2: {"备份文件夹", folder, "更换…"},
		3: {"初始标签数量", "7 个"},
	}
	for step, wants := range steps {
		a.onboarding.step = step
		tt := ui.NewTester(a.onboardingView, 560, 440)
		tt.Frame()
		for _, want := range wants {
			if !tt.HasText(want) {
				t.Fatalf("步骤 %d 缺少 %q: %v", step, want, tt.Texts())
			}
		}
	}
	// 最后一步主按钮是完成，且不再显示跳过。
	a.onboarding.step = onboardingLastStep
	tt := ui.NewTester(a.onboardingView, 560, 440)
	tt.Frame()
	if !tt.HasText("完成，进入 Zakki") {
		t.Fatalf("最后一步应显示完成按钮: %v", tt.Texts())
	}
	// 语言按钮的可见文字必须是 Text 子元素：ButtonBase 的 Label 只是
	// 可访问性标题，也计入 HasText，缺 Text 子元素时按钮渲染为空白。
	a.onboarding.step = 1
	tt = ui.NewTester(a.onboardingView, 560, 440)
	tt.Frame()
	for _, want := range mygo.App.Languages() {
		if strings.Count(strings.Join(tt.Texts(), "\n"), i18n.Autonym(want)) < 2 {
			t.Fatalf("语言 %s 缺少可见 Text 子元素: %v", want, tt.Texts())
		}
	}
}

func dirJSONNames(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".json" {
			out = append(out, e.Name())
		}
	}
	return out
}
