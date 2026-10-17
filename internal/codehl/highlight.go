// Package codehl 把围栏代码块逐行切成着色区间，供源码模式与实时代码
// 卡片上色。纯行级词法：不跨行追踪状态，块注释与多行字符串只着色起始
// 行——对便签里的代码片段足够，换来的是不需要文档级扫描。
package codehl

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// Kind 是 token 类别，同时是配色表的下标。
type Kind uint8

const (
	Plain Kind = iota // 普通文本，沿用主题文字色
	Comment
	String
	Number
	Keyword
	Func // 标识符后随 '('，数据格式语言没有
	Key  // json/yaml/toml 的键名
	// KindCount 是类别总数，配色表按它定长。
	KindCount
)

// Span 是一行内的着色区间，Start/End 为 rune 下标。
type Span struct {
	Start, End int
	Kind       Kind
}

// Lang 把围栏 info 串归一成内部语言名；无法识别返回空串（走通用规则）。
func Lang(info string) string {
	f := strings.Fields(info)
	if len(f) == 0 {
		return ""
	}
	switch strings.TrimLeft(strings.ToLower(f[0]), ".") {
	case "go", "golang":
		return "go"
	case "js", "jsx", "javascript", "mjs", "cjs":
		return "js"
	case "ts", "tsx", "typescript":
		return "ts"
	case "py", "python", "python3":
		return "py"
	case "sh", "bash", "shell", "zsh":
		return "sh"
	case "json", "jsonc", "json5":
		return "json"
	case "yaml", "yml":
		return "yaml"
	case "toml":
		return "toml"
	case "rs", "rust":
		return "rust"
	case "c", "h", "cpp", "cxx", "cc", "c++", "hpp":
		return "c"
	case "java", "kt", "kotlin", "swift", "cs", "csharp":
		return "java"
	default:
		return ""
	}
}

type langStyle struct {
	keywords  map[string]bool
	slashCmt  bool   // // 与 /*（到行尾）
	hashCmt   bool   // #（行首或空白后）
	quotes    string // 有效引号字符
	funcCalls bool   // 标识符后随 '(' 记为 Func
	keys      bool   // 行首（或 "- " 后）标识符/字符串后随 ':' 或 '=' 记为 Key
}

var langStyles = map[string]langStyle{
	"go": {
		slashCmt: true, quotes: "\"`'", funcCalls: true,
		keywords: set("break", "case", "chan", "const", "continue", "default", "defer", "else",
			"fallthrough", "for", "func", "go", "goto", "if", "import", "interface", "map",
			"package", "range", "return", "select", "struct", "switch", "type", "var",
			"nil", "true", "false", "iota"),
	},
	"js": {
		slashCmt: true, quotes: "\"'`", funcCalls: true,
		keywords: set("async", "await", "break", "case", "catch", "class", "const", "continue",
			"debugger", "default", "delete", "do", "else", "export", "extends", "finally",
			"for", "from", "function", "get", "if", "import", "in", "instanceof", "interface",
			"let", "new", "of", "return", "set", "static", "super", "switch", "this", "throw",
			"try", "typeof", "var", "void", "while", "yield", "null", "undefined", "true", "false"),
	},
	"py": {
		hashCmt: true, quotes: "\"'", funcCalls: true,
		keywords: set("and", "as", "assert", "async", "await", "break", "class", "continue",
			"def", "del", "elif", "else", "except", "finally", "for", "from", "global", "if",
			"import", "in", "is", "lambda", "nonlocal", "not", "or", "pass", "raise",
			"return", "try", "while", "with", "yield", "None", "True", "False"),
	},
	"sh": {
		hashCmt: true, quotes: "\"'", funcCalls: true,
		keywords: set("if", "then", "else", "elif", "fi", "for", "while", "until", "do", "done",
			"case", "esac", "in", "function", "select", "export", "local", "readonly",
			"return", "exit", "set", "unset", "shift", "trap", "true", "false"),
	},
	"json": {
		quotes: "\"", keys: true,
		keywords: set("true", "false", "null"),
	},
	"yaml": {
		hashCmt: true, quotes: "\"'", keys: true,
		keywords: set("true", "false", "null"),
	},
	"toml": {
		hashCmt: true, quotes: "\"'", keys: true,
		keywords: set("true", "false"),
	},
	"rust": {
		slashCmt: true, quotes: "\"", funcCalls: true,
		keywords: set("as", "async", "await", "break", "const", "continue", "crate", "dyn",
			"else", "enum", "extern", "false", "fn", "for", "if", "impl", "in", "let", "loop",
			"match", "mod", "move", "mut", "pub", "ref", "return", "self", "Self", "static",
			"struct", "super", "trait", "true", "type", "unsafe", "use", "where", "while"),
	},
	"c": {
		slashCmt: true, quotes: "\"'", funcCalls: true,
		keywords: set("auto", "break", "case", "const", "continue", "default", "do", "else",
			"enum", "extern", "for", "goto", "if", "inline", "register", "restrict", "return",
			"sizeof", "static", "struct", "switch", "typedef", "union", "volatile", "while",
			"void", "char", "int", "long", "short", "float", "double", "unsigned", "signed",
			"bool", "true", "false", "NULL", "class", "namespace", "template", "public",
			"private", "protected", "new", "delete", "this", "virtual", "override"),
	},
	"java": {
		slashCmt: true, quotes: "\"'", funcCalls: true,
		keywords: set("abstract", "assert", "boolean", "break", "byte", "case", "catch",
			"char", "class", "const", "continue", "default", "do", "double", "else", "enum",
			"extends", "final", "finally", "float", "for", "goto", "if", "implements",
			"import", "instanceof", "int", "interface", "long", "native", "new", "package",
			"private", "protected", "public", "return", "short", "static", "strictfp",
			"super", "switch", "synchronized", "this", "throw", "throws", "transient", "try",
			"void", "volatile", "while", "var", "record", "true", "false", "null"),
	},
}

