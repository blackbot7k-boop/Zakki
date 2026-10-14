package prefs

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"

	"tot-md-mygo/internal/legacyconfig"
)

// CurrentVersion 是当前程序读写的偏好版本。
const CurrentVersion = 1

// AccelValidator 校验 keymap 覆盖中的 accelerator 字符串。
type AccelValidator func(acc string) error

// ToggleMigrator 把旧 config 里的全局热键迁移为规范化 accelerator。
type ToggleMigrator func(raw string) (canonical string, err error)

// Preferences 是 preferences.json 的磁盘模型。
type Preferences struct {
	Version    int        `json:"version"`
	Appearance Appearance `json:"appearance"`
	Window     Window     `json:"window"`
	Backup     Backup     `json:"backup"`
	Keymap     Keymap     `json:"keymap"`
	Advanced   Advanced   `json:"advanced"`
}

type Appearance struct {
	AppearanceMode string  `json:"appearanceMode"`
	ColorTheme     string  `json:"colorTheme"`
	MarkdownTheme  string  `json:"markdownTheme"`
	BodyFont       string  `json:"bodyFont"`
	MonoFont       string  `json:"monoFont"`
	BodySize       float32 `json:"bodySize"`
	LineHeight     float32 `json:"lineHeight"`
}

type Window struct {
	AlwaysOnTop    bool `json:"alwaysOnTop"`
	HideOnEscape   bool `json:"hideOnEscape"`
	RememberBounds bool `json:"rememberBounds"`
}

type Backup struct {
	IntervalMinutes int  `json:"intervalMinutes"`
	Keep            int  `json:"keep"`
	OnLaunch        bool `json:"onLaunch"`
}

type Keymap struct {
	Overrides map[string]string `json:"overrides"`
}

type Advanced struct {
	AutosaveDelayMS int `json:"autosaveDelayMs"`
	WindowWidth     int `json:"windowWidth"`
	WindowHeight    int `json:"windowHeight"`
}

// Default 全新安装的默认值。
func Default(backupIntervalMinutes int) Preferences {
	return Preferences{
		Version: CurrentVersion,
		Appearance: Appearance{
			AppearanceMode: "system",
			ColorTheme:     "tot",
			MarkdownTheme:  "sonnet",
		},
		Window: Window{AlwaysOnTop: false, HideOnEscape: true, RememberBounds: true},
		Backup: Backup{IntervalMinutes: backupIntervalMinutes, Keep: 20, OnLaunch: true},
		Keymap: Keymap{Overrides: map[string]string{}},
		Advanced: Advanced{
			AutosaveDelayMS: 800,
			WindowWidth:     400,
			WindowHeight:    540,
		},
	}
}

// Clone 深拷贝 Keymap.Overrides。
func (p Preferences) Clone() Preferences {
	out := p
	out.Keymap.Overrides = make(map[string]string, len(p.Keymap.Overrides))
	for k, v := range p.Keymap.Overrides {
		out.Keymap.Overrides[k] = v
	}
	return out
}

