package codehl

import "testing"

func TestLang(t *testing.T) {
	cases := []struct {
		info, want string
	}{
		{"go", "go"}, {"golang", "go"}, {"GO", "go"}, {".go", "go"},
		{"js", "js"}, {"typescript", "ts"}, {"python3", "py"}, {"bash", "sh"},
		{"json5", "json"}, {"yml", "yaml"}, {"c++", "c"}, {"kotlin", "java"},
		{"go title extra", "go"},
		{"", ""}, {"plaintext", ""}, {"note title", ""},
	}
	for _, tc := range cases {
		if got := Lang(tc.info); got != tc.want {
			t.Errorf("Lang(%q) = %q, want %q", tc.info, got, tc.want)
		}
	}
}

// spansOf 把结果压成 (文本, 类别) 对，断言可读。
func spansOf(info, line string) []Span { return Line(info, line) }

func TestLineGo(t *testing.T) {
	spans := spansOf("go", `func main() { // 入口`)
	want := []Span{
		{0, 4, Keyword},   // func
		{5, 9, Func},      // main
		{14, 19, Comment}, // // 入口
	}
	if len(spans) != len(want) {
		t.Fatalf("got %v, want %v", spans, want)
	}
	for i := range want {
		if spans[i] != want[i] {
			t.Fatalf("span %d = %v, want %v (all: %v)", i, spans[i], want[i], spans)
		}
	}
}

func TestLineStringsAndNumbers(t *testing.T) {
	spans := spansOf("go", `s := "a\"b://x" + 'r' + 42 + 1.5e-3 + 0xFF`)
	var kinds []Kind
	for _, sp := range spans {
		kinds = append(kinds, sp.Kind)
	}
	// "a\"b://x" 是完整字符串，:// 不当注释；42/1.5e-3/0xFF 是数字。
	want := []Kind{String, String, Number, Number, Number}
	if len(kinds) != len(want) {
		t.Fatalf("kinds %v, want %v (spans: %v)", kinds, want, spans)
	}
	for i := range want {
		if kinds[i] != want[i] {
			t.Fatalf("kind %d = %v, want %v (spans: %v)", i, kinds[i], want[i], spans)
		}
	}
}

func TestLineJSON(t *testing.T) {
	spans := spansOf("json", `"name": "zakki", "count": 42, ok: true`)
	var keys, strs int
	for _, sp := range spans {
		switch sp.Kind {
		case Key:
			keys++
		case String:
			strs++
		case Number, Keyword:
		default:
			t.Fatalf("unexpected span %v", sp)
		}
	}
	if keys != 3 || strs != 1 {
		t.Fatalf("keys=%d strs=%d spans=%v", keys, strs, spans)
	}
}

func TestLineYAMLListKey(t *testing.T) {
	spans := spansOf("yaml", `- port: 8080 # 注释`)
	want := []Span{{2, 6, Key}, {8, 12, Number}, {13, 17, Comment}}
	if len(spans) != len(want) {
		t.Fatalf("got %v, want %v", spans, want)
	}
	for i := range want {
		if spans[i] != want[i] {
			t.Fatalf("span %d = %v, want %v", i, spans[i], want[i])
		}
	}
}

func TestLineUnknownLangGeneric(t *testing.T) {
	spans := spansOf("", `talk "hi" # 注释 42`)
	var kinds []Kind
	for _, sp := range spans {
		kinds = append(kinds, sp.Kind)
	}
	want := []Kind{String, Comment}
	if len(kinds) != len(want) {
		t.Fatalf("kinds %v, want %v (spans %v)", kinds, want, spans)
	}
}

func TestLineShebang(t *testing.T) {
	spans := spansOf("sh", "#!/bin/sh")
	if len(spans) != 1 || spans[0].Kind != Comment || spans[0].Start != 0 || spans[0].End != 9 {
		t.Fatalf("shebang not whole-line comment: %v", spans)
	}
}

func TestLineRuneIndices(t *testing.T) {
	// 中文按 1 个 rune 计，区间必须落在 rune 边界。
	spans := spansOf("go", `s := "中文" // 注释`)
	for _, sp := range spans {
		if sp.Kind == String && sp.Start != 5 {
			t.Fatalf("string span %v, want start 5 (5 个 rune 前缀)", sp)
		}
		if sp.Kind == Comment && sp.End != 15 {
			t.Fatalf("comment span %v, want end 15", sp)
		}
	}
}

func TestLinePyDef(t *testing.T) {
	spans := spansOf("py", "def run(name):")
	if len(spans) != 2 || spans[0].Kind != Keyword || spans[0].Start != 0 ||
		spans[1].Kind != Func || spans[1].Start != 4 || spans[1].End != 7 {
		t.Fatalf("py spans = %v", spans)
	}
}

func TestLineNoOverlap(t *testing.T) {
	for _, line := range []string{
		`x := foo(1) + bar("s") // c`, `- k: v`, `{"a": 1}`, `if (a == 1) { return }`,
	} {
		spans := spansOf("go", line)
		for i := 1; i < len(spans); i++ {
			if spans[i].Start < spans[i-1].End {
				t.Fatalf("%q overlapping spans: %v", line, spans)
			}
		}
	}
}
