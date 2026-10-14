package main

import (
	"strings"
	"unicode/utf8"

	"github.com/egoist/mygo/ui"

	"tot-md-mygo/internal/markdown"
)

func lineByteOffset(doc string, line int) int { return markdown.LineByteOffset(doc, line) }

// spliceDocLines 用 insert 替换 doc 的 [startLine,endLine] 行（含端点）。
func spliceDocLines(doc string, startLine, endLine int, insert string) string {
	lines := strings.Split(doc, "\n")
	if startLine < 0 {
		startLine = 0
	}
	if endLine >= len(lines) {
		endLine = len(lines) - 1
	}
	if startLine > endLine {
		endLine = startLine - 1
	}
	ins := strings.Split(insert, "\n")
	// 必须落到新切片：lines[:startLine] 与 lines 共享底层数组，
	// 直接 append 插入行会先改写尾部，把 endLine 之后的原文冲掉。
	out := make([]string, 0, startLine+len(ins)+len(lines)-endLine-1)
	out = append(out, lines[:startLine]...)
	out = append(out, ins...)
	out = append(out, lines[endLine+1:]...)
	return strings.Join(out, "\n")
}

func (a *app) invalidateMdCache() {
	a.mdCacheValid = false
}

// mdBlocks 返回当前标签文档的块列表（按文档串缓存）。
func (a *app) mdBlocks() []liveBlock {
	doc := a.activeTab().Text
	if a.mdCacheValid && a.mdCacheDoc == doc {
		return a.mdCacheBlocks
	}
	prevDoc, prevLines, prevBlocks := a.mdCacheDoc, a.mdCacheLines, a.mdCacheBlocks
	var blocks []liveBlock
	if a.mdCacheValid && prevDoc != "" && prevDoc != doc {
		blocks, _ = spliceBlocks(prevDoc, prevLines, prevBlocks, doc)
	} else {
		blocks = liveBlocks(doc)
	}
	a.mdCacheDoc = doc
	a.mdCacheLines = strings.Split(doc, "\n")
	a.mdCacheBlocks = blocks
	a.mdCacheValid = true
	return blocks
}

// spliceBlocks 增量重切：在公共前缀/后缀行上复用旧块，中间区域 parseLines。
func spliceBlocks(prevDoc string, prevLines []string, prevBlocks []liveBlock, newDoc string) ([]liveBlock, bool) {
	newLines := strings.Split(newDoc, "\n")
	if prevDoc == "" || len(prevLines) == 0 || len(prevBlocks) == 0 {
		return liveBlocks(newDoc), false
	}
	if strings.Count(prevDoc, "\n")+1 != len(prevLines) {
		return liveBlocks(newDoc), false
	}
	if prevDoc == newDoc {
		return prevBlocks, true
	}
	pre := 0
	for pre < len(prevLines) && pre < len(newLines) && prevLines[pre] == newLines[pre] {
		pre++
	}
	suf := 0
	for suf < len(prevLines)-pre && suf < len(newLines)-pre {
		if prevLines[len(prevLines)-1-suf] != newLines[len(newLines)-1-suf] {
			break
		}
		suf++
	}
	midStop := len(newLines) - suf
	if pre > midStop {
		return liveBlocks(newDoc), false
	}
	preBlock := 0
	for preBlock < len(prevBlocks) && prevBlocks[preBlock].end < pre {
		preBlock++
	}
	if preBlock < len(prevBlocks) && prevBlocks[preBlock].start < pre {
		return liveBlocks(newDoc), false
	}
	if preBlock > 0 && !blockComplete(prevLines, prevBlocks, preBlock-1, pre) {
		return liveBlocks(newDoc), false
	}
	sufBlock := len(prevBlocks)
	for sufBlock > preBlock && prevBlocks[sufBlock-1].start >= midStop {
		sufBlock--
	}
	if sufBlock < len(prevBlocks) && prevBlocks[sufBlock].start < midStop {
		return liveBlocks(newDoc), false
	}
	if suf > 0 && sufBlock < len(prevBlocks) && !blockComplete(newLines, prevBlocks, sufBlock, len(newLines)) {
		return liveBlocks(newDoc), false
	}
	mid := parseLines(newLines, pre, midStop)
	out := make([]liveBlock, 0, preBlock+len(mid)+(len(prevBlocks)-sufBlock))
	out = append(out, prevBlocks[:preBlock]...)
	out = append(out, mid...)
	if sufBlock < len(prevBlocks) {
		lineShift := midStop - (len(prevLines) - suf)
		for _, b := range prevBlocks[sufBlock:] {
			b.start += lineShift
			b.end += lineShift
			out = append(out, b)
		}
	}
	if !reflectDeepBlocks(out, liveBlocks(newDoc)) {
		return liveBlocks(newDoc), false
	}
	return out, true
}

