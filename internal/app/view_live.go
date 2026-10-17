package app

import (
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/egoist/mygo/ui"

	"zakki/internal/codehl"
	"zakki/locales"
)

// ---- 实时预览（Typora 式） ----

// livePane 把文档按块渲染：焦点块保留排版结构并只编辑可见文字，其余块即时排版；
// 点击渲染块切换焦点块（live_edit.go 负责状态与光标映射）。
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
			idx, off := a.blockOfRuneOffset(m.Start)
			if idx != a.mdActive {
				a.focusBlock(idx, m.Start-off, m.End-off)
			} else {
				a.selSyncStart, a.selSyncEnd = m.Start-off, m.End-off
			}
		}
	}

	style := a.resolveStyle(-1, dark, true)
	// 虚拟化列表：只构建视口内外的少数块，长文档的首帧与稳态帧成本与
	// 可见块数相关，而与文档总块数无关（编辑器仍按块工作，见 live_edit.go）。
	blockStarts := make([]int, len(blocks))
	for i := 1; i < len(blocks); i++ {
		blockStarts[i] = blockStarts[i-1] + utf8.RuneCountInString(blocks[i-1].raw) + 1
	}
	// 快照 active：块内 Enter 可能在本轮构建中把 mdActive 挤到别的块，
	// 同一帧不能因此误建第二个编辑器（同 handle 冲突），重建交给下一帧。
	active := a.mdActive
	// 外层 Column 兜底空白点击：List 自身不报告左右留白与行间空隙的点击。
	wrap := ui.Column(c).Fill().Grow(1)
	// List 不报告左右留白与行间空隙的点击；外层 Column 兜底为「空白点击」。
	// 内层行/输入框的点击会先置 liveClickHandled（同为 OnClick 回调，按构建
	// 顺序先执行），空白点击据此让路。
	wrap.OnClick(func() {
		if !a.liveClickHandled {
			a.focusBlockEnd(len(a.mdBlocks()) - 1)
		}
	})
	wrap.Children(func() {
		list := ui.List(c, &a.liveList, len(blocks), func(i int) {
			blk := blocks[i]
			if i == active {
				a.activeBlock(c, t)
				return
			}
			start := blockStarts[i]
			blockEnd := start + utf8.RuneCountInString(blk.raw)
			a.liveBlockRow(c, t, dark, i, blk, isLiveSeparator(blocks, i), start, blockEnd+1)
		})
		list.Fill().Background(a.paperBackground(style.background, dark, c.Vibrancy())).
			TrackScroll(&a.liveScroll).
			Padding(style.padX, style.padY).Gap(style.blockGap).
			Font(style.bodyFont).FontSize(style.bodySize).LineHeight(style.lineHeight)
		if style.sketch {
			// 纸面画在滚动视口上而不是内容行上：内容比视口短时，下方空白同样是纸。
			dots := withAlpha(style.textMuted, 0.22)
			margin := withAlpha(style.accent, 0.38)
			marginX := style.padX * 0.45
			list.Draw(func(p *ui.Painter, r ui.Rect) {
				// 绘制时读最新滚动量：滚动可能只重绘不重建，点阵仍与文字同步。
				dotGrid(p, r, a.liveScroll.Y, dots)
				sketchLine(p, r.X+marginX, r.Y, r.X+marginX, r.Y+r.H, 1.2, margin, 3)
			})
		}
	})
	// 点击内容区空白也是编辑意图：见 wrap.OnClick；内层文字、输入框和表格
	// 已处理的点击保留自己的光标，不被重定向。
	// 只消费一次点击标记，避免影响下一次视口点击。
	a.liveClickHandled = false
}

