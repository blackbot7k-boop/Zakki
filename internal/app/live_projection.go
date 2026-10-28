package app

// The native editor holds only visible text. Markdown punctuation stays in the
// document; rune maps translate selections and edits without exposing it on focus.
import (
	"strings"
	"unicode/utf8"

	"github.com/egoist/mygo/ui"

	"zakki/internal/codehl"
)

type projectedRune struct{ start, end int }
type projectedStyle struct {
	start, end                 int // visible rune range
	open, content, close, tail int // source offsets
	kind                       spanKind
}
type liveProjection struct {
	raw, text    string
	start, end   int // editable source region (prefixes/fences remain outside it)
	units        []projectedRune
	styles       []projectedStyle
	objects      []int // 时间组件在可见文本中的单 rune 位置
	cell, inline bool
}

func projectBlock(b liveBlock) liveProjection {
	rs := []rune(b.raw)
	lo, hi := 0, len(rs)
	switch b.kind {
	case bkHeading, bkQuote, bkList, bkTask:
		for lo < hi && (rs[lo] == ' ' || rs[lo] == '\t') {
			lo++
		}
		s := string(rs[lo:])
		switch b.kind {
		case bkHeading:
			for lo < hi && rs[lo] == '#' {
				lo++
			}
		case bkQuote:
			lo++
		case bkList:
			lo += listMarkerLen(s)
		case bkTask:
			lo += utf8.RuneCountInString(s[:taskPrefixLen(s)])
		}
		for lo < hi && (rs[lo] == ' ' || rs[lo] == '\t') {
			lo++
		}
	case bkCode:
		openingLine := b.raw
		if i := strings.IndexRune(b.raw, '\n'); i >= 0 {
			openingLine = b.raw[:i]
			lo = utf8.RuneCountInString(b.raw[:i+1])
		} else {
			lo = hi
		}
		opening, _ := parseFenceOpeningLine(openingLine)
		if i := strings.LastIndexByte(b.raw, '\n'); i >= 0 && isFenceClosingLine(b.raw[i+1:], opening) {
			hi = utf8.RuneCountInString(b.raw[:i])
			if hi < lo {
				hi = lo
			}
		}
	case bkRule:
		lo = hi
	}
	p := projectRegion(b.raw, lo, hi, b.kind != bkCode, false)
	if b.hasTime {
		p = p.withTimeObject()
	}
	return p
}

// plainLineProjection 等价于 projectRegion(raw, lo, lo+行长, false, false)，
// 前提是 line 恰为 raw 从 rune 偏移 lo 起的一整行：代码块逐行预览时不必
// 为每行把整块源码转一遍 rune（行数 × 块长的平方开销）。
func plainLineProjection(raw, line string, lo int) liveProjection {
	n := utf8.RuneCountInString(line)
	p := liveProjection{raw: raw, text: line, start: lo, end: lo + n, units: make([]projectedRune, n)}
	for i := range p.units {
		p.units[i] = projectedRune{lo + i, lo + i + 1}
	}
	return p
}

func projectRegion(raw string, lo, hi int, inline, cell bool) liveProjection {
	rs := []rune(raw)
	lo, hi = clamp(lo, 0, len(rs)), clamp(hi, lo, len(rs))
	p := liveProjection{raw: raw, start: lo, end: hi, cell: cell, inline: inline}
	var spans []mSpan
	if inline {
		spans = inlineSpans(string(rs[lo:hi]))
	}
	var visible []rune
	appendRun := func(from, to int) {
		for i := from; i < to; i++ {
			if cell && rs[i] == '\\' && i+1 < to && (rs[i+1] == '|' || rs[i+1] == '\\') {
				visible = append(visible, rs[i+1])
				p.units = append(p.units, projectedRune{i, i + 2})
				i++
			} else if cell && i+4 <= to && string(rs[i:i+4]) == "<br>" {
				visible = append(visible, '\n')
				p.units = append(p.units, projectedRune{i, i + 4})
				i += 3
			} else {
				visible = append(visible, rs[i])
				p.units = append(p.units, projectedRune{i, i + 1})
			}
		}
	}
	prev := lo
	for _, sp := range spans {
		a, z := lo+sp.start, lo+sp.end
		appendRun(prev, a)
		inner, close := a+1, z-1
		if sp.kind == spanBold {
			inner, close = a+2, z-2
		}
		if sp.kind == spanLink {
			close = a + runeIndexOf(rs[a:z], ']', 1)
		}
		s := projectedStyle{start: len(visible), open: a, content: inner, close: close, tail: z, kind: sp.kind}
		appendRun(inner, close)
		s.end = len(visible)
		p.styles = append(p.styles, s)
		prev = z
	}
	appendRun(prev, hi)
	p.text = string(visible)
	return p
}

