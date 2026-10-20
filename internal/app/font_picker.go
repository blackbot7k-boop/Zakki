package app

import (
	"strings"

	"github.com/egoist/mygo/ui"

	"zakki/internal/fonts"
	"zakki/locales"
)

type fontOption struct {
	key, name, status string
}

// fontDisplayOf 字体键的纯展示名（不含安装状态），供设置页 Select 绑定与
// 选项列表共用：绑定值与选项文本必须同源，否则草稿同步会错位。
func fontDisplayOf(kind int, key string) string {
	if family := fonts.LocalFamily(key); family != "" {
		return family
	}
	switch kind {
	case 0:
		switch key {
		case "pingfang":
			return locales.BodyFontPingFang()
		case "songti":
			return locales.BodyFontSongti()
		case "hand":
			return locales.BodyFontHand()
		case "hannotate":
			return locales.BodyFontHannotate()
		default:
			return locales.BodyFontKaiDefault()
		}
	case 1:
		switch key {
		case "":
			return locales.FontFollowChinese()
		case "system":
			return locales.FontSystemWestern()
		case "jetbrains":
			return locales.FontJetBrains()
		case "noteworthy":
			return locales.FontNoteworthy()
		case "bradley":
			return locales.FontBradley()
		default:
			return "Fira Code"
		}
	default:
		switch key {
		case "sfmono":
			return locales.FontSFMono()
		case "menlo":
			return locales.FontMenlo()
		case "fira":
			return "Fira Code"
		default:
			return locales.FontJetBrainsDefault()
		}
	}
}

// fontOptions 合并预设与本机字体：预设按当前安装状态标注，本机字体去重后
// 追加（kind 2 只列等宽家族）。current 保证始终在列表里，避免丢已选值。
func (a *app) fontOptions(kind int, current string) []fontOption {
	var options []fontOption
	switch kind {
	case 0:
		options = []fontOption{{key: "kai"}, {key: "pingfang"}, {key: "songti"}, {key: "hand"}, {key: "hannotate"}}
	case 1:
		options = []fontOption{{key: ""}, {key: "system"}, {key: "jetbrains"}, {key: "noteworthy"}, {key: "bradley"}, {key: "fira"}}
	default:
		options = []fontOption{{key: "jbm"}, {key: "sfmono"}, {key: "menlo"}, {key: "fira"}}
	}
	for i := range options {
		options[i].name = fontDisplayOf(kind, options[i].key)
	}
	seen := map[string]bool{}
	for i := range options {
		if options[i].key == "" {
			continue
		}
		r, ok := fontResource(options[i].key)
		if !ok {
			continue
		}
		seen[strings.ToLower(r.Family)] = true
		options[i].status = locales.FontNeedsDownload()
		if hasFontFamily(a.fonts.families, r.Family) {
			options[i].status = locales.FontInstalled()
		}
	}
	for _, f := range a.fonts.families {
		key := fonts.LocalKey(f.Name)
		if key == "" || seen[strings.ToLower(f.Name)] {
			continue
		}
		seen[strings.ToLower(f.Name)] = true
		if kind == 2 && !f.Monospaced {
			continue
		}
		options = append(options, fontOption{key: key, name: f.Name, status: locales.FontInstalled()})
	}
	for _, o := range options {
		if o.key == current {
			return options
		}
	}
	label := fontDisplayOf(kind, current)
	if current == "" {
		label = locales.FontThemeDefault()
	}
	return append(options, fontOption{key: current, name: label})
}

// fontPicker 向导用的下拉：键直接绑定，含搜索与状态标注，返回是否变化。
func (a *app) fontPicker(c *ui.Context, key *string, kind int, label string, width float32) bool {
	picker := &a.fonts.pickers[kind]
	options := a.fontOptions(kind, *key)
	display := *key
	for _, o := range options {
		if o.key == *key {
			display = o.name
			if o.status != "" {
				display += " · " + o.status
			}
			break
		}
	}
	sel := ui.SelectBase(c, key)
	sel.Trigger.Width(width).MinHeight(32).Padding(6, 10).Radius(7).
		Border(1, c.Theme().Border).Label(label).Children(func() {
		ui.Text(c, display).FontSize(12).Grow(1)
		ui.Text(c, "⌄").FontSize(12).TextColor(tMuted(c))
	})
	sel.Popup(func(panel ui.Element) {
		panel.Width(width+40).Padding(6).Radius(8).
			Background(c.Theme().Background).Border(1, c.Theme().Border).Column().Gap(6).Children(func() {
			ui.SearchField(c, &picker.query).Label(locales.FontSearch()).FillWidth()
			ui.Scroll(c).FillWidth().Height(220).Children(func() {
				ui.Column(c).FillWidth().Gap(2).Children(func() {
					for _, o := range options {
						if picker.query != "" && !strings.Contains(strings.ToLower(o.name), strings.ToLower(picker.query)) {
							continue
						}
						item := sel.Item(o.key).Padding(6, 8).Radius(5).FillWidth()
						if item.Highlighted() {
							item = item.Background(c.Theme().Surface)
						}
						item.Children(func() {
							ui.Text(c, o.name).FontSize(12).Grow(1)
							if o.status != "" {
								ui.Text(c, o.status).FontSize(10).TextColor(tMuted(c))
							}
						})
					}
				})
			})
		})
	})
	return sel.Trigger.Changed()
}

// fontSelectSettings 设置页下拉的选项与「显示名 → 键」映射；显示名来自
// fontDisplayOf，与 syncDraftBindings 的绑定值同源。下载状态不进选项文本，
// 由行下方的提示与准备进度表达。
func (a *app) fontSelectSettings(kind int, current string) ([]string, map[string]string) {
	options := a.fontOptions(kind, current)
	displays := make([]string, 0, len(options))
	keyOf := map[string]string{}
	for _, o := range options {
		if _, dup := keyOf[o.name]; dup && o.key != current {
			continue
		}
		displays = append(displays, o.name)
		keyOf[o.name] = o.key
	}
	return displays, keyOf
}
