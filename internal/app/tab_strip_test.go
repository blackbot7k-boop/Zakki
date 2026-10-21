package app

import (
	"fmt"
	"image/color"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"zakki/locales"
)

func tabPixel(tt *ui.Tester, x, y float32) color.NRGBA {
	return color.NRGBAModel.Convert(tt.Image().At(int(x), int(y))).(color.NRGBA)
}

func TestCTabStripSolidDotsAndSelectedSize(t *testing.T) {
	a := newTestApp(t)
	t.Cleanup(a.shutdown)
	a.tabs[1].Text = "有内容"
	tt := ui.NewTester(a.view, 400, 540)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	selected, _ := tt.Find("标签 1")
	other, _ := tt.Find("标签 2")
	for _, item := range []struct {
		r    ui.Rect
		want int
		ink  ui.Color
	}{{selected, 12, a.tabInk(0, false)}, {other, 12, muteTabInk(a.tabInk(1, false), false)}} {
		cy := item.r.Y + item.r.H/2
		cx := item.r.X + item.r.W/2
		center := tabPixel(tt, cx, cy)
		if center.R != item.ink.R || center.G != item.ink.G || center.B != item.ink.B {
			t.Fatalf("dot center is not a solid tab color: %+v", center)
		}
		background := tabPixel(tt, cx, cy-15)
		n := 0
		for x := int(cx) - 16; x <= int(cx)+16; x++ {
			p := tabPixel(tt, float32(x), cy)
			distance := func(c color.NRGBA) int {
				return absByte(p.R, c.R) + absByte(p.G, c.G) + absByte(p.B, c.B)
			}
			if distance(center) < distance(background) {
				n++
			}
		}
		if n < item.want-1 || n > item.want+1 {
			t.Fatalf("painted diameter = %d, want %d, rect=%+v, center=%+v", n, item.want, item.r, center)
		}
	}
}

func TestCUnnamedMotionStartsAtPreviousTab(t *testing.T) {
	a, tt, clk := newAnimTestApp(t)
	old, _ := tt.Find("标签 1")
	if err := tt.Click("标签 4"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	p := tabPixel(tt, old.X+old.W/2, old.Y+old.H/2)
	if int(p.R) <= int(p.G)+30 {
		t.Fatalf("motion must start at previous selected center, pixel=%+v", p)
	}
	clk.advance(a.tabAnimDuration() / 2)
	tt.Frame()
	if !a.morph.active {
		t.Fatal("motion should still be active halfway through")
	}
	clk.advance(a.tabAnimDuration())
	tt.Frame()
	for i := 0; i < 3; i++ {
		tt.Frame()
		r, _ := tt.Find("标签 4")
		p = tabPixel(tt, r.X+r.W/2, r.Y+r.H/2)
		if int(p.G) <= int(p.R)+20 {
			t.Fatalf("selected tab disappeared after animation: %+v", p)
		}
	}
}

func TestCTabStripFitsNarrowWindow(t *testing.T) {
	for _, width := range []float32{300, 400, 760} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			a := newTestApp(t)
			t.Cleanup(a.shutdown)
			for len(a.tabs) < MaxTabCount {
				a.addTab()
			}
			a.tabs[a.active].Name = "这是很长的标题用于验证顶栏不遮挡操作按钮"
			tt := ui.NewTester(a.view, int(width), 540)
			tt.SetPreferences(ui.Preferences{ReduceMotion: true})
			r, ok := tt.Find(a.menuTitle(a.active))
			if !ok || r.X < 35 || r.X+r.W > width-35 {
				t.Fatalf("selected tab escapes the available top bar: %+v (width=%g)", r, width)
			}
		})
	}
}

