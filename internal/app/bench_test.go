package app

import (
	"fmt"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

// benchDoc 生成约 180KB 的典型笔记文档（300 节：标题+段落+列表+待办）。
func benchDoc() string {
	var b strings.Builder
	b.Grow(200 << 10)
	for i := 0; i < 300; i++ {
		fmt.Fprintf(&b, "## 第 %d 节 标题\n\n", i+1)
		b.WriteString("这是一段 **加粗** 和 *斜体* 以及 `代码` 与 [链接](https://example.com/x) 的正文，包含一些中文内容用于测试性能，长度适中。\n\n")
		b.WriteString("- 列表项甲 内容\n- 列表项乙 内容\n- [ ] 待办事项 要完成\n\n")
	}
	return b.String()
}

// benchApp 跑分用 app：置 stopped 不落盘，避免定时保存干扰测量或写真实数据。
func benchApp(doc string) *app {
	a := newApp()
	a.notes.SetStopped(true)
	a.tabs[0].Text = doc
	a.resyncLive(0)
	return a
}

// BenchmarkLiveBlocks 块切分本身（每帧 ×2 次、每次提交 ×2-3 次的热点）。
func BenchmarkLiveBlocks(b *testing.B) {
	doc := benchDoc()
	b.SetBytes(int64(len(doc)))
	b.ReportAllocs()
	for b.Loop() {
		if bs := liveBlocks(doc); len(bs) == 0 {
			b.Fatal("no blocks")
		}
	}
}

// BenchmarkMdBlocksCached 带文档缓存后的重复调用（每帧的 ensureDraftSynced+livePane）。
func BenchmarkMdBlocksCached(b *testing.B) {
	a := benchApp(benchDoc())
	b.ReportAllocs()
	for b.Loop() {
		if bs := a.mdBlocks(); len(bs) == 0 {
			b.Fatal("no blocks")
		}
	}
}

// BenchmarkCommitDraft 一次按键的提交管线（拼回文档+重切+重映射）。
func BenchmarkCommitDraft(b *testing.B) {
	doc := benchDoc()
	b.SetBytes(int64(len(doc)))
	b.ReportAllocs()
	for b.Loop() {
		b.StopTimer()
		a := benchApp(doc)
		blocks := liveBlocks(doc)
		// 找中间一个段落块，模拟在其中打一个字
		mid := len(blocks) / 2
		for blocks[mid].kind != bkPara {
			mid++
		}
		a.mdActive = mid
		a.mdDraft = blocks[mid].raw + "字"
		b.StartTimer()
		a.commitDraft(utf8Len(a.mdDraft) - 1)
	}
}

func utf8Len(s string) int {
	n := 0
	for range s {
		n++
	}
	return n
}

// BenchmarkRenderSpans 渲染管线里的行内解析（渲染块 × 每帧）。
func BenchmarkRenderSpans(b *testing.B) {
	doc := benchDoc()
	b.SetBytes(int64(len(doc)))
	b.ReportAllocs()
	for b.Loop() {
		for _, blk := range liveBlocks(doc) {
			if blk.text != "" {
				_ = inlineSpans(blk.text)
			}
		}
	}
}

// BenchmarkEditorRanges 源码模式全文高亮（含 O(n²) 排序风险）。
func BenchmarkEditorRanges(b *testing.B) {
	doc := benchDoc()
	t := &ui.Theme{}
	b.SetBytes(int64(len(doc)))
	b.ReportAllocs()
	for b.Loop() {
		_ = editorRanges(doc, "", t, ui.Hex("#d4253a"))
	}
}

// BenchmarkStatusBarCounts 状态栏每帧统计。
func BenchmarkStatusBarCounts(b *testing.B) {
	doc := benchDoc()
	b.SetBytes(int64(len(doc)))
	b.ReportAllocs()
	for b.Loop() {
		_ = countLines(doc)
		_ = countWords(doc)
		_ = countChars(doc)
	}
}

// BenchmarkFindMatches 搜索命中（含全文 ToLower 拷贝）。
func BenchmarkFindMatches(b *testing.B) {
	doc := benchDoc()
	b.SetBytes(int64(len(doc)))
	b.ReportAllocs()
	for b.Loop() {
		_ = findMatches(doc, "待办")
	}
}
