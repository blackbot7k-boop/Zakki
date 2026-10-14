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

func isFenceLine(s string) bool { return strings.HasPrefix(strings.TrimSpace(s), "```") }
