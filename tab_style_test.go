package main

import (
	"bytes"
	"fmt"
	"image/color"
	"image/png"
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
				want := emptyDotInk(dark)
				c := pixel(0, 0, 0)
				if c.R != want.R || c.G != want.G || c.B != want.B {
					t.Fatal("old empty tab should become solid gray")
				}
				a.tabs[1].Text = ""
				a.tabs[3].Text = "new note"
				tt.Frame()
				if pixel(1, 0, 0) == before || pixel(3, 0, 0).R == want.R {
					t.Fatal("dot colors did not follow content")
				}
			})
		}
	}
}

// 命名标签显示为带标题的胶囊；标题超宽省略；未命名只显示圆点。
func TestTabStripCapsule(t *testing.T) {
	a := newTestApp(t)
	a.tabs[1].Name = "短名"
	a.tabs[2].Name = "这是一个非常非常非常长的中文标签标题超出宽度上限"
	tt := ui.NewTester(a.view, 760, 540)
	const maxPillW = float32(22 + 120)
	// 未命名 active：槽位只有圆点宽。
	r0, ok := tt.Find(fmt.Sprintf("标签 %d", 1))
	if !ok || r0.W != 24 {
		t.Fatalf("unnamed active hit area = %+v, want width 24", r0)
	}
	// 切到命名标签：胶囊出现并带标题（ButtonBase 无障碍名即命名标题）。
	if err := tt.Click("短名"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	r1, ok := tt.Find("短名")
	if !ok {
		t.Fatalf("named pill should show its title, texts=%q", tt.Texts())
	}
	if r1.W <= 18 || r1.W > maxPillW {
		t.Fatalf("named slot width=%g, want within (18, %g]", r1.W, maxPillW)
	}
	// 超长标题：胶囊宽度封顶。
	longTitle := a.tabs[2].Name
	if err := tt.Click(longTitle); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	r2, ok := tt.Find(longTitle)
	if !ok {
		t.Fatalf("overlong pill should show its title, texts=%q", tt.Texts())
	}
	if r2.W != maxPillW {
		t.Fatalf("overlong title slot width=%g, want capped at %g", r2.W, maxPillW)
	}
	// 未命名标签的无障碍名是「标签 N」。
	if !tt.HasText(fmt.Sprintf("标签 %d", 1)) {
		t.Error("unnamed tab should expose 标签 1")
	}
	// 点击胶囊切换当前标签。
	if err := tt.Click("短名"); err != nil {
		t.Fatal(err)
	}
	if a.active != 1 {
		t.Fatalf("click pill switch active=%d", a.active)
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

func TestTrayRingPNG(t *testing.T) {
	img, err := png.Decode(bytes.NewReader(trayPNG()))
	if err != nil {
		t.Fatal(err)
	}
	if img.Bounds().Dx() != 36 || img.Bounds().Dy() != 36 {
		t.Fatalf("tray size=%v, want 36x36", img.Bounds())
	}
	at := func(x, y int) color.NRGBA {
		return color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
	}
	for _, p := range [][2]int{{0, 0}, {18, 18}, {18, 0}, {18, 35}, {8, 18}} {
		if at(p[0], p[1]).A != 0 {
			t.Errorf("tray should be transparent at %v", p)
		}
	}
	if c := at(3, 18); c != (color.NRGBA{255, 255, 255, 255}) {
		t.Errorf("tray ring pixel=%+v, want opaque white", c)
	}
	partial := false
	for y := 0; y < 36; y++ {
		for x := 0; x < 36; x++ {
			c := at(x, y)
			if c.A > 0 && (c.R != 255 || c.G != 255 || c.B != 255) {
				t.Fatalf("non-white tray pixel at %d,%d: %+v", x, y, c)
			}
			if c.A > 0 && c.A < 255 {
				partial = true
			}
			if c != at(35-x, y) || c != at(x, 35-y) {
				t.Fatalf("tray ring is not centered/symmetric at %d,%d", x, y)
			}
		}
	}
	if !partial {
		t.Error("tray ring should have antialiased edges")
	}
}
