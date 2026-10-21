package app

import (
	"strings"

	"github.com/egoist/mygo/ui"

	"zakki/internal/fonts"
	"zakki/locales"
)

// fontOption 字体下拉的一项：local 为本机字体分组，preview 为样字的字体栈，
// 空串表示不画样字（未安装的可下载字体、「跟随中文字体」）。
type fontOption struct {
	key, name, status, preview string
	local                      bool
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
			return locales.FontJetBrainsDefault()
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
// 按文字归类追加——中文只列含中文字形的家族，英文只列西文家族，代码只列
// 等宽家族。current 保证始终在列表里，避免丢已选值。
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
		options[i].preview = presetFontStack(kind, options[i].key)
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
		} else {
			options[i].preview = ""
		}
	}
	for _, f := range a.fonts.families {
		key := fonts.LocalKey(f.Name)
		if key == "" || seen[strings.ToLower(f.Name)] {
			continue
		}
		seen[strings.ToLower(f.Name)] = true
		if !localFontFits(kind, f) {
			continue
		}
		options = append(options, fontOption{key: key, name: f.Name, status: locales.FontInstalled(), preview: f.Name, local: true})
	}
	for _, o := range options {
		if o.key == canonicalFontKey(kind, current) {
			return options
		}
	}
	label := fontDisplayOf(kind, current)
	if current == "" {
		label = locales.FontThemeDefault()
	}
	return append(options, fontOption{key: current, name: label, local: fonts.LocalFamily(current) != ""})
}

// canonicalFontKey 默认字体的别名对应的预设项：中文与代码字体的空串、sonnet
// 显示为各自的默认字体。只用于显示，不能写回——空串不触发字体下载。
func canonicalFontKey(kind int, key string) string {
	if key != "" && key != "sonnet" {
		return key
	}
	switch kind {
	case 0:
		return "kai"
	case 2:
		return "jbm"
	}
	return key
}

// localFontFits 本机字体是否进入某个下拉：按家族名推断文字，表情与符号字体都不列。
func localFontFits(kind int, f ui.FontFamily) bool {
	script := fonts.ClassifyFamily(f.Name)
	switch kind {
	case 0:
		return script == fonts.ScriptChinese
	case 1:
		return script == fonts.ScriptLatin
	default:
		return f.Monospaced && (script == fonts.ScriptLatin || script == fonts.ScriptChinese)
	}
}

// presetFontStack 预设项样字用的字体栈；「跟随中文字体」不画样字。
func presetFontStack(kind int, key string) string {
	switch kind {
	case 0:
		return cjkFontStack(key)
	case 1:
		return latinFontStack(key)
	default:
		return monoFontStack(key)
	}
}

func fontSample(kind int) string {
	switch kind {
	case 0:
		return locales.FontSampleChinese()
	case 1:
		return locales.FontSampleLatin()
	default:
		return locales.FontSampleCode()
	}
}

// fontPicker 字体下拉（设置页与首启向导共用）：键直接绑定；弹层含搜索、
// 「推荐 / 本机字体」两组，每项用该字体写一行样字并标注安装状态，
// 限高滚动、打开时滚到当前项。返回是否变化。
func (a *app) fontPicker(c *ui.Context, key *string, kind int, label string, width float32) bool {
	t := c.Theme()
	picker := &a.fonts.pickers[kind]
	options := a.fontOptions(kind, *key)
	canonical := canonicalFontKey(kind, *key)
	current := fontOption{name: *key}
	for _, o := range options {
		if o.key == canonical {
			current = o
			break
		}
	}
	sel := ui.SelectBase(c, key)
	sel.Trigger.Width(width).Label(label)
	selectTrigger(c, sel.Trigger, func() {
		ui.Text(c, current.name).SingleLine().Shrink(1)
		if current.key == *key && current.status == locales.FontNeedsDownload() {
			ui.Box(c).Grow(1)
			ui.Text(c, current.status).FontSize(11).TextColor(tMuted(c)).Shrink(0)
		}
	})
	if !sel.Open() {
		picker.query = ""
	}
	query := strings.ToLower(strings.TrimSpace(picker.query))
	sel.Popup(func(panel ui.Element) {
		selectPanel(c, panel)
		panel.Width(max(width, 300)).Column().Gap(4).Children(func() {
			ui.SearchField(c, &picker.query).Label(locales.FontSearch()).FillWidth()
			ui.Scroll(c).FillWidth().MaxHeight(selectMaxHeight).Children(func() {
				ui.Column(c).FillWidth().Gap(1).Children(func() {
					shown := 0
					for _, local := range []bool{false, true} {
						header := false
						for _, o := range options {
							if o.local != local || query != "" && !strings.Contains(strings.ToLower(o.name), query) {
								continue
							}
							if !header {
								header = true
								groupName := locales.FontGroupRecommended()
								if local {
									groupName = locales.FontGroupLocal()
								}
								ui.Text(c, groupName).FontSize(11).Bold().TextColor(tMuted(c)).Padding(6, 8, 2, 8)
							}
							shown++
							fontPickerItem(c, sel, o, kind, canonical, t)
						}
					}
					if shown == 0 {
						ui.Text(c, locales.FontNoMatch()).FontSize(12).TextColor(tMuted(c)).Padding(8)
					}
				})
			})
		})
	})
	return sel.Trigger.Changed()
}

func fontPickerItem(c *ui.Context, sel ui.SelectParts[string], o fontOption, kind int, selected string, t *ui.Theme) {
	item := sel.Item(o.key).FillWidth().Column().AlignItems(ui.Start).Gap(2).Padding(5, 8).Radius(t.Radius)
	fg, muted := t.Text, t.TextMuted
	switch {
	case item.Highlighted():
		item.Background(t.Accent)
		fg, muted = t.AccentText, t.AccentText
	case o.key == selected:
		item.Background(t.Surface)
	}
	item.Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(8).Children(func() {
			ui.Text(c, o.name).FontSize(12).TextColor(fg).SingleLine().Grow(1).Shrink(1)
			if o.status != "" && !o.local {
				ui.Text(c, o.status).FontSize(10).TextColor(muted).Shrink(0)
			}
		})
		if o.preview != "" {
			ui.Text(c, fontSample(kind)).Font(o.preview).FontSize(15).TextColor(fg).SingleLine().FillWidth()
		}
	})
}
