package app

import (
	"strconv"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

func BenchmarkPreviewSteady(b *testing.B) {
	cases := []struct{ name, doc string }{
		{"code/1000", "```go\n" + strings.Repeat("var x = 123 // hello\n", 1000) + "```"},
		{"code/5000", "```go\n" + strings.Repeat("var x = 123 // hello\n", 5000) + "```"},
		{"table/1000", "|a|b|\n|---|---|\n" + strings.Repeat("|one|two|\n", 1000)},
		{"table/5000", "|a|b|\n|---|---|\n" + strings.Repeat("|one|two|\n", 5000)},
		{"paragraph/100000", strings.Repeat("a", 100000)},
		{"paragraphs/5000", strings.Repeat("hello **world**\n\n", 5000)},
	}
	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			a := benchApp(b, tc.doc)
			tt := ui.NewTester(a.view, 400, 540)
			tt.Frame()
			a.mdActive = -1
			tt.Frame()
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				tt.Frame()
			}
		})
	}
}

// BenchmarkPreviewLocalFonts 选用本机字体时的稳态帧：样式解析要在
// 系统字体列表里核对家族是否仍已安装，列表规模取常见 macOS 的量级。
func BenchmarkPreviewLocalFonts(b *testing.B) {
	families := make([]ui.FontFamily, 0, 800)
	for i := range 797 {
		families = append(families, ui.FontFamily{Name: "Family " + strconv.Itoa(i)})
	}
	families = append(families, ui.FontFamily{Name: "Body Local"}, ui.FontFamily{Name: "Latin Local"}, ui.FontFamily{Name: "Mono Local"})
	cases := []struct{ name, doc string }{
		{"paragraphs/5000", strings.Repeat("hello **world**\n\n", 5000)},
		{"lists/5000", strings.Repeat("- item `code` and *em*\n", 5000)},
	}
	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			a := benchApp(b, tc.doc)
			a.fonts.families = families
			a.prefs.Appearance.BodyFont = "local:Body Local"
			a.prefs.Appearance.LatinFont = "local:Latin Local"
			a.prefs.Appearance.MonoFont = "local:Mono Local"
			tt := ui.NewTester(a.view, 400, 540)
			tt.Frame()
			a.mdActive = -1
			tt.Frame()
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				tt.Frame()
			}
		})
	}
}
