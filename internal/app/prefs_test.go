package app

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// preferences.json 是用户偏好的唯一来源；迁移一次性完成，旧文件原样保留。

func prefsReadRaw(t *testing.T, path string) []byte {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func persistWriteRaw(t *testing.T, path string, raw string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestPrefsMigrateFromTOMLHasPriority(t *testing.T) {
	for _, tc := range []struct {
		name, toml, jsonBlob string
		check                func(*testing.T, *app, error)
	}{
		{
			name:     "TOML 显式值覆盖笔记旧设置",
			toml:     "# 用户已编辑\n[appearance]\nbodySize = 23\n",
			jsonBlob: `{"appearance":{"bodySize":24}}`,
			check: func(t *testing.T, a *app, err error) {
				if err != nil {
					t.Fatal(err)
				}
				if a.prefs.Appearance.BodySize != 23 {
					t.Fatalf("TOML 必须优先于旧 JSON 与笔记设置: %+v", a.prefs.Appearance)
				}
			},
		},
		{
			name:     "损坏 TOML 报错且不导入",
			toml:     "[bad",
			jsonBlob: `{"appearance":{"bodySize":24}}`,
			check: func(t *testing.T, a *app, err error) {
				if err == nil {
					t.Fatal("损坏 TOML 必须报错")
				}
				if a.prefs.Appearance.BodySize != 19 {
					t.Fatalf("报错时只用笔记旧设置，不导入损坏 TOML 或旧 JSON: %+v", a.prefs.Appearance)
				}
				if a.currentPrefsIssue().Kind != prefsIssueLegacyBroken {
					t.Fatalf("应进入待确认状态: %+v", a.currentPrefsIssue())
				}
				if _, statErr := os.Stat(a.preferencesPath()); !os.IsNotExist(statErr) {
					t.Fatalf("待确认状态不得写 preferences.json: %v", statErr)
				}
				// 用户确认「使用已有偏好继续」后建立新设置文件。
				if err := a.confirmContinueWithoutLegacyConfig(); err != nil {
					t.Fatal(err)
				}
				if _, statErr := os.Stat(a.preferencesPath()); statErr != nil {
					t.Fatalf("确认后应写出偏好文件: %v", statErr)
				}
				if a.currentPrefsIssue().Kind != prefsIssueNone {
					t.Fatalf("确认后应清除待确认状态: %+v", a.currentPrefsIssue())
				}
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := newTestApp(t)
			a.cfg.BodySize = 19 // 笔记内旧设置
			dir := filepath.Dir(a.notes.DataFile)
			if tc.toml != "" {
				persistWriteRaw(t, filepath.Join(dir, "config.toml"), tc.toml)
			}
			if tc.jsonBlob != "" {
				persistWriteRaw(t, filepath.Join(dir, "config.json"), tc.jsonBlob)
			}
			err := a.loadPreferences()
			tc.check(t, a, err)
		})
	}
}

func TestPrefsMigrateFromLegacyJSON(t *testing.T) {
	a := newTestApp(t)
	a.cfg.ThemeSource, a.cfg.BodySize, a.cfg.Source = "light", 19, true
	a.tabs[0].Text = "配置迁移不能改变笔记"
	if err := a.saveNow(); err != nil {
		t.Fatal(err)
	}
	dataBefore := string(prefsReadRaw(t, a.notes.DataFile))
	legacy := `{"appearance":{"theme":"dark","bodySize":24,"markdownTheme":"tot"},"window":{"hideOnEscape":false,"rememberBounds":false},"storage":{"backupKeep":35,"backupIntervalMinutes":9},"shortcuts":{"toggleWindow":""}}`
	dir := filepath.Dir(a.notes.DataFile)
	persistWriteRaw(t, filepath.Join(dir, "config.json"), legacy)

	if err := a.loadPreferences(); err != nil {
		t.Fatal(err)
	}
	if a.prefs.Appearance.AppearanceMode != "dark" || a.prefs.Appearance.BodySize != 24 {
		t.Fatalf("显式值应迁移: %+v", a.prefs.Appearance)
	}
	if a.prefs.Appearance.MarkdownTheme != "note" {
		t.Fatalf("Sonnet/Tot 应映射为同名排版主题: %+v", a.prefs.Appearance)
	}
	if a.prefs.Window.HideOnEscape || a.prefs.Window.RememberBounds {
		t.Fatalf("显式 false 必须保留: %+v", a.prefs.Window)
	}
	if a.prefs.Backup.Keep != 35 || a.prefs.Backup.IntervalMinutes != 9 {
		t.Fatalf("备份策略应迁移: %+v", a.prefs.Backup)
	}
	if over, ok := a.prefs.Keymap.Overrides[cmdToggleWindow]; !ok || over != "" {
		t.Fatalf("显式空热键应解除绑定: %q %v", over, ok)
	}
	if a.cfg.Source != true {
		t.Fatal("编辑会话状态（源码模式）属于笔记文件，不被偏好迁移覆盖")
	}
	// 笔记数据不变；旧 JSON 原样保留。
	if string(prefsReadRaw(t, a.notes.DataFile)) != dataBefore {
		t.Fatal("迁移不得修改笔记数据")
	}
	if string(prefsReadRaw(t, filepath.Join(dir, "config.json"))) != legacy {
		t.Fatal("迁移不得改写旧配置")
	}
	// preferences.json 落盘且可读回。
	prefs, err := decodePreferences(prefsReadRaw(t, a.preferencesPath()))
	if err != nil || prefs.Appearance.BodySize != 24 {
		t.Fatalf("迁移结果必须原子落盘: %+v, %v", prefs, err)
	}
	// 二次启动：直接读取 preferences.json，不再读旧配置。
	if err := a.loadPreferences(); err != nil {
		t.Fatal(err)
	}
	if a.prefs.Appearance.BodySize != 24 {
		t.Fatalf("重复启动应读偏好文件: %+v", a.prefs.Appearance)
	}
}

func TestPrefsNotesSettingsUsedWhenNoConfig(t *testing.T) {
	a := newTestApp(t)
	a.cfg.ThemeSource, a.cfg.BodySize, a.cfg.MarkdownTheme, a.cfg.BodyFont = "dark", 21, "tot", "songti"
	if err := a.loadPreferences(); err != nil {
		t.Fatal(err)
	}
	if a.prefs.Appearance.AppearanceMode != "dark" || a.prefs.Appearance.BodySize != 21 ||
		a.prefs.Appearance.MarkdownTheme != "note" || a.prefs.Appearance.BodyFont != "songti" {
		t.Fatalf("笔记内旧设置应成为迁移基线: %+v", a.prefs.Appearance)
	}
	if a.prefs.Appearance.LatinFont != "" {
		t.Fatalf("选过中文字体的旧用户英文应保持跟随中文: %q", a.prefs.Appearance.LatinFont)
	}
	// 全新安装（无笔记设置、无旧配置）用默认值。
	b := newTestApp(t)
	if err := b.loadPreferences(); err != nil {
		t.Fatal(err)
	}
	if b.prefs.Appearance != defaultPreferences().Appearance {
		t.Fatalf("全新安装应保持默认外观: %+v", b.prefs.Appearance)
	}
}

func TestPrefsBrokenFileRecovery(t *testing.T) {
	a := newTestApp(t)
	broken := `{"version":1, bad`
	persistWriteRaw(t, a.preferencesPath(), broken)
	if err := a.loadPreferences(); err == nil {
		t.Fatal("损坏偏好必须报错")
	}
	if a.currentPrefsIssue().Kind != prefsIssuePrefsBroken {
		t.Fatalf("应进入待恢复状态: %+v", a.currentPrefsIssue())
	}
	if a.prefsSaveError() == "" {
		t.Fatal("待恢复状态必须显示未保存错误")
	}
	if a.prefs.Appearance != defaultPreferences().Appearance {
		t.Fatal("待恢复状态用默认偏好运行")
	}
	// 待恢复状态绝不覆盖原文件（自动保存被阻止）。
	if err := a.updatePreferences(func(p *preferences) { p.Appearance.ColorTheme = "paper" }); err != nil {
		t.Fatal(err)
	}
	if got := string(prefsReadRaw(t, a.preferencesPath())); got != broken {
		t.Fatal("待恢复状态不得改写原文件")
	}
	// 确认后：另存原件并恢复默认写盘。
	if err := a.confirmRecoverPreferences(); err != nil {
		t.Fatal(err)
	}
	if a.currentPrefsIssue().Kind != prefsIssueNone {
		t.Fatalf("恢复后应清除状态: %+v", a.currentPrefsIssue())
	}
	if got := a.prefsSaveError(); got != "" {
		t.Fatalf("恢复写入成功后应清除未保存错误: %q", got)
	}
	prefs, err := decodePreferences(prefsReadRaw(t, a.preferencesPath()))
	if err != nil {
		t.Fatalf("恢复后应写出合法偏好: %v", err)
	}
	if prefs.Appearance.ColorTheme != "paper" {
		t.Fatalf("恢复写盘应包含会话中的修改: %+v", prefs.Appearance)
	}
	entries, err := os.ReadDir(filepath.Dir(a.notes.DataFile))
	if err != nil {
		t.Fatal(err)
	}
	saved := false
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "preferences.json.bad-") {
			saved = true
		}
	}
	if !saved {
		t.Fatal("原件必须已另存")
	}
}