func (p liveProjection) sourceOffset(visible int) int {
	visible = clamp(visible, 0, len(p.units))
	if len(p.units) == 0 {
		return p.start
	}
	if visible == len(p.units) {
		return p.units[visible-1].end
	}
	return p.units[visible].start
}

func (p liveProjection) visibleOffset(source int) int {
	n := 0
	for _, u := range p.units {
		if u.start >= source {
			break
		}
		n++
	}
	return n
}

// nativeCaretAfterEdit 规范一次原生文本编辑后的光标。原生的插入/替换总把光标
// 推进到新内容之后；AX 等整值写入却会原样保留旧光标。旧可见文本是新文本的
// 严格前缀（纯追加）而光标仍停在旧文本长度之内时，与该文本状态一致的唯一
// 插入边界就是新文本末尾——把光标吸到末尾，避免下一个字符插进文本中间。
func nativeCaretAfterEdit(old, neu string, caret int) int {
	oldRunes, neuRunes := utf8.RuneCountInString(old), utf8.RuneCountInString(neu)
	if caret < 0 || caret > neuRunes || neuRunes <= oldRunes || !strings.HasPrefix(neu, old) {
		return caret
	}
	if caret <= oldRunes {
		return neuRunes
	}
	return caret
}

// replace preserves wrappers around surviving characters, including link URLs.
// Deleting an entire styled run removes its wrappers; replacing its contents
// retains the style. Only a table cell escapes inserted pipes and line breaks.
func (p liveProjection) replace(next string, caret int) (string, int) {
	return p.replaceSelection(next, caret, -1, -1)
}

