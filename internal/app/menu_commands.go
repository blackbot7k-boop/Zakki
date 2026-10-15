package app

// 命令注册表：菜单、Keymap 设置页与编辑器共用同一份定义。
//
//   - 每个命令有稳定 ID、显示名、默认绑定、上下文条件与执行入口。
//   - Keymap 只开放 Markdown 格式命令与全局唤起；搜索、复制、粘贴、撤销
//     等保持系统惯例，切点等既有操作保持固定绑定，不在 Keymap 开放改键。
//   - 绑定保存为规范化 accelerator 字符串（MyGo 格式），macOS 界面显示符号。
//   - 第一版仅支持单组组合键，不支持多段 chord。

import (
	"fmt"
	"sort"
	"strings"

	"github.com/egoist/mygo/ui"
)

// ---- 命令 ID ----

const (
	cmdBold         = "format.bold"
	cmdItalic       = "format.italic"
	cmdInlineCode   = "format.code"
	cmdLink         = "format.link"
	cmdH1           = "heading.1"
	cmdH2           = "heading.2"
	cmdH3           = "heading.3"
	cmdH4           = "heading.4"
	cmdH5           = "heading.5"
	cmdH6           = "heading.6"
	cmdBody         = "heading.body"
	cmdQuote        = "block.quote"
	cmdBullet       = "list.bullet"
	cmdOrdered      = "list.ordered"
	cmdTask         = "list.task"
	cmdCodeBlock    = "block.code"
	cmdRule         = "insert.rule"
	cmdTable        = "insert.table"
	cmdToggleWindow = "global.toggleWindow"
)

// ---- 绑定（规范化组合键） ----

// keyBinding 是一组规范化的单组组合键（不支持 chord）。
type keyBinding struct {
	mods ui.Modifiers
	key  ui.Key
}

// keyName 是 ui.Key 与 accelerator 名称的互查表（与 MyGo 的 keyName 一致）。
var namedKeyByName = map[string]ui.Key{}
var keyNameByKey = map[ui.Key]string{}

func init() {
	byName := map[string]ui.Key{
		"enter": ui.KeyEnter, "return": ui.KeyEnter, "escape": ui.KeyEscape, "esc": ui.KeyEscape,
		"backspace": ui.KeyBackspace, "tab": ui.KeyTab, "space": ui.KeySpace,
		"delete": ui.KeyDelete, "del": ui.KeyDelete, "insert": ui.KeyInsert,
		"home": ui.KeyHome, "end": ui.KeyEnd, "pageup": ui.KeyPageUp, "pagedown": ui.KeyPageDown,
		"left": ui.KeyLeft, "right": ui.KeyRight, "up": ui.KeyUp, "down": ui.KeyDown,
		"minus": ui.KeyMinus, "equal": ui.KeyEqual, "comma": ui.KeyComma, "period": ui.KeyPeriod,
		"slash": ui.KeySlash, "semicolon": ui.KeySemicolon, "quote": ui.KeyQuote,
		"bracketleft": ui.KeyBracketLeft, "bracketright": ui.KeyBracketRight,
		"backslash": ui.KeyBackslash, "backquote": ui.KeyBackquote,
	}
	for name, key := range byName {
		namedKeyByName[name] = key
	}
	for k := ui.KeyA; k <= ui.KeyZ; k++ {
		keyNameByKey[k] = string(rune('a' + int(k-ui.KeyA)))
		namedKeyByName[keyNameByKey[k]] = k
	}
	for k := ui.Key0; k <= ui.Key9; k++ {
		keyNameByKey[k] = string(rune('0' + int(k-ui.Key0)))
		namedKeyByName[keyNameByKey[k]] = k
	}
	for k := ui.KeyF1; k <= ui.KeyF12; k++ {
		name := fmt.Sprintf("f%d", int(k-ui.KeyF1)+1)
		keyNameByKey[k] = strings.ToUpper(name)
		namedKeyByName[name] = k
	}
	for k, name := range map[ui.Key]string{
		ui.KeyEnter: "Enter", ui.KeyEscape: "Escape", ui.KeyBackspace: "Backspace",
		ui.KeyTab: "Tab", ui.KeySpace: "Space", ui.KeyDelete: "Delete", ui.KeyInsert: "Insert",
		ui.KeyHome: "Home", ui.KeyEnd: "End", ui.KeyPageUp: "PageUp", ui.KeyPageDown: "PageDown",
		ui.KeyLeft: "Left", ui.KeyRight: "Right", ui.KeyUp: "Up", ui.KeyDown: "Down",
		ui.KeyMinus: "-", ui.KeyEqual: "=", ui.KeyComma: ",", ui.KeyPeriod: ".",
		ui.KeySlash: "/", ui.KeySemicolon: ";", ui.KeyQuote: "'",
		ui.KeyBracketLeft: "[", ui.KeyBracketRight: "]", ui.KeyBackslash: "\\", ui.KeyBackquote: "`",
	} {
		keyNameByKey[k] = name
		namedKeyByName[strings.ToLower(name)] = k
	}
}

