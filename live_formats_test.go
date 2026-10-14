package main

import (
	"strings"
	"testing"
)

// Markdown 格式变换：源文档 + rune 选区 → 新文档 + 新选区的纯函数测试。
// 覆盖中文、emoji、已有格式与链接、代码围栏、表格与光标映射。

func runeLen(s string) int { return len([]rune(s)) }

func indexOf(doc, sub string) int { return runeLen(strings.SplitN(doc, sub, 2)[0]) }

func TestToggleInlineBasic(t *testing.T) {
	doc := "hello world"
	start, end := indexOf(doc, "world"), indexOf(doc, "world")+runeLen("world")
	res := applyInlineToLines(doc, 0, 0, start, end, spanBold)
	if !res.changed || res.doc != "hello **world**" {
		t.Fatalf("加粗: %+v", res)
	}
	if res.selStart != runeLen("hello **") || res.selEnd != runeLen("hello **world") {
		t.Fatalf("选区应包住内容: %d..%d", res.selStart, res.selEnd)
	}
	// 再次执行同范围（映射后的选区）→ 移除。
	res2 := applyInlineToLines(res.doc, 0, 0, res.selStart, res.selEnd, spanBold)
	if !res2.changed || res2.doc != doc {
		t.Fatalf("移除加粗应还原: %+v", res2)
	}
}

func TestToggleInlineChineseAndEmoji(t *testing.T) {
	doc := "中文：你好，世界 🌏 ok"
	start := indexOf(doc, "你好，世界 🌏")
	end := start + runeLen("你好，世界 🌏")
	res := applyInlineToLines(doc, 0, 0, start, end, spanItalic)
	if !strings.Contains(res.doc, "*你好，世界 🌏*") {
		t.Fatalf("中文与 emoji 加斜体: %+v", res)
	}
	res2 := applyInlineToLines(res.doc, 0, 0, res.selStart, res.selEnd, spanItalic)
	if res2.doc != doc {
		t.Fatalf("移除斜体应还原原文: %+v", res2)
	}
}

func TestToggleInlineInsideSpanRemoves(t *testing.T) {
	doc := "前 **加粗内容** 后"
	start := indexOf(doc, "加粗内容")
	end := start + runeLen("加粗内容")
	res := applyInlineToLines(doc, 0, 0, start, end, spanBold)
	if !res.changed || res.doc != "前 加粗内容 后" {
		t.Fatalf("选区在 span 内应移除格式: %+v", res)
	}
	if res.selStart != indexOf(res.doc, "加粗内容") || res.selEnd != res.selStart+runeLen("加粗内容") {
		t.Fatalf("移除后选区应回到内容: %d..%d", res.selStart, res.selEnd)
	}
}

func TestToggleInlineKeepsLinks(t *testing.T) {
	doc := "看 [Tot](https://tot.rocks) 很好"
	start := indexOf(doc, "[Tot](https://tot.rocks)")
	end := start + runeLen("[Tot](https://tot.rocks)")
	res := applyInlineToLines(doc, 0, 0, start, end, spanBold)
	if !strings.Contains(res.doc, "**[Tot](https://tot.rocks)**") {
		t.Fatalf("加粗应保留链接: %+v", res)
	}
}

func TestInlineCodeExclusive(t *testing.T) {
	// 应用代码：清除选区内粗体，不生成嵌套。
	doc := "**bold** text"
	start, end := 0, runeLen(doc)
	res := applyInlineToLines(doc, 0, 0, start, end, spanCode)
	if !strings.Contains(res.doc, "`") || strings.Contains(res.doc, "**") {
		t.Fatalf("应用代码应清除粗体: %+v", res)
	}
	// 应用粗体到代码 span：先清代码。
	doc2 := "`code` x"
	s2, e2 := 0, runeLen("`code`")
	res2 := applyInlineToLines(doc2, 0, 0, s2, e2, spanBold)
	if strings.Contains(res2.doc, "`") || !strings.Contains(res2.doc, "**code**") {
		t.Fatalf("粗体与代码互斥: %+v", res2)
	}
	// 同格式（代码）选区 → 移除。
	res3 := applyInlineToLines(doc2, 0, 0, s2, e2, spanCode)
	if res3.doc != "code x" {
		t.Fatalf("移除代码: %+v", res3)
	}
}

