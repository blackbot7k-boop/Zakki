package app

import (
	"bytes"
	"fmt"
	"image/color"
	"image/png"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestTabStripStyles(t *testing.T) {
	for _, dark := range []bool{false, true} {
		for _, scale := range []float32{1, 2} {
			t.Run(fmt.Sprintf("dark=%t/scale=%g", dark, scale), func(t *testing.T) {
				a := newTestApp(t)
				t.Cleanup(a.shutdown)
				for i := range a.tabs {
					a.tabs[i].Text = ""
				}
				a.tabs[1].Text = "content"
				tt := ui.NewTester(a.view, 400, 540)
				tt.SetDark(dark)
				tt.SetScale(scale)
				tt.SetPreferences(ui.Preferences{ReduceMotion: true})
				pixel := func(i int, dx, dy float32) color.NRGBA {
					r, ok := tt.Find(fmt.Sprintf("标签 %d", i+1))
					if !ok {
						t.Fatalf("missing tab %d", i)
					}
					return color.NRGBAModel.Convert(tt.Image().At(int((r.X+r.W/2+dx)*scale), int((r.Y+r.H/2+dy)*scale))).(color.NRGBA)
				}
				for i := range a.tabs {
					r, _ := tt.Find(fmt.Sprintf("标签 %d", i+1))
					if r.W < 24 || r.H < 28 {
						t.Fatalf("hit area too small: %+v", r)
					}
					want := a.tabInk(i, dark)
					if i != a.active {
						want = muteTabInk(want, dark)
					}
					c := pixel(i, 0, 0)
					if c.R != want.R || c.G != want.G || c.B != want.B {
						t.Fatalf("solid dot %d center=%+v want=%+v", i, c, want)
					}
					if i > 0 && pixel(i, 0, 8) == c {
						t.Fatalf("inactive dot %d exceeds 12 DIP", i)
					}
				}
				before := pixel(1, 0, 0)
				r, _ := tt.Find("标签 2")
				tt.Move(r.X+r.W/2, r.Y+r.H/2)
				if pixel(1, 0, 0) != before {
					t.Fatal("hover changed the solid dot")
				}
				if err := tt.Click("标签 3"); err != nil {
					t.Fatal(err)
				}
				tt.Frame()
				if a.active != 2 {
					t.Fatal("gray empty dot is not clickable")
				}
				want := muteTabInk(emptyDotInk(dark), dark)
				c := pixel(0, 0, 0)
				if c.R != want.R || c.G != want.G || c.B != want.B {
					t.Fatal("old empty tab should become muted gray")
				}
				a.tabs[1].Text = ""
				a.tabs[3].Text = "new note"
				tt.Frame()
				filled := pixel(3, 0, 0)
				if pixel(1, 0, 0) == before || (filled.R == want.R && filled.G == want.G && filled.B == want.B) {
					t.Fatal("dot colors did not follow content")
				}
			})
		}
	}
}

// 未命名时当前标记与其他标记一样大。有名字时，名字画在当前标签的色条里，色条随标题变宽。
func TestTabStripCapsule(t *testing.T) {
	a := newTestApp(t)
	a.tabs[1].Name = "短名"
	a.tabs[2].Name = "这是一个非常非常非常长的中文标签标题超出宽度上限"
	tt := ui.NewTester(a.view, 760, 540)
	r0, ok := tt.Find(fmt.Sprintf("标签 %d", 1))
	if !ok || r0.W != 24 {
		t.Fatalf("unnamed active hit area = %+v, want width 24", r0)
	}
	if a.lastBody.w != dotDiameter || a.lastBody.title != "" {
		t.Fatalf("unnamed active should stay a dot: %+v", a.lastBody)
	}
	if err := tt.Click("短名"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	r1, ok := tt.Find("短名")
	if !ok || r1.W <= 24 {
		t.Fatalf("named capsule = %+v, want wider than a dot", r1)
	}
	if a.lastBody.title != "短名" || a.lastBody.h != pillHeight || a.lastSlots[a.active].w <= dotDiameter {
		t.Fatalf("name should sit on the active capsule: body=%+v slot=%+v", a.lastBody, a.lastSlots[a.active])
	}
	longTitle := a.tabs[2].Name
	if err := tt.Click(longTitle); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if !strings.HasSuffix(a.lastBody.title, "…") {
		t.Fatalf("overlong title was not truncated: %q", a.lastBody.title)
	}
	if a.lastBody.w < a.lastBody.textW+2*pillPadX || a.lastSlots[a.active].w <= dotDiameter {
		t.Fatalf("overlong title should widen the capsule: body=%+v", a.lastBody)
	}
	if !tt.HasText(fmt.Sprintf("标签 %d", 1)) {
		t.Error("unnamed tab should expose 标签 1")
	}
	if err := tt.Click("短名"); err != nil {
		t.Fatal(err)
	}
	if a.active != 1 {
		t.Fatalf("click capsule switch active=%d", a.active)
	}
}

func TestTabStripKeyboardNavigation(t *testing.T) {
	a := newTestApp(t)
	for i := range a.tabs {
		a.tabs[i].Text = ""
	}
	tt := ui.NewTester(func(c *ui.Context) { a.tabStrip(c, false) }, 420, 32)
	tt.Key(0, ui.KeyTab)
	tt.Key(0, ui.KeyTab)
	tt.Key(0, ui.KeyEnter)
	if a.active != 1 {
		t.Fatalf("Tab + Enter should select empty tab 2, active=%d", a.active)
	}
	tt.Key(0, ui.KeyTab)
	tt.Key(0, ui.KeySpace)
	if a.active != 2 {
		t.Fatalf("Tab + Space should select empty tab 3, active=%d", a.active)
	}
}

func absByte(a, b uint8) int {
	if a > b {
		return int(a - b)
	}
	return int(b - a)
}

func TestTrayInkZPNG(t *testing.T) {
	for name, whiteInk := range map[string]bool{"white": true, "black": false} {
		t.Run(name, func(t *testing.T) { testTrayPNG(t, trayPNG(whiteInk), whiteInk) })
	}
}

func testTrayPNG(t *testing.T, pngBytes []byte, whiteInk bool) {
	t.Helper()
	img, err := png.Decode(bytes.NewReader(pngBytes))
	if err != nil {
		t.Fatal(err)
	}
	if img.Bounds().Dx() != 36 || img.Bounds().Dy() != 36 {
		t.Fatalf("tray size=%v, want 36x36", img.Bounds())
	}
	at := func(x, y int) color.NRGBA {
		return color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
	}
	// Z 的上下横之间、Z 与句点之外应透明；笔画中段与句点应不透明。
	for _, p := range [][2]int{{0, 0}, {35, 35}, {10, 14}, {28, 14}} {
		if at(p[0], p[1]).A != 0 {
			t.Errorf("tray should be transparent at %v", p)
		}
	}
	for _, p := range [][2]int{{18, 18}, {8, 26}} {
		if at(p[0], p[1]).A != 255 {
			t.Errorf("tray stroke should be opaque at %v", p)
		}
	}
	partial := false
	for y := 0; y < 36; y++ {
		for x := 0; x < 36; x++ {
			c := at(x, y)
			want := uint8(0)
			if whiteInk {
				want = 255
			}
			if c.A > 0 && (c.R != want || c.G != want || c.B != want) {
				t.Fatalf("wrong tray ink at %d,%d: %+v", x, y, c)
			}
			if (x == 0 || x == 35 || y == 0 || y == 35) && c.A != 0 {
				t.Fatalf("tray should have transparent padding at %d,%d", x, y)
			}
			if c.A > 0 && c.A < 255 {
				partial = true
			}
		}
	}
	if !partial {
		t.Error("tray icon should have antialiased edges")
	}
}
