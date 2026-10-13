package main

// 主界面，对齐 Tot 的版式与尺寸（尺寸/颜色采样自真机截图）：
//   顶栏 32pt，固定浅灰不随点变，可拖动窗口：⊗ 清空 · 7 圆点环居中 · 置顶
//   查找条（搜索图标打开）：搜索框 + 命中计数，Enter 跳到下一个
//   内容区：编辑器（聚焦时画细边框）或 markdown 预览，背景为该点主题底色
//   底栏 36pt：● 行/词/字符 · 备份时间 ··· 圆点 / 预览 / 搜索（Lucide 图标）

import (
	"fmt"

	"github.com/egoist/mygo/ui"
)

// view 每帧由主线程调用，从状态构建界面。
func (a *app) view(c *ui.Context) {
	t0 := c.Theme()
	dark := themeIsDark(t0)
	// 主题 accent 跟随当前点的主题色：输入框焦点边框、勾选框等随之着色（Tot 行为）。
	t := *t0
	t.Accent = dotAccent(a.active, dark)
	c.SetTheme(&t)

	// Esc：先关查找条，再按设置隐藏窗口。
	if c.Shortcut(0, ui.KeyEscape) {
		if a.findOpen {
			a.findOpen = false
		} else if a.cfg.HideOnEscape && a.win != nil {
			a.win.Hide()
		}
	}

	ui.Column(c).Fill().Children(func() {
		a.header(c, &t, dark)
		if a.findOpen {
			a.findBar(c, &t, dark)
		}
		if a.cfg.Preview {
			a.previewPane(c, &t, dark)
		} else {
			a.editorPane(c, &t, dark)
		}
		a.statusBar(c, &t, dark)
	})
}

// ---- 顶栏（52pt，固定浅灰，Tot 同款：无红绿灯）----

func (a *app) header(c *ui.Context, t *ui.Theme, dark bool) {
	ui.Row(c).Height(32).Background(headerBackground(dark)).
		Padding(0, 14).
		BorderWidth(0, 0, 1, 0).BorderColor(t.Border).
		DragWindow().AlignItems(ui.Center).Children(func() {
		// 左：清空当前点（Tot 的 ⊗）
		if iconAction(c, "clear-dot", iconClearDot, "清空当前点", t, 15) {
			a.dots[a.active] = ""
			a.onEdited()
		}
		ui.Box(c).Grow(1)
		a.dotsRow(c)
		ui.Box(c).Grow(1)
		// 右：置顶
		a.iconPin(c, t, dark)
	})
}

// dotsRow 七个圆点：Ø17、间距 15；未激活空心环，激活实心（Tot 的点样式）。
func (a *app) dotsRow(c *ui.Context) {
	ui.Row(c).Gap(14).AlignItems(ui.Center).Children(func() {
		for i := 0; i < DotCount; i++ {
			i := i
			btn := ui.Button(c.Key(fmt.Sprintf("dot-%d", i)), "").Label(dotLabel(i)).
				Width(16).Height(16).Radius(999)
			if i == a.active {
				btn.Background(dotColor(i))
			} else {
				btn.Background(ui.Transparent).Border(2, dotColor(i))
			}
			if btn.Clicked() {
				a.switchDot(i)
			}
		}
	})
}

// iconPin 置顶开关：激活时用点主题色并加粗。
func (a *app) iconPin(c *ui.Context, t *ui.Theme, dark bool) {
	g := ui.Icon(c.Key("pin-top"), iconPin).Size(16, 16).Label("置顶").Cursor(ui.CursorPointer)
	if a.cfg.AlwaysOnTop {
		g.Bold().TextColor(dotAccent(a.active, dark))
	} else {
		g.TextColor(t.TextMuted)
	}
	if g.Clicked() {
		a.cfg.AlwaysOnTop = !a.cfg.AlwaysOnTop
		a.applyAlwaysOnTop()
		a.syncMenus()
		a.scheduleSave()
	}
}

// iconAction 无边框 SVG 图标按钮（Size 是 border-box，勿再加 Padding）。
func iconAction(c *ui.Context, key string, svg *ui.SVG, label string, t *ui.Theme, size float32) bool {
	return ui.Icon(c.Key(key), svg).Size(size, size).Label(label).
		TextColor(t.TextMuted).Cursor(ui.CursorPointer).Clicked()
}

// ---- 查找条 ----

