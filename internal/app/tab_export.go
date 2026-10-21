package app

import (
	"log/slog"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo"

	"zakki/internal/atomicfile"
	"zakki/locales"
)

// exportFileName 导出对话框的建议文件名：去掉文件系统保留字符与首尾空白和点，
// 空名回退到「标签 N」由调用方传入的 fallback。
func exportFileName(title, fallback string) string {
	clean := strings.Map(func(r rune) rune {
		switch {
		case r < 0x20, strings.ContainsRune(`/\:*?"<>|`, r):
			return '-'
		}
		return r
	}, title)
	clean = strings.Trim(clean, " .")
	if clean == "" {
		clean = fallback
	}
	return clean + ".md"
}

// exportSnapshot 主线程：取第 i 个标签的导出快照。实时预览中正在编辑的块
// 先并回文档，导出内容与屏幕一致。
func (a *app) exportSnapshot(i int) (name, text string) {
	if i == a.active && !a.cfg.Source && a.mdActive >= 0 {
		a.commitDraft(-1)
	}
	return exportFileName(a.displayTitle(i), a.menuTitle(i)), a.tabs[i].Text
}

// writeExport 把快照原样写到用户选择的路径（tmp+rename，不留半截文件）。
func writeExport(path, text string) error {
	return atomicfile.WriteBytes(path, []byte(text))
}

// exportTab 「导出为 Markdown…」：主线程取快照，后台弹保存对话框并写文件；
// 写入失败弹原生错误框，成功在底栏提示。
func (a *app) exportTab(i int) {
	if i < 0 || i >= len(a.tabs) || !a.ready {
		return
	}
	name, text := a.exportSnapshot(i)
	parent := a.win
	go func() {
		path, err := mygo.Dialog.Save(mygo.SaveDialogOptions{
			Parent:            parent,
			Title:             locales.DialogExportTitle(),
			DefaultPath:       name,
			CreateDirectories: true,
			Filters:           []mygo.FileFilter{{Name: "Markdown", Extensions: []string{"md"}}},
		})
		if err != nil {
			slog.Error("导出对话框失败", "error", err)
			return
		}
		if path == "" {
			return
		}
		if err := writeExport(path, text); err != nil {
			slog.Error("导出 Markdown 失败", "path", path, "error", err)
			mygo.Dialog.Error(locales.DialogExportFailedTitle(), err.Error())
			return
		}
		a.runOnMain(func() { a.notice = locales.NoticeExported(filepath.Base(path)) })
	}()
}
