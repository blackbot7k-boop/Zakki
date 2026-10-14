package markdown

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
	res := ApplyInlineToLines(doc, 0, 0, start, end, spanBold)
	if !res.Changed || res.Doc != "hello **world**" {
		t.Fatalf("加粗: %+v", res)
	}
	if res.SelStart != runeLen("hello **") || res.SelEnd != runeLen("hello **world") {
		t.Fatalf("选区应包住内容: %d..%d", res.SelStart, res.SelEnd)
	}
	// 再次执行同范围（映射后的选区）→ 移除。
	res2 := ApplyInlineToLines(res.Doc, 0, 0, res.SelStart, res.SelEnd, spanBold)
	if !res2.Changed || res2.Doc != doc {
		t.Fatalf("移除加粗应还原: %+v", res2)
	}
}

func TestToggleInlineChineseAndEmoji(t *testing.T) {
	doc := "中文：你好，世界 🌏 ok"
	start := indexOf(doc, "你好，世界 🌏")
	end := start + runeLen("你好，世界 🌏")
	res := ApplyInlineToLines(doc, 0, 0, start, end, spanItalic)
	if !strings.Contains(res.Doc, "*你好，世界 🌏*") {
		t.Fatalf("中文与 emoji 加斜体: %+v", res)
	}
	res2 := ApplyInlineToLines(res.Doc, 0, 0, res.SelStart, res.SelEnd, spanItalic)
	if res2.Doc != doc {
		t.Fatalf("移除斜体应还原原文: %+v", res2)
	}
}

func TestToggleInlineInsideSpanRemoves(t *testing.T) {
	doc := "前 **加粗内容** 后"
	start := indexOf(doc, "加粗内容")
	end := start + runeLen("加粗内容")
	res := ApplyInlineToLines(doc, 0, 0, start, end, spanBold)
	if !res.Changed || res.Doc != "前 加粗内容 后" {
		t.Fatalf("选区在 span 内应移除格式: %+v", res)
	}
	if res.SelStart != indexOf(res.Doc, "加粗内容") || res.SelEnd != res.SelStart+runeLen("加粗内容") {
		t.Fatalf("移除后选区应回到内容: %d..%d", res.SelStart, res.SelEnd)
	}
}

func TestToggleInlineKeepsLinks(t *testing.T) {
	doc := "看 [Tot](https://tot.rocks) 很好"
	start := indexOf(doc, "[Tot](https://tot.rocks)")
	end := start + runeLen("[Tot](https://tot.rocks)")
	res := ApplyInlineToLines(doc, 0, 0, start, end, spanBold)
	if !strings.Contains(res.Doc, "**[Tot](https://tot.rocks)**") {
		t.Fatalf("加粗应保留链接: %+v", res)
	}
}

func TestInlineCodeExclusive(t *testing.T) {
	// 应用代码：清除选区内粗体，不生成嵌套。
	doc := "**bold** text"
	start, end := 0, runeLen(doc)
	res := ApplyInlineToLines(doc, 0, 0, start, end, spanCode)
	if !strings.Contains(res.Doc, "`") || strings.Contains(res.Doc, "**") {
		t.Fatalf("应用代码应清除粗体: %+v", res)
	}
	// 应用粗体到代码 span：先清代码。
	doc2 := "`code` x"
	s2, e2 := 0, runeLen("`code`")
	res2 := ApplyInlineToLines(doc2, 0, 0, s2, e2, spanBold)
	if strings.Contains(res2.Doc, "`") || !strings.Contains(res2.Doc, "**code**") {
		t.Fatalf("粗体与代码互斥: %+v", res2)
	}
	// 同格式（代码）选区 → 移除。
	res3 := ApplyInlineToLines(doc2, 0, 0, s2, e2, spanCode)
	if res3.Doc != "code x" {
		t.Fatalf("移除代码: %+v", res3)
	}
}

func TestInlinePartialCodeOverlapUnwraps(t *testing.T) {
	// 选区落在代码 span 内部、应用粗体：先解除代码（互斥），再包裹选区，
	// 不生成错误嵌套。
	doc := "`code` x"
	s := runeLen("`co")
	res := ApplyInlineToLines(doc, 0, 0, s, s+2, spanBold)
	if !res.Changed || res.Doc != "co**de** x" {
		t.Fatalf("代码内粗体应先解除代码: %+v", res)
	}
}

func TestInlineAdjacentMarkerGuard(t *testing.T) {
	doc := "a**b**c"
	// 在 ** 紧邻处插入式包裹会产生歧义 run：放弃。
	res := ApplyInlineToLines(doc, 0, 0, 1, 2, spanBold)
	if res.Changed && strings.Contains(res.Doc, "****") {
		t.Fatalf("不应产生歧义 run: %+v", res)
	}
}

func TestSetHeadingToggle(t *testing.T) {
	doc := "# 标题\n\n正文段落"
	// H1 → 正文。
	res := ApplyHeadingToLines(doc, 0, 0, 2, 0)
	if res.Doc != "标题\n\n正文段落" {
		t.Fatalf("同级标题还原正文: %+v", res)
	}
	// 正文 → H2。
	res2 := ApplyHeadingToLines(res.Doc, 0, 0, 0, 2)
	if res2.Doc != "## 标题\n\n正文段落" {
		t.Fatalf("设为 H2: %+v", res2)
	}
	// H2 → H3（不同级别直接改）。
	res3 := ApplyHeadingToLines(res2.Doc, 0, 0, 0, 3)
	if res3.Doc != "### 标题\n\n正文段落" {
		t.Fatalf("H2 → H3: %+v", res3)
	}
	// 光标收敛到新标记之后（原光标在前缀内）。
	if res3.SelStart != runeLen("### ") {
		t.Fatalf("光标应收敛到标记后: %d", res3.SelStart)
	}
}

