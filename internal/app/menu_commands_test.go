package app

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

// testKey 按 accelerator 键名求 ui.Key（测试辅助）。
func testKey(t *testing.T, name string) ui.Key {
	t.Helper()
	b, err := parseAccelerator(name)
	if err != nil {
		t.Fatal(err)
	}
	return b.key
}

// 加速器解析、规范化、显示名与校验（Keymap 的底层契约）。

func TestParseAccelerator(t *testing.T) {
	for _, tc := range []struct {
		in   string
		mods ui.Modifiers
		key  ui.Key
	}{
		{"CmdOrCtrl+B", ui.Super, ui.KeyB},
		{"CmdOrCtrl+Shift+8", ui.Super | ui.Shift, ui.Key8},
		{"Alt+CmdOrCtrl+0", ui.Super | ui.Alt, ui.Key0},
		{"Super+Alt+Space", ui.Super | ui.Alt, ui.KeySpace},
		{"Ctrl+Alt+,", ui.Ctrl | ui.Alt, ui.KeyComma},
		{"Shift+CmdOrCtrl+Q", ui.Super | ui.Shift, ui.KeyQ},
	} {
		b, err := parseAccelerator(tc.in)
		if err != nil {
			t.Fatalf("%s: %v", tc.in, err)
		}
		if b.mods != tc.mods || b.key != tc.key {
			t.Fatalf("%s 解析错误: %+v", tc.in, b)
		}
		// 规范化往返。
		if again, err := parseAccelerator(b.accelerator()); err != nil || again != b {
			t.Fatalf("%s 规范化往返失败: %q %+v %v", tc.in, b.accelerator(), again, err)
		}
	}
	for _, bad := range []string{"", "+", "Shift+", "CmdOrCtrl+F5+", "CmdOrCtrl+NotAKey"} {
		if _, err := parseAccelerator(bad); err == nil {
			t.Fatalf("应拒绝 %q", bad)
		}
	}
	// 裸键是合法 accelerator 字符串，但 validateBinding 拒绝裸字母/数字。
	if b, err := parseAccelerator("B"); err != nil {
		t.Fatal(err)
	} else if validateBinding(b) == nil {
		t.Fatal("裸字母应在 validateBinding 被拒")
	}
}

func TestNormalizeLegacyAccelerator(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"CmdOrCtrl+Shift+Space", "CmdOrCtrl+Shift+Space"},
		{"Super+Alt+Space", "CmdOrCtrl+Alt+Space"},
		{"Cmd+Alt+Space", "CmdOrCtrl+Alt+Space"},
		{"ctrl+alt+delete", "Ctrl+Alt+Delete"},
	} {
		got, err := normalizeLegacyAccelerator(tc.in)
		if err != nil || got != tc.want {
			t.Fatalf("%s → %q (%v), 要 %q", tc.in, got, err, tc.want)
		}
	}
}

func TestBindingDisplayName(t *testing.T) {
	for _, tc := range []struct{ acc, want string }{
		{"CmdOrCtrl+B", "⌘B"},
		{"CmdOrCtrl+Shift+Space", "⌘⇧空格"},
		{"CmdOrCtrl+Alt+1", "⌘⌥1"},
		{"Ctrl+Alt+Delete", "⌃⌥⌦"},
		{"CmdOrCtrl+Shift+E", "⌘⇧E"},
	} {
		b, err := parseAccelerator(tc.acc)
		if err != nil {
			t.Fatal(err)
		}
		if got := b.displayName(); got != tc.want {
			t.Fatalf("%s 显示名 %q，要 %q", tc.acc, got, tc.want)
		}
	}
}

func TestValidateBinding(t *testing.T) {
	mustOK := func(acc string) {
		t.Helper()
		b, err := parseAccelerator(acc)
		if err != nil {
			t.Fatal(err)
		}
		if err := validateBinding(b); err != nil {
			t.Fatalf("%s 应合法: %v", acc, err)
		}
	}
	mustErr := func(acc string, contains string) {
		t.Helper()
		b, err := parseAccelerator(acc)
		if err != nil {
			t.Fatal(err)
		}
		err = validateBinding(b)
		if err == nil || !strings.Contains(err.Error(), contains) {
			t.Fatalf("%s 应拒绝（含 %q）: %v", acc, contains, err)
		}
	}
	mustOK("CmdOrCtrl+B")
	mustOK("CmdOrCtrl+Shift+Space")
	mustOK("F5") // 功能键允许不带修饰键
	mustErr("B", "裸字母")
	mustErr("8", "裸字母")
	mustErr("Shift+B", "修饰键") // Shift+字母会吞掉大写输入
	mustErr("CmdOrCtrl+Tab", "导航输入键")
	mustErr("CmdOrCtrl+Enter", "导航输入键")
	mustErr("CmdOrCtrl+Escape", "导航输入键")
	mustErr("CmdOrCtrl+Left", "导航输入键")
	mustErr("CmdOrCtrl+Backspace", "导航输入键")
	mustErr("Space", "空格")
}

