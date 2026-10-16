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
	if res2.SelStart != start || res2.SelEnd != end {
		t.Fatalf("移除后选区应精确包住原内容: %d..%d", res2.SelStart, res2.SelEnd)
	}
}

func TestToggleInlineUnwrapKeepsDocumentSelection(t *testing.T) {
	doc := "前置\n**世界**\n**🌏 ok**\n后置"
	start := indexOf(doc, "世界")
	end := indexOf(doc, "🌏 ok") + runeLen("🌏 ok")
	res := ApplyInlineToLines(doc, 1, 2, start, end, spanBold)
	want := "前置\n世界\n🌏 ok\n后置"
	if !res.Changed || res.Doc != want {
		t.Fatalf("跨行移除加粗: %+v", res)
	}
	wantStart := indexOf(want, "世界")
	wantEnd := indexOf(want, "🌏 ok") + runeLen("🌏 ok")
	if res.SelStart != wantStart || res.SelEnd != wantEnd {
		t.Fatalf("跨行 Unicode 选区映射=%d..%d, want %d..%d", res.SelStart, res.SelEnd, wantStart, wantEnd)
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
	if res2.Doc != "1. 买牛奶\n2. 写周报" || !res2.Changed {
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

func TestPrefixSwitchesOrderedAndBulletKinds(t *testing.T) {
	bullet := ApplyPrefixToLines("1. item", 0, 0, 3, PrefixBullet)
	if !bullet.Changed || bullet.Doc != "- item" {
		t.Fatalf("有序列表切无序列表: %+v", bullet)
	}
	ordered := ApplyPrefixToLines("1. item", 0, 0, 3, PrefixOrdered)
	if !ordered.Changed || ordered.Doc != "item" {
		t.Fatalf("已有有序列表再次切换应移除标记: %+v", ordered)
	}
	zero := ApplyPrefixToLines("0. item", 0, 0, 3, PrefixOrdered)
	if !zero.Changed || zero.Doc != "item" {
		t.Fatalf("零序号也应识别为有序列表: %+v", zero)
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
	if res.Doc != "# 代码\n```\nprint(1)\nprint(2)\n```" {
		t.Fatalf("包裹所选行: %+v", res)
	}
	if res.SelStart != runeLen("# 代码\n```\n") {
		t.Fatalf("光标落到内容首: %d", res.SelStart)
	}
	res2 := ApplyCodeBlockToggle(res.Doc, 1, 4, res.SelStart)
	if res2.Doc != doc {
		t.Fatalf("还原为正文: %+v", res2)
	}
	if res2.SelStart != indexOf(doc, "print(1)") || res2.SelEnd != res2.SelStart {
		t.Fatalf("解除围栏后光标应使用全文 rune 坐标: %+v", res2)
	}
}

func TestCodeBlockWithLanguageFenceUnwraps(t *testing.T) {
	doc := "```go\nfmt.Println()\n```"
	res := ApplyCodeBlockToggle(doc, 0, 2, 6)
	if res.Doc != "fmt.Println()" {
		t.Fatalf("带语言围栏应能还原: %+v", res)
	}
}

func TestCodeBlockWithVariableFenceUnwraps(t *testing.T) {
	for _, doc := range []string{
		"````go\n```\nvalue\n````",
		"~~~~go\n~~~\nvalue\n~~~~",
	} {
		res := ApplyCodeBlockToggle(doc, 0, docLineCount(doc)-1, 0)
		if res.Doc != strings.Join(strings.Split(doc, "\n")[1:docLineCount(doc)-1], "\n") {
			t.Fatalf("可变围栏应按开栏字符与长度还原: %q -> %q", doc, res.Doc)
		}
	}
}

func TestCodeBlockTogglePreservesDocumentNewlines(t *testing.T) {
	for _, tc := range []struct {
		name string
		doc  string
		line int
	}{
		{name: "后续段落", doc: "before\nselected\nafter", line: 1},
		{name: "后续空行", doc: "before\nselected\n\nafter", line: 1},
		{name: "尾部单换行", doc: "before\nselected\n", line: 1},
		{name: "尾部多个换行", doc: "before\nselected\n\n", line: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			wrapped := ApplyCodeBlockToggle(tc.doc, tc.line, tc.line, 0)
			unwrapped := ApplyCodeBlockToggle(wrapped.Doc, tc.line, tc.line+2, wrapped.SelStart)
			if unwrapped.Doc != tc.doc {
				t.Fatalf("包裹再解除必须逐字还原: got=%q want=%q", unwrapped.Doc, tc.doc)
			}
		})
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

func TestFormatResultsUseDocumentRuneCoordinates(t *testing.T) {
	doc := "前置段\n\n第二段"
	start := indexOf(doc, "第二")
	end := start + runeLen("第二")
	inline := ApplyInlineToLines(doc, 2, 2, start, end, spanBold)
	if inline.SelStart != runeLen("前置段\n\n**") || inline.SelEnd != inline.SelStart+runeLen("第二") {
		t.Fatalf("行内格式选区不是全文 rune 坐标: %+v", inline)
	}
	heading := ApplyHeadingToLines(doc, 2, 2, start, 2)
	if heading.SelStart < runeLen("前置段\n\n## ") {
		t.Fatalf("标题选区不是全文 rune 坐标: %+v", heading)
	}
	prefix := ApplyPrefixToLines(doc, 2, 2, start, PrefixBullet)
	if prefix.SelStart < runeLen("前置段\n\n- ") {
		t.Fatalf("列表选区不是全文 rune 坐标: %+v", prefix)
	}
	fenced := "前置段\n\n```\n第二段\n```"
	codeStart := indexOf(fenced, "第二段")
	unwrapped := ApplyCodeBlockToggle(fenced, 2, 4, codeStart)
	if unwrapped.SelStart != indexOf(unwrapped.Doc, "第二段") || unwrapped.SelEnd != unwrapped.SelStart {
		t.Fatalf("解除围栏光标不是全文 rune 坐标: %+v", unwrapped)
	}
}

func TestLineFormatsSkipFencesAndTables(t *testing.T) {
	doc := "first\n\n```go\n```not a closing fence\nfmt.Println(`code`)\n```\n\n| head | value |\n| --- | --- |\n| **cell** | 1 |\n\nlast"
	protected := "```go\n```not a closing fence\nfmt.Println(`code`)\n```\n\n| head | value |\n| --- | --- |\n| **cell** | 1 |"

	inline := ApplyInlineToLines(doc, 0, 11, 0, runeLen(doc), spanBold)
	if !inline.Changed || !strings.Contains(inline.Doc, protected) ||
		!strings.HasPrefix(inline.Doc, "**first**") || !strings.HasSuffix(inline.Doc, "**last**") {
		t.Fatalf("全文加粗必须跳过围栏与表格: %q", inline.Doc)
	}
	heading := ApplyHeadingToLines(doc, 0, 11, 0, 2)
	if !heading.Changed || !strings.Contains(heading.Doc, protected) ||
		!strings.HasPrefix(heading.Doc, "## first") || !strings.HasSuffix(heading.Doc, "## last") {
		t.Fatalf("全文标题必须跳过围栏与表格: %q", heading.Doc)
	}
	list := ApplyPrefixToLines(doc, 0, 11, 0, PrefixBullet)
	if !list.Changed || !strings.Contains(list.Doc, protected) ||
		!strings.HasPrefix(list.Doc, "- first") || !strings.HasSuffix(list.Doc, "- last") {
		t.Fatalf("全文列表必须跳过围栏与表格: %q", list.Doc)
	}
}

func TestLineFormatsSkipVariableLengthFences(t *testing.T) {
	for _, tc := range []struct {
		name      string
		doc       string
		protected string
	}{
		{
			name:      "四反引号围栏保留内部三反引号",
			doc:       "before\n\n````go\n```\n**code**\n````\n\nafter",
			protected: "````go\n```\n**code**\n````",
		},
		{
			name:      "波浪号围栏保留较短闭栏",
			doc:       "before\n\n~~~~go\n~~~\n**code**\n~~~~\n\nafter",
			protected: "~~~~go\n~~~\n**code**\n~~~~",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			last := docLineCount(tc.doc) - 1
			for name, result := range map[string]DocResult{
				"inline":  ApplyInlineToLines(tc.doc, 0, last, 0, runeLen(tc.doc), spanBold),
				"heading": ApplyHeadingToLines(tc.doc, 0, last, 0, 2),
				"list":    ApplyPrefixToLines(tc.doc, 0, last, 0, PrefixBullet),
			} {
				if !result.Changed || !strings.Contains(result.Doc, tc.protected) {
					t.Fatalf("%s 改写了围栏内容: %q", name, result.Doc)
				}
			}
		})
	}
}

func TestFenceIndentationUsesCommonMarkLimit(t *testing.T) {
	for _, line := range []string{"```go", "   ~~~~ go"} {
		if _, ok := parseFenceOpeningLine(line); !ok {
			t.Fatalf("合法围栏未识别: %q", line)
		}
	}
	for _, line := range []string{"    ```", "\t```", "   \t```"} {
		if _, ok := parseFenceOpeningLine(line); ok {
			t.Fatalf("过度缩进围栏被识别: %q", line)
		}
	}
	opening, ok := parseFenceOpeningLine("```")
	if !ok {
		t.Fatal("测试开栏未识别")
	}
	for _, line := range []string{"```", "   ````  \t"} {
		if !isFenceClosingLine(line, opening) {
			t.Fatalf("合法闭栏未识别: %q", line)
		}
	}
	for _, line := range []string{"    ```", "\t```"} {
		if isFenceClosingLine(line, opening) {
			t.Fatalf("过度缩进闭栏被识别: %q", line)
		}
	}
}

func TestLineFormatsKeepCodeProtectedAfterOverIndentedFence(t *testing.T) {
	for _, overIndentedClose := range []string{"    ```", "\t```"} {
		doc := "before\n\n```\ncode\n" + overIndentedClose + "\nstill code\n```\n\nafter"
		protected := "```\ncode\n" + overIndentedClose + "\nstill code\n```"
		last := docLineCount(doc) - 1
		for name, result := range map[string]DocResult{
			"inline":  ApplyInlineToLines(doc, 0, last, 0, runeLen(doc), spanBold),
			"heading": ApplyHeadingToLines(doc, 0, last, 0, 2),
			"list":    ApplyPrefixToLines(doc, 0, last, 0, PrefixBullet),
		} {
			if !result.Changed || !strings.Contains(result.Doc, protected) {
				t.Fatalf("%s 把过度缩进闭栏后的代码暴露为正文: %q", name, result.Doc)
			}
		}
	}
}

func TestLineFormatsProtectIndentedTableWithoutTrailingNewline(t *testing.T) {
	doc := "before\n  | head | value |\n  | --- | --- |\n  | **cell** | 1 |\nafter"
	protected := "  | head | value |\n  | --- | --- |\n  | **cell** | 1 |"
	last := docLineCount(doc) - 1
	for name, result := range map[string]DocResult{
		"inline":  ApplyInlineToLines(doc, 0, last, 0, runeLen(doc), spanBold),
		"heading": ApplyHeadingToLines(doc, 0, last, 0, 2),
		"list":    ApplyPrefixToLines(doc, 0, last, 0, PrefixBullet),
	} {
		if !result.Changed || !strings.Contains(result.Doc, protected) {
			t.Fatalf("%s 改写了缩进表格: %q", name, result.Doc)
		}
	}
}

func TestInlineFormatWithinSingleTableCell(t *testing.T) {
	doc := "  | hello | world |\n  | --- | --- |\n  | row | value |"
	start := runeLen("  | ")
	end := start + runeLen("hello")
	for _, tc := range []struct {
		name   string
		kind   spanKind
		want   string
		marker int
	}{
		{name: "粗体", kind: spanBold, want: "  | **hello** | world |\n  | --- | --- |\n  | row | value |", marker: 2},
		{name: "斜体", kind: spanItalic, want: "  | *hello* | world |\n  | --- | --- |\n  | row | value |", marker: 1},
		{name: "行内代码", kind: spanCode, want: "  | `hello` | world |\n  | --- | --- |\n  | row | value |", marker: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := ApplyInlineToLines(doc, 0, 0, start, end, tc.kind)
			if !result.Changed || result.Doc != tc.want {
				t.Fatalf("单元格格式化 = %q, changed=%v", result.Doc, result.Changed)
			}
			if result.SelStart != start+tc.marker || result.SelEnd != end+tc.marker {
				t.Fatalf("单元格格式化选区 = %d..%d", result.SelStart, result.SelEnd)
			}
		})
	}
}

func TestInlineFormatAcrossTableCellsIsRejected(t *testing.T) {
	doc := "  | hello | world |\n  | --- | --- |\n  | row | value |"
	start := runeLen("  | ")
	end := start + runeLen("hello | world")

	result := ApplyInlineToLines(doc, 0, 0, start, end, spanBold)
	if result.Changed || result.Doc != doc {
		t.Fatalf("跨单元格格式化不应改写表格: %q", result.Doc)
	}
}
