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
)

// tabAnimSpeedOptions 动画速度选择项（显示文案与倍率）。
var tabAnimSpeedOptions = []struct {
	display string
	speed   float32
}{
	{"慢", 0.5},
	{"正常", 1},
	{"快", 1.5},
	{"更快", 2},
}

func tabAnimSpeedDisplay(speed float32) string {
	if speed <= 0 {
		speed = 1
	}
	for _, o := range tabAnimSpeedOptions {
		if o.speed == speed {
			return o.display
		}
	}
	return tabAnimSpeedOptions[1].display // 非法值收敛到「正常」
}

// tabsPage 标签与外观设置页。
func (a *app) tabsPage(c *ui.Context, t *ui.Theme, dark bool) {
	a.settingsUI.tabAnimSpeedSel = tabAnimSpeedDisplay(a.cfg.TabAnimSpeed)
	ui.Scroll(c).Fill().Grow(1).Children(func() {
		ui.Column(c).FillWidth().Gap(14).Children(func() {
			settingsCard(c, "标签", func() {
				ui.Column(c).FillWidth().Padding(10, 14, 12, 14).Gap(10).Children(func() {
					a.tabListSection(c, dark)
				})
			})

			settingsCard(c, "标签栏外观", func() {
				a.tabAppearanceSection(c, dark)
			})
		})
	})
}

// tabListSection 标签列表：新增按钮 + 每行颜色/名称/快捷键/上移/下移/删除。
func (a *app) tabListSection(c *ui.Context, dark bool) {
	atCap := len(a.tabs) >= MaxTabCount
	ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(10).Children(func() {
		if ui.Button(c, "新建标签").Clicked() {
			a.addTab()
		}
		hint := fmt.Sprintf("共 %d 个；最多 %d 个，至少保留 1 个", len(a.tabs), MaxTabCount)
		if atCap {
			hint = fmt.Sprintf("已达上限 %d 个：先删除才能新增", MaxTabCount)
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
			a.tabRow(c, i, dark)
		}
	})
}

// tabRow 一条标签：颜色点（点开调色板）、名称（即时保存）、快捷键提示、
// 上移/下移/删除；点颜色点或行首区域切换当前标签。
func (a *app) tabRow(c *ui.Context, i int, dark bool) {
	tb := a.tabs[i]
	current := i == a.active
	ui.Column(c.Key(fmt.Sprintf("tabrow-%d", tb.ID))).FillWidth().Gap(4).Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(8).Children(func() {
			// 颜色点：点击切换当前标签；再点是行内的展开箭头语义，调色板
			// 在下方展开（避免弹出层与滚动容器嵌套）。
			dot := ui.ButtonBase(c.Key(fmt.Sprintf("tabcur-%d", tb.ID))).Label("切换到 "+a.menuTitle(i)).
				Size(18, 18).Radius(9).Cursor(ui.CursorPointer)
			dot.Draw(func(p *ui.Painter, r ui.Rect) {
				ink := a.tabColor(i)
				if a.cfg.MonoDots {
					ink = monoInk(dark)
				}
				var path ui.Path
				path.Circle(r.X+r.W/2, r.Y+r.H/2, 8)
				p.FillPath(&path, ink)
				if current {
					p.StrokePath(&path, 2, c.Theme().Text)
				}
			})
			if dot.Clicked() {
				a.switchTab(i)
			}

			// 重命名只在标签栏进行（⌘R / 双击圆点），设置页不再提供重命名。
			ui.Text(c, a.menuTitle(i)).Grow(1)

			if i < 9 {
				ui.Text(c, fmt.Sprintf("⌃%d", i+1)).FontSize(11).TextColor(tMuted(c))
			}
			if ui.Button(c.Key(fmt.Sprintf("tabup-%d", tb.ID)), "↑").
				Label("上移 " + a.menuTitle(i)).Disabled(i == 0).Clicked() {
				a.moveTab(i, -1)
			}
			if ui.Button(c.Key(fmt.Sprintf("tabdown-%d", tb.ID)), "↓").
				Label("下移 " + a.menuTitle(i)).Disabled(i == len(a.tabs)-1).Clicked() {
				a.moveTab(i, 1)
			}
			if ui.Button(c.Key(fmt.Sprintf("tabdel-%d", tb.ID)), "删除").
				Label("删除 " + a.menuTitle(i)).Clicked() {
				a.confirmDeleteTab(i)
			}
		})
		// 展开的调色板：12 色，点击改色。
		if a.settingsUI.expandedColor == tb.ID {
			ui.Row(c).FillWidth().Gap(6).PaddingX(20).Children(func() {
				for pi, cand := range tabPaletteColors {
					pi, cand := pi, cand
					selected := a.tabColor(i) == ui.Hex(cand)
					s := ui.ButtonBase(c.Key(fmt.Sprintf("pal-%d-%d", tb.ID, pi))).
						Label("标签颜色 "+cand).Size(20, 20).Radius(10).Cursor(ui.CursorPointer)
					s.Draw(func(p *ui.Painter, r ui.Rect) {
						var path ui.Path
						path.Circle(r.X+r.W/2, r.Y+r.H/2, 9)
						p.FillPath(&path, ui.Hex(cand))
						if selected {
							p.StrokePath(&path, 2, c.Theme().Text)
						}
					})
					if s.Clicked() {
						a.setTabColor(i, cand)
					}
				}
			})
		}
	})
}

