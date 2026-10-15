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
	if p.Appearance.ColorTheme != "tot" || p.Appearance.MarkdownTheme != "sonnet" || p.Appearance.AppearanceMode != "system" {
		t.Fatalf("全新安装默认：Tot 颜色 + Sonnet 排版 + 跟随系统: %+v", p.Appearance)
	}
	if !p.Window.HideOnEscape || !p.Window.RememberBounds {
		t.Fatalf("窗口默认值错误: %+v", p.Window)
	}
	if p.Backup.IntervalMinutes != 5 || p.Backup.Keep != 20 || !p.Backup.OnLaunch {
		t.Fatalf("备份默认值错误: %+v", p.Backup)
	}
	if p.Backup.MirrorDir != "" || p.Backup.GitMode != "off" {
		t.Fatalf("全新安装不应启用外部备份: %+v", p.Backup)
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
		func(p *Preferences) { p.Backup.GitMode = "auto" },
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
// 新增的 backup.mirrorDir/gitMode 缺键按未配置处理，行为与升级前一致。
func TestPreferencesMigrateFontSplit(t *testing.T) {
	v1 := `{"version":1,"appearance":{"bodyFont":"songti","monoFont":"menlo","bodySize":19,"colorTheme":"paper"},"backup":{"keep":7}}`
	got, err := Decode([]byte(v1), testAccel)
	if err != nil {
		t.Fatal(err)
	}
	if got.Version != CurrentVersion || CurrentVersion != 3 {
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
	// 版本 2 文件没有 backup.mirrorDir/gitMode 键：读回未配置状态，写回即版本 3。
	v2b, err := Decode([]byte(`{"version":2,"backup":{"intervalMinutes":30,"keep":9}}`), testAccel)
	if err != nil {
		t.Fatal(err)
	}
	if v2b.Backup.MirrorDir != "" || v2b.Backup.GitMode != "off" ||
		v2b.Backup.IntervalMinutes != 30 || v2b.Backup.Keep != 9 {
		t.Fatalf("版本 2 备份策略应原样保留且外部备份未配置: %+v", v2b.Backup)
	}
	// 版本 3 显式值原样读回。
	v3, err := Decode([]byte(`{"version":3,"backup":{"mirrorDir":"/tmp/bak","gitMode":"commitPush"}}`), testAccel)
	if err != nil {
		t.Fatal(err)
	}
	if v3.Backup.MirrorDir != "/tmp/bak" || v3.Backup.GitMode != "commitPush" {
		t.Fatalf("版本 3 应读回外部备份配置: %+v", v3.Backup)
	}
	// 保存的是版本 3。
	if d := Default(5); d.Version != 3 || d.Appearance.LatinFont != "" {
		t.Fatalf("默认偏好应为版本 3 且西文跟随中文: %+v", d)
	}
}
