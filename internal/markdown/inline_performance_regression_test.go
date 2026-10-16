package markdown

import (
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestInlineSpansUnmatchedOpenBracketScalesLinearly(t *testing.T) {
	const smallSize = 20_000
	smallInput := strings.Repeat("[", smallSize)
	largeInput := strings.Repeat("[", 8*smallSize)

	measureBest := func(input string, runs int) time.Duration {
		best := time.Duration(1<<63 - 1)
		for range runs {
			runtime.GC()
			started := time.Now()
			if spans := inlineSpans(input); len(spans) != 0 {
				t.Fatalf("未闭合链接不应产生 span: %d", len(spans))
			}
			if elapsed := time.Since(started); elapsed < best {
				best = elapsed
			}
		}
		return best
	}

	small := measureBest(smallInput, 5)
	large := measureBest(largeInput, 3)
	// 输入扩大 8 倍时，线性实现约扩大 8 倍；24 倍阈值给调度和 GC 留出余量，
	// 但仍能稳定挡住理论上扩大约 64 倍的逐个向后扫描。
	if large > small*24 {
		t.Fatalf("160k 未闭合链接解析退化: 20k=%v 160k=%v 比值=%.1f", small, large, float64(large)/float64(small))
	}
}

func BenchmarkInlineSpansUnmatchedOpenBracket160K(b *testing.B) {
	input := strings.Repeat("[", 160_000)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		inlineSpans(input)
	}
}
