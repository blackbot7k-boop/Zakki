package app

import (
	"strings"
	"unicode/utf8"

	"zakki/internal/markdown"
)

func lineByteOffset(doc string, line int) int { return markdown.LineByteOffset(doc, line) }

// spliceDocLines 用 insert 替换 doc 的 [startLine,endLine] 行（含端点）。
func spliceDocLines(doc string, startLine, endLine int, insert string) string {
	joined, _ := spliceLines(strings.Split(doc, "\n"), startLine, endLine, insert)
	return joined
}

// spliceDoc 同 spliceDocLines，但复用块缓存里已切好的行省一次全文 Split，
// 并把新文档的行切分暂存到 hint，供紧随其后的 mdBlocks 直接复用。
func (a *app) spliceDoc(doc string, startLine, endLine int, insert string) string {
	var lines []string
	if a.mdCacheValid && a.mdCacheDoc == doc && a.mdCacheLines != nil {
		lines = a.mdCacheLines
	} else {
		lines = strings.Split(doc, "\n")
	}
	joined, newLines := spliceLines(lines, startLine, endLine, insert)
	a.spliceHintDoc, a.spliceHintLines = joined, newLines
	return joined
}

// spliceLines 是 spliceDocLines 的行切片核心。lines 元素是字符串头，
// out 重新拼装时不改写 lines，调用方可直接传入缓存的行切片。
// 返回拼接结果与新文档的行切分（切片元素与结果共享字符串头，无额外拷贝）。
func spliceLines(lines []string, startLine, endLine int, insert string) (string, []string) {
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
	return strings.Join(out, "\n"), out
}

func (a *app) invalidateMdCache() {
	a.mdCacheValid = false
}

func (a *app) clearFenceDraft() {
	if !a.fenceDraft {
		return
	}
	a.fenceDraft = false
	a.fenceDraftLine = 0
	a.spliceHintDoc, a.spliceHintLines = "", nil
	a.invalidateMdCache()
}

func (a *app) startFenceDraft(line int) {
	a.fenceDraft = true
	a.fenceDraftLine = line
	a.invalidateMdCache()
}

// mdBlocks 返回当前标签文档的块列表（按文档串缓存，编辑处增量重切）。
func (a *app) mdBlocks() []liveBlock {
	doc := a.activeTab().Text
	if a.mdCacheValid && a.mdCacheDoc == doc {
		return a.mdCacheBlocks
	}
	var blocks []liveBlock
	var lines []string
	if a.fenceDraft {
		lines = strings.Split(doc, "\n")
		line := clamp(a.fenceDraftLine, 0, len(lines)-1)
		blocks = append(blocks, parseLines(lines, 0, line)...)
		blocks = append(blocks, parseLines(lines, line, line+1)...)
		blocks = append(blocks, parseLines(lines, line+1, len(lines))...)
	} else {
		switch {
		case a.spliceHintDoc == doc && a.spliceHintLines != nil && a.mdCacheDoc != "" && a.mdCacheLines != nil:
			// 提交管线刚拼好的新文档：行切分已由 spliceDoc 备好，直接复用。
			// 注意 applyDocEditAt 已先行 invalidate（mdCacheValid=false），
			// 但旧代缓存数据仍在，正是拼接所需的基线。
			blocks, _, _ = spliceBlocksLines(a.mdCacheDoc, a.mdCacheLines, a.mdCacheBlocks, doc, a.spliceHintLines)
			lines = a.spliceHintLines
		case a.mdCacheValid && a.mdCacheDoc != "" && a.mdCacheDoc != doc:
			blocks, lines, _ = spliceBlocks(a.mdCacheDoc, a.mdCacheLines, a.mdCacheBlocks, doc)
		default:
			lines = strings.Split(doc, "\n")
			blocks = parseLines(lines, 0, len(lines))
		}
	}
	a.spliceHintDoc, a.spliceHintLines = "", nil
	a.mdCacheDoc = doc
	a.mdCacheLines = lines
	a.mdCacheBlocks = blocks
	a.previewCache = livePreviewCache{}
	a.rebuildRuneOffsets()
	a.mdCacheValid = true
	return blocks
}

