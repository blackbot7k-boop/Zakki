package app

import (
	"fmt"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

var highlightBenchmarkRanges []ui.TextRange

func BenchmarkEditorRangesCodeScaling(b *testing.B) {
	for _, lines := range []int{1000, 2000, 4000, 8000} {
		b.Run(fmt.Sprintf("lines_%d", lines), func(b *testing.B) {
			text := "```go\n" + strings.Repeat("var x = 123 // hello\n", lines) + "```"
			theme := &ui.Theme{TextMuted: ui.Hex("#777777")}
			accent := ui.Hex("#cc3333")
			b.ReportAllocs()
			b.SetBytes(int64(len(text)))
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				highlightBenchmarkRanges = editorRanges(text, "", theme, accent, softCodePalette)
			}
		})
	}
}