func TestFindBindingConflict(t *testing.T) {
	a := newTestApp(t)
	// 固定键：搜索 ⌘F。
	b, _ := parseAccelerator("CmdOrCtrl+F")
	if name := a.findBindingConflict(cmdBold, b); name != "查找" {
		t.Fatalf("固定键冲突应显示名称: %q", name)
	}
	// 其他命令的当前绑定。
	if err := a.setCommandBinding(cmdBold, "CmdOrCtrl+Alt+U"); err != nil {
		t.Fatal(err)
	}
	b2, _ := parseAccelerator("CmdOrCtrl+Alt+U")
	if name := a.findBindingConflict(cmdItalic, b2); name != "加粗" {
		t.Fatalf("命令冲突应显示名称: %q", name)
	}
	// 自己的旧绑定不算冲突。
	if name := a.findBindingConflict(cmdBold, b2); name != "" {
		t.Fatalf("自身绑定不算冲突: %q", name)
	}
	// 系统保留键。
	b3, _ := parseAccelerator("CmdOrCtrl+Q")
	if name := a.findBindingConflict(cmdBold, b3); name != "退出" {
		t.Fatalf("系统保留键应识别: %q", name)
	}
}

func TestPlatformAcceleratorsAndWindowsHeadingDefaults(t *testing.T) {
	winPrimary, err := parseAcceleratorForPlatform("CmdOrCtrl+Alt+1", "windows")
	if err != nil {
		t.Fatal(err)
	}
	if winPrimary.mods != ui.Ctrl|ui.Alt || !winPrimary.primary {
		t.Fatalf("Windows CmdOrCtrl 解析错误: %+v", winPrimary)
	}
	if got := winPrimary.displayNameForPlatform("windows"); got != "Ctrl+Alt+1" {
		t.Fatalf("Windows 显示名 = %q", got)
	}
	if got := winPrimary.acceleratorForPlatform("windows"); got != "CmdOrCtrl+Alt+1" {
		t.Fatalf("Windows accelerator = %q", got)
	}
	winSuper, err := parseAcceleratorForPlatform("Super+Alt+1", "windows")
	if err != nil {
		t.Fatal(err)
	}
	if winSuper.mods != ui.Super|ui.Alt || winSuper.primary || sameBinding(winPrimary, winSuper) {
		t.Fatalf("Windows 显式 Super 未与 CmdOrCtrl 区分: primary=%+v super=%+v", winPrimary, winSuper)
	}
	if got := winSuper.displayNameForPlatform("windows"); got != "Win+Alt+1" {
		t.Fatalf("Windows Super 显示名 = %q", got)
	}

	macPrimary, err := parseAcceleratorForPlatform("CmdOrCtrl+1", "darwin")
	if err != nil {
		t.Fatal(err)
	}
	if macPrimary.mods != ui.Super || !macPrimary.primary || macPrimary.displayNameForPlatform("darwin") != "⌘1" {
		t.Fatalf("macOS CmdOrCtrl 解析或显示错误: %+v %q", macPrimary, macPrimary.displayNameForPlatform("darwin"))
	}

	a := newTestApp(t)
	t.Cleanup(a.shutdown)
	// 标签键两平台同形（主修饰键+数字）；标题退到各平台次级修饰键。
	if got := a.commandBindingForPlatform(cmdH1, "darwin"); got != "Ctrl+1" {
		t.Fatalf("macOS H1 默认键 = %q", got)
	}
	if got := a.commandBindingForPlatform(cmdBody, "darwin"); got != "Ctrl+0" {
		t.Fatalf("macOS 正文默认键 = %q", got)
	}
	if got := a.commandBindingForPlatform(cmdH1, "windows"); got != "Alt+1" {
		t.Fatalf("Windows H1 默认键 = %q", got)
	}
	macTab, err := parseAcceleratorForPlatform(fixedBindingByKeyForPlatform("tab1", "darwin"), "darwin")
	if err != nil {
		t.Fatal(err)
	}
	if macTab.mods != ui.Super || macTab.displayNameForPlatform("darwin") != "⌘1" {
		t.Fatalf("macOS 标签固定键 = %+v %q, want ⌘1", macTab, macTab.displayNameForPlatform("darwin"))
	}
	winTab, err := parseAcceleratorForPlatform(fixedBindingByKeyForPlatform("tab1", "windows"), "windows")
	if err != nil {
		t.Fatal(err)
	}
	if winTab.mods != ui.Ctrl || winTab.displayNameForPlatform("windows") != "Ctrl+1" {
		t.Fatalf("Windows 标签固定键 = %+v %q, want Ctrl+1", winTab, winTab.displayNameForPlatform("windows"))
	}
	// 删除标签固定键：macOS ⌘⇧D，Windows Ctrl+Shift+D；不用 ⌘W（关窗）
	// 与 ⌥⌘D（macOS 系统 Dock 键）。
	delMac, delErr := parseAcceleratorForPlatform(fixedBindingByKeyForPlatform("deleteTab", "darwin"), "darwin")
	if delErr != nil || delMac.mods != ui.Super|ui.Shift || delMac.key != ui.KeyD {
		t.Fatalf("macOS 删除标签固定键 = %+v err=%v", delMac, delErr)
	}
	delWin, delErr := parseAcceleratorForPlatform(fixedBindingByKeyForPlatform("deleteTab", "windows"), "windows")
	if delErr != nil || delWin.mods != ui.Ctrl|ui.Shift || delWin.key != ui.KeyD {
		t.Fatalf("Windows 删除标签固定键 = %+v err=%v", delWin, delErr)
	}
	if got := fixedBindingName("deleteTab"); got == "" || got == "deleteTab" {
		t.Fatalf("删除标签固定键应有显示名: %q", got)
	}
	macH1, _ := parseAcceleratorForPlatform(a.commandBindingForPlatform(cmdH1, "darwin"), "darwin")
	if name := a.findBindingConflictForPlatform(cmdH1, macH1, "darwin"); name != "" {
		t.Fatalf("macOS 标题键不应有冲突: %q", name)
	}
	h1win, _ := parseAcceleratorForPlatform(a.commandBindingForPlatform(cmdH1, "windows"), "windows")
	if name := a.findBindingConflictForPlatform(cmdH1, h1win, "windows"); name != "" {
		t.Fatalf("Windows 标题键不应有冲突: %q", name)
	}

	type physicalBinding struct {
		mods ui.Modifiers
		key  ui.Key
	}
	// 两平台各自扫一遍：默认键位表内不得有物理键冲突，且全部可解析。
	for _, goos := range []string{"darwin", "windows"} {
		seen := map[physicalBinding]string{}
		for _, fixed := range fixedBindingsForPlatform(goos) {
			b, parseErr := parseAcceleratorForPlatform(fixed.acc, goos)
			if parseErr != nil {
				t.Fatalf("%s 固定键 %s: %v", goos, fixed.acc, parseErr)
			}
			physical := physicalBinding{mods: b.mods, key: b.key}
			if other, exists := seen[physical]; exists {
				t.Fatalf("%s 固定键冲突: %s 与 %s 都是 %s", goos, fixed.key, other, fixed.acc)
			}
			seen[physical] = fixed.key
		}
		for _, def := range commandRegistry {
			acc := a.commandBindingForPlatform(def.id, goos)
			if acc == "" {
				continue
			}
			b, parseErr := parseAcceleratorForPlatform(acc, goos)
			if parseErr != nil {
				t.Fatalf("%s 命令 %s 默认键 %s: %v", goos, def.id, acc, parseErr)
			}
			physical := physicalBinding{mods: b.mods, key: b.key}
			if other, exists := seen[physical]; exists {
				t.Fatalf("%s 默认键冲突: %s 与 %s 都是 %s", goos, def.id, other, acc)
			}
			seen[physical] = def.id
		}
	}
}

