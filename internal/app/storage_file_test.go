package app

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestValidateStorageDir(t *testing.T) {
	a := newTestApp(t)
	t.Cleanup(a.shutdown)
	for _, dir := range []string{filepath.Dir(a.notes.DataPath()), t.TempDir()} {
		got, err := a.validateStorageDir(dir)
		if err != nil {
			t.Fatalf("已有可写目录应被接受: %v", err)
		}
		want, err := canonicalPath(dir)
		if err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Fatalf("目录 = %q, want %q", got, want)
		}
	}
	blocker := filepath.Join(t.TempDir(), "file")
	persistWrite(t, blocker, "不是目录")
	for _, path := range []string{blocker, filepath.Join(blocker, "sub")} {
		if _, err := a.validateStorageDir(path); err == nil {
			t.Fatalf("应拒绝非目录: %q", path)
		}
	}
}

func TestValidateStorageDirCanonicalizesSymlink(t *testing.T) {
	a := newTestApp(t)
	t.Cleanup(a.shutdown)
	dir := t.TempDir()
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(dir, link); err != nil {
		t.Skip("无法创建符号链接:", err)
	}
	got, err := a.validateStorageDir(link)
	if err != nil {
		t.Fatal(err)
	}
	want, err := canonicalPath(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("符号链接应解析为真实目录: %q, want %q", got, want)
	}
}

func TestValidateStorageDirPreservesPreexistingProbe(t *testing.T) {
	for _, symlink := range []bool{false, true} {
		t.Run(map[bool]string{false: "普通文件", true: "符号链接"}[symlink], func(t *testing.T) {
			a := newTestApp(t)
			t.Cleanup(a.shutdown)
			dir := t.TempDir()
			target := filepath.Join(t.TempDir(), "target")
			persistWrite(t, target, "target must survive")
			for _, name := range []string{".zakki-storage-probe", ".zakki-backup-probe"} {
				probe := filepath.Join(dir, name)
				if symlink {
					if err := os.Symlink(target, probe); err != nil {
						t.Skip("无法创建符号链接:", err)
					}
				} else {
					persistWrite(t, probe, "must survive")
				}
			}
			if _, err := a.validateStorageDir(dir); err != nil {
				t.Fatal(err)
			}
			for _, name := range []string{".zakki-storage-probe", ".zakki-backup-probe"} {
				probe := filepath.Join(dir, name)
				if symlink {
					info, err := os.Lstat(probe)
					if err != nil || info.Mode()&os.ModeSymlink == 0 {
						t.Fatalf("预存链接被移除或替换: %v %v", info, err)
					}
				} else if got := string(persistReadRaw(t, probe)); got != "must survive" {
					t.Fatalf("预存探针被改写: %q", got)
				}
			}
			if got := string(persistReadRaw(t, target)); got != "target must survive" {
				t.Fatalf("探针链接目标被改写: %q", got)
			}
			assertOnlyStorageFiles(t, dir, ".zakki-storage-probe", ".zakki-backup-probe")
		})
	}
}

func TestApplyStorageDirMovesSingleFileAndPersistsLocation(t *testing.T) {
	a := newTestApp(t)
	t.Cleanup(a.shutdown)
	if err := a.saveNow(); err != nil {
		t.Fatal(err)
	}
	oldPath := a.notes.DataPath()
	for i := range a.tabs {
		a.tabs[i].Text = fmt.Sprintf("标签 %d", i+1)
	}
	a.tabs[1].Name = "迁移标签"
	a.active = 1
	want := a.snapshotFile()
	dir := t.TempDir()
	a.applyStorageDir(dir)
	canonical, err := canonicalPath(dir)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(canonical, "zakki.json")
	if a.notes.DataPath() != path || a.prefs.Storage.DataFile != path {
		t.Fatalf("迁移路径未一致应用: %q %+v", a.notes.DataPath(), a.prefs.Storage)
	}
	if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
		t.Fatalf("成功后旧业务文件应移除: %v", err)
	}
	got := persistReadSaved(t, path)
	if !reflect.DeepEqual(got.Tabs, want.Tabs) || got.Active != want.Active {
		t.Fatalf("迁移丢失全部标签或活动位置: %+v", got)
	}
	p, err := decodePreferences(persistReadRaw(t, a.preferencesPath()))
	if err != nil {
		t.Fatal(err)
	}
	if p.Storage.DataFile != path {
		t.Fatalf("新位置未同步落盘: %+v", p.Storage)
	}
	assertOnlyStorageFiles(t, dir, "zakki.json")
	assertOnlyStorageFiles(t, filepath.Dir(oldPath), "preferences.json")
	b := newAppAt(filepath.Dir(a.preferencesPath()))
	t.Cleanup(b.shutdown)
	if err := b.loadPreferences(); err != nil {
		t.Fatal(err)
	}
	if err := b.openConfiguredNotes(); err != nil {
		t.Fatal(err)
	}
	if b.notes.DataPath() != path || !reflect.DeepEqual(tabTexts(b), tabTexts(a)) || b.active != a.active {
		t.Fatalf("重启未沿用文件: path=%q tabs=%+v", b.notes.DataPath(), b.tabs)
	}
}

