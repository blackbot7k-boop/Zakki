#!/usr/bin/env python3
"""临时独立 bundle 测量；源码通过 go overlay 加探针，不接触日常数据。"""
import argparse
import json
import os
import pathlib
import subprocess
import tempfile
import time

ROOT = pathlib.Path(__file__).resolve().parents[2]
MAIN = r'''package main
import (
 "fmt"
 "os"
 "path/filepath"
 "sync/atomic"
 "time"
 "github.com/egoist/mygo"
 "zakki/internal/app"
)
func mark(name string) { fmt.Fprintf(os.Stderr,"ZAKKI_LIFECYCLE %s %d\n",name,time.Now().UnixNano()) }
func main() {
 mark("main")
 dir := os.Getenv("ZAKKI_PROBE_DIR")
 mygo.App.SetName("Zakki Lifecycle Probe")
 mygo.App.SetPath(mygo.PathAppData,filepath.Join(dir,"data"))
 mygo.App.SetPath(mygo.PathUserData,filepath.Join(dir,"data","Zakki Dev"))
 mygo.App.SetPath(mygo.PathLogs,filepath.Join(dir,"logs"))
 mygo.App.SetPath(mygo.PathCache,filepath.Join(dir,"cache"))
 var current atomic.Pointer[mygo.Window]
 mygo.App.OnWindowCreated(func(w *mygo.Window) {
  mark("window_created")
  current.Store(w)
  w.OnReadyToShow(func() {
   mark("first_draw")
   w.Update(func() {mark("responsive")})
  })
  w.OnHide(func(){mark("hidden")})
  w.OnClosed(func(){mark("closed")})
 })
 mygo.App.OnBeforeQuit(func(*mygo.QuitEvent){mark("before_quit")})
 go func(){
  ticker := time.NewTicker(2*time.Millisecond)
  defer ticker.Stop()
  for range ticker.C {
   path := filepath.Join(dir,"command")
   raw,err := os.ReadFile(path);if err!=nil || len(raw)==0 {continue};os.Remove(path)
   action := string(raw)
   if action=="quit" {mark("quit_request");mygo.App.Quit();continue}
   if w:=current.Load();w!=nil {
    w.Update(func(){
     mark(action+"_start")
     switch action {
     case "hide":w.Hide()
     case "show":w.Show();w.Focus()
     case "close":w.Close()
     case "edit":app.LifecycleProbeEdit()
     }
     mark(action+"_end")
    })
   }
  }
 }()
 app.Run()
 mark("run_returned")
}
'''
HELPER = r'''
var lifecycleProbeApp *app
func LifecycleProbeEdit() {
 a := lifecycleProbeApp
 a.tabs[0].Text += "\nquit pending sentinel"
 a.resyncLive(0)
 a.scheduleSave()
}
func lifecycleProbeMark(name string) {
 fmt.Fprintf(os.Stderr,"ZAKKI_LIFECYCLE %s %d\n",name,time.Now().UnixNano())
}
func lifecycleProbeStep(name string, fn func() error) error {
 lifecycleProbeMark(name+"_start")
 defer lifecycleProbeMark(name+"_end")
 if name == "mirror_scan" {
  delay, _ := strconv.Atoi(os.Getenv("ZAKKI_PROBE_SCAN_DELAY_MS"))
  if delay > 0 { time.Sleep(time.Duration(delay)*time.Millisecond) }
 }
 return fn()
}
'''


def replace_once(source, old, new):
    assert source.count(old) == 1, (old, source.count(old))
    return source.replace(old, new)


