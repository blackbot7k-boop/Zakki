package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// preferences.json 是用户偏好的唯一来源；全新安装直接写默认值，
// 不再导入旧 config.toml / config.json。

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

// TestPrefsFreshInstallIgnoresLegacyConfig 缺失 preferences.json 时写默认值；
// 同目录残留的旧 config 文件原样保留且不被读取。
func TestPrefsFreshInstallIgnoresLegacyConfig(t *testing.T) {
	a := newTestApp(t)
	dir := filepath.Dir(a.notes.DataPath())
	legacyTOML := "[appearance]\ncolorTheme = \"paper\"\nbodySize = 24\n"
	legacyJSON := `{"appearance":{"theme":"dark"},"storage":{"backupKeep":35}}`
	persistWriteRaw(t, filepath.Join(dir, "config.toml"), legacyTOML)
	persistWriteRaw(t, filepath.Join(dir, "config.json"), legacyJSON)

	if err := a.loadPreferences(); err != nil {
		t.Fatal(err)
	}
	if a.currentPrefsIssue().Kind != prefsIssueNone {
		t.Fatalf("全新安装不应有异常状态: %+v", a.currentPrefsIssue())
	}
	if a.prefs.Appearance != defaultPreferences().Appearance {
		t.Fatalf("应使用默认外观，不导入旧配置: %+v", a.prefs.Appearance)
	}
	if string(prefsReadRaw(t, filepath.Join(dir, "config.toml"))) != legacyTOML ||
		string(prefsReadRaw(t, filepath.Join(dir, "config.json"))) != legacyJSON {
		t.Fatal("旧配置文件必须原样保留")
	}
	// preferences.json 落盘且为当前版本。
	prefs, err := decodePreferences(prefsReadRaw(t, a.preferencesPath()))
	if err != nil || prefs.Version != currentPreferencesVersion {
		t.Fatalf("默认偏好应原子落盘: %+v, %v", prefs, err)
	}
}

// TestPrefsOldBackupFieldsRejected 旧版本的 backup / autosaveDelayMs 字段
// 按未知字段拒绝，不静默映射或丢弃。
func TestPrefsOldBackupFieldsRejected(t *testing.T) {
	for _, raw := range []string{
		`{"version":11,"backup":{"intervalMinutes":5,"keep":20}}`,
		`{"version":11,"advanced":{"autosaveDelayMs":800}}`,
	} {
		if _, err := decodePreferences([]byte(raw)); err == nil {
			t.Fatalf("应拒绝旧字段: %s", raw)
		}
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
	entries, err := os.ReadDir(filepath.Dir(a.notes.DataPath()))
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
	if a.prefs.Appearance.ColorTheme != "classic" {
		t.Fatalf("超前版本用默认值运行: %+v", a.prefs.Appearance)
	}
	if err := a.confirmRecoverPreferences(); err != nil {
		t.Fatal(err)
	}
	if a.prefs.Appearance.ColorTheme != "classic" {
		t.Fatalf("恢复后仍是默认值: %+v", a.prefs.Appearance)
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

func TestPrefsStorageFieldsApplyAndPersist(t *testing.T) {
	a := newTestApp(t)
	if err := a.loadPreferences(); err != nil {
		t.Fatal(err)
	}
	if err := a.updatePreferences(func(p *preferences) {
		p.Storage.SaveIntervalMS = 1500
		p.Git.AutoCommit = true
	}); err != nil {
		t.Fatal(err)
	}
	if a.saveInterval() != 1500*time.Millisecond {
		t.Fatalf("保存间隔未生效: %v", a.saveInterval())
	}
	a.shutdownPreferences()
	prefs, err := decodePreferences(prefsReadRaw(t, a.preferencesPath()))
	if err != nil || prefs.Storage.SaveIntervalMS != 1500 || !prefs.Git.AutoCommit || prefs.Git.AutoPush {
		t.Fatalf("存储偏好应持久化: %+v, %v", prefs, err)
	}
	// AutoPush 依赖 AutoCommit：单独打开推送必须拒绝。
	if err := a.updatePreferences(func(p *preferences) { p.Git.AutoCommit = false; p.Git.AutoPush = true }); err == nil {
		t.Fatal("无自动提交时打开自动推送必须拒绝")
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

func TestPrefsNoteThemeStaysInNotesFile(t *testing.T) {
	a := newTestApp(t)
	a.cfg.MarkdownTheme = "note"
	if err := a.saveNow(); err != nil {
		t.Fatal(err)
	}

	b := newTestApp(t)
	b.notes.Close()
	b.notes = newNotesStore(a.notes.DataPath(), b.now)
	if err := b.load(); err != nil {
		t.Fatal(err)
	}
	if b.cfg.MarkdownTheme != "note" {
		t.Fatalf("笔记载入应得到 note: %q", b.cfg.MarkdownTheme)
	}
	if err := b.loadPreferences(); err != nil {
		t.Fatal(err)
	}
	if b.prefs.Appearance.MarkdownTheme != "sonnet" {
		t.Fatalf("笔记里的排版主题不影响偏好默认: %+v", b.prefs.Appearance)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(prefsReadRaw(t, b.preferencesPath()), &raw); err != nil {
		t.Fatal(err)
	}
	if _, ok := raw["backup"]; ok {
		t.Fatal("偏好文件不应再写出 backup 字段")
	}
}
