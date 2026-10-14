package main

// live_syntax.go 纯函数的边界测试（表驱动）。
// 只测纯函数，不碰持久化与真实用户目录。
// 语义一律以现实现为准锁定；发现的偏差在注释里注明归属文件。

import (
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/egoist/mygo/ui"
)

// edgeMuted / edgeTheme：行内样式过滤要求 Color 非零或 Weight 非零，
// 零值主题的 TextMuted 是透明色，行内代码会被过滤掉（见 TestEditorRangesInlineStyleFilter），
// 所以常规断言用带非零 TextMuted 的主题。
var edgeMuted = ui.RGB(128, 128, 128)

func edgeTheme() *ui.Theme { return &ui.Theme{TextMuted: edgeMuted} }

var edgeAccent = ui.RGB(212, 37, 58)

// checkRangeInvariants 校验高亮区间不变式：非空、落在 [0,total]、按起点有序、互不重叠。
func checkRangeInvariants(t *testing.T, ranges []ui.TextRange, total int) {
	t.Helper()
	for i, r := range ranges {
		if r.Start < 0 || r.End > total || r.Start >= r.End {
			t.Fatalf("区间 %d 越界或为空: %+v (total=%d)", i, r, total)
		}
		if i > 0 && r.Start < ranges[i-1].End {
			t.Fatalf("区间 %d 无序或与前一区间重叠: 前 %+v 当前 %+v 全部 %+v", i, ranges[i-1], r, ranges)
		}
	}
}

// ---- inlineSpans ----

type spanWant struct {
	kind spanKind
	src  string // 区间在原文覆盖的子串（含标记符）；唯一出现即锁定精确位置
	href string
}

// TestInlineSpansEdge 行内解析边界：连续标记、未闭合、代码内标记、链接畸形、嵌套。
func TestInlineSpansEdge(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want []spanWant
	}{
		{"连续粗体斜体", "**a***b*", []spanWant{{spanBold, "**a**", ""}, {spanItalic, "*b*", ""}}},
		{"未闭合粗体", "a **b", nil},
		{"未闭合粗体尾星", "x **y** z **", []spanWant{{spanBold, "**y**", ""}}},
		{"未闭合斜体", "a *b", nil},
		{"孤星无匹配", "**", nil},
		{"代码内含星号", "`*not italic*`", []spanWant{{spanCode, "`*not italic*`", ""}}},
		{"代码未闭合", "`abc", nil},
		{"多个代码段", "`a` `b`", []spanWant{{spanCode, "`a`", ""}, {spanCode, "`b`", ""}}},
		// 链接：href 在第一个 ')' 处截断，多余的 ')' 是普通字符（子集语义）。
		{"链接含右括号", "[a](u(v))", []spanWant{{spanLink, "[a](u(v)", "u(v"}}},
		{"链接中文", "[中文](https://例.中/路径)", []spanWant{{spanLink, "[中文](https://例.中/路径)", "https://例.中/路径"}}},
		{"链接缺右括号", "[a](b", nil},
		{"链接缺左括号", "[a]b", nil},
		{"无中括号", "a](b)", nil},
		{"空文本链接仍成区间", "[](b)", []spanWant{{spanLink, "[](b)", "b"}}},
		{"链接文本内星号", "[*a*](u)", []spanWant{{spanLink, "[*a*](u)", "u"}}},
		// 子集不支持嵌套标记：整段按最外层粗体吃掉，内部 * 原样保留在文本里。
		{"嵌套标记不解析", "**bold *italic* end**", []spanWant{{spanBold, "**bold *italic* end**", ""}}},
		{"下划线斜体", "_i_", []spanWant{{spanItalic, "_i_", ""}}},
		// __ 不是粗体标记（子集只用 **）：最外层两个下划线成普通字符。
		{"双下划线不是粗体", "__i__", []spanWant{{spanItalic, "_i_", ""}}},
		{"多个斜体", "*a*b*c*", []spanWant{{spanItalic, "*a*", ""}, {spanItalic, "*c*", ""}}},
		{"粗体加斜体", "**a** *b*", []spanWant{{spanBold, "**a**", ""}, {spanItalic, "*b*", ""}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rs := []rune(tc.in)
			got := inlineSpans(tc.in)
			if len(got) != len(tc.want) {
				t.Fatalf("inlineSpans(%q) = %+v，要 %d 个区间 %+v", tc.in, got, len(tc.want), tc.want)
			}
			for i, sp := range got {
				w := tc.want[i]
				if sp.kind != w.kind {
					t.Errorf("区间 %d kind=%v，要 %v (%+v)", i, sp.kind, w.kind, sp)
				}
				if sp.start < 0 || sp.end > len(rs) || sp.start >= sp.end {
					t.Fatalf("区间 %d 非法: %+v (len=%d)", i, sp, len(rs))
				}
				if src := string(rs[sp.start:sp.end]); src != w.src {
					t.Errorf("区间 %d 覆盖 %q (rune [%d,%d))，要 %q", i, src, sp.start, sp.end, w.src)
				}
				if sp.href != w.href {
					t.Errorf("区间 %d href=%q，要 %q", i, sp.href, w.href)
				}
				if i > 0 && sp.start < got[i-1].end {
					t.Errorf("区间 %d 与前一区间重叠: %+v %+v", i, got[i-1], sp)
				}
			}
		})
	}
}

