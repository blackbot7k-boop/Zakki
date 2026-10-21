package prefs

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"

	"zakki/internal/fonts"
	"zakki/internal/legacyconfig"
)

// CurrentVersion 是当前程序读写的偏好版本。
// 版本 2 起 appearance 分开记录中文字体（bodyFont）与西文字体（latinFont）。
// 版本 3 起 backup 记录外部备份目的地（mirrorDir）与 Git 自动化模式（gitMode）；
// 旧版本程序读到版本 3 会按「版本超前」拒绝并保留原文件。
// 版本 4 起主题 ID 更名：colorTheme "tot" → "classic"、markdownTheme "tot" → "note"。
// 版本 5 起代码块配色默认「柔和」：旧版本写入方从未写过 codeTheme 键，升级时统一补为
// soft；版本 5 起该键始终落盘，空串是显式的「跟随文本」，不再参与补默认。
// 版本 6 起备份只有一个文件夹：backup.mirrorDir 更名为 backup.backupDir 并成为
// 唯一备份存储地址；gitMode 随 Git 自动化一并移除，旧键由迁移读取后丢弃。
// 版本 7 起代码块缩进只有 2 / 4 空格两档（appearance.codeIndent，默认 2 空格）；
// 旧文件从未写过该键，升级时统一补默认。
// 版本 8 扩展既有字体键：支持 local:<真实家族名>，西文与代码字体另支持 fira；
// 不新增 JSON 字段，不改默认字体或触发下载。
// 版本 9 起备份文件夹支持 Git 自动化：backup.gitAutoPush（开关，默认关）与
// backup.gitPushMinutes（自动推送最小间隔，默认 10，1–1440）。旧文件从未写过
// 这两个键，迁移时统一补默认；无破坏性变更。
// 版本 10 新增 appearance.style（"standard" / "sketch"）：手账是整体风格，
// colorTheme / markdownTheme 不再取 "sketch"，只记标准风格下的选择，切回标准时原样恢复。
const CurrentVersion = 10

// 整体风格：手账风格下颜色与排版由风格决定，colorTheme / markdownTheme 保留不动。
const (
	StyleStandard = "standard"
	StyleSketch   = "sketch"
)

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
	AppearanceMode string `json:"appearanceMode"`
	// Style 是整体风格；手账风格下实际生效的颜色与排版见 EffectiveColorTheme /
	// EffectiveMarkdownTheme。
	Style         string `json:"style"`
	ColorTheme    string `json:"colorTheme"`
	MarkdownTheme string `json:"markdownTheme"`
	// Language 是界面语言；空串 = 跟随系统（MyGo 按用户系统语言挑目录语言）。
	Language string `json:"language,omitempty"`
	// CodeTheme 是代码块着色方案；空串 = 跟随文本（不上色）。不能加 omitempty：
	// 空串必须落盘，否则「显式跟随文本」与「从未设置」在下次解码时无法区分。
	CodeTheme string `json:"codeTheme"`
	// CodeIndent 是代码块内 Tab 插入的缩进档位："space2"（默认）/ "space4"
	// 空格；Shift+Tab 按同一档位反向移除。
	CodeIndent string `json:"codeIndent"`
	// BodyFont 是中文字体；LatinFont 是英文字体，空串表示跟随中文字体。
	BodyFont   string  `json:"bodyFont"`
	LatinFont  string  `json:"latinFont"`
	MonoFont   string  `json:"monoFont"`
	BodySize   float32 `json:"bodySize"`
	LineHeight float32 `json:"lineHeight"`
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
	// BackupDir 是备份文件夹：唯一的备份存储地址，每次备份直写这里。
	// 空串 = 未选择，写入内置目录（应用数据 backups/）。可选云盘同步
	// 文件夹（iCloud、坚果云等），即云备份。
	BackupDir string `json:"backupDir"`
	// GitAutoPush 是备份文件夹为 git 仓库时是否自动提交并推送。提交随每次
	// 备份进行，推送按 GitPushMinutes 节流。GitAutoPush 只在该文件夹真的是
	// git 仓库时生效。
	GitAutoPush bool `json:"gitAutoPush"`
	// GitPushMinutes 是两次自动推送尝试之间的最小间隔（分钟）。
	GitPushMinutes int `json:"gitPushMinutes"`
}

type Keymap struct {
	Overrides map[string]string `json:"overrides"`
}

type Advanced struct {
	AutosaveDelayMS int `json:"autosaveDelayMs"`
	WindowWidth     int `json:"windowWidth"`
	WindowHeight    int `json:"windowHeight"`
}

// defaultGitPushMinutes 是 Git 自动推送间隔的默认值（分钟）。
const defaultGitPushMinutes = 10