func (a *app) findBar(c *ui.Context, t *ui.Theme, dark bool) {
	ui.Row(c).Height(44).Background(headerBackground(dark)).
		Padding(0, 14).Gap(8).AlignItems(ui.Center).
		BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
		sf := ui.SearchField(c.Key("find-query"), &a.query).Grow(1)
		if sf.Changed() {
			a.matchIdx = 0
			a.jumpPending = a.query != ""
		}
		// Enter 跳到下一个命中。
		if sf.Submitted() {
			if ms := findMatches(a.dots[a.active], a.query); len(ms) > 0 {
				a.matchIdx = (a.matchIdx + 1) % len(ms)
				a.jumpPending = true
			}
		}
		matches := findMatches(a.dots[a.active], a.query)
		if ui.Button(c.Key("find-prev"), "‹").Label("上一个").Clicked() && len(matches) > 0 {
			a.matchIdx = (a.matchIdx - 1 + len(matches)) % len(matches)
			a.jumpPending = true
		}
		count := "无结果"
		if a.query != "" && len(matches) > 0 {
			a.matchIdx = clamp(a.matchIdx, 0, len(matches)-1)
			count = fmt.Sprintf("%d/%d", a.matchIdx+1, len(matches))
		} else if a.query != "" {
			count = "0/0"
		}
		ui.Text(c, count).FontSize(12).TextColor(t.TextMuted)
		if ui.Button(c.Key("find-next"), "›").Label("下一个").Clicked() && len(matches) > 0 {
			a.matchIdx = (a.matchIdx + 1) % len(matches)
			a.jumpPending = true
		}
		if ui.Button(c.Key("find-close"), "完成").Clicked() {
			a.findOpen = false
		}
	})
}

// ---- 编辑器 ----

func (a *app) editorPane(c *ui.Context, t *ui.Theme, dark bool) {
	text := &a.dots[a.active]
	matches := findMatches(*text, a.query)
	accent := dotAccent(a.active, dark)

	// TextAreaBase 无自带样式：背景/内边距自己给，聚焦时不会有任何边框。
	ta := ui.TextAreaBase(c, text).
		Label("内容").
		Fill().Grow(1).
		Padding(14, 20).
		FontSize(15).
		FocusRing(false).
		Background(dotBackground(a.active, dark)).
		AutoFocus()

	// 底栏 ✱ 或菜单请求在行首插入/取消圆点。
	if a.insertBullet {
		a.insertBullet = false
		start, _ := ta.TextSelection()
		newText, caret := toggleBulletPrefix(*text, start)
		if caret < 0 {
			caret = 0
		}
		*text = newText
		ta.SetTextSelection(caret, caret)
		a.onEdited()
	}

	ta.TextRanges(editorRanges(*text, a.query, t, accent)...)

	// 查找跳转：把选区移到当前命中处（编辑器会滚动到选区）。
	if a.jumpPending {
		a.jumpPending = false
		if len(matches) > 0 {
			a.matchIdx = clamp(a.matchIdx, 0, len(matches)-1)
			m := matches[a.matchIdx]
			ta.SetTextSelection(m.Start, m.End)
		}
	}

	if ta.Changed() {
		a.onEdited()
	}
}

// ---- 预览 ----

func (a *app) previewPane(c *ui.Context, t *ui.Theme, dark bool) {
	src := a.dots[a.active]
	ui.Scroll(c).Grow(1).Fill().Background(dotBackground(a.active, dark)).Children(func() {
		ui.Column(c).Fill().Padding(16, 24).Gap(10).Selectable().Children(func() {
			blocks := parseBlocks(src)
			if len(blocks) == 0 {
				ui.Text(c, "（空）").TextColor(t.TextMuted)
			}
			for _, blk := range blocks {
				blk := blk
				a.renderBlock(c, t, dark, blk)
			}
		})
	})
}

func (a *app) renderBlock(c *ui.Context, t *ui.Theme, dark bool, blk block) {
	accent := dotAccent(a.active, dark)
	switch blk.kind {
	case bkHeading:
		h := ui.Text(c, blk.text).Bold().TextColor(t.Text)
		switch blk.level {
		case 1:
			h.FontSize(24)
		case 2:
			h.FontSize(20)
		case 3:
			h.FontSize(17)
		default:
			h.FontSize(15)
		}
	case bkRule:
		ui.Box(c).Height(1).FillWidth().Background(t.Border)
	case bkQuote:
		ui.Row(c).Gap(10).Children(func() {
			ui.Box(c).Width(3).FillHeight().Radius(2).Background(t.Border)
			ui.RichText(c).TextColor(t.TextMuted).Children(func() {
				renderInline(c, t, accent, blk.text)
			})
		})
	case bkCode:
		ui.Column(c).Fill().Padding(10, 12).Radius(8).Background(t.Surface).Children(func() {
			for _, l := range blk.lines {
				if l == "" {
					l = " "
				}
				ui.Text(c, l).Font("monospace").FontSize(13)
			}
		})
	case bkList:
		ui.Row(c).Gap(8).Children(func() {
			label := "•"
			if blk.n > 0 {
				label = fmt.Sprintf("%d.", blk.n)
			}
			ui.Text(c, label).TextColor(dotColor(a.active)).Bold()
			ui.RichText(c).Grow(1).Children(func() { renderInline(c, t, accent, blk.text) })
		})
	case bkTask:
		checked := blk.checked
		cb := ui.Checkbox(c.Key(fmt.Sprintf("task-%d", blk.lineNo)), &checked, blk.text)
		cb.OnChange(func() {
			a.dots[a.active] = toggleTaskLine(a.dots[a.active], blk.lineNo)
			a.onEdited()
		})
	default:
		ui.RichText(c).FontSize(15).Children(func() { renderInline(c, t, accent, blk.text) })
	}
}