func TestSetHeadingEmptyLineUnchanged(t *testing.T) {
	doc := "# a\n\nb"
	res := ApplyHeadingToLines(doc, 1, 1, 1, 2)
	if res.Changed {
		t.Fatalf("空行设标题应无变化: %+v", res)
	}
}

func TestPrefixQuoteToggle(t *testing.T) {
	doc := "第一行\n第二行\n第三行"
	res := ApplyPrefixToLines(doc, 0, 1, 2, PrefixQuote)
	if res.Doc != "> 第一行\n> 第二行\n第三行" {
		t.Fatalf("引用所选行: %+v", res)
	}
	res2 := ApplyPrefixToLines(res.Doc, 0, 1, 2, PrefixQuote)
	if res2.Doc != doc {
		t.Fatalf("移除引用保留文本: %+v", res2)
	}
}

func TestPrefixListKinds(t *testing.T) {
	doc := "买牛奶\n写周报"
	res := ApplyPrefixToLines(doc, 0, 1, 0, PrefixBullet)
	if res.Doc != "- 买牛奶\n- 写周报" {
		t.Fatalf("无序列表: %+v", res)
	}
	res2 := ApplyPrefixToLines(res.Doc, 0, 1, 0, PrefixOrdered)
	if res2.Doc != "1. 买牛奶\n2. 写周报" {
		t.Fatalf("有序列表编号: %+v", res2)
	}
	res3 := ApplyPrefixToLines(res2.Doc, 0, 1, 0, PrefixTask)
	if res3.Doc != "- [ ] 买牛奶\n- [ ] 写周报" {
		t.Fatalf("任务列表: %+v", res3)
	}
	res4 := ApplyPrefixToLines(res3.Doc, 0, 1, 0, PrefixTask)
	if res4.Doc != doc {
		t.Fatalf("移除任务标记不丢内容: %+v", res4)
	}
}

func TestPrefixConvertsHeadingToBullet(t *testing.T) {
	doc := "## 标题行"
	res := ApplyPrefixToLines(doc, 0, 0, 3, PrefixBullet)
	if res.Doc != "- 标题行" {
		t.Fatalf("标题转列表保留文本: %+v", res)
	}
	if res.SelStart != runeLen("- ") {
		t.Fatalf("光标映射（标记差值）: %d", res.SelStart)
	}
}

func TestCodeBlockWrapAndUnwrap(t *testing.T) {
	doc := "# 代码\nprint(1)\nprint(2)"
	res := ApplyCodeBlockToggle(doc, 1, 2, indexOf(doc, "print(1)"))
	if res.Doc != "# 代码\n```\nprint(1)\nprint(2)\n```\n" {
		t.Fatalf("包裹所选行: %+v", res)
	}
	if res.SelStart != runeLen("# 代码\n```\n") {
		t.Fatalf("光标落到内容首: %d", res.SelStart)
	}
	res2 := ApplyCodeBlockToggle(res.Doc, 1, 4, res.SelStart)
	if res2.Doc != doc {
		t.Fatalf("还原为正文: %+v", res2)
	}
}

func TestCodeBlockWithLanguageFenceUnwraps(t *testing.T) {
	doc := "```go\nfmt.Println()\n```"
	res := ApplyCodeBlockToggle(doc, 0, 2, 6)
	if res.Doc != "fmt.Println()" {
		t.Fatalf("带语言围栏应能还原: %+v", res)
	}
}

func TestInsertRuleAndTable(t *testing.T) {
	doc := "第一段\n\n第二段"
	res := ApplyInsertAfterLine(doc, 0, "---", 3)
	if res.Doc != "第一段\n\n---\n\n第二段" {
		t.Fatalf("分隔线插入当前块之后: %+v", res)
	}
	res2 := ApplyInsertAfterLine(doc, 0, DefaultTableBlock(), 2)
	want := "第一段\n\n" + DefaultTableBlock() + "\n\n第二段"
	if res2.Doc != want {
		t.Fatalf("表格插入: %+v", res2)
	}
	if res2.SelStart != runeLen("第一段\n\n| ") {
		t.Fatalf("光标应落在表头第一格: %d", res2.SelStart)
	}
}

func TestReplaceLineWith(t *testing.T) {
	doc := "a\n\n\nb"
	res := ReplaceLineWith(doc, 2, "```\n\n```", 4)
	if res.Doc != "a\n\n```\n\n```\nb" {
		t.Fatalf("空块原地变代码块: %+v", res)
	}
	if res.SelStart != runeLen("a\n\n```\n") {
		t.Fatalf("光标在空代码块内: %d", res.SelStart)
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
	res := ApplyInlineToLines(doc, 0, 2, s, e, spanBold)
	want := "**line one**\n**line two**\n**line three**"
	if res.Doc != want {
		t.Fatalf("源码跨行选区逐行包裹: %+v", res)
	}
	if res.SelStart != runeLen("**") || res.SelEnd != runeLen(want)-runeLen("**") {
		t.Fatalf("选区映射: %d..%d", res.SelStart, res.SelEnd)
	}
}
