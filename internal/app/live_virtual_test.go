package app

import (
	"fmt"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestVirtWindow(t *testing.T) {
	h := func(int) float32 { return 10 }
	for _, c := range []struct {
		top, bottom         float32
		wantFirst, wantLast int
	}{
		{-50, 25, 0, 3},
		{35, 60, 3, 6},
		{990, 2000, 99, 100},
		{2000, 3000, 100, 100},
	} {
		first, last := virtWindow(100, h, c.top, c.bottom)
		if first != c.wantFirst || last != c.wantLast {
			t.Errorf("virtWindow(%v,%v) = [%d,%d), want [%d,%d)", c.top, c.bottom, first, last, c.wantFirst, c.wantLast)
		}
	}
}

func virtCodeDoc(n int) string {
	var b strings.Builder
	b.WriteString("```\n")
	for i := range n {
		fmt.Fprintf(&b, "line %d\n", i)
	}
	b.WriteString("```")
	return b.String()
}

// visibleLine 返回视口内可见的第一条 "line N" 的 N；没有返回 -1。
func visibleLine(tt *ui.Tester, n int, top, bottom float32) int {
	for i := range n {
		if r, ok := tt.Find(fmt.Sprintf("line %d", i)); ok && r.Y >= top && r.Y+r.H <= bottom {
			return i
		}
	}
	return -1
}

func TestVirtualRowsCacheInvalidatesWithGeometryStyle(t *testing.T) {
	base := resolvedStyle{
		bodyFont: "Body", monoFont: "Mono", codeSize: 14, lineHeight: 1.5,
		codeLineHeigh: 1.4, tablePadX: 8, tablePadY: 6,
	}
	cases := []struct {
		name   string
		kind   blockKind
		change func(*resolvedStyle)
	}{
		{"代码字体", bkCode, func(s *resolvedStyle) { s.monoFont = "Other Mono" }},
		{"代码字号", bkCode, func(s *resolvedStyle) { s.codeSize++ }},
		{"代码行高", bkCode, func(s *resolvedStyle) { s.codeLineHeigh += 0.1 }},
		{"表格正文字体", bkTable, func(s *resolvedStyle) { s.bodyFont = "Other Body" }},
		{"表格代码字体", bkTable, func(s *resolvedStyle) { s.monoFont = "Other Mono" }},
		{"表格字号", bkTable, func(s *resolvedStyle) { s.codeSize++ }},
		{"表格行高", bkTable, func(s *resolvedStyle) { s.lineHeight += 0.1 }},
		{"表格横向留白", bkTable, func(s *resolvedStyle) { s.tablePadX++ }},
		{"表格纵向留白", bkTable, func(s *resolvedStyle) { s.tablePadY++ }},
		{"表格排版变体", bkTable, func(s *resolvedStyle) { s.sketch = true }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			v := liveVirtState{}
			v.beginFrame(0)
			old := v.entry(virtualRowsKey("same", tc.kind, base), 1)
			old.record(0, 99)

			changed := base
			tc.change(&changed)
			fresh := v.entry(virtualRowsKey("same", tc.kind, changed), 1)
			if fresh == old {
				t.Fatal("几何样式变化后复用了旧行高缓存")
			}
			if got, want := fresh.height(0, 12), float32(12); got != want {
				t.Fatalf("几何样式变化后行高=%v, want 未实测估值 %v", got, want)
			}
		})
	}
}

func TestVirtualCodeBlockScrollsAndClicks(t *testing.T) {
	a := newTestApp(t)
	t.Cleanup(a.shutdown)
	const n = 3000
	a.tabs[0].Text = virtCodeDoc(n)
	a.invalidateMdCache()
	tt := ui.NewTester(a.view, 400, 540)
	tt.Frame()
	a.mdActive = -1
	tt.Frame()

	if got := visibleLine(tt, 50, 0, 540); got != 0 {
		t.Fatalf("顶部应显示 line 0，得到 %d", got)
	}
	if _, ok := tt.Find(fmt.Sprintf("line %d", n-1)); ok {
		t.Fatal("视口外的末行不应被构建")
	}

	// 滚到中段：视口里的行必须已构建，不能是空白占位。
	for range 40 {
		tt.Scroll(200, 300, 0, 1000)
		tt.Frame()
	}
	mid := -1
	for i := range n {
		if r, ok := tt.Find(fmt.Sprintf("line %d", i)); ok && r.Y >= 40 && r.Y+r.H <= 500 {
			mid = i
			break
		}
	}
	if mid <= 0 {
		t.Fatal("滚动到中段后视口内没有代码行（占位未被替换）")
	}

	// 点击远处的行：光标落到这一行的源码偏移。
	label := fmt.Sprintf("line %d", mid)
	r, _ := tt.Find(label)
	tt.ClickAt(r.X+2, r.Y+r.H/2)
	tt.Frame()
	if a.mdActive != 0 {
		t.Fatalf("点击代码行应聚焦代码块，mdActive=%d", a.mdActive)
	}
	want := len([]rune(strings.SplitN(a.tabs[0].Text, label+"\n", 2)[0]))
	if a.mdCaret < want || a.mdCaret > want+len(label) {
		t.Fatalf("光标 %d 应落在 %q（源码偏移 %d）", a.mdCaret, label, want)
	}

	// 一路滚到底：末行可见，说明占位总高与实际内容一致。
	a.mdActive = -1
	tt.Frame()
	for range 400 {
		tt.Scroll(200, 300, 0, 5000)
		tt.Frame()
	}
	if r, ok := tt.Find(fmt.Sprintf("line %d", n-1)); !ok || r.Y+r.H > 540 || r.Y < 0 {
		t.Fatalf("滚到底后末行应可见: %v %v", r, ok)
	}
}

func TestVirtualTableEditsFarRow(t *testing.T) {
	a := newTestApp(t)
	t.Cleanup(a.shutdown)
	var b strings.Builder
	b.WriteString("|a|b|\n|---|---|\n")
	for i := range 1000 {
		fmt.Fprintf(&b, "|r%d|x|\n", i)
	}
	a.tabs[0].Text = strings.TrimSuffix(b.String(), "\n")
	a.invalidateMdCache()
	tt := ui.NewTester(a.view, 400, 540)
	tt.Frame()
	a.mdActive = -1
	tt.Frame()
	if _, ok := tt.Find("r900"); ok {
		t.Fatal("视口外的表格行不应被构建")
	}

	// 表头算第 0 行，r900 在第 901 行：远离视口的编辑行必须被构建出来。
	a.focusTableCell(0, 901, 0)
	tt.Frame()
	tt.Frame()
	if a.mdActive != 0 || a.mdVisible != "r900" {
		t.Fatalf("应编辑 r900 单元格: active=%d visible=%q", a.mdActive, a.mdVisible)
	}
}