// TestSpanTextEdge spanText 去标记符取纯文本。
func TestSpanTextEdge(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"粗体", "**a**", "a"},
		{"星号斜体", "*i*", "i"},
		{"下划线斜体", "_i_", "i"},
		{"代码", "`c`", "c"},
		{"链接", "[t](h)", "t"},
		{"空文本链接", "[](b)", ""},
		{"链接中文", "[中文](u)", "中文"},
		// 嵌套不解析：内部标记符原样保留。
		{"嵌套粗体", "**bold *italic* end**", "bold *italic* end"},
		{"代码内星号", "`*x*`", "*x*"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			spans := inlineSpans(tc.in)
			if len(spans) != 1 {
				t.Fatalf("inlineSpans(%q) = %+v，要恰好 1 个区间", tc.in, spans)
			}
			if got := spanText(tc.in, spans[0]); got != tc.want {
				t.Errorf("spanText(%q) = %q，要 %q", tc.in, got, tc.want)
			}
		})
	}
	// 防御路径：链接区间里没有 ']' 时原样返回（手工构造的畸形区间）。
	if got := spanText("abc", mSpan{start: 0, end: 3, kind: spanLink}); got != "abc" {
		t.Errorf("链接区间缺 ']' 应原样返回，得到 %q", got)
	}
}

// ---- runeIndexOf / runeSeq ----

// TestRuneIndexOf rune 版 Index，from 起向后找。
func TestRuneIndexOf(t *testing.T) {
	cases := []struct {
		name   string
		rs     string
		target rune
		from   int
		want   int
	}{
		{"基本", "abc", 'b', 0, 1},
		{"from 之后才命中", "abcb", 'b', 2, 3},
		{"from 恰在命中处", "ab", 'b', 1, 1},
		{"from 之后无目标", "abc", 'b', 2, -1},
		{"from 等于长度", "abc", 'c', 3, -1},
		{"from 越界", "abc", 'c', 9, -1},
		{"空串", "", 'a', 0, -1},
		{"多字节", "中x中", '中', 1, 2},
	}
	for _, tc := range cases {
		if got := runeIndexOf([]rune(tc.rs), tc.target, tc.from); got != tc.want {
			t.Errorf("%s: runeIndexOf(%q,%q,%d) = %d，要 %d", tc.name, tc.rs, tc.target, tc.from, got, tc.want)
		}
	}
}