// Validate 校验全部取值范围。
func (p Preferences) Validate(v AccelValidator) error {
	for _, field := range []struct {
		name, value string
		allowed     []string
	}{
		{"appearance.appearanceMode", p.Appearance.AppearanceMode, []string{"system", "light", "dark"}},
		{"appearance.colorTheme", p.Appearance.ColorTheme, []string{"tot", "paper", "warmSand", "mistTeal", "graphite"}},
		{"appearance.markdownTheme", p.Appearance.MarkdownTheme, []string{"sonnet", "tot", "clear", "book", "compact"}},
		{"appearance.bodyFont", p.Appearance.BodyFont, []string{"", "pingfang", "songti"}},
		{"appearance.monoFont", p.Appearance.MonoFont, []string{"", "sfmono", "menlo"}},
	} {
		valid := false
		for _, allowed := range field.allowed {
			valid = valid || field.value == allowed
		}
		if !valid {
			return fmt.Errorf("%s 的值 %q 无效，可选值：%q", field.name, field.value, field.allowed)
		}
	}
	for _, field := range []struct {
		name          string
		value, lo, hi float32
		allowFollow   bool
	}{
		{"appearance.bodySize", p.Appearance.BodySize, 10, 40, true},
		{"appearance.lineHeight", p.Appearance.LineHeight, 1, 3, true},
	} {
		if field.allowFollow && field.value == 0 {
			continue
		}
		if math.IsNaN(float64(field.value)) || math.IsInf(float64(field.value), 0) || field.value < field.lo || field.value > field.hi {
			return fmt.Errorf("%s 必须在 %g–%g 之间（0 表示跟随主题）", field.name, field.lo, field.hi)
		}
	}
	for _, field := range []struct {
		name          string
		value, lo, hi int
	}{
		{"backup.intervalMinutes", p.Backup.IntervalMinutes, 1, 1440},
		{"backup.keep", p.Backup.Keep, 1, 1000},
		{"advanced.autosaveDelayMs", p.Advanced.AutosaveDelayMS, 100, 10000},
		{"advanced.windowWidth", p.Advanced.WindowWidth, 300, 4096},
		{"advanced.windowHeight", p.Advanced.WindowHeight, 340, 4096},
	} {
		if field.value < field.lo || field.value > field.hi {
			return fmt.Errorf("%s 必须在 %d–%d 之间", field.name, field.lo, field.hi)
		}
	}
	if v != nil {
		for id, acc := range p.Keymap.Overrides {
			if acc == "" {
				continue
			}
			if err := v(acc); err != nil {
				return fmt.Errorf("keymap.overrides[%s] 的快捷键 %q 无效: %w", id, acc, err)
			}
		}
	}
	return nil
}

// Decode 严格解码偏好文件。
func Decode(raw []byte, v AccelValidator) (Preferences, error) {
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return Preferences{}, err
	}
	if _, ok := value.(map[string]any); !ok {
		return Preferences{}, errors.New("偏好必须是 JSON 对象")
	}
	if err := rejectJSONNull(value, "preferences"); err != nil {
		return Preferences{}, err
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var p Preferences
	if err := dec.Decode(&p); err != nil {
		return Preferences{}, err
	}
	if p.Keymap.Overrides == nil {
		p.Keymap.Overrides = map[string]string{}
	}
	if p.Version == 0 {
		p.Version = CurrentVersion
	}
	if p.Version > CurrentVersion {
		return Preferences{}, fmt.Errorf("偏好版本 %d 高于当前支持的 %d（文件保持原样）", p.Version, CurrentVersion)
	}
	def := Default(5)
	if p.Appearance.AppearanceMode == "" {
		p.Appearance.AppearanceMode = def.Appearance.AppearanceMode
	}
	if p.Appearance.ColorTheme == "" {
		p.Appearance.ColorTheme = def.Appearance.ColorTheme
	}
	if p.Appearance.MarkdownTheme == "" {
		p.Appearance.MarkdownTheme = def.Appearance.MarkdownTheme
	}
	if p.Backup.IntervalMinutes == 0 {
		p.Backup.IntervalMinutes = def.Backup.IntervalMinutes
	}
	if p.Backup.Keep == 0 {
		p.Backup.Keep = def.Backup.Keep
	}
	if p.Advanced.AutosaveDelayMS == 0 {
		p.Advanced.AutosaveDelayMS = def.Advanced.AutosaveDelayMS
	}
	if p.Advanced.WindowWidth == 0 {
		p.Advanced.WindowWidth = def.Advanced.WindowWidth
	}
	if p.Advanced.WindowHeight == 0 {
		p.Advanced.WindowHeight = def.Advanced.WindowHeight
	}
	if err := p.Validate(v); err != nil {
		return Preferences{}, err
	}
	return p, nil
}

func rejectJSONNull(value any, path string) error {
	if value == nil {
		return fmt.Errorf("%s 不允许 null", path)
	}
	if fields, ok := value.(map[string]any); ok {
		for key, child := range fields {
			if err := rejectJSONNull(child, path+"."+key); err != nil {
				return err
			}
		}
	}
	return nil
}

// MigrationResult 记录一次性迁移的结论。
type MigrationResult struct {
	From    string
	Applied bool
	Err     error
}

