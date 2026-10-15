package app

import (
	"testing"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// 按实际菜单配置分发：系统角色发送编辑命令，自定义项目调用 Click。
func chooseEditMenu(t *testing.T, a *app, tt *ui.Tester, command string) {
	t.Helper()
	roles := map[string]mygo.MenuRole{
		"undo": mygo.RoleUndo, "redo": mygo.RoleRedo,
		"copy": mygo.RoleCopy, "cut": mygo.RoleCut,
		"paste": mygo.RolePaste, "selectAll": mygo.RoleSelectAll,
	}
	labels := map[string]string{
		"undo": "撤销", "redo": "重做", "copy": "复制", "cut": "剪切",
		"paste": "粘贴", "selectAll": "全选",
	}
	for _, top := range a.createMenu().Items() {
		if top.Label != "编辑" {
			continue
		}
		for _, item := range top.Submenu {
			if item.Role != roles[command] && item.Label != labels[command] {
				continue
			}
			if item.Role != "" {
				tt.Command(command)
			} else if item.Click != nil {
				item.Click(item, nil)
			} else {
				t.Fatalf("编辑菜单 %q 没有执行入口", command)
			}
			tt.Frame()
			return
		}
	}
	t.Fatalf("找不到编辑菜单 %q", command)
}

func TestEditMenuUndoRestoresThreeDeletedParagraphs(t *testing.T) {
	const text = "甲行\n\n乙行\n\n丙行"
	for _, source := range []bool{false, true} {
		t.Run(map[bool]string{false: "实时预览", true: "源码"}[source], func(t *testing.T) {
			a := newTestApp(t)
			t.Cleanup(a.shutdown)
			a.cfg.Source = source
			a.tabs[0].Text = text
			tt := ui.NewTester(a.view, 500, 600)
			chooseEditMenu(t, a, tt, "selectAll")
			tt.Key(0, ui.KeyBackspace)
			if got := a.tabs[0].Text; got != "" {
				t.Fatalf("全选删除后=%q, want 空文档", got)
			}
			chooseEditMenu(t, a, tt, "undo")
			if got := a.tabs[0].Text; got != text {
				t.Fatalf("菜单撤销三行后=%q, want %q", got, text)
			}
			chooseEditMenu(t, a, tt, "redo")
			if got := a.tabs[0].Text; got != "" {
				t.Fatalf("菜单重做后=%q, want 空文档", got)
			}
		})
	}
}

func TestUndoRestoresDocumentSelection(t *testing.T) {
	const text = "甲\n\n乙\n\n丙"
	a := newTestApp(t)
	t.Cleanup(a.shutdown)
	a.tabs[0].Text = text
	tt := ui.NewTester(a.view, 500, 600)
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Key(0, ui.KeyBackspace)
	tt.Key(ui.Cmd, ui.KeyZ)
	if a.tabs[0].Text != text {
		t.Fatalf("撤销后=%q, want %q", a.tabs[0].Text, text)
	}
	tt.Key(0, ui.KeyBackspace)
	if got := a.tabs[0].Text; got != "" {
		t.Fatalf("撤销后再次删除原选区=%q, want 空文档", got)
	}
}

func TestEditMenuCopyCutPasteAcrossParagraphs(t *testing.T) {
	const text = "甲\n\n乙\n\n丙"
	a := newTestApp(t)
	t.Cleanup(a.shutdown)
	a.tabs[0].Text = text
	tt := ui.NewTester(a.view, 500, 600)
	chooseEditMenu(t, a, tt, "selectAll")
	chooseEditMenu(t, a, tt, "copy")
	if got := tt.Clipboard(); got != text {
		t.Fatalf("菜单复制全文=%q, want %q", got, text)
	}
	chooseEditMenu(t, a, tt, "cut")
	if got := a.tabs[0].Text; got != "" {
		t.Fatalf("菜单剪切全文=%q, want 空文档", got)
	}
	chooseEditMenu(t, a, tt, "undo")
	tt.SetClipboard("替换\n多行")
	chooseEditMenu(t, a, tt, "paste")
	if got := a.tabs[0].Text; got != "替换\n多行" {
		t.Fatalf("菜单粘贴覆盖恢复选区=%q", got)
	}
	chooseEditMenu(t, a, tt, "undo")
	if got := a.tabs[0].Text; got != text {
		t.Fatalf("撤销粘贴=%q, want %q", got, text)
	}
}

func TestEditMenuSelectAllReplacesSourceDocument(t *testing.T) {
	a := newTestApp(t)
	t.Cleanup(a.shutdown)
	a.cfg.Source = true
	a.tabs[0].Text = "甲\n乙\n丙"
	tt := ui.NewTester(a.view, 500, 600)
	chooseEditMenu(t, a, tt, "selectAll")
	tt.Type("X")
	if got := a.tabs[0].Text; got != "X" {
		t.Fatalf("源码菜单全选后输入=%q, want X", got)
	}
}

func chooseEditorContext(t *testing.T, tt *ui.Tester, label string) {
	t.Helper()
	if err := tt.RightClick("内容"); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem(label); err != nil {
		t.Fatal(err)
	}
}

func TestEditorContextMenuUndoRedo(t *testing.T) {
	const text = "甲行\n\n乙行\n\n丙行"
	for _, source := range []bool{false, true} {
		t.Run(map[bool]string{false: "实时预览", true: "源码"}[source], func(t *testing.T) {
			a := newTestApp(t)
			t.Cleanup(a.shutdown)
			a.cfg.Source = source
			a.tabs[0].Text = text
			tt := ui.NewTester(a.view, 500, 600)
			chooseEditMenu(t, a, tt, "selectAll")
			chooseEditMenu(t, a, tt, "cut")
			for _, step := range []struct{ label, want string }{
				{"撤销", text}, {"重做", ""}, {"撤销", text},
			} {
				chooseEditorContext(t, tt, step.label)
				if got := a.tabs[0].Text; got != step.want {
					t.Fatalf("右键菜单%s后=%q, want %q", step.label, got, step.want)
				}
			}
		})
	}
}

func TestEditorContextMenuCopyCutPaste(t *testing.T) {
	const text = "甲\n\n乙\n\n丙"
	for _, source := range []bool{false, true} {
		t.Run(map[bool]string{false: "实时预览", true: "源码"}[source], func(t *testing.T) {
			a := newTestApp(t)
			t.Cleanup(a.shutdown)
			a.cfg.Source = source
			a.tabs[0].Text = text
			tt := ui.NewTester(a.view, 500, 600)
			if !source {
				a.focusBlockEnd(len(a.mdBlocks()) - 1)
				tt.Frame()
			}
			chooseEditorContext(t, tt, "全选")
			chooseEditorContext(t, tt, "复制")
			if got := tt.Clipboard(); got != text {
				t.Fatalf("右键复制全文=%q, want %q", got, text)
			}
			chooseEditorContext(t, tt, "剪切")
			if got := a.tabs[0].Text; got != "" {
				t.Fatalf("右键剪切全文=%q, want 空", got)
			}
			chooseEditorContext(t, tt, "撤销")
			tt.SetClipboard("替换\n多行")
			chooseEditorContext(t, tt, "粘贴")
			if got := a.tabs[0].Text; got != "替换\n多行" {
				t.Fatalf("右键粘贴替换选区=%q, want 替换\\n多行", got)
			}
			chooseEditorContext(t, tt, "撤销")
			if got := a.tabs[0].Text; got != text {
				t.Fatalf("右键撤销粘贴=%q, want %q", got, text)
			}
			chooseEditorContext(t, tt, "重做")
			if got := a.tabs[0].Text; got != "替换\n多行" {
				t.Fatalf("右键重做粘贴=%q, want 替换\\n多行", got)
			}
		})
	}
}

func TestEditMenuKeepsAuxiliaryInputHistory(t *testing.T) {
	const text = "正文甲\n\n正文乙"
	for _, tc := range []struct {
		name                  string
		source, rename, paste bool
	}{
		{"预览/搜索/输入", false, false, false},
		{"预览/搜索/粘贴", false, false, true},
		{"预览/重命名/输入", false, true, false},
		{"预览/重命名/粘贴", false, true, true},
		{"源码/搜索/输入", true, false, false},
		{"源码/搜索/粘贴", true, false, true},
		{"源码/重命名/输入", true, true, false},
		{"源码/重命名/粘贴", true, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := newTestApp(t)
			t.Cleanup(a.shutdown)
			a.cfg.Source = tc.source
			a.tabs[0].Text = text
			tt := ui.NewTester(a.view, 500, 600)
			if tc.rename {
				a.beginRename(0)
				tt.Frame()
				if err := tt.Click("重命名标签"); err != nil {
					t.Fatal(err)
				}
			} else {
				a.findOpen = true
				tt.Frame()
				done, ok := tt.Find("完成")
				if !ok {
					t.Fatal("找不到搜索条")
				}
				tt.ClickAt(70, done.Y+done.H/2)
			}
			if tc.paste {
				tt.SetClipboard("输入甲乙")
				chooseEditMenu(t, a, tt, "selectAll")
				chooseEditMenu(t, a, tt, "paste")
			} else {
				tt.Type("输入甲乙")
			}
			chooseEditMenu(t, a, tt, "selectAll")
			chooseEditMenu(t, a, tt, "cut")
			value := func() string {
				if tc.rename {
					return a.renameDraft
				}
				return a.query
			}
			if got := value(); got != "" {
				t.Fatalf("输入框剪切后=%q, want 空", got)
			}
			for _, step := range []struct{ command, want string }{
				{"undo", "输入甲乙"}, {"redo", ""}, {"undo", "输入甲乙"},
			} {
				chooseEditMenu(t, a, tt, step.command)
				if got := value(); got != step.want {
					t.Fatalf("输入框%s后=%q, want %q", step.command, got, step.want)
				}
				if got := a.tabs[0].Text; got != text {
					t.Fatalf("输入框编辑改变正文=%q, want %q", got, text)
				}
			}
		})
	}
}

func TestEditMenuSelectAllWithinFormattedTableCell(t *testing.T) {
	const text = "| 甲 | **乙** |\n| --- | --- |\n| 1 | 2 |\n"
	a := newTestApp(t)
	t.Cleanup(a.shutdown)
	a.tabs[0].Text = text
	tt := ui.NewTester(a.view, 500, 600)
	a.focusBlock(0, 0, 0)
	tt.Frame()
	a.focusTableCell(0, 0, 1)
	tt.Frame()
	chooseEditMenu(t, a, tt, "selectAll")
	chooseEditMenu(t, a, tt, "copy")
	if got := tt.Clipboard(); got != "乙" {
		t.Fatalf("表格菜单复制=%q, want 乙", got)
	}
	tt.Type("新")
	const want = "| 甲 | **新** |\n| --- | --- |\n| 1 | 2 |\n"
	if got := a.tabs[0].Text; got != want {
		t.Fatalf("表格菜单全选替换=%q, want %q", got, want)
	}
	chooseEditMenu(t, a, tt, "undo")
	if got := a.tabs[0].Text; got != text {
		t.Fatalf("表格替换撤销=%q, want %q", got, text)
	}
}
