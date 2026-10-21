package app

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestSettingsSidebarNavigation(t *testing.T) {
	a := newTestApp(t)
	a.openSettings(pageKeymap)
	tt := ui.NewTester(a.settingsView, 900, 620)
	if err := tt.Click("通用"); err != nil {
		t.Fatal(err)
	}
	if a.settingsPage != pageGeneral {
		t.Fatalf("sidebar click did not switch page: got %q", a.settingsPage)
	}
	if _, ok := tt.Find("外观"); ok {
		t.Fatal("合并后不应保留外观侧栏入口")
	}
}

func TestSettingsSidebarStartsAtTop(t *testing.T) {
	a := newTestApp(t)
	a.openSettings(pageGeneral)
	tt := ui.NewTester(a.settingsView, 900, 620)
	r, ok := tt.Find("通用")
	if !ok || r.Y > 40 {
		t.Fatalf("sidebar should start at top, got %+v (found=%v)", r, ok)
	}
}

func TestSettingsGeneralControlsStartAtTop(t *testing.T) {
	a := newTestApp(t)
	a.openSettings(pageGeneral)
	tt := ui.NewTester(a.settingsView, 900, 620)
	lang, ok := tt.Find("应用语言")
	if !ok || lang.W <= 0 || lang.H <= 0 || lang.Y < 24 || lang.Y > 120 {
		t.Fatalf("first general row must be visible near top: %+v (found=%v)", lang, ok)
	}
	mode, ok := tt.Find("界面外观")
	if !ok || mode.W <= 0 || mode.H <= 0 || mode.Y <= lang.Y {
		t.Fatalf("appearance mode must follow the general section: %+v (found=%v)", mode, ok)
	}
	tt.Scroll(400, 350, 0, 240)
	modeAfter, _ := tt.Find("界面外观")
	if modeAfter.Y >= mode.Y {
		t.Fatalf("controls did not scroll: before=%+v after=%+v", mode, modeAfter)
	}
	settingsScrollTo(t, tt, "排版主题", 900, 620)
}

func settingsScrollTo(t *testing.T, tt *ui.Tester, label string, width, height int) {
	t.Helper()
	for i := 0; i < 20; i++ {
		r, ok := tt.Find(label)
		if ok && r.W > 0 && r.H > 0 && r.Y >= 28 && r.Y+r.H <= float32(height-28) {
			return
		}
		dy := float32(220)
		if ok && r.Y <= 28 {
			dy = -dy
		}
		tt.Scroll(float32(width-80), float32(height/2), 0, dy)
	}
	r, ok := tt.Find(label)
	t.Fatalf("滚动后控件 %q 仍不可见: %+v (found=%v)", label, r, ok)
}

func TestSettingsGeneralScrollAllSections(t *testing.T) {
	for _, size := range [][2]int{{900, 620}, {760, 520}, {640, 480}} {
		a := newTestApp(t)
		a.openSettings(pageGeneral)
		tt := ui.NewTester(a.settingsView, size[0], size[1])
		for _, label := range []string{"应用语言", "风格", "排版主题", "中文字体", "语法配色", "标签形状", "恢复本页默认值", "应用语言"} {
			settingsScrollTo(t, tt, label, size[0], size[1])
		}
		settingsScrollTo(t, tt, "经典", size[0], size[1])
		if err := tt.Click("经典"); err != nil {
			t.Fatal(err)
		}
		if err := tt.Click("石墨"); err != nil {
			t.Fatal(err)
		}
		if a.prefs.Appearance.ColorTheme != "graphite" {
			t.Fatalf("滚动后颜色下拉未生效: %+v", a.prefs.Appearance)
		}
	}
}

func TestSettingsLayoutRender(t *testing.T) {
	for _, size := range [][2]int{{900, 620}, {760, 520}} {
		for _, page := range []string{pageGeneral, pageBackup, pageKeymap} {
			a := newTestApp(t)
			a.openSettings(page)
			tt := ui.NewTester(a.settingsView, size[0], size[1])
			if dir := os.Getenv("TOT_MD_SETTINGS_RENDER_DIR"); dir != "" {
				if err := os.MkdirAll(dir, 0755); err != nil {
					t.Fatal(err)
				}
				name := page + "-wide.png"
				if size[0] == 760 {
					name = page + "-narrow.png"
				}
				f, err := os.Create(filepath.Join(dir, name))
				if err != nil {
					t.Fatal(err)
				}
				err = png.Encode(f, tt.Image())
				closeErr := f.Close()
				if err != nil || closeErr != nil {
					t.Fatalf("save render: %v %v", err, closeErr)
				}
			}
		}
	}
}