func TestInlinePartialCodeOverlapUnwraps(t *testing.T) {
	// 选区落在代码 span 内部、应用粗体：先解除代码（互斥），再包裹选区，
	// 不生成错误嵌套。
	doc := "`code` x"
	s := runeLen("`co")
	res := applyInlineToLines(doc, 0, 0, s, s+2, spanBold)
	if !res.changed || res.doc != "co**de** x" {
		t.Fatalf("代码内粗体应先解除代码: %+v", res)
	}
}

func TestInlineAdjacentMarkerGuard(t *testing.T) {
	doc := "a**b**c"
	// 在 ** 紧邻处插入式包裹会产生歧义 run：放弃。
	res := applyInlineToLines(doc, 0, 0, 1, 2, spanBold)
	if res.changed && strings.Contains(res.doc, "****") {
		t.Fatalf("不应产生歧义 run: %+v", res)
	}
}

func TestSetHeadingToggle(t *testing.T) {
	doc := "# 标题\n\n正文段落"
	// H1 → 正文。
	res := applyHeadingToLines(doc, 0, 0, 2, 0)
	if res.doc != "标题\n\n正文段落" {
		t.Fatalf("同级标题还原正文: %+v", res)
	}
	// 正文 → H2。
	res2 := applyHeadingToLines(res.doc, 0, 0, 0, 2)
	if res2.doc != "## 标题\n\n正文段落" {
		t.Fatalf("设为 H2: %+v", res2)
	}
	// H2 → H3（不同级别直接改）。
	res3 := applyHeadingToLines(res2.doc, 0, 0, 0, 3)
	if res3.doc != "### 标题\n\n正文段落" {
		t.Fatalf("H2 → H3: %+v", res3)
	}
	// 光标收敛到新标记之后（原光标在前缀内）。
	if res3.selStart != runeLen("### ") {
		t.Fatalf("光标应收敛到标记后: %d", res3.selStart)
	}
}

func TestSetHeadingEmptyLineUnchanged(t *testing.T) {
	doc := "# a\n\nb"
	res := applyHeadingToLines(doc, 1, 1, 1, 2)
	if res.changed {
		t.Fatalf("空行设标题应无变化: %+v", res)
	}
}

func TestPrefixQuoteToggle(t *testing.T) {
	doc := "第一行\n第二行\n第三行"
	res := applyPrefixToLines(doc, 0, 1, 2, prefixQuote)
	if res.doc != "> 第一行\n> 第二行\n第三行" {
		t.Fatalf("引用所选行: %+v", res)
	}
	res2 := applyPrefixToLines(res.doc, 0, 1, 2, prefixQuote)
	if res2.doc != doc {
		t.Fatalf("移除引用保留文本: %+v", res2)
	}
}

func TestPrefixListKinds(t *testing.T) {
	doc := "买牛奶\n写周报"
	res := applyPrefixToLines(doc, 0, 1, 0, prefixBullet)
	if res.doc != "- 买牛奶\n- 写周报" {
		t.Fatalf("无序列表: %+v", res)
	}
	res2 := applyPrefixToLines(res.doc, 0, 1, 0, prefixOrdered)
	if res2.doc != "1. 买牛奶\n2. 写周报" {
		t.Fatalf("有序列表编号: %+v", res2)
	}
	res3 := applyPrefixToLines(res2.doc, 0, 1, 0, prefixTask)
	if res3.doc != "- [ ] 买牛奶\n- [ ] 写周报" {
		t.Fatalf("任务列表: %+v", res3)
	}
	res4 := applyPrefixToLines(res3.doc, 0, 1, 0, prefixTask)
	if res4.doc != doc {
		t.Fatalf("移除任务标记不丢内容: %+v", res4)
	}
}

func TestPrefixConvertsHeadingToBullet(t *testing.T) {
	doc := "## 标题行"
	res := applyPrefixToLines(doc, 0, 0, 3, prefixBullet)
	if res.doc != "- 标题行" {
		t.Fatalf("标题转列表保留文本: %+v", res)
	}
	if res.selStart != runeLen("- ") {
		t.Fatalf("光标映射（标记差值）: %d", res.selStart)
	}
}

