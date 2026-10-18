# 调查报告归档

一次性性能、启动与渲染核查报告，带日期前缀，不作为日常维护的架构文档。
长期事实以 [architecture.md](../architecture.md) 为准。

| 报告 | 说明 |
|---|---|
| [2026-10-18-performance-review.md](2026-10-18-performance-review.md) | 窗口唤起、编辑/预览/撤销优化与剩余瓶颈 |
| [2026-10-18-startup-shutdown-review.md](2026-10-18-startup-shutdown-review.md) | 首帧后后台任务、启动/关窗原生实测 |
| [2026-10-18-focus-rendering-review.md](2026-10-18-focus-rendering-review.md) | 聚焦前后样式差异（已修复，保留根因记录） |

汇总数据见 [performance/summary.json](../performance/summary.json)；复现脚本见 [performance/](../performance/)。

## 仍未解决的性能与 I/O 项（摘自 2026-10-18 性能报告）

- **大块内部布局**：整张表格、超长无空行段落仍是单块，`ui.List` 虚拟化无法限制块内布局成本。
- **同步安全备份**：`backupNow`、清空、写回、删标签前的备份仍在主线程等待完成。
- **目录升级迁移**：`datamigrate` 合并旧备份路径可能一次性读入大量备份正文（仅影响该迁移路径）。
- **外部卷与 Git**：镜像目录扫描、`.git` 探测、Git 任务尚无超时/后台缓存；默认 Metal 冷启动长尾需真机复验。
