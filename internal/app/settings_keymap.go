package app

// 快捷键页：名称搜索、按 Markdown/全局分组、按键录制、恢复单项、
// 恢复全部。录键期间屏蔽应用命令，并临时摘除应用菜单与全局热键（两者都
// 先于窗口按键被消费，见 setKeymapRecording），Esc 取消；冲突与非法组合显示原因。
// 全局热键变更先注册新键，成功后再注销旧键；失败保留旧绑定并提示。

import (
	"runtime"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"zakki/locales"
)

// setKeymapRecording 切换录键态，是进入/退出录制的唯一入口。录制期间临时
// 摘除应用菜单并注销全局热键：macOS 菜单 accelerator 是应用级、全局热键是
// OS 级，都先于窗口按键被消费——不摘除则录制框收不到已占用的组合键
// （冲突提示无法显示），且菜单动作会带副作用执行（备份、建标签甚至退出）。
// 退出录制时重建菜单并按偏好重新注册热键。
func (a *app) setKeymapRecording(id string) {
	if a.settingsUI.recording == id {
		return
	}
	a.settingsUI.recording = id
	if !a.ready {
		return // 事件循环未启动（启动载入/测试）：只走状态，不触达原生 API
	}
	if id != "" {
		mygo.App.SetMenu(nil)
		if a.hotkeyAcc != "" {
			mygo.GlobalShortcut.Unregister(a.hotkeyAcc)
			a.hotkeyAcc = "" // 置空，退出时 syncGlobalHotkey 才会按偏好重新注册
		}
		return
	}
	a.buildMenu()
	a.syncGlobalHotkey()
}

// syncGlobalHotkey 让全局热键与偏好一致：先尝试注册新键，成功后再注销
// 旧键；相同规范化绑定不重复注册；失败保留旧绑定并提示。
// 事件循环未启动（启动载入/测试）时只记录目标状态，不触达原生 API。
func (a *app) syncGlobalHotkey() bool {
	want := a.commandBinding(cmdToggleWindow)
	if !a.ready {
		a.hotkeyAcc = "" // 启动时由 WhenReady 统一注册
		return false
	}
	if want == a.hotkeyAcc {
		return false
	}
	if want != "" {
		b, err := parseAccelerator(want)
		if err != nil {
			a.noticeHotkey = locales.KeymapHotkeyInvalid(err.Error())
			return false
		}
		if err := mygo.GlobalShortcut.Register(want, func() { a.toggleWindow() }); err != nil {
			// 注册失败：保留旧绑定并提示。
			msg := locales.KeymapHotkeyFailed(err.Error())
			if a.hotkeyAcc != "" {
				a.noticeHotkey = msg + locales.KeymapHotkeyKeptOld(b.displayName())
			} else {
				a.noticeHotkey = msg
				// 回滚偏好中的无效绑定？保留用户输入，仅提示；托盘仍可唤起。
			}
			return false
		}
	}
	if a.hotkeyAcc != "" {
		mygo.GlobalShortcut.Unregister(a.hotkeyAcc)
	}
	a.hotkeyAcc = want
	a.noticeHotkey = ""
	return true
}

// ---- 快捷键页 ----

// keymapConflict 录到或恢复出的组合键已被另一条可改键命令占用：在该行下方
// 就地询问是否改用到这里（同时解除对方）。
type keymapConflict struct {
	id, acc, other string
}

func (a *app) keymapPage(c *ui.Context, t *ui.Theme, dark bool) {
	st := &a.settingsUI
	modified := a.keymapModifiedCount()
	query := strings.ToLower(strings.TrimSpace(st.keySearch))
	ui.Column(c).Fill().Grow(1).Gap(10).Children(func() {
		settingsLead(c, locales.KeymapHint())
		ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(8).Children(func() {
			ui.SearchField(c, &st.keySearch).Label(locales.KeymapSearchPlaceholder()).Placeholder(locales.KeymapSearchPlaceholder()).Grow(1)
			if modified > 0 {
				ui.Text(c, locales.KeymapModified(modified)).FontSize(12).TextColor(tMuted(c)).Shrink(0)
			}
			if ui.Button(c.Key("km-reset-all"), locales.KeymapResetAll()).Disabled(modified == 0).Clicked() {
				a.confirmResetPage(pageKeymap)
			}
		})

		ui.Scroll(c).Fill().Grow(1).ScrollbarInsets(0, 12, 0, 0).Children(func() {
			ui.Column(c).FillWidth().Padding(0, 22, 0, 0).Gap(2).Children(func() {
				for _, groupID := range keymapGroupIDs {
					items := a.keymapGroupItems(groupID, query)
					if len(items) == 0 {
						continue
					}
					settingsSectionTitle(c, keymapGroupName(groupID))
					for _, def := range items {
						a.keymapRow(c, def)
					}
				}
				keymapFixedRows(c, query)
			})
		})

		if st.keyErr != "" && st.keyErrID == "" {
			ui.Text(c, st.keyErr).FontSize(12).TextColor(t.Danger)
		}
		if a.noticeHotkey != "" {
			ui.Text(c, a.noticeHotkey).FontSize(12).TextColor(t.Warning)
		}
	})
}

