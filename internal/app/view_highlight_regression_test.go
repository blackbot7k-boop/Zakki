package app

import (
	"fmt"
	"math/rand"
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/egoist/mygo/ui"
)

func TestEditorRangeOccupancyBoundaries(t *testing.T) {
	for _, total := range []int{0, 1, 63, 64, 65, 127, 128, 129, 257} {
		t.Run(fmt.Sprint(total), func(t *testing.T) {
			o := editorRangeOccupancy{total: total, words: make([]uint64, (total+63)/64)}
			var accepted [][2]int
			check := func(start, end int) {
				t.Helper()
				want := start >= 0 && start < end && end <= total
				for _, a := range accepted {
					if start < a[1] && a[0] < end {
						want = false
					}
				}
				if got := o.add(start, end); got != want {
					t.Fatalf("add(%d, %d) = %v, want %v; accepted %v", start, end, got, want, accepted)
				}
				if want {
					accepted = append(accepted, [2]int{start, end})
				}
			}
			for _, r := range [][2]int{{-1, 1}, {0, 0}, {1, 0}, {0, total + 1}, {64, 65}, {0, 129}, {0, 64}, {65, 128}, {128, 129}, {63, 65}, {0, total}, {total - 1, total}} {
				check(r[0], r[1])
			}
			rng := rand.New(rand.NewSource(20260604))
			for i := 0; i < 1000; i++ {
				check(rng.Intn(total+4)-1, rng.Intn(total+4)-1)
			}
		})
	}
}

func TestEditorRangesExactScanOracle(t *testing.T) {
	theme := &ui.Theme{TextMuted: ui.Hex("#777777")}
	accent := ui.Hex("#cc3333")
	fixtures := []string{
		"", "中文🙂é\n", "# 标题 **粗体**\n> 引用 `代码`\n- [x] 完成 [链接](url)",
		"```go\nvar 中文 = 123 // hello🙂\nvar x = \"é\"\n```\n**尾部🙂**",
		strings.Repeat("字", 63) + "🙂**粗体**\n" + strings.Repeat("a", 64) + "`code`",
		"**粗体** [链接](目标) `code`\n末行", "```\n\n```\n", "# A\r\n> B\r\n",
		"```go\n" + strings.Repeat("var x = 123 // hello\n", 30) + "```",
	}
	lines := []string{"# 标题🙂", "> 引用 **粗体**", "- [ ] 事项中文", "1. [x] done", "**粗体** `代码` [链接](url)", "```go", "```python", "```", "var x = 123 // hello", "var 中文 = \"🙂\"", "print('é') # 注释", "\t缩进🙂", "", "普通尾部"}
	rng := rand.New(rand.NewSource(20260604))
	for i := 0; i < 100; i++ {
		var doc []string
		for j, n := 0, 1+rng.Intn(50); j < n; j++ {
			doc = append(doc, lines[rng.Intn(len(lines))])
		}
		fixtures = append(fixtures, strings.Join(doc, "\n"))
	}
	for i, text := range fixtures {
		queries := []string{"", "不存在", "🙂", "var", "**", "\n", "hello\nvar", "🙂\n", "A\r\n>"}
		runes := []rune(text)
		for j := 0; j < 5 && len(runes) > 0; j++ {
			start := rng.Intn(len(runes))
			end := min(len(runes), start+1+rng.Intn(80))
			queries = append(queries, string(runes[start:end]))
		}
		for p, palette := range [][]codePalette{nil, {{}}, {softCodePalette}, {softCodePaletteDark}} {
			for _, query := range queries {
				t.Run(fmt.Sprintf("doc%d/palette%d/query%q", i, p, query), func(t *testing.T) {
					got := editorRanges(text, query, theme, accent, palette...)
					want := editorRangesScanOracle(text, query, theme, accent, palette...)
					if !reflect.DeepEqual(got, want) {
						t.Fatalf("text %q\ngot  %#v\nwant %#v", text, got, want)
					}
					for j, tr := range got {
						if tr.Start < 0 || tr.Start >= tr.End || tr.End > utf8.RuneCountInString(text) || (j > 0 && got[j-1].End > tr.Start) {
							t.Fatalf("invalid or overlapping output: %#v", got)
						}
					}
				})
			}
		}
	}
}

func TestEditorRangesFenceSemantics(t *testing.T) {
	theme := &ui.Theme{TextMuted: ui.Hex("#777777")}
	accent := ui.Hex("#cc3333")
	cases := []struct {
		name string
		text string
	}{
		{
			name: "波浪线围栏",
			text: "~~~go\npackage main\n~~~\n# tail",
		},
		{
			name: "长围栏包含短 marker",
			text: "````go\n```\npackage main\n````\n# tail",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ranges := editorRanges(tc.text, "", theme, accent)
			lineStart := 0
			for _, line := range strings.Split(tc.text, "\n") {
				lineEnd := lineStart + utf8.RuneCountInString(line)
				wantColor, wantWeight := theme.TextMuted, 0
				if line == "# tail" {
					wantColor, wantWeight = accent, 700
				}
				found := false
				for _, tr := range ranges {
					if tr.Start == lineStart && tr.End == lineEnd && tr.Color == wantColor && tr.Weight == wantWeight {
						found = true
						break
					}
				}
				if !found {
					t.Fatalf("行 %q 未按预期着色 color=%v weight=%d: %#v", line, wantColor, wantWeight, ranges)
				}
				lineStart = lineEnd + 1
			}
		})
	}
}

func TestEditorRangesSearchRejectsWholeHeading(t *testing.T) {
	theme := &ui.Theme{TextMuted: ui.Hex("#777777")}
	accent := ui.Hex("#cc3333")
	text := "# 中文🙂\n# 次行"
	query := "🙂\n#"
	got := editorRanges(text, query, theme, accent, softCodePalette)
	want := []ui.TextRange{searchHit(4, 7, accent)}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}
