package fonts

import (
	"strings"
	"unicode"
)

// Script 字体家族主要面向的文字。
type Script int

const (
	// ScriptLatin 西文字体；名字识别不出的家族一律归入此类。
	ScriptLatin Script = iota
	// ScriptChinese 含中文字形的家族。
	ScriptChinese
	// ScriptOther 日文、韩文及其他非拉丁文字的家族。
	ScriptOther
	// ScriptSymbol 表情、符号、图标等不适合排正文的家族。
	ScriptSymbol
)

// 系统字体接口只给家族名、不给字符覆盖，只能按名字推断：
// 名字里带汉字、地区后缀（SC/TC/HK/GB…）或已知中文字体系列名时归为中文。
var (
	chineseTokens = map[string]bool{
		"sc": true, "tc": true, "hk": true, "cn": true, "gb": true, "gbk": true, "gb18030": true,
		"chs": true, "cht": true, "hans": true, "hant": true, "big5": true,
	}
	otherCJKTokens  = map[string]bool{"jp": true, "jpn": true, "kr": true, "ko": true, "kor": true, "japanese": true, "korean": true}
	chinesePrefixes = []string{
		"pingfang", "songti", "heiti", "kaiti", "fangsong", "yuanti", "libian", "weibei", "wawati", "xingkai",
		"hannotate", "hanzipen", "baoli", "yuppy", "lantinghei", "lanting", "lxgw", "source han", "sarasa",
		"microsoft yahei", "microsoft jhenghei", "simsun", "nsimsun", "simhei", "simkai", "simfang", "dengxian",
		"misans", "alibaba puhuiti", "smiley sans", "zcool", "wenquanyi", "wqy", "ar pl", "cwtex", "mingliu",
		"pmingliu", "dfkai", "biaukai", "apple lisung", "lisong", "lihei", "huawen", "stheiti", "stsong", "stkaiti",
		"stfangsong", "stxihei", "stzhongsong", "stxingkai", "stliti", "sthupo", "stcaiyun", "stxinwei", "stbaoli",
		"styuanti", "fz", "hiragino sans gb", "hiragino sans tc", "harmonyos sans sc", "harmonyos sans tc", "gensen",
		"edukai", "apple ligothic", "simsong", "lingwai",
	}
	// chineseNames 老式 macOS 中文字体的整名，不能按前缀匹配。
	chineseNames  = map[string]bool{"hei": true, "kai": true}
	otherPrefixes = []string{
		"hiragino", "osaka", "yu gothic", "yu mincho", "yugothic", "yumincho", "meiryo", "ms gothic", "ms mincho",
		"ms pgothic", "ms pmincho", "toppan", "klee", "tsukushi", "kyokasho", "ipa", "takao", "biz ud",
		"apple sd gothic", "applegothic", "applemyungjo", "nanum", "malgun", "gulim", "dotum", "batang", "gungsuh",
		"pilgi", "kohinoor", "geeza", "al ", "baghdad", "damascus", "kailasa", "mshtakan", "sana", "bm ", "gungseo",
		"headlinea", "pcmyungjo", "yukyokasho", "beirut", "farah", "farisi", "mishafi", "muna", "nadeem", "waseem",
		"diwan", "decotype", "kufistandard", "kefa", "ayuthaya", "krungthep", "sathu", "silom", "thonburi",
		"sukhumvit", "psl ", "new peninim", "raanana", "inaimathi", "mukta", "kokonor",
	}
	otherTokens = map[string]bool{
		"arabic": true, "hebrew": true, "devanagari": true, "bangla": true, "bengali": true, "tamil": true,
		"telugu": true, "kannada": true, "malayalam": true, "gujarati": true, "gurmukhi": true, "oriya": true,
		"sinhala": true, "thai": true, "lao": true, "khmer": true, "myanmar": true, "tibetan": true, "ethiopic": true,
		"armenian": true, "georgian": true, "mongolian": true, "cherokee": true, "syriac": true, "thaana": true,
		"naskh": true, "kufi": true, "thuluth": true, "nastaliq": true, "urdu": true, "batak": true, "nko": true,
		"tagalog": true, "sangam": true, "ucas": true,
	}
	symbolMarkers = []string{
		"emoji", "symbol", "dingbat", "webdings", "wingdings", "last resort", "lastresort", "ornaments",
		"braille", "marlett", "segoe mdl2", "segoe fluent", "fontawesome", "font awesome", "material icons", "icons",
		" math",
	}
)

// ClassifyFamily 按家族名推断字体面向的文字；只靠名字，覆盖不了所有本机字体。
func ClassifyFamily(name string) Script {
	lower := strings.ToLower(strings.TrimSpace(name))
	for _, m := range symbolMarkers {
		if strings.Contains(lower, m) {
			return ScriptSymbol
		}
	}
	var han, kana, hangul bool
	for _, r := range name {
		switch {
		case unicode.Is(unicode.Hiragana, r), unicode.Is(unicode.Katakana, r):
			kana = true
		case unicode.Is(unicode.Hangul, r):
			hangul = true
		case unicode.Is(unicode.Han, r):
			han = true
		}
	}
	if kana || hangul {
		return ScriptOther
	}
	if han {
		return ScriptChinese
	}
	tokens := strings.FieldsFunc(lower, func(r rune) bool {
		return r == ' ' || r == '-' || r == '_' || r == '.'
	})
	for _, t := range tokens {
		if otherCJKTokens[t] || otherTokens[t] {
			return ScriptOther
		}
	}
	if chineseNames[lower] || strings.Contains(lower, "wenkai") {
		return ScriptChinese
	}
	for _, p := range chinesePrefixes {
		if strings.HasPrefix(lower, p) {
			return ScriptChinese
		}
	}
	for _, t := range tokens {
		if chineseTokens[t] || t == "cjk" {
			return ScriptChinese
		}
	}
	for _, p := range otherPrefixes {
		if strings.HasPrefix(lower, p) {
			return ScriptOther
		}
	}
	return ScriptLatin
}
