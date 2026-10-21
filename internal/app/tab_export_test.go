package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExportFileName(t *testing.T) {
	cases := []struct{ title, fallback, want string }{
		{"周报", "标签 1", "周报.md"},
		{"a/b:c*?", "标签 1", "a-b-c--.md"},
		{"  .草稿.  ", "标签 1", "草稿.md"},
		{"", "标签 3", "标签 3.md"},
		{"...", "标签 2", "标签 2.md"},
		{"x\ty", "标签 1", "x-y.md"},
	}
	for _, c := range cases {
		if got := exportFileName(c.title, c.fallback); got != c.want {
			t.Errorf("exportFileName(%q) = %q, want %q", c.title, got, c.want)
		}
	}
}

func TestWriteExportOverwritesAtomically(t *testing.T) {
	path := filepath.Join(t.TempDir(), "out.md")
	if err := os.WriteFile(path, []byte("旧内容"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writeExport(path, "# 标题\n正文"); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != "# 标题\n正文" {
		t.Fatalf("导出内容 = %q, %v", got, err)
	}
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Fatalf("不应残留临时文件: %v", err)
	}
	if err := writeExport(filepath.Join(path, "sub.md"), "x"); err == nil {
		t.Fatal("写入失败必须返回错误")
	}
}

func TestExportSnapshotCommitsLiveDraft(t *testing.T) {
	a := newTestApp(t)
	t.Cleanup(a.shutdown)
	a.tabs[0].Name = "周报"
	a.tabs[0].Text = "甲"
	a.mdActive, a.mdDraft = 0, "甲乙"

	name, text := a.exportSnapshot(0)
	if name != "周报.md" || text != "甲乙" {
		t.Fatalf("导出快照 = %q %q，应含未提交的草稿", name, text)
	}
	a.tabs[1].Text = "丙"
	if name, text := a.exportSnapshot(1); name != "标签 2.md" || text != "丙" {
		t.Fatalf("未命名标签快照 = %q %q", name, text)
	}
}
