package main

// 主界面：液态标签栏与 Typora 式编辑器。
//   顶栏 38pt，中性材质，可拖动窗口：⊗ 清空 · 动态实心标签居中 · 置顶
//   查找条（搜索图标打开）：搜索框 + 命中计数，Enter 跳到下一个
//   内容区：Typora 式所见即所得编辑（焦点块保留排版；⌘⇧P 切源码模式），
//           背景为统一中性材质
//   底栏 36pt：● 行/词/字符 · 备份时间 ··· 源码 / 搜索（Lucide 图标）

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/egoist/mygo/ui"
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
	// 重命名会话优先消费 Esc；表格浮层次之；其余时候先关链接浮层、
	// 再关查找条，最后按设置隐藏窗口。
	switch {
	case a.renaming != 0 && c.Shortcut(0, ui.KeyEscape):
		a.cancelRename()
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
	// 标签快捷键（有菜单时 accelerator 在系统层先消费；这里兜底无菜单环境）。
	a.handleTabShortcuts(c)

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
		return ui.RGBA(24, 24, 24, 0.46)
	}
	return ui.RGBA(255, 255, 255, 0.40)
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

// ---- 顶栏（38 DIP）----

func (a *app) header(c *ui.Context, t *ui.Theme, dark bool) {
	bg := a.chromeColor(headerBackground(dark), dark, c.Vibrancy())
	ui.Row(c).Height(tabTrackHeight).Background(bg).PaddingX(14).
		BorderWidth(0, 0, 1, 0).BorderColor(t.Border).
		DragWindow().AlignItems(ui.Center).Children(func() {
		if iconAction(c, "clear-dot", iconClearDot, "清空当前标签", t, 16) {
			a.clearActiveTab()
		}
		a.tabStrip(c, dark)
		a.iconPin(c, t, dark)
	})
}

