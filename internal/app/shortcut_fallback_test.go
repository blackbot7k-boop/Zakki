package app

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// 窗口级快捷键兜底（shortcut_fallback.go）：键位与菜单同源，动作与菜单
// Click 同一实现。这里锁定键位派生的完整性与分发的正确性。

// TestWindowShortcutsDeriveFromTables 兜底注册的键位集合与「当前平台
// 生效绑定」完全一致：每条非空命令绑定与可执行固定键都能解析成物理键，
// 且两平台各自集合内无重复（无歧义分发的前提）。
func TestWindowShortcutsDeriveFromTables(t *testing.T) {
	a := newTestApp(t)
	t.Cleanup(a.shutdown)
	for _, goos := range []string{"darwin", "windows"} {
		type physicalBinding struct {
			mods ui.Modifiers
			key  ui.Key
		}
		seen := map[physicalBinding]string{}
		count := 0
		for _, def := range commandRegistry {
			if def.global {
				continue
			}
			b, ok := a.parsedCommandBindingForPlatform(def.id, goos)
			if !ok {
				continue
			}
			physical := physicalBinding{mods: b.mods, key: b.key}
			if other, exists := seen[physical]; exists {
				t.Fatalf("%s 兜底键冲突: %s 与 %s", goos, def.id, other)
			}
			seen[physical] = def.id
			count++
		}
		for _, fixed := range fixedBindingsForPlatform(goos) {
			switch fixed.key {
			case "undo", "redo", "cut", "copy", "paste", "selectAll",
				"appSwitcher", "hideApp":
				continue // 不注册兜底，见文件头注释
			}
			b, err := parseAcceleratorForPlatform(fixed.acc, goos)
			if err != nil {
				t.Fatalf("%s 固定键 %s: %v", goos, fixed.acc, err)
			}
			physical := physicalBinding{mods: b.mods, key: b.key}
			if other, exists := seen[physical]; exists {
				t.Fatalf("%s 兜底键冲突: 固定键 %s 与 %s", goos, fixed.key, other)
			}
			seen[physical] = "fixed." + fixed.key
			count++
		}
		// 键位覆盖量级：12 条命令 + 约 21 条可执行固定键。
		if count < 30 {
			t.Fatalf("%s 兜底键位只有 %d 条，疑似漏注册", goos, count)
		}
	}
}

// TestWindowShortcutsKeymapOverrideFollows 用户改键后兜底跟随新键：
// 把加粗改到 ⌘/Ctrl+Shift+F1，解析出的物理键应随之变化。
func TestWindowShortcutsKeymapOverrideFollows(t *testing.T) {
	a := newTestApp(t)
	t.Cleanup(a.shutdown)
	before, ok := a.parsedCommandBinding(cmdBold)
	if !ok || before.key != ui.KeyB {
		t.Fatalf("加粗默认键应为 B: %+v", before)
	}
	if err := a.setCommandBinding(cmdBold, "CmdOrCtrl+Shift+F1"); err != nil {
		t.Fatal(err)
	}
	after, ok := a.parsedCommandBinding(cmdBold)
	if !ok || after.key != ui.KeyF1 || after.mods&ui.Shift == 0 {
		t.Fatalf("改键后兜底应跟随: %+v", after)
	}
}

// TestRunFixedShortcutDispatch 兜底动作与菜单 Click 同一语义（headless 下
// 逐项触发并断言可见状态）。
func TestRunFixedShortcutDispatch(t *testing.T) {
	a := newTestApp(t)
	t.Cleanup(a.shutdown)

	a.runFixedShortcut("find")
	if !a.findOpen {
		t.Fatal("find 兜底应打开查找条")
	}
	a.query = "内"
	a.runFixedShortcut("findNext")
	if !a.findOpen {
		t.Fatal("findNext 应保持查找条")
	}
	a.runFixedShortcut("settings")
	if a.settingsPage != pageGeneral {
		t.Fatalf("settings 兜底应定位到常规页: %q", a.settingsPage)
	}

	if len(a.tabs) < 3 {
		for len(a.tabs) < 3 {
			a.addTab()
		}
	}
	a.active = 0
	a.runFixedShortcut("tab2")
	if a.active != 1 {
		t.Fatalf("tab2 兜底后 active=%d", a.active)
	}
	a.runFixedShortcut("nextTab")
	if a.active != 2 {
		t.Fatalf("nextTab 兜底后 active=%d", a.active)
	}
	a.runFixedShortcut("prevTab")
	if a.active != 1 {
		t.Fatalf("prevTab 兜底后 active=%d", a.active)
	}

	before := len(a.tabs)
	a.runFixedShortcut("newTab")
	if len(a.tabs) != before+1 || a.active != before {
		t.Fatalf("newTab 兜底 len=%d active=%d", len(a.tabs), a.active)
	}
	a.runFixedShortcut("renameTab")
	if a.renaming == 0 {
		t.Fatal("renameTab 兜底应进入重命名")
	}
	a.cancelRename()

	// 无动作的占位键与越界标签静默不动。
	a.active = 0
	a.runFixedShortcut("tab9")
	if a.active != 0 {
		t.Fatalf("tab9 越界时不应切换: active=%d", a.active)
	}
	a.runFixedShortcut("appSwitcher")
	if a.active != 0 || a.renaming != 0 {
		t.Fatal("appSwitcher 占位键不应有任何动作")
	}
}