def build_probe(work):
    lifecycle = (ROOT / "internal/app/lifecycle.go").read_text()
    lifecycle = replace_once(lifecycle, '"path/filepath"', '"path/filepath"\n "time"\n "strconv"')
    for label, call in [("notes_load", "a.load"), ("prefs_load", "a.loadPreferences")]:
        lifecycle = replace_once(lifecycle, f"if err := {call}();", f'if err := lifecycleProbeStep("{label}", {call});')
    lifecycle = replace_once(lifecycle, "dir, a, err := startupApp()", 'lifecycleProbeMark("startup_start")\n dir, a, err := startupApp()\n lifecycleProbeMark("startup_end")\n lifecycleProbeApp = a')
    lifecycle = replace_once(lifecycle, "a.ready = true", 'lifecycleProbeMark("native_ready")\n a.ready = true')
    lifecycle = replace_once(lifecycle, "win := mygo.NewWindow(a.windowOptions())", 'lifecycleProbeMark("create_start")\n win := mygo.NewWindow(a.windowOptions())\n lifecycleProbeMark("create_end")')
    lifecycle += HELPER
    state = (ROOT / "internal/app/state.go").read_text()
    state = replace_once(state, "if err := a.notes.Shutdown(a.snapshotFile());", 'if err := lifecycleProbeStep("final_notes", func() error { return a.notes.Shutdown(a.snapshotFile()) });')
    state = replace_once(state, "if err := a.shutdownPreferences();", 'if err := lifecycleProbeStep("final_prefs", a.shutdownPreferences);')
    store = (ROOT / "internal/notes/store.go").read_text()
    store = replace_once(store, '"sort"', '"sort"\n "strconv"')
    store = replace_once(store, "stateWrite:  atomicfile.WriteBytes,", '''stateWrite: func(path string, raw []byte) error {
 delay, _ := strconv.Atoi(os.Getenv("ZAKKI_PROBE_WRITE_DELAY_MS"))
 if delay > 0 {time.Sleep(time.Duration(delay)*time.Millisecond)}
 return atomicfile.WriteBytes(path, raw)
},''')
    jobs = (ROOT / "internal/app/startup_jobs.go").read_text()
    for label, call in [("backup_scan", "store.LoadLastBackupTime"), ("mirror_scan", "store.LoadMirrorTime")]:
        jobs = replace_once(jobs, f"if err := {call}();", f'if err := lifecycleProbeStep("{label}", {call});')
    files = {"main.go": MAIN, "internal/app/lifecycle.go": lifecycle, "internal/app/startup_jobs.go": jobs, "internal/app/state.go": state, "internal/notes/store.go": store}
    overlay = {}
    for i, (target, source) in enumerate(files.items()):
        path = work / f"overlay-{i}.go"
        path.write_text(source)
        overlay[str(ROOT / target)] = str(path)
    (work / "overlay.json").write_text(json.dumps({"Replace": overlay}))
    bundle = work / "Zakki Lifecycle Probe.app"
    executable = bundle / "Contents/MacOS/Zakki Lifecycle Probe"
    executable.parent.mkdir(parents=True)
    (bundle / "Contents/Info.plist").write_text('''<?xml version="1.0" encoding="UTF-8"?>
<plist version="1.0"><dict><key>CFBundleExecutable</key><string>Zakki Lifecycle Probe</string>
<key>CFBundleIdentifier</key><string>com.gomeen.zakki.lifecycleprobe</string>
<key>CFBundleName</key><string>Zakki Lifecycle Probe</string><key>CFBundlePackageType</key><string>APPL</string>
<key>NSHighResolutionCapable</key><true/></dict></plist>''')
    subprocess.run(["go", "build", "-overlay", str(work / "overlay.json"), "-o", str(executable), "."], cwd=ROOT, check=True)
    return executable


def fixture(case):
    if case == "ordinary":
        return ("## 笔记\n\n今天的 **计划** 和待办内容。\n\n- [ ] 测试启动\n\n" * 80, "", False)
    if case == "1m-paragraphs":
        return (("hello **world** 中文笔记\n\n" * 40000)[:1 << 20], "", False)
    if case == "10m-background":
        return ("## 当前笔记\n\n普通短便签", "x" * (10 << 20), False)
    if case == "long-paragraph":
        return ("a" * 100000, "", False)
    if case == "table":
        return ("|a|b|\n|---|---|\n" + "|one|two|\n" * 5000, "", False)
    if case == "source-code":
        return ("```go\n" + "var x = 123 // hello\n" * 8000 + "```", "", True)
    raise ValueError(case)


