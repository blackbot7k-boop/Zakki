package app

import (
	"runtime"
	"testing"
)

// 语料取自 locales/*.json 的实际文案与顶栏标签提示，锁定 Windows 展示形式。
func TestShortcutTextFor(t *testing.T) {
	tests := []struct {
		name string
		in   string
		goos string
		want string
	}{
		{"darwin 原样", "查找 ⌘F，源码 ⌘⇧P", "darwin", "查找 ⌘F，源码 ⌘⇧P"},
		{"单键", "Find ⌘F, next ⌘G", "windows", "Find Ctrl+F, next Ctrl+G"},
		{"带 Shift", "source ⌘⇧P", "windows", "source Ctrl+Shift+P"},
		{"Alt+主修饰键排序", "keep on top ⌥⌘T", "windows", "keep on top Ctrl+Alt+T"},
		{"⌃ 是 mac 标题键映射 Alt", "headings ⌃1–6", "windows", "headings Alt+1–6"},
		{"方向键", "⌥⌘←/→", "windows", "Ctrl+Alt+←/→"},
		{"逗号键", "Settings… ⌘,", "windows", "Settings… Ctrl+,"},
		{"撤销", "Undo with ⌘Z.", "windows", "Undo with Ctrl+Z."},
		{"中文夹键位", "查找 ⌘F，下一个 ⌘G，备份 ⌘S", "windows", "查找 Ctrl+F，下一个 Ctrl+G，备份 Ctrl+S"},
		{"标题组合", "⌘1–6 set headings", "windows", "Ctrl+1–6 set headings"},
		{"全局热键", "⌘⇧Space shows/hides", "windows", "Ctrl+Shift+Space shows/hides"},
		{"修饰键列表去重", "add ⌘/⌥", "windows", "add Ctrl/Alt"},
		{"修饰键列表中文", "请加上 ⌘/⌥ 修饰键", "windows", "请加上 Ctrl/Alt 修饰键"},
		{"重复主修饰键", "⌘⌘F 不存在但应稳定", "windows", "Ctrl+F 不存在但应稳定"},
		{"孤立符号不带键", "开关 ⌘ 与 ⌥ 组合", "windows", "开关 Ctrl 与 Alt 组合"},
		{"无符号原样", "plain text ⌘", "windows", "plain text Ctrl"},
		{"linux 同 windows", "⌘S", "linux", "Ctrl+S"},
		{"⌃ 在 linux 也是 Alt", "⌃1–6", "linux", "Alt+1–6"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shortcutTextFor(tt.in, tt.goos); got != tt.want {
				t.Errorf("shortcutTextFor(%q, %s) = %q, want %q", tt.in, tt.goos, got, tt.want)
			}
		})
	}
}

// 当前平台的包装只分平台不变形：macOS 上原样，非 macOS 上键位仍在。
func TestShortcutTextCurrentPlatform(t *testing.T) {
	in := "⌥⌘T keeps the window on top"
	got := shortcutText(in)
	if runtime.GOOS == "darwin" {
		if got != in {
			t.Errorf("darwin 应原样返回：%q", got)
		}
	} else if got != "Ctrl+Alt+T keeps the window on top" {
		t.Errorf("非 darwin 应转换：%q", got)
	}
}
