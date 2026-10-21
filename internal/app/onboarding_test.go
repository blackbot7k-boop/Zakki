package app

// 首启向导：全新安装判定、配置应用、跳过语义与视图渲染。

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

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

// 默认值必须来自注入的数据目录，不能触达真实用户 Documents。
func TestOnboardingDefaultStorageDirIsCurrentDataDir(t *testing.T) {
	a := newTestApp(t)
	t.Cleanup(a.shutdown)
	a.initOnboarding()
	if got, want := a.onboarding.dirDraft, filepath.Dir(a.notes.DataPath()); got != want {
		t.Fatalf("向导默认目录 = %q, want 当前数据目录 %q", got, want)
	}
	assertOnlyStorageFiles(t, filepath.Dir(a.notes.DataPath()))
}

func TestOnboardingFinishAppliesConfig(t *testing.T) {
	a := newTestApp(t)
	folder := filepath.Join(filepath.Dir(a.notes.DataPath()), "wizard-storage")
	if err := os.MkdirAll(folder, 0o755); err != nil {
		t.Fatal(err)
	}
	a.onboarding.dirDraft = folder
	a.onboarding.tabCount = 3
	a.onboarding.phase = "ready" // 主按钮只在就绪页生效

	a.finishOnboarding(true)

	// 位置必须持久化为实际业务文件路径。
	wantPath := filepath.Join(folder, "zakki.json")
	if a.prefs.Storage.DataFile != wantPath || a.notes.DataPath() != wantPath {
		t.Fatalf("向导配置未写入偏好: %+v path=%q", a.prefs.Storage, a.notes.DataPath())
	}
	p, err := decodePreferences(persistReadRaw(t, a.preferencesPath()))
	if err != nil {
		t.Fatal(err)
	}
	if p.Storage.DataFile != wantPath {
		t.Fatalf("偏好未持久化: %+v", p.Storage)
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
	if _, err := os.Stat(a.notes.DataPath()); err != nil {
		t.Fatalf("数据文件应已写入: %v", err)
	}
	// 初始标签只保存到一个业务文件，不创建备份。
	names := dirJSONNames(t, folder)
	if len(names) != 1 || names[0] != "zakki.json" {
		t.Fatalf("向导完成后应只有 zakki.json: %v", names)
	}
	assertOnlyStorageFiles(t, folder, "zakki.json")
	assertOnlyStorageFiles(t, filepath.Dir(a.preferencesPath()), "preferences.json", "wizard-storage")
	b := newAppAt(filepath.Dir(a.preferencesPath()))
	t.Cleanup(b.shutdown)
	if err := b.loadPreferences(); err != nil {
		t.Fatal(err)
	}
	if err := b.openConfiguredNotes(); err != nil {
		t.Fatal(err)
	}
	if b.notes.DataPath() != wantPath || len(b.tabs) != 3 || b.tabs[0].Text != a.tabs[0].Text || b.firstRun() {
		t.Fatalf("重启应沿用向导文件与标签: path=%q tabs=%+v", b.notes.DataPath(), b.tabs)
	}
	// 向导标记结束（OnClosed 在真实窗口关闭时清理状态）。
	if !a.onboarding.finished {
		t.Fatal("向导应标记为已结束")
	}
}

func TestOnboardingSkipBlocksWhenDataSaveFails(t *testing.T) {
	a := newTestApp(t)
	t.Cleanup(a.shutdown)
	// 用目录占据目标文件路径，跨平台稳定触发写入失败。
	if err := os.Mkdir(a.notes.DataPath(), 0o755); err != nil {
		t.Fatal(err)
	}
	a.finishOnboarding(false)
	if a.onboarding.finished {
		t.Fatal("落盘失败不应结束向导")
	}
	if a.onboarding.dirErr == "" {
		t.Fatal("应提示数据写入失败")
	}
	info, err := os.Stat(a.notes.DataPath())
	if err != nil || !info.IsDir() {
		t.Fatalf("失败不得替换原目标: %v %v", info, err)
	}
	assertOnlyStorageFiles(t, filepath.Dir(a.notes.DataPath()), "zakki.json")
}

func TestOnboardingSkipKeepsCurrentStoragePath(t *testing.T) {
	a := newTestApp(t)
	t.Cleanup(a.shutdown)
	path := a.notes.DataPath()
	a.onboarding.dirDraft = t.TempDir()
	a.finishOnboarding(false)
	if !a.onboarding.finished {
		t.Fatalf("跳过应完成默认初始化: %s", a.onboarding.dirErr)
	}
	if a.notes.DataPath() != path {
		t.Fatalf("跳过不应迁文件: %q", a.notes.DataPath())
	}
	if len(a.tabs) != DefaultTabCount {
		t.Fatalf("跳过应保留默认标签数: %d", len(a.tabs))
	}
	f := persistReadSaved(t, path)
	if len(f.Tabs) != DefaultTabCount || f.Tabs[0].Text != welcomeTextFor(DefaultTabCount) {
		t.Fatalf("默认内容未保存: %+v", f)
	}
	if a.firstRun() {
		t.Fatal("下次启动不应重复向导")
	}
	assertOnlyStorageFiles(t, filepath.Dir(path), "zakki.json", "preferences.json")
	assertOnlyStorageFiles(t, a.onboarding.dirDraft)
}

func TestObApplyFolderValidates(t *testing.T) {
	a := newTestApp(t)
	base := t.TempDir()

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
	blocker := filepath.Join(base, "not-a-dir")
	persistWrite(t, blocker, "普通文件")
	a.obApplyFolder(blocker)
	if a.onboarding.dirErr == "" || a.onboarding.dirDraft != wantPlain {
		t.Fatalf("非法目录应报错且不改草稿: %+v", a.onboarding)
	}
}

func TestOnboardingViewRendersSteps(t *testing.T) {
	a := newTestApp(t)
	a.onboarding.tabCount = 7
	folder := filepath.Join(t.TempDir(), "view-folder")
	if err := os.MkdirAll(folder, 0o755); err != nil {
		t.Fatal(err)
	}
	a.onboarding.dirDraft = folder

	// 用固定顺序的切片而非 map：map 迭代随机，步骤 1 若留在最后，
	// 下面的「下载字体」断言会跑在步骤 1 界面上而随机失败。
	steps := []struct {
		step  int
		wants []string
	}{
		{1, []string{"外观与字体", "English", "简体中文", "明暗", "风格", "中文字体", "英文字体"}},
		{2, []string{"笔记与保存", "初始标签数量", "保存位置", folder, "完成设置"}},
	}
	for _, s := range steps {
		a.onboarding.step = s.step
		tt := ui.NewTester(a.onboardingView, 640, 560)
		tt.Frame()
		for _, want := range s.wants {
			if !tt.HasText(want) {
				t.Fatalf("步骤 %d 缺少 %q: %v", s.step, want, tt.Texts())
			}
		}
	}
	// 未扫描到已装字体且有可下载选择时，主按钮变为下载字体；
	// 字体摘要不出现（选择与下载分离）。
	a.onboarding.appearance.BodyFont = "hand"
	tt := ui.NewTester(a.onboardingView, 640, 560)
	tt.Frame()
	if !tt.HasText("下载字体") || tt.HasText("本次字体选择") {
		t.Fatalf("提交按钮或摘要不符合预期: %v", tt.Texts())
	}
	// 就绪页等待「开始使用」，不自动进入主界面。
	a.onboarding.phase = "ready"
	tt = ui.NewTester(a.onboardingView, 640, 560)
	tt.Frame()
	if !tt.HasText("开始使用") || !tt.HasText("准备完成") {
		t.Fatalf("就绪页应显示开始使用: %v", tt.Texts())
	}
	// 直接调用 finishOnboarding(true) 必须先经过就绪页。
	a.onboarding.phase = ""
	a.finishOnboarding(true)
	if a.onboarding.finished {
		t.Fatal("未经就绪页不应结束向导")
	}
	// 语言按钮的可见文字必须是 Text 子元素：ButtonBase 的 Label 只是
	// 可访问性标题，不设 Label 时 Texts 里的唯一一条就是可见文字。
	a.onboarding.step = 1
	tt = ui.NewTester(a.onboardingView, 640, 560)
	tt.Frame()
	for _, want := range []string{"English", "简体中文"} {
		if strings.Count(strings.Join(tt.Texts(), "\n"), want) < 1 {
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

// TestOnboardingTabCountInput 标签数输入框：越界钳制到 1–12，非法输入回写规范化文本。
func TestOnboardingTabCountInput(t *testing.T) {
	a := newTestApp(t)
	a.onboarding.tabCountText = "0"
	a.commitTabCountText()
	if a.onboarding.tabCount != 1 {
		t.Fatalf("0 应钳制到 1: %d", a.onboarding.tabCount)
	}
	a.onboarding.tabCountText = "99"
	a.commitTabCountText()
	if a.onboarding.tabCount != MaxTabCount {
		t.Fatalf("99 应钳制到 %d: %d", MaxTabCount, a.onboarding.tabCount)
	}
	a.onboarding.tabCountText = "abc"
	a.commitTabCountText()
	if a.onboarding.tabCountText != strconv.Itoa(MaxTabCount) {
		t.Fatalf("非法输入应回写规范化文本: %q", a.onboarding.tabCountText)
	}
}

// 向导选手账风格：完成后写入风格，颜色与排版保持标准默认（切回标准时用），
// 偏好通过版本 10 校验。
func TestOnboardingSketchStyle(t *testing.T) {
	a := newTestApp(t)
	a.initOnboarding()
	folder := filepath.Join(filepath.Dir(a.notes.DataPath()), "wizard-style")
	if err := os.MkdirAll(folder, 0o755); err != nil {
		t.Fatal(err)
	}
	a.onboarding.dirDraft = folder
	tt := ui.NewTester(a.onboardingView, 640, 560)
	tt.Frame()
	if err := tt.Click("手账"); err != nil {
		t.Fatal(err)
	}
	if a.onboarding.appearance.Style != styleSketch {
		t.Fatalf("选手账应写入风格草稿: %+v", a.onboarding.appearance)
	}
	a.onboarding.phase = "ready"
	a.finishOnboarding(true)
	ap := a.prefs.Appearance
	if ap.Style != styleSketch || ap.ColorTheme != "classic" || ap.MarkdownTheme != "sonnet" {
		t.Fatalf("向导应只写风格、颜色与排版保持标准默认: %+v", ap)
	}
}
