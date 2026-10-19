package app

import (
	"strings"
	"unicode/utf8"

	"github.com/egoist/mygo/ui"
)

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

// applyCodeTab 处理代码块内的 Tab / Shift+Tab：无选区在光标处插入制表符，
// 同行选区按普通输入替换为制表符，跨行选区整行增减一级缩进（制表符或至多
// 四个空格）并保留选区以便连续调整。退化围栏（未确认的单行围栏、零宽可编辑
// 区）没有内容行，改为在围栏后新开内容行落缩进，避免写进围栏行的语言标识。
// 按键已在输入层消费，焦点不会跳出编辑器。
func (a *app) applyCodeTab(shift bool, p liveProjection, start, end int) {
	if a.fenceDraft || codeBlockZeroWidth(p) {
		if shift {
			return
		}
		blocks := a.mdBlocks()
		if a.mdActive < 0 || a.mdActive >= len(blocks) {
			return
		}
		blk := blocks[a.mdActive]
		draft := a.mdDraft + "\n\t"
		docCaret := a.blockRuneOffset(a.mdActive) + utf8.RuneCountInString(draft)
		newDoc := spliceDocLines(a.activeTab().Text, blk.start, blk.end, draft)
		a.pendingEditKind = editCommand
		a.mdDraft = draft
		a.clearFenceDraft()
		a.applyDocEditAt(newDoc, docCaret, true)
		return
	}
	raw := a.mdDraft
	lo, hi := start, end
	if lo > hi {
		lo, hi = hi, lo
	}
	lo, hi = clamp(lo, 0, utf8.RuneCountInString(raw)), clamp(hi, 0, utf8.RuneCountInString(raw))
	if lo == hi && !shift {
		// 无选区：光标处插入一个制表符。
		b := runeToByte(raw, lo)
		a.pendingEditKind = editCommand
		a.mdDraft = raw[:b] + "\t" + raw[b:]
		a.publishDraft(lo + 1)
		return
	}
	first, firstStart := codeLineOf(raw, lo)
	last, lastStart := codeLineOf(raw, hi)
	if last > first && hi == lastStart {
		// 选区止于行首时该行不计入。
		last--
	}
	if first == last && hi > lo && !shift {
		// 同行选区：与普通输入一致，把选区替换为制表符。
		ls, le := runeToByte(raw, lo), runeToByte(raw, hi)
		a.pendingEditKind = editCommand
		a.mdDraft = raw[:ls] + "\t" + raw[le:]
		a.publishDraft(lo + 1)
		return
	}
	// 行级缩进：选区覆盖的每一行行首插入或移除一级缩进。
	lines := strings.Split(raw, "\n")
	added, lastDelta, firstDelta := 0, 0, 0
	changed := false
	for i := first; i <= last; i++ {
		if !shift {
			lines[i] = "\t" + lines[i]
			if i < last {
				added++
			} else {
				lastDelta = 1
			}
			changed = true
			continue
		}
		k := 0
		if strings.HasPrefix(lines[i], "\t") {
			k = 1
		} else {
			for k < len(lines[i]) && k < 4 && lines[i][k] == ' ' {
				k++
			}
		}
		if i == first {
			firstDelta = -k
		}
		if k == 0 {
			continue
		}
		lines[i] = lines[i][k:]
		if i < last {
			added -= k
		} else {
			lastDelta = -k
		}
		changed = true
	}
	if !changed {
		return
	}
	a.pendingEditKind = editCommand
	a.mdDraft = strings.Join(lines, "\n")
	// 选区端点随行首增减平移：终点若停在缩进区间内则收缩到行首。
	caret := lastStart + added + max(0, hi-lastStart+lastDelta)
	anchor := firstStart
	if shift {
		anchor = firstStart + max(0, lo-firstStart+firstDelta)
	}
	a.publishDraft(caret)
	a.selSyncStart, a.selSyncEnd = anchor, caret
}

// codeBlockZeroWidth 报告代码块投影是否为行末零宽可编辑区：未确认的单行
// 围栏没有内容行，Tab 需改落到围栏后的合成内容行（与键入重定向同规则）。
func codeBlockZeroWidth(p liveProjection) bool {
	if p.start != p.end || p.end <= 0 || p.end != utf8.RuneCountInString(p.raw) {
		return false
	}
	i := strings.LastIndexByte(p.raw, '\n')
	return isFenceOpeningLine(p.raw[i+1:])
}

// codeLineOf 返回块内 rune 偏移 off 所在的行号与该行行首的 rune 偏移。
func codeLineOf(raw string, off int) (line, lineStart int) {
	r, b := 0, 0
	for b < len(raw) && r < off {
		if raw[b] == '\n' {
			line++
			lineStart = r + 1
		}
		_, sz := utf8.DecodeRuneInString(raw[b:])
		b += sz
		r++
	}
	return line, lineStart
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

func (a *app) completeFenceDraft(start, end int) bool {
	if !a.fenceDraft || start != end || end != utf8.RuneCountInString(a.mdDraft) {
		return false
	}
	opening, ok := parseFenceOpeningLine(a.mdDraft)
	if !ok {
		return false
	}
	blocks := a.mdBlocks()
	if a.mdActive < 0 || a.mdActive >= len(blocks) {
		return false
	}
	blk := blocks[a.mdActive]
	closing := fenceClosingMarker(opening)
	draft := a.mdDraft + "\n\n" + closing
	caret := utf8.RuneCountInString(a.mdDraft) + 1
	docCaret := a.blockRuneOffset(a.mdActive) + caret
	newDoc := spliceDocLines(a.activeTab().Text, blk.start, blk.end, draft)
	a.pendingEditKind = editFencePair
	a.clearFenceDraft()
	a.applyDocEditAt(newDoc, docCaret, true)
	return true
}

func (a *app) afterEnter(start, end int) {
	if a.fenceDraft {
		if a.completeFenceDraft(start, end) {
			return
		}
		a.clearFenceDraft()
	}
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
			doc, _ := spliceLines(lines, at, at-1, "")
			a.applyDocEditAt(doc, a.blockRuneOffset(a.mdActive), true)
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
				doc, _ := spliceLines(lines, at, at-1, "")
				a.applyDocEditAt(doc, a.blockRuneOffset(a.mdActive), true)
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
	newDoc := a.spliceDoc(a.activeTab().Text, prev.start, cur.end, joined)
	caret := joinCaret(prev, cur)
	a.applyDocEditAt(newDoc, a.blockRuneOffset(prevIdx)+caret, true)
}

// blurLive 退出块编辑，回到全块预览态。
func (a *app) blurLive() {
	a.commitDraft(-1)
	a.clearFenceDraft()
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
