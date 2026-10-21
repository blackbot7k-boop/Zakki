package app

// 主界面：液态标签栏与 Typora 式编辑器。
//   顶栏 38pt（Windows 隐藏标题栏模式 32，与系统按钮带同高对齐），中性材质，
//           可拖动窗口：⊗ 关闭窗口 · 圆点居中（有名字时当前是色条）· 置顶；
//           右侧按 c.TitleBar() 避让 Windows 原生最小化/最大化/关闭
//   查找条（搜索图标打开）：搜索框 + 命中计数，Enter 跳到下一个
//   内容区：Typora 式所见即所得编辑（焦点块保留排版；⌘⇧P 切源码模式），
//           背景为统一中性材质
//   底栏 36pt：● 行/词/字符 · 备份时间 ··· 源码 / 搜索（Lucide 图标）

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"

	"zakki/locales"
)

// view 每帧由主线程调用，从状态构建界面。
func (a *app) view(c *ui.Context) {
	t0 := c.Theme()
	dark := themeIsDark(t0)
	// 内容区不再随标签换底色：整窗用中性主题叠在窗口级毛玻璃上；
	// 标签颜色只出现在圆点/胶囊、焦点 accent 和状态栏小标识上。
	t := *t0
	style := a.resolveStyle(-1, dark, true)
	t.Accent = a.tabAccent(a.active, dark)
	c.SetTheme(&t)

	if a.cfg.Source || a.mdActive < 0 || a.mdActive >= len(a.mdBlocks()) || a.mdBlocks()[a.mdActive].kind != bkTable {
		a.tableTools = tableTools{}
	}
	// 重命名会话优先消费 Esc；代码块语言徽标会话次之；表格浮层次之；
	// 文档级全选再次；其余时候先关链接浮层、再关查找条，最后按设置隐藏窗口。
	switch {
	case a.renaming != 0 && c.Shortcut(0, ui.KeyEscape):
		a.cancelRename()
	case a.codeLangBlock >= 0 && c.Shortcut(0, ui.KeyEscape):
		a.cancelCodeLang() // Esc 放弃语言编辑，焦点回到代码内容
		a.liveFocus = 1
		c.Invalidate()
	case a.selDoc && c.Shortcut(0, ui.KeyEscape):
		a.collapseDocSelection(false)
	case !a.tableTools.open && c.Shortcut(0, ui.KeyEscape):
		switch {
		case a.linkTool.open:
			a.closeLinkTool()
		case a.findOpen:
			a.findOpen = false
		case a.cfg.HideOnEscape && a.win != nil:
			a.win.Hide()
		}
	}
	// 窗口级快捷键兑底：与菜单同一键位表，见 shortcut_fallback.go。
	a.handleWindowShortcuts(c)

	ui.Column(c).Fill().Children(func() {
		a.header(c, &t, dark)
		if a.findOpen {
			a.findBar(c, &t, dark)
		}
		contentTheme := style.apply(t)
		c.SetTheme(&contentTheme)
		if a.linkTool.open {
			a.linkOverlay(c, &contentTheme)
		}
		if a.cfg.Source {
			a.editorPane(c, &contentTheme, dark)
		} else {
			a.livePane(c, &contentTheme, dark)
		}
		c.SetTheme(&t)
		a.statusBar(c, &t, dark)
	})
	a.syncEditorMenu(c)
	if command := a.editMenuPending; command != "" {
		a.editMenuPending = ""
		a.runEditCommand(command)
		c.Invalidate()
	}
	a.runTabMenuPending(c)
	// SetTheme repaints the root, so clear it only after the final theme change.
	if c.Vibrancy() {
		c.Root().Background(ui.Transparent)
	}
}

// contentBackground leaves the native material visible beneath a neutral layer.
// Without material, retain the theme's opaque content background.
func contentBackground(base ui.Color, dark, vibrancy bool) ui.Color {
	if !vibrancy {
		return base
	}
	if dark {
		return ui.RGBA(24, 24, 24, 0.62)
	}
	return ui.RGBA(255, 255, 255, 0.58)
}

// chromeColor 顶栏/底栏/查找条的中性半透明层：窗口有系统材质（Vibrancy）
// 时半透明叠在材质上；没有材质的平台按回退强度与主题背景混合，
// 不影响功能。
func (a *app) chromeColor(base ui.Color, dark, vibrancy bool) ui.Color {
	if vibrancy {
		if dark {
			return ui.RGBA(24, 24, 24, 0.40)
		}
		return ui.RGBA(255, 255, 255, 0.30)
	}
	bg := a.resolveStyle(-1, dark, true).background
	return base.Mix(bg, 1-a.glassStrength())
}