// tabAppearanceSection 标签栏外观：标题宽、动画速度、明暗模式、
// 彩色圆点、毛玻璃回退强度。
func (a *app) tabAppearanceSection(c *ui.Context, dark bool) {
	titleW := float64(a.cfg.TabTitleWidth)
	if titleW <= 0 {
		titleW = 120
	}
	glass := float64(a.cfg.GlassStrength)
	if glass <= 0 {
		glass = 0.5
	}
	ui.Column(c).FillWidth().Children(func() {
		settingsRow(c, "标签标题最大宽度", "80–220 DIP；超出的文字用 … 省略，未命名标签只显示圆点", func() {
			if ui.NumberInput(c, &titleW, 80, 220, 10).Changed() {
				a.cfg.TabTitleWidth = float32(titleW)
				a.scheduleSave()
				a.withWindow(func() {})
			}
		}).PaddingX(14)
		cardDivider(c)
		settingsRow(c, "切换动画速度", "液态融合动画的节奏", func() {
			if ui.Select(c, &a.settingsUI.tabAnimSpeedSel,
				[]string{"慢", "正常", "快", "更快"}).Changed() {
				a.cfg.TabAnimSpeed = tabAnimSpeedKey(a.settingsUI.tabAnimSpeedSel)
				a.scheduleSave()
			}
		}).PaddingX(14)
		cardDivider(c)
		settingsRow(c, "界面外观", "浅色/深色/跟随系统，即时生效", func() {
			if ui.Select(c, &a.settingsUI.appearanceModeSel,
				[]string{"跟随系统", "浅色", "深色"}).Changed() {
				a.settingsUI.draft.Appearance.AppearanceMode = appearanceModeKey(a.settingsUI.appearanceModeSel)
				a.pushAppearance()
			}
		}).PaddingX(14)
		cardDivider(c)
		settingsRow(c, "彩色圆点", "关闭后圆点与胶囊使用中性色，不再区分标签颜色", func() {
			colored := !a.cfg.MonoDots
			if ui.Switch(c, &colored).Label("彩色圆点开关").Changed() {
				a.cfg.MonoDots = !colored
				a.scheduleSave()
				a.withWindow(func() {})
			}
		}).PaddingX(14)
		cardDivider(c)
		settingsRow(c, "毛玻璃回退强度", "无系统材质的平台上的半透明层浓度（20–90%）", func() {
			if ui.NumberInput(c, &glass, 0.2, 0.9, 0.05).Changed() {
				a.cfg.GlassStrength = float32(glass)
				a.scheduleSave()
				a.withWindow(func() {})
			}
		}).PaddingX(14)
		cardDivider(c)
		ui.Text(c, "标签内容、名称与颜色属于笔记数据，随自动保存落盘；窗口材质是运行平台能力，不支持时自动回退为半透明色。").
			FontSize(11).TextColor(tMuted(c)).Padding(8, 14, 10, 14)
	})
}

func tabAnimSpeedKey(display string) float32 {
	for _, o := range tabAnimSpeedOptions {
		if o.display == display {
			return o.speed
		}
	}
	return 1
}