// acceleratorKey 返回 accelerator 字符串里的键名（字母小写、命名词首字母大写）。
func acceleratorKey(k ui.Key) string { return keyNameByKey[k] }

// parseAccelerator 解析 MyGo 格式 accelerator（如 "CmdOrCtrl+Shift+Space"）
// 为规范化绑定。CmdOrCtrl 按 macOS 语义解析为 Super。
func parseAccelerator(s string) (keyBinding, error) {
	parts := strings.Split(s, "+")
	if strings.TrimSpace(s) == "" {
		return keyBinding{}, fmt.Errorf("空快捷键")
	}
	var b keyBinding
	for i, raw := range parts {
		p := strings.ToLower(strings.TrimSpace(raw))
		if p == "" {
			return keyBinding{}, fmt.Errorf("无效快捷键 %q", s)
		}
		last := i == len(parts)-1
		if !last {
			switch p {
			case "cmdorctrl", "commandorcontrol", "cmd", "command", "super", "meta", "win":
				b.mods |= ui.Super
				continue
			case "ctrl", "control":
				b.mods |= ui.Ctrl
				continue
			case "alt", "opt", "option":
				b.mods |= ui.Alt
				continue
			case "shift":
				b.mods |= ui.Shift
				continue
			}
			return keyBinding{}, fmt.Errorf("未知修饰键 %q", raw)
		}
		if p == "plus" {
			b.key = ui.KeyEqual
			continue
		}
		key, ok := namedKeyByName[p]
		if !ok || len([]rune(p)) != 1 && key == ui.KeyUnknown {
			return keyBinding{}, fmt.Errorf("未知按键 %q", raw)
		}
		b.key = key
	}
	if b.key == ui.KeyUnknown {
		return keyBinding{}, fmt.Errorf("快捷键 %q 缺少主键", s)
	}
	return b, nil
}

// accelerator 输出规范化 accelerator：修饰键顺序 CmdOrCtrl+Ctrl+Alt+Shift，
// 主键字母大写（与菜单/GlobalShortcut 的书写习惯一致）。
func (b keyBinding) accelerator() string {
	name := acceleratorKey(b.key)
	if name == "" {
		return ""
	}
	var parts []string
	if b.mods&ui.Super != 0 {
		parts = append(parts, "CmdOrCtrl")
	}
	if b.mods&ui.Ctrl != 0 {
		parts = append(parts, "Ctrl")
	}
	if b.mods&ui.Alt != 0 {
		parts = append(parts, "Alt")
	}
	if b.mods&ui.Shift != 0 {
		parts = append(parts, "Shift")
	}
	if len(name) == 1 {
		name = strings.ToUpper(name)
	}
	parts = append(parts, name)
	return strings.Join(parts, "+")
}