func TestCMotionGeometryAndLayoutChanges(t *testing.T) {
	a, tt, clk := newAnimTestApp(t)
	for _, target := range []int{3, 0} {
		old := a.lastBody
		a.switchTab(target)
		tt.Frame()
		m := a.morph
		if m.bodyAt(clk.now()) != old {
			t.Fatal("motion did not start at previous selected body")
		}
		clk.advance(m.duration / 4)
		tt.Frame()
		cx := m.bodyAt(clk.now()).cx
		if cx <= min(old.cx, m.toBody.cx) || cx >= max(old.cx, m.toBody.cx) {
			t.Fatalf("moving body is not between endpoints: %g", cx)
		}
		clk.advance(m.duration)
		tt.Frame()
		if a.morph.active || a.lastBody != m.toBody {
			t.Fatal("motion did not settle at its target")
		}
	}
	for _, change := range []struct {
		name  string
		apply func()
	}{
		{"resize", func() { tt.SetSize(300, 540) }},
		{"rename", func() { a.renameTab(a.active, "很长的中文标题与 emoji 🙂") }},
		{"add", func() { a.addTab() }},
		{"reorder", func() { a.moveTab(a.active, 1) }},
		{"delete", func() { a.deleteTabNow(5) }},
	} {
		t.Run(change.name, func(t *testing.T) {
			a.switchTab((a.active + 1) % len(a.tabs))
			tt.Frame()
			clk.advance(a.tabAnimDuration() / 4)
			tt.Frame()
			change.apply()
			tt.Frame()
			if a.morph.active {
				t.Fatal("layout change must cancel old motion")
			}
		})
	}
}

func TestCLiquidBridgeNarrowsAndSeparates(t *testing.T) {
	body := tabBody{cx: 50, w: 100, h: 22, color: ui.Hex("#d4253a")}
	paint := func(gap float32) *ui.Tester {
		dotX := body.cx + body.w/2 + dotDiameter/2 + gap
		return ui.NewTester(func(c *ui.Context) {
			c.Root().Background(ui.Transparent)
			ui.Box(c).Fill().Draw(func(p *ui.Painter, r ui.Rect) {
				if path, ok := liquidBridge(body, dotX, 30); ok {
					p.FillPath(&path, body.color)
				}
				paintSolidDot(p, dotX, 30, ui.Hex("#0f75e9"))
				paintBody(p, body, 30)
			})
		}, 150, 60)
	}
	thickness := func(gap float32) int {
		tt := paint(gap)
		n := 0
		for y := 15; y < 45; y++ {
			if tabPixel(tt, 100+gap/2, float32(y)).A > 20 {
				n++
			}
		}
		if p := tabPixel(tt, 106+gap, 30); p.B != 233 {
			t.Fatal("neighbor disappeared during fusion")
		}
		return n
	}
	near, far, apart := thickness(2), thickness(8), thickness(11)
	if near <= far || far == 0 || apart != 0 {
		t.Fatalf("bridge thickness near/far/apart = %d/%d/%d", near, far, apart)
	}
	if _, ok := liquidBridge(body, 50, 30); ok {
		t.Fatal("buried dot must not produce an external bridge")
	}
}

func TestCTitlesFitAndStayInsideCapsule(t *testing.T) {
	for _, name := range []string{"中文标题超长需要省略这个内容", "Very long English title with many words", "🙂🚀🎉🌈🙂🚀🎉🌈🙂🚀🎉🌈"} {
		a := newTestApp(t)
		t.Cleanup(a.shutdown)
		a.tabs[0].Name = name
		a.tabs[1].Name = name + " next"
		tt := ui.NewTester(a.view, 400, 540)
		if a.lastBody.title == name || !strings.HasSuffix(a.lastBody.title, "…") {
			t.Fatalf("title not fitted: %+v", a.lastBody)
		}
		if a.lastBody.h != pillHeight || a.lastBody.w < a.lastBody.textW+2*pillPadX {
			t.Fatalf("title should stay inside the capsule: %+v", a.lastBody)
		}
		for i, s := range a.lastSlots {
			if i != a.active && s.w != dotDiameter {
				t.Fatalf("inactive slot %d width = %g, want %g", i, s.w, dotDiameter)
			}
		}
		a.tabs[a.active].Name = ""
		tt.Frame()
		if a.lastBody.w != dotDiameter || a.lastBody.title != "" {
			t.Fatal("cleared name should return to a same-sized dot")
		}
	}
}

func TestCDeleteOtherTabPreservesActiveIdentity(t *testing.T) {
	a := newTestApp(t)
	t.Cleanup(a.shutdown)
	a.active = 4
	id := a.activeTab().ID
	if !a.deleteTabNow(1) || a.activeTab().ID != id {
		t.Fatal("deleting an earlier tab changed the active note")
	}
}

