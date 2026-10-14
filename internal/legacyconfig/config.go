package legacyconfig

// 旧配置文件（config.toml / config.json）的解码器：仅供一次性迁移读取。

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"

	"github.com/pelletier/go-toml/v2"
)

// NotesBaseline 迁移时「笔记内旧 settings」对应的缺省基线（不含 Source 等会话字段）。
type NotesBaseline struct {
	ThemeSource   string
	MarkdownTheme string
	BodyFont      string
	MonoFont      string
	BodySize      float32
	LineHeight    float32
	AlwaysOnTop   bool
	HideOnEscape  bool
}

// Config 是旧配置文件的磁盘格式（TOML 与旧版 JSON 同构）。
type Config struct {
	Appearance Appearance `json:"appearance" toml:"appearance"`
	Editor     Editor     `json:"editor" toml:"editor"`
	Window     Window     `json:"window" toml:"window"`
	Shortcuts  Shortcuts  `json:"shortcuts" toml:"shortcuts"`
	Storage    Storage    `json:"storage" toml:"storage"`
}

type Appearance struct {
	Theme         string  `json:"theme" toml:"theme"`
	MarkdownTheme string  `json:"markdownTheme" toml:"markdownTheme"`
	BodyFont      string  `json:"bodyFont" toml:"bodyFont"`
	MonoFont      string  `json:"monoFont" toml:"monoFont"`
	BodySize      float32 `json:"bodySize" toml:"bodySize"`
	LineHeight    float32 `json:"lineHeight" toml:"lineHeight"`
}

type Editor struct {
	Source bool `json:"source" toml:"source"`
}

type Window struct {
	AlwaysOnTop    bool `json:"alwaysOnTop" toml:"alwaysOnTop"`
	HideOnEscape   bool `json:"hideOnEscape" toml:"hideOnEscape"`
	Width          int  `json:"width" toml:"width"`
	Height         int  `json:"height" toml:"height"`
	RememberBounds bool `json:"rememberBounds" toml:"rememberBounds"`
}

type Shortcuts struct {
	ToggleWindow string `json:"toggleWindow" toml:"toggleWindow"`
}

type Storage struct {
	AutosaveDelayMS       int  `json:"autosaveDelayMs" toml:"autosaveDelayMs"`
	BackupIntervalMinutes int  `json:"backupIntervalMinutes" toml:"backupIntervalMinutes"`
	BackupKeep            int  `json:"backupKeep" toml:"backupKeep"`
	BackupOnLaunch        bool `json:"backupOnLaunch" toml:"backupOnLaunch"`
}

// Default 用笔记内旧设置补缺，新增运行参数保持原有行为。
func Default(b NotesBaseline, backupIntervalMinutes int) Config {
	return Config{
		Appearance: Appearance{
			Theme: b.ThemeSource, MarkdownTheme: b.MarkdownTheme,
			BodyFont: b.BodyFont, MonoFont: b.MonoFont, BodySize: b.BodySize, LineHeight: b.LineHeight,
		},
		Editor: Editor{Source: false},
		Window: Window{
			AlwaysOnTop: b.AlwaysOnTop, HideOnEscape: b.HideOnEscape,
			Width: 400, Height: 540, RememberBounds: true,
		},
		Shortcuts: Shortcuts{ToggleWindow: "CmdOrCtrl+Shift+Space"},
		Storage: Storage{
			AutosaveDelayMS: 800, BackupIntervalMinutes: backupIntervalMinutes,
			BackupKeep: 20, BackupOnLaunch: true,
		},
	}
}

// DecodeTOML 在已有缺省上覆盖 TOML 显式字段，严格拒绝未知配置项。
func DecodeTOML(raw []byte, defaults Config) (Config, error) {
	dec := toml.NewDecoder(bytes.NewReader(raw)).DisallowUnknownFields()
	if err := dec.Decode(&defaults); err != nil {
		return Config{}, err
	}
	if err := defaults.validate(); err != nil {
		return Config{}, err
	}
	return defaults, nil
}

// DecodeJSON 只用于首次迁移旧 JSON，沿用旧版的严格校验。
func DecodeJSON(raw []byte, defaults Config) (Config, error) {
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return Config{}, err
	}
	if _, ok := value.(map[string]any); !ok {
		return Config{}, errors.New("配置必须是 JSON 对象")
	}
	if err := rejectNull(value, "config"); err != nil {
		return Config{}, err
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&defaults); err != nil {
		return Config{}, err
	}
	if err := defaults.validate(); err != nil {
		return Config{}, err
	}
	return defaults, nil
}

func rejectNull(value any, path string) error {
	if value == nil {
		return fmt.Errorf("%s 不允许 null；请删除该字段以使用默认值", path)
	}
	if fields, ok := value.(map[string]any); ok {
		for key, child := range fields {
			if err := rejectNull(child, path+"."+key); err != nil {
				return err
			}
		}
	}
	return nil
}

func (c Config) validate() error {
	for _, field := range []struct {
		name, value string
		allowed     []string
	}{
		{"appearance.theme", c.Appearance.Theme, []string{"system", "light", "dark"}},
		{"appearance.markdownTheme", c.Appearance.MarkdownTheme, []string{"", "sonnet", "tot"}},
		{"appearance.bodyFont", c.Appearance.BodyFont, []string{"", "pingfang", "songti"}},
		{"appearance.monoFont", c.Appearance.MonoFont, []string{"", "sfmono", "menlo"}},
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
		allowZero     bool
	}{
		{"appearance.bodySize", c.Appearance.BodySize, 10, 40, true},
		{"appearance.lineHeight", c.Appearance.LineHeight, 1, 3, true},
		{"window.width", float32(c.Window.Width), 300, 4096, false},
		{"window.height", float32(c.Window.Height), 340, 4096, false},
		{"storage.autosaveDelayMs", float32(c.Storage.AutosaveDelayMS), 100, 10000, false},
		{"storage.backupIntervalMinutes", float32(c.Storage.BackupIntervalMinutes), 1, 1440, false},
		{"storage.backupKeep", float32(c.Storage.BackupKeep), 1, 1000, false},
	} {
		if field.allowZero && field.value == 0 {
			continue
		}
		if math.IsNaN(float64(field.value)) || math.IsInf(float64(field.value), 0) || field.value < field.lo || field.value > field.hi {
			return fmt.Errorf("%s 必须在 %g–%g 之间", field.name, field.lo, field.hi)
		}
	}
	return nil
}