// normalizeLegacyAccelerator 兼容旧配置里 Super/Cmd 书写的热键
// （统一转成 CmdOrCtrl 规范形式）。
func normalizeLegacyAccelerator(s string) (string, error) {
	b, err := parseAccelerator(s)
	if err != nil {
		return "", err
	}
	return b.accelerator(), nil
}

// sameBinding 比较两个绑定的规范化形态（修饰键集合 + 主键）。
func sameBinding(a, b keyBinding) bool { return a.mods == b.mods && a.key == b.key }

// ---- 显示名（macOS 符号） ----

// displayName 输出面向用户的符号，如 "⌘B"、"⌘⇧空格"、"⌘⌥1"。
func (b keyBinding) displayName() string {
	var sb strings.Builder
	if b.mods&ui.Super != 0 {
		sb.WriteRune('⌘')
	}
	if b.mods&ui.Ctrl != 0 {
		sb.WriteRune('⌃')
	}
	if b.mods&ui.Alt != 0 {
		sb.WriteRune('⌥')
	}
	if b.mods&ui.Shift != 0 {
		sb.WriteRune('⇧')
	}
	sb.WriteString(displayKeyName(b.key))
	return sb.String()
}

func displayKeyName(k ui.Key) string {
	switch k {
	case ui.KeySpace:
		return "空格"
	case ui.KeyEnter:
		return "回车"
	case ui.KeyEscape:
		return "Esc"
	case ui.KeyBackspace:
		return "删除"
	case ui.KeyTab:
		return "Tab"
	case ui.KeyDelete:
		return "⌦"
	case ui.KeyHome:
		return "↖"
	case ui.KeyEnd:
		return "↘"
	case ui.KeyPageUp:
		return "⇞"
	case ui.KeyPageDown:
		return "⇟"
	case ui.KeyLeft:
		return "←"
	case ui.KeyRight:
		return "→"
	case ui.KeyUp:
		return "↑"
	case ui.KeyDown:
		return "↓"
	}
	if name := acceleratorKey(k); name != "" {
		if len(name) == 1 {
			return strings.ToUpper(name)
		}
		return name
	}
	return "?"
}

// ---- 命令定义 ----

// commandDef 是一条可绑定命令。执行统一走 runFormatCommand(id)，
// 避免 registry 在包初始化期引用执行器形成环。
type commandDef struct {
	id     string
	name   string // 显示名（Keymap 与菜单共用）
	group  string // keymap 分组：inline / heading / blocks / insert / global
	def    string // 默认 accelerator（""= 默认未绑定）
	global bool   // 全局命令（后台可触发）而非文档命令
}

// keymapGroups 决定 Keymap 页与 Markdown 菜单的分组顺序。
var keymapGroups = []struct{ id, name string }{
	{"inline", "行内格式"},
	{"heading", "标题"},
	{"blocks", "列表与引用"},
	{"insert", "插入"},
	{"global", "全局"},
}