// keymapModifiedCount 偏离默认键的可改键命令数（含解除绑定）。
func (a *app) keymapModifiedCount() int {
	n := 0
	for _, def := range commandRegistry {
		if _, ok := a.prefs.Keymap.Overrides[def.id]; ok {
			n++
		}
	}
	return n
}

// keymapGroupItems 按组返回命令；query 同时匹配命令名与当前快捷键文字。
func (a *app) keymapGroupItems(group, query string) []commandDef {
	var out []commandDef
	for _, def := range commandRegistry {
		if def.group != group {
			continue
		}
		if query != "" && !keymapMatches(query, commandDisplayName(def), a.bindingDisplay(def.id)) {
			continue
		}
		out = append(out, def)
	}
	return out
}

func keymapMatches(query string, texts ...string) bool {
	for _, s := range texts {
		if strings.Contains(strings.ToLower(s), query) {
			return true
		}
	}
	return false
}

// bindingDisplay 命令当前快捷键的显示文字；未绑定为空串。
func (a *app) bindingDisplay(id string) string {
	if b, ok := a.parsedCommandBinding(id); ok {
		return b.displayName()
	}
	return ""
}

// keymapRow 一条命令：偏离默认绑定时行首有圆点并显示「恢复默认」，
// 回到默认即消失。冲突与错误显示在行下方。
func (a *app) keymapRow(c *ui.Context, def commandDef) {
	st := &a.settingsUI
	b, bound := a.parsedCommandBinding(def.id)
	_, overridden := a.prefs.Keymap.Overrides[def.id]
	// 只有当前绑定偏离平台默认时才提供「恢复默认」。
	deviated := overridden && (!bound || b.acceleratorForPlatform(runtime.GOOS) != defaultBindingForPlatform(def, runtime.GOOS))
	recording := st.recording == def.id
	ui.Column(c).FillWidth().Gap(4).Children(func() {
		row := ui.Row(c.Key("km-row-"+def.id)).FillWidth().MinHeight(34).AlignItems(ui.Center).
			Gap(6).Padding(2, 8).Radius(6)
		hovered := row.Hovered()
		if hovered || recording {
			row.Background(c.Theme().Surface)
		}
		row.Children(func() {
			dot := ui.Box(c).Width(6).Height(6).Radius(3).Shrink(0)
			if overridden {
				dot.Background(c.Theme().Accent)
			}
			ui.Text(c, commandDisplayName(def)).Grow(1)
			if recording {
				a.keymapRecorder(c, def)
				return
			}
			ui.Box(c).Width(24).Height(24).Shrink(0).Children(func() {
				if deviated {
					keymapIconButton(c, "km-reset-"+def.id, iconRotateCCW, locales.KeymapResetOne(), func() { a.resetKeymapRow(def) })
				}
			})
			a.keymapBadge(c, def, b, bound)
		})
		if cf := st.keyConflict; cf.id == def.id {
			a.keymapConflictPanel(c, cf)
		}
		if st.keyErrID == def.id && st.keyErr != "" {
			ui.Text(c, st.keyErr).FontSize(12).TextColor(c.Theme().Danger).Padding(0, 22)
		}
	})
}

// keymapBadge 当前快捷键：每个修饰键与主键一枚键帽，整体可点击进入录制。
func (a *app) keymapBadge(c *ui.Context, def commandDef, b keyBinding, bound bool) {
	st := &a.settingsUI
	display := locales.KeymapUnbound()
	if bound {
		display = b.displayName()
	}
	btn := ui.ButtonBase(c.Key("km-bind-"+def.id)).Label(locales.KeymapChangeFor(commandDisplayName(def), display)).
		MinWidth(96).Height(26).Padding(0, 6).Radius(6).Gap(3).Shrink(0).
		AlignItems(ui.Center).Justify(ui.End).Cursor(ui.CursorPointer).FocusRing(true)
	if btn.Hovered() {
		btn.Background(c.Theme().Background).BorderWidth(1).BorderColor(c.Theme().Border)
	}
	btn.Children(func() {
		if !bound {
			ui.Text(c, locales.KeymapClickToSet()).FontSize(12).TextColor(tMuted(c))
			return
		}
		keymapCaps(c, b, false)
	})
	btn.OnClick(func() {
		st.keyConflict, st.keyErr, st.keyErrID = keymapConflict{}, "", ""
		a.setKeymapRecording(def.id)
	})
}

