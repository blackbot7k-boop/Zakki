package app

import (
	"image/color"
	"runtime"
	"testing"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// TestHeaderHeightSystemButtonBand 顶栏高度：无系统按钮带保持 38，
// Windows 隐藏标题栏模式与按钮带同高（不低于 28，保证圆点/胶囊放得下）。
func TestHeaderHeightSystemButtonBand(t *testing.T) {
	if got := headerHeight(ui.TitleBar{}); got != tabTrackHeight {
		t.Fatalf("headerHeight(无按钮带) = %g, want %g", got, tabTrackHeight)
	}
	if got := headerHeight(ui.TitleBar{Height: 32, Right: 138}); got != 32 {
		t.Fatalf("headerHeight(按钮带 32) = %g, want 32", got)
	}
	if got := headerHeight(ui.TitleBar{Height: 40}); got != 40 {
		t.Fatalf("headerHeight(按钮带 40) = %g, want 40", got)
	}
	if got := headerHeight(ui.TitleBar{Height: 20}); got != 28 {
		t.Fatalf("headerHeight(过矮按钮带) = %g, want 28", got)
	}
}

// TestHeaderAvoidsSystemButtons Windows 隐藏标题栏模式：顶栏高度降到按钮带，
// 标签区可用宽度扣除按钮带，圆点垂直中心与按钮 glyph 同轴（带中心 = 顶栏中心）；
// 清掉按钮带后恢复 38 与原布局。像素只做相对差分，不依赖精确墨色。
func TestHeaderAvoidsSystemButtons(t *testing.T) {
	a := newTestApp(t)
	t.Cleanup(a.shutdown)
	tt := ui.NewTester(a.view, 620, 400)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	tt.Frame()

	// 采样一枚非活动空标签的圆点（活动槽是有名字的胶囊，覆盖范围更大）；
	// cx 是标签视口内坐标，窗口 x 还要加上顶栏左内边距与关闭按钮命中区宽。
	slot := (a.active + 1) % len(a.lastSlots)
	x := headerPadX + chromeHit + a.lastSlots[slot].cx
	if !dotCenteredAt(tt, x, 19) {
		t.Fatal("无按钮带时圆点应垂直居中于 38 顶栏（中心 19）")
	}
	if a.tabViewportWidth != 620-tabChromeWidth {
		t.Fatalf("无按钮带 tabViewportWidth = %g, want %g", a.tabViewportWidth, 620-tabChromeWidth)
	}

	// Windows 按钮带：高 32、右让 138，圆点中心与按钮 glyph 同轴（16）。
	tt.SetTitleBar(ui.TitleBar{Height: 32, Right: 138})
	tt.Frame()
	if a.tabViewportWidth != 620-tabChromeWidth-138 {
		t.Fatalf("按钮带下 tabViewportWidth = %g, want %g", a.tabViewportWidth, 620-tabChromeWidth-138)
	}
	if !dotCenteredAt(tt, x, 16) {
		t.Fatal("按钮带下圆点应与系统按钮同轴（中心 16）")
	}

	// 全屏等场景 MyGo 把按钮带清零：顶栏恢复 38。
	tt.SetTitleBar(ui.TitleBar{})
	tt.Frame()
	if a.tabViewportWidth != 620-tabChromeWidth {
		t.Fatalf("按钮带清零后 tabViewportWidth = %g, want %g", a.tabViewportWidth, 620-tabChromeWidth)
	}
	if !dotCenteredAt(tt, x, 19) {
		t.Fatal("按钮带清零后圆点应回到中心 19")
	}
}

// dotCenteredAt 采样一枚标签圆点：中心是墨色、上下 8px 外是背景，即中心在 cy。
// 用相对差分避开背景混合色的精确值。
func dotCenteredAt(tt *ui.Tester, x, cy float32) bool {
	center := tabPixel(tt, x, cy)
	above := tabPixel(tt, x, cy-8)
	below := tabPixel(tt, x, cy+8)
	return !sameColor(center, above) && !sameColor(center, below) && sameColor(above, below)
}

func sameColor(a, b color.NRGBA) bool { return a == b }

// TestWindowOptionsKeepNativeButtonsOnWindows Windows 主窗口走隐藏标题栏模式，
// 保留系统最小化/最大化/关闭；其余平台保持 Frameless 自绘顶栏。
func TestWindowOptionsKeepNativeButtonsOnWindows(t *testing.T) {
	a := newTestApp(t)
	a.prefs.Window.RememberBounds = false
	opts := a.windowOptions()
	if !opts.Transparent || opts.Vibrancy != mygo.VibrancyContent {
		t.Fatalf("毛玻璃要求透明窗口与内容材质: %+v", opts)
	}
	if runtime.GOOS == "windows" {
		if opts.Frameless || opts.TitleBarStyle != mygo.TitleBarHidden {
			t.Fatalf("windows 应保留系统按钮（隐藏标题栏模式）: %+v", opts)
		}
		return
	}
	if !opts.Frameless || opts.TitleBarStyle != "" {
		t.Fatalf("非 windows 保持 Frameless 自绘顶栏: %+v", opts)
	}
}