// TestRuneSeq 连续 n 个相同 rune 的首个起点。
func TestRuneSeq(t *testing.T) {
	cases := []struct {
		name          string
		rs            string
		target        rune
		n, from, want int
	}{
		{"二连命中", "*a**b", '*', 2, 0, 2},
		// 三个连续时返回首个二连起点（闭合 ** 吃前两个）。
		{"三连取首对", "***", '*', 2, 0, 0},
		{"from 后二连", "****", '*', 2, 2, 2},
		{"只有单个", "a*b*c", '*', 2, 0, -1},
		{"末尾二连", "aa**", '*', 2, 0, 2},
		{"三连目标", "xxyyy", 'y', 3, 0, 2},
		{"空串", "", '*', 2, 0, -1},
		{"from 越界", "**", '*', 2, 2, -1},
	}
	for _, tc := range cases {
		if got := runeSeq([]rune(tc.rs), tc.target, tc.n, tc.from); got != tc.want {
			t.Errorf("%s: runeSeq(%q,%q,%d,%d) = %d，要 %d", tc.name, tc.rs, tc.target, tc.n, tc.from, got, tc.want)
		}
	}
}

// ---- 行级标记 ----

// TestIsHeading 标题识别：# 后必须有空格，最多 6 个 #。
func TestIsHeading(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"# 标题", true},
		{"###### 六级", true},
		{"####### 七个", false}, // 超过 6 个
		{"#a", false},         // # 后无空格
		{"#", false},
		{"", false},
		// 函数只看 trimmed[n] 是否空格；调用方传的是 TrimSpace 结果，
		// "# " 在真实链路里已被裁成 "#"，到不了 true 分支。
		{"# ", true},
		{"#\ta", false}, // tab 不算空格
		{"a # b", false},
		{"##  双空格", true},
	}
	for _, tc := range cases {
		if got := isHeading(tc.in); got != tc.want {
			t.Errorf("isHeading(%q) = %v，要 %v", tc.in, got, tc.want)
		}
	}
}

// TestTaskPrefixLen 待办前缀：四种标记、必须行首、必须有尾空格；大写 [X] 不识别。
func TestTaskPrefixLen(t *testing.T) {
	cases := []struct {
		in   string
		want int
	}{
		{"- [ ] 甲", 6},
		{"* [ ] 甲", 6},
		{"- [x] 甲", 6},
		{"* [x] 甲", 6},
		{"- [ ]  中文", 6},
		{"- [X] 甲", 0},   // 大写不识别
		{"- [ ]", 0},     // 缺尾空格
		{"- [ ]甲", 0},    // 标记后直接接字
		{"x - [ ] 甲", 0}, // 不在行首
		{"", 0},
	}
	for _, tc := range cases {
		if got := taskPrefixLen(tc.in); got != tc.want {
			t.Errorf("taskPrefixLen(%q) = %d，要 %d", tc.in, got, tc.want)
		}
	}
}

// TestListMarkerLen 列表标记（输入须已去左缩进，见嵌套用例）。
func TestListMarkerLen(t *testing.T) {
	cases := []struct {
		in   string
		want int
	}{
		{"- 甲", 2},
		{"* 甲", 2},
		{"- ", 2},
		{"* ", 2},
		{"3. 甲", 3},
		{"12. 甲", 4},
		{"01. 甲", 4},
		{"007. 甲", 5},
		{"100. 甲", 5},
		{"1. ", 3},
		// 数字后多空格：标记只算到第一个空格。
		{"1.  甲", 3},
		{"1.甲", 0},   // '.' 后无空格
		{"1.", 0},    // 行尾没有空格
		{"1 . 甲", 0}, // 空格在 '.' 前
		{"3.中文", 0},  // '.' 后无空格（中文内容也要空格）
		{"3a. 甲", 0}, // 数字前缀外有字符
		{"- 中文", 2},
		// 嵌套缩进：本函数不做 TrimLeft，前导空白直接 0；
		// 调用方（parseLines/editorRanges）传入的是去缩进后的串。
		{"  - 甲", 0},
		{"\t3. 甲", 0},
		{"", 0},
		{"甲", 0},
	}
	for _, tc := range cases {
		if got := listMarkerLen(tc.in); got != tc.want {
			t.Errorf("listMarkerLen(%q) = %d，要 %d", tc.in, got, tc.want)
		}
	}
}

