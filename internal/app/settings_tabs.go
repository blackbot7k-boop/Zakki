package app

// 「标签与外观」设置页：标签列表（命名、新增、删除、排序、改色）与
// 标签栏外观（标题宽、动画速度、明暗模式、彩色圆点、毛玻璃回退强度）。
//
// 标签元数据存在笔记数据快照里（renameTab/addTab 等统一 scheduleSave）；
// 明暗模式沿用偏好（preferences.json，经 updatePreferences 即时生效）。
// 所有回调在主线程执行；标签变更后主窗口经 withWindow/refreshSettingsWindow
// 刷新。

import (
	"fmt"

	"github.com/egoist/mygo/ui"

	"zakki/locales"
)

// tabAnimSpeedOptions 动画速度选择项（当前语言文案与倍率，顺序固定）。
func tabAnimSpeedOptions() []struct {
	display string
	speed   float32
} {
	return []struct {
		display string
		speed   float32
	}{
		{locales.AnimSlow(), 0.5},
		{locales.AnimNormal(), 1},
		{locales.AnimFast(), 1.5},
		{locales.AnimFaster(), 2},
	}
}

func tabAnimSpeedDisplay(speed float32) string {
	if speed <= 0 {
		speed = 1
	}
	for _, o := range tabAnimSpeedOptions() {
		if o.speed == speed {
			return o.display
		}
	}
	return tabAnimSpeedOptions()[1].display // 非法值收敛到「正常」
}

// tabDotShapeOptions 标签形状选择项（ID 与当前语言文案，顺序即下拉顺序）。
func tabDotShapeOptions() []struct {
	id, display string
} {
	return []struct {
		id, display string
	}{
		{tabDotShapeDot, locales.ShapeDot()},
		{tabDotShapeRing, locales.ShapeRing()},
	}
}

func tabDotShapeDisplay(id string) string {
	for _, o := range tabDotShapeOptions() {
		if o.id == id {
			return o.display
		}
	}
	return tabDotShapeOptions()[0].display // 空串/非法值收敛到「圆点」
}

func tabDotShapeKey(display string) string {
	for _, o := range tabDotShapeOptions() {
		if o.display == display {
			return o.id
		}
	}
	return tabDotShapeDot
}

// tabsPage 标签与外观设置页。
func (a *app) tabsPage(c *ui.Context, t *ui.Theme) {
	a.settingsUI.tabAnimSpeedSel = tabAnimSpeedDisplay(a.cfg.TabAnimSpeed)
	a.settingsUI.tabDotShapeSel = tabDotShapeDisplay(a.activeDotShape())
	a.settingsUI.tabShapeIdx = 0
	if a.activeDotShape() == tabDotShapeRing {
		a.settingsUI.tabShapeIdx = 1
	}
	ui.Scroll(c).Fill().Grow(1).ScrollbarInsets(0, 12, 0, 0).Children(func() {
		ui.Column(c).FillWidth().Padding(0, 22, 0, 0).Children(func() {
			settingsSectionTitle(c, locales.CardTabs())
			settingsLead(c, locales.TabsListLead())
			ui.Column(c).FillWidth().PaddingY(6).Gap(8).Children(func() {
				a.tabListSection(c)
			})
			sectionDivider(c)
			settingsSectionTitle(c, locales.CardTabBar())
			settingsLead(c, locales.TabDotShapeHint())
			a.tabAppearanceSection(c)
		})
	})
}

// tabListSection 标签列表：新增按钮 + 每行颜色/名称/快捷键/上移/下移/删除。
func (a *app) tabListSection(c *ui.Context) {
	atCap := len(a.tabs) >= MaxTabCount
	ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(10).Children(func() {
		if ui.Button(c, locales.MenuNewTab()).Clicked() {
			a.addTab()
		}
		hint := locales.TabCountSummary(len(a.tabs), MaxTabCount)
		if atCap {
			hint = locales.TabMaxReached(MaxTabCount)
			ui.Text(c, hint).FontSize(11).TextColor(c.Theme().Warning)
		} else {
			ui.Text(c, hint).FontSize(11).TextColor(tMuted(c))
		}
	})
	if a.notice != "" {
		ui.Text(c, a.notice).FontSize(11).TextColor(c.Theme().Warning)
	}

	ui.Column(c).FillWidth().Gap(6).Children(func() {
		for i := range a.tabs {
			a.tabRow(c, i)
		}
	})
}

