package markdown

import "strings"

func isHeading(trimmed string) bool {
	if trimmed == "" || trimmed[0] != '#' {
		return false
	}
	n := 0
	for n < len(trimmed) && trimmed[n] == '#' {
		n++
	}
	return n <= 6 && n < len(trimmed) && trimmed[n] == ' '
}

func taskPrefixLen(trimmed string) int {
	for _, mark := range []string{"- [ ] ", "* [ ] ", "- [x] ", "* [x] "} {
		if strings.HasPrefix(trimmed, mark) {
			return len(mark)
		}
	}
	return 0
}

func listMarkerLen(trimmed string) int {
	if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") {
		return 2
	}
	for i := 0; i < len(trimmed); i++ {
		if trimmed[i] == '.' && i+1 < len(trimmed) && trimmed[i+1] == ' ' && i > 0 {
			for _, c := range trimmed[:i] {
				if c < '0' || c > '9' {
					return 0
				}
			}
			return i + 2
		}
		if trimmed[i] < '0' || trimmed[i] > '9' {
			return 0
		}
	}
	return 0
}

type fenceMarker struct {
	char byte
	size int
}

func fenceLineContent(s string) (string, bool) {
	s = strings.TrimSuffix(s, "\r")
	indent := 0
	for indent < len(s) && s[indent] == ' ' {
		indent++
		if indent > 3 {
			return "", false
		}
	}
	return s[indent:], true
}

func parseFenceOpeningLine(s string) (fenceMarker, bool) {
	content, ok := fenceLineContent(s)
	if !ok || len(content) < 3 || content[0] != '`' && content[0] != '~' {
		return fenceMarker{}, false
	}
	char := content[0]
	size := 1
	for size < len(content) && content[size] == char {
		size++
	}
	if size < 3 || char == '`' && strings.Contains(content[size:], "`") {
		return fenceMarker{}, false
	}
	return fenceMarker{char: char, size: size}, true
}

func isFenceOpeningLine(s string) bool {
	_, ok := parseFenceOpeningLine(s)
	return ok
}

func isFenceClosingLine(s string, opening fenceMarker) bool {
	content, ok := fenceLineContent(s)
	if !ok || opening.size < 3 || len(content) < opening.size || content[0] != opening.char {
		return false
	}
	size := 1
	for size < len(content) && content[size] == opening.char {
		size++
	}
	return size >= opening.size && strings.Trim(content[size:], " \t") == ""
}

func fenceClosingMarker(opening fenceMarker) string {
	return strings.Repeat(string(opening.char), opening.size)
}

// fenceInfo 提取开栏行的 info 串（标记后的剩余部分，已去空白）。
func fenceInfo(s string) string {
	content, ok := fenceLineContent(s)
	if !ok || len(content) < 3 || content[0] != '`' && content[0] != '~' {
		return ""
	}
	size := 0
	for size < len(content) && content[size] == content[0] {
		size++
	}
	return strings.TrimSpace(content[size:])
}
