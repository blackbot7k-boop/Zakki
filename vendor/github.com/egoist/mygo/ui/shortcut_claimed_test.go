package ui

import "testing"

// 补丁回归：已登记为快捷键（焦点链或窗口）的和弦不再交给聚焦的文本编辑器
// 做词间移动或选区扩展，而是交给快捷键系统——菜单等价键在所有原生平台都
// 优先于文本导航。未登记的和弦仍归编辑器。
func TestClaimedShortcutBeatsEditorNavigation(t *testing.T) {
	value := "one two"
	fired := false
	tt := NewTester(func(c *Context) {
		Column(c).Fill().Children(func() {
			TextAreaBase(c.Key("ed"), &value).Fill().Grow(1).AutoFocus().Label("ed")
		})
		if c.Shortcut(Super|Alt, KeyRight) {
			fired = true
		}
	}, 300, 200)
	tt.Frame()
	if !tt.Focused("ed") {
		t.Fatal("编辑器应已聚焦")
	}
	// 未登记的 Alt+→ 仍是编辑器的词间移动。
	tt.Key(0, KeyHome)
	tt.Frame()
	before, ok := tt.TextCaret()
	if !ok {
		t.Fatal("没有光标")
	}
	tt.Key(Alt, KeyRight)
	tt.Frame()
	after, _ := tt.TextCaret()
	if after == before {
		t.Fatal("未登记的 Alt+→ 应仍是词间移动")
	}
	// 已登记的 ⌘⌥→ 交给快捷键，编辑器不再消费。
	tt.Key(Super|Alt, KeyRight)
	tt.Frame()
	if !fired {
		t.Fatal("已登记的 ⌘⌥→ 应触发快捷键")
	}
	final, _ := tt.TextCaret()
	if final != after {
		t.Fatal("已登记的 ⌘⌥→ 不应移动光标")
	}
}
