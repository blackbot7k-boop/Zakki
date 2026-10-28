package app

import (
	"testing"
	"unicode/utf8"

	"zakki/internal/markdown"
)

func TestTimeObjectAdjacentInputKeepsTokenBoundary(t *testing.T) {
	for _, suffix := range []string{"", " ~1h", " ~1h ^1d", " ^w@1-5"} {
		for _, input := range []string{"中文", "paste😀\n第二行", "**粗体**", " 有空格"} {
			raw := "正文 @2024-06-10 10:00" + suffix
			p := projectBlock(liveBlocks(raw)[0])
			next := p.text + input
			got, caret := p.replace(next, utf8.RuneCountInString(next))
			want := raw + " " + input
			if input[0] == ' ' {
				want = raw + input
			}
			if got != want {
				t.Fatalf("got %q want %q", got, want)
			}
			if _, ok := markdown.FindTimeToken(got); !ok {
				t.Fatalf("丢失时间令牌 %q", got)
			}
			updated := projectBlock(liveBlocks(got)[0])
			if input != "**粗体**" && caret != utf8.RuneCountInString(got) || input == "**粗体**" && caret != updated.sourceOffset(len(updated.units)) {
				t.Fatalf("caret=%d raw=%q", caret, got)
			}
		}
	}
}
