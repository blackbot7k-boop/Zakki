package app

import (
	"unicode/utf8"

	"github.com/egoist/mygo/ui"
)

// selectDocWide 进入文档级全选：选区覆盖活动标签的全部文本，
// 跨块的高亮与复制/剪切/删除/输入由 app 层接管（见 docSelectionInput）。
func (a *app) selectDocWide() {
	// 表格单元格内：保持单元格原生全选语义。
	if a.mdActive >= 0 && a.mdActive < len(a.mdBlocks()) && a.mdBlocks()[a.mdActive].kind == bkTable {
		a.clearDocSelection()
		p := a.mdProjection
		a.selSyncStart, a.selSyncEnd = p.sourceOffset(0), p.sourceOffset(len(p.units))
		return
	}
	a.selDoc = true
	a.selDocStart = 0
	a.selDocEnd = utf8.RuneCountInString(a.activeTab().Text)
	// 全部块统一用选区底色呈现（liveBlockRow/activeBlock 画色带）；
	// 焦点块只落光标，不再叠加原生蓝色选区，避免两种选中样式混排。
}

// menuSelectAll 菜单「全选」入口：预览态先聚焦末块，再进文档级选区。
func (a *app) menuSelectAll() {
	if a.cfg.Source {
		a.clearDocSelection()
		a.srcSelSyncStart, a.srcSelSyncEnd = 0, utf8.RuneCountInString(a.activeTab().Text)
		return
	}
	if a.mdActive < 0 {
		a.focusBlockEnd(len(a.mdBlocks()) - 1)
	}
	a.selectDocWide()
}

// collapseDocSelection 收缩文档级选区到 range 的一端，恢复普通编辑状态。
// 目标点在非焦点块时，切换焦点块到该处。
func (a *app) collapseDocSelection(toEnd bool) {
	at := a.selDocStart
	if toEnd {
		at = a.selDocEnd
	}
	a.clearDocSelection()
	idx, off := blockOfRuneOffset(a.mdBlocks(), at)
	if idx == a.mdActive {
		a.selSyncStart, a.selSyncEnd = at-off, at-off
		return
	}
	a.focusBlock(idx, at-off, at-off)
}

func (a *app) clearDocSelection() {
	a.selDoc = false
	a.selDocStart, a.selDocEnd = 0, 0
}

// docSelectionInput 在文档级选区激活时接管编辑键与编辑命令。
// 返回 true 表示事件已按文档语义消费，不进单块控件。
func (a *app) docSelectionInput(ev ui.InputEvent) bool {
	switch {
	case ev.Kind == ui.InputCommand && ev.Text == "copy",
		ev.Kind == ui.InputKeyDown && ev.Key == ui.KeyC && ev.Mods == ui.Super:
		a.writeDocClipboard()
		return true
	case ev.Kind == ui.InputCommand && ev.Text == "cut",
		ev.Kind == ui.InputKeyDown && ev.Key == ui.KeyX && ev.Mods == ui.Super:
		a.writeDocClipboard()
		a.spliceDocSelection("")
		return true
	case ev.Kind == ui.InputKeyDown && ev.Mods == 0 && ev.Key == ui.KeyEnter:
		a.spliceDocSelection("\n")
		return true
	case ev.Kind == ui.InputKeyDown && ev.Mods == 0 &&
		(ev.Key == ui.KeyBackspace || ev.Key == ui.KeyDelete):
		a.spliceDocSelection("")
		return true
	case ev.Kind == ui.InputKeyDown && ev.Mods == 0 && ev.Key == ui.KeyLeft,
		ev.Kind == ui.InputKeyDown && ev.Mods == 0 && ev.Key == ui.KeyUp:
		a.collapseDocSelection(false)
		return true
	case ev.Kind == ui.InputKeyDown && ev.Mods == 0 && ev.Key == ui.KeyRight,
		ev.Kind == ui.InputKeyDown && ev.Mods == 0 && ev.Key == ui.KeyDown:
		a.collapseDocSelection(true)
		return true
	}
	return false
}

