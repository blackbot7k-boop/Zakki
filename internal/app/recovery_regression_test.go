package app

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadInvalidFilePreservesOriginalAndRefusesSave(t *testing.T) {
	for _, raw := range []string{"{broken", "", fmt.Sprintf(`{"version":%d,"tabs":[{"id":1,"text":"future"}]}`, currentDataVersion+1)} {
		t.Run(fmt.Sprintf("原件=%q", raw), func(t *testing.T) {
			a := newTestApp(t)
			t.Cleanup(a.shutdown)
			path := a.notes.DataPath()
			persistWrite(t, path, raw)
			if err := a.load(); err == nil {
				t.Fatal("损坏、空文件和超前版本均应返回错误")
			}
			if !a.notes.SaveBlocked() {
				t.Fatal("载入失败必须写保护")
			}
			if a.firstRun() {
				t.Fatal("异常数据不是首启")
			}
			a.tabs[0].Text = "不允许覆盖原件的新编辑"
			if err := a.saveNow(); err == nil {
				t.Fatal("写保护期间保存必须失败")
			}
			if err := a.shutdownForQuit(); err == nil {
				t.Fatal("写保护期间应拒绝退出")
			}
			if got := string(persistReadRaw(t, path)); got != raw {
				t.Fatalf("原件被改写: %q", got)
			}
			assertOnlyStorageFiles(t, filepath.Dir(path), "zakki.json")
		})
	}
}

func TestLoadDoesNotRecoverLegacyBackupOrPending(t *testing.T) {
	a := newTestApp(t)
	t.Cleanup(a.shutdown)
	path := a.notes.DataPath()
	const raw = `{"version":99,"tabs":[{"id":1,"text":"future"}]}`
	persistWrite(t, path, raw)
	legacyDir := filepath.Join(filepath.Dir(path), "backups")
	if err := os.MkdirAll(legacyDir, 0o755); err != nil {
		t.Fatal(err)
	}
	const old = `{"version":3,"active":0,"tabs":[{"id":1,"text":"不应恢复的旧副本"}]}`
	backup := filepath.Join(legacyDir, "zakki-20240601-120000.json")
	pending := path + ".pending"
	persistWrite(t, backup, old)
	persistWrite(t, pending, old)
	if err := a.load(); err == nil {
		t.Fatal("超前版本应报告错误而非恢复旧副本")
	}
	if !a.notes.SaveBlocked() {
		t.Fatal("应维持写保护")
	}
	for _, tb := range a.tabs {
		if tb.Text == "不应恢复的旧副本" {
			t.Fatal("禁止从旧备份或 pending 恢复")
		}
	}
	if err := a.saveNow(); err == nil {
		t.Fatal("应拒绝保存")
	}
	if got := string(persistReadRaw(t, path)); got != raw {
		t.Fatalf("原件被覆盖: %q", got)
	}
	for _, p := range []string{backup, pending} {
		if got := string(persistReadRaw(t, p)); got != old {
			t.Fatalf("旧副本不得改写或迁移: %s = %q", p, got)
		}
	}
	assertOnlyStorageFiles(t, filepath.Dir(path), "zakki.json", "zakki.json.pending", "backups")
	assertOnlyStorageFiles(t, legacyDir, "zakki-20240601-120000.json")
}

func TestReloadNotesKeepsProtectionUntilValidFile(t *testing.T) {
	a := newTestApp(t)
	t.Cleanup(a.shutdown)
	path := a.notes.DataPath()
	persistWrite(t, path, "broken")
	if err := a.load(); err == nil {
		t.Fatal("损坏文件应报错")
	}
	a.reloadNotes()
	if !a.notes.SaveBlocked() {
		t.Fatal("重读失败不得解除写保护")
	}
	if err := a.saveNow(); err == nil {
		t.Fatal("失败重读后仍应拒写")
	}
	if got := string(persistReadRaw(t, path)); got != "broken" {
		t.Fatalf("原件被覆盖: %q", got)
	}
	persistWrite(t, path, `{"version":3,"active":0,"tabs":[{"id":7,"name":"修复","text":"磁盘修复内容"}]}`)
	a.reloadNotes()
	if a.notes.SaveBlocked() {
		t.Fatal("显式重读有效文件后应解除写保护")
	}
	if len(a.tabs) != 1 || a.tabs[0].ID != 7 || a.tabs[0].Name != "修复" || a.tabs[0].Text != "磁盘修复内容" {
		t.Fatalf("应使用修复后的磁盘内容: %+v", a.tabs)
	}
	a.tabs[0].Text = "修复后继续编辑"
	if err := a.saveNow(); err != nil {
		t.Fatal(err)
	}
	if got := persistReadSaved(t, path).Tabs[0].Text; got != "修复后继续编辑" {
		t.Fatalf("未恢复正常保存: %q", got)
	}
	assertOnlyStorageFiles(t, filepath.Dir(path), "zakki.json")
}

func TestSaveRejectsExternalDiskChangeAndReloadsExplicitly(t *testing.T) {
	for _, raw := range []string{`{"version":3,"tabs":[{"id":1,"text":"外部编辑"}]}`, ""} {
		t.Run(fmt.Sprintf("磁盘=%q", raw), func(t *testing.T) {
			a := newTestApp(t)
			t.Cleanup(a.shutdown)
			a.tabs[0].Text = "原内容"
			if err := a.saveNow(); err != nil {
				t.Fatal(err)
			}
			path := a.notes.DataPath()
			persistWrite(t, path, raw)
			a.tabs[0].Text = "内存改动"
			if err := a.saveNow(); err == nil {
				t.Fatal("外部修改后应拒绝覆盖")
			}
			if err := a.shutdownForQuit(); err == nil {
				t.Fatal("冲突时应拒绝退出")
			}
			if got := string(persistReadRaw(t, path)); got != raw {
				t.Fatalf("外部内容被覆盖: %q", got)
			}
			a.reloadNotes()
			if raw == "" {
				if !a.notes.SaveBlocked() {
					t.Fatal("空文件重读失败应保持保护")
				}
			} else if a.notes.SaveBlocked() || a.tabs[0].Text != "外部编辑" {
				t.Fatalf("有效外部内容应由显式重读接受: %+v", a.tabs)
			}
			assertOnlyStorageFiles(t, filepath.Dir(path), "zakki.json")
		})
	}
}

func TestMissingCustomDataFileIsNotFirstRun(t *testing.T) {
	a := newTestApp(t)
	t.Cleanup(a.shutdown)
	appDir := filepath.Dir(a.preferencesPath())
	a.applyStorageDir(t.TempDir())
	path := a.notes.DataPath()
	if a.prefs.Storage.DataFile != path {
		t.Fatal("应先持久化所选路径")
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	b := newAppAt(appDir)
	t.Cleanup(b.shutdown)
	if err := b.loadPreferences(); err != nil {
		t.Fatal(err)
	}
	if err := b.openConfiguredNotes(); err == nil {
		t.Fatal("重启时自定义数据文件丢失必须报错")
	}
	if b.notes.DataPath() != path || !b.notes.SaveBlocked() || b.firstRun() {
		t.Fatalf("不得退回默认首启: path=%q blocked=%v firstRun=%v", b.notes.DataPath(), b.notes.SaveBlocked(), b.firstRun())
	}
	if err := b.saveNow(); err == nil {
		t.Fatal("自定义文件丢失时不得创建空文件")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("丢失文件不应自动重建: %v", err)
	}
	if _, err := os.Stat(filepath.Join(appDir, "zakki.json")); !os.IsNotExist(err) {
		t.Fatalf("不得写入默认位置: %v", err)
	}
}