def run_sample(executable, base, case, gpu, timeout, index, quit_state, settle_ms, pending_edit, write_delay_ms, scan_delay_ms):
    work = base / f"{case}-gpu{gpu}-{index}"
    data = work / "data/Zakki Dev"
    data.mkdir(parents=True)
    for name in ["logs", "cache"]:
        (work / name).mkdir()
    text, background, source = fixture(case)
    tabs = [{"id": 1, "text": text}]
    if background:
        tabs.append({"id": 2, "text": background})
    (data / "zakki.json").write_text(json.dumps({"version": 3, "active": 0, "tabs": tabs, "settings": {"source": source}}, ensure_ascii=False))
    prefs = {"version": 5, "appearance": {"appearanceMode": "system", "colorTheme": "classic", "markdownTheme": "sonnet", "codeTheme": "soft", "language": "zh-Hans"}, "window": {"rememberBounds": True, "hideOnEscape": True}, "backup": {"intervalMinutes": 5, "keep": 20, "onLaunch": True, "gitMode": "off"}, "keymap": {"overrides": {}}, "advanced": {"autosaveDelayMs": 800, "windowWidth": 400, "windowHeight": 540}}
    (data / "preferences.json").write_text(json.dumps(prefs))
    env = os.environ.copy()
    env.update(ZAKKI_PROBE_DIR=str(work), MYGO_GPU=gpu, MYGO_LOCALE="zh-Hans", ZAKKI_PROBE_WRITE_DELAY_MS=str(write_delay_ms), ZAKKI_PROBE_SCAN_DELAY_MS=str(scan_delay_ms))
    if gpu == "default":
        env.pop("MYGO_GPU", None)
    log = work / "process.log"
    events = {}
    record = {"case": case, "gpu": gpu, "sourceState": "after-startup-optimization", "sample": index, "textBytes": len(text.encode()), "backgroundBytes": len(background), "startupTimeoutSeconds": timeout, "quitState": quit_state, "settleMS": settle_ms, "pendingEdit": pending_edit, "writeDelayMS": write_delay_ms, "scanDelayMS": scan_delay_ms}
    with log.open("w") as output:
        started = time.time_ns()
        process = subprocess.Popen([str(executable)], env=env, stdout=output, stderr=output)
        def refresh():
            for line in log.read_text(errors="replace").splitlines():
                if line.startswith("ZAKKI_LIFECYCLE "):
                    _, name, value = line.split()
                    events.setdefault(name, int(value))
        def wait(name, seconds=5):
            deadline = time.monotonic() + seconds
            while time.monotonic() < deadline:
                refresh()
                if name in events:
                    return
                if process.poll() is not None:
                    raise RuntimeError(f"process exited {process.returncode} waiting {name}")
                time.sleep(0.002)
            raise TimeoutError(name)
        def send(action):
            temporary = work / "command.next"
            temporary.write_text(action)
            temporary.replace(work / "command")
        def command(action, expected):
            requested = time.time_ns()
            send(action)
            wait(expected)
            record[action + "RequestToEndMS"] = (events[expected] - requested) / 1e6
        try:
            wait("responsive", timeout)
            if settle_ms:
                time.sleep(settle_ms / 1000)
            if pending_edit:
                command("edit", "edit_end")
                text += "\nquit pending sentinel"
                tabs[0]["text"] = text
            if quit_state == "closed":
                command("hide", "hide_end")
                command("show", "show_end")
                command("close", "close_end")
            elif quit_state == "hidden":
                command("hide", "hide_end")
            command("quit", "quit_request")
            process.wait(timeout=10)
            ended = time.time_ns()
            refresh()
            record["exitCode"] = process.returncode
            record["status"] = "ok" if process.returncode == 0 else "error"
            record["processStartUnixNS"] = started
            record["processExitObservedUnixNS"] = ended
            record["startupFirstDrawMS"] = (events["first_draw"] - started) / 1e6
            record["startupResponsiveMS"] = (events["responsive"] - started) / 1e6
            record["quitToExitObservedMS"] = (ended - events["quit_request"]) / 1e6
            for name in ["startup", "notes_load", "prefs_load", "backup_scan", "mirror_scan", "create", "hide", "show", "close", "final_notes", "final_prefs"]:
                if name + "_start" in events and name + "_end" in events:
                    record[name + "MS"] = (events[name + "_end"] - events[name + "_start"]) / 1e6
            saved = json.loads((data / "zakki.json").read_text())
            record["finalContentPreserved"] = [t["text"] for t in saved["tabs"]] == [t["text"] for t in tabs]
        except (TimeoutError, RuntimeError, subprocess.TimeoutExpired) as error:
            record.update(status="timeout" if isinstance(error, (TimeoutError, subprocess.TimeoutExpired)) else "error", failure=str(error))
            if process.poll() is None:
                sample_path = work / "hang-sample.txt"
                subprocess.run(["sample", str(process.pid), "1", "-file", str(sample_path)], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, timeout=8)
                record["hangSample"] = sample_path.read_text(errors="replace") if sample_path.exists() else "sample unavailable"
                send("quit")
                try:
                    process.wait(timeout=2)
                    record["quitAfterTimeoutExited"] = True
                except subprocess.TimeoutExpired:
                    record["quitAfterTimeoutExited"] = False
        finally:
            if process.poll() is None:
                process.kill()
            process.wait()
            refresh()
    record["eventsUnixNS"] = events
    record["log"] = log.read_text(errors="replace")
    return record


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--cases", nargs="+", default=["ordinary"])
    parser.add_argument("--gpu", choices=["0", "1", "default"], default="0")
    parser.add_argument("--quit-state", choices=["visible", "hidden", "closed"], default="visible")
    parser.add_argument("--settle-ms", type=int, default=0)
    parser.add_argument("--pending-edit", action="store_true")
    parser.add_argument("--scan-delay-ms", type=int, default=0)
    parser.add_argument("--write-delay-ms", type=int, default=0)
    parser.add_argument("--count", type=int, default=5)
    parser.add_argument("--timeout", type=float, default=15)
    parser.add_argument("--output", type=pathlib.Path, required=True)
    args = parser.parse_args()
    args.output.parent.mkdir(parents=True, exist_ok=True)
    with tempfile.TemporaryDirectory(prefix="zakki-lifecycle-perf-") as temporary:
        work = pathlib.Path(temporary)
        executable = build_probe(work)
        for case in args.cases:
            for i in range(args.count):
                record = run_sample(executable, work, case, args.gpu, args.timeout, i + 1, args.quit_state, args.settle_ms, args.pending_edit, args.write_delay_ms, args.scan_delay_ms)
                with args.output.open("a") as output:
                    output.write(json.dumps(record, ensure_ascii=False) + "\n")
                print(case, "gpu=" + args.gpu, i + 1, record["status"], "firstDrawMS=", round(record.get("startupFirstDrawMS", -1), 2), "quitMS=", round(record.get("quitToExitObservedMS", -1), 2), flush=True)
                if record["status"] != "ok":
                    print("Remaining repetitions of this case omitted after failure; see raw record.", flush=True)
                    break


if __name__ == "__main__":
    main()
