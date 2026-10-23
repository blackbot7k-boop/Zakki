package app

import (
	"fmt"
	"strconv"

	"github.com/egoist/mygo/ui"

	"zakki/locales"
)

// ---- 查找条 ----

// openFind 打开查找条（⌘F / ⇧⌘F 与菜单共用）：未打开时清空查询，
// 已打开时保留查询，只切换查找范围并把焦点放到查找框。
func (a *app) openFind(allTabs bool) {
	if !a.findOpen {
		a.findOpen, a.query, a.matchIdx = true, "", 0
		a.findAllTabs = false
	}
	a.findAllTabs = a.findAllTabs || allTabs
	a.findFocus = 1
}

// closeFind 关闭查找条；查询词保留到下次打开前，但不再高亮。
func (a *app) closeFind() {
	a.findOpen, a.findFocus = false, 0
}

// findStep 在命中集合里循环跳匹配（+1 下一个 / -1 上一个）；
// 无命中时不动，调用方（查找条按钮、Enter、菜单查找下一个/上一个）共用。
// 跨标签查找时越过当前标签首尾，切到前/后一个有命中的标签。
func (a *app) findStep(dir int) {
	ms := a.docMatches(a.activeTab().Text, a.query)
	if !a.findAllTabs || a.query == "" {
		if len(ms) == 0 {
			return
		}
		a.matchIdx = (a.matchIdx + dir + len(ms)) % len(ms)
		a.jumpPending = true
		return
	}
	if next := a.matchIdx + dir; len(ms) > 0 && next >= 0 && next < len(ms) {
		a.matchIdx, a.jumpPending = next, true
		return
	}
	n := len(a.tabs)
	for k := 1; k <= n; k++ {
		j := ((a.active+dir*k)%n + n) % n
		cnt := a.tabHitCount(j)
		if cnt == 0 {
			continue
		}
		if j != a.active {
			a.switchTabKeepFind(j)
		}
		a.matchIdx = 0
		if dir < 0 {
			a.matchIdx = cnt - 1
		}
		a.jumpPending = true
		return
	}
}

// switchTabKeepFind 跨标签跳转命中：切换标签会重置查找状态，这里原样带过去。
func (a *app) switchTabKeepFind(i int) {
	query, focus := a.query, a.findFocus
	a.switchTab(i)
	a.findOpen, a.query, a.findAllTabs = true, query, true
	a.findFocus = focus
}

// tabHitCount 第 i 个标签的命中数：当前标签复用 docMatches 缓存，其余按标签
// 缓存（文本或查询变了才重扫）。
func (a *app) tabHitCount(i int) int {
	if a.query == "" {
		return 0
	}
	if i == a.active {
		return len(a.docMatches(a.activeTab().Text, a.query))
	}
	tb := a.tabs[i]
	if hit, ok := a.tabHits[tb.ID]; ok && hit.doc == tb.Text && hit.query == a.query {
		return hit.n
	}
	n := len(findMatches(tb.Text, a.query))
	if a.tabHits == nil {
		a.tabHits = make(map[uint64]tabHitCount)
	}
	a.tabHits[tb.ID] = tabHitCount{doc: tb.Text, query: a.query, n: n}
	return n
}

// allTabHits 跨标签命中总数与有命中的标签数。
func (a *app) allTabHits() (total, tabs int) {
	for i := range a.tabs {
		if n := a.tabHitCount(i); n > 0 {
			total += n
			tabs++
		}
	}
	return total, tabs
}

// highlightQuery 当前要标记命中的查询：查找条关闭后正文不再保留命中标记。
func (a *app) highlightQuery() string {
	if !a.findOpen {
		return ""
	}
	return a.query
}

// findOpenOrStep 菜单「查找下一个/上一个」入口：查找条未开时先打开，否则跳匹配。
func (a *app) findOpenOrStep(dir int) {
	if !a.findOpen {
		a.openFind(false)
		return
	}
	a.findStep(dir)
}

// ---- 查找条视图 ----