func TestApplyStorageDirSameLocationIsNoOp(t *testing.T) {
	a := newTestApp(t)
	t.Cleanup(a.shutdown)
	if err := a.saveNow(); err != nil {
		t.Fatal(err)
	}
	path, configured := a.notes.DataPath(), a.prefs.Storage.DataFile
	raw := string(persistReadRaw(t, path))
	a.applyStorageDir(filepath.Dir(path))
	if a.notes.DataPath() != path || a.prefs.Storage.DataFile != configured || string(persistReadRaw(t, path)) != raw {
		t.Fatal("同一路径应保持文件与偏好不变")
	}
	if a.storageError != "" {
		t.Fatalf("同一路径不是目标冲突: %s", a.storageError)
	}
	assertOnlyStorageFiles(t, filepath.Dir(path), "zakki.json")
}

func TestApplyStorageDirFailurePreservesOriginal(t *testing.T) {
	for _, reason := range []string{"目标已存在", "路径不可写", "偏好持久化失败"} {
		t.Run(reason, func(t *testing.T) {
			a := newTestApp(t)
			t.Cleanup(a.shutdown)
			a.tabs[0].Text = "原文件必须保留"
			if err := a.saveNow(); err != nil {
				t.Fatal(err)
			}
			oldPath, oldPref := a.notes.DataPath(), a.prefs.Storage.DataFile
			raw := string(persistReadRaw(t, oldPath))
			dir := t.TempDir()
			target := filepath.Join(dir, "zakki.json")
			switch reason {
			case "目标已存在":
				persistWrite(t, target, "用户已有文件")
			case "路径不可写":
				blocker := filepath.Join(dir, "blocker")
				persistWrite(t, blocker, "普通文件")
				dir = filepath.Join(blocker, "sub")
			case "偏好持久化失败":
				original := a.prefsStore.Path
				a.prefsStore.Path = t.TempDir()
				t.Cleanup(func() { a.prefsStore.Path = original })
			}
			a.applyStorageDir(dir)
			if a.notes.DataPath() != oldPath || a.prefs.Storage.DataFile != oldPref {
				t.Fatal("失败不得切换运行时路径或偏好")
			}
			if got := string(persistReadRaw(t, oldPath)); got != raw {
				t.Fatalf("失败不得丢失原件: %q", got)
			}
			if a.storageError == "" {
				t.Fatal("失败必须报告错误")
			}
			if reason == "目标已存在" && string(persistReadRaw(t, target)) != "用户已有文件" {
				t.Fatal("不得覆盖已有目标")
			}
			if reason == "偏好持久化失败" {
				assertOnlyStorageFiles(t, dir)
			}
		})
	}
}

func TestSettingsStoragePageShowsAuthoritativeFile(t *testing.T) {
	a := newTestApp(t)
	t.Cleanup(a.shutdown)
	a.applyStorageDir(t.TempDir())
	a.settingsPage = pageStorage
	a.settingsUI.syncDraftFromPrefs(a.prefs)
	tt := ui.NewTester(a.settingsView, 900, 700)
	tt.Frame()
	for _, want := range []string{"笔记文件", a.notes.DataPath(), "显示文件", "更换位置…"} {
		if !tt.HasText(want) {
			t.Fatalf("保存页缺少 %q: %v", want, tt.Texts())
		}
	}
	for _, gone := range []string{"断开", "备份文件夹", "写回数据文件", "选择文件夹…"} {
		if tt.HasText(gone) {
			t.Fatalf("不应保留废弃入口 %q: %v", gone, tt.Texts())
		}
	}
}
