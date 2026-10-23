# 2026-10-22 代码审查修复记录

对应本轮仓库审查的 19 个问题。第 3 项按用户确认，完整移除查找替换功能，保留普通查找与跨标签查找。
笔记版本仍为 3，偏好版本仍为 13；本轮不改变文件格式或旧目录迁移策略。

| 编号 | 修复结果 | 主要实现与回归证据 |
|---|---|---|
| 1 | 迁址结果明确返回是否提交；提交后的清理失败仍同步内存偏好与新路径，并报告警告 | `notes/relocate.go`、`app/storage_file.go`、`app/onboarding.go`；迁址清理失败及偏好提交失败测试 |
| 2 | 恢复偏好后立即打开并加载笔记；加载成功才恢复可写，失败继续保护文件 | `app/prefs_runtime.go`、`app/prefs_test.go` |
| 3 | 移除替换栏、单次/全部/跨标签替换、菜单、固定快捷键、状态及专用文案 | `app/find_bar.go`、命令表与菜单契约测试；新增跨标签查找测试 |
| 4 | 随机同目录临时文件、完整写入、Sync、Close、rename 和目录同步；发布后同步失败保留指纹与错误，同内容重试重新同步；迁址候选目录先同步再提交偏好 | `atomicfile/write.go`、`notes/save.go`、`notes/relocate.go`、`prefs/store.go`；故障注入与持续失败重试测试 |
| 5 | 空列表项 Enter/Backspace 使用块索引定位光标，前面有多行块也能正确继续输入 | `app/live_input.go`、`TestEmptyListExitAfterMultilineBlockKeepsCaret` |
| 6 | 菜单与右键粘贴共用输入投影与待输入格式处理，正确消费格式意图 | `app/live_format.go`、`app/menu_edit.go`、`TestPendingFormatPasteFromMenus` |
| 7 | 虚拟行高缓存键纳入实际布局使用的字体、字号、行高、单元格 padding 与排版变体 | `app/live_virtual.go`、`TestVirtualRowsCacheInvalidatesWithGeometryStyle` |
| 8 | 源码高亮复用围栏开闭判断，支持波浪线、长 marker 与其中的短 marker 内容 | `app/view_highlight.go`、`TestEditorRangesFenceSemantics` |
| 9 | 窗口下限随有效排版调整，保留 10 列、4 行正文和查找栏空间；运行期主动扩大过小窗口 | `app/window.go`、`app/lifecycle.go`、`app/window_test.go`；极端排版渲染截图 |
| 10 | 状态栏正文选择满足 4.5:1 的中性色，持久错误使用主题危险色，标签原色保留为小标识 | `app/view.go`、`TestReadableStatusTextMeetsContrast` |
| 11 | 单个命名标签仍显示标题与右键入口 | `app/view.go`、`TestHeaderKeepsNamedSingleTab`；渲染截图 |
| 12 | 当前标签增加不参与槽位布局的中性轮廓，圆点、圆环、标题胶囊均适用 | `app/tab_strip.go`、圆点内部直径与选中轮廓像素测试 |
| 13 | 表格工具和尺寸选择器恢复焦点环；网格命中区至少 28 DIP，限高滚动并随键盘焦点滚入可视区；设置页按钮扩大为 30 DIP | `app/table_toolbar.go`、`app/settings_tabs.go`、小窗口表格工具测试 |
| 14 | 标签右键菜单与设置页增加前移、后移，按稳定 ID 执行动作 | `app/tab_strip.go`、`app/settings_tabs.go`、`TestTabContextMenuMoveUsesStableID` |
| 15 | 向导关窗提供继续设置、使用默认设置、退出应用；未完成设置直接退出不创建笔记 | `app/onboarding.go`、`app/updates.go`；关闭决策、保存门禁、退出无文件测试及 macOS 原生交互验证 |
| 16 | 统一旧目录不自动迁移的说明，区分目录迁移与当前目录文件的格式兼容解码 | `configuration.md`、`architecture.md`、`release.md` |
| 17 | 偏好版本说明更新至 13，补充 padX/padY 覆盖 | `architecture.md` |
| 18 | 报告索引只列当前仍存在的问题；旧报告保留当时背景 | `reports/README.md` |
| 19 | 正式 tag 使用规范版本格式，新版本必须严格高于当前 Latest | `.github/workflows/release.yml`、版本排序用例与 actionlint |

另修复了 300×340 跨标签查找时计数文字覆盖按钮的问题：计数单行截断，悬停显示全文，输入框保留可用宽度。

## 验证

`make check` 的所有检查项均通过。首次整条命令在竞态阶段达到工具的 120 秒等待上限，随后独立执行剩余的 `make race` 与 `make arch-check`，两者退出码均为 0。

- gofmt、Go vet、MyGo vet：通过。
- 全量普通测试与 MyGo vendor UI 测试：通过。
- 全量 race 与 MyGo vendor UI race：通过；应用包 race 测试约 235 秒。
- 架构依赖检查：通过。
- `make build`：成功生成 macOS arm64/amd64 通用应用与 DMG，未生成带更新签名的发行包。
- Windows amd64：全仓库测试二进制交叉编译通过，使用空执行器跳过运行；未做 Windows 实机验证。
- actionlint：通过；实际工作流版本比较片段覆盖升级、相同版本、倒退等 9 个用例。
- 中英文各 489 个消息键，集合一致。
- 渲染截图检查：多标签、圆环、单个命名标签、最小窗口查找与极端字号/留白。
- 隔离 macOS 原生实例：关闭向导出现三选项；继续设置返回向导；退出后没有笔记文件且重启仍显示向导；使用默认设置后创建 7 个标签；实际编辑菜单不包含替换入口。实例与探针代码均已清理，未使用真实用户数据目录。

Windows 路径同步文件内容，但标准库不提供本实现所需的目录句柄同步，目录持久性保障与 Unix 平台不同。文件系统和硬件的断电行为不能由单元测试或交叉编译证明。