func reflectDeepBlocks(a, b []liveBlock) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !sameBlock(a[i], b[i]) {
			return false
		}
	}
	return true
}

// blockComplete 报告 lines[0:stop) 内 blocks[i] 是否已解析完整（未截断）。
func blockComplete(lines []string, blocks []liveBlock, i int, stop int) bool {
	if i < 0 || i >= len(blocks) {
		return true
	}
	b := blocks[i]
	if b.end+1 < stop {
		return true
	}
	switch b.kind {
	case bkCode:
		if b.end+1 < len(lines) && isFenceLine(lines[b.end+1]) {
			return true
		}
		return false
	case bkTable:
		if b.end+1 < stop {
			_, ok := pipeCells(lines[b.end+1])
			return !ok
		}
		return false
	default:
		if b.end+1 < stop {
			line := strings.TrimSpace(lines[b.end+1])
			if mdLineSpecial(line) {
				return true
			}
			if _, ok := tableStart(lines, b.end+1, stop); ok {
				return false
			}
		}
		return b.end < stop-1 || stop >= len(lines)
	}
}

func (a *app) resyncLive(caret int) {
	blocks := a.mdBlocks()
	if len(blocks) == 0 {
		a.mdActive, a.mdDraft = -1, ""
		return
	}
	if a.mdActive < 0 {
		return
	}
	if a.mdActive >= len(blocks) {
		a.mdActive = len(blocks) - 1
	}
	a.mdDraft = blocks[a.mdActive].raw
	if caret >= 0 {
		a.selSyncStart, a.selSyncEnd = caret, caret
		a.liveFocus = 1
	}
}

func (a *app) ensureDraftSynced() {
	blocks := a.mdBlocks()
	if a.mdActive < 0 {
		return
	}
	if len(blocks) == 0 {
		a.mdActive = -1
		a.mdDraft = ""
		return
	}
	if a.mdActive >= len(blocks) {
		a.mdActive = len(blocks) - 1
	}
	want := blocks[a.mdActive].raw
	if a.mdDraft != want {
		a.mdDraft = want
	}
}

func (a *app) commitDraft(caret int) {
	a.commitDraftSel(caret, caret >= 0 && a.selSyncStart >= 0 && a.selSyncEnd >= 0)
}

// publishDraft 提交草稿并强制同步选区（结构化编辑：Enter、合块等）。
func (a *app) publishDraft(caret int) {
	a.commitDraftSel(caret, caret >= 0)
}

func (a *app) commitDraftSel(caret int, syncSel bool) {
	if a.mdActive < 0 {
		return
	}
	blocks := a.mdBlocks()
	if len(blocks) == 0 {
		return
	}
	a.mdActive = clamp(a.mdActive, 0, len(blocks)-1)
	blk := blocks[a.mdActive]
	draft := a.mdDraft
	doc := a.activeTab().Text
	newDoc := spliceDocLines(doc, blk.start, blk.end, draft)
	if newDoc == doc && draft == blk.raw {
		return
	}
	if caret < 0 {
		if a.selSyncStart >= 0 {
			caret = a.selSyncStart
		} else {
			caret = a.mdCaret
		}
	}
	docCaret := a.blockRuneOffset(a.mdActive) + caret
	a.applyDocEditAt(newDoc, docCaret, syncSel)
}