// activeBlock keeps the block's rendered structure while editing visible contents.
func (a *app) activeBlock(c *ui.Context, t *ui.Theme) {
	blk := a.mdBlocks()[a.mdActive]
	projection := projectBlock(blk)
	if a.fenceDraft && isFenceDraftLine(blk.raw) {
		projection = projectRegion(blk.raw, 0, utf8.RuneCountInString(blk.raw), false, false)
		// 回车确认前保持原生输入框层级不变，避免重挂到代码卡片时重复输入或错位光标。
		blk.kind = bkPara
	}
	md := a.resolveStyle(-1, themeIsDark(t), true)
	row := ui.Row(c.Key("blk-" + strconv.Itoa(a.mdActive))).Gap(8).FillWidth()
	// 文档级全选：焦点块与其余块呈现一致的选区底色。
	if a.selDoc {
		row.Background(t.Selection)
	}
	row.Children(func() {
		switch blk.kind {
		case bkTable:
			a.editTable(c, t, blk)
		case bkTask:
			a.taskCheckbox(c, blk, md)
			a.activeText(c, t, blk, projection)
		case bkList:
			listMarker(c, md, blk)
			a.activeText(c, t, blk, projection)
		case bkQuote:
			quoteBar(c, md, uint32(blk.start))
			a.activeText(c, t, blk, projection)
		case bkCode:
			codeCard(c, md, t, uint32(blk.start)).FillWidth().Children(func() {
				a.activeText(c, t, blk, projection)
			})
		case bkRule:
			ui.Column(c).FillWidth().Children(func() {
				ruleLine(c, md, uint32(blk.start))
				a.activeText(c, t, blk, projection)
			})
		default:
			a.activeText(c, t, blk, projection)
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

	a.captureClipboardService(c)
	ta := ui.TextAreaBase(c.Key("md-active"), &a.mdVisible).
		Bind(&a.editorHandle).ContextMenu(a.editorContextMenu).Label(locales.EditorContent())
	if a.selDoc {
		// 文档级全选：焦点块的输入框铺选区底色，避免自绘背景盖住色带。
		ta = ta.Background(t.Selection)
	}
	ta = ta.
		Lines(1, 4096).
		FillWidth().Grow(1).MinWidth(0).
		Font(md.bodyFont).FontSize(md.bodySize).LineHeight(md.lineHeight).
		FocusRing(false)
	if !a.findOpen && !a.linkTool.open {
		ta = ta.AutoFocus()
	}
	ta = ta.
		OnPaste(func(text string) bool {
			if text != "" {
				a.nextEditIsPaste = true
			}
			return false
		}).
		HandleInput(func(ev ui.InputEvent) bool { return a.recordProjectedKey(ev, p) })
	// 文本框是滚动容器的子控件；标记已处理，避免父容器把同一次点击
	// 当作空白点击，从而将光标重置到文末。
	if ta.Clicked() {
		a.liveClickHandled = true
	}
	if a.liveFocus > 0 {
		a.liveFocus = 0
		a.editorHandle.Focus(c)
	}
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
		if a.selDoc {
			// 文档级选区：输入文本不可在事件层拦截（走 NSTextInputClient），
			// 在这里把控件对整块可见文字的替换升级为对整个文档的替换。
			a.docSelectionReplace(a.mdVisible, p, cs)
			c.Invalidate()
			return
		}
		// 原生编辑应把光标推进到新内容之后，但 AX 整值替换会保留旧光标；
		// 纯追加而光标仍停在前缀内时先吸附到末尾，否则下一个字符会插进中间。
		if cs == ce {
			if n := nativeCaretAfterEdit(p.text, a.mdVisible, cs); n != cs {
				cs, ce = n, n
			}
		}
		// 退化的未闭合围栏（整块只有一行围栏，如刚键入的 ```）：可编辑区域
		// 是围栏行末尾的零宽区间，直接插入会改写围栏行（变成语言标识）。
		// 换用带合成内容行的投影，让输入落到围栏之后。
		if p.start == p.end && p.end == utf8.RuneCountInString(p.raw) && p.end > 0 {
			if i := strings.LastIndexByte(p.raw, '\n'); isFenceOpeningLine(p.raw[i+1:]) {
				p = projectRegion(p.raw+"\n", p.end+1, p.end+1, false, false)
			}
		}
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
		startFence := !a.fenceDraft && kind == editInput && strings.Trim(p.raw, " ") == "" && isFenceDraftLine(raw)
		clearFence := a.fenceDraft && !isFenceDraftLine(raw)
		if startFence {
			a.startFenceDraft(blk.start)
		}
		a.mdDraft = raw
		a.commitDraft(caret)
		if clearFence {
			a.clearFenceDraft()
		}
		if kind == editPaste {
			a.scrollActive = true
			a.liveList.ScrollIntoView(a.mdActive)
		}
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
		if a.activeTab().Text != beforeDoc {
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
func (a *app) liveBlockRow(c *ui.Context, t *ui.Theme, dark bool, i int, blk liveBlock, separator bool, docStart, docEnd int) {
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
	// 文档级全选：非焦点块画选区底色，提示它在选区内。
	if a.blockSelectionBand(docStart, docEnd) {
		row.Background(t.Selection)
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
	switch blk.kind {
	case bkTable:
		a.tableGrid(c, t, md, blk, false)
	case bkHeading:
		if md.sketch {
			a.sketchHeading(c, t, md, blk)
			return
		}
		h := projectBlock(blk).content(c, t, md).TextColor(t.Text)
		styleHeading(h, blk.level, md)
	case bkRule:
		ruleLine(c, md, uint32(blk.start))
	case bkQuote:
		quoteBar(c, md, uint32(blk.start))
		ui.RichText(c).Grow(1).TextColor(t.TextMuted).Children(func() {
			renderInline(c, t, md, blk.text, blk.spans)
		})
	case bkCode:
		codeCard(c, md, t, uint32(blk.start)).Fill().Children(func() {
			cp, codeOn := codePaletteFor(a.prefs.Appearance.CodeTheme, t)
			lang := codehl.Lang(blk.info)
			lines := blk.lines
			if len(lines) == 0 {
				lines = []string{" "}
			}
			for _, l := range lines {
				if l == "" {
					l = " "
				}
				if codeOn {
					ui.RichText(c, codeLineSpans(l, codehl.Line(lang, l), cp, ui.Color{})...).
						Font(md.monoFont).FontSize(md.codeSize).LineHeight(md.codeLineHeigh)
				} else {
					ui.Text(c, l).Font(md.monoFont).FontSize(md.codeSize).LineHeight(md.codeLineHeigh)
				}
			}
		})
	case bkList:
		listMarker(c, md, blk)
		inlineContent(c, t, md, blk.text, blk.spans).Grow(1)
	case bkTask:
		a.taskCheckbox(c, blk, md)
		projectBlock(blk).content(c, t, md).Grow(1)
	default:
		projectBlock(blk).content(c, t, md).Grow(1)
	}
}

// inlineContent 输出段落文字：无行内标记时用普通 Text（可被命中测试/测试
// 工具找到），有标记时用 RichText 混排。spans 用切分时预计算的缓存。
func inlineContent(c *ui.Context, t *ui.Theme, md resolvedStyle, text string, spans []mSpan) ui.Element {
	if len(spans) == 0 {
		return ui.Text(c, text)
	}
	rt := ui.RichText(c)
	rt.Children(func() { renderInline(c, t, md, text, spans) })
	return rt
}

// renderInline 在当前 RichText 段落内输出带样式的行内片段。
// spans 是切分时预计算的行内标记（勿传 nil 渲染带标记的文本）。
func renderInline(c *ui.Context, t *ui.Theme, md resolvedStyle, text string, spans []mSpan) {
	accent := md.accent
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
			if md.sketch {
				styled.TextBackground(sketchMark(md))
			}
		case spanItalic:
			styled.Italic()
		case spanCode:
			styled.Font("monospace").TextColor(t.TextMuted)
			if md.sketch {
				styled.TextBackground(md.codeBackground)
			}
		case spanLink:
			styled.TextColor(accent)
			if md.sketch {
				styled.WavyUnderline().DecorationColor(accent)
			} else {
				styled.Underline()
			}
		}
		prev = sp.end
	}
	if prev < len(rs) {
		ui.Text(c, string(rs[prev:]))
	}
}
