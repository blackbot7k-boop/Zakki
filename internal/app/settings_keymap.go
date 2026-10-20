package app

// 快捷键页：名称搜索、按 Markdown/全局分组、按键录制、解除绑定、恢复单项、
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

func (a *app) keymapPage(c *ui.Context, t *ui.Theme, dark bool) {
	ui.Column(c).Fill().Grow(1).Gap(10).Children(func() {
		settingsLead(c, locales.KeymapHint())
		ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(8).Children(func() {
			ui.SearchField(c, &a.settingsUI.keySearch).Placeholder(locales.KeymapSearchPlaceholder()).Grow(1)
			if ui.Button(c, locales.KeymapResetAll()).Clicked() {
				a.confirmResetPage(pageKeymap)
			}
		})

		ui.Scroll(c).Fill().Grow(1).ScrollbarInsets(0, 12, 0, 0).Children(func() {
			ui.Column(c).FillWidth().Padding(0, 22, 0, 0).Gap(12).Children(func() {
				for _, groupID := range keymapGroupIDs {
					items := a.keymapGroupItems(groupID)
					if len(items) == 0 {
						continue
					}
					settingsSectionTitle(c, keymapGroupName(groupID))
					ui.Column(c).FillWidth().Gap(6).Children(func() {
						for _, def := range items {
							a.keymapRow(c, def)
						}
					})
				}
				settingsSectionTitle(c, locales.KeymapFixedTitle())
				ui.Text(c, shortcutText(locales.KeymapFixedHint())).
					FontSize(12).TextColor(tMuted(c))
			})
		})

		if a.settingsUI.keyErr != "" {
			ui.Text(c, a.settingsUI.keyErr).FontSize(12).TextColor(t.Danger)
		}
		if a.noticeHotkey != "" {
			ui.Text(c, a.noticeHotkey).FontSize(12).TextColor(t.Warning)
		}
	})
}

// keymapGroupItems 按组返回命令（应用搜索过滤）。
func (a *app) keymapGroupItems(group string) []commandDef {
	query := strings.ToLower(strings.TrimSpace(a.settingsUI.keySearch))
	var out []commandDef
	for _, def := range commandRegistry {
		if def.group != group {
			continue
		}
		if query != "" && !strings.Contains(strings.ToLower(commandDisplayName(def)), query) {
			continue
		}
		out = append(out, def)
	}
	return out
}

// keymapRow 一条命令：名称 + 当前绑定 + 录制/解除/恢复默认。
func (a *app) keymapRow(c *ui.Context, def commandDef) {
	acc := a.commandBinding(def.id)
	b, _ := parseAccelerator(acc)
	display := locales.KeymapUnbound()
	if acc != "" {
		display = b.displayName()
	}
	if a.settingsUI.recording == def.id {
		// 录制态：焦点捕获框，Esc 取消，屏蔽应用命令。
		box := ui.Box(c.Key("record-" + def.id)).FillWidth().Height(32).Radius(6).
			Background(c.Theme().Accent.Mix(c.Theme().Background, 0.9)).
			BorderWidth(1.5).BorderColor(c.Theme().Accent).
			Focusable().AutoFocus().FocusRing(true).
			Label(locales.KeymapRecord()).
			AlignItems(ui.Center).Justify(ui.Center).
			HandleInput(func(ev ui.InputEvent) bool {
				return a.captureKeymapKey(def, ev)
			}).Children(func() {
			ui.Text(c, locales.KeymapRecording()).TextColor(c.Theme().Accent).Bold()
		})
		if !box.Focused() {
			// 焦点被抢走（点击其他控件或空白处）：取消录制并恢复菜单与
			// 全局热键，避免录制态卡在失焦状态（Esc 已送达不了捕获框）。
			a.setKeymapRecording("")
		}
		return
	}
	ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(10).Children(func() {
		ui.Text(c, commandDisplayName(def)).Grow(1)
		badge := ui.Box(c.Key("bind-"+def.id)).MinWidth(88).Padding(2, 8).Radius(5).
			Background(c.Theme().Surface).BorderWidth(1).BorderColor(c.Theme().Border).
			AlignItems(ui.Center).Justify(ui.Center).
			Label(locales.KeymapCurrent(display)).Children(func() {
			ui.Text(c, display).FontSize(12)
		})
		_ = badge
		if ui.Button(c.Key("rec-"+def.id), locales.KeymapChange()).Clicked() {
			a.setKeymapRecording(def.id)
			a.settingsUI.keyErr = ""
		}
		if acc != "" {
			if ui.Button(c.Key("clear-"+def.id), locales.KeymapClear()).Clicked() {
				if err := a.setCommandBinding(def.id, ""); err != nil {
					a.settingsUI.keyErr = err.Error()
				}
			}
		}
		if over, ok := a.prefs.Keymap.Overrides[def.id]; ok {
			_ = over
			if ui.Button(c.Key("reset-"+def.id), locales.KeymapResetOne()).Clicked() {
				if err := a.resetCommandBinding(def.id); err != nil {
					a.settingsUI.keyErr = err.Error()
				}
			}
		}
	})
}

// captureKeymapKey 录键：Esc 取消；合法组合经统一校验后写入。
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
	b := keyBindingFromInput(ev.Mods, ev.Key, runtime.GOOS)
	acc := b.accelerator()
	if err := a.setCommandBinding(def.id, acc); err != nil {
		a.settingsUI.keyErr = commandDisplayName(def) + "：" + err.Error()
	} else {
		a.settingsUI.keyErr = ""
	}
	a.setKeymapRecording("")
	return true
}
