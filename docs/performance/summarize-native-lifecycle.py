#!/usr/bin/env python3
"""汇总原生生命周期原始样本；保留超时，不混合不同场景。"""
import collections
import json
import pathlib
import statistics

ROOT = pathlib.Path(__file__).resolve().parent
EXCLUDED = {"native-lifecycle-close-harness-race.jsonl": "旧探针非原子发布命令，读到空命令；用于诊断探针，不计应用失败或性能样本"}


def summarize():
    groups = []
    paths = [*ROOT.glob("native-lifecycle*.jsonl"), *ROOT.glob("native-startup*.jsonl")]
    for path in sorted(paths):
        if path.name in EXCLUDED:
            continue
        rows = [json.loads(line) for line in path.read_text().splitlines()]
        keys = dict.fromkeys((r["case"], r["gpu"], r.get("quitState", "closed"), r.get("settleMS", 0), r.get("pendingEdit", False), r.get("writeDelayMS", 0), r.get("scanDelayMS", 0)) for r in rows)
        for case, gpu, state, settle, edit, delay, scan_delay in keys:
            samples = [r for r in rows if (r["case"], r["gpu"], r.get("quitState", "closed"), r.get("settleMS", 0), r.get("pendingEdit", False), r.get("writeDelayMS", 0), r.get("scanDelayMS", 0)) == (case, gpu, state, settle, edit, delay, scan_delay)]
            good = [r for r in samples if r["status"] == "ok"]
            metrics = {}
            for key in dict.fromkeys(k for r in good for k in r if k.endswith("MS") and isinstance(r[k], (int, float))):
                if key in {"settleMS", "writeDelayMS", "scanDelayMS"}:
                    continue
                values = [r[key] for r in good if key in r]
                metrics[key] = {"median": statistics.median(values), "min": min(values), "max": max(values), "samples": values}
            groups.append({"file": path.name, "case": case, "gpu": gpu, "quitState": state, "settleMS": settle, "pendingEdit": edit, "writeDelayMS": delay, "scanDelayMS": scan_delay, "sourceState": samples[0].get("sourceState", "after-startup-optimization" if path.name in {"native-startup-scan-after.jsonl", "native-startup-local-after.jsonl"} else "before-startup-optimization"), "statusCounts": dict(collections.Counter(r["status"] for r in samples)), "failures": [{"sample": r["sample"], "failure": r.get("failure")} for r in samples if r["status"] != "ok"], "allSuccessfulSamplesPreserveContent": all(r.get("finalContentPreserved") is True for r in good) if good else None, "metricsMS": metrics})
    return {"units": "milliseconds", "environment": "Intel i9-9880H, darwin/amd64, macOS 26.7.1, MyGo v0.5.1", "exclusions": EXCLUDED, "limitations": ["直接执行独立临时 bundle 的可执行文件，不包含 Finder/LaunchServices 点击开销", "首帧后投递空主线程回调确认事件循环可响应；不是自动化输入完整编辑验证", "每组最多 5 次，失败后省略同场景剩余重复并显式记录；GPU 默认后续组为 3 次", "未清空系统/驱动缓存，不将第一次测量等同于重启后冷启动", "内部 Window.Show 返回时间不代表恢复后的首帧，Dock 全链路另见原生回归", "模拟待保存编辑直接在主线程改模型并走正常 scheduleSave；不代表真实键盘/IME 输入"], "groups": groups}


if __name__ == "__main__":
    (ROOT / "native-lifecycle-summary.json").write_text(json.dumps(summarize(), ensure_ascii=False, indent=2) + "\n")
