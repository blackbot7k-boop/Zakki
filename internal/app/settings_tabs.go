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
	"strings"

	"github.com/egoist/mygo/ui"

	"zakki/locales"
)

// settingsTabRowHeight 标签列表的固定行高：行内一行排开
// （取色器色块、名称、当前标记、删除图标）。
const settingsTabRowHeight float32 = 34

// tabsPage 标签设置页：标签列表（拖拽排序、取色、行内重命名、删除）。
// 顶栏外观项在「外观」页（见 settings_ui.go 的 tabAppearanceSection）。
func (a *app) tabsPage(c *ui.Context, t *ui.Theme) {
	ui.Scroll(c).Fill().Grow(1).ScrollbarInsets(0, 12, 0, 0).Children(func() {
		ui.Column(c).FillWidth().Padding(0, 22, 0, 0).Children(func() {
			settingsSectionTitle(c, locales.CardTabs())
			settingsLead(c, locales.TabsListLead())
			ui.Column(c).FillWidth().PaddingY(6).Gap(8).Children(func() {
				a.tabListSection(c)
			})
		})
	})
}

// tabListSection 标签列表：新增按钮 + 拖拽排序的行列表
// （点色块改色、双击名称重命名、行内删除图标）；行高固定，List 视口
// 恰好容纳全部行，与外层设置页滚动不嵌套。
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

	s := &a.settingsUI
	n := len(a.tabs)
	s.tabList.Reorder = func(rows []int, to int) {
		if len(rows) == 1 && a.reorderTab(rows[0], to) {
			a.reseedTabWells()
		}
	}
	if s.tabWellLen != n {
		// 行数变化（新建/删除）后行序整体漂移，草稿全部重播种。
		s.tabWellLen = n
		a.reseedTabWells()
	}
	ui.List(c, &s.tabList, n).FillWidth().Height(float32(n) * settingsTabRowHeight).
		ItemKey(func(row int) any { return a.tabs[row].ID }).
		Rows(func(r ui.ListRow) { a.settingsTabRow(r.Context, r.Index) })
}

// reseedTabWells 把每行取色器草稿重置为对应标签色并收起弹层（行序变化后调用）。
func (a *app) reseedTabWells() {
	s := &a.settingsUI
	for k := range a.tabs {
		s.tabWellColors[k] = a.tabColor(k)
		s.tabWellSeeded[k] = s.tabWellColors[k]
		s.tabWellOpen[k] = false
	}
}

// colorToHex 把取色器写出的颜色转成标签存储的 #rrggbb（标签色不透明，丢弃透明度）。
func colorToHex(c ui.Color) string {
	return fmt.Sprintf("#%02x%02x%02x", c.R, c.G, c.B)
}