func (p liveProjection) replaceSelection(next string, caret, selectedStart, selectedEnd int) (string, int) {
	old, neu := []rune(p.text), []rune(next)
	caret = clamp(caret, 0, len(neu))
	prefix := 0
	for prefix < len(old) && prefix < len(neu) && old[prefix] == neu[prefix] {
		prefix++
	}
	// Repeated characters can have several equivalent diffs. The resulting
	// caret identifies the actual insertion/deletion boundary.
	prefix = min(prefix, max(0, caret-max(len(neu)-len(old), 0)))
	suffix := 0
	for suffix < len(old)-prefix && suffix < len(neu)-caret && old[len(old)-1-suffix] == neu[len(neu)-1-suffix] {
		suffix++
	}
	if selectedStart >= 0 && selectedEnd >= selectedStart && selectedEnd <= len(old) &&
		selectedStart <= caret && caret <= len(neu) &&
		string(neu[:selectedStart]) == string(old[:selectedStart]) &&
		string(neu[caret:]) == string(old[selectedEnd:]) {
		prefix, suffix = selectedStart, len(old)-selectedEnd
	}
	end := len(old) - suffix
	inserted := string(neu[prefix : len(neu)-suffix])
	// 时间令牌必须以空白收尾；对象后相邻输入也要写成合法源码。
	// 分隔符同时进入投影，光标越过它，后续输入继续沿用原生编辑路径。
	if inserted != "" && !strings.ContainsRune(" \t\r\n", []rune(inserted)[0]) {
		for _, object := range p.objects {
			if prefix == object+1 {
				inserted = " " + inserted
				caret++
				break
			}
		}
	}
	if p.cell {
		inserted = strings.ReplaceAll(inserted, `\`, `\\`)
		inserted = strings.ReplaceAll(inserted, "|", `\|`)
		inserted = strings.ReplaceAll(strings.ReplaceAll(inserted, "\r\n", "\n"), "\n", "<br>")
	}
	at := p.sourceOffset(prefix)
	rs := []rune(p.raw)
	remove := make([]bool, len(rs))
	mark := func(start, stop int) {
		for i := start; i < stop; i++ {
			remove[i] = true
		}
	}
	for _, u := range p.units[prefix:end] {
		mark(u.start, u.end)
	}
	for _, s := range p.styles {
		if prefix <= s.start && end >= s.end && (inserted == "" || at < s.content || at > s.close) {
			mark(s.open, s.content)
			mark(s.close, s.tail)
		}
	}
	var out []rune
	for i := 0; i <= len(rs); i++ {
		if i == at {
			out = append(out, []rune(inserted)...)
		}
		if i < len(rs) && !remove[i] {
			out = append(out, rs[i])
		}
	}
	raw := string(out)
	projected := projectRegion(raw, p.start, p.end+len(out)-len(rs), p.inline, p.cell)
	if len(p.objects) > 0 {
		projected = projected.withTimeObject()
	}
	return raw, projected.sourceOffset(caret)
}

// fragment serializes a visible slice with complete wrappers. Paragraph splits
// close styles on the left and reopen them on the right, retaining link targets.
func (p liveProjection) fragment(start, end int) string {
	start, end = clamp(start, 0, len(p.units)), clamp(end, start, len(p.units))
	rs := []rune(p.raw)
	var out strings.Builder
	appendUnits := func(lo, hi int) {
		for _, u := range p.units[lo:hi] {
			out.WriteString(string(rs[u.start:u.end]))
		}
	}
	pos := start
	for _, s := range p.styles {
		lo, hi := max(start, s.start), min(end, s.end)
		if lo >= hi {
			continue
		}
		appendUnits(pos, lo)
		out.WriteString(string(rs[s.open:s.content]))
		appendUnits(lo, hi)
		out.WriteString(string(rs[s.close:s.tail]))
		pos = hi
	}
	appendUnits(pos, end)
	return out.String()
}

func (p liveProjection) content(c *ui.Context, t *ui.Theme, md resolvedStyle) ui.Element {
	text := p.text
	if text == "" {
		text = " "
	}
	if len(p.styles) == 0 {
		return ui.Text(c, text)
	}
	rs := []rune(p.text)
	var spans []ui.Span
	prev := 0
	for _, s := range p.styles {
		if s.start > prev {
			spans = append(spans, ui.Span{Text: string(rs[prev:s.start])})
		}
		sp := inlineStyle(s.kind, t, md)
		sp.Text = string(rs[s.start:s.end])
		spans = append(spans, sp)
		prev = s.end
	}
	if prev < len(rs) {
		spans = append(spans, ui.Span{Text: string(rs[prev:])})
	}
	return ui.RichText(c, spans...)
}

// searchContent 是带查找命中标记的非焦点块内容：命中与焦点块编辑区同样
// 只改颜色与字重；没有命中时与 content 完全一致。
func (p liveProjection) searchContent(c *ui.Context, t *ui.Theme, md resolvedStyle, query string) ui.Element {
	if query == "" || p.text == "" || len(findMatches(p.text, query)) == 0 {
		return p.content(c, t, md)
	}
	rs := []rune(p.text)
	var spans []ui.Span
	prev := 0
	for _, r := range p.textRanges(query, t, md) {
		if r.Start > prev {
			spans = append(spans, ui.Span{Text: string(rs[prev:r.Start])})
		}
		spans = append(spans, ui.Span{
			Text: string(rs[r.Start:r.End]), Color: r.Color, Background: r.Background,
			Weight: r.Weight, Font: r.Font, Italic: r.Italic,
			Underline: r.Underline, WavyUnderline: r.WavyUnderline, DecorationColor: r.DecorationColor,
		})
		prev = r.End
	}
	if prev < len(rs) {
		spans = append(spans, ui.Span{Text: string(rs[prev:])})
	}
	return ui.RichText(c, spans...)
}

func (p liveProjection) textRanges(query string, t *ui.Theme, md resolvedStyle) []ui.TextRange {
	accent := md.accent
	// 搜索命中覆盖颜色、字重与底色（见 searchHit），保留字体、斜体与装饰。
	ranges := make([]ui.TextRange, len(p.units))
	for _, s := range p.styles {
		r := inlineTextRange(inlineStyle(s.kind, t, md))
		for i := s.start; i < s.end; i++ {
			ranges[i] = r
		}
	}
	for _, m := range findMatches(p.text, query) {
		for i := m.Start; i < m.End; i++ {
			hit := searchHit(0, 0, accent)
			ranges[i].Color, ranges[i].Weight, ranges[i].Background = hit.Color, hit.Weight, hit.Background
		}
	}
	var out []ui.TextRange
	for i, r := range ranges {
		if r == (ui.TextRange{}) {
			continue
		}
		if n := len(out); n > 0 && out[n-1].End == i {
			previous := out[n-1]
			previous.Start, previous.End = 0, 0
			if previous == r {
				out[n-1].End++
				continue
			}
		}
		r.Start, r.End = i, i+1
		out = append(out, r)
	}
	return out
}

// liveSnapshot stores raw selections, so undo restores hidden formatting too.
type liveSnapshot struct {
	doc        string
	start, end int
}

// editKind 区分事务类型：连续输入合并为一个撤销事务；粘贴、IME 一次确认、
// 格式命令各自独立成事务。
type editKind uint8

const (
	editInput     editKind = iota // 连续输入（同光标连续插入可合并）
	editPaste                     // 粘贴
	editIME                       // 输入法一次确认
	editCommand                   // 格式命令/表格工具等结构化编辑
	editFencePair                 // 围栏逐键输入与 Enter 配对合并成一个事务
)

type liveEdit struct {
	before, after liveSnapshot
	kind          editKind
	delta         *liveEditDelta
}

func (a *app) liveSnapshot() liveSnapshot {
	if a.selDoc {
		return liveSnapshot{a.activeTab().Text, a.selDocStart, a.selDocEnd}
	}
	if a.cfg.Source {
		// 源码模式：全文即文档，选区为文档 rune 偏移。
		start, end := a.srcCaret, a.srcSelEnd
		if a.srcSelSyncStart >= 0 {
			start, end = a.srcSelSyncStart, a.srcSelSyncEnd
		}
		return liveSnapshot{a.activeTab().Text, max(start, 0), max(end, 0)}
	}
	start, end := a.mdCaret, a.mdSelEnd
	if a.selSyncStart >= 0 {
		start, end = a.selSyncStart, a.selSyncEnd
	}
	blocks := a.mdBlocks()
	block := a.mdActive
	if block < 0 {
		// 无焦点块时沿用旧扫描的末尾偏移（包含最后一个合成换行）。
		block = len(blocks)
	}
	off := a.blockRuneOffset(block)
	return liveSnapshot{a.activeTab().Text, off + max(start, 0), off + max(end, 0)}
}

// rememberLiveEdit 记录一次文档级事务。连续输入（同光标位置的连续插入）
// 合并进上一事务；粘贴/IME/格式命令各自独立。上限 200 条。
func (a *app) rememberLiveEdit(before liveSnapshot, kind editKind) {
	after := a.liveSnapshot()
	if after.doc == before.doc {
		return
	}
	if len(a.liveUndo) > 0 && !a.liveUndo[len(a.liveUndo)-1].matchesAfter(before.doc) {
		a.liveUndo = nil
	}
	if n := len(a.liveUndo); n > 0 && kind == editFencePair {
		last := &a.liveUndo[n-1]
		if last.kind == editInput && last.matchesAfter(before.doc) {
			*last, _ = last.expanded(before.doc, false)
			last.after = after
			last.kind = editCommand
			a.liveRedo = nil
			return
		}
	}
	// 合并：上一事务也是输入、光标连续且都是插入（不合并删除）。
	if n := len(a.liveUndo); n > 0 && kind == editInput {
		last := &a.liveUndo[n-1]
		grew := len(after.doc) >= len(before.doc)
		contiguous := before.start == before.end && last.after.start == last.after.end &&
			last.after.end == before.start
		if last.kind == editInput && last.matchesAfter(before.doc) && contiguous && grew {
			*last, _ = last.expanded(before.doc, false)
			last.after = after
			a.liveRedo = nil
			return
		}
	}
	if n := len(a.liveUndo); n > 0 {
		a.liveUndo[n-1] = a.liveUndo[n-1].compressed()
	}
	a.liveUndo = append(a.liveUndo, liveEdit{before: before, after: after, kind: kind})
	if len(a.liveUndo) > 200 {
		a.liveUndo[0] = liveEdit{}
		a.liveUndo = a.liveUndo[1:]
	}
	a.liveRedo = nil
}
func (a *app) undoLive(redo bool) {
	a.tableTools = tableTools{}
	a.cancelCodeLang() // 撤销重写文档，未提交的语言草稿作废
	from, to := &a.liveUndo, &a.liveRedo
	if redo {
		from, to = to, from
	}
	if len(*from) == 0 {
		return
	}
	top := (*from)[len(*from)-1]
	e, ok := top.expanded(a.activeTab().Text, redo)
	if top.delta != nil || !ok {
		a.liveUndo, a.liveRedo = nil, nil
		return
	}
	want := e.before
	if redo {
		want = e.after
	}
	(*from)[len(*from)-1] = liveEdit{}
	*from = (*from)[:len(*from)-1]
	if n := len(*from); n > 0 {
		next, valid := (*from)[n-1].expanded(want.doc, redo)
		if !valid {
			a.liveUndo, a.liveRedo = nil, nil
			return
		}
		(*from)[n-1] = next
	}
	if n := len(*to); n > 0 {
		(*to)[n-1] = (*to)[n-1].compressed()
	}
	*to = append(*to, e)
	a.clearDocSelection()
	a.activeTab().Text = want.doc
	if a.cfg.Source {
		// 源码模式：恢复文本与选区，编辑器缓冲下一帧由绑定重新同步。
		a.srcSelSyncStart, a.srcSelSyncEnd = want.start, want.end
		a.onEdited()
		return
	}
	blocks := a.mdBlocks()
	idx, off := a.blockOfRuneOffset(want.start)
	a.mdActive, a.mdDraft = idx, blocks[idx].raw
	a.selSyncStart, a.selSyncEnd = want.start-off, want.end-off
	if want.end > off+utf8.RuneCountInString(blocks[idx].raw) {
		a.selDoc, a.selDocStart, a.selDocEnd = true, want.start, want.end
		a.selSyncEnd = a.selSyncStart
	}
	a.liveFocus, a.scrollActive = 1, true
	a.mdPending, a.lastPendingCaret = nil, 0
	a.onEdited()
}

// textRangesInBlock 按块类型生成可见文本着色：搜索命中与行内样式优先，
// 代码块在其空隙上叠加 token 配色（跟随文本时无 token 层，原样返回）。
// 焦点块的编辑路径（activeText）与非焦点块的 RichText 路径共用同一套配色，
// 避免编辑中的代码块褪色为单色文本。
func (p liveProjection) textRangesInBlock(query string, t *ui.Theme, md resolvedStyle, blk liveBlock, codeTheme string) []ui.TextRange {
	ranges := p.textRanges(query, t, md)
	if blk.kind != bkCode {
		return ranges
	}
	cp, ok := codePaletteFor(codeTheme, t)
	if !ok {
		return ranges
	}
	// token 色：逐 rune 记录（跨行累计偏移），再填进 base 的空隙。
	token := make([]ui.Color, len(p.units))
	runeOff := 0
	for _, line := range strings.Split(p.text, "\n") {
		for _, s := range codehl.Line(codehl.Lang(blk.info), line) {
			if cp[s.Kind] == (ui.Color{}) {
				continue
			}
			for i := s.Start; i < s.End; i++ {
				token[runeOff+i] = cp[s.Kind]
			}
		}
		runeOff += utf8.RuneCountInString(line) + 1
	}
	var out []ui.TextRange
	next := 0
	for _, r := range ranges {
		if r.Start > next {
			out = appendTokenRuns(out, token, next, r.Start)
		}
		out = append(out, r)
		next = r.End
	}
	if next < len(token) {
		out = appendTokenRuns(out, token, next, len(token))
	}
	return out
}

// appendTokenRuns 把 [start,end) 内连续同色的 token 区间并成 runs 追加到 out。
func appendTokenRuns(out []ui.TextRange, token []ui.Color, start, end int) []ui.TextRange {
	for i := start; i < end; {
		if token[i] == (ui.Color{}) {
			i++
			continue
		}
		j := i + 1
		for j < end && token[j] == token[i] {
			j++
		}
		out = append(out, ui.TextRange{Start: i, End: j, Color: token[i]})
		i = j
	}
	return out
}