// renderInline 在当前 RichText 段落内输出带样式的行内片段。
func renderInline(c *ui.Context, t *ui.Theme, accent ui.Color, text string) {
	spans := inlineSpans(text)
	if len(spans) == 0 {
		ui.Text(c, text)
		return
	}
	prev := 0
	rs := []rune(text)
	for _, sp := range spans {
		if sp.start > prev {
			ui.Text(c, string(rs[prev:sp.start]))
		}
		styled := ui.Text(c, spanText(text, sp))
		switch sp.kind {
		case spanBold:
			styled.Bold()
		case spanItalic:
			styled.Italic()
		case spanCode:
			styled.Font("monospace").TextColor(t.TextMuted)
		case spanLink:
			styled.TextColor(accent).Underline()
		}
		prev = sp.end
	}
	if prev < len(rs) {
		ui.Text(c, string(rs[prev:]))
	}
}

// ---- 底栏（36pt，随点的 status 色，Tot 同布局）----

func (a *app) statusBar(c *ui.Context, t *ui.Theme, dark bool) {
	bg := dotStatusBackground(a.active, dark)
	fg := dotAccent(a.active, dark)
	ui.Row(c).Height(36).Background(bg).
		Padding(0, 14).Gap(8).AlignItems(ui.Center).Children(func() {
		ui.Box(c).Width(8).Height(8).Radius(999).Background(dotColor(a.active))
		ui.Textf(c, "%d 行 · %d 词 · %d 字符",
			countLines(a.dots[a.active]), countWords(a.dots[a.active]), countChars(a.dots[a.active])).
			FontSize(12).TextColor(fg)
		ui.Box(c).Width(1).Height(16).Background(fg.Mix(bg, 0.55))
		ui.Text(c, a.backupLabel()).FontSize(12).TextColor(fg)
		ui.Box(c).Grow(1)

		// 圆点：行首插入（Lucide asterisk）
		a.iconStatusToggle(c, "sb-bullet", iconBullet, "插入圆点", false, t, dark, func() {
			a.insertBullet = true
		})
		// 预览开关（Lucide eye）
		a.iconStatusToggle(c, "sb-preview", iconPreview, "预览", a.cfg.Preview, t, dark, func() {
			a.cfg.Preview = !a.cfg.Preview
			a.syncMenus()
			a.scheduleSave()
		})
		// 搜索开关（Lucide search）
		a.iconStatusToggle(c, "sb-find", iconSearch, "搜索", a.findOpen, t, dark, func() {
			a.findOpen = !a.findOpen
			if a.findOpen {
				a.query, a.matchIdx = "", 0
			}
		})
	})
}

// iconStatusToggle 底栏图标：与 Tot 一致用 status_text 色；激活加粗提亮。
func (a *app) iconStatusToggle(c *ui.Context, key string, svg *ui.SVG, label string, on bool, t *ui.Theme, dark bool, fn func()) {
	fg := dotAccent(a.active, dark)
	ix := ui.Icon(c.Key(key), svg).Size(16, 16).Label(label).Cursor(ui.CursorPointer)
	if on {
		ix.Bold().TextColor(fg)
	} else {
		ix.TextColor(fg.Mix(dotStatusBackground(a.active, dark), 0.30))
	}
	if ix.Clicked() {
		fn()
	}
}

// ---- 状态流转 ----

// switchDot 切换当前点，并关掉查找条。
func (a *app) switchDot(i int) {
	if i == a.active {
		return
	}
	a.active = clamp(i, 0, DotCount-1)
	a.findOpen, a.query, a.matchIdx = false, "", 0
	a.scheduleSave()
}

// onEdited 文本变化后统一入口。
func (a *app) onEdited() { a.scheduleSave() }
