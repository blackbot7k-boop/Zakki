package app

import (
	"strings"
	"testing"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// 菜单契约：应用菜单只提供一个设置入口；设置窗口内部提供常规、外观、标签与外观、
// 备份与数据和快捷键导航；不再有打开配置文件入口；
// Markdown 命令收纳为「段落/格式」两个顶层菜单并展示当前绑定。

func menuTopLabels(menu *mygo.Menu) []string {
	var out []string
	for _, top := range menu.Items() {
		out = append(out, top.Label)
	}
	return out
}

func findMenuItem(menu *mygo.Menu, id string) *mygo.MenuItem { return menu.ItemByID(id) }

func TestMenuStructureNoConfigEntries(t *testing.T) {
	a := newTestApp(t)
	a.winOpen = true
	menu := a.createMenu()

	labels := strings.Join(menuTopLabels(menu), ",")
	for _, want := range []string{"文件", "编辑", "显示"} {
		if !strings.Contains(labels, want) {
			t.Fatalf("缺少顶层菜单 %s：%s", want, labels)
		}
	}
	for _, top := range menu.Items() {
		if top.Role == mygo.RoleAppMenu {
			foundSettings := false
			for _, child := range top.Submenu {
				if child.ID == "app-settings" && strings.Contains(child.Label, "设置") {
					if child.Accelerator != "CmdOrCtrl+," {
						t.Fatalf("设置… 应绑定 ⌘,: %q", child.Accelerator)
					}
					foundSettings = true
				}
				if child.ID == "app-tab-settings" || child.ID == "app-keymap" {
					t.Fatalf("应用菜单不应再有重复的设置入口: %q", child.Label)
				}
				if strings.Contains(child.Label, "配置文件") || strings.Contains(child.Label, "配置") {
					t.Fatalf("应用菜单不应再有配置文件入口: %q", child.Label)
				}
			}
			if !foundSettings {
				t.Fatalf("应用菜单缺少唯一的 设置… 入口: %+v", top.Submenu)
			}
		}
		if top.Label == "文件" {
			var labels []string
			for _, child := range top.Submenu {
				labels = append(labels, child.Label)
				if strings.Contains(child.Label, "配置") || strings.Contains(child.Label, "备份目录") || child.Label == "写回数据文件" {
					t.Fatalf("文件菜单不应再有配置/备份目录/写回入口（迁入设置页）: %q", child.Label)
				}
			}
			joined := strings.Join(labels, ",")
			for _, want := range []string{"立即备份", "新建标签"} {
				if !strings.Contains(joined, want) {
					t.Fatalf("文件菜单缺少 %s：%s", want, joined)
				}
			}
		}
		if top.Label == "编辑" {
			var labels []string
			for _, child := range top.Submenu {
				labels = append(labels, child.Label)
			}
			editJoined := strings.Join(labels, ",")
			for _, want := range []string{"复制标签全文", "查找", "查找下一个", "查找上一个"} {
				if !strings.Contains(editJoined, want) {
					t.Fatalf("编辑菜单缺少 %s：%s", want, editJoined)
				}
			}
		}
	}
}

func TestMenuMarkdownCommandsInParagraphAndFormat(t *testing.T) {
	a := newTestApp(t)
	a.winOpen = true
	menu := a.createMenu()

	var para, format *mygo.MenuItem
	for _, top := range menu.Items() {
		switch top.Label {
		case "段落":
			para = top
		case "格式":
			format = top
		}
	}
	if para == nil || format == nil {
		t.Fatal("缺少段落/格式顶层菜单")
	}
	// 全部 Markdown 命令都在（按组展开），且显示当前绑定。
	wantIDs := map[string]bool{}
	for _, def := range commandRegistry {
		if def.group != "global" {
			wantIDs["cmd-"+def.id] = true
		}
	}
	got := 0
	for _, top := range []*mygo.MenuItem{para, format} {
		for _, item := range top.Submenu {
			if item.ID == "" {
				continue // 分组分隔线
			}
			if !wantIDs[item.ID] {
				t.Fatalf("意外的 Markdown 菜单项 %q", item.ID)
			}
			got++
			defID := strings.TrimPrefix(item.ID, "cmd-")
			if item.Accelerator != a.commandBinding(defID) {
				t.Fatalf("%s 菜单绑定应与 Keymap 一致: %q != %q", defID, item.Accelerator, a.commandBinding(defID))
			}
			if item.Click == nil {
				t.Fatalf("%s 缺少执行入口", defID)
			}
		}
	}
	if got != len(wantIDs) {
		t.Fatalf("段落/格式菜单命令数量不符: %d != %d", got, len(wantIDs))
	}
	// Typora 式分工：块级收段落、行内收格式。
	inPara := map[string]bool{}
	for _, item := range para.Submenu {
		inPara[item.ID] = true
	}
	if inPara["cmd-"+cmdBold] {
		t.Fatal("行内命令应收在格式菜单")
	}
	if !inPara["cmd-"+cmdH1] {
		t.Fatal("标题命令应收在段落菜单")
	}
	// 链接命令拿到 ⌘K；清空不再有键盘绑定。
	if link := findMenuItem(menu, "cmd-"+cmdLink); link.Accelerator != "CmdOrCtrl+K" {
		t.Fatalf("插入链接应默认 ⌘K: %q", link.Accelerator)
	}
	for _, top := range menu.Items() {
		for _, child := range top.Submenu {
			if child.Label == "清空当前标签" && child.Accelerator != "" {
				t.Fatalf("清空当前标签不应有快捷键: %q", child.Accelerator)
			}
		}
	}
}

func TestMenuMarkdownCommandsFollowKeymap(t *testing.T) {
	a := newTestApp(t)
	a.winOpen = true
	if err := a.setCommandBinding(cmdBold, "CmdOrCtrl+Alt+U"); err != nil {
		t.Fatal(err)
	}
	a.buildMenu()
	item := a.menu.ItemByID("cmd-" + cmdBold)
	if item == nil || item.Accelerator != "CmdOrCtrl+Alt+U" {
		t.Fatalf("菜单应展示新绑定: %+v", item)
	}
	// 解除绑定后菜单无 accelerator。
	if err := a.setCommandBinding(cmdBold, ""); err != nil {
		t.Fatal(err)
	}
	a.syncMenus()
	if item.Accelerator != "" {
		t.Fatalf("解除后菜单不应显示绑定: %q", item.Accelerator)
	}
}

func TestMenuMarkdownDisabledWithoutMainWindow(t *testing.T) {
	a := newTestApp(t)
	a.winOpen = false
	a.tabs[0].Text = ""
	menu := a.createMenu()
	a.menu = menu
	item := menu.ItemByID("cmd-" + cmdBold)
	if item == nil {
		t.Fatal("缺少加粗菜单项")
	}
	if !item.Disabled {
		t.Fatal("主窗口未打开时 Markdown 命令应禁用")
	}
	item.Click(item, nil) // 不得 panic
	if a.tabs[0].Text != "" {
		t.Fatal("禁用时不应改文档")
	}
	a.winOpen = true
	a.menu = menu
	a.syncMenus()
	if item.Disabled {
		t.Fatal("主窗口打开后应启用")
	}
}

func TestMenuViewMenuKeepsFixedEntries(t *testing.T) {
	a := newTestApp(t)
	a.winOpen = true
	menu := a.createMenu()
	var viewTop *mygo.MenuItem
	for _, top := range menu.Items() {
		if top.Label == "显示" {
			viewTop = top
		}
	}
	if viewTop == nil {
		t.Fatal("缺少显示菜单")
	}
	var tabsSub *mygo.MenuItem
	for _, child := range viewTop.Submenu {
		switch child.Label {
		case "源码模式", "窗口置顶", "上一个标签", "下一个标签":
		default:
			if child.Label == "切换标签" {
				tabsSub = child
			}
		}
	}
	if tabsSub == nil || len(tabsSub.Submenu) != len(a.tabs) {
		t.Fatal("全部标签应收纳为「切换标签」子菜单")
	}
	for i, item := range tabsSub.Submenu {
		if !strings.Contains(item.Label, a.menuTitle(i)) {
			t.Fatalf("切标签子菜单标题错误: %+v", item)
		}
		if i < 9 && item.Accelerator == "" {
			t.Fatalf("前 9 个标签应有 ⌘ 数字加速键: %+v", item)
		}
		if i >= 9 && item.Accelerator != "" {
			t.Fatalf("第 %d 个标签不应有数字加速键: %+v", i+1, item)
		}
	}
	// 不再有主题/字体/字号/行距/Esc 设置菜单。
	texts := menuTexts(menu)
	for _, banned := range []string{"正文字体", "正文字号", "行距", "按 Esc 隐藏窗口", "Markdown 主题", "跟随系统"} {
		if strings.Contains(texts, banned) {
			t.Fatalf("显示菜单不应再有 %s（迁入设置窗口）", banned)
		}
	}
}

// 角色菜单项默认是英文标签，且「服务」不带子菜单时会成为死项；
// 这里钉住中文化覆盖与服务子菜单，防止回退。
func TestMenuRoleItemsLocalizedAndServicesAlive(t *testing.T) {
	a := newTestApp(t)
	a.winOpen = true
	menu := a.createMenu()

	var appSub, winSub []*mygo.MenuItem
	var fileSub []*mygo.MenuItem
	for _, top := range menu.Items() {
		switch {
		case top.Role == mygo.RoleAppMenu:
			appSub = top.Submenu
		case top.Role == mygo.RoleWindowMenu:
			if top.Label != "窗口" {
				t.Fatalf("窗口菜单顶层标签应中文化: %q", top.Label)
			}
			winSub = top.Submenu
		case top.Label == "文件":
			fileSub = top.Submenu
		}
	}
	wantApp := []string{"关于 " + appName, "服务", "隐藏 " + appName, "隐藏其他", "显示全部", "退出 " + appName}
	var gotApp []string
	for _, child := range appSub {
		if child.Role == mygo.RoleServices && child.Submenu == nil {
			t.Fatal("服务项必须带子菜单，否则 macOS 不填充 servicesMenu")
		}
		switch child.Role {
		case mygo.RoleAbout, mygo.RoleServices, mygo.RoleHide, mygo.RoleHideOthers, mygo.RoleUnhide, mygo.RoleQuit:
			gotApp = append(gotApp, child.Label)
		}
	}
	if strings.Join(gotApp, ",") != strings.Join(wantApp, ",") {
		t.Fatalf("应用菜单角色项应中文化: %v", gotApp)
	}
	var gotWin []string
	for _, child := range winSub {
		gotWin = append(gotWin, child.Label)
	}
	if joined := strings.Join(gotWin, ","); !strings.Contains(joined, "最小化") || !strings.Contains(joined, "缩放") || !strings.Contains(joined, "前置全部窗口") {
		t.Fatalf("窗口菜单子项应中文化: %v", joined)
	}
	for _, child := range fileSub {
		if child.Role == mygo.RoleClose && child.Label != "关闭窗口" {
			t.Fatalf("关闭项应中文化: %q", child.Label)
		}
	}
}

func menuTexts(menu *mygo.Menu) string {
	var sb strings.Builder
	var walk func(items []*mygo.MenuItem)
	walk = func(items []*mygo.MenuItem) {
		for _, it := range items {
			sb.WriteString(it.Label)
			sb.WriteString(",")
			walk(it.Submenu)
		}
	}
	for _, top := range menu.Items() {
		sb.WriteString(top.Label)
		sb.WriteString(",")
		walk(top.Submenu)
	}
	return sb.String()
}

func TestSettingsWindowLifecycleState(t *testing.T) {
	a := newTestApp(t)
	// 打开 Keymap 直接定位快捷键页（不开原生窗口，只验证状态路由）。
	a.settingsPage = ""
	a.openSettings(pageKeymap)
	if a.settingsPage != pageKeymap {
		t.Fatalf("快捷键入口应定位快捷键页: %q", a.settingsPage)
	}
	if a.settingsUI.draft.Appearance != a.prefs.Appearance {
		t.Fatal("打开时应同步草稿")
	}
	// 重复打开只聚焦：状态仍是单实例。
	a.openSettings(pageGeneral)
	if a.settingsPage != pageGeneral {
		t.Fatalf("重复打开应切换页面: %q", a.settingsPage)
	}
}

func TestSettingsViewRendersPages(t *testing.T) {
	a := newTestApp(t)
	a.winOpen = true
	for _, page := range []string{pageGeneral, pageAppearance, pageBackup, pageKeymap} {
		a.settingsPage = page
		a.settingsUI.syncDraftFromPrefs(a.prefs)
		tt := ui.NewTester(a.settingsView, 760, 600)
		tt.Frame()
		switch page {
		case pageGeneral:
			if !tt.HasText("记忆主窗口位置与尺寸") {
				t.Fatalf("常规页缺少设置项: %v", tt.Texts())
			}
			// 置顶走顶栏按钮，Esc 隐藏保持默认；两者不再提供开关。
			if tt.HasText("窗口置顶") || tt.HasText("按 Esc 隐藏窗口") {
				t.Fatalf("常规页不应再有置顶/Esc 开关: %v", tt.Texts())
			}
		case pageAppearance:
			if !tt.HasText("颜色主题") || !tt.HasText("Markdown 排版主题") {
				t.Fatalf("外观页缺少主题选择: %v", tt.Texts())
			}
			if !tt.HasText("纸白") || !tt.HasText("暖砂") || !tt.HasText("雾青") || !tt.HasText("石墨") {
				t.Fatalf("外观页缺少五套颜色主题卡片: %v", tt.Texts())
			}
			// 排版主题已收进下拉，当前选中名可见即可。
			if !tt.HasText("Sonnet（Typora）") {
				t.Fatalf("外观页缺少排版主题下拉: %v", tt.Texts())
			}
		case pageBackup:
			if !tt.HasText("打开备份目录") || !tt.HasText("立即备份") {
				t.Fatalf("备份页缺少操作: %v", tt.Texts())
			}
		case pageKeymap:
			if !tt.HasText("全局显示/隐藏主窗口") || !tt.HasText("加粗") {
				t.Fatalf("快捷键页缺少命令: %v", tt.Texts())
			}
		}
	}
}

func TestSettingsAppearanceInstantApply(t *testing.T) {
	a := newTestApp(t)
	a.winOpen = true
	a.settingsPage = pageAppearance
	a.settingsUI.syncDraftFromPrefs(a.prefs)
	tt := ui.NewTester(a.settingsView, 900, 900)
	tt.Frame()
	if err := tt.Click("颜色主题 石墨"); err != nil {
		t.Fatal(err)
	}
	if a.prefs.Appearance.ColorTheme != "graphite" {
		t.Fatalf("点击颜色主题卡片应即时生效: %+v", a.prefs.Appearance)
	}
	// 排版主题下拉：点开当前值，再点选项。
	if err := tt.Click("Sonnet（Typora）"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("书页"); err != nil {
		t.Fatal(err)
	}
	if a.prefs.Appearance.MarkdownTheme != "book" {
		t.Fatalf("点击排版主题卡片应即时生效: %+v", a.prefs.Appearance)
	}
	a.shutdownPreferences()
	prefs, err := decodePreferences(prefsReadRaw(t, a.preferencesPath()))
	if err != nil || prefs.Appearance.ColorTheme != "graphite" || prefs.Appearance.MarkdownTheme != "book" {
		t.Fatalf("变更应自动保存: %+v %v", prefs, err)
	}
}

func TestSettingsGeneralSwitchesApply(t *testing.T) {
	a := newTestApp(t)
	a.winOpen = true
	a.settingsPage = pageGeneral
	a.settingsUI.syncDraftFromPrefs(a.prefs)
	tt := ui.NewTester(a.settingsView, 760, 600)
	tt.Frame()
	if err := tt.Click("记忆主窗口位置开关"); err != nil {
		t.Fatal(err)
	}
	if a.prefs.Window.RememberBounds {
		t.Fatalf("记忆位置开关应即时生效: %+v", a.prefs.Window)
	}
	a.shutdownPreferences()
	prefs, err := decodePreferences(prefsReadRaw(t, a.preferencesPath()))
	if err != nil || prefs.Window.RememberBounds {
		t.Fatalf("变更应自动保存: %+v %v", prefs, err)
	}
}

func TestSettingsNumberFieldRejectsInvalid(t *testing.T) {
	a := newTestApp(t)
	a.winOpen = true
	a.settingsPage = pageAppearance
	a.settingsUI.syncDraftFromPrefs(a.prefs)
	tt := ui.NewTester(a.settingsView, 760, 600)
	tt.Frame()
	// 直接驱动字段状态（控件绑定的文本工作副本）。
	a.settingsUI.bodySizeText = "abc"
	a.commitFloatField("bodySize")
	if a.settingsUI.bodySizeErr == "" {
		t.Fatal("非法数字必须显示行内错误")
	}
	if a.prefs.Appearance.BodySize != 0 {
		t.Fatalf("非法数字不得提交: %+v", a.prefs.Appearance)
	}
	a.settingsUI.bodySizeText = "21.5"
	a.commitFloatField("bodySize")
	if a.settingsUI.bodySizeErr != "" || a.prefs.Appearance.BodySize != 21.5 {
		t.Fatalf("合法数字应提交: %+v err=%q", a.prefs.Appearance, a.settingsUI.bodySizeErr)
	}
	a.settingsUI.bodySizeText = ""
	a.commitFloatField("bodySize")
	if a.prefs.Appearance.BodySize != 0 {
		t.Fatal("清空 = 跟随主题")
	}
}

func TestKeymapRecordAndConflicts(t *testing.T) {
	a := newTestApp(t)
	a.winOpen = true
	a.settingsPage = pageKeymap
	a.settingsUI.syncDraftFromPrefs(a.prefs)
	tt := ui.NewTester(a.settingsView, 760, 600)
	tt.Frame()
	// 进入录制态。
	if err := tt.Click("更改"); err != nil {
		t.Fatal(err)
	}
	if a.settingsUI.recording == "" {
		t.Fatal("点击更改应进入录制态")
	}
	tt.Frame()
	if !tt.HasText("按下组合键，Esc 取消") {
		t.Fatalf("录制态应有提示: %v", tt.Texts())
	}
	// 通过测试器发送 ⌘⇧E（行内代码默认键，换到加粗上应冲突）。
	target := a.settingsUI.recording
	a.captureKeymapKey(commandDefByID2(target), ui.InputEvent{Kind: ui.InputKeyDown, Key: ui.KeyE, Mods: ui.Super | ui.Shift})
	if a.settingsUI.keyErr == "" || !strings.Contains(a.settingsUI.keyErr, "行内代码") {
		t.Fatalf("应提示与「行内代码」冲突: %q", a.settingsUI.keyErr)
	}
	if a.commandBinding(cmdBold) != "CmdOrCtrl+B" {
		t.Fatalf("冲突不得改键: %q", a.commandBinding(cmdBold))
	}
	// 合法改键：⌘⌥B。
	a.settingsUI.recording = target
	a.captureKeymapKey(commandDefByID2(target), ui.InputEvent{Kind: ui.InputKeyDown, Key: ui.KeyB, Mods: ui.Super | ui.Alt})
	if a.commandBinding(cmdBold) != "CmdOrCtrl+Alt+B" {
		t.Fatalf("合法组合应写入: %q", a.commandBinding(cmdBold))
	}
	// Esc 取消录制。
	a.settingsUI.recording = target
	a.captureKeymapKey(commandDefByID2(target), ui.InputEvent{Kind: ui.InputKeyDown, Key: ui.KeyEscape})
	if a.settingsUI.recording != "" {
		t.Fatal("Esc 应取消录制")
	}
	// 重启持久化。
	a.shutdownPreferences()
	b := newTestApp(t)
	b.notes = a.notes
	b.prefsStore = a.prefsStore
	if err := b.loadPreferences(); err != nil {
		t.Fatal(err)
	}
	if b.commandBinding(cmdBold) != "CmdOrCtrl+Alt+B" {
		t.Fatalf("改键应持久化: %q", b.commandBinding(cmdBold))
	}
}

func commandDefByID2(id string) commandDef {
	def, _ := commandDefByID(id)
	return def
}

func TestKeymapUnbindAndReset(t *testing.T) {
	a := newTestApp(t)
	if err := a.setCommandBinding(cmdRule, "CmdOrCtrl+Alt+R"); err != nil {
		t.Fatal(err)
	}
	if a.commandBinding(cmdRule) != "CmdOrCtrl+Alt+R" {
		t.Fatal("默认未绑定命令应可设置")
	}
	// 固定键冲突。
	if err := a.setCommandBinding(cmdItalic, "CmdOrCtrl+F"); err == nil || !strings.Contains(err.Error(), "查找") {
		t.Fatalf("固定键冲突应显示名称: %v", err)
	}
	// 全局命令不能占用文档命令的键。
	if err := a.setCommandBinding(cmdToggleWindow, "CmdOrCtrl+B"); err == nil || !strings.Contains(err.Error(), "加粗") {
		t.Fatalf("全局键与文档键冲突应拒绝: %v", err)
	}
	// 解除绑定（显式空值）。
	if err := a.setCommandBinding(cmdBold, ""); err != nil {
		t.Fatal(err)
	}
	if a.commandBinding(cmdBold) != "" {
		t.Fatal("解除绑定后应为空")
	}
	a.shutdownPreferences()
	prefs, err := decodePreferences(prefsReadRaw(t, a.preferencesPath()))
	if err != nil {
		t.Fatal(err)
	}
	if over, ok := prefs.Keymap.Overrides[cmdBold]; !ok || over != "" {
		t.Fatalf("显式空绑定必须持久化: %q %v", over, ok)
	}
	// 恢复单项。
	if err := a.resetCommandBinding(cmdBold); err != nil {
		t.Fatal(err)
	}
	if a.commandBinding(cmdBold) != "CmdOrCtrl+B" {
		t.Fatal("恢复单项应回到默认")
	}
	// 恢复全部。
	if err := a.resetKeymap(); err != nil {
		t.Fatal(err)
	}
	if len(a.prefs.Keymap.Overrides) != 0 {
		t.Fatalf("恢复全部应清空覆盖: %+v", a.prefs.Keymap.Overrides)
	}
}