func (a *app) blockRuneOffset(block int) int {
	blocks := a.mdBlocks()
	off := 0
	for i := 0; i < block && i < len(blocks); i++ {
		off += utf8.RuneCountInString(blocks[i].raw) + 1
	}
	return off
}

func (a *app) applyDocEdit(doc string, line, col int) {
	blocks := a.mdBlocks()
	idx := 0
	for i, b := range blocks {
		if line >= b.start && line <= b.end {
			idx = i
			break
		}
		if b.start <= line {
			idx = i
		}
	}
	off := a.blockRuneOffset(idx)
	for li := blocks[idx].start; li < line; li++ {
		off += utf8.RuneCountInString(strings.Split(blocks[idx].raw, "\n")[li-blocks[idx].start]) + 1
	}
	off += col
	a.applyDocEditAt(doc, off, true)
}

func (a *app) applyDocEditAt(doc string, docCaret int, syncSel bool) {
	a.activeTab().Text = doc
	a.lowerDoc = ""
	a.invalidateMdCache()
	blocks := a.mdBlocks()
	idx, blockOff := blockOfRuneOffset(blocks, docCaret)
	a.mdActive = idx
	a.mdDraft = blocks[idx].raw
	caret := docCaret - blockOff
	if caret < 0 {
		caret = 0
	}
	if caret > utf8.RuneCountInString(a.mdDraft) {
		caret = utf8.RuneCountInString(a.mdDraft)
	}
	if syncSel {
		a.selSyncStart, a.selSyncEnd = caret, caret
	}
	a.mdCaret, a.mdSelEnd = caret, caret
	a.liveFocus = 1
}

func (a *app) focusBlock(idx int, start, end int) {
	if a.mdActive >= 0 {
		blocks := a.mdBlocks()
		if a.mdActive < len(blocks) && a.mdDraft != blocks[a.mdActive].raw {
			a.commitDraft(-1)
		}
	}
	blocks := a.mdBlocks()
	if len(blocks) == 0 {
		a.mdActive, a.mdDraft = -1, ""
		return
	}
	idx = clamp(idx, 0, len(blocks)-1)
	a.mdActive = idx
	a.mdDraft = blocks[idx].raw
	n := utf8.RuneCountInString(a.mdDraft)
	if start < 0 {
		start = n
	}
	if end < 0 {
		end = n
	}
	start, end = clamp(start, 0, n), clamp(end, 0, n)
	a.selSyncStart, a.selSyncEnd = start, end
	a.mdCaret, a.mdSelEnd = start, end
	a.liveFocus = 1
	a.scrollActive = true
	a.maybeExpirePending()
}

func (a *app) focusBlockEnd(idx int) {
	blocks := a.mdBlocks()
	if len(blocks) == 0 {
		a.focusBlock(0, 0, 0)
		return
	}
	idx = len(blocks) - 1
	for idx > 0 && idx == len(blocks)-1 && blocks[idx].kind == bkBlank && blocks[idx].raw == "" {
		idx--
	}
	n := utf8.RuneCountInString(blocks[idx].raw)
	a.focusBlock(idx, n, n)
}

