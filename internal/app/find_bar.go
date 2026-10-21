package app

import (
	"fmt"

	"github.com/egoist/mygo/ui"

	"zakki/locales"
)

// ---- 查找条 ----

// findStep 在命中集合里循环跳匹配（+1 下一个 / -1 上一个）；
// 无命中时不动，调用方（查找条按钮、Enter、菜单查找下一个/上一个）共用。
func (a *app) findStep(dir int) {
	ms := a.docMatches(a.activeTab().Text, a.query)
	if len(ms) == 0 {
		return
	}
	a.matchIdx = (a.matchIdx + dir + len(ms)) % len(ms)
	a.jumpPending = true
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
		a.findOpen, a.query, a.matchIdx, a.findFocus = true, "", 0, 1
		return
	}
	a.findStep(dir)
}

func (a *app) findBar(c *ui.Context, t *ui.Theme, dark bool) {
	ui.Row(c).Height(44).Background(a.chromeColor(headerBackground(dark), dark, c.Vibrancy())).
		Padding(0, headerPadX, 0, 14).Gap(4).AlignItems(ui.Center).
		BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
		in := ui.TextInputBase(c.Key("find-query"), &a.query).
			Label(locales.FixedFind()).Placeholder(locales.FixedFind()).Grow(1).
			MinWidth(120).Height(30).PaddingX(10).Radius(6).
			Background(t.Background).Border(1, t.Border).
			FocusBind(&a.findFocus, 1).
			HandleInput(func(ev ui.InputEvent) bool {
				// Shift+Enter 跳到上一个命中；普通 Enter 走 Submitted。
				if ev.Kind == ui.InputKeyDown && ev.Key == ui.KeyEnter && ev.Mods&ui.Shift != 0 && !a.findComposing {
					a.findStep(-1)
					return true
				}
				return false
			})
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
		matches := a.docMatches(a.activeTab().Text, a.query)
		count := locales.FindNoResults()
		if a.query != "" && len(matches) > 0 {
			a.matchIdx = clamp(a.matchIdx, 0, len(matches)-1)
			count = fmt.Sprintf("%d/%d", a.matchIdx+1, len(matches))
		} else if a.query != "" {
			count = "0/0"
		}
		ui.Text(c, count).FontSize(12).TextColor(t.TextMuted)
		ink, hover := t.TextMuted, chromeHover(t.TextMuted)
		if chromeIcon(c, "find-prev", iconChevronUp, locales.FindPrev(), ink, hover, false).Clicked() {
			a.findStep(-1)
		}
		if chromeIcon(c, "find-next", iconChevronDown, locales.FindNext(), ink, hover, false).Clicked() {
			a.findStep(1)
		}
		if chromeIcon(c, "find-close", iconX, locales.FindClose(), ink, hover, false).Clicked() {
			a.findOpen = false
		}
	})
}