// ---- 顶栏（38 DIP；Windows 隐藏标题栏模式与系统按钮带同高）----

// headerPadX 顶栏左右内边距；tabChromeWidth 是顶栏两侧固定元素的宽：
// 左右内边距 + 关闭与置顶按钮（各 chromeHit），标签区可用宽度要扣掉它们。
// 内边距加按钮内留白（(chromeHit-16)/2）仍是 14，图形位置与此前一致。
const (
	headerPadX      float32 = 14 - (chromeHit-16)/2
	tabChromeWidth  float32 = 2*headerPadX + 2*chromeHit
	statusBarHeight float32 = 36
)

// headerHeight 顶栏高度：无系统按钮带（mac Frameless，TitleBar 为零值）
// 保持 38；Windows 隐藏标题栏模式降到按钮带同高（32），顶栏内容与
// 按钮 glyph 垂直同轴居中，不会比系统按钮偏下 3px。
func headerHeight(bar ui.TitleBar) float32 {
	if bar.Height > 0 {
		return max(bar.Height, 28)
	}
	return tabTrackHeight
}

func (a *app) header(c *ui.Context, t *ui.Theme, dark bool) {
	bg := a.chromeColor(headerBackground(dark), dark, c.Vibrancy())
	// 系统窗口按钮占位（Windows 在右上，mac 为零值）：右侧内边距加上按钮带宽，
	// 图钉与圆点永远排在按钮左侧；左侧取较大值，兼容日后带左侧按钮的平台。
	bar := c.TitleBar()
	padL, padR := headerPadX, headerPadX
	if bar.Height > 0 {
		padL, padR = max(headerPadX, bar.Left), headerPadX+bar.Right
	}
	ui.Row(c).Height(headerHeight(bar)).Background(bg).Padding(0, padR, 0, padL).
		BorderWidth(0, 0, 1, 0).BorderColor(t.Border).
		DragWindow().AlignItems(ui.Center).Children(func() {
		chromeIcon(c, "close-window", iconCloseWindow, locales.CloseWindow(), t.TextMuted, chromeHover(t.TextMuted), false).
			OnClick(func() {
				if a.win != nil {
					// 便签常驻：复用窗口与渲染资源，菜单关闭和退出仍真正关窗。
					a.win.Hide()
				}
			})
		// 只有一个标签（且不在重命名）时不渲染标签区：顶栏只剩关闭与置顶。
		if a.tabStripHidden() {
			a.resetTabStripLayout()
		} else {
			a.tabStrip(c, dark)
		}
		a.iconPin(c, t, dark)
	})
}

// tabStripHidden 报告顶栏是否应隐藏标签区：只有一个标签（且不在重命名）
// 时不需要圆点与胶囊指示器，顶栏只剩关闭与置顶。
func (a *app) tabStripHidden() bool { return len(a.tabs) <= 1 && a.renaming == 0 }

// resetTabStripLayout 清掉上一帧的标签几何与动画状态：隐藏期间不追踪布局，
// 恢复多标签时从头排布，不会从旧几何补间。
func (a *app) resetTabStripLayout() {
	a.morph = tabMorphState{}
	a.lastSlots = nil
	a.lastIDs = nil
	a.builtActiveID = 0
	a.tabViewportWidth = 0
}