// rebuildRuneOffsets 重建块首 rune 偏移表：offsets[i] 为块 i 首行的 rune 偏移，
// widths[i] 为块 i 含换行的 rune 宽度。供 blockRuneOffset/blockOfRuneOffset 以
// 二分替代每次提交 O(全文) 的逐块扫描。
func (a *app) rebuildRuneOffsets() {
	// 当前构建可能仍持有上一代偏移表；编辑中重切不能覆写该帧的快照。
	offsets := make([]int, 0, len(a.mdCacheBlocks)+1)
	off := 0
	for _, b := range a.mdCacheBlocks {
		offsets = append(offsets, off)
		off += utf8.RuneCountInString(b.raw) + 1
	}
	a.mdCacheOffsets = append(offsets, off)
}

// blockRuneOffset 返回块 idx 在全文中的 rune 偏移（O(log n)，见 rebuildRuneOffsets）。
func (a *app) blockRuneOffset(block int) int {
	if !a.mdCacheValid {
		a.mdBlocks()
	}
	if block < 0 {
		return 0
	}
	if block >= len(a.mdCacheOffsets) {
		return a.mdCacheOffsets[len(a.mdCacheOffsets)-1]
	}
	return a.mdCacheOffsets[block]
}

// blockOfRuneOffset 用缓存的偏移表定位 rune 所在块（O(log n)）；缓存未建时退化为线性扫描。
func (a *app) blockOfRuneOffset(runeOff int) (idx, blockStart int) {
	if !a.mdCacheValid {
		return blockOfRuneOffset(a.mdBlocks(), runeOff)
	}
	i, j := 0, len(a.mdCacheBlocks)
	for i < j {
		m := int(uint(i+j) >> 1)
		if a.mdCacheOffsets[m] <= runeOff {
			i = m + 1
		} else {
			j = m
		}
	}
	idx = i - 1
	if idx < 0 {
		idx = 0
	}
	return idx, a.mdCacheOffsets[idx]
}

// spliceBlocks 增量重切：把公共前缀/后缀的边界对齐到块边界，夹在中间的完整块
// 原样复用，只重切受编辑影响的块。返回新文档的行切分（调用方直接入缓存，省一次
// 全文 Split）。正确性由 main_test.go 的随机一致性测试兼守。
func spliceBlocks(prevDoc string, prevLines []string, prevBlocks []liveBlock, newDoc string) ([]liveBlock, []string, bool) {
	return spliceBlocksLines(prevDoc, prevLines, prevBlocks, newDoc, strings.Split(newDoc, "\n"))
}