func TestCMotionRunsWithoutAdditionalInput(t *testing.T) {
	a, tt, clk := newAnimTestApp(t)
	if err := tt.Click("标签 2"); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 8; i++ {
		clk.advance(a.tabAnimDuration()/4 + time.Millisecond)
		tt.Frame()
	}
	if a.morph.active {
		t.Fatal("animation did not settle")
	}
	r, _ := tt.Find("标签 2")
	p := tabPixel(tt, r.X+r.W/2, r.Y+r.H/2)
	if int(p.R) <= int(p.B)+20 {
		t.Fatalf("orange selected tab is absent: %+v", p)
	}
}

// TestHeaderHidesStripWithSingleTab 只有一个标签时顶栏不渲染标签区；
// 恢复多标签后标签区重新出现，且不残留旧几何导致的动画。
func TestHeaderHidesStripWithSingleTab(t *testing.T) {
	a := newTestApp(t)
	a.tabs = a.tabs[:1]
	tt := ui.NewTester(func(c *ui.Context) { a.header(c, c.Theme(), false) }, 420, 38)
	tt.Frame()
	if !a.tabStripHidden() {
		t.Fatal("单标签应隐藏标签区")
	}
	if tt.HasText("标签 1") {
		t.Fatalf("单标签顶栏不应有标签元素, texts=%q", tt.Texts())
	}
	// 新增第二个标签：标签区恢复，当前标签以同大圆点呈现。
	a.tabs = append(a.tabs, tab{ID: 2})
	tt.Frame()
	if a.tabStripHidden() {
		t.Fatal("多标签应显示标签区")
	}
	if !tt.HasText("标签 1") {
		t.Fatalf("恢复后应有当前标签胶囊, texts=%q", tt.Texts())
	}
}

// TestTabDotShapesRender 圆点填充、圆环描边；当前标记与其余标记同大。
// 已去掉的形状视为非法，载入时回退圆点。
func TestTabDotShapesRender(t *testing.T) {
	a := newTestApp(t)
	a.tabs = a.tabs[:1]
	a.tabs = append(a.tabs, tab{ID: 2, Color: "#0f75e9"})
	prep := ui.NewTester(func(c *ui.Context) { a.tabStrip(c, false) }, 300, 38)
	prep.Frame()
	activeX := a.lastSlots[0].cx
	cx := a.lastSlots[1].cx
	cy := float32(19)
	for _, shape := range []string{"dot", "ring"} {
		a.cfg.TabDotShape = shape
		tt := ui.NewTester(func(c *ui.Context) {
			c.Root().Background(ui.Transparent)
			ui.Box(c).Fill().Draw(func(p *ui.Painter, r ui.Rect) {
				a.paintTabStrip(p, ui.Rect{X: 0, Y: 0, W: 300, H: 38}, false)
			})
		}, 300, 38)
		tt.Frame()
		center := tabPixel(tt, cx, cy)
		active := tabPixel(tt, activeX, cy)
		if tabPixel(tt, cx, cy-8).A != 0 || tabPixel(tt, activeX, cy-8).A != 0 {
			t.Fatalf("%s 当前与其余标记应一样大，外侧 8px 不应有像素", shape)
		}
		if shape == "ring" {
			if center.A != 0 || active.A != 0 {
				t.Fatalf("%s 圆环中心应透明: inactive=%+v active=%+v", shape, center, active)
			}
			if tabPixel(tt, cx, cy-4.5).A == 0 || tabPixel(tt, activeX, cy-4.5).A == 0 {
				t.Fatalf("%s 圆环描边应有像素", shape)
			}
			continue
		}
		if center.A == 0 || active.A == 0 {
			t.Fatalf("%s 中心应有像素", shape)
		}
	}
	for _, gone := range []string{"square", "hexagon", "star", "nope"} {
		if isValidTabDotShape(gone) {
			t.Fatalf("%s 已从可选形状中去掉", gone)
		}
	}
	if !isValidTabDotShape("") || !isValidTabDotShape(tabDotShapeDot) || !isValidTabDotShape(tabDotShapeRing) {
		t.Fatal("空串、圆点与圆环应为合法值")
	}
}