// genericStyle 是未知语言的兜底：字符串、数字与常见注释。
var genericStyle = langStyle{slashCmt: true, hashCmt: true, quotes: "\"'"}

func styleFor(lang string) langStyle {
	if st, ok := langStyles[lang]; ok {
		return st
	}
	return genericStyle
}

func set(words ...string) map[string]bool {
	m := make(map[string]bool, len(words))
	for _, w := range words {
		m[w] = true
	}
	return m
}

// Line 切分一行代码；info 是围栏 info 串。区间按出现顺序，互不重叠。
func Line(info, line string) []Span {
	st := styleFor(Lang(info))
	var spans []Span
	push := func(start, end, runeStart int, kind Kind) {
		if end > start {
			spans = append(spans, Span{Start: runeStart, End: runeStart + runeLen(line[start:end]), Kind: kind})
		}
	}
	i, r, n := 0, 0, len(line)
	for i < n {
		c := line[i]
		// 注释：命中前缀即吞到行尾。
		if kind, ok := st.commentAt(line, i); ok {
			push(i, n, r, kind)
			break
		}
		// 字符串（含数据格式的键名字符串）。
		if strings.IndexByte(st.quotes, c) >= 0 {
			j := st.skipString(line, i)
			kind := String
			if st.keys && st.isKey(line, i, j) {
				kind = Key
			}
			push(i, j, r, kind)
			r += runeLen(line[i:j])
			i = j
			continue
		}
		// 数字：吞掉十六进制、指数、下划线与常见后缀。
		if c >= '0' && c <= '9' {
			j := i + 1
			for j < n {
				ch := line[j]
				// 指数符号只跟在 e/E 后。
				if (ch == '+' || ch == '-') && (line[j-1] == 'e' || line[j-1] == 'E') {
					j++
					continue
				}
				if ch >= '0' && ch <= '9' || ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z' || ch == '_' || ch == '.' {
					j++
					continue
				}
				break
			}
			push(i, j, r, Number)
			r += runeLen(line[i:j])
			i = j
			continue
		}
		// 标识符：关键字、函数调用、数据格式键名。
		if rc, sz := utf8.DecodeRuneInString(line[i:]); rc == '_' || rc == '$' || unicode.IsLetter(rc) {
			j := i + sz
			for j < n {
				rc2, sz2 := utf8.DecodeRuneInString(line[j:])
				if rc2 == '_' || rc2 == '$' || unicode.IsLetter(rc2) || unicode.IsDigit(rc2) {
					j += sz2
					continue
				}
				break
			}
			word := line[i:j]
			switch {
			case st.keywords[word]:
				push(i, j, r, Keyword)
			case st.funcCalls && strings.IndexFunc(line[j:], func(rn rune) bool { return rn != ' ' && rn != '\t' }) == 0 && nextByte(line, j) == '(':
				push(i, j, r, Func)
			case st.keys && st.isKey(line, i, j):
				push(i, j, r, Key)
			}
			r += runeLen(word)
			i = j
			continue
		}
		_, sz := utf8.DecodeRuneInString(line[i:])
		i += sz
		r++
	}
	return spans
}

func (st langStyle) commentAt(line string, i int) (Kind, bool) {
	// 行注释要求行首或前导空白，避免 URL 的 // 、C# 的 a#b 误伤。
	if i > 0 && line[i-1] != ' ' && line[i-1] != '\t' {
		return 0, false
	}
	rest := line[i:]
	if st.hashCmt && strings.HasPrefix(rest, "#") {
		return Comment, true
	}
	if st.slashCmt {
		if strings.HasPrefix(rest, "//") || strings.HasPrefix(rest, "/*") {
			return Comment, true
		}
	}
	return 0, false
}

// skipString 返回引号字符串的结束下标（不含尾引号时为行尾）。
// 反斜杠转义按 rune 跳过；三引号按普通引号处理，只着色到行尾。
func (st langStyle) skipString(line string, i int) int {
	q := rune(line[i])
	for j := i + 1; j < len(line); {
		rc, sz := utf8.DecodeRuneInString(line[j:])
		if rc == '\\' && q != '`' {
			_, sz2 := utf8.DecodeRuneInString(line[j+sz:])
			j += sz + sz2
			continue
		}
		if rc == q {
			return j + sz
		}
		j += sz
	}
	return len(line)
}

// isKey 判断 [i,j) 是否处于键位置：行首缩进后（yaml 列表项 "- " 之后也算），
// 且冒号/等号在同一个 token 之后。
func (st langStyle) isKey(line string, i, j int) bool {
	if !st.keys {
		return false
	}
	prefix := strings.TrimRight(line[:i], " \t")
	// yaml 列表项、json 逗号或开括号之后仍算键位置。
	if prefix != "" && !strings.HasSuffix(prefix, "-") && !strings.HasSuffix(prefix, ",") &&
		!strings.HasSuffix(prefix, "{") && !strings.HasSuffix(prefix, "[") {
		return false
	}
	k := j
	for k < len(line) && (line[k] == ' ' || line[k] == '\t') {
		k++
	}
	return k < len(line) && (line[k] == ':' || line[k] == '=')
}

func nextByte(line string, i int) byte {
	if i < len(line) {
		return line[i]
	}
	return 0
}

func runeLen(s string) int { return utf8.RuneCountInString(s) }
