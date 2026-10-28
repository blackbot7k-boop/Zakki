package app

import (
	"errors"
	"testing"

	"zakki/internal/notes"
)

func TestStoppedManualSaveDoesNotBlockQuit(t *testing.T) {
	a := newTestApp(t)
	a.tabs[0].Text = "最终快照"
	if err := a.shutdownForQuit(); err != nil {
		t.Fatal(err)
	}
	a.tabs[0].Text = "退出后迟到的保存"
	if err := a.saveNow(); !errors.Is(err, notes.ErrStopped) {
		t.Fatalf("同步保存 = %v, want ErrStopped", err)
	}
	// 已排队的菜单保存不应把有意停止变成持久错误或阻止退出。
	a.saveCurrent()
	if got := a.saveError(); got != "" {
		t.Fatalf("迟到保存误报持久错误: %q", got)
	}
	if err := a.shutdownForQuit(); err != nil {
		t.Fatalf("重复退出不应被停止状态拦截: %v", err)
	}
	if got := persistReadSaved(t, a.notes.DataPath()).Tabs[0].Text; got != "最终快照" {
		t.Fatalf("退出后的请求覆盖最终快照: %q", got)
	}
}
