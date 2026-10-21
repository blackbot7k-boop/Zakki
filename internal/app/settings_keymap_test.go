package app

// 录键态临时摘除应用菜单并注销全局热键：macOS 菜单 accelerator 是应用级、
// 全局热键是 OS 级，都先于窗口按键被消费——不摘除则录制框收不到已占用
// 的组合键（冲突提示无法显示），且菜单动作会带副作用执行。退出录制统一恢复。
//
// ready=true 的分支只做簿记断言：测试环境无事件循环，偏好写入（applyTheme）
// 与热键注册（onMain）都会永久阻塞，录键写入路径在 ready=false 下覆盖。

import (
	"testing"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"zakki/locales"
)

// 录制态切换与录键各退出路径（Esc、成功写入）都经 setKeymapRecording。
func TestKeymapRecordingStateTransitions(t *testing.T) {
	a := newTestApp(t)

	a.setKeymapRecording(cmdBold)
	a.setKeymapRecording(cmdBold) // 重复进入应无副作用
	if a.settingsUI.recording != cmdBold {
		t.Fatalf("应进入录制态: %q", a.settingsUI.recording)
	}

	// Esc 取消。
	a.captureKeymapKey(commandDefByID2(cmdBold), ui.InputEvent{Kind: ui.InputKeyDown, Key: ui.KeyEscape})
	if a.settingsUI.recording != "" {
		t.Fatal("Esc 应取消录制")
	}
	a.setKeymapRecording("") // 重复退出应无副作用

	// 录键成功：写入绑定并退出录制态。
	a.setKeymapRecording(cmdBold)
	a.captureKeymapKey(commandDefByID2(cmdBold), ui.InputEvent{Kind: ui.InputKeyDown, Key: ui.KeyB, Mods: ui.Super | ui.Alt})
	if a.settingsUI.recording != "" {
		t.Fatal("录键后应退出录制态")
	}
	if got := a.commandBinding(cmdBold); got != "CmdOrCtrl+Alt+B" {
		t.Fatalf("录制 ⌘⌥B 应写入: %q", got)
	}
}

// 失焦取消录制：点击其他控件或空白处后捕获框失去焦点，录制随之取消，
// 避免录制态卡在失焦状态（Esc 已送达不了捕获框，菜单也回不来）。
func TestKeymapRecordingBlurCancels(t *testing.T) {
	a := newTestApp(t)
	a.winOpen = true
	a.settingsPage = pageKeymap
	a.settingsUI.syncDraftFromPrefs(a.prefs)
	tt := ui.NewTester(a.settingsView, 760, 600)
	tt.Frame()
	if err := tt.Click(keymapBadgeLabel(a, cmdBold)); err != nil {
		t.Fatal(err)
	}
	if a.settingsUI.recording == "" {
		t.Fatal("点击快捷键应进入录制态")
	}
	tt.Frame()
	if a.settingsUI.recording == "" {
		t.Fatal("捕获框出生帧应持有焦点，录制态不得误取消")
	}
	if !tt.Focused("录制快捷键") {
		t.Fatal("捕获框应持有焦点")
	}
	// 点击命令名文本（不可聚焦）：引擎清空焦点，录制随失焦取消。
	if err := tt.Click("斜体"); err != nil {
		t.Fatal(err)
	}
	if a.settingsUI.recording != "" {
		t.Fatalf("失焦应取消录制: %q", a.settingsUI.recording)
	}
}

