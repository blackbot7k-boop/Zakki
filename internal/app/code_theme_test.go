package app

// 代码配色方案：调色解析、源码模式区间与设置项持久化。

import (
	"testing"

	"github.com/egoist/mygo/ui"
	"zakki/internal/codehl"
)

func TestCodePaletteFor(t *testing.T) {
	light := &ui.Theme{Background: ui.RGB(250, 250, 250)}
	dark := &ui.Theme{Background: ui.RGB(30, 30, 32)}

	if _, ok := codePaletteFor("", light); ok {
		t.Fatal("空 id 应表示跟随文本")
	}
	if _, ok := codePaletteFor("nope", light); ok {
		t.Fatal("未知 id 应表示跟随文本")
	}
	softL, ok := codePaletteFor("soft", light)
	if !ok {
		t.Fatal("soft 应可解析")
	}
	softD, _ := codePaletteFor("soft", dark)
	if softL == softD {
		t.Fatal("浅深两套调色不应相同")
	}
	if softL[codehl.String] == (ui.Color{}) {
		t.Fatal("String 位应有色")
	}
	if softL[codehl.Plain] != (ui.Color{}) {
		t.Fatal("Plain 位应留零（沿用文字色）")
	}
}

func TestEditorRangesCodeScheme(t *testing.T) {
	text := "```go\nfunc main() {}\n```"
	th, accent := edgeTheme(), edgeAccent
	cp, _ := codePaletteFor("vivid", th)
	got := editorRanges(text, "", th, accent, cp)
	if len(got) == 0 {
		t.Fatal("应有区间")
	}
	// 围栏行仍是 muted；内容行应出现非 muted 的 token 色（func 关键字）。
	if got[0].Start != 0 || got[0].Color != th.TextMuted {
		t.Fatalf("首行应为 muted 围栏: %v", got[0])
	}
	colored := false
	for _, tr := range got {
		if tr.Color != th.TextMuted && tr.Color != accent {
			colored = true
		}
	}
	if !colored {
		t.Fatalf("代码 token 应有方案色: %v", got)
	}
	// 跟随文本时与旧路径一致：整行 muted。
	// 跟随文本时与旧路径一致：逐行整行 muted（两根围栏 + 一行内容）。
	plain := editorRanges(text, "", th, accent)
	if len(plain) != 3 {
		t.Fatalf("跟随文本应只有三条整行区间: %v", plain)
	}
}

func TestCodeLineSpansGaps(t *testing.T) {
	cp, _ := codePaletteFor("vivid", &ui.Theme{Background: ui.RGB(250, 250, 250)})
	line := "n := 42"
	spans := codeLineSpans(line, codehl.Line("go", line), cp, ui.Color{})
	if len(spans) == 0 {
		t.Fatal("应有片段")
	}
	var joined string
	for _, sp := range spans {
		joined += sp.Text
	}
	if joined != line {
		t.Fatalf("片段拼回应还原整行: %q", joined)
	}
	if spans[0].Text != "n := " || spans[0].Color != (ui.Color{}) {
		t.Fatalf("空隙应留零继承文字色: %+v", spans[0])
	}
}
