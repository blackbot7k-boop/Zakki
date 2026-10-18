package app

import "github.com/egoist/mygo/ui"

// inlineStyle 是预览片段与原生编辑区间共用的样式定义；零值属性保留块级继承。
func inlineStyle(kind spanKind, t *ui.Theme, md resolvedStyle) ui.Span {
	var s ui.Span
	switch kind {
	case spanBold:
		s.Weight = 700
		if md.sketch {
			s.Background = sketchMark(md)
		}
	case spanItalic:
		s.Italic = true
	case spanCode:
		s.Font, s.Color = "monospace", t.TextMuted
		if md.sketch {
			s.Background = md.codeBackground
		}
	case spanLink:
		s.Color, s.Underline = md.accent, true
		if md.sketch {
			s.Underline, s.WavyUnderline, s.DecorationColor = false, true, md.accent
		}
	}
	return s
}

func inlineTextRange(s ui.Span) ui.TextRange {
	return ui.TextRange{
		Color: s.Color, Background: s.Background, Weight: s.Weight,
		Font: s.Font, Italic: s.Italic,
		Underline: s.Underline, WavyUnderline: s.WavyUnderline,
		DecorationColor: s.DecorationColor,
	}
}
