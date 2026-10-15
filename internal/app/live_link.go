package app

import (
	"strings"
	"unicode/utf8"

	"github.com/egoist/mygo/ui"
)

// ---- 链接浮层 ----

// linkToolState 链接文本/地址浮层：已有链接填入当前值；无选区要求填写
// 链接文字，不自动读取剪贴板。
type linkToolState struct {
	open  bool
	text  string
	url   string
	err   string
	focus int // FocusBind：打开时聚焦文字框
	start int // 作用范围（文档 rune 偏移）
	end   int
	block int // 焦点块索引（实时预览恢复焦点用）
}

func (a *app) openLinkEditor() {
	a.ensureFocusBlock()
	start, end := a.editorCaret()
	tool := linkToolState{open: true, start: start, end: end, block: a.mdActive}
	doc := a.activeTab().Text
	// 已有链接：光标/选区落在链接 span 内时填入当前值（实时预览在块内找，
	// 避免跨块误配；偏移在块坐标与文档坐标间换算）。
	region := doc
	regionOff := 0
	if !a.cfg.Source {
		blocks := a.mdBlocks()
		if a.mdActive >= 0 && a.mdActive < len(blocks) {
			region = blocks[a.mdActive].raw
			// 块首在文档中的 rune 偏移。
			pos := 0
			for i, b := range blocks {
				if i == a.mdActive {
					regionOff = pos
					break
				}
				pos += utf8.RuneCountInString(b.raw) + 1
			}
		}
	}
	if sp := findLinkSpanAt(region, start-regionOff, end-regionOff); sp != nil {
		tool.text = spanText(region, *sp)
		tool.url = sp.href
		tool.start, tool.end = regionOff+sp.start, regionOff+sp.end
	} else if end > start {
		rs := []rune(doc)
		tool.text = string(rs[clamp(start, 0, len(rs)):clamp(end, 0, len(rs))])
	}
	tool.focus = 1
	a.linkTool = tool
	if a.win != nil && a.winOpen {
		a.win.Update(func() {})
	}
}

func (a *app) closeLinkTool() {
	if a.linkTool.open {
		a.linkTool = linkToolState{}
		if a.win != nil && a.winOpen {
			a.win.Update(func() {})
		}
	}
}

// findLinkSpanAt 找包含 [start,end) 的链接 span（区域相对坐标）。
func findLinkSpanAt(region string, start, end int) *mSpan {
	rs := []rune(region)
	if start > len(rs) {
		return nil
	}
	for _, sp := range inlineSpans(region) {
		if sp.kind != spanLink {
			continue
		}
		if sp.start <= start && end <= sp.end && end > start {
			s := sp
			return &s
		}
		if start == end && sp.start <= start && start <= sp.end {
			s := sp
			return &s
		}
	}
	return nil
}

// applyLinkTool 确认链接浮层：替换选区/已有链接为 [文字](地址)。
func (a *app) applyLinkTool() {
	tool := a.linkTool
	if !tool.open {
		return
	}
	text := strings.TrimSpace(tool.text)
	url := strings.TrimSpace(tool.url)
	if text == "" {
		a.linkTool.err = "请填写链接文字"
		if a.win != nil && a.winOpen {
			a.win.Update(func() {})
		}
		return
	}
	if url == "" {
		a.linkTool.err = "请填写链接地址"
		if a.win != nil && a.winOpen {
			a.win.Update(func() {})
		}
		return
	}
	markdown := "[" + text + "](" + url + ")"
	before := a.liveSnapshot()
	a.pendingEditKind = editCommand
	rs := []rune(a.activeTab().Text)
	start, end := clamp(tool.start, 0, len(rs)), clamp(tool.end, 0, len(rs))
	if end < start {
		start, end = end, start
	}
	newDoc := string(rs[:start]) + markdown + string(rs[end:])
	caret := start + utf8.RuneCountInString(markdown)
	a.activeTab().Text = newDoc
	if a.cfg.Source {
		a.srcSelSyncStart, a.srcSelSyncEnd = caret, caret
	} else {
		blocks := a.mdBlocks()
		idx, off := a.blockOfRuneOffset(caret)
		a.mdActive = idx
		a.mdDraft = blocks[idx].raw
		a.selSyncStart, a.selSyncEnd = caret-off, caret-off
		a.liveFocus, a.scrollActive = 1, true
	}
	a.linkTool = linkToolState{}
	a.rememberLiveEdit(before, editCommand)
	a.onEdited()
	if a.win != nil && a.winOpen {
		a.win.Update(func() {})
	}
}

// linkOverlay 链接文本/地址浮层（轻量、非模态）。已有链接填入当前值；
// 无选区要求填写链接文字，不自动读取剪贴板。
func (a *app) linkOverlay(c *ui.Context, t *ui.Theme) {
	ui.Column(c.Key("link-tool")).FillWidth().Padding(10, 12).Gap(8).
		Radius(8).Background(t.Surface).BorderWidth(1).BorderColor(t.Border).
		Label("链接浮层").Children(func() {
		ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "链接文字").TextColor(t.TextMuted)
			ti := ui.TextInput(c.Key("link-text"), &a.linkTool.text).Grow(1)
			ti.FocusBind(&a.linkTool.focus, 1)
			if ti.Submitted() {
				a.applyLinkTool()
			}
		})
		ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "地址").TextColor(t.TextMuted)
			if ui.TextInput(c.Key("link-url"), &a.linkTool.url).Grow(1).Submitted() {
				a.applyLinkTool()
			}
		})
		ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
			if a.linkTool.err != "" {
				ui.Text(c, a.linkTool.err).TextColor(t.Danger).FontSize(12).Grow(1)
			} else {
				ui.Box(c).Grow(1)
			}
			if ui.Button(c.Key("link-cancel"), "取消").Clicked() {
				a.closeLinkTool()
			}
			if ui.Button(c.Key("link-ok"), "确定").Clicked() {
				a.applyLinkTool()
			}
		})
	})
}