func (a *app) tabStrip(c *ui.Context, dark bool) {
	measure := func(s string) float32 {
		w, _ := c.MeasureText(0, ui.Span{Text: s, Size: pillTitleSize})
		return w
	}
	windowW, _ := c.Size()
	available := max(1, windowW-60)
	name := a.displayTitle(a.active)
	titleW := measuredTitleWidth(name, a.tabTitleWidthLimit(), measure)
	if a.renaming != 0 {
		titleW = max(titleW, 120)
	}
	slots, total, titleW := fitTabLayout(available-2*morphPadX, titleW, len(a.tabs), a.active)
	if a.renaming != 0 {
		// Editing needs a usable field even when the strip has to scroll.
		titleW = max(titleW, 80)
		slots, total = layoutTabSlotsWithGap(tabSlotWidths(pillWidthOf(titleW), len(a.tabs), a.active), 4)
	}
	contentW := max(available, total+2*morphPadX)
	inset := (contentW - total) / 2
	for i := range slots {
		slots[i].cx += inset
	}
	title, textW := fitTabTitle(name, titleW, measure)
	body := tabBody{cx: slots[a.active].cx, w: slots[a.active].w, h: pillHeight,
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
	ui.ScrollHorizontal(c.Key("tab-viewport")).Width(available).Height(tabTrackHeight).
		TrackScroll(&a.tabScroll).Children(func() {
		strip := ui.Box(c.Key("tab-strip")).Width(contentW).Height(tabTrackHeight)
		strip.Draw(func(p *ui.Painter, r ui.Rect) { a.paintTabStrip(p, r, dark) })
		strip.Children(func() {
			for i := range a.tabs {
				a.tabSlot(c, i, slots[i], changed)
			}
		})
	})
}

func (a *app) tabSlot(c *ui.Context, i int, slot tabSlotGeom, reveal bool) {
	tb := a.tabs[i]
	selected := i == a.active
	renaming := a.renaming == tb.ID
	hitW := max(24, slot.w)
	btn := ui.ButtonBase(c.Key(fmt.Sprintf("tab-%d", tb.ID))).Label(a.menuTitle(i)).
		Absolute().Left(slot.cx-hitW/2).Top((tabTrackHeight-28)/2).
		Size(hitW, 28).Radius(pillHeight / 2).Cursor(ui.CursorPointer).FocusRing(true)
	if selected && reveal {
		btn.ScrollIntoView()
	}
	if renaming {
		btn.Children(func() {
			ui.Box(c).Fill().PaddingX(pillPadX).Children(func() {
				in := ui.TextInputBase(c.Key("tab-rename"), &a.renameDraft).
					Label("重命名标签").FontSize(pillTitleSize).Fill().
					TextColor(pillTextColor(a.lastBody.color)).
					FocusBind(&a.renameFocus, 1).AutoFocus().
					HandleInput(func(ev ui.InputEvent) bool { return a.handleRenameInput(ev) })
				a.renameComposing = in.Composing()
			})
		})
	} else {
		btn.OnClick(func() { a.switchTab(i) })
		if selected && btn.DoubleClicked() {
			a.beginRename(i)
		}
	}
}

// tabInk 标签圆点的墨色：关闭彩色圆点或空标签用中性色，其余用标签色。
func (a *app) tabInk(i int, dark bool) ui.Color {
	if a.cfg.MonoDots {
		return monoInk(dark)
	}
	if i != a.active && strings.TrimSpace(a.tabs[i].Text) == "" {
		return emptyDotInk(dark)
	}
	return a.tabColor(i)
}

// handleTabShortcuts 标签栏快捷键（窗口作用域兜底，有菜单时由
// accelerator 先消费）：⌘1–⌘9 切换、⌘T 新增、⌘R 重命名、⌘⌥←/→ 循环。
func (a *app) handleTabShortcuts(c *ui.Context) {
	for k := ui.Key1; k <= ui.Key9; k++ {
		n := int(k - ui.Key1)
		if n < len(a.tabs) && c.Shortcut(ui.Super, k) {
			a.switchTab(n)
			return
		}
	}
	switch {
	case c.Shortcut(ui.Super, ui.KeyT):
		a.addTab()
	case c.Shortcut(ui.Super, ui.KeyR):
		a.beginRename(a.active)
	case c.Shortcut(ui.Super|ui.Alt, ui.KeyLeft):
		a.cycleTab(-1)
	case c.Shortcut(ui.Super|ui.Alt, ui.KeyRight):
		a.cycleTab(1)
	}
}

// iconPin 置顶开关：激活时用强调色并加粗。
func (a *app) iconPin(c *ui.Context, t *ui.Theme, dark bool) {
	style := a.resolveStyle(-1, dark, true)
	g := ui.Icon(c.Key("pin-top"), iconPin).Size(16, 16).Label("置顶").Cursor(ui.CursorPointer)
	if a.cfg.AlwaysOnTop {
		g.Bold().TextColor(style.accent)
	} else {
		g.TextColor(t.TextMuted)
	}
	if g.Clicked() {
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

// ---- 查找条 ----

func (a *app) findBar(c *ui.Context, t *ui.Theme, dark bool) {
	ui.Row(c).Height(44).Background(a.chromeColor(headerBackground(dark), dark, c.Vibrancy())).
		Padding(0, 14).Gap(8).AlignItems(ui.Center).
		BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
		sf := ui.SearchField(c.Key("find-query"), &a.query).Grow(1)
		if sf.Changed() {
			a.matchIdx = 0
			// 输入查询本身不强制定位：预览态不该因为打关键词就把命中行变成源码。
			a.jumpPending = false
		}
		// Enter 跳到下一个命中。
		if sf.Submitted() {
			if ms := a.docMatches(a.activeTab().Text, a.query); len(ms) > 0 {
				a.matchIdx = (a.matchIdx + 1) % len(ms)
				a.jumpPending = true
			}
		}
		matches := a.docMatches(a.activeTab().Text, a.query)
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
	ta := ui.TextAreaBase(c, text).
		Label("内容").
		Fill().Grow(1).
		Padding(14, 20).
		FontSize(md.bodySize).Font(md.monoFont).LineHeight(md.lineHeight).
		FocusRing(false).
		Background(contentBackground(md.background, dark, c.Vibrancy())).
		AutoFocus().
		HandleInput(func(ev ui.InputEvent) bool { return a.recordSourceKey(ev) })

	ta.TextRanges(editorRanges(*text, a.query, t, accent)...)

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

	if ta.Changed() {
		kind := a.consumePendingEditKind()
		a.rememberLiveEdit(a.srcPrev, kind)
		a.onEdited()
	}
	// 记录本帧结束时的快照，作为下一帧编辑的“before”。
	a.srcPrev = a.liveSnapshot()
}

// ---- 实时预览（Typora 式） ----

// livePane 把文档按块渲染：焦点块保留排版结构并只编辑可见文字，其余块即时排版；
// 点击渲染块切换焦点块（liveedit.go 负责状态与光标映射）。
func (a *app) livePane(c *ui.Context, t *ui.Theme, dark bool) {
	a.ensureDraftSynced()
	doc := a.activeTab().Text
	blocks := a.mdBlocks()

	// 搜索跳转：把命中所在块切为焦点块，选区停在命中处。
	if a.jumpPending {
		a.jumpPending = false
		if ms := a.docMatches(doc, a.query); len(ms) > 0 {
			a.matchIdx = clamp(a.matchIdx, 0, len(ms)-1)
			m := ms[a.matchIdx]
			idx, off := blockOfRuneOffset(blocks, m.Start)
			if idx != a.mdActive {
				a.focusBlock(idx, m.Start-off, m.End-off)
			} else {
				a.selSyncStart, a.selSyncEnd = m.Start-off, m.End-off
			}
		}
	}

	style := a.resolveStyle(-1, dark, true)
	pane := ui.Scroll(c).Grow(1).Fill().Background(contentBackground(style.background, dark, c.Vibrancy())).Children(func() {
		ui.Column(c).Fill().Padding(style.padX, style.padY).Gap(style.blockGap).
			Font(style.bodyFont).FontSize(style.bodySize).LineHeight(style.lineHeight).Children(func() {
			// 预览态（mdActive=-1）全块渲染；点击内容区后编辑可见文字，
			// 聚焦不会把当前行切成 Markdown 源码。
			// 快照 active：块内 Enter 可能在本轮构建中把 mdActive 挧到别的块，
			// 同一路循环不能因此误建第二个编辑器（同 key 冲突），重建交给下一帧。
			active := a.mdActive
			for i, blk := range blocks {
				if i == active {
					a.activeBlock(c, t)
				} else {
					a.liveBlockRow(c, t, dark, i, blk, isLiveSeparator(blocks, i))
				}
			}
		})
	})
	// 点击内容区空白也是编辑意图：空点从头输入，有内容的点在文末继续。
	// 内层文字、输入框和表格已处理的点击保留自己的光标，不被重定向。
	if pane.Clicked() && !a.liveClickHandled {
		a.focusBlockEnd(len(a.mdBlocks()) - 1)
	}
	// 只消费一次点击标记，避免影响下一次视口点击。
	a.liveClickHandled = false
}

// activeBlock keeps the block's rendered structure while editing visible contents.
func (a *app) activeBlock(c *ui.Context, t *ui.Theme) {
	blk := a.mdBlocks()[a.mdActive]
	md := a.resolveStyle(-1, themeIsDark(t), true)
	ui.Row(c.Key("blk-" + strconv.Itoa(a.mdActive))).Gap(8).FillWidth().Children(func() {
		switch blk.kind {
		case bkTable:
			a.editTable(c, t, blk)
		case bkTask:
			a.taskCheckbox(c, blk)
			a.activeText(c, t, blk, projectBlock(blk))
		case bkList:
			label := "•"
			if blk.n > 0 {
				label = fmt.Sprintf("%d.", blk.n)
			}
			ui.Text(c, label).TextColor(md.accent)
			a.activeText(c, t, blk, projectBlock(blk))
		case bkQuote:
			ui.Box(c).Width(3).FillHeight().Radius(2).Background(md.quoteBorder)
			a.activeText(c, t, blk, projectBlock(blk))
		case bkCode:
			ui.Column(c).FillWidth().Padding(md.codePadX, md.codePadY).Radius(8).Background(t.Surface).Children(func() {
				a.activeText(c, t, blk, projectBlock(blk))
			})
		case bkRule:
			ui.Column(c).FillWidth().Children(func() {
				ui.Box(c).Height(1).FillWidth().Background(md.rule)
				a.activeText(c, t, blk, projectBlock(blk))
			})
		default:
			a.activeText(c, t, blk, projectBlock(blk))
		}
	})
}

func (a *app) activeText(c *ui.Context, t *ui.Theme, blk liveBlock, p liveProjection) {
	md := a.resolveStyle(-1, themeIsDark(t), true)
	if a.mdProjection.raw != p.raw || a.mdProjection.start != p.start || a.mdProjection.end != p.end || a.mdProjection.cell != p.cell {
		a.mdVisible = p.text
	}
	a.mdProjection = p
	beforeEdit := a.liveSnapshot()
	oldStart, oldEnd := p.visibleOffset(a.mdCaret), p.visibleOffset(a.mdSelEnd)
	if a.selSyncStart >= 0 {
		oldStart, oldEnd = p.visibleOffset(a.selSyncStart), p.visibleOffset(a.selSyncEnd)
	}

	ta := ui.TextAreaBase(c.Key("md-active"), &a.mdVisible).
		Label("内容").
		Lines(1, 4096).
		FillWidth().Grow(1).MinWidth(0).
		Font(md.bodyFont).FontSize(md.bodySize).LineHeight(md.lineHeight).
		FocusRing(false).
		AutoFocus().
		HandleInput(func(ev ui.InputEvent) bool { return a.recordProjectedKey(ev, p) })
	// 文本框是滚动容器的子控件；标记已处理，避免父容器把同一次点击
	// 当作空白点击，从而将光标重置到文末。
	if ta.Clicked() {
		a.liveClickHandled = true
	}
	ta.FocusBind(&a.liveFocus, 1)
	switch blk.kind {
	case bkHeading:
		styleHeading(ta, blk.level, md)
	case bkQuote:
		ta.TextColor(t.TextMuted)
	case bkCode:
		ta.Font(md.monoFont).FontSize(md.codeSize).LineHeight(md.codeLineHeigh)
	case bkTable:
		ta.FontSize(md.codeSize)
		if len(blk.table.rows) > 0 && p.start < tableCellPositions(blk.raw)[len(blk.table.rows[0])-1].end {
			ta.Bold()
		}
	}
	if len(p.styles) == 1 && p.styles[0].start == 0 && p.styles[0].end == len(p.units) && p.styles[0].kind == spanItalic {
		ta.Italic()
	}
	ta.TextRanges(p.textRanges(a.query, t, md.accent)...)

	// Native selections are visible offsets; the existing document pipeline
	// continues to use source offsets. IME composition stays with the widget.
	cs, ce := ta.TextSelection()
	wasComposing := a.prevComposing
	a.mdComposing = ta.Composing()
	a.prevComposing = a.mdComposing
	a.mdCaret, a.mdSelEnd = p.sourceOffset(cs), p.sourceOffset(ce)
	// 光标移动/换块后重新按当前位置计算待输入格式（意图清零）。
	if a.selSyncStart < 0 {
		a.maybeExpirePending()
	}

	if a.liveUndoPending != 0 {
		redo := a.liveUndoPending < 0
		a.liveUndoPending = 0
		a.undoLive(redo)
		c.Invalidate()
		return
	}
	defer func() { a.rememberLiveEdit(beforeEdit, a.takePendingEditKind()) }()

	if ta.Changed() && !a.mdComposing {
		raw, caret := p.replaceSelection(a.mdVisible, cs, oldStart, oldEnd)
		kind := editInput
		if a.nextEditIsPaste {
			kind, a.nextEditIsPaste = editPaste, false
		} else if wasComposing && !a.mdComposing {
			kind = editIME // IME 一次确认作为一个事务
		}
		a.pendingEditKind = kind
		// 待输入格式：第一次输入/粘贴/IME 确认时生成有效 Markdown。
		if pre, post := a.pendingWrapFor(p, oldStart, oldEnd, blk, caret); pre != "" || post != "" {
			raw, caret = applyMarkerWrap(raw, p.sourceOffset(oldStart), caret, pre, post)
			a.mdPending, a.lastPendingCaret, a.pendingBlock = nil, 0, -1
			a.pendingEditKind = editCommand
		}
		a.mdDraft = raw
		a.commitDraft(caret)
		if a.selSyncStart < 0 {
			a.selSyncStart, a.selSyncEnd = caret, caret
		}
		a.onEdited()
		c.Invalidate()
		return
	}

	// 自动建表是一次可撤销的输入转换。
	if a.tableUndoPending {
		a.tableUndoPending = false
		if a.activeTab().Text == a.autoTableAfter {
			a.applyDocEdit(a.autoTableBefore, a.autoTableLine, a.autoTableCol)
			a.autoTableAfter = ""
			a.onEdited()
			c.Invalidate()
			return
		}
	}

	// 上一帧记录的边界键：在已发布的光标上验证执行。
	if a.keyHint != 0 {
		hint := a.keyHint
		a.keyHint = 0
		before, beforeDoc, beforeActive := a.mdDraft, a.activeTab().Text, a.mdActive
		a.applyProjectedHint(hint, p, cs, ce)
		if hint == ui.KeyEnter && a.activeTab().Text != beforeDoc {
			a.onEdited()
		}
		// 表格编辑改变草稿后照常走重建流程（无早退返回）。
		if hint == ui.KeyEnter && a.mdDraft != before {
			c.Invalidate()
			return
		}
		if a.mdDraft != before || a.activeTab().Text != beforeDoc || a.mdActive != beforeActive ||
			(p.cell && a.selSyncStart >= 0 && (a.selSyncStart < p.start || a.selSyncStart > p.end)) {
			// 草稿/文档已改写：本轮的块列表与编辑器缓冲已过期，
			// 选区留到重建后再应用（构造时 setText 会重新同步缓冲）。
			c.Invalidate()
			return
		}
	}

	// 强制选区（切块/合块/搜索跳转后）。
	if a.selSyncStart >= 0 {
		s, e := a.selSyncStart, a.selSyncEnd
		a.selSyncStart, a.selSyncEnd = -1, -1
		if !a.mdComposing {
			ta.SetTextSelection(p.visibleOffset(s), p.visibleOffset(max(e, s)))
			vs, ve := ta.TextSelection()
			a.mdCaret, a.mdSelEnd = p.sourceOffset(vs), p.sourceOffset(ve)
		} else {
			a.selSyncStart, a.selSyncEnd = s, e
		}
	}

	if a.scrollActive {
		a.scrollActive = false
		ta.ScrollIntoView()
	}
}

// liveBlockRow 渲染一个非焦点块；整行可点击进入编辑。
// 勾选框是更内层的可点元素，它的点击不会冒泡到行（不会误切换焦点块）。
func (a *app) liveBlockRow(c *ui.Context, t *ui.Theme, dark bool, i int, blk liveBlock, separator bool) {
	var row ui.Element
	if blk.kind == bkBlank {
		// 源码分隔符不占行高；额外的空段落可见、可点击，且一次退格只删一个。
		height := float32(20)
		if separator {
			height = 0
		}
		row = ui.Row(c.Key("blk-" + strconv.Itoa(i))).FillWidth().Height(height)
	} else {
		row = ui.Row(c.Key("blk-" + strconv.Itoa(i))).Gap(8).FillWidth()
		row.Children(func() { a.renderLiveBlock(c, t, dark, blk) })
	}
	row.OnClick(func() {
		a.liveClickHandled = true
		n := utf8.RuneCountInString(blk.raw)
		a.focusBlock(i, n, n)
	})
}

// renderLiveBlock 输出一个非焦点块的排版内容（列表现有 Row 内）。
func (a *app) renderLiveBlock(c *ui.Context, t *ui.Theme, dark bool, blk liveBlock) {
	md := a.resolveStyle(-1, dark, true)
	accent := md.accent
	switch blk.kind {
	case bkTable:
		a.tableGrid(c, t, md, blk, false)
	case bkHeading:
		h := projectBlock(blk).content(c, t, accent).TextColor(t.Text)
		styleHeading(h, blk.level, md)
	case bkRule:
		ui.Box(c).Height(1).FillWidth().Background(md.rule)
	case bkQuote:
		ui.Box(c).Width(3).FillHeight().Radius(2).Background(md.quoteBorder)
		ui.RichText(c).Grow(1).TextColor(t.TextMuted).Children(func() {
			renderInline(c, t, accent, blk.text, blk.spans)
		})
	case bkCode:
		ui.Column(c).Fill().Padding(md.codePadX, md.codePadY).Radius(8).Background(t.Surface).Children(func() {
			lines := blk.lines
			if len(lines) == 0 {
				lines = []string{" "}
			}
			for _, l := range lines {
				if l == "" {
					l = " "
				}
				ui.Text(c, l).Font(md.monoFont).FontSize(md.codeSize).LineHeight(md.codeLineHeigh)
			}
		})
	case bkList:
		label := "•"
		if blk.n > 0 {
			label = fmt.Sprintf("%d.", blk.n)
		}
		ui.Text(c, label).TextColor(accent)
		inlineContent(c, t, accent, blk.text, blk.spans).Grow(1)
	case bkTask:
		a.taskCheckbox(c, blk)
		projectBlock(blk).content(c, t, accent).Grow(1)
	default:
		projectBlock(blk).content(c, t, accent).Grow(1)
	}
}

// inlineContent 输出段落文字：无行内标记时用普通 Text（可被命中测试/测试
// 工具找到），有标记时用 RichText 混排。spans 用切分时预计算的缓存。
func inlineContent(c *ui.Context, t *ui.Theme, accent ui.Color, text string, spans []mSpan) ui.Element {
	if len(spans) == 0 {
		return ui.Text(c, text)
	}
	rt := ui.RichText(c)
	rt.Children(func() { renderInline(c, t, accent, text, spans) })
	return rt
}

// renderInline 在当前 RichText 段落内输出带样式的行内片段。
// spans 是切分时预计算的行内标记（勿传 nil 渲染带标记的文本）。
func renderInline(c *ui.Context, t *ui.Theme, accent ui.Color, text string, spans []mSpan) {
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
	style := a.resolveStyle(-1, dark, true)
	bg := a.chromeColor(style.status, dark, c.Vibrancy())
	fg := a.tabAccent(a.active, dark)
	ui.Row(c).Height(36).Background(bg).
		Padding(0, 14).Gap(8).AlignItems(ui.Center).Children(func() {
		// 标签色小标识：不随标签换底色，只留这一点颜色。
		dotInk := a.tabColor(a.active)
		if a.cfg.MonoDots {
			dotInk = monoInk(dark)
		}
		ui.Box(c).Width(8).Height(8).Radius(999).Background(dotInk)
		ui.Textf(c, "%d 行 · %d 词 · %d 字符",
			countLines(a.activeTab().Text), countWords(a.activeTab().Text), countChars(a.activeTab().Text)).
			FontSize(12).TextColor(fg)
		ui.Box(c).Width(1).Height(16).Background(fg.Mix(bg, 0.55))
		ui.Text(c, a.backupLabel()).FontSize(12).TextColor(fg)
		// 保存/载入异常要在界面上可见（完整内容在日志里）。
		if warn := a.statusWarn(); warn != "" {
			ui.Text(c, warn).FontSize(12).TextColor(fg).Bold()
		}
		ui.Box(c).Grow(1)

		// 源码模式开关（Lucide eye）：默认实时预览，打开后全文源码 + 语法高亮。
		a.iconStatusToggle(c, "sb-source", iconPreview, "源码", a.cfg.Source, t, dark, func() {
			a.toggleSourceMode()
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

// iconStatusToggle 底栏图标：用状态栏前景色；激活加粗提亮。
func (a *app) iconStatusToggle(c *ui.Context, key string, svg *ui.SVG, label string, on bool, t *ui.Theme, dark bool, fn func()) {
	style := a.resolveStyle(-1, dark, true)
	fg := style.accent
	ix := ui.Icon(c.Key(key), svg).Size(16, 16).Label(label).Cursor(ui.CursorPointer)
	if on {
		ix.Bold().TextColor(fg)
	} else {
		ix.TextColor(fg.Mix(style.status, 0.30))
	}
	if ix.Clicked() {
		fn()
	}
}

// ---- 状态流转 ----

// toggleSourceMode 切换源码/实时预览（编辑会话状态，随笔记持久化）。
func (a *app) toggleSourceMode() {
	a.commitDraft(-1)
	a.cfg.Source = !a.cfg.Source
	if a.cfg.Source {
		a.srcPrev = a.liveSnapshot()
		a.srcSelSyncStart, a.srcSelSyncEnd = -1, -1
	} else {
		a.resyncLive(-1)
	}
	a.syncMenus()
	a.scheduleSave()
}

// onEdited 文本变化后统一入口。
func (a *app) onEdited() { a.scheduleSave() }