// FromNotes 用笔记内旧设置补默认偏好。
func FromNotes(b legacyconfig.NotesBaseline, backupIntervalMinutes int) Preferences {
	p := Default(backupIntervalMinutes)
	if v := b.ThemeSource; v != "" {
		p.Appearance.AppearanceMode = v
	}
	switch b.MarkdownTheme {
	case "tot":
		p.Appearance.MarkdownTheme = "tot"
	default:
		p.Appearance.MarkdownTheme = "sonnet"
	}
	p.Appearance.BodyFont, p.Appearance.MonoFont = b.BodyFont, b.MonoFont
	p.Appearance.BodySize, p.Appearance.LineHeight = b.BodySize, b.LineHeight
	p.Window.AlwaysOnTop = b.AlwaysOnTop
	p.Window.HideOnEscape = b.HideOnEscape
	return p
}

// MergeFileConfig 把旧配置叠加到已有偏好上。
func MergeFileConfig(cfg legacyconfig.Config, base Preferences, cmdToggleWindow string, migrateToggle ToggleMigrator) Preferences {
	p := base
	p.Appearance.AppearanceMode = cfg.Appearance.Theme
	switch cfg.Appearance.MarkdownTheme {
	case "tot":
		p.Appearance.MarkdownTheme = "tot"
	case "":
	default:
		p.Appearance.MarkdownTheme = "sonnet"
	}
	p.Appearance.BodyFont, p.Appearance.MonoFont = cfg.Appearance.BodyFont, cfg.Appearance.MonoFont
	p.Appearance.BodySize, p.Appearance.LineHeight = cfg.Appearance.BodySize, cfg.Appearance.LineHeight
	p.Window.AlwaysOnTop = cfg.Window.AlwaysOnTop
	p.Window.HideOnEscape = cfg.Window.HideOnEscape
	p.Window.RememberBounds = cfg.Window.RememberBounds
	p.Advanced.WindowWidth, p.Advanced.WindowHeight = cfg.Window.Width, cfg.Window.Height
	p.Backup.IntervalMinutes, p.Backup.Keep, p.Backup.OnLaunch =
		cfg.Storage.BackupIntervalMinutes, cfg.Storage.BackupKeep, cfg.Storage.BackupOnLaunch
	p.Advanced.AutosaveDelayMS = cfg.Storage.AutosaveDelayMS
	if cfg.Shortcuts.ToggleWindow != "" {
		if migrateToggle != nil {
			if acc, err := migrateToggle(cfg.Shortcuts.ToggleWindow); err == nil {
				p.Keymap.Overrides[cmdToggleWindow] = acc
			}
		}
	} else {
		p.Keymap.Overrides[cmdToggleWindow] = ""
	}
	p.Version = CurrentVersion
	return p
}

// MigrateLegacy 读取旧配置目录并迁移为偏好。
func MigrateLegacy(dir string, baseline legacyconfig.NotesBaseline, backupIntervalMinutes int, cmdToggle string, migrateToggle ToggleMigrator) (Preferences, MigrationResult) {
	p := FromNotes(baseline, backupIntervalMinutes)
	def := legacyconfig.Default(baseline, backupIntervalMinutes)
	tomlPath := filepath.Join(dir, "config.toml")
	if raw, err := os.ReadFile(tomlPath); err == nil {
		cfg, err := legacyconfig.DecodeTOML(raw, def)
		if err != nil {
			return p, MigrationResult{From: "config.toml", Err: fmt.Errorf("配置文件 config.toml 无效: %w", err)}
		}
		return MergeFileConfig(cfg, p, cmdToggle, migrateToggle), MigrationResult{From: "config.toml", Applied: true}
	} else if !errors.Is(err, os.ErrNotExist) {
		return p, MigrationResult{From: "config.toml", Err: fmt.Errorf("读取配置文件 config.toml: %w", err)}
	}
	jsonPath := filepath.Join(dir, "config.json")
	if raw, err := os.ReadFile(jsonPath); err == nil {
		cfg, err := legacyconfig.DecodeJSON(raw, def)
		if err != nil {
			return p, MigrationResult{From: "config.json", Err: fmt.Errorf("旧配置 config.json 无法迁移: %w", err)}
		}
		return MergeFileConfig(cfg, p, cmdToggle, migrateToggle), MigrationResult{From: "config.json", Applied: true}
	} else if !errors.Is(err, os.ErrNotExist) {
		return p, MigrationResult{From: "config.json", Err: fmt.Errorf("读取旧配置 config.json: %w", err)}
	}
	return p, MigrationResult{}
}
