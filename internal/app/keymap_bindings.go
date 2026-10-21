package app

// 改键的校验与写入：录键合法性、与已有命令和固定键的冲突检测、
// 单项与全部恢复默认。命令定义与默认键见 menu_commands.go。

import (
	"fmt"
	"runtime"
	"sort"

	"github.com/egoist/mygo/ui"

	"zakki/locales"
)

// ---- 绑定解析与冲突校验 ----

// defaultBindingForPlatform 返回命令在目标平台的默认绑定。标题与正文的
// 数字键是唯一的平台差异点：标签键统一占用主修饰键+数字（⌘N / Ctrl+N）后，
// 标题退到各平台次级修饰键——macOS ⌃N，其他平台 Alt+N。
func defaultBindingForPlatform(def commandDef, goos string) string {
	if def.group == "heading" {
		digit := "0" // 正文；标题 id 形如 heading.1..heading.6
		if def.id != cmdBody {
			digit = def.id[len(def.id)-1:]
		}
		if goos == "darwin" {
			return "Ctrl+" + digit
		}
		return "Alt+" + digit
	}
	return def.def
}

// commandBinding 返回当前平台的生效绑定（""=未绑定）。
func (a *app) commandBinding(id string) string {
	return a.commandBindingForPlatform(id, runtime.GOOS)
}

func (a *app) commandBindingForPlatform(id, goos string) string {
	if over, ok := a.prefs.Keymap.Overrides[id]; ok {
		return over
	}
	if def, ok := commandDefByID(id); ok {
		return defaultBindingForPlatform(def, goos)
	}
	return ""
}

// parsedCommandBinding 返回当前平台解析后的绑定；未绑定或解析失败返回 false。
func (a *app) parsedCommandBinding(id string) (keyBinding, bool) {
	return a.parsedCommandBindingForPlatform(id, runtime.GOOS)
}

func (a *app) parsedCommandBindingForPlatform(id, goos string) (keyBinding, bool) {
	acc := a.commandBindingForPlatform(id, goos)
	if acc == "" {
		return keyBinding{}, false
	}
	b, err := parseAcceleratorForPlatform(acc, goos)
	if err != nil {
		return keyBinding{}, false
	}
	return b, true
}

// validateBinding 校验一个待录组合键：拒绝仅修饰键、裸字母/数字与
// 导航输入键；普通字符键至少要带一个命令修饰键。
func validateBinding(b keyBinding) error {
	if b.key == ui.KeyUnknown {
		return fmt.Errorf("%s", locales.KeyErrIncomplete())
	}
	switch b.key {
	case ui.KeyTab, ui.KeyEnter, ui.KeyEscape, ui.KeyBackspace, ui.KeyDelete,
		ui.KeyHome, ui.KeyEnd, ui.KeyPageUp, ui.KeyPageDown,
		ui.KeyLeft, ui.KeyRight, ui.KeyUp, ui.KeyDown:
		return fmt.Errorf("%s", locales.KeyErrNavigation(displayKeyName(b.key)))
	}
	cmdMod := b.mods&(ui.Super|ui.Ctrl|ui.Alt) != 0
	if !cmdMod {
		if b.key >= ui.KeyA && b.key <= ui.KeyZ || b.key >= ui.Key0 && b.key <= ui.Key9 {
			return fmt.Errorf("%s", shortcutText(locales.AccBareLetter()))
		}
		switch b.key {
		case ui.KeySpace:
			return fmt.Errorf("%s", shortcutText(locales.AccSpaceNeedsMod()))
		}
	}
	return nil
}

// findBindingConflict 返回当前平台与已有命令、固定快捷键冲突的名称。
func (a *app) findBindingConflict(id string, b keyBinding) string {
	return a.findBindingConflictForPlatform(id, b, runtime.GOOS)
}