func TestResetCommandBindingRejectsConflictAndKeepsOverride(t *testing.T) {
	a := newTestApp(t)
	t.Cleanup(a.shutdown)
	if err := a.setCommandBinding(cmdBold, ""); err != nil {
		t.Fatal(err)
	}
	if err := a.setCommandBinding(cmdItalic, "CmdOrCtrl+B"); err != nil {
		t.Fatal(err)
	}
	if err := a.resetCommandBinding(cmdBold); err == nil {
		t.Fatal("默认绑定与其他命令冲突时，单项恢复应失败")
	}
	if got, ok := a.prefs.Keymap.Overrides[cmdBold]; !ok || got != "" {
		t.Fatalf("恢复失败后应保留解除绑定覆盖: value=%q exists=%v", got, ok)
	}
	if got := a.commandBinding(cmdItalic); got != "CmdOrCtrl+B" {
		t.Fatalf("冲突命令绑定被意外改写: %q", got)
	}
}

func TestGlobalKeymapHotkeyToggle(t *testing.T) {
	a := newTestApp(t)
	// 解除全局热键：托盘仍可唤起（toggleWindow 不依赖热键）。
	if err := a.setCommandBinding(cmdToggleWindow, ""); err != nil {
		t.Fatal(err)
	}
	if a.commandBinding(cmdToggleWindow) != "" {
		t.Fatal("应解除全局热键")
	}
	// 恢复默认。
	if err := a.resetCommandBinding(cmdToggleWindow); err != nil {
		t.Fatal(err)
	}
	if a.commandBinding(cmdToggleWindow) != "CmdOrCtrl+Shift+Space" {
		t.Fatalf("全局热键默认 ⌘⇧空格: %q", a.commandBinding(cmdToggleWindow))
	}
	// 相同规范化绑定不重复注册（syncGlobalHotkey 幂等）。
	a.ready = true // 模拟运行期；注册失败会保留旧键并提示（原生注册在测试环境跳过状态一致）
	a.hotkeyAcc = "CmdOrCtrl+Shift+Space"
	if a.syncGlobalHotkey() {
		t.Fatal("相同绑定不应重复注册")
	}
}
