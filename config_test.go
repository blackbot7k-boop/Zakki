package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// 旧配置解码器仅供一次性迁移（prefs_runtime.go）；这些测试锁定其严格语义，
// 保证迁移不静默丢弃或曲解旧文件中的显式值。

func TestConfigDefaultsAndPartialOverrides(t *testing.T) {
	s := settings{ThemeSource: "dark", HideOnEscape: true, BodySize: 18, Source: true, AlwaysOnTop: true}
	defaults := defaultFileConfig(s)
	for _, raw := range []string{"", " \t\n\r\n", "# 仅注释配置\n# 保留已有偏好\n"} {
		got, err := decodeConfig([]byte(raw), defaults)
		if err != nil || got != defaults {
			t.Fatalf("空白或仅注释文件应保留所有默认值: %q: %+v, %v", raw, got, err)
		}
	}
	got, err := decodeConfig([]byte(`
[appearance]
bodySize = 21
lineHeight = 1.75
[window]
alwaysOnTop = false
hideOnEscape = false
[editor]
source = false
[shortcuts]
toggleWindow = ""
`), defaults)
	if err != nil {
		t.Fatal(err)
	}
	if got.Appearance.Theme != "dark" || got.Appearance.BodySize != 21 || got.Appearance.LineHeight != 1.75 ||
		got.Window.AlwaysOnTop || got.Window.HideOnEscape || got.Editor.Source || got.Shortcuts.ToggleWindow != "" ||
		got.Storage != defaults.Storage || got.Window.Width != 400 {
		t.Fatalf("覆盖/缺省/显式 false 语义错误: %+v", got)
	}
}

func TestConfigRejectInvalid(t *testing.T) {
	defaults := defaultFileConfig(newApp().cfg)
	for _, raw := range []string{
		`[`, `null`, `[]`, `true`, `appearance.bodySize =`, `unknown = 1`,
		`appearance.bodySzie = 18`, `[unknown]`, `editor = null`, `window.hideOnEscape = null`,
		`appearance.theme = "blue"`, `appearance.markdownTheme = "github"`,
		`appearance.bodyFont = "missing"`, `appearance.monoFont = "missing"`,
		`appearance.bodySize = 9`, `appearance.bodySize = 41`,
		`appearance.lineHeight = 0.5`, `appearance.lineHeight = 4`,
		`window.width = 299`, `window.height = 339`, `window.width = 4097`,
		`window.rememberBounds = "false"`, `storage.autosaveDelayMs = 0`,
		`storage.autosaveDelayMs = 10001`, `storage.backupKeep = 0`,
		`storage.backupKeep = 1001`, `storage.backupIntervalMinutes = -1`,
		`storage.backupIntervalMinutes = 1441`, `storage.backupKeep = 2.5`,
		`appearance.bodySize = "18"`, `appearance.lineHeight = true`, `appearance.theme = 1`,
		`editor = false`, `window = []`, `editor.source = 1`,
		"appearance.bodySize = 18\nappearance.bodySize = 20",
		"[appearance]\nbodySize = 18\nbodySize = 20",
		"[appearance]\nbodySize = 18\n[appearance]\nlineHeight = 1.5",
	} {
		t.Run(raw, func(t *testing.T) {
			if _, err := decodeConfig([]byte(raw), defaults); err == nil {
				t.Fatalf("不应接受 %s", raw)
			}
		})
	}
	for _, key := range []string{"bodySize", "lineHeight"} {
		for _, value := range []string{"nan", "+nan", "-nan", "inf", "+inf", "-inf"} {
			raw := "appearance." + key + " = " + value
			t.Run(raw, func(t *testing.T) {
				if _, err := decodeConfig([]byte(raw), defaults); err == nil {
					t.Fatalf("字号和行高必须拒绝非有限值: %s", raw)
				}
			})
		}
	}
	for _, raw := range []string{
		"appearance.bodySize = 0\nappearance.lineHeight = 0",
		`[appearance]
bodySize = 10
lineHeight = 1
[window]
width = 300
height = 340
[storage]
autosaveDelayMs = 100
backupIntervalMinutes = 1
backupKeep = 1`,
		`[appearance]
bodySize = 40
lineHeight = 3
[window]
width = 4096
height = 4096
[storage]
autosaveDelayMs = 10000
backupIntervalMinutes = 1440
backupKeep = 1000`,
	} {
		t.Run(raw, func(t *testing.T) {
			if _, err := decodeConfig([]byte(raw), defaults); err != nil {
				t.Fatalf("边界值应被接受: %s: %v", raw, err)
			}
		})
	}
}

func TestLegacyJSONRejectInvalid(t *testing.T) {
	defaults := defaultFileConfig(newApp().cfg)
	for _, raw := range []string{
		`{bad`, `null`, `[]`, `{} {}`, `{"appearance":{"theme":"blue"}}`,
		`{"storage":{"backupKeep":0}}`, `{"window":{"hideOnEscape":null}}`,
		`{"unknown":true}`,
	} {
		t.Run(raw, func(t *testing.T) {
			if _, err := decodeLegacyConfig([]byte(raw), defaults); err == nil {
				t.Fatalf("旧 JSON 不应接受 %s", raw)
			}
		})
	}
}

func TestConfigDecodeRoundTripAllFields(t *testing.T) {
	want := fileConfig{
		Appearance: appearanceConfig{Theme: "dark", MarkdownTheme: "tot", BodyFont: "songti", MonoFont: "menlo", BodySize: 21.5, LineHeight: 1.75},
		Editor:     editorConfig{Source: true},
		Window:     windowConfig{AlwaysOnTop: true, HideOnEscape: false, Width: 721, Height: 841, RememberBounds: false},
		Shortcuts:  shortcutConfig{ToggleWindow: "CmdOrCtrl+Alt+Space"},
		Storage:    storageConfig{AutosaveDelayMS: 255, BackupIntervalMinutes: 17, BackupKeep: 45, BackupOnLaunch: false},
	}
	for _, key := range []string{"CmdOrCtrl+Alt+Space", "", "引号\"、反斜线\\和换行\n不能逃逸配置表"} {
		want.Shortcuts.ToggleWindow = key
		raw, err := json.Marshal(want)
		if err != nil {
			t.Fatal(err)
		}
		got, err := decodeLegacyConfig(raw, defaultFileConfig(settings{}))
		if err != nil {
			t.Fatalf("JSON 迁移解码失败: %v", err)
		}
		if got != want {
			t.Fatalf("全部配置值必须无损往返: %+v != %+v", got, want)
		}
	}
	if !strings.Contains(string(mustJSON(t, want)), "toggleWindow") {
		t.Fatal("sanity")
	}
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
