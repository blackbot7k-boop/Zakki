package app

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"zakki/locales"
)

// ---- 查找条 ----

// openFind 打开查找条（⌘F / ⌥⌘F / ⇧⌘F 与菜单共用）：未打开时清空查询，
// 已打开时保留查询只切换替换行与范围，焦点落到对应输入框。
func (a *app) openFind(replace, allTabs bool) {
	if !a.findOpen {
		a.findOpen, a.query, a.matchIdx = true, "", 0
		a.replaceOpen, a.findAllTabs = false, false
	}
	a.replaceOpen = a.replaceOpen || replace
	a.findAllTabs = a.findAllTabs || allTabs
	a.findFocus = 1
	if replace {
		a.findFocus = 2
	}
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
	query, replaceOpen, replaceText, focus := a.query, a.replaceOpen, a.replaceText, a.findFocus
	a.switchTab(i)
	a.findOpen, a.query, a.findAllTabs = true, query, true
	a.replaceOpen, a.replaceText, a.findFocus = replaceOpen, replaceText, focus
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
		a.openFind(false, false)
		return
	}
	a.findStep(dir)
}

// ---- 替换 ----

// replaceMatches 把 text 中给定命中（rune 区间，升序不重叠）全部替换为 with。
func replaceMatches(text string, ms []ui.TextRange, with string) string {
	var b strings.Builder
	b.Grow(len(text))
	runes, byteAt := 0, 0
	for _, m := range ms {
		start := advanceRunes(text, byteAt, m.Start-runes)
		end := advanceRunes(text, start, m.End-m.Start)
		b.WriteString(text[byteAt:start])
		b.WriteString(with)
		byteAt, runes = end, m.End
	}
	b.WriteString(text[byteAt:])
	return b.String()
}

// advanceRunes 从字节偏移 at 起前进 n 个 rune，返回新的字节偏移。
func advanceRunes(s string, at, n int) int {
	for ; n > 0 && at < len(s); n-- {
		_, size := utf8.DecodeRuneInString(s[at:])
		at += size
	}
	return at
}

// prepareFindEdit 替换前把实时预览焦点块的草稿并入文档，命中坐标才与文本一致。
func (a *app) prepareFindEdit() {
	if !a.cfg.Source && a.mdActive >= 0 {
		a.commitDraft(-1)
	}
}

// replaceCurrent 替换当前命中并跳到其后的下一个命中；一次替换是一个撤销事务。
func (a *app) replaceCurrent() {
	a.prepareFindEdit()
	text := a.activeTab().Text
	ms := a.docMatches(text, a.query)
	if len(ms) == 0 {
		if a.findAllTabs {
			a.findStep(1)
		}
		return
	}
	idx := clamp(a.matchIdx, 0, len(ms)-1)
	m := ms[idx]
	next := replaceMatches(text, ms[idx:idx+1], a.replaceText)
	caret := m.Start + utf8.RuneCountInString(a.replaceText)
	a.runAsCommand(func() docResult {
		return docResult{doc: next, selStart: m.Start, selEnd: caret, changed: next != text}
	})
	rest := a.docMatches(a.activeTab().Text, a.query)
	a.matchIdx = 0
	for i, r := range rest {
		if r.Start >= caret {
			a.matchIdx = i
			break
		}
	}
	if len(rest) > 0 {
		a.jumpPending = true
	} else if a.findAllTabs {
		a.findStep(1)
	}
}

// replaceAllCurrentTab 替换当前标签内全部命中，作为一个撤销事务；返回替换数。
func (a *app) replaceAllCurrentTab(query, with string) int {
	a.prepareFindEdit()
	text := a.activeTab().Text
	ms := findMatches(text, query)
	if len(ms) == 0 {
		return 0
	}
	next := replaceMatches(text, ms, with)
	at := ms[0].Start
	a.runAsCommand(func() docResult {
		return docResult{doc: next, selStart: at, selEnd: at, changed: next != text}
	})
	a.matchIdx = 0
	return len(ms)
}

// replaceAll 「全部替换」：单标签直接替换（可撤销）；跨标签先弹确认，确认后
// 先做安全备份再替换（其他标签的改动不进撤销历史），备份失败不动任何标签。
func (a *app) replaceAll() {
	query, with := a.query, a.replaceText
	if query == "" {
		return
	}
	if !a.findAllTabs {
		if n := a.replaceAllCurrentTab(query, with); n > 0 {
			a.notice = locales.FindReplacedCount(strconv.Itoa(n))
		}
		return
	}
	total, tabs := a.allTabHits()
	if total == 0 {
		return
	}
	if a.win == nil {
		a.replaceAllTabsNow(query, with)
		return
	}
	go func() {
		res, err := mygo.Dialog.Message(mygo.MessageOptions{
			Parent:        a.win,
			Type:          mygo.MessageQuestion,
			Title:         locales.DialogReplaceAllTitle(),
			Message:       locales.DialogReplaceAllMessage(strconv.Itoa(total), strconv.Itoa(tabs)),
			Detail:        locales.DialogReplaceAllDetail(),
			Buttons:       []string{locales.FindReplaceAll(), locales.CommonCancel()},
			DefaultButton: 1,
			CancelButton:  1,
		})
		if err != nil || res.Button != 0 {
			return
		}
		a.runOnMain(func() {
			if a.winOpen {
				a.replaceAllTabsNow(query, with)
			}
		})
	}()
}