func (a *app) tabStrip(c *ui.Context, dark bool) {
	measure := func(s string) float32 {
		w, _ := c.MeasureText(0, ui.Span{Text: s, Size: pillTitleSize})
		return w
	}
	windowW, _ := c.Size()
	// 与 header 同源的系统按钮带避让：无按钮带时 Left/Right 为零，
	// 高度也保持 38，与直调 tabStrip 的测试和历史几何完全一致。
	bar := c.TitleBar()
	h := headerHeight(bar)
	available := max(1, windowW-tabChromeWidth-bar.Left-bar.Right)
	slotTop := (h - 28) / 2
	name := a.displayTitle(a.active)
	titleW := float32(0)
	if name != "" || a.renaming != 0 {
		titleW = measuredTitleWidth(name, a.tabTitleWidthLimit(), measure)
		if a.renaming != 0 {
			titleW = max(titleW, 120)
		}
	}
	n := len(a.tabs)
	var slots []tabSlotGeom
	var total float32
	if titleW > 0 {
		slots, total, titleW = fitTabLayout(available-2*morphPadX, titleW, n, a.active)
		if a.renaming != 0 {
			titleW = max(titleW, 80)
			slots, total = layoutTabSlotsWithGap(tabSlotWidths(pillWidthOf(titleW), n, a.active), 4)
		}
	} else {
		// 端部留白：滑动回弹会越过第一枚和最后一枚圆点，没有余量时会被滚动视口裁掉。
		const edgePad = morphPadX
		gap := dotGap
		slots, total = layoutTabSlotsWithGap(equalTabWidths(n), gap)
		if available < total+2*edgePad && n > 1 {
			gap = clampF((available-2*edgePad-float32(n)*dotDiameter)/float32(n-1), 4, dotGap)
			slots, total = layoutTabSlotsWithGap(equalTabWidths(n), gap)
		}
		for i := range slots {
			slots[i].cx += edgePad
		}
		total += 2 * edgePad
	}
	contentW := max(available, total)
	if titleW > 0 {
		contentW = max(available, total+2*morphPadX)
	}
	inset := (contentW - total) / 2
	for i := range slots {
		slots[i].cx += inset
	}
	title, textW := fitTabTitle(name, titleW, measure)
	bodyH := dotDiameter
	if titleW > 0 {
		bodyH = pillHeight
	}
	body := tabBody{cx: slots[a.active].cx, w: slots[a.active].w, h: bodyH,
		color: a.tabInk(a.active, dark), title: title, textW: textW}
	ids := make([]uint64, len(a.tabs))
	for i := range ids {
		ids[i] = a.tabs[i].ID
	}
	changed := a.builtActiveID != ids[a.active] || !sameSlots(a.lastSlots, slots)
	if a.tabViewportWidth != available {
		a.morph = tabMorphState{}
		a.builtActiveID = 0
		a.tabViewportWidth = available
	}
	a.syncTabMorph(a.uiNow(c), c.Preferences().ReduceMotion, slots, ids, body)
	if a.morph.active {
		c.AnimationFrame()
	}
	ui.ScrollHorizontal(c.Key("tab-viewport")).Width(available).Height(h).
		TrackScroll(&a.tabScroll).Children(func() {
		strip := ui.Box(c.Key("tab-strip")).Width(contentW).Height(h)
		dropAt := a.tabStripDrop(strip, slots)
		mark := a.resolveStyle(-1, dark, true).accent
		strip.Draw(func(p *ui.Painter, r ui.Rect) {
			a.paintTabStrip(p, r, dark)
			if dropAt >= 0 {
				x := r.X + tabDropMarkX(slots, dropAt)
				p.Fill(ui.Rect{X: x - 1, Y: r.Y + (r.H-18)/2, W: 2, H: 18}, mark, 1)
			}
		})
		strip.Children(func() {
			for i := range a.tabs {
				a.tabSlot(c, i, slots[i], changed, slotTop, dark)
			}
		})
	})
}

// tabSlot 顶栏内一枚标签的命中区与内容；slotTop 是槽位按钮顶部，随顶栏高度变化
// （28 是槽位按钮高，圆点/胶囊始终垂直居中于顶栏，与系统按钮 glyph 同轴）。
func (a *app) tabSlot(c *ui.Context, i int, slot tabSlotGeom, reveal bool, slotTop float32, dark bool) {
	tb := a.tabs[i]
	selected := i == a.active
	renaming := a.renaming == tb.ID
	if renaming {
		// 失焦即提交：焦点曾落在输入框、现已离开，且不在输入法组合中。
		if ui.FocusedValue(c, &a.renameFocus) == 1 {
			a.renameSeenFocus = true
		} else if a.renameSeenFocus && !a.renameComposing {
			a.commitRenameOnBlur()
			renaming = false
		}
	}
	hitW := max(float32(24), slot.w)
	radius := dotDiameter / 2
	if slot.w > dotDiameter+1 {
		radius = pillHeight / 2
	}
	btn := ui.ButtonBase(c.Key(fmt.Sprintf("tab-%d", tb.ID))).Label(a.menuTitle(i)).
		Absolute().Left(slot.cx-hitW/2).Top(slotTop).
		Size(hitW, 28).Radius(radius).Cursor(ui.CursorPointer).FocusRing(true)
	if selected && reveal {
		btn.ScrollIntoView()
	}
	if renaming {
		btn.Children(func() {
			ui.Box(c).Fill().PaddingX(pillPadX).Children(func() {
				in := ui.TextInputBase(c.Key("tab-rename"), &a.renameDraft).
					Label(locales.RenameTabField()).FontSize(pillTitleSize).Fill().
					TextColor(pillTextColor(a.lastBody.color)).
					FocusBind(&a.renameFocus, 1).AutoFocus().
					HandleInput(func(ev ui.InputEvent) bool { return a.handleRenameInput(ev) })
				a.renameComposing = in.Composing()
			})
		})
		return
	}
	btn.Tooltip(a.tabTooltip(i))
	btn.Drag(tabDrag(tb.ID))
	if btn.Dragging() {
		// 跟随指针的浮层复制的是按钮本身，而标记由标签条统一绘制，按钮自身为空；
		// 拖动期间补画一枚，浮层才看得见。
		w, h := float32(dotDiameter), float32(dotDiameter)
		if slot.w > dotDiameter+1 {
			w, h = slot.w, pillHeight
		}
		btn.AlignItems(ui.Center).Justify(ui.Center).Children(func() {
			ui.Box(c).Size(w, h).Radius(h / 2).Background(a.tabInk(i, dark))
		})
	}
	btn.OnClick(func() { a.switchTab(i) })
	btn.ContextMenu(a.tabContextMenu(i))
	if selected && btn.DoubleClicked() {
		a.beginRename(i)
	}
}

