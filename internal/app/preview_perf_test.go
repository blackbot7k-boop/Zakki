package app

import (
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