// TestOrderedNumber 有序列表序号。
// 回归锁：orderedNumber("3. item") 曾误返回 0（旧实现按 n-3 取 '.'），
// 现按 n-2 取 '.'，此表锁住该行为。
func TestOrderedNumber(t *testing.T) {
	cases := []struct {
		in   string
		want int
	}{
		{"3. item", 3},   // 回归：曾返回 0
		{"12. item", 12}, // 两位数
		{"01. item", 1},  // 前导 0
		{"007. x", 7},
		{"1. ", 1},
		{"3. 中文内容", 3},
		// "0. " 的序号是 0，与无序列表的 0 无法区分（现实现语义，锁定）。
		{"0. item", 0},
		{"- item", 0},
		{"* item", 0},
		{"1.item", 0}, // 不是列表标记
		{"", 0},
		{"x", 0},
	}
	for _, tc := range cases {
		if got := orderedNumber(tc.in); got != tc.want {
			t.Errorf("orderedNumber(%q) = %d，要 %d", tc.in, got, tc.want)
		}
	}
}

// ---- parseBlocks ----

// TestParseBlocksListMarkers 列表/待办/标题块：序号、大小写 [X]、缩进拍平。
func TestParseBlocksListMarkers(t *testing.T) {
	cases := []struct {
		name    string
		src     string
		kind    blockKind
		level   int
		n       int
		checked bool
		text    string
	}{
		{"有序 3.", "3. item", bkList, 0, 3, false, "item"},
		{"有序 12.", "12. item", bkList, 0, 12, false, "item"},
		{"有序 01.", "01. item", bkList, 0, 1, false, "item"},
		{"有序 0. 与无序同值", "0. item", bkList, 0, 0, false, "item"},
		{"1. 后无空格是段落", "1.item", bkPara, 0, 0, false, "1.item"},
		{"数字后多空格", "1.  甲", bkList, 0, 1, false, "甲"},
		{"无序 -", "- 甲", bkList, 0, 0, false, "甲"},
		{"无序 *", "* 甲", bkList, 0, 0, false, "甲"},
		{"有序中文内容", "3. 中文内容", bkList, 0, 3, false, "中文内容"},
		{"中文顿号不是列表", "一、中文", bkPara, 0, 0, false, "一、中文"},
		// 嵌套缩进：调用方 TrimLeft 前导空白后识别为列表，
		// 无层级信息，深层项与顶层项同级（现实现语义，锁定）。
		{"嵌套缩进拍平", "  - 深层", bkList, 0, 0, false, "深层"},
		{"标题一级", "# 标题", bkHeading, 1, 0, false, "标题"},
		// 标题级别只区分 1-4，5 个以上 # 压到 4；且现实现先压级再截文本，
		// 多余的 '#' 会残留在 text 里（live_blocks.go 的 parseLines 现状，锁定）。
		{"标题级别压到 4", "##### x", bkHeading, 4, 0, false, "# x"},
		{"6 个 # 残留两个 #", "###### x", bkHeading, 4, 0, false, "## x"},
		{"待办未勾", "- [ ] 买牛奶", bkTask, 0, 0, false, "买牛奶"},
		{"待办已勾", "- [x] 买牛奶", bkTask, 0, 0, true, "买牛奶"},
		// 大写 [X] 不识别为待办，退化成普通列表项（现实现语义，锁定）。
		{"大写 X 不是待办", "- [X] 买牛奶", bkList, 0, 0, false, "[X] 买牛奶"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			blocks := parseBlocks(tc.src)
			if len(blocks) != 1 {
				t.Fatalf("parseBlocks(%q) = %+v，要恰好 1 块", tc.src, blocks)
			}
			b := blocks[0]
			if b.kind != tc.kind || b.level != tc.level || b.n != tc.n || b.checked != tc.checked || b.text != tc.text {
				t.Errorf("parseBlocks(%q) = kind=%v level=%d n=%d checked=%v text=%q，要 kind=%v level=%d n=%d checked=%v text=%q",
					tc.src, b.kind, b.level, b.n, b.checked, b.text, tc.kind, tc.level, tc.n, tc.checked, tc.text)
			}
		})
	}
}