// replaceAllTabsNow 跨标签全部替换的执行体（主线程）；查询与替换文本是
// 对话框弹出前捕获的不可变值，期间改了查找框也按确认时的内容执行。
func (a *app) replaceAllTabsNow(query, with string) int {
	total := a.replaceAllCurrentTab(query, with)
	for i := range a.tabs {
		if i == a.active {
			continue
		}
		if ms := findMatches(a.tabs[i].Text, query); len(ms) > 0 {
			a.tabs[i].Text = replaceMatches(a.tabs[i].Text, ms, with)
			total += len(ms)
		}
	}
	a.notice = locales.FindReplacedCount(strconv.Itoa(total))
	a.scheduleSave()
	return total
}

// ---- 查找条视图 ----

func (a *app) findBar(c *ui.Context, t *ui.Theme, dark bool) {
	ink, hover := t.TextMuted, chromeHover(t.TextMuted)
	ui.Column(c).FillWidth().Background(a.chromeColor(headerBackground(dark), dark, c.Vibrancy())).
		Padding(7, headerPadX, 7, headerPadX).Gap(6).
		BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
		ui.Row(c).FillWidth().Gap(4).AlignItems(ui.Center).Children(func() {
			toggle := iconChevronRight
			if a.replaceOpen {
				toggle = iconChevronDown
			}
			if chromeIcon(c, "find-replace-toggle", toggle, locales.FindReplaceToggle(), ink, hover, false).Clicked() {
				a.replaceOpen = !a.replaceOpen
				if a.replaceOpen {
					a.findFocus = 2
				}
			}
			a.findQueryField(c, t)
			scopeInk := ink
			if a.findAllTabs {
				scopeInk = t.Accent
			}
			if chromeIcon(c, "find-all-tabs", iconLayers, locales.FindAllTabs(), scopeInk, hover, a.findAllTabs).Clicked() {
				a.findAllTabs = !a.findAllTabs
				a.matchIdx = 0
			}
			if chromeIcon(c, "find-prev", iconChevronUp, locales.FindPrev(), ink, hover, false).Clicked() {
				a.findStep(-1)
			}
			if chromeIcon(c, "find-next", iconChevronDown, locales.FindNext(), ink, hover, false).Clicked() {
				a.findStep(1)
			}
			if chromeIcon(c, "find-close", iconX, locales.FindClose(), ink, hover, false).Clicked() {
				a.closeFind()
			}
		})
		if a.replaceOpen {
			a.replaceRow(c, t)
		}
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
		ui.Text(c, count).FontSize(12).TextColor(color).SingleLine().Shrink(0)
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

func (a *app) replaceRow(c *ui.Context, t *ui.Theme) {
	ui.Row(c).FillWidth().Gap(4).AlignItems(ui.Center).Children(func() {
		ui.Box(c).Width(chromeHit).Shrink(0)
		focused := ui.FocusedValue(c, &a.findFocus) == 2
		findField(c, t, focused, nil, func() {
			in := ui.TextInputBase(c.Key("find-replace"), &a.replaceText).
				Label(locales.FindReplaceField()).Placeholder(locales.FindReplaceField()).
				Grow(1).MinWidth(60).FocusBind(&a.findFocus, 2).
				HandleInput(func(ev ui.InputEvent) bool {
					if ev.Kind == ui.InputKeyDown && ev.Key == ui.KeyEscape && ev.Mods == 0 && !a.replaceComposing {
						a.closeFind()
						return true
					}
					return false
				})
			a.replaceComposing = in.Composing()
			if in.Submitted() {
				a.replaceCurrent()
			}
		})
		disabled := a.query == ""
		if findTextButton(c, t, "find-replace-one", locales.FindReplaceOne(), disabled).Clicked() {
			a.replaceCurrent()
		}
		if findTextButton(c, t, "find-replace-all", locales.FindReplaceAll(), disabled).Clicked() {
			a.replaceAll()
		}
	})
}

// findTextButton 查找条的小号文字按钮（替换 / 全部替换）。
func findTextButton(c *ui.Context, t *ui.Theme, key, label string, disabled bool) ui.Element {
	btn := ui.ButtonBase(c.Key(key)).Label(label).Height(30).PaddingX(10).Radius(8).
		Border(1, t.Border).Background(t.Background).Cursor(ui.CursorPointer).
		AlignItems(ui.Center).Justify(ui.Center).Shrink(0).Disabled(disabled)
	if btn.Hovered() && !disabled {
		btn.Background(t.Text.Mix(t.Background, 0.94))
	}
	ink := t.Text
	if disabled {
		ink = t.TextMuted
	}
	btn.Children(func() {
		ui.Text(c, label).FontSize(12).TextColor(ink).SingleLine()
	})
	return btn
}
