package app

import (
	"strings"
	"unicode/utf8"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"zakki/locales"
)

// 系统编辑角色直接操作 TextArea 的块内历史；文档聚焦时改由应用统一执行。
// 其他输入框仍保留系统角色，避免修改搜索词、标签名或设置时误操作笔记。
func (a *app) editMenuItems() []*mygo.MenuItem {
	commands := []struct {
		id, label, accelerator string
		role                   mygo.MenuRole
	}{
		{"undo", locales.FixedUndo(), "CmdOrCtrl+Z", mygo.RoleUndo},
		{"redo", locales.FixedRedo(), "Shift+CmdOrCtrl+Z", mygo.RoleRedo},
		{"cut", locales.FixedCut(), "CmdOrCtrl+X", mygo.RoleCut},
		{"copy", locales.FixedCopy(), "CmdOrCtrl+C", mygo.RoleCopy},
		{"paste", locales.FixedPaste(), "CmdOrCtrl+V", mygo.RolePaste},
		{"selectAll", locales.FixedSelectAll(), "CmdOrCtrl+A", mygo.RoleSelectAll},
	}
	items := make([]*mygo.MenuItem, 0, len(commands)+1)
	for i, command := range commands {
		if i == 2 || i == 5 {
			items = append(items, mygo.Separator())
		}
		item := &mygo.MenuItem{ID: "edit-" + command.id, Label: command.label, Accelerator: command.accelerator}
		if a.documentMenu {
			item.Click = func(*mygo.MenuItem, *mygo.Window) {
				a.withWindow(func() { a.runEditCommand(command.id) })
			}
		} else {
			item.Role = command.role
		}
		items = append(items, item)
		if i == 4 { // 粘贴之后：Typora 式把拷贝族放在一起，这里收整标签正文进剪贴板。
			items = append(items, &mygo.MenuItem{
				ID:          "copy-tab",
				Label:       locales.MenuCopyTabText(),
				Accelerator: "Shift+CmdOrCtrl+C",
				Click: func(*mygo.MenuItem, *mygo.Window) {
					text := a.activeTab().Text
					if a.clipSvcOK {
						a.clipSvc.WriteClipboard(text)
					} else {
						mygo.Clipboard.WriteText(text)
					}
				},
			})
		}
	}
	return items
}

func (a *app) setDocumentMenu(document bool) {
	if a.documentMenu == document {
		return
	}
	a.documentMenu = document
	a.refreshEditMenuRouting()
}

// refreshEditMenuRouting 只切换编辑子菜单在系统角色与应用命令之间的路由，
// 避免焦点变化时整栏重建。
func (a *app) refreshEditMenuRouting() {
	if !a.ready || a.menu == nil {
		return
	}
	roles := map[string]mygo.MenuRole{
		"undo": mygo.RoleUndo, "redo": mygo.RoleRedo,
		"cut": mygo.RoleCut, "copy": mygo.RoleCopy,
		"paste": mygo.RolePaste, "selectAll": mygo.RoleSelectAll,
	}
	for id, role := range roles {
		item := a.menu.ItemByID("edit-" + id)
		if item == nil {
			continue
		}
		if a.documentMenu {
			item.Role = ""
			cid := id
			item.Click = func(*mygo.MenuItem, *mygo.Window) {
				a.withWindow(func() { a.runEditCommand(cid) })
			}
		} else {
			item.Role = role
			item.Click = nil
		}
	}
	mygo.App.SetMenu(a.menu)
}

func (a *app) syncEditorMenu(c *ui.Context) {
	if a.win != nil && !a.win.IsFocused() {
		a.setDocumentMenu(false)
		return
	}
	preview := !a.cfg.Source && a.mdActive < 0 && !a.findOpen && a.renaming == 0 && !a.linkTool.open
	// 已完成本帧构建；换块后必须查询新绑定，上一帧的控件身份已经失效。
	a.setDocumentMenu(c.Resolve(a.editorHandle).Focused() || preview)
}

func (a *app) runEditCommand(command string) {
	if !a.documentMenu {
		return
	}
	switch command {
	case "undo", "redo":
		a.undoLive(command == "redo")
	case "selectAll":
		a.menuSelectAll()
	case "copy", "cut":
		if a.selDoc {
			a.writeDocClipboard()
		} else if a.clipSvcOK {
			text, lo, hi := a.editorSelection()
			a.clipSvc.WriteClipboard(string([]rune(text)[lo:hi]))
		}
		if command == "cut" {
			a.replaceEditorSelection("", editCommand)
		}
	case "paste":
		if a.clipSvcOK {
			text := strings.ReplaceAll(a.clipSvc.ReadClipboard(), "\r\n", "\n")
			if text != "" {
				a.replaceEditorSelection(text, editPaste)
			}
		}
	}
}

// editorSelection 返回控件可见选区；文档级跨块范围由 selDoc 单独处理。
func (a *app) editorSelection() (text string, lo, hi int) {
	if a.cfg.Source {
		text = a.activeTab().Text
		lo, hi = a.editorCaret()
	} else {
		p := a.mdProjection
		text = p.text
		lo, hi = p.visibleOffset(a.mdCaret), p.visibleOffset(a.mdSelEnd)
		if a.selSyncStart >= 0 {
			lo, hi = p.visibleOffset(a.selSyncStart), p.visibleOffset(a.selSyncEnd)
		}
	}
	if lo > hi {
		lo, hi = hi, lo
	}
	n := utf8.RuneCountInString(text)
	return text, clamp(lo, 0, n), clamp(hi, 0, n)
}

func (a *app) replaceEditorSelection(text string, kind editKind) {
	if a.selDoc {
		a.spliceDocSelection(text)
		return
	}
	before := a.liveSnapshot()
	visible, lo, hi := a.editorSelection()
	if lo == hi && text == "" {
		return
	}
	rs := []rune(visible)
	next := string(rs[:lo]) + text + string(rs[hi:])
	caret := lo + utf8.RuneCountInString(text)
	if a.cfg.Source {
		a.applyDocResult(docResult{doc: next, selStart: caret, selEnd: caret, changed: next != visible})
	} else {
		a.mdDraft, caret = a.mdProjection.replaceSelection(next, caret, lo, hi)
		a.publishDraft(caret)
		a.onEdited()
	}
	a.rememberLiveEdit(before, kind)
}

func (a *app) editorContextMenu(m *ui.Menu) {
	for i, command := range []struct{ id, label string }{
		{"undo", locales.FixedUndo()}, {"redo", locales.FixedRedo()}, {"cut", locales.FixedCut()},
		{"copy", locales.FixedCopy()}, {"paste", locales.FixedPaste()}, {"selectAll", locales.FixedSelectAll()},
	} {
		if i == 2 || i == 5 {
			m.Separator()
		}
		if m.Item(command.label).Chosen() {
			// 右键回调处于编辑器构建中；帧末再执行，避免撤销恢复被重复记入历史。
			a.editMenuPending = command.id
		}
	}
}