var commandRegistry = []commandDef{
	{id: cmdBold, name: "加粗", group: "inline", def: "CmdOrCtrl+B"},
	{id: cmdItalic, name: "斜体", group: "inline", def: "CmdOrCtrl+I"},
	{id: cmdInlineCode, name: "行内代码", group: "inline", def: "CmdOrCtrl+Shift+E"},
	{id: cmdLink, name: "插入或编辑链接", group: "inline", def: "CmdOrCtrl+K"},
	{id: cmdH1, name: "标题 1", group: "heading", def: "CmdOrCtrl+1"},
	{id: cmdH2, name: "标题 2", group: "heading", def: "CmdOrCtrl+2"},
	{id: cmdH3, name: "标题 3", group: "heading", def: "CmdOrCtrl+3"},
	{id: cmdH4, name: "标题 4", group: "heading", def: "CmdOrCtrl+4"},
	{id: cmdH5, name: "标题 5", group: "heading", def: "CmdOrCtrl+5"},
	{id: cmdH6, name: "标题 6", group: "heading", def: "CmdOrCtrl+6"},
	{id: cmdBody, name: "正文", group: "heading", def: "CmdOrCtrl+0"},
	{id: cmdQuote, name: "引用", group: "blocks", def: "CmdOrCtrl+Shift+Q"},
	{id: cmdBullet, name: "无序列表", group: "blocks", def: "CmdOrCtrl+Shift+8"},
	{id: cmdOrdered, name: "有序列表", group: "blocks", def: "CmdOrCtrl+Shift+7"},
	{id: cmdTask, name: "任务列表", group: "blocks", def: "CmdOrCtrl+Shift+9"},
	{id: cmdCodeBlock, name: "代码块", group: "insert", def: "CmdOrCtrl+Alt+C"},
	{id: cmdRule, name: "分隔线", group: "insert", def: ""},
	{id: cmdTable, name: "插入表格", group: "insert", def: ""},
	{id: cmdToggleWindow, name: "全局显示/隐藏主窗口", group: "global", def: "CmdOrCtrl+Shift+Space", global: true},
}

func commandDefByID(id string) (commandDef, bool) {
	for _, def := range commandRegistry {
		if def.id == id {
			return def, true
		}
	}
	return commandDef{}, false
}

// ---- 固定绑定（不改键） ----

// fixedBindings 是不开放改键的既有快捷键：Keymap 录键时作为冲突源，
// 让「应用命令与全局命令统一校验」有完整依据。
var fixedBindings = []struct{ acc, name string }{
	{"CmdOrCtrl+F", "查找"},
	{"CmdOrCtrl+G", "查找下一个"},
	{"Shift+CmdOrCtrl+G", "查找上一个"},
	{"Shift+CmdOrCtrl+P", "源码模式"},
	{"Alt+CmdOrCtrl+T", "窗口置顶"},
	{"Ctrl+1", "切换到标签 1"},
	{"Ctrl+2", "切换到标签 2"},
	{"Ctrl+3", "切换到标签 3"},
	{"Ctrl+4", "切换到标签 4"},
	{"Ctrl+5", "切换到标签 5"},
	{"Ctrl+6", "切换到标签 6"},
	{"Ctrl+7", "切换到标签 7"},
	{"Ctrl+8", "切换到标签 8"},
	{"Ctrl+9", "切换到标签 9"},
	{"CmdOrCtrl+T", "新建标签"},
	{"CmdOrCtrl+R", "重命名标签"},
	{"Alt+CmdOrCtrl+Left", "上一个标签"},
	{"Alt+CmdOrCtrl+Right", "下一个标签"},
	{"CmdOrCtrl+S", "立即备份"},
	{"Shift+CmdOrCtrl+C", "复制标签全文"},
	{"CmdOrCtrl+Z", "撤销"},
	{"Shift+CmdOrCtrl+Z", "重做"},
	{"CmdOrCtrl+X", "剪切"},
	{"CmdOrCtrl+C", "复制"},
	{"CmdOrCtrl+V", "粘贴"},
	{"CmdOrCtrl+A", "全选"},
	{"CmdOrCtrl+Q", "退出"},
	{"CmdOrCtrl+W", "关闭窗口"},
	{"CmdOrCtrl+M", "最小化"},
	{"CmdOrCtrl+H", "隐藏应用"},
	{"CmdOrCtrl+,", "设置"},
	{"CmdOrCtrl+Tab", "应用切换"},
}

// ---- 绑定解析与冲突校验 ----

// commandBinding 返回命令的当前生效绑定（""=未绑定）。
func (a *app) commandBinding(id string) string {
	if over, ok := a.prefs.Keymap.Overrides[id]; ok {
		return over
	}
	if def, ok := commandDefByID(id); ok {
		return def.def
	}
	return ""
}