func (a *app) recordKeyHint(ev ui.InputEvent) bool {
	if a.mdComposing {
		return false
	}
	if ev.Kind != ui.InputKeyDown || ev.Mods != 0 {
		return false
	}
	if a.mdSelEnd > a.mdCaret {
		return false
	}
	blocks := a.mdBlocks()
	if a.mdActive < 0 || a.mdActive >= len(blocks) {
		return false
	}
	switch ev.Key {
	case ui.KeyBackspace:
		if a.mdCaret == 0 && a.mdSelEnd <= a.mdCaret {
			a.keyHint = ui.KeyBackspace
			return true
		}
	case ui.KeyEnter:
		a.keyHint = ui.KeyEnter
		return true
	case ui.KeyUp, ui.KeyDown, ui.KeyLeft, ui.KeyRight:
		p := projectBlock(blocks[a.mdActive])
		vis := p.visibleOffset(a.mdCaret)
		n := len(p.units)
		atStart := vis == 0
		atEnd := vis == n
		switch ev.Key {
		case ui.KeyUp, ui.KeyLeft:
			if atStart {
				a.keyHint = ev.Key
				return true
			}
		case ui.KeyDown, ui.KeyRight:
			if atEnd {
				a.keyHint = ev.Key
				return true
			}
		}
	}
	return false
}

func (a *app) applyKeyHint(key ui.Key, start, end int) {
	if key == ui.KeyTab {
		backward := a.tableBackward
		a.tableBackward = false
		if a.mdActive >= 0 {
			blocks := a.mdBlocks()
			if a.mdActive < len(blocks) && blocks[a.mdActive].kind == bkTable {
				a.navigateTable(backward, start)
			}
		}
		return
	}
	if key == ui.KeyEnter {
		a.afterEnter(start, end)
		return
	}
	if key == ui.KeyBackspace {
		if start == 0 && end == 0 {
			a.backspaceAtBlockStart()
		}
		return
	}
	if start != end {
		return
	}
	blocks := a.mdBlocks()
	if a.mdActive < 0 || a.mdActive >= len(blocks) {
		return
	}
	n := utf8.RuneCountInString(a.mdDraft)
	switch key {
	case ui.KeyUp, ui.KeyLeft:
		if start == 0 {
			a.crossBlock(-1)
		}
	case ui.KeyDown, ui.KeyRight:
		if start == n {
			a.crossBlock(1)
		}
	}
}

func (a *app) afterEnter(start, end int) {
	blocks := a.mdBlocks()
	if a.mdActive < 0 || a.mdActive >= len(blocks) {
		return
	}
	blk := blocks[a.mdActive]
	if a.createTableOnEnter(start, end, blk) {
		return
	}
	if blk.kind == bkCode {
		// 代码块内单换行。
		if start == end {
			raw := a.mdDraft
			b := runeToByte(raw, start)
			a.mdDraft = raw[:b] + "\n" + raw[b:]
			a.publishDraft(start + 1)
		}
		return
	}
	if blk.kind == bkTable {
		a.tableEnter(start)
		return
	}
	lo, hi := start, end
	if hi < lo {
		hi = lo
	}
	if hi > lo {
		raw := a.mdDraft
		ls, le := runeToByte(raw, lo), runeToByte(raw, hi)
		a.mdDraft = raw[:ls] + raw[le:]
		a.publishDraft(lo)
		blocks = a.mdBlocks()
		if a.mdActive >= len(blocks) {
			return
		}
		blk = blocks[a.mdActive]
		hi = lo
	}
	line := strings.TrimLeft(a.mdDraft, " \t")
	switch blk.kind {
	case bkList, bkTask:
		if bare := bareListMarker(line); bare != "" && lo == hi &&
			(lo == 0 || lo == utf8.RuneCountInString(a.mdDraft)) {
			lines := strings.Split(a.activeTab().Text, "\n")
			if blk.start < len(lines) {
				lines[blk.start] = strings.TrimPrefix(lines[blk.start], bare)
				a.applyDocEditAt(strings.Join(lines, "\n"), a.blockRuneOffset(blk.start), true)
				return
			}
		}
		if lo == 0 && hi == 0 {
			m := listContinuePrefix(blk)
			a.mdDraft = m + "\n" + a.mdDraft
			a.publishDraft(utf8.RuneCountInString(m))
			return
		}
		left := a.mdDraft[:runeToByte(a.mdDraft, lo)]
		right := a.mdDraft[runeToByte(a.mdDraft, hi):]
		prefix := listContinuePrefix(blk)
		a.mdDraft = left + "\n" + prefix + right
		caret := utf8.RuneCountInString(left) + 1 + utf8.RuneCountInString(prefix)
		a.publishDraft(caret)
		return
	case bkBlank:
		if lo == 0 && hi == 0 && strings.TrimSpace(a.mdDraft) == "" {
			lines := strings.Split(a.activeTab().Text, "\n")
			at := blk.end + 1
			if at > len(lines) {
				at = len(lines)
			}
			newLines := append(append(lines[:at], ""), lines[at:]...)
			a.applyDocEditAt(strings.Join(newLines, "\n"), a.blockRuneOffset(a.mdActive), true)
			return
		}
	case bkPara, bkHeading:
		if lo == 0 && hi == 0 {
			if blk.kind == bkPara && strings.TrimSpace(a.mdDraft) == "" {
				lines := strings.Split(a.activeTab().Text, "\n")
				at := blk.end + 1
				if at > len(lines) {
					at = len(lines)
				}
				newLines := append(append(lines[:at], ""), lines[at:]...)
				a.applyDocEditAt(strings.Join(newLines, "\n"), a.blockRuneOffset(a.mdActive), true)
				return
			}
			a.mdDraft = "\n\n" + a.mdDraft
			a.publishDraft(2)
			return
		}
	}
	left := a.mdDraft[:runeToByte(a.mdDraft, lo)]
	right := a.mdDraft[runeToByte(a.mdDraft, hi):]
	a.mdDraft = left + "\n\n" + right
	a.publishDraft(utf8.RuneCountInString(left) + 2)
}

