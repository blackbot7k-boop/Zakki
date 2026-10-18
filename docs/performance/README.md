# 性能测量工具与汇总

结论与场景说明见归档报告 [reports/2026-10-18-performance-review.md](../reports/2026-10-18-performance-review.md)
与 [reports/2026-10-18-startup-shutdown-review.md](../reports/2026-10-18-startup-shutdown-review.md)。

| 文件 | 用途 |
|---|---|
| `summary.json` | 源码高亮、预览帧、保存争用、撤销堆等微基准的中位数与环境信息 |
| `native-lifecycle-probe.py` | 临时 bundle 原生生命周期探针（阶段打点、正文校验） |
| `summarize-native-lifecycle.py` | 从 JSONL 重算分场景中位数（输入文件需本地保留或重新采集） |

仓库不再提交大批量 JSONL/txt 原始样本；需要完整原生生命周期数据时，用探针重新跑并本地保存 JSONL。

微基准复现：对应当前代码运行 `internal/app/*_bench_test.go` 与 `internal/notes/save_performance_test.go`，
或对照 `summary.json` 中的 commit 与环境字段。

```bash
python3 docs/performance/summarize-native-lifecycle.py   # 需自备 *.jsonl
```
