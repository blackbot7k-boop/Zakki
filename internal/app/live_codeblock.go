package app

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/egoist/mygo/ui"

	"zakki/internal/markdown"
	"zakki/locales"
)

// 本文件聚集代码块（围栏）编辑的专属行为：Tab 缩进（live_input.go 只负责
// 按键路由）、Enter 续行缩进、右下角语言徽标。缩进单位由外观页
// 「代码块缩进」决定（appearance.codeIndent）。

// codeIndentUnit 代码块缩进单位：2 或 4 个空格（外观页「代码块缩进」）。
func (a *app) codeIndentUnit() string {
	if a.prefs.Appearance.CodeIndent == "space2" {
		return "  "
	}
	return "    "
}

// applyCodeTab 处理代码块内的 Tab / Shift+Tab：无选区在光标处插入缩进单位
// （2 或 4 空格），同行选区按普通输入替换为缩进单位，跨行选区整行增减一级
// 缩进（反向时行首制表符按一级移除，空格至多同档宽度）并保留选区以便连续
// 调整。退化围栏（未
// 确认的单行围栏、零宽可编辑区）没有内容行，改为在围栏后新开内容行落缩进，
// 避免写进围栏行的语言标识。按键已在输入层消费，焦点不会跳出编辑器。
func (a *app) applyCodeTab(shift bool, p liveProjection, start, end int) {
	unit := a.codeIndentUnit()
	if a.fenceDraft || codeBlockZeroWidth(p) {
		if shift {
			return
		}
		blocks := a.mdBlocks()
		if a.mdActive < 0 || a.mdActive >= len(blocks) {
			return
		}
		blk := blocks[a.mdActive]
		draft := a.mdDraft + "\n" + unit
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
		// 无选区：光标处插入一个缩进单位。
		b := runeToByte(raw, lo)
		a.pendingEditKind = editCommand
		a.mdDraft = raw[:b] + unit + raw[b:]
		a.publishDraft(lo + utf8.RuneCountInString(unit))
		return
	}
	first, firstStart := codeLineOf(raw, lo)
	last, lastStart := codeLineOf(raw, hi)
	if last > first && hi == lastStart {
		// 选区止于行首时该行不计入。
		last--
	}
	if first == last && hi > lo && !shift {
		// 同行选区：与普通输入一致，把选区替换为缩进单位。
		ls, le := runeToByte(raw, lo), runeToByte(raw, hi)
		a.pendingEditKind = editCommand
		a.mdDraft = raw[:ls] + unit + raw[le:]
		a.publishDraft(lo + utf8.RuneCountInString(unit))
		return
	}
	// 行级缩进：选区覆盖的每一行行首插入或移除一级缩进。
	unitRunes := utf8.RuneCountInString(unit)
	lines := strings.Split(raw, "\n")
	added, lastDelta, firstDelta := 0, 0, 0
	changed := false
	for i := first; i <= last; i++ {
		if !shift {
			lines[i] = unit + lines[i]
			if i < last {
				added += unitRunes
			} else {
				lastDelta = unitRunes
			}
			changed = true
			continue
		}
		k := codeDedentSize(lines[i], unit)
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

// codeDedentSize 计算一行行首要移除的反向缩进字节数：行首是制表符时按
// 一级缩进移除（粘贴进来的制表符缩进内容也能用 Shift+Tab 调整），
// 否则移除至多一个档位宽度的空格。
func codeDedentSize(line, unit string) int {
	if strings.HasPrefix(line, "\t") {
		return 1
	}
	width := utf8.RuneCountInString(unit)
	spaces := 0
	for spaces < len(line) && spaces < width && line[spaces] == ' ' {
		spaces++
	}
	return spaces
}

// applyCodeEnter 代码块内回车：新行继承光标所在行的行首缩进（编辑器式续行，
// 光标停在缩进之后）；光标前的文本以开括号结尾时再多一级缩进。
func (a *app) applyCodeEnter(start int) {
	raw := a.mdDraft
	b := runeToByte(raw, start)
	indent := codeLineIndentBefore(raw, start)
	linePrefix := raw[codeLineStartByte(raw, start):b]
	if codeEndsWithOpenBracket(strings.TrimRight(linePrefix, " \t")) {
		indent += a.codeIndentUnit()
	}
	a.mdDraft = raw[:b] + "\n" + indent + raw[b:]
	a.publishDraft(start + 1 + utf8.RuneCountInString(indent))
}

// codeLineIndentBefore 返回光标所在行行首空白中光标之前的部分：光标落在
// 缩进区间内时只保留到光标为止（与主流编辑器一致，不凭空放大缩进）。
func codeLineIndentBefore(raw string, caret int) string {
	start := codeLineStartByte(raw, caret)
	b := runeToByte(raw, caret)
	indent := 0
	for start+indent < b && (raw[start+indent] == ' ' || raw[start+indent] == '\t') {
		indent++
	}
	return raw[start : start+indent]
}

// codeLineStartByte 返回光标所在行的行首字节偏移。
func codeLineStartByte(raw string, caret int) int {
	b := runeToByte(raw, caret)
	if i := strings.LastIndexByte(raw[:b], '\n'); i >= 0 {
		return i + 1
	}
	return 0
}

// codeEndsWithOpenBracket 报告行尾是否以开括号结尾（回车再多缩进一级）。
func codeEndsWithOpenBracket(line string) bool {
	return strings.HasSuffix(line, "{") || strings.HasSuffix(line, "[") || strings.HasSuffix(line, "(")
}

// codeBlockZeroWidth 报告代码块投影是否为行末零宽可编辑区：未确认的单行
// 围栏没有内容行，Tab 需改落到围栏后的合成内容行（与键入重定向同规则）。
func codeBlockZeroWidth(p liveProjection) bool {
	if p.start != p.end || p.end <= 0 || p.end != utf8.RuneCountInString(p.raw) {
		return false
	}
	i := strings.LastIndexByte(p.raw, '\n')
	return markdown.IsFenceOpeningLine(p.raw[i+1:])
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

// codeLangBadge 活动代码块右下角的语言徽标：显示围栏 info 串（语言提示），
// 点击即可改语言，Enter 提交、Esc 取消，焦点回到代码内容时自动提交。
// 非活动块不构建徽标，预览热路径不增加输入控件。
func (a *app) codeLangBadge(c *ui.Context, md resolvedStyle, blk liveBlock) {
	if a.codeLangBlock >= 0 && a.codeLangBlock != blk.start {
		return
	}
	// 只在换块或会话重置后重置草稿：编辑中（含引擎尚未写回的当帧）不得覆盖
	// 绑定字段，否则会吞掉正在键入的字符（同 activeText 对 mdVisible 的处理）。
	target := blk.start + 1
	if a.codeLangBlock < 0 && a.codeLangSynced != blk.start {
		a.codeLangText = blk.info
		a.codeLangSynced = blk.start
	}
	// 失焦提交：徽标曾真正持有焦点且现已离开（与重命名输入的 seenFocus 同惯例，
	// 避免焦点请求落定前的首帧误判）；请求在途（未持过焦点）不动，避免取消在途请求。
	focused := ui.FocusedValue(c, &a.codeLangFocus) == target
	if focused {
		a.codeLangSeenFocus = true
	} else if a.codeLangSeenFocus {
		if a.codeLangBlock == blk.start {
			a.commitCodeLang()
			a.codeLangText = blk.info
		} else {
			// 只点了徽标没编辑就离开：放掉聚焦请求，避免反复抢焦点。
			a.codeLangFocus, a.codeLangSeenFocus = 0, false
		}
	}
	badge := ui.Row(c.Key(fmt.Sprintf("code-lang-%d", blk.start))).FillWidth()
	badge.Children(func() {
		ui.Box(c).Grow(1).Basis(0).MinWidth(0)
		in := ui.TextInput(c, &a.codeLangText).
			Placeholder(locales.CodeLangPlaceholder()).Label(locales.CodeLangLabel()).
			Width(120).Font(md.monoFont).FontSize(md.codeSize-2).
			TextColor(md.textMuted).Bind(&a.codeLangHandle).
			FocusBind(&a.codeLangFocus, target)
		if in.Clicked() {
			// 徽标自己消费这次点击并请求聚焦：行级空白点击不得把焦点抢回代码内容。
			a.liveClickHandled = true
			a.codeLangFocus = target
			c.Invalidate()
		}
		if in.Changed() && a.codeLangBlock < 0 {
			a.codeLangBlock = blk.start
		}
		if in.Submitted() {
			a.commitCodeLang()
			a.codeLangFocus = 0
			a.liveFocus = 1 // 改完语言回到代码内容继续写
			c.Invalidate()
		}
	})
}

// commitCodeLang 把徽标草稿写回开栏行的 info 串：只拼接该行，不动块结构，
// 不抢主编辑器的焦点与光标；草稿与原文相同时仅结束编辑。
func (a *app) commitCodeLang() {
	block, seen := a.codeLangBlock, a.codeLangSeenFocus
	a.codeLangBlock, a.codeLangSeenFocus, a.codeLangSynced, a.codeLangFocus = -1, false, -1, 0
	// 没有真正持焦点的会话（含零值误置）不提交，避免把空 info 写回围栏。
	if block < 0 || !seen {
		return
	}
	idx := -1
	for i, b := range a.mdBlocks() {
		if b.start == block {
			idx = i
			break
		}
	}
	if idx < 0 || a.mdBlocks()[idx].kind != bkCode {
		return // 块已重切，行号对不上时放弃（撤销/外部改写后的过期会话）
	}
	blk := a.mdBlocks()[idx]
	line0 := blk.raw
	if i := strings.IndexByte(blk.raw, '\n'); i >= 0 {
		line0 = blk.raw[:i]
	}
	newLine, ok := markdown.SetFenceInfo(line0, a.codeLangText)
	if !ok || newLine == line0 {
		return
	}
	before := a.liveSnapshot()
	newRaw := spliceDocLines(blk.raw, 0, 0, newLine)
	newDoc := spliceDocLines(a.activeTab().Text, blk.start, blk.end, newRaw)
	a.activeTab().Text = newDoc
	a.mdDraft = newRaw
	a.lowerDoc = ""
	a.invalidateMdCache()
	a.onEdited()
	a.rememberLiveEdit(before, editCommand)
}

// cancelCodeLang 放弃语言编辑，草稿在下次构建时按文档内容重置。
func (a *app) cancelCodeLang() {
	a.codeLangBlock, a.codeLangSeenFocus, a.codeLangSynced, a.codeLangFocus = -1, false, -1, 0
}
