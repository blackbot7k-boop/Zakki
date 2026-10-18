# 性能测量原始记录

结论、输入规模、命令、单位与限制见 [性能报告](../performance-review.md)。
这些是合成输入的测量结果，不包含真实笔记。
启动/关窗的完整方法与限制见 [启动与退出评估](../startup-shutdown-review.md)。

| 文件 | 内容 |
|---|---|
| `before/after-highlight.txt` | 1,000–8,000 行代码的源码高亮计算 |
| `before/after-preview.txt` | 400×540 真实视图的预热稳态帧 |
| `before/after-save.txt` | 150ms 慢写争用及 1/10MiB 快照同步保存 |
| `before/after-undo.txt` | 1MiB 文档、250 次独立事务、保留 200 条，GC 后堆增量 |
| `before/after-undo-steady.txt` | 同进程 `-count=5` 补测，显示撤销差分的扫描成本 |
| `summary.json` | 前四组各 5 个样本的中位数及环境信息 |
| `before-save-instrumentation.patch` | 仅用于旧版本测量的写入注入接缝与 benchmark 测试 helper 签名 |

首窗等待与解析优化补充文件：

- `native-startup-scan-before.jsonl`：优化前，镜像状态扫描调用注入 3 秒等待，普通笔记 3 次。
- `native-startup-scan-after.jsonl`：同一注入、优化后 3 次；带待保存编辑，扫描未完成时退出且校验正文。
- `native-startup-local-after.jsonl`：优化后的本地普通笔记/1.20MB 多段落，各 5 次；软件渲染。
- `startup-markdown-before.txt`、`startup-markdown-after.txt`：首次块缓存微基准，各 5 次、每次 10 轮。
  分配量稳定少一次行切片；时间差很小，不单独用于声称首帧改善比例。

`--scan-delay-ms` 在镜像扫描调用内注入等待，用于验证调用是否阻塞首帧，不模拟真实网络文件系统的所有行为。
数据仍来自临时本地目录。记录包含 `scanDelayMS`，阶段事件包含状态扫描开始/结束；优化后快速退出时，
后台扫描可能还未返回，所以结束事件允许缺失。汇总的 `sourceState` 区分旧基线与本轮优化。
以下最初的生命周期样本全部早于本轮启动优化，保留用于对照；后续数字见[当前报告](../startup-shutdown-review.md)。

原生生命周期补充文件：

| 文件 | 内容 |
|---|---|
| `native-lifecycle-probe.py` | 临时独立 bundle + Go overlay；阶段打点、主线程命令、最终正文验证、超时线程采样 |
| `summarize-native-lifecycle.py` / `native-lifecycle-summary.json` | 从所有有效 JSONL 重算分场景中位数、范围、状态和原始数值 |
| `native-lifecycle-visible.jsonl` | 普通笔记、1.20MB 多段落、10MiB 后台标签，可见窗口直接退出，各 5 次 |
| `native-lifecycle-large-blocks.jsonl` | 超长段落、大表格、8,000 行源码，各 5 次 |
| `native-lifecycle-software.jsonl` | 早期隐藏/显示/关窗/退出基线，5 次，不含命令请求端计时 |
| `native-lifecycle-gpu.jsonl` | 显式 GPU 首样本 15 秒超时及完整主线程采样；同场景剩余重复省略 |
| `native-lifecycle-gpu-default.jsonl` | 不设置 MYGO_GPU，默认路径首帧 12.88 秒，1 次 |
| `native-lifecycle-gpu-default-repeat.jsonl` | 默认路径后续三个进程，0.92–1.95 秒 |
| `native-lifecycle-pending.jsonl` | 普通笔记/大表格等待 2 秒，再模拟编辑并在防抖前退出，各 5 次 |
| `native-lifecycle-table-idle.jsonl` | 大表格等待 2 秒后直接退出，5 次 |
| `native-lifecycle-slow-write.jsonl` | 待保存编辑 + 最终写入注入 150ms 等待，5 次 |
| `native-lifecycle-close.jsonl` | 原子命令发布后，隐藏→显示→关窗→退出，普通/表格各 5 次 |
| `native-lifecycle-hidden.jsonl` | 模拟编辑后隐藏再退出，普通笔记 5 次 |
| `native-lifecycle-close-harness-race.jsonl` | **排除**：旧探针非原子命令文件导致两个空命令超时，保留供复查，不算应用故障 |

JSONL 中 `eventsUnixNS` 为内部阶段时间，`log` 含全部探针输出；GPU 超时的 `hangSample`
保存线程采样全文。原探针命令文件曾在截断与写入之间被读取，后改为临时文件 + 原子重命名，
修正后关闭组 10/10 成功。统计脚本显式记录排除原因，不删除失败证据。

```bash
python3 docs/performance/summarize-native-lifecycle.py
```

`1m-paragraphs` 为场景名称，实际 UTF-8 1,200,000 字节；按 JSONL 的 `textBytes` 解释规模。
默认 GPU 测试没有清空系统/驱动缓存，不能据此声称复现了重启后冷启动。

表中的 `before/after-*.txt` 表示分别命名的两个文件，不是目录。
主测量按组串行运行，样本顺序为前→后、后→前、前→后、后→前、前→后；
每次独立进程使用 `-count=1`，原始文件的 `# sample` 记录样本编号。
中途工具超时后从尚未完成的样本继续，最终每个主基准恰有 5 个完整前/后样本；无缺失项。

优化前微基准源码来自 commit `09fa67df21eccab9cd3e2f980bd39898cf619744`。
复现时在独立临时副本中应用 `before-save-instrumentation.patch`，再复制当前版本的以下驱动：

- `internal/app/view_highlight_bench_test.go`
- `internal/app/preview_perf_test.go`
- `internal/app/live_undo_bench_test.go`
- `internal/notes/save_performance_test.go`

该补丁不修改旧保存算法：编码和写盘仍持有旧状态锁。
`stateWrite` 只提供可控慢写入口，默认仍写临时目录中的真实文件。
不要把此补丁应用到优化后的 Store，它只用于重建对照组。

`retained-B/op` 是相对初始文档、强制 GC 后的堆增量；`B/op` 是测量期间累计分配，含义不同。
独立进程与同进程补测的撤销计时差异说明首次堆增长、GC 和缓存状态对耗时有影响；
不能只选择较好的一组声称持续输入更快。原生 Dock/GPU 的端到端耗时也不由这些无窗口基准代表。