// settingsTabRow 一行：颜色色块（点开取色器，含常用色）、名称（双击重命名）、
// 当前标记与删除图标；整行可上下拖拽排序（List.Reorder）。行内只有一枚
// 取色器在编辑：写入 tabWellColors 草稿、Changed 落盘；草稿偏离上次播种
// 基线（tabWellSeeded）即取色器已写入而 Changed 可能未到，绝不回播种。
func (a *app) settingsTabRow(c *ui.Context, i int) {
	tb := a.tabs[i]
	current := i == a.active
	s := &a.settingsUI
	renaming := s.tabRenaming == tb.ID
	if renaming {
		// 失焦即提交：焦点曾落在输入框、现已离开，且不在输入法组合中。
		if ui.FocusedValue(c, &s.tabRenameFocus) == 1 {
			s.tabRenameSeenFocus = true
		} else if s.tabRenameSeenFocus && !s.tabRenameComposing {
			a.commitSettingsRename()
			renaming = false
		}
	}
	row := ui.Row(c.Key(fmt.Sprintf("tabrow-%d", tb.ID))).FillWidth().Height(settingsTabRowHeight).
		Padding(0, 8).Radius(8).AlignItems(ui.Center).Gap(10)
	if current {
		row.Background(c.Theme().Surface)
	}
	row.Children(func() {
		// 草稿还在基线上而标签色变了：外部改色，回播种。
		if s.tabWellColors[i] == s.tabWellSeeded[i] && s.tabWellColors[i] != a.tabColor(i) {
			s.tabWellColors[i] = a.tabColor(i)
			s.tabWellSeeded[i] = s.tabWellColors[i]
		}
		// 小圆点与顶栏同形：点开 Popover 里的取色器（含常用色色板）。
		dot := ui.ButtonBase(c.Key(fmt.Sprintf("tabdot-%d", tb.ID))).
			Label(locales.TabColorOf(a.tabs[i].Color)).
			Size(18, 18).Radius(9).Cursor(ui.CursorPointer)
		dot.Draw(func(p *ui.Painter, r ui.Rect) {
			var path ui.Path
			path.Circle(r.X+r.W/2, r.Y+r.H/2, 7)
			p.FillPath(&path, a.tabColor(i))
		})
		if dot.Clicked() {
			s.tabWellOpen[i] = !s.tabWellOpen[i]
		}
		ui.Popover(c, dot, &s.tabWellOpen[i], func() {
			picker := ui.ColorPicker(c, &s.tabWellColors[i])
			if picker.Changed() {
				a.setTabColor(i, colorToHex(s.tabWellColors[i]))
				s.tabWellSeeded[i] = s.tabWellColors[i]
			}
		})
		if renaming {
			in := ui.TextInputBase(c.Key(fmt.Sprintf("tab-rename-%d", tb.ID)), &s.tabRenameDraft).
				Label(locales.RenameTabField()).Grow(1).
				FocusBind(&s.tabRenameFocus, 1).AutoFocus().
				HandleInput(func(ev ui.InputEvent) bool { return a.handleSettingsRenameInput(ev) })
			s.tabRenameComposing = in.Composing()
		} else {
			name := ui.ButtonBase(c.Key(fmt.Sprintf("tabname-%d", tb.ID))).Grow(1).
				Radius(6).Cursor(ui.CursorPointer).AlignItems(ui.Center).Justify(ui.Start).
				Label(locales.TabSwitchTo(a.menuTitle(i)))
			name.Children(func() {
				ui.Text(c, a.menuTitle(i))
			})
			if name.DoubleClicked() {
				a.beginSettingsRename(i)
			}
		}
		del := ui.ButtonBase(c.Key(fmt.Sprintf("tabdel-%d", tb.ID))).Label(locales.TabDelete(a.menuTitle(i))).
			Size(26, 26).Radius(6).Cursor(ui.CursorPointer).
			AlignItems(ui.Center).Justify(ui.Center).Disabled(len(a.tabs) <= 1)
		del.Children(func() {
			ui.Icon(c, iconTrash).Size(14, 14).TextColor(c.Theme().TextMuted)
		})
		del.OnClick(func() {
			if cur := a.tabIndexByID(tb.ID); cur >= 0 {
				a.confirmDeleteTab(cur)
			}
		})
	})
}

// beginSettingsRename 进入设置页第 i 个标签的行内重命名（双击名称）：
// 会话状态独立于顶栏胶囊重命名，两者互不干扰。
func (a *app) beginSettingsRename(i int) {
	if i < 0 || i >= len(a.tabs) {
		return
	}
	s := &a.settingsUI
	if s.tabRenaming != 0 && s.tabRenaming != a.tabs[i].ID {
		a.commitSettingsRename()
	}
	s.tabRenaming = a.tabs[i].ID
	s.tabRenameDraft = a.tabs[i].Name
	s.tabRenameFocus = 1
	s.tabRenameSeenFocus = false
}

// commitSettingsRename 提交设置页重命名（去首尾空白；空串 = 未命名只显示圆点）。
func (a *app) commitSettingsRename() {
	s := &a.settingsUI
	if s.tabRenaming == 0 {
		return
	}
	id, draft := s.tabRenaming, s.tabRenameDraft
	a.endSettingsRename()
	if i := a.tabIndexByID(id); i >= 0 && a.tabs[i].Name != strings.TrimSpace(draft) {
		a.renameTab(i, draft)
	}
}

// cancelSettingsRename 取消设置页重命名，不落盘。
func (a *app) cancelSettingsRename() { a.endSettingsRename() }

func (a *app) endSettingsRename() {
	s := &a.settingsUI
	s.tabRenaming, s.tabRenameDraft, s.tabRenameFocus = 0, "", 0
	s.tabRenameSeenFocus, s.tabRenameComposing = false, false
}

// handleSettingsRenameInput 设置页重命名输入框按键：Enter/Tab 提交，
// Escape 取消；其余事件（含输入法组合）交还原生输入框。
func (a *app) handleSettingsRenameInput(ev ui.InputEvent) bool {
	if ev.Kind != ui.InputKeyDown {
		return false
	}
	switch ev.Key {
	case ui.KeyEnter, ui.KeyTab:
		if !a.settingsUI.tabRenameComposing {
			a.commitSettingsRename()
			return true
		}
		return false
	case ui.KeyEscape:
		a.cancelSettingsRename()
		return true
	}
	return false
}
