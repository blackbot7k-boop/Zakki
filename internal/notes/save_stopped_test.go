package notes

import (
	"errors"
	"testing"
)

func TestSaveNowStoppedBeforeAndAfterWriteLock(t *testing.T) {
	for _, queued := range []bool{false, true} {
		t.Run(map[bool]string{false: "已停止", true: "排队中停止"}[queued], func(t *testing.T) {
			s := newTestStore(t, nil)
			if err := s.SaveNow(doc("原件")); err != nil {
				t.Fatal(err)
			}
			var err error
			if queued {
				s.writeMu.Lock()
				done := make(chan error, 1)
				go func() { done <- s.SaveNow(doc("未保存")) }()
				waitSaveState(t, s, func() bool { return s.saveRevision == 2 })
				s.SetStopped(true)
				s.writeMu.Unlock()
				err = <-done
			} else {
				s.SetStopped(true)
				err = s.SaveNow(doc("未保存"))
			}
			if !errors.Is(err, ErrStopped) {
				t.Fatalf("SaveNow = %v, want ErrStopped", err)
			}
			if got := readFile(t, s.DataPath()).Tabs[0].Text; got != "原件" {
				t.Fatalf("停止后写盘: %q", got)
			}
			s.SetStopped(false)
			if err := s.SaveNow(doc("恢复后保存")); err != nil {
				t.Fatal(err)
			}
			if got := readFile(t, s.DataPath()).Tabs[0].Text; got != "恢复后保存" {
				t.Fatalf("恢复保存失败: %q", got)
			}
		})
	}
}