func TestPrefsVersionTooNewBlocked(t *testing.T) {
	a := newTestApp(t)
	future := `{"version":99}`
	persistWriteRaw(t, a.preferencesPath(), future)
	if err := a.loadPreferences(); err == nil {
		t.Fatal("超前版本必须报错")
	}
	if a.currentPrefsIssue().Kind != prefsIssuePrefsBroken {
		t.Fatalf("应进入待恢复状态: %+v", a.currentPrefsIssue())
	}
	// 不退回旧 TOML：即使存在 config.toml 也不读。
	persistWriteRaw(t, filepath.Join(filepath.Dir(a.notes.DataFile), "config.toml"), "[appearance]\ncolorTheme = \"paper\"\n")
	if a.prefs.Appearance.ColorTheme != "classic" {
		t.Fatal("超前版本不得退回旧 TOML")
	}
	if err := a.confirmRecoverPreferences(); err != nil {
		t.Fatal(err)
	}
	if a.prefs.Appearance.ColorTheme != "classic" {
		t.Fatalf("恢复默认不读旧配置: %+v", a.prefs.Appearance)
	}
}

func TestPrefsUpdateAppliesAndSaves(t *testing.T) {
	a := newTestApp(t)
	if err := a.loadPreferences(); err != nil {
		t.Fatal(err)
	}
	if err := a.updatePreferences(func(p *preferences) {
		p.Appearance.ColorTheme = "mistTeal"
		p.Appearance.BodySize = 18
		p.Window.AlwaysOnTop = true
	}); err != nil {
		t.Fatal(err)
	}
	if a.cfg.ThemeSource != "system" || a.cfg.BodySize != 18 || a.cfg.AlwaysOnTop != true {
		t.Fatalf("运行时投影未更新: %+v", a.cfg)
	}
	a.shutdownPreferences()
	prefs, err := decodePreferences(prefsReadRaw(t, a.preferencesPath()))
	if err != nil || prefs.Appearance.ColorTheme != "mistTeal" || prefs.Appearance.BodySize != 18 {
		t.Fatalf("变更应持久化: %+v, %v", prefs, err)
	}
	// 重启读回。
	b := newTestApp(t)
	b.notes = a.notes
	b.prefsStore = a.prefsStore
	if err := b.loadPreferences(); err != nil {
		t.Fatal(err)
	}
	if b.prefs.Appearance.ColorTheme != "mistTeal" {
		t.Fatalf("重启后应保留: %+v", b.prefs)
	}
}