// TestParseBlocksCodeFences 代码围栏：未闭合、语言名、围栏内 markdown 逐字、多段。
func TestParseBlocksCodeFences(t *testing.T) {
	cases := []struct {
		name  string
		src   string
		wantK []blockKind
		wantL [][]string
		wantT []string
	}{
		{
			// 未闭合围栏到文档尾（raw 保留全部行）；但现实现把区域末行当
			// 潜在闭栏行，lines 少收最后一行——属于 live_blocks.go 的 parseLines，
			// 不在本次可改范围，此处按现状锁定。
			name: "未闭合围栏", src: "```\nx\ny",
			wantK: []blockKind{bkCode}, wantL: [][]string{{"x"}}, wantT: []string{""},
		},
		{
			name: "未闭合仅一行内容", src: "```\nx",
			wantK: []blockKind{bkCode}, wantL: [][]string{nil}, wantT: []string{""},
		},
		{
			// "```go" 的语言名只留在 raw，块上没有语言字段。
			name: "围栏带语言名", src: "```go\nx\n```",
			wantK: []blockKind{bkCode}, wantL: [][]string{{"x"}}, wantT: []string{""},
		},
		{
			// 围栏内标题/列表/行内标记一律逐字保留，不再解析。
			name: "围栏内 markdown 逐字", src: "```\n# 标题\n- 列表\n**b**\n```",
			wantK: []blockKind{bkCode}, wantL: [][]string{{"# 标题", "- 列表", "**b**"}}, wantT: []string{""},
		},
		{
			name: "围栏内空行不拆块", src: "```\n\nx\n```",
			wantK: []blockKind{bkCode}, wantL: [][]string{{"", "x"}}, wantT: []string{""},
		},
		{
			name: "多段代码块", src: "```\na\n```\n正文\n```\nb\n```",
			wantK: []blockKind{bkCode, bkPara, bkCode},
			wantL: [][]string{{"a"}, nil, {"b"}},
			wantT: []string{"", "正文", ""},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			blocks := parseBlocks(tc.src)
			if len(blocks) != len(tc.wantK) {
				t.Fatalf("parseBlocks(%q) = %+v，要 %d 块", tc.src, blocks, len(tc.wantK))
			}
			for i, b := range blocks {
				if b.kind != tc.wantK[i] {
					t.Errorf("块 %d kind=%v，要 %v", i, b.kind, tc.wantK[i])
				}
				if !reflect.DeepEqual(b.lines, tc.wantL[i]) {
					t.Errorf("块 %d lines=%q，要 %q", i, b.lines, tc.wantL[i])
				}
				if b.text != tc.wantT[i] {
					t.Errorf("块 %d text=%q，要 %q", i, b.text, tc.wantT[i])
				}
			}
		})
	}
}

// TestParseBlocksNestedIndent 缩进列表项与上一项并列，不并块也不带层级。
func TestParseBlocksNestedIndent(t *testing.T) {
	blocks := parseBlocks("- 甲\n  - 乙")
	if len(blocks) != 2 || blocks[0].kind != bkList || blocks[1].kind != bkList {
		t.Fatalf("缩进列表应切两个列表块: %+v", blocks)
	}
	if blocks[0].text != "甲" || blocks[1].text != "乙" {
		t.Errorf("文本 = %q / %q，要 甲 / 乙", blocks[0].text, blocks[1].text)
	}
	if blocks[0].level != 0 || blocks[1].level != 0 {
		t.Errorf("列表层级恒为 0（子集无嵌套）: %d / %d", blocks[0].level, blocks[1].level)
	}
}

