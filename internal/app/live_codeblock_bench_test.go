package app

import (
	"fmt"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

// benchCodeDoc 生成一个大代码块（n 行典型 Go 代码，行首两级缩进），
// 周围有正文块，模拟真实笔记里的代码段。
func benchCodeDoc(n int) string {
	var b strings.Builder
	b.WriteString("前文一段普通正文，让代码块不是文档第一个块。\n\n")
	b.WriteString("```go\n")
	for i := 0; i < n; i++ {
		switch i % 4 {
		case 0:
			fmt.Fprintf(&b, "  func handler%d(w *http.Request) error {\n", i)
		case 1:
			fmt.Fprintf(&b, "    if err := step%d(ctx); err != nil {\n      return fmt.Errorf(\"step: %%w\", err)\n    }\n", i)
		case 2:
			fmt.Fprintf(&b, "    items := map[string]int{\"alpha\": %d, \"beta\": %d}\n", i, i*2)
		case 3:
			b.WriteString("  }\n")
		}
	}
	b.WriteString("```\n\n后文一段普通正文。\n")
	return b.String()
}

// benchCodeBlock 在跑分 app 里定位大代码块并返回块首行号。
func benchCodeBlock(b *testing.B, lines int) (*app, int) {
	b.Helper()
	a := benchApp(b, benchCodeDoc(lines))
	idx := -1
	for i, blk := range a.mdBlocks() {
		if blk.kind == bkCode {
			idx = i
			break
		}
	}
	if idx < 0 {
		b.Fatal("no code block")
	}
	a.mdActive = idx
	return a, idx
}

// resetCodeDoc 每轮迭代前恢复固定文档与草稿：缩进是改写性事务，
// 不复位会让上一轮的缩进泄漏进下一轮，测得的不再是单次成本。
func resetCodeDoc(b *testing.B, a *app, start int, doc string) (liveBlock, liveProjection) {
	b.Helper()
	b.StopTimer()
	a.activeTab().Text = doc
	a.lowerDoc = ""
	a.invalidateMdCache()
	var blk liveBlock
	for j, b2 := range a.mdBlocks() {
		if b2.start == start {
			a.mdActive = j
			a.mdDraft = b2.raw
			blk = b2
		}
	}
	a.selSyncStart, a.selSyncEnd = -1, -1
	a.codeLangBlock, a.codeLangSynced, a.codeLangSeenFocus, a.codeLangFocus = -1, -1, false, 0
	p := projectBlock(blk)
	b.StartTimer()
	return blk, p
}

// codeBlockAt 返回当前文档里行号 start 的代码块（每轮迭代后重取，避免旧代）。
func codeBlockAt(a *app, start int) liveBlock {
	for _, blk := range a.mdBlocks() {
		if blk.start == start {
			return blk
		}
	}
	panic("code block gone")
}

// BenchmarkCodeTabInsert 大代码块内无选区按 Tab（插入缩进单位 + 提交管线）。
func BenchmarkCodeTabInsert(b *testing.B) {
	doc := benchCodeDoc(2000)
	a, start := benchCodeBlock(b, 2000)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; b.Loop(); i++ {
		_, p := resetCodeDoc(b, a, start, doc)
		caret := 100 + i%1000
		a.applyCodeTab(false, p, caret, caret)
	}
}

// BenchmarkCodeTabIndentSelection 大代码块内跨 500 行选区整行缩进。
func BenchmarkCodeTabIndentSelection(b *testing.B) {
	doc := benchCodeDoc(2000)
	a, start := benchCodeBlock(b, 2000)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		_, p := resetCodeDoc(b, a, start, doc)
		a.applyCodeTab(false, p, 50, 2550)
	}
}

// BenchmarkCodeTabDedentSelection 大代码块内跨 500 行选区 Shift+Tab 反向缩进。
func BenchmarkCodeTabDedentSelection(b *testing.B) {
	doc := benchCodeDoc(2000)
	a, start := benchCodeBlock(b, 2000)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		b.StopTimer()
		a.activeTab().Text = doc
		a.invalidateMdCache()
		for j, blk := range a.mdBlocks() {
			if blk.start == start {
				a.mdActive = j
				lines := strings.Split(blk.raw, "\n")
				for i := 1; i < len(lines)-1; i++ {
					lines[i] = "\t" + lines[i]
				}
				a.mdDraft = strings.Join(lines, "\n")
			}
		}
		a.selSyncStart, a.selSyncEnd = -1, -1
		p := projectBlock(liveBlock{raw: a.mdDraft, start: start, kind: bkCode})
		b.StartTimer()
		a.applyCodeTab(true, p, 50, 2550)
	}
}

// BenchmarkApplyCodeEnter 大代码块内回车续行（继承行首缩进）。
func BenchmarkApplyCodeEnter(b *testing.B) {
	doc := benchCodeDoc(2000)
	a, start := benchCodeBlock(b, 2000)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; b.Loop(); i++ {
		resetCodeDoc(b, a, start, doc)
		a.applyCodeEnter(200 + i%1000)
	}
}

// BenchmarkCommitCodeLang 语言徽标提交：改写大文档中围栏行的 info 串
// （含块重切、撤销事务记录与状态清理）。
func BenchmarkCommitCodeLang(b *testing.B) {
	doc := benchCodeDoc(2000)
	a, start := benchCodeBlock(b, 2000)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; b.Loop(); i++ {
		resetCodeDoc(b, a, start, doc)
		blk := codeBlockAt(a, start)
		a.codeLangBlock = blk.start
		a.codeLangText = fmt.Sprintf("lang%d", i%7)
		a.codeLangSeenFocus = true
		a.codeLangSynced = -1
		a.codeLangFocus = 0
		a.commitCodeLang()
	}
}

// BenchmarkFrameActiveCodeBlockWithBadge 焦点代码块（含语言徽标输入框）
// 的整帧构建成本；与 BenchmarkFrameActiveParaBlock 对比可得整块渲染成本，
// 与 NoBadge 变体对比可得徽标增量。
func BenchmarkFrameActiveCodeBlockWithBadge(b *testing.B) {
	benchFrameCodeBlock(b, true)
}

// BenchmarkFrameActiveCodeBlockNoBadge 同上但不构建语言徽标
// （codeLangBlock 设为不可能命中的 -2 走早退分支），用于隔离徽标成本。
func BenchmarkFrameActiveCodeBlockNoBadge(b *testing.B) {
	benchFrameCodeBlock(b, false)
}

func benchFrameCodeBlock(b *testing.B, badge bool) {
	b.Helper()
	a := benchApp(b, benchCodeDoc(200))
	for i, blk := range a.mdBlocks() {
		if blk.kind == bkCode {
			a.focusBlock(i, 4, 4)
			break
		}
	}
	if !badge {
		a.codeLangBlock = -2
	}
	tt := ui.NewTester(a.view, 400, 540)
	tt.Frame()
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		b.StopTimer()
		if !badge {
			a.codeLangBlock = -2
		}
		b.StartTimer()
		tt.Frame()
	}
}

// BenchmarkFrameActiveParaBlock 焦点段落块整帧构建（整块渲染对比基线）。
func BenchmarkFrameActiveParaBlock(b *testing.B) {
	doc := "前面一段。\n\n```go\nx := 1\n```\n\n这是一段用于对比的普通正文文本，长度与代码块相当，含 **行内样式** 与 `代码`。"
	a := benchApp(b, doc)
	a.focusBlock(2, 0, 0)
	tt := ui.NewTester(a.view, 400, 540)
	tt.Frame()
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		tt.Frame()
	}
}
