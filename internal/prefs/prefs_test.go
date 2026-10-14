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
