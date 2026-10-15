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
	a.openSettings(pageGeneral)
	tt := ui.NewTester(a.settingsView, 900, 620)
	if err := tt.Click("外观"); err != nil {
		t.Fatal(err)
	}
	if a.settingsPage != pageAppearance {
		t.Fatalf("sidebar click did not switch page: got %q", a.settingsPage)
	}
}

func TestSettingsSidebarStartsAtTop(t *testing.T) {
	a := newTestApp(t)
	a.openSettings(pageGeneral)
	tt := ui.NewTester(a.settingsView, 900, 620)
	r, ok := tt.Find("常规")
	if !ok || r.Y > 40 {
		t.Fatalf("sidebar should start at top, got %+v (found=%v)", r, ok)
	}
}

func TestSettingsAppearanceControlsStartAtTop(t *testing.T) {
	a := newTestApp(t)
	a.openSettings(pageAppearance)
	tt := ui.NewTester(a.settingsView, 900, 620)
	mode, ok := tt.Find("界面外观")
	if !ok || mode.W <= 0 || mode.H <= 0 || mode.Y < 24 || mode.Y > 120 {
		t.Fatalf("appearance mode must be visible near top: %+v (found=%v)", mode, ok)
	}
	preview, ok := tt.Find("预览")
	if !ok || preview.X < 490 || preview.W < 360 || preview.W > 400 {
		t.Fatalf("preview must be in a fixed right pane: %+v", preview)
	}
	controlsBefore, _ := tt.Find("Markdown 排版主题")
	tt.Scroll(400, 350, 0, 240)
	controlsAfter, _ := tt.Find("Markdown 排版主题")
	if controlsAfter.Y >= controlsBefore.Y {
		t.Fatalf("controls did not scroll: before=%+v after=%+v", controlsBefore, controlsAfter)
	}
	after, ok := tt.Find("预览")
	if !ok || after != preview {
		t.Fatalf("preview moved while controls scroll: before=%+v after=%+v", preview, after)
	}
}

func TestSettingsLayoutRender(t *testing.T) {
	for _, size := range [][2]int{{900, 620}, {760, 520}} {
		for _, page := range []string{pageGeneral, pageAppearance, pageBackup} {
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