// tabRow 一条标签：颜色点（点开调色板）、名称（即时保存）、快捷键提示、
// 上移/下移/删除；点颜色点或行首区域切换当前标签。
func (a *app) tabRow(c *ui.Context, i int) {
	tb := a.tabs[i]
	current := i == a.active
	row := ui.Column(c.Key(fmt.Sprintf("tabrow-%d", tb.ID))).FillWidth().Gap(6).Padding(8, 10).Radius(8)
	if current {
		row.Background(c.Theme().Surface)
	}
	row.Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(8).Children(func() {
			tabID := tb.ID
			// 颜色点与顶栏同大：当前标签只加「当前」二字，不再描一圈把点撑大。
			dot := ui.ButtonBase(c.Key(fmt.Sprintf("tabcur-%d", tabID))).Label(locales.TabSwitchTo(a.menuTitle(i))).
				Size(18, 18).Radius(9).Cursor(ui.CursorPointer)
			dot.Draw(func(p *ui.Painter, r ui.Rect) {
				var path ui.Path
				path.Circle(r.X+r.W/2, r.Y+r.H/2, 7)
				p.FillPath(&path, a.tabColor(i))
			})
			if dot.Clicked() {
				a.switchTab(i)
			}
			ui.Text(c, a.menuTitle(i)).Grow(1)
			if current {
				ui.Text(c, locales.TabCurrent()).FontSize(11).TextColor(c.Theme().Accent)
			}
			if i < 9 {
				ui.Text(c, shortcutText(fmt.Sprintf("⌃%d", i+1))).FontSize(11).TextColor(tMuted(c))
			}
		})
		ui.Row(c).Gap(8).Children(func() {
			tabID := tb.ID
			if ui.Button(c.Key(fmt.Sprintf("tabpalette-%d", tabID)), locales.TabRecolor()).Clicked() {
				if a.settingsUI.expandedColor == tabID {
					a.settingsUI.expandedColor = 0
					return
				}
				a.settingsUI.expandedColor = tabID
			}
			if ui.Button(c.Key(fmt.Sprintf("tabup-%d", tb.ID)), "↑").
				Label(locales.TabMoveUp(a.menuTitle(i))).Disabled(i == 0).Clicked() {
				a.moveTab(i, -1)
			}
			if ui.Button(c.Key(fmt.Sprintf("tabdown-%d", tb.ID)), "↓").
				Label(locales.TabMoveDown(a.menuTitle(i))).Disabled(i == len(a.tabs)-1).Clicked() {
				a.moveTab(i, 1)
			}
			ui.Button(c.Key(fmt.Sprintf("tabdel-%d", tabID)), locales.CommonDelete()).
				Label(locales.TabDelete(a.menuTitle(i))).OnClick(func() {
				current := a.tabIndexByID(tabID)
				if current >= 0 {
					a.confirmDeleteTab(current)
				}
			})
		})
		// 展开的调色板：12 色，点击改色。
		if a.settingsUI.expandedColor == tb.ID {
			ui.Row(c).FillWidth().Gap(6).PaddingX(20).Children(func() {
				for pi, cand := range tabPaletteColors {
					pi, cand := pi, cand
					selected := a.tabColor(i) == ui.Hex(cand)
					s := ui.ButtonBase(c.Key(fmt.Sprintf("pal-%d-%d", tb.ID, pi))).
						Label(locales.TabColorOf(cand)).Size(20, 20).Radius(10).Cursor(ui.CursorPointer)
					s.Draw(func(p *ui.Painter, r ui.Rect) {
						var path ui.Path
						path.Circle(r.X+r.W/2, r.Y+r.H/2, 9)
						p.FillPath(&path, ui.Hex(cand))
						if selected {
							p.StrokePath(&path, 2, c.Theme().Text)
						}
					})
					s.OnClick(func() {
						current := a.tabIndexByID(tb.ID)
						if current >= 0 {
							a.setTabColor(current, cand)
							a.settingsUI.expandedColor = 0
						}
					})
				}
			})
		}
	})
}

// tabAppearanceSection 标签栏外观：形状、标题宽、动画速度、毛玻璃回退强度。
func (a *app) tabAppearanceSection(c *ui.Context) {
	titleW := float64(a.cfg.TabTitleWidth)
	if titleW <= 0 {
		titleW = 120
	}
	glass := float64(a.cfg.GlassStrength)
	if glass <= 0 {
		glass = 0.5
	}
	ui.Column(c).FillWidth().Children(func() {
		settingsCompactRow(c, locales.TabDotShape(), "", func() {
			if ui.Segmented(c, &a.settingsUI.tabShapeIdx, locales.ShapeDot(), locales.ShapeRing()).
				Label(locales.TabDotShape()).Changed() {
				if a.settingsUI.tabShapeIdx == 1 {
					a.cfg.TabDotShape = tabDotShapeRing
				} else {
					a.cfg.TabDotShape = tabDotShapeDot
				}
				a.scheduleSave()
				a.withWindow(func() {})
			}
		})
		settingsCompactRow(c, locales.TabTitleMaxWidth(), locales.TabTitleMaxWidthHint(), func() {
			if ui.NumberInput(c, &titleW, 80, 220, 10).Changed() {
				a.cfg.TabTitleWidth = float32(titleW)
				a.scheduleSave()
				a.withWindow(func() {})
			}
		})
		settingsCompactRow(c, locales.TabAnimSpeed(), locales.TabAnimSpeedHint(), func() {
			opts := tabAnimSpeedOptions()
			displays := make([]string, len(opts))
			for i, o := range opts {
				displays[i] = o.display
			}
			if ui.Select(c, &a.settingsUI.tabAnimSpeedSel, displays).Width(200).Changed() {
				a.cfg.TabAnimSpeed = tabAnimSpeedKey(a.settingsUI.tabAnimSpeedSel)
				a.scheduleSave()
			}
		})
		settingsCompactRow(c, locales.TabMotion(), locales.TabMotionHint(), func() {
			motion := !a.cfg.TabMotionOff
			if ui.Switch(c, &motion).Label(locales.TabMotionSwitch()).Changed() {
				a.cfg.TabMotionOff = !motion
				a.scheduleSave()
				a.withWindow(func() {})
			}
		})
		settingsCompactRow(c, locales.GlassStrength(), locales.GlassStrengthHint(), func() {
			if ui.NumberInput(c, &glass, 0.2, 0.9, 0.05).Changed() {
				a.cfg.GlassStrength = float32(glass)
				a.scheduleSave()
				a.withWindow(func() {})
			}
		})
		ui.Text(c, locales.TabsPageFootnote()).
			FontSize(11).TextColor(tMuted(c)).PaddingY(10)
	})
}

func tabAnimSpeedKey(display string) float32 {
	for _, o := range tabAnimSpeedOptions() {
		if o.display == display {
			return o.speed
		}
	}
	return 1
}