// parsedCommandBinding 返回解析后的绑定；未绑定或解析失败返回 false。
func (a *app) parsedCommandBinding(id string) (keyBinding, bool) {
	acc := a.commandBinding(id)
	if acc == "" {
		return keyBinding{}, false
	}
	b, err := parseAccelerator(acc)
	if err != nil {
		return keyBinding{}, false
	}
	return b, true
}

// validateBinding 校验一个待录组合键：拒绝仅修饰键、裸字母/数字与
// 导航输入键；普通字符键至少要带一个命令修饰键。
func validateBinding(b keyBinding) error {
	if b.key == ui.KeyUnknown {
		return fmt.Errorf("请按下一组完整快捷键（仅修饰键无效）")
	}
	switch b.key {
	case ui.KeyTab, ui.KeyEnter, ui.KeyEscape, ui.KeyBackspace, ui.KeyDelete,
		ui.KeyHome, ui.KeyEnd, ui.KeyPageUp, ui.KeyPageDown,
		ui.KeyLeft, ui.KeyRight, ui.KeyUp, ui.KeyDown:
		return fmt.Errorf("%s 是导航输入键，不能用作快捷键", displayKeyName(b.key))
	}
	cmdMod := b.mods&(ui.Super|ui.Ctrl|ui.Alt) != 0
	if !cmdMod {
		if b.key >= ui.KeyA && b.key <= ui.KeyZ || b.key >= ui.Key0 && b.key <= ui.Key9 {
			return fmt.Errorf("裸字母/数字不能用作快捷键，请加上 ⌘/⌃/⌥ 修饰键")
		}
		switch b.key {
		case ui.KeySpace:
			return fmt.Errorf("空格必须与 ⌘/⌃/⌥ 组合使用")
		}
	}
	return nil
}

// findBindingConflict 返回与已有命令、固定快捷键冲突的名称（""=无冲突）。
// 同一命令自身的旧绑定不算冲突。
func (a *app) findBindingConflict(id string, b keyBinding) string {
	for _, def := range commandRegistry {
		if def.id == id {
			continue
		}
		if other, ok := a.parsedCommandBinding(def.id); ok && sameBinding(other, b) {
			return def.name
		}
	}
	for _, fixed := range fixedBindings {
		if other, err := parseAccelerator(fixed.acc); err == nil && sameBinding(other, b) {
			return fixed.name
		}
	}
	return ""
}

// validateKeymapChange 校验一次改键请求，返回错误信息（nil=允许）。
// 冲突时显示冲突名称，不自动抢占。
func (a *app) validateKeymapChange(id string, acc string) error {
	if acc != "" {
		b, err := parseAccelerator(acc)
		if err != nil {
			return err
		}
		if err := validateBinding(b); err != nil {
			return err
		}
		if name := a.findBindingConflict(id, b); name != "" {
			return fmt.Errorf("快捷键已被「%s」使用", name)
		}
	}
	// 全局命令不允许和文档命令同键：全局键会吞掉文档快捷键。
	if def, ok := commandDefByID(id); ok && def.global && acc != "" {
		b, _ := parseAccelerator(acc)
		for _, other := range commandRegistry {
			if other.global || other.id == id {
				continue
			}
			if ob, ok := a.parsedCommandBinding(other.id); ok && sameBinding(ob, b) {
				return fmt.Errorf("全局快捷键不能与「%s」的文档快捷键相同", other.name)
			}
		}
	}
	return nil
}

// setCommandBinding 通过统一偏好入口写入一条绑定（acc="" 解除绑定）。
func (a *app) setCommandBinding(id string, acc string) error {
	if _, ok := commandDefByID(id); !ok {
		return fmt.Errorf("未知命令 %q", id)
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

// resetCommandBinding 恢复单项默认绑定。
func (a *app) resetCommandBinding(id string) error {
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
	if def, ok := commandDefByID(id); ok {
		return def.def
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