func listMarkerPrefix(b liveBlock) string {
	t := strings.TrimSpace(b.raw)
	switch b.kind {
	case bkList:
		if n := listMarkerLen(t); n > 0 {
			return t[:n]
		}
	case bkTask:
		if n := taskPrefixLen(t); n > 0 {
			return t[:n]
		}
	}
	return ""
}

func (a *app) crossBlock(dir int) {
	blocks := a.mdBlocks()
	i := a.mdActive + dir
	for i >= 0 && i < len(blocks) && blocks[i].kind == bkBlank && isLiveSeparator(blocks, i) {
		i += dir
	}
	if i < 0 || i >= len(blocks) {
		return
	}
	n := utf8.RuneCountInString(blocks[i].raw)
	if dir < 0 {
		a.focusBlock(i, n, n)
	} else {
		a.focusBlock(i, 0, 0)
	}
}

func (a *app) blockFromLineCol(line, col int) {
	blocks := a.mdBlocks()
	idx := 0
	for i, b := range blocks {
		if line >= b.start && line <= b.end {
			idx = i
			break
		}
		if b.start <= line {
			idx = i
		}
	}
	blk := blocks[idx]
	caret := col
	if line > blk.start {
		lines := strings.Split(blk.raw, "\n")
		for li := blk.start; li < line && li-blk.start < len(lines); li++ {
			caret += utf8.RuneCountInString(lines[li-blk.start]) + 1
		}
	}
	a.focusBlock(idx, caret, caret)
}

