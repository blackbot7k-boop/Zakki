package main

// 块切分、源码高亮与搜索的边界测试。
// 行内解析与格式变换的纯函数测试在 internal/markdown。
// 不碰持久化与真实用户目录。语义以现实现为准锁定。

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

// TestInlineSpansEdge 行内解析边界：连续标记、未闭合、代码内标记、链接畸形、嵌套。

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