// ---- editorRanges ----

// TestEditorRangesInvariants 不变式扫描：区间非空、有序、互不重叠、不出界。
func TestEditorRangesInvariants(t *testing.T) {
	texts := []struct {
		name string
		text string
	}{
		{"空文本", ""},
		{"全代码块", "```\n*x* **y** [z](w)\n```"},
		{"混排", "# 标题\n\n**bold** 和 [link](https://x.y)\n- [ ] 待办\n- 普通\n12. 有序\n> 引用\n`code`\n---\n"},
		{"超长单行", strings.Repeat("**b** `c` [d](e) - i ", 400)},
		{"未闭合围栏", "```\n# 不是标题\n**b**"},
		// \r\n：现实现只按 \n 切行，\r 作为行内容 rune 参与区间（见 TestEditorRangesCRLF）。
		{"CRLF", "a\r\n**b**\r\n- [ ] c\r\n"},
	}
	queries := []string{"", "b", "待办", "x"}
	for _, tx := range texts {
		for _, q := range queries {
			ranges := editorRanges(tx.text, q, edgeTheme(), edgeAccent)
			checkRangeInvariants(t, ranges, utf8.RuneCountInString(tx.text))
		}
	}
}

// TestEditorRangesCodeBlockNoInline 代码块行整行着色，行内标记不高亮。
func TestEditorRangesCodeBlockNoInline(t *testing.T) {
	text := "```\n**a** `b` [c](d)\n```\n"
	got := editorRanges(text, "", edgeTheme(), edgeAccent)
	want := []ui.TextRange{
		{Start: 0, End: 3, Color: edgeMuted},   // 开栏行
		{Start: 4, End: 20, Color: edgeMuted},  // 内容行整行（**a** `b` [c](d) 共 16 rune）
		{Start: 21, End: 24, Color: edgeMuted}, // 闭栏行
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("代码块区间 = %+v，要 %+v", got, want)
	}
}

// TestEditorRangesTaskListMarks 待办标记 3 rune 与列表标记相邻共存。
func TestEditorRangesTaskListMarks(t *testing.T) {
	text := "- [ ] 待办 title\n- 普通\n12. 有序"
	got := editorRanges(text, "", edgeTheme(), edgeAccent)
	want := []ui.TextRange{
		{Start: 0, End: 2, Color: edgeMuted},               // "- " 列表标记
		{Start: 2, End: 5, Color: edgeAccent, Weight: 600}, // "[ ]" 待办标记
		{Start: 15, End: 17, Color: edgeMuted},             // 第二行 "- "
		{Start: 20, End: 24, Color: edgeMuted},             // 第三行 "12. "
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("待办/列表区间 = %+v，要 %+v", got, want)
	}
}

// TestEditorRangesSearchOverridesCode 搜索命中优先于代码块整行着色。
func TestEditorRangesSearchOverridesCode(t *testing.T) {
	got := editorRanges("```\nfoo\n```", "foo", edgeTheme(), edgeAccent)
	want := []ui.TextRange{
		{Start: 0, End: 3, Color: edgeMuted},
		{Start: 4, End: 7, Color: edgeAccent, Weight: 700}, // 搜索命中覆盖整行
		{Start: 8, End: 11, Color: edgeMuted},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("搜索优先区间 = %+v，要 %+v", got, want)
	}
}

