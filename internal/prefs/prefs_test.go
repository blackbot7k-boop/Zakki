package prefs

import (
	"errors"
	"strings"
	"testing"
)

// testAccel 只拒绝含空格的绑定。本文件的非法样例是 "not a key+"；
// 空串由 Validate 跳过，表示解除绑定。生产路径的解析器仍由根目录集成测试覆盖。
func testAccel(acc string) error {
	if strings.Contains(acc, " ") {
		return errors.New("invalid accelerator")
	}
	return nil
}

func TestPreferencesDefaultsAndValidation(t *testing.T) {
	p := Default(5) // 与根目录 backupInterval（5 分钟）一致
	if err := p.Validate(testAccel); err != nil {
		t.Fatalf("默认偏好必须合法: %v", err)
	}
	if p.Appearance.ColorTheme != "classic" || p.Appearance.MarkdownTheme != "sonnet" || p.Appearance.AppearanceMode != "system" {
		t.Fatalf("全新安装默认：Tot 颜色 + Sonnet 排版 + 跟随系统: %+v", p.Appearance)
	}
	if !p.Window.HideOnEscape || !p.Window.RememberBounds {
		t.Fatalf("窗口默认值错误: %+v", p.Window)
	}
	if p.Backup.IntervalMinutes != 5 || p.Backup.Keep != 20 || !p.Backup.OnLaunch {
		t.Fatalf("备份默认值错误: %+v", p.Backup)
	}
	if p.Backup.BackupDir != "" {
		t.Fatalf("全新安装不应预选备份文件夹: %+v", p.Backup)
	}
	if p.Advanced.AutosaveDelayMS != 800 || p.Advanced.WindowWidth != 400 || p.Advanced.WindowHeight != 540 {
		t.Fatalf("兼容参数默认值错误: %+v", p.Advanced)
	}
	for _, mutate := range []func(*Preferences){
		func(p *Preferences) { p.Appearance.AppearanceMode = "blue" },
		func(p *Preferences) { p.Appearance.ColorTheme = "neon" },
		func(p *Preferences) { p.Appearance.MarkdownTheme = "github" },
		func(p *Preferences) { p.Appearance.BodyFont = "comic" },
		func(p *Preferences) { p.Appearance.BodySize = 9 },
		func(p *Preferences) { p.Appearance.BodySize = 41 },
		func(p *Preferences) { p.Appearance.LineHeight = 0.5 },
		func(p *Preferences) { p.Appearance.LineHeight = 3.1 },
		func(p *Preferences) { p.Backup.IntervalMinutes = 0 },
		func(p *Preferences) { p.Backup.IntervalMinutes = 1441 },
		func(p *Preferences) { p.Backup.Keep = 1001 },
		func(p *Preferences) { p.Advanced.AutosaveDelayMS = 99 },
		func(p *Preferences) { p.Advanced.WindowWidth = 299 },
		func(p *Preferences) { p.Advanced.WindowHeight = 339 },
		func(p *Preferences) { p.Keymap.Overrides["format.bold"] = "not a key+" },
	} {
		bad := Default(5)
		mutate(&bad)
		if err := bad.Validate(testAccel); err == nil {
			t.Fatalf("非法偏好应被拒绝: %+v", bad)
		}
	}
	// 0 = 跟随主题；显式空串 = 解除绑定。
	ok := Default(5)
	ok.Appearance.BodySize, ok.Appearance.LineHeight = 0, 0
	ok.Keymap.Overrides["format.bold"] = ""
	if err := ok.Validate(testAccel); err != nil {
		t.Fatalf("跟随主题值与显式空绑定必须合法: %v", err)
	}
}

func TestPreferencesDecodeStrict(t *testing.T) {
	for _, raw := range []string{
		`{`, `null`, `[]`, `{"version":1,}`,
		`{"unknown":1}`, `{"appearance":{"bodySize":99}}`,
		`{"appearance":null}`, `{"keymap":{"overrides":null}}`,
		`{"version":99}`,
	} {
		t.Run(raw, func(t *testing.T) {
			if _, err := Decode([]byte(raw), testAccel); err == nil {
				t.Fatalf("不应接受 %s", raw)
			}
		})
	}
	got, err := Decode([]byte(`{"version":1,"appearance":{"colorTheme":"graphite"}}`), testAccel)
	if err != nil {
		t.Fatal(err)
	}
	if got.Appearance.ColorTheme != "graphite" || got.Appearance.MarkdownTheme != "sonnet" {
		t.Fatalf("缺项应用默认值: %+v", got)
	}
	if got.Keymap.Overrides == nil {
		t.Fatal("缺 keymap 节应补空 map")
	}
}