func (a *app) findBar(c *ui.Context, t *ui.Theme, dark bool) {
	ink := t.TextMuted
	ui.Column(c).FillWidth().Background(a.chromeColor(headerBackground(dark), dark, c.Vibrancy())).
		Padding(7, headerPadX, 7, headerPadX).Gap(6).
		BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
		ui.Row(c).FillWidth().Gap(4).AlignItems(ui.Center).Children(func() {
			a.findQueryField(c, t)
			scopeInk := ink
			if a.findAllTabs {
				scopeInk = t.Accent
			}
			if chromeIcon(c, "find-all-tabs", iconLayers, locales.FindAllTabs(), scopeInk).Clicked() {
				a.findAllTabs = !a.findAllTabs
				a.matchIdx = 0
			}
			if chromeIcon(c, "find-prev", iconChevronUp, locales.FindPrev(), ink).Clicked() {
				a.findStep(-1)
			}
			if chromeIcon(c, "find-next", iconChevronDown, locales.FindNext(), ink).Clicked() {
				a.findStep(1)
			}
			if chromeIcon(c, "find-close", iconX, locales.FindClose(), ink).Clicked() {
				a.closeFind()
			}
		})
	})
}

// findField 查找条输入框外壳：圆角底、聚焦时描边换强调色，左侧图标、右侧附注。
func findField(c *ui.Context, t *ui.Theme, focused bool, icon *ui.SVG, body func()) ui.Element {
	border := t.Border
	if focused {
		border = t.Accent
	}
	row := ui.Row(c).Grow(1).MinWidth(0).Height(30).PaddingX(8).Gap(6).Radius(8).
		AlignItems(ui.Center).Background(t.Background).Border(1, border)
	row.Children(func() {
		if icon != nil {
			ui.Icon(c, icon).Size(14, 14).TextColor(t.TextMuted).Shrink(0)
		}
		body()
	})
	return row
}

func (a *app) findQueryField(c *ui.Context, t *ui.Theme) {
	focused := ui.FocusedValue(c, &a.findFocus) == 1
	findField(c, t, focused, iconSearch, func() {
		in := ui.TextInputBase(c.Key("find-query"), &a.query).
			Label(locales.FixedFind()).Placeholder(locales.FixedFind()).
			Grow(1).MinWidth(60).FocusBind(&a.findFocus, 1).
			HandleInput(func(ev ui.InputEvent) bool { return a.findInputKey(ev) })
		a.findComposing = in.Composing()
		if in.Changed() {
			a.matchIdx = 0
			// 输入查询本身不强制定位：预览态不该因为打关键词就把命中行变成源码。
			a.jumpPending = false
		}
		// Enter 跳到下一个命中。
		if in.Submitted() {
			a.findStep(1)
		}
		count, color := a.findCountText(), t.TextMuted
		if a.query != "" && a.tabHitCount(a.active) == 0 && !a.findAllTabs {
			color = t.Danger
		}
		ui.Text(c, count).FontSize(12).TextColor(color).SingleLine().MinWidth(0).Shrink(1).Tooltip(count)
	})
}

// findInputKey 查找框按键：Shift+Enter 上一个，Esc 关闭查找条；组合中的按键归输入法。
func (a *app) findInputKey(ev ui.InputEvent) bool {
	if ev.Kind != ui.InputKeyDown || a.findComposing {
		return false
	}
	switch {
	case ev.Key == ui.KeyEnter && ev.Mods&ui.Shift != 0:
		a.findStep(-1)
		return true
	case ev.Key == ui.KeyEscape && ev.Mods == 0:
		a.closeFind()
		return true
	}
	return false
}

// findCountText 计数：当前标签「第几/共几」，跨标签时附全部标签的总数。
func (a *app) findCountText() string {
	if a.query == "" {
		return ""
	}
	n := a.tabHitCount(a.active)
	cur := 0
	if n > 0 {
		a.matchIdx = clamp(a.matchIdx, 0, n-1)
		cur = a.matchIdx + 1
	}
	if a.findAllTabs {
		total, tabs := a.allTabHits()
		return locales.FindCountAll(strconv.Itoa(cur), strconv.Itoa(n), strconv.Itoa(total), strconv.Itoa(tabs))
	}
	if n == 0 {
		return locales.FindNoResults()
	}
	return fmt.Sprintf("%d/%d", cur, n)
}