// TestEditorRangesInlineStyleFilter 行内样式过滤：斜体不着色；
// 零值主题的 TextMuted 是透明色，行内代码区间被过滤（现状锁定）。
func TestEditorRangesInlineStyleFilter(t *testing.T) {
	if got := editorRanges("*i*", "", edgeTheme(), edgeAccent); len(got) != 0 {
		t.Errorf("斜体不应产生区间，得到 %+v", got)
	}
	if got := editorRanges("`x`", "", &ui.Theme{}, edgeAccent); len(got) != 0 {
		t.Errorf("零值主题下行内代码应被过滤，得到 %+v", got)
	}
	got := editorRanges("`x`", "", edgeTheme(), edgeAccent)
	want := []ui.TextRange{{Start: 0, End: 3, Color: edgeMuted}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("行内代码区间 = %+v，要 %+v", got, want)
	}
	bold := editorRanges("**b**", "", edgeTheme(), edgeAccent)
	wantBold := []ui.TextRange{{Start: 0, End: 5, Weight: 600}}
	if !reflect.DeepEqual(bold, wantBold) {
		t.Errorf("粗体区间 = %+v，要 %+v", bold, wantBold)
	}
}

// TestEditorRangesCRLF \r\n 换行：只按 \n 切行，行级区间把行尾 \r 算进内容。
func TestEditorRangesCRLF(t *testing.T) {
	got := editorRanges("# 标题\r\n", "", edgeTheme(), edgeAccent)
	want := []ui.TextRange{{Start: 0, End: 5, Color: edgeAccent, Weight: 700}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("CRLF 标题区间 = %+v，要 %+v（含行尾 \\r，现实现不剥 \\r）", got, want)
	}
}

// ---- runeToByte ----

// TestRuneToByte rune 下标转字节下标；越界（含负数）收敛到 len(s)。
func TestRuneToByte(t *testing.T) {
	cases := []struct {
		name          string
		s             string
		runeIdx, want int
	}{
		{"空串", "", 0, 0},
		{"空串越界", "", 5, 0},
		{"首字符", "abc", 0, 0},
		{"末尾（等于 rune 数）", "abc", 3, 3},
		{"越界", "abc", 5, 3},
		{"负数", "abc", -1, 3},
		{"多字节中间", "中文", 1, 3},
		{"多字节末尾", "中文", 2, 6}, // 等于 rune 数 → len(s)
		{"混合", "a中b", 2, 4},
		{"全多字节中间", "日本語", 2, 6},
		{"全多字节末尾", "日本語", 3, 9},
	}
	for _, tc := range cases {
		if got := runeToByte(tc.s, tc.runeIdx); got != tc.want {
			t.Errorf("%s: runeToByte(%q,%d) = %d，要 %d", tc.name, tc.s, tc.runeIdx, got, tc.want)
		}
	}
}

// ---- findMatches ----