// keymapCaps 按平台顺序把组合键拆成键帽。
func keymapCaps(c *ui.Context, b keyBinding, muted bool) {
	for _, part := range bindingCaps(b, runtime.GOOS) {
		keycap := ui.Box(c).MinWidth(20).Height(20).Padding(0, 5).Radius(4).
			Background(c.Theme().Surface).BorderWidth(1).BorderColor(c.Theme().Border).
			AlignItems(ui.Center).Justify(ui.Center).Shrink(0)
		keycap.Children(func() {
			text := ui.Text(c, part).FontSize(12)
			if muted {
				text.TextColor(tMuted(c))
			}
		})
	}
}

// bindingCaps 组合键的键帽文字，顺序与 displayNameForPlatform 一致。
func bindingCaps(b keyBinding, goos string) []string {
	var out []string
	if goos == "darwin" {
		for _, m := range []struct {
			mod ui.Modifiers
			cap string
		}{{ui.Super, "⌘"}, {ui.Ctrl, "⌃"}, {ui.Alt, "⌥"}, {ui.Shift, "⇧"}} {
			if b.mods&m.mod != 0 {
				out = append(out, m.cap)
			}
		}
		return append(out, displayKeyName(b.key))
	}
	for _, m := range []struct {
		mod ui.Modifiers
		cap string
	}{{ui.Super, "Win"}, {ui.Ctrl, "Ctrl"}, {ui.Alt, "Alt"}, {ui.Shift, "Shift"}} {
		if b.mods&m.mod != 0 {
			out = append(out, m.cap)
		}
	}
	return append(out, displayKeyNameForPlatform(b.key, goos))
}

func keymapIconButton(c *ui.Context, key string, icon *ui.SVG, label string, onClick func()) {
	btn := ui.ButtonBase(c.Key(key)).Label(label).Tooltip(label).Size(24, 24).Radius(5).
		AlignItems(ui.Center).Justify(ui.Center).Cursor(ui.CursorPointer)
	if btn.Hovered() {
		btn.Background(c.Theme().Border.Mix(c.Theme().Background, 0.5))
	}
	btn.Children(func() {
		ui.Icon(c, icon).Size(13, 13).TextColor(tMuted(c))
	})
	btn.OnClick(onClick)
}

// keymapRecorder 录制态：原快捷键位置换成焦点捕获框，Esc 取消，失焦也取消。
func (a *app) keymapRecorder(c *ui.Context, def commandDef) {
	box := ui.Box(c.Key("record-" + def.id)).Width(168).Height(26).Radius(6).Shrink(0).
		Background(c.Theme().Accent.Mix(c.Theme().Background, 0.9)).
		BorderWidth(1.5).BorderColor(c.Theme().Accent).
		Focusable().AutoFocus().FocusRing(false).
		Label(locales.KeymapRecord()).
		AlignItems(ui.Center).Justify(ui.Center).
		HandleInput(func(ev ui.InputEvent) bool {
			return a.captureKeymapKey(def, ev)
		}).Children(func() {
		ui.Text(c, locales.KeymapRecording()).FontSize(12).TextColor(c.Theme().Accent).Bold()
	})
	if !box.Focused() {
		// 焦点被抢走（点击其他控件或空白处）：取消录制并恢复菜单与
		// 全局热键，避免录制态卡在失焦状态（Esc 已送达不了捕获框）。
		a.setKeymapRecording("")
	}
}

// keymapConflictPanel 行下方的冲突提示：改用到这里会在同一次写入里解除对方。
func (a *app) keymapConflictPanel(c *ui.Context, cf keymapConflict) {
	st := &a.settingsUI
	other, _ := commandDefByID(cf.other)
	b, _ := parseAccelerator(cf.acc)
	t := c.Theme()
	ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(8).Padding(6, 10).Radius(7).
		Background(t.Warning.Mix(t.Background, 0.88)).BorderWidth(1).BorderColor(t.Warning.Mix(t.Background, 0.6)).
		Children(func() {
			ui.Text(c, locales.KeymapInUse(b.displayName(), commandDisplayName(other))).FontSize(12).Grow(1)
			if ui.Button(c.Key("km-steal-"+cf.id), locales.KeymapUseHere(commandDisplayName(other))).Clicked() {
				st.keyConflict = keymapConflict{}
				if err := a.reassignCommandBinding(cf.id, cf.acc, cf.other); err != nil {
					st.keyErr, st.keyErrID = err.Error(), cf.id
				}
			}
			if ui.Button(c.Key("km-steal-cancel-"+cf.id), locales.CommonCancel()).Clicked() {
				st.keyConflict = keymapConflict{}
			}
		})
}