func TestPrefsUpdateRejectsInvalid(t *testing.T) {
	a := newTestApp(t)
	before := a.prefs
	if err := a.updatePreferences(func(p *preferences) { p.Appearance.BodySize = 99 }); err == nil {
		t.Fatal("非法字号必须拒绝")
	}
	if a.prefs.Appearance.BodySize != before.Appearance.BodySize {
		t.Fatal("校验失败不得改动状态")
	}
}

func TestPrefsLegacyHotkeyMigration(t *testing.T) {
	a := newTestApp(t)
	dir := filepath.Dir(a.notes.DataFile)
	persistWriteRaw(t, filepath.Join(dir, "config.toml"), "shortcuts.toggleWindow = \"Super+Alt+Space\"\n")
	if err := a.loadPreferences(); err != nil {
		t.Fatal(err)
	}
	acc := a.prefs.Keymap.Overrides[cmdToggleWindow]
	if acc == "" {
		t.Fatal("旧热键应迁移")
	}
	if _, err := parseAccelerator(acc); err != nil {
		t.Fatalf("迁移后的热键必须是规范化 accelerator: %q %v", acc, err)
	}
	if acc != "CmdOrCtrl+Alt+Space" {
		t.Fatalf("Super/Cmd 书写应规范化为 CmdOrCtrl: %q", acc)
	}
}

func TestPrefsSaveFailureReported(t *testing.T) {
	a := newTestApp(t)
	if err := a.loadPreferences(); err != nil {
		t.Fatal(err)
	}
	// 把偏好路径变成目录，制造写失败。
	prefsPath := a.preferencesPath()
	if err := os.Remove(prefsPath); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(prefsPath, 0o755); err != nil {
		t.Fatal(err)
	}
	a.prefsStore.ResetRevsForTest()
	if err := a.updatePreferences(func(p *preferences) { p.Appearance.BodySize = 20 }); err != nil {
		t.Fatal(err)
	}
	if err := a.savePreferencesNow(); err == nil {
		t.Fatal("写失败必须返回错误")
	}
	if msg := a.prefsSaveError(); msg == "" {
		t.Fatal("写失败必须展示「未保存」")
	}
	// 会话内继续用已生效值。
	if a.prefs.Appearance.BodySize != 20 {
		t.Fatal("保存失败时已生效值继续可用")
	}
	_ = os.Remove(prefsPath)
}

