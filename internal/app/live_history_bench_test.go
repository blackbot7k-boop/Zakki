package app

import (
	"strings"
	"testing"
)

func BenchmarkHistoryCompression(b *testing.B) {
	doc := strings.Repeat("x", 1<<20)
	b.Run("anchor-equality", func(b *testing.B) {
		e := liveEdit{after: liveSnapshot{doc: strings.Clone(doc)}}
		b.SetBytes(int64(len(doc)))
		b.ReportAllocs()
		for b.Loop() {
			e.matchesAfter(doc)
		}
	})
	b.Run("compact", func(b *testing.B) {
		e := liveEdit{before: liveSnapshot{doc: doc}, after: liveSnapshot{doc: doc + "字"}, kind: editIME}
		b.ReportAllocs()
		for b.Loop() {
			e.compressed()
		}
	})
}
