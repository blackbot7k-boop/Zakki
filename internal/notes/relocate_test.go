package notes

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"zakki/internal/atomicfile"
)

func TestRelocateCommitAndPendingUsesNewPath(t *testing.T) {
	s := newTestStore(t, nil)
	if err := s.SaveNow(doc("旧")); err != nil {
		t.Fatal(err)
	}
	old := s.DataPath()
	target := filepath.Join(t.TempDir(), "nested", "zakki.json")
	start, release := make(chan struct{}), make(chan struct{})
	type relocateDone struct {
		result RelocateResult
		err    error
	}
	done := make(chan relocateDone, 1)
	go func() {
		result, err := s.Relocate(target, doc("迁址"), func() error { close(start); <-release; return nil })
		done <- relocateDone{result: result, err: err}
	}()
	<-start
	saved := make(chan struct{}, 1)
	s.ScheduleSave(doc("最新"), 0, func() { saved <- struct{}{} })
	close(release)
	completed := <-done
	if completed.err != nil {
		t.Fatal(completed.err)
	}
	if !completed.result.Committed {
		t.Fatal("迁址成功必须报告已提交")
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
			result, err := s.Relocate(target, doc("迁址"), persist)
			if err == nil {
				t.Fatal("应失败")
			}
			if result.Committed {
				t.Fatal("提交前失败不得报告已切换")
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
	result, err := s.Relocate(target, doc("内容"), func() error { persisted = true; return nil })
	if err == nil {
		t.Fatal("必须报告删除错误")
	}
	if !result.Committed {
		t.Fatal("删除旧文件失败时迁址已经提交")
	}
	var cleanupErr *RelocateCleanupError
	if !errors.As(err, &cleanupErr) {
		t.Fatalf("提交后的错误类型 = %T, want *RelocateCleanupError", err)
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

func TestRelocateCommittedPreferenceErrorKeepsBothFiles(t *testing.T) {
	s := newTestStore(t, nil)
	if err := s.SaveNow(doc("内容")); err != nil {
		t.Fatal(err)
	}
	old := s.DataPath()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(dir, "zakki.json")
	wantErr := errors.New("directory sync failed")

	result, err := s.Relocate(target, doc("内容"), func() error {
		return &atomicfile.CommitError{Err: wantErr}
	})

	if !result.Committed || !atomicfile.IsCommitted(err) || !errors.Is(err, wantErr) {
		t.Fatalf("迁址结果 = %+v, err=%v, want committed durability error", result, err)
	}
	if s.DataPath() != target {
		t.Fatalf("权威路径 = %q, want %q", s.DataPath(), target)
	}
	if _, err := os.Stat(old); err != nil {
		t.Fatalf("偏好持久性未确认时必须保留旧文件: %v", err)
	}
	if got := readFile(t, target).Tabs[0].Text; got != "内容" {
		t.Fatalf("新文件内容 = %q", got)
	}
	if s.LastSaveError() == "" {
		t.Fatal("目录同步失败必须如实上报")
	}
}

func TestWriteExclusiveLinkFallback(t *testing.T) {
	for _, mode := range []string{"unsupported", "exists", "fallback-race"} {
		t.Run(mode, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "zakki.json")
			link := func(_, target string) error {
				if mode == "exists" || mode == "fallback-race" {
					writeRaw(t, target, "别人的内容")
					if mode == "exists" {
						return os.ErrExist
					}
				}
				return syscall.EOPNOTSUPP
			}
			err := writeExclusiveWithLink(path, []byte("迁址内容"), link)
			if mode != "unsupported" {
				if !errors.Is(err, os.ErrExist) || string(readRaw(t, path)) != "别人的内容" {
					t.Fatalf("抢先创建不得覆盖：%v", err)
				}
			} else if err != nil || string(readRaw(t, path)) != "迁址内容" {
				t.Fatalf("不支持硬链接时应排他写入：%v", err)
			}
			entries, err := os.ReadDir(dir)
			if err != nil || len(entries) != 1 || entries[0].Name() != "zakki.json" {
				t.Fatalf("临时文件未清理：%v %v", entries, err)
			}
		})
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
	result, err := s.Relocate(target, doc("不能写"), func() error { t.Error("相同路径不能写偏好"); return nil })
	if err != nil {
		t.Fatal(err)
	}
	if result.Committed {
		t.Fatal("相同路径不应报告新提交")
	}
	if readFile(t, s.DataPath()).Tabs[0].Text != "内容" {
		t.Fatal("相同地址必须无操作")
	}
	fileLink := filepath.Join(t.TempDir(), "alias.json")
	if err := os.Symlink(s.DataPath(), fileLink); err != nil {
		t.Fatal(err)
	}
	result, err = s.Relocate(fileLink, doc("不能写"), func() error { t.Error("同文件不能写偏好"); return nil })
	if err != nil {
		t.Fatal(err)
	}
	if result.Committed {
		t.Fatal("同一文件不应报告新提交")
	}
}