// docSelectionReplace 把控件对整块可见文字的替换升级为文档级替换：
// 新文档就是控件新可见文字的源码映射，光标落在替换后的位置。
func (a *app) docSelectionReplace(visible string, p liveProjection, caret int) {
	// 控件的新可见文本里，真正新增的部分才是用户输入；把它映射回源码。
	oldRs, newRs := []rune(p.text), []rune(visible)
	prefix := 0
	for prefix < len(oldRs) && prefix < len(newRs) && oldRs[prefix] == newRs[prefix] {
		prefix++
	}
	suffix := 0
	for suffix < len(oldRs)-prefix && suffix < len(newRs)-caret &&
		oldRs[len(oldRs)-1-suffix] == newRs[len(newRs)-1-suffix] {
		suffix++
	}
	typed := string(newRs[prefix : len(newRs)-suffix])
	raw, at := p.replaceSelection(typed, utf8.RuneCountInString(typed), 0, len(oldRs))
	a.activeTab().Text = raw
	a.clearDocSelection()
	idx, off := blockOfRuneOffset(a.mdBlocks(), at)
	a.mdActive, a.mdDraft = idx, a.mdBlocks()[idx].raw
	a.selSyncStart, a.selSyncEnd = at-off, at-off
	a.liveFocus, a.scrollActive = 1, true
	a.pendingEditKind = editCommand
	a.onEdited()
}

// captureClipboardService 在构建期缓存窗口服务，供事件回调写剪贴板。
func (a *app) captureClipboardService(c *ui.Context) {
	if !a.clipSvcOK {
		a.clipSvc = c.Services()
		a.clipSvcOK = true
	}
}

// writeDocClipboard 把文档级选区写入剪贴板（经窗口服务，测试下走 headless）。
func (a *app) writeDocClipboard() {
	if a.clipSvcOK {
		a.clipSvc.WriteClipboard(a.docSelectionText())
	}
}

func (a *app) docSelectionText() string {
	rs := []rune(a.activeTab().Text)
	lo, hi := a.selDocStart, a.selDocEnd
	if lo > hi {
		lo, hi = hi, lo
	}
	lo = clamp(lo, 0, len(rs))
	hi = clamp(hi, 0, len(rs))
	return string(rs[lo:hi])
}

// spliceDocSelection 把文档级选区替换为 text，光标落在插入文本之后。
// 整个替换是一个撤销事务。
func (a *app) spliceDocSelection(text string) {
	lo, hi := a.selDocStart, a.selDocEnd
	if lo > hi {
		lo, hi = hi, lo
	}
	rs := []rune(a.activeTab().Text)
	lo, hi = clamp(lo, 0, len(rs)), clamp(hi, 0, len(rs))
	before := liveSnapshot{doc: a.activeTab().Text, start: lo, end: hi}
	a.activeTab().Text = string(rs[:lo]) + text + string(rs[hi:])
	after := utf8.RuneCountInString(text)
	blocks := a.mdBlocks()
	if len(blocks) == 0 {
		// 文档清空：预览态没有可聚焦块，状态归零，下次点击重建。
		a.mdActive, a.mdDraft = -1, ""
		a.clearDocSelection()
		a.onEdited()
		a.rememberLiveEdit(before, editCommand)
		return
	}
	idx, off := blockOfRuneOffset(blocks, lo+after)
	a.mdActive, a.mdDraft = idx, blocks[idx].raw
	a.clearDocSelection()
	a.selSyncStart, a.selSyncEnd = lo+after-off, lo+after-off
	a.liveFocus, a.scrollActive = 1, true
	a.onEdited()
	a.rememberLiveEdit(before, editCommand)
}

// blockSelectionBand 返回非焦点块的选区底色：文档级选区与块有交集时返回 true。
func (a *app) blockSelectionBand(blkStart, blkEnd int) bool {
	if !a.selDoc {
		return false
	}
	lo, hi := a.selDocStart, a.selDocEnd
	if lo > hi {
		lo, hi = hi, lo
	}
	return lo < blkEnd && hi > blkStart
}