// spliceBlocksLines 同 spliceBlocks，但接受调用方已备好的新文档行切分（免一次全文 Split）。
func spliceBlocksLines(prevDoc string, prevLines []string, prevBlocks []liveBlock, newDoc string, newLines []string) ([]liveBlock, []string, bool) {
	if prevDoc == "" || len(prevLines) == 0 || len(prevBlocks) == 0 {
		return liveBlocks(newDoc), newLines, false
	}
	if strings.Count(prevDoc, "\n")+1 != len(prevLines) {
		return liveBlocks(newDoc), newLines, false
	}
	if prevDoc == newDoc {
		return prevBlocks, prevLines, true
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
		return liveBlocks(newDoc), newLines, false
	}
	preBlock := 0
	for preBlock < len(prevBlocks) && prevBlocks[preBlock].end < pre {
		preBlock++
	}
	// 前边界落在块内：回退到该块块首，整块重切。块首行在公共前缀内，
	// 新旧坐标一致，之前的块解析必然不变，可安全复用。
	if preBlock < len(prevBlocks) && prevBlocks[preBlock].start < pre {
		pre = prevBlocks[preBlock].start
	}
	// 文末追加：末块紧贴 pre 时新行可能被末块吸收（段落续行/围栏延伸），
	// 回退整块重切，避免误把旧末块当作已完整的前缀复用。
	if preBlock == len(prevBlocks) && preBlock > 0 && prevBlocks[preBlock-1].end == pre-1 {
		preBlock--
		pre = prevBlocks[preBlock].start
	}
	// 前缀接缝：前末块紧贴 pre 且会吸收首条新行（普通文本续行等）时，
	// 全量解析会把前末块与重切区合并——把 pre 对齐回前末块块首，整块重切，
	// 结果与全量一致且仍保持增量。对齐后仍吸收（极罕见）则保守全量。
	if preBlock > 0 && prefixSeamContinuation(newLines, prevBlocks[preBlock-1], pre) {
		preBlock--
		pre = prevBlocks[preBlock].start
		if preBlock > 0 && prefixSeamContinuation(newLines, prevBlocks[preBlock-1], pre) {
			return liveBlocks(newDoc), newLines, false
		}
	}
	// 前缀接缝完整性：前末块相邻时 peek 必须读重切区首行（新行坐标，stop=pre+1），
	// 否则相邻块永远判不完整，每次提交都退化为全量。
	if preBlock > 0 && !blockComplete(newLines, prevBlocks[:preBlock], preBlock-1, pre+1) {
		return liveBlocks(newDoc), newLines, false
	}
	sufBlock := len(prevBlocks)
	for sufBlock > preBlock && prevBlocks[sufBlock-1].start >= len(prevLines)-suf {
		sufBlock--
	}
	// 复用尾块必须整体落在旧文档的公共后缀里：块首与后缀起点都用旧行号比较。
	// 若用新行号 midStop 比较，增删行时新旧坐标错位，会把已改动的块当尾块复用。
	if sufBlock < len(prevBlocks) && prevBlocks[sufBlock].start < len(prevLines)-suf {
		return liveBlocks(newDoc), newLines, false
	}
	// 跨越块：被丢弃的块同时压住前缀边界与后缀起点（如删除合并段落的首行）时，
	// 前缀/后缀都无法整块覆盖它，只能全量重切。
	if sufBlock > preBlock && prevBlocks[sufBlock-1].end >= len(prevLines)-suf {
		return liveBlocks(newDoc), newLines, false
	}
	if suf > 0 && sufBlock < len(prevBlocks) && !blockComplete(prevLines, prevBlocks, sufBlock, len(prevLines)) {
		return liveBlocks(newDoc), newLines, false
	}
	mid := parseLines(newLines, pre, midStop)
	// 接缝守卫：重切区末块若会吸收首条尾行（未闭合围栏、待续表格、普通续行段落），
	// 全量解析会把两侧合并成一块，拼接结果必然多出一块，须退化为全量。
	if len(mid) > 0 && blockAbsorbsNextLine(newLines, mid[len(mid)-1], midStop) {
		return liveBlocks(newDoc), newLines, false
	}
	out := make([]liveBlock, 0, preBlock+len(mid)+(len(prevBlocks)-sufBlock))
	out = append(out, prevBlocks[:preBlock]...)
	out = append(out, mid...)
	// 常见的小块编辑（重切 ≤2 块、行数微变）在上述守卫后直接复用，
	// 不再全量解析校验；复杂编辑（大段粘贴/删行）仍全量校验兜底，
	// 正确性同时由 main_test.go 的随机一致性测试持续兼守。
	lineShift := midStop - (len(prevLines) - suf)
	if len(mid) > 2 || lineShift < -2 || lineShift > 2 {
		if !reflectDeepBlocks(out, liveBlocks(newDoc)) {
			return liveBlocks(newDoc), newLines, false
		}
	}
	if sufBlock < len(prevBlocks) {
		for _, b := range prevBlocks[sufBlock:] {
			b.start += lineShift
			b.end += lineShift
			out = append(out, b)
		}
	}
	return out, newLines, true
}

// reflectDeepBlocks 逐块比较两次解析结果是否一致（复杂编辑的全量校验用）。
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

// prefixSeamContinuation 报告前末块 pb 是否会吸收新文档第 pre 行——若是，
// 全量解析会把前缀末块与重切区合并，拼接结果必然多出一块，须退化为全量。
func prefixSeamContinuation(lines []string, pb liveBlock, pre int) bool {
	if pre >= len(lines) || pb.end+1 < pre {
		return false // 没有新行，或与新行之间有间隔行（空行等）不会合并
	}
	switch pb.kind {
	case bkCode:
		// 相邻围栏块：无论闭合与否，全量解析都可能把新行并入围栏，保守全量。
		return true
	case bkTable:
		_, ok := pipeCells(lines[pre])
		return ok
	default:
		if pb.end+1 != pre {
			return false
		}
		line := lines[pre]
		return strings.TrimSpace(line) != "" && !mdLineSpecial(line)
	}
}

