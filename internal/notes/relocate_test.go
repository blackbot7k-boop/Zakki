package notes

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRelocateCommitAndPendingUsesNewPath(t *testing.T) {
	s := newTestStore(t, nil)
	if err := s.SaveNow(doc("旧")); err != nil {
		t.Fatal(err)
	}
	old := s.DataPath()
	target := filepath.Join(t.TempDir(), "nested", "zakki.json")
	start, release := make(chan struct{}), make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- s.Relocate(target, doc("迁址"), func() error { close(start); <-release; return nil })
	}()
	<-start
	saved := make(chan struct{}, 1)
	s.ScheduleSave(doc("最新"), 0, func() { saved <- struct{}{} })
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	select {
	case <-saved:
	case <-time.After(2 * time.Second):
		t.Fatal("待写快照未保存")
	}
	if got := readFile(t, s.DataPath()).Tabs[0].Text; got != "最新" {
		t.Fatalf("%q", got)
	}
	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Fatal("旧地址不能再写")
	}
	canonical, _ := canonicalPath(target)
	if s.DataPath() != canonical {
		t.Fatalf("路径未切换：%s", s.DataPath())
	}
}

func TestRelocateFailuresLeaveAuthority(t *testing.T) {
	for _, mode := range []string{"existing", "persist", "external", "write"} {
		t.Run(mode, func(t *testing.T) {
			s := newTestStore(t, nil)
			if err := s.SaveNow(doc("原件")); err != nil {
				t.Fatal(err)
			}
			old := s.DataPath()
			target := filepath.Join(t.TempDir(), "zakki.json")
			called := false
			persist := func() error { called = true; return errors.New("prefs failed") }
			switch mode {
			case "existing":
				writeRaw(t, target, "不得覆盖")
			case "external":
				writeRaw(t, old, "外部冲突")
			case "write":
				parent := filepath.Join(t.TempDir(), "file")
				writeRaw(t, parent, "x")
				target = filepath.Join(parent, "zakki.json")
			}
			if err := s.Relocate(target, doc("迁址"), persist); err == nil {
				t.Fatal("应失败")
			}
			if s.DataPath() != old {
				t.Fatal("失败不得切换")
			}
			if mode == "persist" && !called {
				t.Fatal("未执行偏好提交")
			}
			if mode != "persist" && called {
				t.Fatal("校验失败不能写偏好")
			}
			if mode == "existing" && string(readRaw(t, target)) != "不得覆盖" {
				t.Fatal("覆盖已有目标")
			}
			if mode == "persist" {
				if _, err := os.Stat(target); !os.IsNotExist(err) {
					t.Fatal("失败应清理候选")
				}
			}
		})
	}
}

func TestRelocateDeleteFailureReportsCommittedPath(t *testing.T) {
	s := newTestStore(t, nil)
	if err := s.SaveNow(doc("内容")); err != nil {
		t.Fatal(err)
	}
	old := s.DataPath()
	dir, err := filepath.EvalSymlinks(t.TempDir()) // macOS 临时目录经符号链接，Relocate 内部会规范化
	if err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(dir, "zakki.json")
	persisted := false
	s.stateRemove = func(string) error { return errors.New("remove denied") }
	if err := s.Relocate(target, doc("内容"), func() error { persisted = true; return nil }); err == nil {
		t.Fatal("必须报告删除错误")
	}
	if !persisted || s.DataPath() != target || s.LastSaveError() == "" {
		t.Fatal("已提交偏好与路径不一致")
	}
	if _, err := os.Stat(old); err != nil {
		t.Fatal("失败后旧文件还应存在")
	}
	if err := s.SaveNow(doc("新")); err != nil {
		t.Fatal(err)
	}
	if readFile(t, old).Tabs[0].Text != "内容" {
		t.Fatal("切换后不能改旧文件")
	}
}

func TestRelocateCanonicalAndSymlinkSamePath(t *testing.T) {
	s := newTestStore(t, nil)
	if err := s.SaveNow(doc("内容")); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(filepath.Dir(s.DataPath()), link); err != nil {
		t.Skip(err)
	}
	target := filepath.Join(link, "zakki.json")
	if err := s.Relocate(target, doc("不能写"), func() error { t.Error("相同路径不能写偏好"); return nil }); err != nil {
		t.Fatal(err)
	}
	if readFile(t, s.DataPath()).Tabs[0].Text != "内容" {
		t.Fatal("相同地址必须无操作")
	}
	fileLink := filepath.Join(t.TempDir(), "alias.json")
	if err := os.Symlink(s.DataPath(), fileLink); err != nil {
		t.Fatal(err)
	}
	if err := s.Relocate(fileLink, doc("不能写"), func() error { t.Error("同文件不能写偏好"); return nil }); err != nil {
		t.Fatal(err)
	}
}