// TestPreferencesMigrateFontSplit 版本 1 只有单一正文字体键；升级后
// latinFont 留空（跟随中文字体），渲染结果与升级前一致，其余字段原样保留；
// backup 节缺键按未选择备份文件夹处理，行为与升级前一致。
func TestPreferencesMigrateFontSplit(t *testing.T) {
	v1 := `{"version":1,"appearance":{"bodyFont":"songti","monoFont":"menlo","bodySize":19,"colorTheme":"paper"},"backup":{"keep":7}}`
	got, err := Decode([]byte(v1), testAccel)
	if err != nil {
		t.Fatal(err)
	}
	if got.Version != CurrentVersion || CurrentVersion != 9 {
		t.Fatalf("版本应升到 %d，当前 %d", CurrentVersion, got.Version)
	}
	if got.Appearance.LatinFont != "" {
		t.Fatalf("升级后西文应跟随中文字体: %q", got.Appearance.LatinFont)
	}
	if got.Appearance.BodyFont != "songti" || got.Appearance.MonoFont != "menlo" ||
		got.Appearance.BodySize != 19 || got.Appearance.ColorTheme != "paper" || got.Backup.Keep != 7 {
		t.Fatalf("升级不得改动其余偏好: %+v", got)
	}
	// 无 version 的旧文件同样升级。
	noVer, err := Decode([]byte(`{"appearance":{"bodyFont":"hand"}}`), testAccel)
	if err != nil {
		t.Fatal(err)
	}
	if noVer.Version != CurrentVersion || noVer.Appearance.BodyFont != "hand" || noVer.Appearance.LatinFont != "" {
		t.Fatalf("缺 version 的旧文件应升级: %+v", noVer)
	}
	// 版本 2 文件里的西文字体原样读回，非法值拒绝。
	v2, err := Decode([]byte(`{"version":2,"appearance":{"bodyFont":"hand","latinFont":"noteworthy"}}`), testAccel)
	if err != nil {
		t.Fatal(err)
	}
	if v2.Appearance.LatinFont != "noteworthy" || v2.Appearance.BodyFont != "hand" {
		t.Fatalf("版本 2 应分别读回中西文字体: %+v", v2.Appearance)
	}
	if _, err := Decode([]byte(`{"version":2,"appearance":{"latinFont":"comic"}}`), testAccel); err == nil {
		t.Fatal("未知西文字体必须拒绝")
	}
	// 版本 2 文件没有 backup 节的目录键：读回未选择状态。
	v2b, err := Decode([]byte(`{"version":2,"backup":{"intervalMinutes":30,"keep":9}}`), testAccel)
	if err != nil {
		t.Fatal(err)
	}
	if v2b.Backup.BackupDir != "" ||
		v2b.Backup.IntervalMinutes != 30 || v2b.Backup.Keep != 9 {
		t.Fatalf("版本 2 备份策略应原样保留且未选择备份文件夹: %+v", v2b.Backup)
	}
	// 版本 3 的 mirrorDir 迁移为 backupDir；gitMode 随 Git 自动化移除。
	v3, err := Decode([]byte(`{"version":3,"backup":{"mirrorDir":"/tmp/bak","gitMode":"commitPush"}}`), testAccel)
	if err != nil {
		t.Fatal(err)
	}
	if v3.Backup.BackupDir != "/tmp/bak" {
		t.Fatalf("版本 3 的外部备份目录应迁移为备份文件夹: %+v", v3.Backup)
	}
	// 保存的是版本 5，磁盘模型随当前版本落盘。
	if d := Default(5); d.Version != CurrentVersion || d.Appearance.LatinFont != "" {
		t.Fatalf("默认偏好应为当前版本且西文跟随中文: %+v", d)
	}
}

// TestPreferencesMigrateThemeRenaming 版本 4 把更名前的主题 ID（"tot"）
// 映射到新 ID（"classic" / "note"），主题取值本身不变。
func TestPreferencesMigrateThemeRenaming(t *testing.T) {
	v3 := `{"version":3,"appearance":{"colorTheme":"tot","markdownTheme":"tot","bodyFont":"kai"},"backup":{"keep":7}}`
	got, err := Decode([]byte(v3), testAccel)
	if err != nil {
		t.Fatal(err)
	}
	if got.Version != CurrentVersion {
		t.Fatalf("版本应升到 %d，当前 %d", CurrentVersion, got.Version)
	}
	if got.Appearance.ColorTheme != "classic" || got.Appearance.MarkdownTheme != "note" {
		t.Fatalf("主题 ID 应映射到新 ID: %+v", got.Appearance)
	}
	if got.Appearance.BodyFont != "kai" || got.Backup.Keep != 7 {
		t.Fatalf("迁移不得改动其余偏好: %+v", got)
	}
	// 新 ID 原样通过；缺 colorTheme 键的新文件直接用新默认。
	fresh, err := Decode([]byte(`{"version":4}`), testAccel)
	if err != nil {
		t.Fatal(err)
	}
	if fresh.Appearance.ColorTheme != "classic" || fresh.Appearance.MarkdownTheme != "sonnet" {
		t.Fatalf("新文件默认值: %+v", fresh.Appearance)
	}
}

