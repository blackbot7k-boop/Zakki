package app

import (
	"runtime"
	"strings"
)

// 静态文案（locale 字符串、顶栏提示）里的快捷键以 macOS 符号书写；Windows
// 没有对应的修饰键符号，展示前用 shortcutText 转成 Ctrl/Alt/Shift 名称。
// 动态键位展示不走这里，keyBinding.displayNameForPlatform 已经分平台。

// shortcutGlyphs 是 locale 文案中出现的 macOS 修饰键符号。
const shortcutGlyphs = "⌘⌃⌥⇧"

// shortcutText 把快捷键文案转为当前平台的展示形式。
func shortcutText(s string) string {
	return shortcutTextFor(s, runtime.GOOS)
}

// shortcutTextFor 把以 macOS 符号书写的快捷键文案转为 goos 平台的展示形式。
// darwin 原样返回；其他平台把修饰键符号换成 Windows 名称：
// ⌘→Ctrl（主命令修饰键，CmdOrCtrl）、⌥→Alt、⇧→Shift，
// 组合按 Ctrl+Alt+Shift 排序去重，修饰键列表（⌘/⌥）映射后用 "/" 连接去重，
// 其余字符原样保留。
// ⌃ 是唯一例外：整库文案里它只出现在 mac 标题数字键（⌃1–6，标题在
// macOS 上退到次级修饰键），而标题键在 Windows 上是 Alt+N
// （defaultBindingForPlatform），所以映射为 Alt 而非 Ctrl——
// 若将来出现表示普通 Control 组合的 ⌃ 文案，需改用占位符传入。
func shortcutTextFor(s, goos string) string {
	if goos == "darwin" {
		return s
	}
	var sb strings.Builder
	runes := []rune(s)
	for i := 0; i < len(runes); {
		if !strings.ContainsRune(shortcutGlyphs, runes[i]) {
			sb.WriteRune(runes[i])
			i++
			continue
		}
		// 收集连续修饰键符号；斜杠分隔的符号列表（⌘/⌃/⌥）一并收集。
		j := i
		slashList := false
		for j < len(runes) {
			if strings.ContainsRune(shortcutGlyphs, runes[j]) {
				j++
			} else if j+1 < len(runes) && runes[j] == '/' && strings.ContainsRune(shortcutGlyphs, runes[j+1]) {
				slashList = true
				j += 2
			} else {
				break
			}
		}
		sep := "+"
		if slashList {
			sep = "/"
		}
		names := shortcutNames(runes[i:j], sep)
		// 修饰键组合后紧跟的第一个非空白字符是键位（⌘F、⌘,、⌥⌘←），补上分隔符
		// 再带它；斜杠列表与孤立组合不带（列表后跟的是文字，不是键位）。
		if !slashList && j < len(runes) && !isShortcutSpace(runes[j]) {
			sb.WriteString(names)
			sb.WriteString(sep)
			sb.WriteRune(runes[j])
			i = j + 1
			continue
		}
		sb.WriteString(names)
		i = j
	}
	return sb.String()
}

// shortcutNames 把修饰键符号序列转成 Windows 名称，按 Ctrl/Alt/Shift 排序去重；
// 序列里的斜杠等非符号字符（收集时一并扫过的分隔符）跳过。
func shortcutNames(glyphs []rune, sep string) string {
	pri := func(r rune) int {
		switch r {
		case '⌘':
			return 0
		case '⌃', '⌥':
			return 1
		default: // '⇧'
			return 2
		}
	}
	name := func(r rune) string {
		switch r {
		case '⌘':
			return "Ctrl"
		case '⌃':
			return "Alt" // mac 标题修饰键：Windows 上标题是 Alt+N，见 defaultBindingForPlatform
		case '⌥':
			return "Alt"
		default: // '⇧'
			return "Shift"
		}
	}
	// 符号量最多三四个，插入排序即可。
	sorted := make([]rune, 0, len(glyphs))
	for _, r := range glyphs {
		if strings.ContainsRune(shortcutGlyphs, r) {
			sorted = append(sorted, r)
		}
	}
	for a := 1; a < len(sorted); a++ {
		for b := a; b > 0 && pri(sorted[b]) < pri(sorted[b-1]); b-- {
			sorted[b], sorted[b-1] = sorted[b-1], sorted[b]
		}
	}
	parts := make([]string, 0, len(sorted))
	for k, r := range sorted {
		if k > 0 && name(r) == name(sorted[k-1]) {
			continue
		}
		parts = append(parts, name(r))
	}
	return strings.Join(parts, sep)
}

func isShortcutSpace(r rune) bool {
	switch r {
	case ' ', '\t', '\n', '\r', '　':
		return true
	}
	return false
}
