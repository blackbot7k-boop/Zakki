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

	"zakki/locales"
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

func TestOnboardingFinishAppliesConfig(t *testing.T) {
	a := newTestApp(t)
	a.notes.RunGit = nil // 本测试只验镜像文件，不验 git，避免异步噪音
	mirror := filepath.Join(filepath.Dir(a.notes.BackupDir), "wizard-mirror")
	if err := os.MkdirAll(filepath.Join(mirror, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	a.onboarding.dirDraft = mirror
	a.onboarding.dirIsGit = true
	a.onboarding.gitMode = "commitPush"
	a.onboarding.gitModeSel = gitModeDisplay("commitPush")
	a.onboarding.tabCount = 3
	a.onboarding.step = onboardingLastStep

	settled := nextMirrorSettlement(a)
	a.finishOnboarding(true)
	waitForMirrorSettlement(t, settled)

	// 偏好：外部备份 + Git 模式落盘。
	if a.prefs.Backup.MirrorDir != mirror || a.prefs.Backup.GitMode != "commitPush" {
		t.Fatalf("向导配置未写入偏好: %+v", a.prefs.Backup)
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

func TestOnboardingSkipKeepsDefaults(t *testing.T) {
	a := newTestApp(t)
	a.finishOnboarding(false)
	if a.prefs.Backup.MirrorDir != "" || a.prefs.Backup.GitMode != "off" {
		t.Fatalf("跳过不应改动偏好: %+v", a.prefs.Backup)
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

func TestObApplyFolderLinksGit(t *testing.T) {
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
	if a.onboarding.dirDraft != wantPlain || a.onboarding.dirIsGit {
		t.Fatalf("普通目录应原样记录且非 Git: %+v", a.onboarding)
	}

	repo := filepath.Join(base, "repo")
	if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	a.onboarding.gitMode = "commit" // 模拟之前选过 Git
	a.obApplyFolder(plain)          // 换成普通目录应联动关闭
	if a.onboarding.gitMode != "off" {
		t.Fatalf("换成普通目录应联动关闭 Git: %q", a.onboarding.gitMode)
	}
	a.obApplyFolder(repo)
	if !a.onboarding.dirIsGit {
		t.Fatal("Git 仓库目录应被识别")
	}
	// 非法目录：记录错误且不改草稿。
	before := a.onboarding.dirDraft
	a.obApplyFolder(filepath.Dir(a.notes.BackupDir))
	if a.onboarding.dirErr == "" || a.onboarding.dirDraft != before {
		t.Fatalf("非法目录应报错且不改草稿: %+v", a.onboarding)
	}
}

func TestOnboardingViewRendersSteps(t *testing.T) {
	a := newTestApp(t)
	a.onboarding.tabCount = 7
	mirror := filepath.Join(filepath.Dir(filepath.Dir(a.notes.BackupDir)), "view-mirror")
	if err := os.MkdirAll(filepath.Join(mirror, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	a.onboarding.dirDraft = mirror
	a.onboarding.dirIsGit = true
	a.onboarding.gitMode = "commitPush"
	a.onboarding.gitModeSel = gitModeDisplay("commitPush")
	a.onboarding.tabCount = 7

	steps := map[int][]string{
		1: {"选择语言", "English", "简体中文"},
		2: {"欢迎使用 Zakki", "跳过，使用默认设置"},
		3: {"备份", "仅本地备份（推荐）", "同步到外部文件夹"},
		4: {"初始标签数量", "7 个"},
		5: {"准备就绪", "完成，进入 Zakki", "打开备份文件夹"},
	}
	// 第 3 步选中「同步到外部文件夹」卡片时，目录与 Git 档位内联展开。
	a.onboarding.mirrorCard = true
	for step, wants := range steps {
		a.onboarding.step = step
		tt := ui.NewTester(a.onboardingView, 560, 500)
		tt.Frame()
		for _, want := range wants {
			if !tt.HasText(want) {
				t.Fatalf("步骤 %d 缺少 %q: %v", step, want, tt.Texts())
			}
		}
	}
	// 第 3 步展开后：目录卡片、Git 档位与白话解释同屏可见。
	a.onboarding.step = 3
	tt := ui.NewTester(a.onboardingView, 560, 500)
	tt.Frame()
	for _, want := range []string{mirror, "Git 仓库", "更换…", "Git 自动化", "提交并推送"} {
		if !tt.HasText(want) {
			t.Fatalf("备份步骤缺少 %q: %v", want, tt.Texts())
		}
	}
	// 语言按钮的可见文字必须是 Text 子元素：ButtonBase 的 Label 只是
	// 可访问性标题，也计入 HasText，缺 Text 子元素时按钮渲染为空白。
	a.onboarding.step = 1
	tt = ui.NewTester(a.onboardingView, 560, 500)
	tt.Frame()
	for _, want := range mygo.App.Languages() {
		if strings.Count(strings.Join(tt.Texts(), "\n"), i18n.Autonym(want)) < 2 {
			t.Fatalf("语言 %s 缺少可见 Text 子元素: %v", want, tt.Texts())
		}
	}
	// 非 Git 目录：备份步骤说明原因而不是给 Git 档位选项。
	a.onboarding.step = 3
	a.onboarding.dirIsGit = false
	tt = ui.NewTester(a.onboardingView, 560, 500)
	tt.Frame()
	if tt.HasText("提交并推送") {
		t.Fatalf("非 Git 目录不应出现档位选项: %v", tt.Texts())
	}
	if !tt.HasText("不是 Git 仓库") {
		t.Fatalf("非 Git 目录应说明原因: %v", tt.Texts())
	}
}

// TestOnboardingBackupCards 备份方式卡片：点「仅本地」清空外部备份草稿，
// 点「同步到外部文件夹」重新展开；目录与档位只在展开时可见。
func TestOnboardingBackupCards(t *testing.T) {
	a := newTestApp(t)
	ob := &a.onboarding
	ob.step = 3
	ob.mirrorCard = true
	ob.dirDraft = string(filepath.Separator) + filepath.Join("tmp", "wizard-repo")
	ob.dirIsGit = true
	ob.gitMode = "commit"
	ob.gitModeSel = gitModeDisplay("commit")

	tt := ui.NewTester(a.onboardingView, 560, 500)
	tt.Frame()
	if err := tt.Click(locales.OnboardBackupLocalTitle()); err != nil {
		t.Fatal(err)
	}
	if ob.mirrorCard || ob.dirDraft != "" || ob.dirIsGit || ob.gitMode != "off" {
		t.Fatalf("选仅本地应清空外部备份草稿: %+v", ob)
	}
	tt = ui.NewTester(a.onboardingView, 560, 500)
	tt.Frame()
	if tt.HasText("Git 自动化") {
		t.Fatalf("仅本地卡片下不应出现 Git 配置: %v", tt.Texts())
	}
	if err := tt.Click(locales.OnboardBackupMirrorTitle()); err != nil {
		t.Fatal(err)
	}
	if !ob.mirrorCard {
		t.Fatal("选同步卡片应重新展开外部备份配置")
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