func TestPrefsReadErrorBlocksAutomaticAndShutdownWrites(t *testing.T) {
	a := newTestApp(t)
	path := a.preferencesPath()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := a.loadPreferences(); err == nil {
		t.Fatal("非 NotExist 读取错误必须上报")
	}
	if a.currentPrefsIssue().Kind != prefsIssuePrefsBroken {
		t.Fatalf("读取错误必须建立持久写门: %+v", a.currentPrefsIssue())
	}
	if err := a.updatePreferences(func(p *preferences) { p.Appearance.ColorTheme = "paper" }); err != nil {
		t.Fatal(err)
	}
	if err := a.shutdownPreferences(); err != nil {
		t.Fatal(err)
	}
	st, err := os.Stat(path)
	if err != nil || !st.IsDir() {
		t.Fatalf("shutdown 不得覆盖造成读取错误的原路径: stat=%v err=%v", st, err)
	}
}

func TestPrefsReadErrorRecoveryRemainsExplicit(t *testing.T) {
	a := newTestApp(t)
	path := a.preferencesPath()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := a.loadPreferences(); err == nil {
		t.Fatal("目录占位必须造成读取错误")
	}
	if err := a.confirmRecoverPreferences(); err == nil {
		t.Fatal("原件仍不可读时确认恢复必须失败")
	}
	if a.currentPrefsIssue().Kind != prefsIssuePrefsBroken {
		t.Fatal("恢复失败不得清除写门")
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	persistWriteRaw(t, path, `{"version":1, bad`)
	if err := a.confirmRecoverPreferences(); err != nil {
		t.Fatalf("原件恢复可读后应可明确确认: %v", err)
	}
	if a.currentPrefsIssue().Kind != prefsIssueNone {
		t.Fatal("确认写入成功后才应清除写门")
	}
}

func TestLegacyConfigIssueBlocksFirstWriteAndFailedConfirmation(t *testing.T) {
	a := newTestApp(t)
	dir := filepath.Dir(a.notes.DataFile)
	persistWriteRaw(t, filepath.Join(dir, "config.toml"), "[broken")
	if err := a.loadPreferences(); err == nil {
		t.Fatal("损坏 legacy 配置必须上报")
	}
	if err := a.updatePreferences(func(p *preferences) { p.Appearance.ColorTheme = "paper" }); err != nil {
		t.Fatal(err)
	}
	if err := a.savePreferencesNow(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(a.preferencesPath()); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("确认前首次写入必须被门控: %v", err)
	}

	originalPath := a.prefsStore.Path
	blocker := filepath.Join(dir, "not-a-directory")
	persistWriteRaw(t, blocker, "preserve")
	a.prefsStore.Path = filepath.Join(blocker, "preferences.json")
	if err := a.confirmContinueWithoutLegacyConfig(); err == nil {
		t.Fatal("确认写入失败必须返回错误")
	}
	if a.currentPrefsIssue().Kind != prefsIssueLegacyBroken {
		t.Fatal("确认写入失败不得清除 legacy 写门")
	}
	a.prefsStore.Path = originalPath
	if err := a.confirmContinueWithoutLegacyConfig(); err != nil {
		t.Fatalf("修复写入路径后应可重试确认: %v", err)
	}
	raw := prefsReadRaw(t, originalPath)
	var saved preferences
	if err := json.Unmarshal(raw, &saved); err != nil {
		t.Fatal(err)
	}
	if got := saved.Appearance.ColorTheme; got != "paper" {
		t.Fatalf("确认写入应包含门控期间的用户修改: %q", got)
	}
}

func TestPrefsMigrationKeepsNormalizedNoteThemeFromRealNotesLoad(t *testing.T) {
	a := newTestApp(t)
	a.cfg.MarkdownTheme = "note"
	if err := a.saveNow(); err != nil {
		t.Fatal(err)
	}

	b := newTestApp(t)
	b.notes = newNotesStore(a.notes.DataFile, a.notes.BackupDir(), b.now)
	if err := b.load(); err != nil {
		t.Fatal(err)
	}
	if b.cfg.MarkdownTheme != "note" {
		t.Fatalf("笔记载入应先得到规范化 note，实际 %q", b.cfg.MarkdownTheme)
	}
	if err := b.loadPreferences(); err != nil {
		t.Fatal(err)
	}
	if b.prefs.Appearance.MarkdownTheme != "note" {
		t.Fatalf("note 基线迁入 preferences 后不得回退 sonnet: %q", b.prefs.Appearance.MarkdownTheme)
	}
}