// resetKeymapRow 恢复单项默认；默认键已被别的命令占用时就地询问。
func (a *app) resetKeymapRow(def commandDef) {
	st := &a.settingsUI
	st.keyConflict, st.keyErr, st.keyErrID = keymapConflict{}, "", ""
	acc := defBindingOf(def.id)
	if b, err := parseAccelerator(acc); acc != "" && err == nil {
		if other, ok := a.commandUsingBinding(def.id, b); ok {
			st.keyConflict = keymapConflict{id: def.id, acc: acc, other: other.id}
			return
		}
	}
	if err := a.resetCommandBinding(def.id); err != nil {
		st.keyErr, st.keyErrID = err.Error(), def.id
	}
}

// keymapFixedRows 不能改的快捷键，作为灰色只读行列在最后，搜索同样生效；
// 切换到第 1–9 个标签合成一行。
func keymapFixedRows(c *ui.Context, query string) {
	type fixedRow struct {
		name     string
		bindings []keyBinding
	}
	var rows []fixedRow
	var tabs []keyBinding
	for _, f := range fixedBindingsForPlatform(runtime.GOOS) {
		b, err := parseAccelerator(f.acc)
		if err != nil {
			continue
		}
		if strings.HasPrefix(f.key, "tab") && len(f.key) == 4 {
			tabs = append(tabs, b)
			continue
		}
		rows = append(rows, fixedRow{name: fixedBindingName(f.key), bindings: []keyBinding{b}})
	}
	if len(tabs) > 0 {
		rows = append([]fixedRow{{name: locales.FixedSwitchTabs(), bindings: []keyBinding{tabs[0], tabs[len(tabs)-1]}}}, rows...)
	}
	shown := rows[:0]
	for _, r := range rows {
		display := ""
		for _, b := range r.bindings {
			display += b.displayName() + " "
		}
		if query == "" || keymapMatches(query, r.name, display) {
			shown = append(shown, r)
		}
	}
	if len(shown) == 0 {
		return
	}
	settingsSectionTitle(c, locales.KeymapFixedTitle())
	for _, r := range shown {
		ui.Row(c).FillWidth().MinHeight(30).AlignItems(ui.Center).Gap(6).Padding(2, 14, 2, 8).Children(func() {
			ui.Icon(c, iconLock).Size(12, 12).TextColor(tMuted(c)).Shrink(0)
			ui.Text(c, r.name).TextColor(tMuted(c)).Grow(1)
			for i, b := range r.bindings {
				if i > 0 {
					ui.Text(c, "–").TextColor(tMuted(c))
				}
				keymapCaps(c, b, true)
			}
		})
	}
}

// captureKeymapKey 录键：Esc 取消；被其他命令占用时退出录制并就地询问；
// 非法组合留在录制态并在行下方提示原因，可直接重按。
func (a *app) captureKeymapKey(def commandDef, ev ui.InputEvent) bool {
	if ev.Kind != ui.InputKeyDown {
		return true // 吞掉其他事件，屏蔽应用命令
	}
	if ev.Key == ui.KeyEscape {
		a.setKeymapRecording("")
		return true
	}
	if ev.Key == ui.KeyUnknown {
		return true // 仅修饰键按下：等待完整组合
	}
	st := &a.settingsUI
	b := keyBindingFromInput(ev.Mods, ev.Key, runtime.GOOS)
	acc := b.accelerator()
	st.keyConflict, st.keyErr, st.keyErrID = keymapConflict{}, "", ""
	if validateBinding(b) == nil {
		if other, ok := a.commandUsingBinding(def.id, b); ok {
			st.keyConflict = keymapConflict{id: def.id, acc: acc, other: other.id}
			a.setKeymapRecording("")
			return true
		}
	}
	if err := a.setCommandBinding(def.id, acc); err != nil {
		st.keyErr, st.keyErrID = err.Error(), def.id
		return true
	}
	a.setKeymapRecording("")
	return true
}
