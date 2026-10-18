package app

import (
	"strings"
	"testing"
)

func BenchmarkStartupMarkdownCache(b *testing.B) {
	for _, tc := range []struct {
		name string
		doc  string
	}{
		{"ordinary", strings.Repeat("## 笔记\n\n今天的 **计划** 和待办内容。\n\n- [ ] 测试启动\n\n", 80)},
		{"paragraphs-1.2MB", strings.Repeat("hello **world** 中文笔记\n\n", 40000)},
	} {
		b.Run(tc.name, func(b *testing.B) {
			a := newTestApp(b)
			a.activeTab().Text = tc.doc
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				a.invalidateMdCache()
				a.mdBlocks()
			}
		})
	}
}
