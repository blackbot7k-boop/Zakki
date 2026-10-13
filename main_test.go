package main

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

// persistOffState 在测试里关闭磁盘写入。
func persistOffState(t *testing.T) {
	t.Helper()
	persistOff = true
	t.Cleanup(func() { persistOff = false })
}

// TestView 以无窗口方式跑视图：点切换、字数统计、预览与待办勾选。
func TestView(t *testing.T) {
	persistOffState(t)

	a := newApp()
	a.dots[0] = "hello world"
	tt := ui.NewTester(a.view, 400, 540)

	if !tt.HasText("点 1") {
		t.Errorf("初始应显示点 1，texts %q", tt.Texts())
	}
	if !tt.HasText("1 行 · 2 词 · 11 字符") {
		t.Errorf("应显示行/词/字符统计，texts %q", tt.Texts())
	}
	if !tt.HasText("尚未备份") {
		t.Errorf("无备份时应显示尚未备份，texts %q", tt.Texts())
	}

	// 点 1 的圆点按钮（Label 为“点 1”）与状态栏文案都含“点 1”，
	// 点击用 Label 精确命中的是按钮。
	if err := tt.Click("点 2"); err != nil {
		t.Fatal(err)
	}
	if a.active != 1 {
		t.Errorf("点击点 2 后 active=%d，应为 1", a.active)
	}
	if !tt.HasText("0 行 · 0 词 · 0 字符") {
		t.Errorf("点 2 为空应显示 0 行，texts %q", tt.Texts())
	}

	// 预览模式 + 待办勾选（底栏预览图标）。
	a.dots[1] = "- [ ] 买牛奶"
	if err := tt.Click("预览"); err != nil {
		t.Fatal(err)
	}
	if !a.cfg.Preview {
		t.Error("点击预览后应进入预览模式")
	}
	if !tt.HasText("买牛奶") {
		t.Errorf("预览应渲染待办文字，texts %q", tt.Texts())
	}
	if err := tt.Click("买牛奶"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(a.dots[1], "[x]") {
		t.Errorf("勾选后源文本应变为 [x]，得到 %q", a.dots[1])
	}

	// 搜索条开合（底栏搜索图标）。
	if err := tt.Click("搜索"); err != nil {
		t.Fatal(err)
	}
	if !a.findOpen {
		t.Error("点击搜索后应打开查找条")
	}
}

// TestMarkdown 覆盖行内解析、块级解析与圆点切换。
func TestMarkdown(t *testing.T) {
	spans := inlineSpans("a **bold** `code` [link](https://x.y) *i*")
	kinds := map[spanKind]bool{}
	for _, sp := range spans {
		kinds[sp.kind] = true
	}
	if !kinds[spanBold] || !kinds[spanCode] || !kinds[spanLink] || !kinds[spanItalic] {
		t.Errorf("行内标记解析不全: %+v", spans)
	}

	blocks := parseBlocks("# 标题\n\n正文一段\n\n- 列表\n- [ ] 待办\n> 引用\n```\ncode\n```\n---\n")
	var got []blockKind
	for _, b := range blocks {
		got = append(got, b.kind)
	}
	want := []blockKind{bkHeading, bkPara, bkList, bkTask, bkQuote, bkCode, bkRule}
	if len(got) != len(want) {
		t.Fatalf("块数 %d != %d: %+v", len(got), len(want), blocks)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("块 %d = %v, 要 %v", i, got[i], want[i])
		}
	}

	// 圆点插入与取消。
	s, caret := toggleBulletPrefix("abc", 1)
	if s != "- abc" || caret != 3 {
		t.Errorf("插入圆点: %q caret=%d", s, caret)
	}
	s2, _ := toggleBulletPrefix(s, 3)
	if s2 != "abc" {
		t.Errorf("取消圆点: %q", s2)
	}

	// 待办翻转。
	if got := toggleTaskLine("- [ ] a\n- [x] b", 0); !strings.Contains(got, "[x] a") {
		t.Errorf("翻转第 0 行: %q", got)
	}
}

// TestEditorRanges 检查高亮区间互不重叠且有序。
func TestEditorRanges(t *testing.T) {
	text := "# 标题\n\n**bold** 和 [link](https://x.y)\n- [ ] 待办 title\n```go\ncode here\n```\n"
	ranges := editorRanges(text, "待办", &ui.Theme{}, ui.Hex("#d4253a"))
	for i := 1; i < len(ranges); i++ {
		if ranges[i].Start < ranges[i-1].End {
			t.Fatalf("区间重叠: %+v", ranges)
		}
	}
	if len(ranges) == 0 {
		t.Fatal("应至少有搜索命中的区间")
	}
}

// TestFindMatches 验证大小写不敏感搜索。
func TestFindMatches(t *testing.T) {
	ms := findMatches("Go go GOlang", "go")
	if len(ms) != 3 {
		t.Fatalf("应命中 3 处，得到 %d: %+v", len(ms), ms)
	}
	if findMatches("abc", "") != nil {
		t.Error("空查询应无命中")
	}
}