// tabInk 标签圆点的墨色：关闭彩色圆点或空标签用中性色，其余用标签色。
func (a *app) tabInk(i int, dark bool) ui.Color {
	if i != a.active && strings.TrimSpace(a.tabs[i].Text) == "" {
		return emptyDotInk(dark)
	}
	return a.tabColor(i)
}

// iconPin 置顶开关：激活时用强调色并加粗。
func (a *app) iconPin(c *ui.Context, t *ui.Theme, dark bool) {
	style := a.resolveStyle(-1, dark, true)
	ink := t.TextMuted
	if a.cfg.AlwaysOnTop {
		ink = style.accent
	}
	if chromeIcon(c, "pin-top", iconPin, locales.PinTop(), ink, chromeHover(t.TextMuted), a.cfg.AlwaysOnTop).Clicked() {
		a.withWindow(func() {
			a.setAlwaysOnTop(!a.cfg.AlwaysOnTop)
		})
	}
}

// iconAction 无边框 SVG 图标按钮（Size 是 border-box，勿再加 Padding）。
func iconAction(c *ui.Context, key string, svg *ui.SVG, label string, t *ui.Theme, size float32) bool {
	return ui.Icon(c.Key(key), svg).Size(size, size).Label(label).
		TextColor(t.TextMuted).Cursor(ui.CursorPointer).Clicked()
}

// chromeHit 是顶栏、底栏与查找条图标按钮的点击区边长；图形仍是 16。
const chromeHit float32 = 24

// chromeIcon 外框图标按钮：16pt 图形居中于 chromeHit 点击区，悬停铺浅底，
// 名称同时作无障碍标签与悬停提示。
func chromeIcon(c *ui.Context, key string, svg *ui.SVG, label string, ink, hover ui.Color, bold bool) ui.Element {
	btn := ui.ButtonBase(c.Key(key)).Label(label).Tooltip(label).
		Size(chromeHit, chromeHit).Radius(6).Cursor(ui.CursorPointer)
	if btn.Hovered() {
		btn.Background(hover)
	}
	btn.Children(func() {
		g := ui.Icon(c, svg).Size(16, 16).TextColor(ink)
		if bold {
			g.Bold()
		}
	})
	return btn
}

// chromeHover 图标悬停底色：取图标墨色的淡色，深浅主题与标签色下都可见。
func chromeHover(ink ui.Color) ui.Color { return withAlpha(ink, 0.14) }

// ---- 编辑器（源码模式） ----