// TestTabMotionOffSkipsMorph 关闭顶栏动效后切换标签直接就位，无补间动画。
func TestTabMotionOffSkipsMorph(t *testing.T) {
	a, tt, clk := newAnimTestApp(t)
	a.cfg.TabMotionOff = true
	a.switchTab((a.active + 1) % len(a.tabs))
	tt.Frame()
	if a.morph.active {
		t.Fatal("关闭动效后不应有补间状态")
	}
	clk.advance(time.Second)
	tt.Frame()
	if a.morph.active {
		t.Fatal("关闭动效后不应有补间状态（推进时间后）")
	}
}

// TestTabContextMenuDeleteAndRename 标签右键菜单：删除与重命名都落在被
// 右键的那个标签（按稳定 ID 重定位）；重命名先切到该标签再进入重命名态，
// 与双击标签同路径。空标签删除不走确认弹窗，测试保持同步。
func TestTabContextMenuDeleteAndRename(t *testing.T) {
	a := newTestApp(t)
	t.Cleanup(a.shutdown)
	for i := range a.tabs {
		a.tabs[i].Text = ""
	}
	tt := ui.NewTester(a.view, 400, 540)
	tt.Frame()

	if err := tt.RightClick("标签 2"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	menu := strings.Join(tt.Menu(), ",")
	for _, want := range []string{"重命名标签", "删除标签"} {
		if !strings.Contains(menu, want) {
			t.Fatalf("右键菜单缺 %q: %q", want, menu)
		}
	}
	wantID := a.tabs[1].ID
	if err := tt.ChooseMenuItem("删除标签"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.tabIndexByID(wantID) >= 0 || len(a.tabs) != 6 {
		t.Fatalf("应删除右键的标签: len=%d", len(a.tabs))
	}

	// 删除后原第 3 个标签顶到第 2 位：右键它重命名，应切换过去再进重命名态。
	if err := tt.RightClick("标签 2"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.ChooseMenuItem("重命名标签"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.active != 1 || a.renaming != a.tabs[1].ID {
		t.Fatalf("应切换到被右键的标签并进入重命名: active=%d renaming=%d", a.active, a.renaming)
	}
	a.cancelRename()
}

// TestSettingsTabListDragReorder 设置页标签列表拖拽排序：按住行左空白
// （避开按钮）竖拖一行，Reorder 回调按 List 语义重排并落盘。
func TestSettingsTabListDragReorder(t *testing.T) {
	a := newTestApp(t)
	t.Cleanup(a.shutdown)
	a.settingsPage = pageTabs
	before := []uint64{a.tabs[0].ID, a.tabs[1].ID, a.tabs[2].ID}
	tt := ui.NewTester(a.settingsView, 760, 600)
	tt.Frame()

	src, ok := tt.Find(locales.TabColorOf(a.tabs[0].Color))
	if !ok {
		t.Fatal("找不到第 1 行色块")
	}
	dst, ok := tt.Find(locales.TabColorOf(a.tabs[2].Color))
	if !ok {
		t.Fatal("找不到第 3 行色块")
	}
	mx := src.X - 4 // 行左 padding 空白，按在按钮上不会触发行拖拽
	tt.Press(mx, src.Y+src.H/2)
	for i := 1; i <= 6; i++ {
		y := src.Y + src.H/2 + (dst.Y+dst.H/2-(src.Y+src.H/2))*float32(i)/6
		tt.Move(mx, y)
		tt.Frame()
	}
	tt.Release(mx, dst.Y+dst.H/2)
	tt.Frame()

	after := []uint64{a.tabs[0].ID, a.tabs[1].ID, a.tabs[2].ID}
	// 拖到第 3 行中心释放，落在第 3 行之后（List 按指针相对行中心的
	// 上下半区决定插到行前还是行后）。
	if after[2] != before[0] {
		t.Fatalf("拖拽未生效: before=%v after=%v", before, after)
	}
	if a.active != 2 {
		t.Fatalf("当前选中应跟随被拖的标签: active=%d", a.active)
	}
}