// TestPreferencesMigrateCodeThemeSoft 版本 5 起代码块配色默认「柔和」：版本 4 及
// 更早的写入方从未写过 codeTheme 键，缺值文件补 soft；带值文件原样保留。版本 5
// 起该键始终落盘，缺键与空串都按显式「跟随文本」处理，不再补默认。
func TestPreferencesMigrateCodeThemeSoft(t *testing.T) {
	// 0.1.0/0.2.0 升级路径：v4 文件无 codeTheme 键 → 补默认 soft。
	v4, err := Decode([]byte(`{"version":4,"appearance":{"colorTheme":"paper"}}`), testAccel)
	if err != nil {
		t.Fatal(err)
	}
	if v4.Appearance.CodeTheme != "soft" {
		t.Fatalf("v4 缺键应补默认 soft: %+v", v4.Appearance)
	}
	// 旧开发文件带显式值：迁移不得覆盖。
	v4set, err := Decode([]byte(`{"version":4,"appearance":{"codeTheme":"vivid"}}`), testAccel)
	if err != nil {
		t.Fatal(err)
	}
	if v4set.Appearance.CodeTheme != "vivid" {
		t.Fatalf("v4 带值应原样保留: %+v", v4set.Appearance)
	}
	// v5 文件缺键：不补默认，按「跟随文本」处理。
	v5missing, err := Decode([]byte(`{"version":5}`), testAccel)
	if err != nil {
		t.Fatal(err)
	}
	if v5missing.Appearance.CodeTheme != "" {
		t.Fatalf("v5 缺键应保持跟随文本: %+v", v5missing.Appearance)
	}
	// v5 显式空串 = 跟随文本，解码原样读回。
	v5follow, err := Decode([]byte(`{"version":5,"appearance":{"codeTheme":""}}`), testAccel)
	if err != nil {
		t.Fatal(err)
	}
	if v5follow.Appearance.CodeTheme != "" {
		t.Fatalf("显式跟随文本应原样保留: %+v", v5follow.Appearance)
	}
	if got := Default(5).Appearance.CodeTheme; got != "soft" {
		t.Fatalf("全新安装默认应为 soft: %q", got)
	}
}

// TestPreferencesMigrateSingleBackupFolder 版本 6 起备份只有一个文件夹：
// mirrorDir 平移为 backupDir 并成为唯一备份地址，gitMode 随 Git 自动化移除。
func TestPreferencesMigrateSingleBackupFolder(t *testing.T) {
	// v5 文件的 mirrorDir 原样迁移；gitMode 键被摘除后严格解码放行。
	v5, err := Decode([]byte(`{"version":5,"backup":{"mirrorDir":"/tmp/bak","gitMode":"commitPush"}}`), testAccel)
	if err != nil {
		t.Fatal(err)
	}
	if v5.Version != CurrentVersion {
		t.Fatalf("版本应升到 %d，当前 %d", CurrentVersion, v5.Version)
	}
	if v5.Backup.BackupDir != "/tmp/bak" {
		t.Fatalf("mirrorDir 应迁移为备份文件夹: %+v", v5.Backup)
	}
	// 只有 gitMode 没有 mirrorDir 的文件也能解码：迁移结果为未选择。
	v5git, err := Decode([]byte(`{"version":5,"backup":{"gitMode":"commit"}}`), testAccel)
	if err != nil {
		t.Fatal(err)
	}
	if v5git.Backup.BackupDir != "" {
		t.Fatalf("无目录时迁移后应未选择: %+v", v5git.Backup)
	}
	// v6 文件直接用新键，旧键按未知字段拒绝。
	v6, err := Decode([]byte(`{"version":6,"backup":{"backupDir":"/tmp/new"}}`), testAccel)
	if err != nil {
		t.Fatal(err)
	}
	if v6.Backup.BackupDir != "/tmp/new" || v6.Version != CurrentVersion {
		t.Fatalf("v6 备份值应原样保留并升版: %+v 版本 %d", v6.Backup, v6.Version)
	}
	if _, err := Decode([]byte(`{"version":6,"backup":{"mirrorDir":"/tmp/old"}}`), testAccel); err == nil {
		t.Fatal("v6 文件出现旧键 mirrorDir 必须拒绝")
	}
	if _, err := Decode([]byte(`{"version":6,"backup":{"gitMode":"commit"}}`), testAccel); err == nil {
		t.Fatal("v6 文件出现旧键 gitMode 必须拒绝")
	}
}