func (a *app) findBindingConflictForPlatform(id string, b keyBinding, goos string) string {
	for _, def := range commandRegistry {
		if def.id == id {
			continue
		}
		if other, ok := a.parsedCommandBindingForPlatform(def.id, goos); ok && sameBinding(other, b) {
			return commandDisplayName(def)
		}
	}
	for _, fixed := range fixedBindingsForPlatform(goos) {
		if other, err := parseAcceleratorForPlatform(fixed.acc, goos); err == nil && sameBinding(other, b) {
			return fixedBindingName(fixed.key)
		}
	}
	return ""
}

// validateKeymapChange 校验当前平台的一次改键请求。
func (a *app) validateKeymapChange(id string, acc string) error {
	return a.validateKeymapChangeForPlatform(id, acc, runtime.GOOS)
}

func (a *app) validateKeymapChangeForPlatform(id string, acc string, goos string) error {
	if acc != "" {
		b, err := parseAcceleratorForPlatform(acc, goos)
		if err != nil {
			return err
		}
		if err := validateBinding(b); err != nil {
			return err
		}
		if name := a.findBindingConflictForPlatform(id, b, goos); name != "" {
			return fmt.Errorf("%s", locales.KeyErrConflict(name))
		}
	}
	// 全局命令不允许和文档命令同键：全局键会吞掉文档快捷键。
	if def, ok := commandDefByID(id); ok && def.global && acc != "" {
		b, _ := parseAcceleratorForPlatform(acc, goos)
		for _, other := range commandRegistry {
			if other.global || other.id == id {
				continue
			}
			if ob, ok := a.parsedCommandBindingForPlatform(other.id, goos); ok && sameBinding(ob, b) {
				return fmt.Errorf("%s", locales.KeyErrConflict(commandDisplayName(other)))
			}
		}
	}
	return nil
}

// setCommandBinding 通过统一偏好入口写入一条绑定（acc="" 解除绑定）。
func (a *app) setCommandBinding(id string, acc string) error {
	if _, ok := commandDefByID(id); !ok {
		return fmt.Errorf("%s", locales.KeyErrUnknownCommand(id))
	}
	if err := a.validateKeymapChange(id, acc); err != nil {
		return err
	}
	return a.updatePreferences(func(p *preferences) {
		if p.Keymap.Overrides == nil {
			p.Keymap.Overrides = map[string]string{}
		}
		if acc == defBindingOf(id) {
			delete(p.Keymap.Overrides, id) // 恢复默认：移除覆盖
		} else {
			p.Keymap.Overrides[id] = acc
		}
	})
}

// resetCommandBinding 恢复单项默认绑定；默认键也必须通过同一冲突校验。
func (a *app) resetCommandBinding(id string) error {
	return a.resetCommandBindingForPlatform(id, runtime.GOOS)
}

func (a *app) resetCommandBindingForPlatform(id, goos string) error {
	def, ok := commandDefByID(id)
	if !ok {
		return fmt.Errorf("%s", locales.KeyErrUnknownCommand(id))
	}
	if err := a.validateKeymapChangeForPlatform(id, defaultBindingForPlatform(def, goos), goos); err != nil {
		return err
	}
	return a.updatePreferences(func(p *preferences) {
		delete(p.Keymap.Overrides, id)
	})
}

// resetKeymap 恢复全部默认绑定。
func (a *app) resetKeymap() error {
	return a.updatePreferences(func(p *preferences) {
		p.Keymap.Overrides = map[string]string{}
	})
}

func defBindingOf(id string) string {
	return defBindingOfForPlatform(id, runtime.GOOS)
}

func defBindingOfForPlatform(id, goos string) string {
	if def, ok := commandDefByID(id); ok {
		return defaultBindingForPlatform(def, goos)
	}
	return ""
}

// sortedOverrideIDs 稳定序列化顺序（测试用）。
func sortedOverrideIDs(m map[string]string) []string {
	ids := make([]string, 0, len(m))
	for id := range m {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}
