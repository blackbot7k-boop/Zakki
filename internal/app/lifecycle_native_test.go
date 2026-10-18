package app

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

// TestNativeDockReopen 用独立 bundle 与临时数据跑真实 macOS 重新打开事件；
// 需要图形会话，显式设置 ZAKKI_NATIVE_TEST=1 后运行，不操作日常实例。
func TestNativeDockReopen(t *testing.T) {
	if runtime.GOOS != "darwin" || os.Getenv("ZAKKI_NATIVE_TEST") != "1" {
		t.Skip("需要 macOS 图形会话与 ZAKKI_NATIVE_TEST=1")
	}
	dir := t.TempDir()
	name := "Zakki Dock Probe " + filepath.Base(filepath.Dir(dir))
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	bundle := filepath.Join(dir, name+".app")
	executable := filepath.Join(bundle, "Contents", "MacOS", "Zakki Dock Probe")
	data := filepath.Join(dir, "data", devDataDirectoryName)
	for _, path := range []string{filepath.Dir(executable), data, filepath.Join(dir, "logs")} {
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	write := func(path, content string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(data, dataFileName), `{"version":3,"active":0,"tabs":[{"id":1,"text":"Dock reopen probe"}],"settings":{}}`)
	write(filepath.Join(bundle, "Contents", "Info.plist"), fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<plist version="1.0"><dict><key>CFBundleExecutable</key><string>Zakki Dock Probe</string><key>CFBundleIdentifier</key><string>com.gomeen.zakki.dockprobe.%s</string><key>CFBundleName</key><string>%s</string><key>CFBundlePackageType</key><string>APPL</string><key>NSHighResolutionCapable</key><true/></dict></plist>`, strings.ToLower(filepath.Base(filepath.Dir(dir))), name))
	main := filepath.Join(dir, "main.go")
	gpu := os.Getenv("MYGO_GPU")
	if gpu == "" {
		gpu = "0" // 生命周期回归隔离 GPU 驱动初始化，原生 GPU 可用 MYGO_GPU=1 单独验证。
	}
	write(main, fmt.Sprintf(nativeDockProbeMain, dir, name, gpu))
	overlay, err := json.Marshal(map[string]any{"Replace": map[string]string{filepath.Join(root, "main.go"): main}})
	if err != nil {
		t.Fatal(err)
	}
	overlayPath := filepath.Join(dir, "overlay.json")
	write(overlayPath, string(overlay))
	build := exec.Command("go", "build", "-overlay", overlayPath, "-o", executable, ".")
	build.Dir = root
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("构建独立实例: %v\n%s", err, out)
	}
	log, err := os.Create(filepath.Join(dir, "process.log"))
	if err != nil {
		t.Fatal(err)
	}
	defer log.Close()
	process := exec.Command(executable)
	process.Stdout, process.Stderr = log, log
	if err := process.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = process.Process.Signal(os.Interrupt)
		done := make(chan error, 1)
		go func() { done <- process.Wait() }()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			_ = process.Process.Kill()
			<-done
		}
	})
	waitFile := func(name string, want string) {
		t.Helper()
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			got, _ := os.ReadFile(filepath.Join(dir, name))
			if strings.TrimSpace(string(got)) == want {
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
		got, _ := os.ReadFile(filepath.Join(dir, name))
		out, _ := os.ReadFile(filepath.Join(dir, "process.log"))
		t.Fatalf("%s = %q，期望 %q\n%s", name, got, want, out)
	}
	command := func(action string) {
		t.Helper()
		_ = os.Remove(filepath.Join(dir, "done"))
		write(filepath.Join(dir, "command"), action)
		waitFile("done", action)
	}
	runAppleScript := func(script string) ([]byte, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return exec.CommandContext(ctx, "osascript", "-e", script).CombinedOutput()
	}
	reopen := func() {
		t.Helper()
		script := fmt.Sprintf("tell application %s to reopen", strconv.Quote(bundle))
		if out, err := runAppleScript(script); err != nil {
			t.Fatalf("发送 Dock 重新打开事件: %v\n%s", err, out)
		}
	}
	waitFile("created", "1")
	waitFile("ready", "1")
	for i := 2; i <= 6; i++ {
		command("close")
		started := time.Now()
		reopen()
		waitFile("ready", strconv.Itoa(i))
		t.Logf("关闭后第 %d 次重新打开: %s（含 AppleScript 进程开销）", i-1, time.Since(started))
	}
	for _, action := range []string{"hide", "hide", "hide", "hide", "hide", "minimize"} {
		command(action)
		reopen()
		command("inspect")
		waitFile("visible", "true")
		waitFile("minimized", "false")
		waitFile("created", "6")
	}
	reopen()
	command("inspect")
	waitFile("created", "6")
}

// 探针通过公开桌面 API 分别触发菜单关闭的 Close 与顶栏隐藏的 Hide 路径；
// 所有 app 可变状态仍只由产品代码在主线程访问。
const nativeDockProbeMain = `package main
import (
 "os"
 "path/filepath"
 "strconv"
 "sync/atomic"
 "time"
 "github.com/egoist/mygo"
 "zakki/internal/app"
)
func main() {
 dir := %q
 write := func(name, value string) { if err := os.WriteFile(filepath.Join(dir,name), []byte(value), 0600); err != nil { panic(err) } }
 mygo.App.SetName(%q)
 os.Setenv("MYGO_GPU",%q)
 mygo.App.SetPath(mygo.PathAppData, filepath.Join(dir,"data"))
 mygo.App.SetPath(mygo.PathUserData, filepath.Join(dir,"data","Zakki Dev"))
 mygo.App.SetPath(mygo.PathLogs, filepath.Join(dir,"logs"))
 mygo.App.SetPath(mygo.PathCache, filepath.Join(dir,"cache"))
 var current atomic.Pointer[mygo.Window]
 created := 0
 mygo.App.OnWindowCreated(func(w *mygo.Window) {
  current.Store(w)
  created++
  serial := created
  write("created",strconv.Itoa(serial))
  w.Update(func() { write("ready",strconv.Itoa(serial)) })
 })
 go func() {
  for range time.Tick(10*time.Millisecond) {
   data, err := os.ReadFile(filepath.Join(dir,"command")); if err != nil { continue }
   os.Remove(filepath.Join(dir,"command"))
   action := string(data)
   if w := current.Load(); w != nil {
    w.Update(func() {
     switch action {
     case "close": w.Close()
     case "hide": w.Hide()
     case "minimize": w.Minimize()
     case "inspect":
      write("visible",strconv.FormatBool(w.IsVisible()))
      write("minimized",strconv.FormatBool(w.IsMinimized()))
     }
     write("done",action)
    })
   }
  }
 }()
 app.Run()
}
`