func (a *app) backspaceAtBlockStart() {
	blocks := a.mdBlocks()
	i := a.mdActive
	if i < 0 || i >= len(blocks) {
		return
	}
	cur := blocks[i]
	if cur.kind == bkBlank {
		lo, hi := blankRun(blocks, i)
		run := hi - lo + 1
		// 两段正文之间多出来的 \n\n：一次退格去掉整段分段，留回原来的一个空行。
		if hi+1 < len(blocks) && run >= 3 {
			a.deleteBlankBreak(blocks[lo].start)
			return
		}
		// 文末连续空段一次只删一行；只剩分隔行加光标空段时并回上一段。
		if hi+1 >= len(blocks) && run > 2 {
			a.deleteDocLine(cur.start)
			return
		}
	}
	if m := bareListMarker(strings.TrimLeft(cur.raw, " \t")); m != "" {
		lines := strings.Split(a.activeTab().Text, "\n")
		if cur.start < len(lines) {
			lines[cur.start] = strings.TrimPrefix(lines[cur.start], m)
			a.applyDocEditAt(strings.Join(lines, "\n"), a.blockRuneOffset(cur.start), true)
		}
		return
	}
	prevIdx := i - 1
	for prevIdx >= 0 && blocks[prevIdx].kind == bkBlank && isLiveSeparator(blocks, prevIdx) {
		prevIdx--
	}
	if prevIdx < 0 {
		return
	}
	prev := blocks[prevIdx]
	if prev.kind == bkCode || cur.kind == bkCode {
		return
	}
	joined := joinBlockRaw(prev, cur)
	newDoc := spliceDocLines(a.activeTab().Text, prev.start, cur.end, joined)
	caret := joinCaret(prev, cur)
	a.applyDocEditAt(newDoc, a.blockRuneOffset(prevIdx)+caret, true)
}

// blurLive 退出块编辑，回到全块预览态。
func (a *app) blurLive() {
	a.commitDraft(-1)
	a.mdActive = -1
	a.mdDraft = ""
	a.mdVisible = ""
	a.mdProjection = liveProjection{}
	a.selSyncStart, a.selSyncEnd = -1, -1
	a.keyHint = 0
}

func blankRun(blocks []liveBlock, i int) (lo, hi int) {
	lo, hi = i, i
	for lo > 0 && blocks[lo-1].kind == bkBlank {
		lo--
	}
	for hi+1 < len(blocks) && blocks[hi+1].kind == bkBlank {
		hi++
	}
	return lo, hi
}

// deleteBlankBreak 删掉从 line 起的两行空行，光标落在上一段末尾。
func (a *app) deleteBlankBreak(line int) {
	lines := strings.Split(a.activeTab().Text, "\n")
	if line < 0 || line+1 >= len(lines) {
		return
	}
	kept := make([]string, 0, len(lines)-2)
	kept = append(kept, lines[:line]...)
	kept = append(kept, lines[line+2:]...)
	newDoc := strings.Join(kept, "\n")
	caret := 0
	if line > 0 {
		for i := 0; i < line-1; i++ {
			caret += utf8.RuneCountInString(kept[i]) + 1
		}
		caret += utf8.RuneCountInString(kept[line-1])
	}
	a.applyDocEditAt(newDoc, caret, true)
}

func (a *app) deleteDocLine(line int) {
	lines := strings.Split(a.activeTab().Text, "\n")
	if line < 0 || line >= len(lines) {
		return
	}
	kept := make([]string, 0, len(lines)-1)
	kept = append(kept, lines[:line]...)
	kept = append(kept, lines[line+1:]...)
	newDoc := strings.Join(kept, "\n")
	caret := 0
	if line >= len(kept) {
		caret = utf8.RuneCountInString(newDoc)
	} else {
		for i := 0; i < line; i++ {
			caret += utf8.RuneCountInString(kept[i]) + 1
		}
	}
	a.applyDocEditAt(newDoc, caret, true)
}

func joinBlockRaw(prev, cur liveBlock) string {
	switch {
	case prev.kind == bkPara && cur.kind == bkPara:
		return prev.raw + cur.raw
	case prev.kind == bkList && cur.kind == bkList:
		m := listMarkerPrefix(cur)
		body := strings.TrimSpace(strings.TrimPrefix(cur.raw, m))
		return prev.raw + "\n" + body
	default:
		return prev.raw + cur.raw
	}
}

func joinCaret(prev, cur liveBlock) int {
	switch {
	case prev.kind == bkPara && cur.kind == bkPara:
		return utf8.RuneCountInString(prev.raw)
	case prev.kind == bkList && cur.kind == bkList:
		return utf8.RuneCountInString(prev.raw + "\n")
	default:
		return utf8.RuneCountInString(prev.raw)
	}
}