// Default 全新安装的默认值。
func Default(backupIntervalMinutes int) Preferences {
	return Preferences{
		Version: CurrentVersion,
		Appearance: Appearance{
			AppearanceMode: "system",
			Style:          StyleStandard,
			ColorTheme:     "classic",
			MarkdownTheme:  "sonnet",
			CodeTheme:      "soft",
			CodeIndent:     "space2",
			LatinFont:      "jetbrains",
		},
		Window: Window{AlwaysOnTop: false, HideOnEscape: true, RememberBounds: true},
		Backup: Backup{IntervalMinutes: backupIntervalMinutes, Keep: 20, OnLaunch: true, GitAutoPush: false, GitPushMinutes: defaultGitPushMinutes},
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
		{"appearance.language", p.Appearance.Language, []string{"", "en", "zh-Hans"}},
		{"appearance.style", p.Appearance.Style, []string{StyleStandard, StyleSketch}},
		{"appearance.colorTheme", p.Appearance.ColorTheme, []string{"classic", "paper", "warmSand", "mistTeal", "graphite", "claude", "qoder"}},
		{"appearance.markdownTheme", p.Appearance.MarkdownTheme, []string{"sonnet", "note", "clear", "book", "compact"}},
		{"appearance.codeTheme", p.Appearance.CodeTheme, []string{"", "soft", "vivid", "claude", "qoder"}},
		{"appearance.codeIndent", p.Appearance.CodeIndent, []string{"space2", "space4"}},
		{"appearance.bodyFont", p.Appearance.BodyFont, []string{"", "kai", "sonnet", "pingfang", "songti", "hand", "hannotate"}},
		{"appearance.latinFont", p.Appearance.LatinFont, []string{"", "jetbrains", "system", "noteworthy", "bradley", "fira"}},
		{"appearance.monoFont", p.Appearance.MonoFont, []string{"", "jbm", "sonnet", "sfmono", "menlo", "fira"}},
	} {
		valid := false
		for _, allowed := range field.allowed {
			valid = valid || field.value == allowed
		}
		if !valid {
			switch field.name {
			case "appearance.bodyFont", "appearance.latinFont", "appearance.monoFont":
				valid = fonts.LocalFamily(field.value) != ""
			}
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
		{"backup.gitPushMinutes", p.Backup.GitPushMinutes, 1, 1440},
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
	legacyMirrorDir, hasLegacy := stripLegacyBackupKeys(value)
	if hasLegacy {
		// 严格解码不认旧键；重新编回 JSON 再解码。map 编码只影响键序，
		// 不改变取值，原文件保持原样。
		sanitized, err := json.Marshal(value)
		if err != nil {
			return Preferences{}, err
		}
		raw = sanitized
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
	if p.Version > CurrentVersion {
		return Preferences{}, fmt.Errorf("偏好版本 %d 高于当前支持的 %d（文件保持原样）", p.Version, CurrentVersion)
	}
	p = migrate(p)
	if hasLegacy {
		// 版本 6 起备份文件夹是唯一备份地址：旧 mirrorDir 平移为新键。
		p.Backup.BackupDir = legacyMirrorDir
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
	if p.Appearance.Style == "" {
		p.Appearance.Style = def.Appearance.Style
	}
	// codeTheme 不做空串补默认：空串是显式「跟随文本」，默认柔和由版本 5 迁移承担。
	// codeIndent 只认 2 / 4 空格两档：版本 7 早期内部构建写过的 tab / space8
	// 档位（未发布）归为 4 空格，缺键或空串补默认，非法取值仍由 Validate 拒绝。
	if p.Appearance.CodeIndent == "tab" || p.Appearance.CodeIndent == "space8" {
		p.Appearance.CodeIndent = "space2"
	}
	if p.Appearance.CodeIndent == "" {
		p.Appearance.CodeIndent = def.Appearance.CodeIndent
	}
	if p.Backup.IntervalMinutes == 0 {
		p.Backup.IntervalMinutes = def.Backup.IntervalMinutes
	}
	if p.Backup.Keep == 0 {
		p.Backup.Keep = def.Backup.Keep
	}
	// gitPushMinutes 缺键（含版本 9 之前的文件）补默认；0 本身不合法，
	// 由 Validate 拒绝，不会与显式值混淆。
	if p.Backup.GitPushMinutes == 0 {
		p.Backup.GitPushMinutes = def.Backup.GitPushMinutes
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

// migrate 把旧版本偏好升级到 CurrentVersion。版本 1 只有一个正文字体键，
// 西文当时取自同一套字体栈，因此升级后 latinFont 留空（跟随中文字体），
// 渲染结果与升级前完全一致。版本 4 把更名前的主题 ID（"tot"）映射到新 ID
// （"classic" / "note"），主题取值不变、只换名字。版本 5 给从未设置过代码
// 配色的旧文件补默认方案 soft；带值文件原样保留。版本 6 的 mirrorDir →
// backupDir 键更名发生在严格解码前（见 Decode），这里只承载版本号提升。
// 版本 7 给从未写过 codeIndent 的旧文件补默认 2 空格。
// 版本 8 仅扩展字体键取值，保留字体值与此前迁移行为。
// 版本 9 的 Git 键由默认值补齐（gitAutoPush 关、间隔 10 分钟）：自动推送会把
// 笔记送到远端，只能由用户在设置页显式开启，迁移不替用户打开。
// 版本 10 把颜色「手绘」+ 排版「手账」的整套组合迁为手账风格，颜色与排版回到
// 标准默认；只有一项是手绘/手账的半套组合迁为标准风格，那一项回到默认。
func migrate(p Preferences) Preferences {
	if p.Version < 2 {
		p.Appearance.LatinFont = ""
	}
	if p.Version < 4 {
		if p.Appearance.ColorTheme == legacyThemeTot {
			p.Appearance.ColorTheme = "classic"
		}
		if p.Appearance.MarkdownTheme == legacyThemeTot {
			p.Appearance.MarkdownTheme = "note"
		}
	}
	if p.Version < 5 && p.Appearance.CodeTheme == "" {
		// 版本 4 及更早的写入方从未写过 codeTheme 键，空值即「从未设置」，
		// 补默认方案；带值的旧文件（含显式 soft/vivid）原样保留。
		p.Appearance.CodeTheme = "soft"
	}
	if p.Version < 7 && p.Appearance.CodeIndent == "" {
		// 版本 6 及更早的写入方从未写过 codeIndent 键，升级时补默认 2 空格。
		p.Appearance.CodeIndent = "space2"
	}
	if p.Version < 10 {
		def := Default(5).Appearance
		p.Appearance.Style = StyleStandard
		if p.Appearance.ColorTheme == StyleSketch && p.Appearance.MarkdownTheme == StyleSketch {
			p.Appearance.Style = StyleSketch
		}
		if p.Appearance.ColorTheme == StyleSketch {
			p.Appearance.ColorTheme = def.ColorTheme
		}
		if p.Appearance.MarkdownTheme == StyleSketch {
			p.Appearance.MarkdownTheme = def.MarkdownTheme
		}
	}
	p.Version = CurrentVersion
	return p
}

// EffectiveColorTheme 实际生效的颜色主题：手账风格固定为手绘点阵纸。
func (a Appearance) EffectiveColorTheme() string {
	if a.Style == StyleSketch {
		return "sketch"
	}
	return a.ColorTheme
}

// EffectiveMarkdownTheme 实际生效的排版主题：手账风格固定为手账排版。
func (a Appearance) EffectiveMarkdownTheme() string {
	if a.Style == StyleSketch {
		return "sketch"
	}
	return a.MarkdownTheme
}

// legacyThemeTot 是主题 ID 更名前旧文件里的写法；仅用于识别旧值并映射新 ID。
const legacyThemeTot = "tot"

// stripLegacyBackupKeys 从已解码的偏好映射里摘除版本 6 移除的旧备份键，
// 返回旧 mirrorDir 的值与是否摘除了键；gitMode 随 Git 自动化移除，直接丢弃。
// 版本 6 及以上的文件不应再出现旧键，保持原样交给严格解码拒绝。
func stripLegacyBackupKeys(value any) (string, bool) {
	m, ok := value.(map[string]any)
	if !ok {
		return "", false
	}
	if ver, _ := m["version"].(float64); ver >= 6 {
		return "", false
	}
	b, ok := m["backup"].(map[string]any)
	if !ok {
		return "", false
	}
	dir, hasDir := b["mirrorDir"].(string)
	_, hasGit := b["gitMode"]
	if !hasDir && !hasGit {
		return "", false
	}
	delete(b, "mirrorDir")
	delete(b, "gitMode")
	return dir, true
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
	case legacyThemeTot, "note":
		p.Appearance.MarkdownTheme = "note"
	case "":
		// 保持默认值。
	default:
		p.Appearance.MarkdownTheme = "sonnet"
	}
	p.Appearance.BodyFont, p.Appearance.MonoFont = b.BodyFont, b.MonoFont
	if b.BodyFont != "" {
		// 旧版没有单独的英文字体：选过中文字体的用户保持跟随中文字体，渲染与升级前一致。
		p.Appearance.LatinFont = ""
	}
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
	case legacyThemeTot:
		p.Appearance.MarkdownTheme = "note"
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
