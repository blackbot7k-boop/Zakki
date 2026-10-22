package app

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

// TestToastViewport 排队提示在下一帧以居中 toast 展示，随后队列清空。
func TestToastViewport(t *testing.T) {
	a := newTestApp(t)
	tt := ui.NewTester(a.view, 400, 540)
	if tt.HasText("最多 9 个标签，无法新增") {
		t.Fatal("未排队时不应显示 toast")
	}
	a.showToast("最多 9 个标签，无法新增")
	tt.Frame()
	if !tt.HasText("最多 9 个标签，无法新增") {
		t.Fatalf("toast 应显示，texts %q", tt.Texts())
	}
	if len(a.toasts.pending) != 0 {
		t.Fatalf("排队提示应被视图消费，pending = %q", a.toasts.pending)
	}
}

// TestToastQueueDrainsOnce 同一帧重复排队不重复入列：相同文案共用同一
// toast ID，引擎替换既有条目重新计时。
func TestToastQueueDrainsOnce(t *testing.T) {
	a := newTestApp(t)
	a.showToast("已替换 3 处")
	a.showToast("已替换 3 处")
	tt := ui.NewTester(a.view, 400, 540)
	if n := strings.Count(strings.Join(tt.Texts(), "\n"), "已替换 3 处"); n != 1 {
		t.Fatalf("相同文案应只显示一条 toast，texts %q", tt.Texts())
	}
}

// TestShowToastEmpty 空文案不排队。
func TestShowToastEmpty(t *testing.T) {
	a := newTestApp(t)
	a.showToast("")
	if len(a.toasts.pending) != 0 {
		t.Fatalf("空文案不应排队，pending = %q", a.toasts.pending)
	}
}