func TestCodeBlockWrapAndUnwrap(t *testing.T) {
	doc := "# 代码\nprint(1)\nprint(2)"
	res := applyCodeBlockToggle(doc, 1, 2, indexOf(doc, "print(1)"))
	if res.doc != "# 代码\n```\nprint(1)\nprint(2)\n```\n" {
		t.Fatalf("包裹所选行: %+v", res)
	}
	if res.selStart != runeLen("# 代码\n```\n") {
		t.Fatalf("光标落到内容首: %d", res.selStart)
	}
	res2 := applyCodeBlockToggle(res.doc, 1, 4, res.selStart)
	if res2.doc != doc {
		t.Fatalf("还原为正文: %+v", res2)
	}
}

func TestCodeBlockWithLanguageFenceUnwraps(t *testing.T) {
	doc := "```go\nfmt.Println()\n```"
	res := applyCodeBlockToggle(doc, 0, 2, 6)
	if res.doc != "fmt.Println()" {
		t.Fatalf("带语言围栏应能还原: %+v", res)
	}
}

func TestInsertRuleAndTable(t *testing.T) {
	doc := "第一段\n\n第二段"
	res := applyInsertAfterLine(doc, 0, "---", 3)
	if res.doc != "第一段\n\n---\n\n第二段" {
		t.Fatalf("分隔线插入当前块之后: %+v", res)
	}
	res2 := applyInsertAfterLine(doc, 0, defaultTableBlock(), 2)
	want := "第一段\n\n" + defaultTableBlock() + "\n\n第二段"
	if res2.doc != want {
		t.Fatalf("表格插入: %+v", res2)
	}
	if res2.selStart != runeLen("第一段\n\n| ") {
		t.Fatalf("光标应落在表头第一格: %d", res2.selStart)
	}
	// 插入的表格必须可被块解析器识别。
	blocks := liveBlocks(defaultTableBlock())
	if len(blocks) != 1 || blocks[0].kind != bkTable || len(blocks[0].table.rows) != 3 {
		t.Fatalf("默认表格两列三行（表头+两行正文）: %+v", blocks)
	}
}

func TestReplaceLineWith(t *testing.T) {
	doc := "a\n\n\nb"
	res := replaceLineWith(doc, 2, "```\n\n```", 4)
	if res.doc != "a\n\n```\n\n```\nb" {
		t.Fatalf("空块原地变代码块: %+v", res)
	}
	if res.selStart != runeLen("a\n\n```\n") {
		t.Fatalf("光标在空代码块内: %d", res.selStart)
	}
}

func TestCaretHelpers(t *testing.T) {
	doc := "中文abc\nde"
	if line, col := caretLineCol(doc, 5); line != 0 || col != 5 {
		t.Fatalf("caretLineCol: %d %d", line, col)
	}
	if line, col := caretLineCol(doc, 7); line != 1 || col != 1 {
		t.Fatalf("caretLineCol 跨行: %d %d", line, col)
	}
	if got := caretFromLineCol(doc, 1, 1); got != 7 {
		t.Fatalf("caretFromLineCol: %d", got)
	}
	// 越界收敛。
	if got := caretFromLineCol(doc, 99, 99); got != runeLen(doc) {
		t.Fatalf("越界收敛: %d", got)
	}
}

func TestInlineMultiLineSourceSelection(t *testing.T) {
	doc := "line one\nline two\nline three"
	s := indexOf(doc, "line one")
	e := indexOf(doc, "line three") + runeLen("line three")
	res := applyInlineToLines(doc, 0, 2, s, e, spanBold)
	want := "**line one**\n**line two**\n**line three**"
	if res.doc != want {
		t.Fatalf("源码跨行选区逐行包裹: %+v", res)
	}
	if res.selStart != runeLen("**") || res.selEnd != runeLen(want)-runeLen("**") {
		t.Fatalf("选区映射: %d..%d", res.selStart, res.selEnd)
	}
}
