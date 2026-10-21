package app

import (
	"runtime"
	"testing"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

func TestGlassContentBackground(t *testing.T) {
	for _, tc := range []struct {
		name string
		dark bool
		want ui.Color
	}{
		{"light", false, ui.RGBA(255, 255, 255, 0.58)},
		{"dark", true, ui.RGBA(24, 24, 24, 0.62)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, base := range []ui.Color{ui.RGB(250, 240, 230), ui.RGB(35, 40, 50)} {
				if got := contentBackground(base, tc.dark, true); got != tc.want {
					t.Errorf("material layer for %v = %v, want neutral %v", base, got, tc.want)
				}
				if got := contentBackground(base, tc.dark, false); got != base || got.A != 255 {
					t.Errorf("no-material fallback = %v, want opaque theme %v", got, base)
				}
			}
		})
	}
}

func TestGlassChromeColor(t *testing.T) {
	a := newTestApp(t)
	for _, dark := range []bool{false, true} {
		want := ui.RGBA(255, 255, 255, 0.30)
		if dark {
			want = ui.RGBA(24, 24, 24, 0.40)
		}
		for _, strength := range []float32{0, 0.2, 0.9} {
			a.cfg.GlassStrength = strength
			// Header and status inputs may differ, but native chrome stays neutral.
			for _, base := range []ui.Color{headerBackground(dark), ui.RGB(120, 180, 220)} {
				if got := a.chromeColor(base, dark, true); got != want {
					t.Errorf("dark=%v strength=%v material chrome = %v, want %v", dark, strength, got, want)
				}
				bg := a.resolveStyle(-1, dark, true).background
				fallback := base.Mix(bg, 1-a.glassStrength())
				if got := a.chromeColor(base, dark, false); got != fallback || got.A != 255 {
					t.Errorf("dark=%v strength=%v fallback = %v, want opaque %v", dark, strength, got, fallback)
				}
			}
		}
	}
}

func TestGlassRenderedLayers(t *testing.T) {
	for _, source := range []bool{false, true} {
		for _, dark := range []bool{false, true} {
			a := newTestApp(t)
			t.Cleanup(a.shutdown)
			a.cfg.Source = source
			a.tabs[0].Text = ""
			tt := ui.NewTester(a.view, 400, 540)
			tt.SetDark(dark)
			tt.SetVibrancy(true)
			wantContent := contentBackground(ui.Color{}, dark, true)
			wantChrome := a.chromeColor(ui.Color{}, dark, true)
			for _, sample := range []struct {
				x, y float32
				want ui.Color
			}{
				{390, 300, wantContent}, {200, 3, wantChrome}, {390, 537, wantChrome},
			} {
				p := tabPixel(tt, sample.x, sample.y)
				if p.A != sample.want.A {
					t.Fatalf("source=%v dark=%v at (%g,%g): alpha=%d want=%d; material is covered", source, dark, sample.x, sample.y, p.A, sample.want.A)
				}
			}
			before := tabPixel(tt, 390, 300)
			a.switchTab(2)
			tt.Frame()
			if tabPixel(tt, 390, 300) != before {
				t.Fatal("switch changed content background")
			}
			tt.SetVibrancy(false)
			if p := tabPixel(tt, 390, 300); p.A != 255 {
				t.Fatal("non-material fallback must be opaque")
			}
		}
	}
}

func TestGlassNativeWindowOptions(t *testing.T) {
	a := newTestApp(t)
	for _, remember := range []bool{false, true} {
		a.prefs.Window.RememberBounds = remember
		opts := a.windowOptions()
		if !opts.Transparent || opts.Vibrancy != mygo.VibrancyContent {
			t.Fatalf("remember=%v: glass requires a transparent native window with content material: %+v", remember, opts)
		}
		// 平台形态：windows 用隐藏标题栏保留系统按钮（见
		// TestWindowOptionsKeepNativeButtonsOnWindows），其余平台自绘顶栏。
		wantFrameless := runtime.GOOS != "windows"
		if opts.Frameless != wantFrameless {
			t.Fatalf("remember=%v: frameless = %v, want %v", remember, opts.Frameless, wantFrameless)
		}
	}
}