// TestPreferencesMigrateGitAutomation 版本 9 起备份文件夹支持 Git 自动化：
// 旧文件从未写过这两个键，迁移补默认（关、默认推送间隔），不替用户开启推送；
// 版本 9 文件的显式值原样保留。
func TestPreferencesMigrateGitAutomation(t *testing.T) {
	// v8 文件缺 Git 键：迁移后保持关闭，间隔补默认。
	v8, err := Decode([]byte(`{"version":8,"backup":{"backupDir":"/tmp/repo"}}`), testAccel)
	if err != nil {
		t.Fatal(err)
	}
	if v8.Version != CurrentVersion || v8.Backup.GitAutoPush {
		t.Fatalf("旧文件迁移后 Git 自动化应默认关闭: %+v 版本 %d", v8.Backup, v8.Version)
	}
	if v8.Backup.GitPushMinutes != defaultGitPushMinutes {
		t.Fatalf("缺推送间隔应补默认 %d: %d", defaultGitPushMinutes, v8.Backup.GitPushMinutes)
	}
	// v9 文件显式开启或关闭：原样保留，不受迁移影响。
	v9off, err := Decode([]byte(`{"version":9,"backup":{"gitAutoPush":false,"gitPushMinutes":30}}`), testAccel)
	if err != nil {
		t.Fatal(err)
	}
	if v9off.Backup.GitAutoPush || v9off.Backup.GitPushMinutes != 30 {
		t.Fatalf("v9 显式值应原样保留: %+v", v9off.Backup)
	}
	v9on, err := Decode([]byte(`{"version":9,"backup":{"gitAutoPush":true}}`), testAccel)
	if err != nil {
		t.Fatal(err)
	}
	if !v9on.Backup.GitAutoPush {
		t.Fatalf("v9 显式开启应原样保留: %+v", v9on.Backup)
	}
	// 全新安装默认关闭且间隔为默认值。
	if d := Default(5); d.Backup.GitAutoPush || d.Backup.GitPushMinutes != defaultGitPushMinutes {
		t.Fatalf("默认偏好 Git 键错误: %+v", d.Backup)
	}
	// 推送间隔超出 1–1440 拒绝。
	bad := Default(5)
	bad.Backup.GitPushMinutes = 1441
	if err := bad.Validate(testAccel); err == nil {
		t.Fatal("gitPushMinutes 超范围必须拒绝")
	}
}

// TestPreferencesMigrateCodeIndent 版本 7 起代码块缩进只有 2 / 4 空格两档：
// 旧文件从未写过该键，升级时补默认 4 空格；早期内部构建写过的 tab / space8
// 废弃档位归为默认；其余非法取值仍拒绝。
func TestPreferencesMigrateCodeIndent(t *testing.T) {
	v6, err := Decode([]byte(`{"version":6,"appearance":{"colorTheme":"paper"}}`), testAccel)
	if err != nil {
		t.Fatal(err)
	}
	if v6.Version != CurrentVersion || v6.Appearance.CodeIndent != "space2" {
		t.Fatalf("v6 升级后应补默认 2 空格: 版本 %d codeIndent=%q", v6.Version, v6.Appearance.CodeIndent)
	}
	for _, key := range []string{"space2", "space4"} {
		v7, err := Decode([]byte(`{"version":7,"appearance":{"codeIndent":"`+key+`"}}`), testAccel)
		if err != nil {
			t.Fatalf("codeIndent=%s 应合法: %v", key, err)
		}
		if v7.Appearance.CodeIndent != key {
			t.Fatalf("codeIndent 应原样保留: %q", v7.Appearance.CodeIndent)
		}
	}
	for _, legacy := range []string{"tab", "space8"} {
		v7, err := Decode([]byte(`{"version":7,"appearance":{"codeIndent":"`+legacy+`"}}`), testAccel)
		if err != nil {
			t.Fatalf("废弃档位 %q 应归为默认而非拒绝: %v", legacy, err)
		}
		if v7.Appearance.CodeIndent != "space2" {
			t.Fatalf("废弃档位 %q 应归为默认 2 空格: %q", legacy, v7.Appearance.CodeIndent)
		}
	}
	if _, err := Decode([]byte(`{"version":7,"appearance":{"codeIndent":"space3"}}`), testAccel); err == nil {
		t.Fatal("非法 codeIndent 必须拒绝")
	}
}
