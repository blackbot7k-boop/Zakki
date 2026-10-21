package fonts

import "testing"

func TestClassifyFamily(t *testing.T) {
	cases := map[string]Script{
		"PingFang SC":          ScriptChinese,
		"PingFang HK":          ScriptChinese,
		"Songti TC":            ScriptChinese,
		"STHeiti":              ScriptChinese,
		"Hiragino Sans GB":     ScriptChinese,
		"LXGW WenKai Lite":     ScriptChinese,
		"Source Han Serif SC":  ScriptChinese,
		"Noto Sans CJK SC":     ScriptChinese,
		"Microsoft YaHei UI":   ScriptChinese,
		"Sarasa Mono SC":       ScriptChinese,
		"Hannotate SC":         ScriptChinese,
		"霞鹜文楷":                 ScriptChinese,
		"Hiragino Sans":        ScriptOther,
		"Noto Sans CJK JP":     ScriptOther,
		"ヒラギノ角ゴシック":            ScriptOther,
		"Apple SD Gothic Neo":  ScriptOther,
		"Noto Sans Devanagari": ScriptOther,
		"Apple Color Emoji":    ScriptSymbol,
		"Zapf Dingbats":        ScriptSymbol,
		"Segoe UI Symbol":      ScriptSymbol,
		"JetBrains Mono":       ScriptLatin,
		"Helvetica Neue":       ScriptLatin,
		"STIXGeneral":          ScriptLatin,
		"Fira Code":            ScriptLatin,
		"Dank Mono":            ScriptLatin,
		"SF Pro Text":          ScriptLatin,
		"Noteworthy":           ScriptLatin,
	}
	for name, want := range cases {
		if got := ClassifyFamily(name); got != want {
			t.Errorf("ClassifyFamily(%q) = %d，期望 %d", name, got, want)
		}
	}
}