// TestFindMatchesUnicode 大小写不敏感搜索的 Unicode 边界：
// 返回区间按 rune 计，必须与原文对齐。
func TestFindMatchesUnicode(t *testing.T) {
	cases := []struct {
		name, text, query string
		want              []ui.TextRange
	}{
		{"拉丁大小写", "CAFÉ café", "café", []ui.TextRange{{Start: 0, End: 4}, {Start: 5, End: 9}}},
		{"中文", "中文abc中文", "abc", []ui.TextRange{{Start: 2, End: 5}}},
		{"中文重复", "中文中文", "中文", []ui.TextRange{{Start: 0, End: 2}, {Start: 2, End: 4}}},
		{"emoji 组合序列", "a👩‍💻 b", "👩‍💻", []ui.TextRange{{Start: 1, End: 4}}},
		{"emoji 后偏移", "👩‍💻 hello", "hello", []ui.TextRange{{Start: 4, End: 9}}},
		{"组合字符", "cafe\u0301 latte", "e\u0301", []ui.TextRange{{Start: 3, End: 5}}},
		// 不做 Unicode 归一化：预组合 é 与 e+组合音标互不匹配（保守子集语义，锁定）。
		{"预组合不匹配组合形式", "cafe\u0301", "café", nil},
		{"组合形式不匹配预组合", "café", "cafe\u0301", nil},
		// İ 小写后是 1 个 rune（simple case mapping），rune 偏移不漂移。
		{"大写 İ 偏移不漂移", "İx x", "x", []ui.TextRange{{Start: 1, End: 2}, {Start: 3, End: 4}}},
		{"İ 作查询", "ix", "İ", []ui.TextRange{{Start: 0, End: 1}}},
		{"ß 原样", "straße", "ß", []ui.TextRange{{Start: 4, End: 5}}},
		{"大写 ẞ 对齐", "straße STRAẞE", "ß", []ui.TextRange{{Start: 4, End: 5}, {Start: 11, End: 12}}},
		// 不做全 case folding：ß 不折叠成 ss（保守子集语义，锁定）。
		{"ß 不折叠为 ss", "ß", "ss", nil},
		{"西里尔大小写", "Привет МИР", "мир", []ui.TextRange{{Start: 7, End: 10}}},
		{"命中不重叠", "aaaa", "aa", []ui.TextRange{{Start: 0, End: 2}, {Start: 2, End: 4}}},
		{"尾部残段不命中", "aaa", "aa", []ui.TextRange{{Start: 0, End: 2}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := findMatches(tc.text, tc.query)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("findMatches(%q,%q) = %+v，要 %+v", tc.text, tc.query, got, tc.want)
			}
		})
	}
	// 双向核对：每个命中的 rune 区间切回原文，应与小写后的查询逐 rune 相等
	//（覆盖大小写转换改变字节长度的字符，如 İ、ẞ）。
	for _, tc := range []struct{ text, query string }{
		{"İstanbul İstanbul", "i"},
		{"straße STRAẞE ß", "ß"},
		{"CAFÉ café", "café"},
		{"a👩‍💻👩‍💻b", "👩‍💻"},
	} {
		rs := []rune(tc.text)
		for _, m := range findMatches(tc.text, tc.query) {
			if m.Start < 0 || m.End > len(rs) || m.Start >= m.End {
				t.Fatalf("findMatches(%q,%q) 越界: %+v", tc.text, tc.query, m)
			}
			if got := strings.ToLower(string(rs[m.Start:m.End])); got != strings.ToLower(tc.query) {
				t.Errorf("findMatches(%q,%q) 命中 %+v 切回 %q，小写后要等于查询 %q",
					tc.text, tc.query, m, string(rs[m.Start:m.End]), strings.ToLower(tc.query))
			}
		}
	}
}

// ---- toggleTaskLine ----

// TestToggleTaskLine 待办翻转：越界、只翻第一个、大写 [X] 现状。
func TestToggleTaskLine(t *testing.T) {
	cases := []struct {
		name   string
		src    string
		lineNo int
		want   string
	}{
		{"勾上", "- [ ] a", 0, "- [x] a"},
		{"取消", "- [x] a", 0, "- [ ] a"},
		{"星号待办", "* [ ] a", 0, "* [x] a"},
		{"只改目标行", "- [ ] a\n- [ ] b", 1, "- [ ] a\n- [x] b"},
		// 连续两个 [ ] 只翻第一个（Replace n=1）。
		{"两个待办记号只翻第一个", "- [ ] a [ ] b", 0, "- [x] a [ ] b"},
		// 大写 [X]：既不含 "[ ]" 也不含小写 "[x]"，现状是原样返回（锁定）。
		{"大写 X 不翻转", "- [X] a", 0, "- [X] a"},
		{"行号越界", "- [ ] a", 3, "- [ ] a"},
		{"行号负数", "- [ ] a", -1, "- [ ] a"},
		// Contains 语义：行内任意位置的 "[ ]" 都翻，不限待办前缀。
		{"非待办行也翻", "see [ ] here", 0, "see [x] here"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := toggleTaskLine(tc.src, tc.lineNo); got != tc.want {
				t.Errorf("toggleTaskLine(%q,%d) = %q，要 %q", tc.src, tc.lineNo, got, tc.want)
			}
		})
	}
}