// editorPane 源码模式：全文源码 + 语法高亮（⌘⇧P 或底栏眼睛图标切换）。
// 实时预览是默认模式；源码模式保留作为备选（批量调整、粘贴大段文本时更顺手）。
// 源码编辑纳入与实时预览相同的文档级撤销事务（含格式命令）。
func (a *app) editorPane(c *ui.Context, t *ui.Theme, dark bool) {
	if a.srcSelSyncStart < 0 && a.srcCaret == 0 && a.srcSelEnd == 0 && a.srcPrev.doc == "" {
		a.srcPrev = a.liveSnapshot() // 首帧初始化撤销基线
	}
	text := &a.activeTab().Text
	matches := a.docMatches(*text, a.query)
	md := a.resolveStyle(-1, themeIsDark(t), true)
	accent := md.accent

	// TextAreaBase 无自带样式：背景/内边距自己给，聚焦时不会有任何边框。
	a.captureClipboardService(c)
	ta := ui.TextAreaBase(c.Key(fmt.Sprintf("source-editor-%d", a.activeTab().ID)), text).
		Bind(&a.editorHandle).
		ContextMenu(a.editorContextMenu).
		Label(locales.EditorContent()).
		Fill().Grow(1).
		Padding(14, 20).
		FontSize(md.bodySize).Font(md.monoFont).LineHeight(md.lineHeight).
		FocusRing(false).
		Background(a.paperBackground(md.background, dark, c.Vibrancy())).
		AutoFocus().
		OnPaste(func(text string) bool {
			if text != "" {
				a.nextEditIsPaste = true
			}
			return false
		}).
		HandleInput(func(ev ui.InputEvent) bool { return a.recordSourceKey(ev) })

	ta.TextRanges(a.sourceRanges(*text, a.highlightQuery(), t, accent)...)
	wasComposing := a.srcComposing
	a.srcComposing = ta.Composing()
	if !wasComposing && a.srcComposing {
		a.srcComposeBefore = a.srcPrev
	}

	// 查找跳转：把选区移到当前命中处（编辑器会滚动到选区）。
	if a.jumpPending {
		a.jumpPending = false
		if len(matches) > 0 {
			a.matchIdx = clamp(a.matchIdx, 0, len(matches)-1)
			m := matches[a.matchIdx]
			a.srcSelSyncStart, a.srcSelSyncEnd = int(m.Start), int(m.End)
		}
	}
	// 强制选区（撤销恢复/查找跳转后）。
	if a.srcSelSyncStart >= 0 {
		s, e := a.srcSelSyncStart, a.srcSelSyncEnd
		a.srcSelSyncStart, a.srcSelSyncEnd = -1, -1
		ta.SetTextSelection(s, max(e, s))
	}

	cs, ce := ta.TextSelection()
	a.srcCaret, a.srcSelEnd = cs, ce

	if a.liveUndoPending != 0 {
		redo := a.liveUndoPending < 0
		a.liveUndoPending = 0
		a.undoLive(redo)
		c.Invalidate()
		return
	}
	if ta.Changed() && !a.srcComposing {
		before := a.srcPrev
		kind := a.consumePendingEditKind()
		if a.nextEditIsPaste {
			kind, a.nextEditIsPaste = editPaste, false
		} else if wasComposing {
			kind, before = editIME, a.srcComposeBefore
		}
		a.rememberLiveEdit(before, kind)
		a.onEdited()
	}
	// 组合文本只存在原生控件缓冲中；确认后再推进撤销基线。
	if !a.srcComposing {
		a.srcPrev = a.liveSnapshot()
		a.srcComposeBefore = liveSnapshot{}
	}
}

// ---- 底栏（36pt，随点的 status 色，Tot 同布局）----

func (a *app) statusBar(c *ui.Context, t *ui.Theme, dark bool) {
	style := a.resolveStyle(-1, dark, true)
	bg := a.chromeColor(style.status, dark, c.Vibrancy())
	fg := a.tabAccent(a.active, dark)
	// 窄窗口里文字单行截断（悬停看全文），图标固定不被挤出。
	ui.Row(c).Height(statusBarHeight).Background(bg).
		Padding(0, headerPadX, 0, 14).Gap(8).AlignItems(ui.Center).Children(func() {
		// 标签色小标识：不随标签换底色，只留这一点颜色。
		ui.Box(c).Width(8).Height(8).Radius(999).Background(a.tabColor(a.active)).Shrink(0)
		lines, words, chars := a.docStats(a.activeTab().Text)
		stats := locales.StatsLinesWordsChars(lines, words, chars)
		ui.Text(c, stats).FontSize(12).TextColor(fg).SingleLine().MinWidth(0).Tooltip(stats)
		ui.Box(c).Width(1).Height(16).Background(fg.Mix(bg, 0.55)).Shrink(0)
		backup := a.backupLabel()
		ui.Text(c, backup).FontSize(12).TextColor(fg).SingleLine().MinWidth(0).Tooltip(backup)
		// 保存/载入异常要在界面上可见（完整内容在日志里）；它比统计更重要，最后才截断。
		if warn := a.statusWarn(); warn != "" {
			ui.Text(c, warn).FontSize(12).TextColor(fg).Bold().SingleLine().MinWidth(0).Shrink(0.2).Tooltip(warn)
		}
		ui.Box(c).Grow(1)

		ui.Row(c).Shrink(0).AlignItems(ui.Center).Children(func() {
			// 源码模式开关（Lucide eye）：默认实时预览，打开后全文源码 + 语法高亮。
			a.iconStatusToggle(c, "sb-source", iconPreview, locales.StatusSource(), a.cfg.Source, t, dark, func() {
				a.toggleSourceMode()
			})
			// 搜索开关（Lucide search）
			a.iconStatusToggle(c, "sb-find", iconSearch, locales.StatusSearch(), a.findOpen, t, dark, func() {
				a.findOpen = !a.findOpen
				if a.findOpen {
					a.query, a.matchIdx, a.findFocus = "", 0, 1
				}
			})
		})
	})
}

