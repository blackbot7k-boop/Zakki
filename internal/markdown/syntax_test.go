package markdown

import (
	"testing"
)

type spanWant struct {
	kind spanKind
	src  string // 区间在原文覆盖的子串（含标记符）；唯一出现即锁定精确位置
	href string
}

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
				if sp.Kind != w.kind {
					t.Errorf("区间 %d kind=%v，要 %v (%+v)", i, sp.Kind, w.kind, sp)
				}
				if sp.Start < 0 || sp.End > len(rs) || sp.Start >= sp.End {
					t.Fatalf("区间 %d 非法: %+v (len=%d)", i, sp, len(rs))
				}
				if src := string(rs[sp.Start:sp.End]); src != w.src {
					t.Errorf("区间 %d 覆盖 %q (rune [%d,%d))，要 %q", i, src, sp.Start, sp.End, w.src)
				}
				if sp.Href != w.href {
					t.Errorf("区间 %d href=%q，要 %q", i, sp.Href, w.href)
				}
				if i > 0 && sp.Start < got[i-1].End {
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
	if got := spanText("abc", mSpan{Start: 0, End: 3, Kind: spanLink}); got != "abc" {
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
		// 正文中的方框不是任务标记，不能被点击/切换。
		{"非待办行保持原样", "see [ ] here", 0, "see [ ] here"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := toggleTaskLine(tc.src, tc.lineNo); got != tc.want {
				t.Errorf("toggleTaskLine(%q,%d) = %q，要 %q", tc.src, tc.lineNo, got, tc.want)
			}
		})
	}
}
