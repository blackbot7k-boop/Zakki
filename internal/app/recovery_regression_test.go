package app

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadRestoresValidatedPendingBeforePrimary(t *testing.T) {
	a := newTestApp(t)
	persistWrite(t, a.notes.DataFile, `{"version":99,"tabs":[{"id":1,"text":"future"}]}`)
	if err := a.load(); err == nil {
		t.Fatal("超前版本应报告载入异常")
	}
	a.tabs[0].Text = "写保护期间的新编辑"
	if err := a.saveNow(); err != nil {
		t.Fatal(err)
	}

	b := newTestApp(t)
	b.notes = newNotesStore(a.notes.DataFile, a.notes.BackupDir(), b.now)
	if err := b.load(); err != nil {
		t.Fatalf("有效 pending 应可恢复，得到 %v", err)
	}
	if got := b.tabs[0].Text; got != "写保护期间的新编辑" {
		t.Fatalf("pending 恢复内容=%q", got)
	}
	if !b.notes.SaveBlocked() {
		t.Fatal("恢复 pending 后必须继续写保护")
	}
	if _, err := os.Stat(b.pendingPath()); err != nil {
		t.Fatalf("pending 不应被自动删除: %v", err)
	}
}

func TestFutureVersionBackupRecoveryRemainsWriteProtected(t *testing.T) {
	a := newTestApp(t)
	persistWrite(t, a.notes.DataFile, `{"version":99,"tabs":[{"id":1,"text":"future"}]}`)
	if err := os.MkdirAll(a.notes.BackupDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	persistWrite(t, filepath.Join(a.notes.BackupDir(), "zakki-20240601-120000.json"),
		`{"version":3,"active":0,"tabs":[{"id":1,"text":"backup"}],"settings":{"hideOnEscape":true}}`)
	if err := a.load(); err == nil {
		t.Fatal("超前版本应如实报告错误")
	}
	if got := a.tabs[0].Text; got != "backup" {
		t.Fatalf("应从备份恢复，得到 %q", got)
	}
	if !a.notes.SaveBlocked() {
		t.Fatal("超前版本即使可恢复也必须继续写保护")
	}
	a.tabs[0].Text = "只应写入 pending"
	if err := a.shutdownForQuit(); err != nil {
		t.Fatal(err)
	}
	if got := string(persistReadRaw(t, a.notes.DataFile)); got != `{"version":99,"tabs":[{"id":1,"text":"future"}]}` {
		t.Fatalf("超前版本原件被覆盖: %q", got)
	}
	if got := persistReadSaved(t, a.pendingPath()).Tabs[0].Text; got != "只应写入 pending" {
		t.Fatalf("退出应写 pending，得到 %q", got)
	}
}

func TestConfirmWriteBackRemovePendingFailureKeepsProtection(t *testing.T) {
	a := newTestApp(t)
	a.notes.SetSaveBlocked(true)
	a.tabs[0].Text = "pending 优先"
	if err := a.saveNow(); err != nil {
		t.Fatal(err)
	}
	a.tabs[0].Text = "主文件新内容"

	injected := errors.New("injected remove pending failure")
	oldRemove := removePendingAfterWriteBack
	removePendingAfterWriteBack = func(*app) error {
		a.tabs[0].Text = "清理失败后的编辑"
		return injected
	}
	t.Cleanup(func() { removePendingAfterWriteBack = oldRemove })
	if err := a.confirmWriteBack(); !errors.Is(err, injected) {
		t.Fatalf("清理失败必须返回原错误: %v", err)
	}
	if !a.notes.SaveBlocked() {
		t.Fatal("清理 pending 失败后必须恢复写保护")
	}
	if a.notice == "" || !strings.Contains(a.notes.LastSaveError(), injected.Error()) {
		t.Fatalf("必须保留明确提示与持久化错误: notice=%q saveErr=%q", a.notice, a.notes.LastSaveError())
	}
	mustExist(t, a.pendingPath())
	if got := persistReadSaved(t, a.notes.DataFile).Tabs[0].Text; got != "主文件新内容" {
		t.Fatalf("确认写回的主文件内容错误: %q", got)
	}
	if got := persistReadSaved(t, a.pendingPath()).Tabs[0].Text; got != "主文件新内容" {
		t.Fatalf("清理失败后 pending 必须回滚到同一写回快照: %q", got)
	}

	b := newTestApp(t)
	b.notes = newNotesStore(a.notes.DataFile, a.notes.BackupDir(), b.now)
	if err := b.load(); err != nil {
		t.Fatal(err)
	}
	if got := b.tabs[0].Text; got != "主文件新内容" {
		t.Fatalf("残留 pending 下次启动必须与已写主文件一致，得到 %q", got)
	}
	if !b.notes.SaveBlocked() {
		t.Fatal("从残留 pending 恢复后必须持续写保护")
	}
}