// iconStatusToggle 底栏图标：用状态栏前景色；激活加粗提亮。
func (a *app) iconStatusToggle(c *ui.Context, key string, svg *ui.SVG, label string, on bool, t *ui.Theme, dark bool, fn func()) {
	style := a.resolveStyle(-1, dark, true)
	fg := style.accent
	ink := fg
	if !on {
		ink = fg.Mix(style.status, 0.30)
	}
	if chromeIcon(c, key, svg, label, ink, chromeHover(fg), on).Clicked() {
		fn()
	}
}

// ---- 状态流转 ----

// saveSourceSession 必须在 active 改变前调用；restoreSourceSession 在改变后调用。
// 会话只记光标与选区：撤销历史只属于当前这段编辑，切换标签即清空，不按标签保留。
func (a *app) saveSourceSession() {
	if !a.cfg.Source || len(a.tabs) == 0 {
		return
	}
	if a.srcSessions == nil {
		a.srcSessions = make(map[uint64]sourceSession)
	}
	a.srcSessions[a.activeTab().ID] = sourceSession{
		caret: a.srcCaret, selEnd: a.srcSelEnd,
		selSyncStart: a.srcSelSyncStart, selSyncEnd: a.srcSelSyncEnd,
		prev: a.srcPrev,
	}
}

func (a *app) restoreSourceSession() {
	a.srcCaret, a.srcSelEnd = 0, 0
	a.srcSelSyncStart, a.srcSelSyncEnd = -1, -1
	a.srcPrev, a.srcComposeBefore = liveSnapshot{}, liveSnapshot{}
	a.srcComposing = false
	if session, ok := a.srcSessions[a.activeTab().ID]; ok {
		a.srcCaret, a.srcSelEnd = session.caret, session.selEnd
		a.srcSelSyncStart, a.srcSelSyncEnd = session.selSyncStart, session.selSyncEnd
		a.srcPrev = session.prev
	}
}

// toggleSourceMode 切换源码/实时预览（编辑会话状态，随笔记持久化）。
func (a *app) toggleSourceMode() {
	if a.cfg.Source {
		a.saveSourceSession()
	} else {
		a.commitDraft(-1)
	}
	a.clearFenceDraft()
	a.cfg.Source = !a.cfg.Source
	if a.cfg.Source {
		a.restoreSourceSession()
		a.srcPrev = a.liveSnapshot()
	} else {
		a.resyncLive(-1)
	}
	a.syncMenus()
	a.scheduleSave()
}

// onEdited 文本变化后统一入口。
func (a *app) onEdited() { a.scheduleSave() }

// sourceRanges 缓存源码模式高亮区间：视图每帧重建，避免长文档每帧全文重算。
func (a *app) sourceRanges(text, query string, t *ui.Theme, accent ui.Color) []ui.TextRange {
	dark := themeIsDark(t)
	codeID := a.prefs.Appearance.CodeTheme
	if a.rangesCacheDoc == text && a.rangesCacheQuery == query &&
		a.rangesCacheDark == dark && a.rangesCacheAccent == accent && a.rangesCacheCode == codeID {
		return a.rangesCache
	}
	cp, _ := codePaletteFor(codeID, t)
	rs := editorRanges(text, query, t, accent, cp)
	a.rangesCacheDoc, a.rangesCacheQuery = text, query
	a.rangesCacheDark, a.rangesCacheAccent, a.rangesCacheCode = dark, accent, codeID
	a.rangesCache = rs
	return rs
}
