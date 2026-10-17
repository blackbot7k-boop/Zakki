package app

import (
	"unicode/utf8"

	"github.com/egoist/mygo/ui"
	"zakki/internal/codehl"
)

// 代码配色方案：id → 浅色/深色两套 token 颜色。Plain 位沿用文字色，
// 不进配色表；数组按 codehl.Kind 下标对齐，零值表示"跟随文本"。
type codePalette [codehl.KindCount]ui.Color

var codeSchemes = map[string][2]codePalette{
	"soft":   {softCodePalette, softCodePaletteDark},
	"vivid":  {vividCodePalette, vividCodePaletteDark},
	"claude": {claudeCodePalette, claudeCodePaletteDark},
	"qoder":  {qoderCodePalette, qoderCodePaletteDark},
}

// Claude 代码配色：Mayoi 同名主题的语法色（Key 取其 Tag 位）。
var claudeCodePalette = codePalette{
	codehl.Comment: ui.Hex("#938d82"),
	codehl.String:  ui.Hex("#5e8a3a"),
	codehl.Number:  ui.Hex("#a85a1f"),
	codehl.Keyword: ui.Hex("#b8573a"),
	codehl.Func:    ui.Hex("#3b6ea8"),
	codehl.Key:     ui.Hex("#bf4d3a"),
}

var claudeCodePaletteDark = codePalette{
	codehl.Comment: ui.Hex("#7d786f"),
	codehl.String:  ui.Hex("#b3c98a"),
	codehl.Number:  ui.Hex("#d4a27f"),
	codehl.Keyword: ui.Hex("#d97757"),
	codehl.Func:    ui.Hex("#8fb6dd"),
	codehl.Key:     ui.Hex("#e8705f"),
}

// Qoder 代码配色：Mayoi 同名主题的语法色（Key 取其 Tag 位）。
var qoderCodePalette = codePalette{
	codehl.Comment: ui.Hex("#848c86"),
	codehl.String:  ui.Hex("#8a5a00"),
	codehl.Number:  ui.Hex("#b85a1f"),
	codehl.Keyword: ui.Hex("#2f7550"),
	codehl.Func:    ui.Hex("#0969a8"),
	codehl.Key:     ui.Hex("#b3434a"),
}

var qoderCodePaletteDark = codePalette{
	codehl.Comment: ui.Hex("#758077"),
	codehl.String:  ui.Hex("#d8c07a"),
	codehl.Number:  ui.Hex("#efb080"),
	codehl.Keyword: ui.Hex("#73cd94"),
	codehl.Func:    ui.Hex("#87c3ff"),
	codehl.Key:     ui.Hex("#e394dc"),
}

// 柔和：低饱和的同系色，浅色底上再压暗一档。
var softCodePalette = codePalette{
	codehl.Comment: ui.Hex("#9aa4ad"),
	codehl.String:  ui.Hex("#5f9e6e"),
	codehl.Number:  ui.Hex("#b08a5a"),
	codehl.Keyword: ui.Hex("#7d92c4"),
	codehl.Func:    ui.Hex("#a98a6b"),
	codehl.Key:     ui.Hex("#c0918a"),
}

var softCodePaletteDark = codePalette{
	codehl.Comment: ui.Hex("#7d8791"),
	codehl.String:  ui.Hex("#93c0a0"),
	codehl.Number:  ui.Hex("#cfae85"),
	codehl.Keyword: ui.Hex("#9db1d8"),
	codehl.Func:    ui.Hex("#c4a98a"),
	codehl.Key:     ui.Hex("#d4a49c"),
}

// 明快：经典 IDE 配色（浅色取 One Light 系，深色取 One Dark 系）。
var vividCodePalette = codePalette{
	codehl.Comment: ui.Hex("#a0a1a7"),
	codehl.String:  ui.Hex("#50a14f"),
	codehl.Number:  ui.Hex("#986801"),
	codehl.Keyword: ui.Hex("#4078f2"),
	codehl.Func:    ui.Hex("#b76b01"),
	codehl.Key:     ui.Hex("#e45649"),
}

var vividCodePaletteDark = codePalette{
	codehl.Comment: ui.Hex("#7f848e"),
	codehl.String:  ui.Hex("#98c379"),
	codehl.Number:  ui.Hex("#d19a66"),
	codehl.Keyword: ui.Hex("#c678dd"),
	codehl.Func:    ui.Hex("#61afef"),
	codehl.Key:     ui.Hex("#e06c75"),
}

// codePaletteFor 解析方案 id 与深浅模式；ok=false 表示跟随文本（不上色）。
func codePaletteFor(id string, t *ui.Theme) (codePalette, bool) {
	variants, ok := codeSchemes[id]
	if !ok {
		return codePalette{}, false
	}
	if themeIsDark(t) {
		return variants[1], true
	}
	return variants[0], true
}

// codeLineSpans 把一行代码切成 RichText 片段。token 区间用方案色，空隙
// 与 Plain 用 plainColor（零值 = 继承元素文字色）。
func codeLineSpans(line string, spans []codehl.Span, cp codePalette, plainColor ui.Color) []ui.Span {
	runes := []rune(line)
	out := make([]ui.Span, 0, len(spans)+2)
	push := func(start, end int, kind codehl.Kind) {
		if end <= start {
			return
		}
		sp := ui.Span{Text: string(runes[start:end])}
		if kind != codehl.Plain {
			sp.Color = cp[kind]
		} else {
			sp.Color = plainColor
		}
		out = append(out, sp)
	}
	prev := 0
	for _, s := range spans {
		push(prev, s.Start, codehl.Plain)
		push(s.Start, s.End, s.Kind)
		prev = s.End
	}
	push(prev, len(runes), codehl.Plain)
	return out
}

// codeSourceRanges 为一行代码生成源码模式着色区间（rune 下标，行内偏移）。
// token 用方案色，其余沿用 muted，保持代码区整体退后的观感。
func codeSourceRanges(line string, spans []codehl.Span, cp codePalette, muted ui.Color) []ui.TextRange {
	out := make([]ui.TextRange, 0, len(spans))
	prev := 0
	for _, s := range spans {
		if s.Start > prev {
			out = append(out, ui.TextRange{Start: prev, End: s.Start, Color: muted})
		}
		out = append(out, ui.TextRange{Start: s.Start, End: s.End, Color: cp[s.Kind]})
		prev = s.End
	}
	if prev < utf8.RuneCountInString(line) {
		out = append(out, ui.TextRange{Start: prev, End: utf8.RuneCountInString(line), Color: muted})
	}
	return out
}