// ready=true：进入录制摘除菜单并注销全局热键；录制期间菜单重建被跳过；
// 退出录制恢复菜单。
func TestKeymapRecordingSuppressesMenuAndHotkey(t *testing.T) {
	a := newTestApp(t)
	// 先在未 ready 时解除全局唤起：恢复路径经 syncGlobalHotkey 的「目标为空」
	// 早退，不触达原生注册（测试环境会阻塞）。
	if err := a.setCommandBinding(cmdToggleWindow, ""); err != nil {
		t.Fatal(err)
	}
	a.ready = true // 模拟运行期：菜单与热键簿记走真实分支
	a.menu = a.createMenu()
	mygo.App.SetMenu(a.menu)
	t.Cleanup(func() { mygo.App.SetMenu(nil) })
	a.hotkeyAcc = "CmdOrCtrl+Shift+Space" // 模拟已注册的全局热键

	a.setKeymapRecording(cmdBold)
	if a.settingsUI.recording != cmdBold {
		t.Fatalf("应进入录制态: %q", a.settingsUI.recording)
	}
	if mygo.App.Menu() != nil {
		t.Fatal("录制期间应摘除应用菜单")
	}
	if a.hotkeyAcc != "" {
		t.Fatalf("录制期间应注销全局热键: %q", a.hotkeyAcc)
	}

	// 录制期间的菜单重建触发点（标签变化、语言切换）不得把菜单挂回来。
	a.rebuildTabMenus()
	if mygo.App.Menu() != nil {
		t.Fatal("录制期间应跳过菜单重建")
	}

	// Esc 取消：走同一恢复路径，菜单重建恢复。
	a.captureKeymapKey(commandDefByID2(cmdBold), ui.InputEvent{Kind: ui.InputKeyDown, Key: ui.KeyEscape})
	if a.settingsUI.recording != "" {
		t.Fatal("Esc 应取消录制")
	}
	if mygo.App.Menu() == nil {
		t.Fatal("退出录制应恢复应用菜单")
	}
}

// keymapBadgeLabel 快捷键徽标的无障碍名称（点击它进入录制）。
func keymapBadgeLabel(a *app, id string) string {
	def := commandDefByID2(id)
	display := a.bindingDisplay(id)
	if display == "" {
		display = locales.KeymapUnbound()
	}
	return locales.KeymapChangeFor(commandDisplayName(def), display)
}

// 冲突时就地改用：对方解除绑定，本命令拿到组合键，一次写入完成。
func TestKeymapReassignFromConflict(t *testing.T) {
	a := newTestApp(t)
	a.setKeymapRecording(cmdItalic)
	a.captureKeymapKey(commandDefByID2(cmdItalic), ui.InputEvent{Kind: ui.InputKeyDown, Key: ui.KeyB, Mods: ui.Super})
	cf := a.settingsUI.keyConflict
	if cf.id != cmdItalic || cf.other != cmdBold || a.settingsUI.recording != "" {
		t.Fatalf("⌘B 应提示与加粗冲突并退出录制: %+v rec=%q", cf, a.settingsUI.recording)
	}
	if err := a.reassignCommandBinding(cf.id, cf.acc, cf.other); err != nil {
		t.Fatal(err)
	}
	if a.commandBinding(cmdItalic) != "CmdOrCtrl+B" || a.commandBinding(cmdBold) != "" {
		t.Fatalf("改用后斜体应为 ⌘B、加粗解除: italic=%q bold=%q", a.commandBinding(cmdItalic), a.commandBinding(cmdBold))
	}
	// 加粗恢复默认时 ⌘B 被斜体占用：就地询问而不是直接失败。
	a.resetKeymapRow(commandDefByID2(cmdBold))
	if a.settingsUI.keyConflict.other != cmdItalic {
		t.Fatalf("恢复默认撞键应就地询问: %+v", a.settingsUI.keyConflict)
	}
}

// 固定键不能让出：录到 ⌘F 留在录制态并在行下方提示。
func TestKeymapFixedKeyStaysRecording(t *testing.T) {
	a := newTestApp(t)
	a.setKeymapRecording(cmdBold)
	a.captureKeymapKey(commandDefByID2(cmdBold), ui.InputEvent{Kind: ui.InputKeyDown, Key: ui.KeyF, Mods: ui.Super})
	if a.settingsUI.recording != cmdBold || a.settingsUI.keyErrID != cmdBold || a.settingsUI.keyErr == "" {
		t.Fatalf("固定键应留在录制态并就地提示: rec=%q id=%q err=%q", a.settingsUI.recording, a.settingsUI.keyErrID, a.settingsUI.keyErr)
	}
	if a.settingsUI.keyConflict != (keymapConflict{}) {
		t.Fatalf("固定键不应给出改用选项: %+v", a.settingsUI.keyConflict)
	}
}