// blockAbsorbsNextLine 报告块 b 在区域末尾（其末行 == next-1）是否会吸收 lines[next]：
// 会吸收则全量解析会把该块与后续内容合并，拼接结果必须退化为全量。
func blockAbsorbsNextLine(lines []string, b liveBlock, next int) bool {
	if b.end+1 != next || next >= len(lines) {
		return false
	}
	line := lines[next]
	switch b.kind {
	case bkCode:
		// 单行围栏块是未闭合的 opener，必然延续；多行块按匹配开栏判断闭合。
		if b.end == b.start {
			return true
		}
		opening, ok := parseFenceOpeningLine(lines[b.start])
		return !ok || !isFenceClosingLine(lines[b.end], opening)
	case bkTable:
		_, ok := pipeCells(line)
		return ok
	default:
		return strings.TrimSpace(line) != "" && !mdLineSpecial(line)
	}
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
		opening, ok := parseFenceOpeningLine(lines[b.start])
		if ok && b.end+1 < len(lines) && isFenceClosingLine(lines[b.end+1], opening) {
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
			line := lines[b.end+1]
			if strings.TrimSpace(line) == "" {
				return true // 空行终止一切非围栏块
			}
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

func isFenceDraftLine(raw string) bool {
	if raw == "" || strings.ContainsAny(raw, "\r\n") {
		return false
	}
	indent := 0
	for indent < len(raw) && raw[indent] == ' ' {
		indent++
		if indent > 3 {
			return false
		}
	}
	if indent == len(raw) || raw[indent] != '`' && raw[indent] != '~' {
		return false
	}
	marker := raw[indent]
	run := 1
	for indent+run < len(raw) && raw[indent+run] == marker {
		run++
	}
	if run < 3 {
		return indent+run == len(raw)
	}
	_, ok := parseFenceOpeningLine(raw)
	return ok
}

func (a *app) resyncLive(caret int) {
	a.clearFenceDraft()
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
	a.clearDocSelection()
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
	newDoc := a.spliceDoc(doc, blk.start, blk.end, draft)
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
	blockLines := strings.Split(blocks[idx].raw, "\n")
	for li := blocks[idx].start; li < line; li++ {
		off += utf8.RuneCountInString(blockLines[li-blocks[idx].start]) + 1
	}
	off += col
	a.applyDocEditAt(doc, off, true)
}

func (a *app) applyDocEditAt(doc string, docCaret int, syncSel bool) {
	a.activeTab().Text = doc
	a.lowerDoc = ""
	a.invalidateMdCache()
	blocks := a.mdBlocks()
	idx, blockOff := a.blockOfRuneOffset(docCaret)
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
	docStart, docEnd := -1, -1
	if a.fenceDraft {
		blocks := a.mdBlocks()
		if len(blocks) > 0 {
			target := clamp(idx, 0, len(blocks)-1)
			n := utf8.RuneCountInString(blocks[target].raw)
			localStart, localEnd := start, end
			if localStart < 0 {
				localStart = n
			}
			if localEnd < 0 {
				localEnd = n
			}
			off := a.blockRuneOffset(target)
			docStart = off + clamp(localStart, 0, n)
			docEnd = off + clamp(localEnd, 0, n)
		}
	}
	a.clearFenceDraft()
	a.clearDocSelection()
	blocks := a.mdBlocks()
	if len(blocks) == 0 {
		a.mdActive, a.mdDraft = -1, ""
		return
	}
	if docStart >= 0 {
		var off int
		idx, off = a.blockOfRuneOffset(docStart)
		start, end = docStart-off, docEnd-off
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
	// 虚拟列表：目标块可能不在已构建范围，请求列表把它滚入视图，
	// 下一帧焦点编辑器才会被构建并接管焦点。
	a.liveList.ScrollIntoView(idx)
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
