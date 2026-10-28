# Zakki 移动客户端规格（iOS 先行 + Android 对齐）

> 状态：**草案，待用户确认**。本文是 wayfinder 地图
> [#1「手机提醒与 iOS 客户端路线」](https://github.com/gomeen/Zakki/issues/1) 的目的地产物，
> 汇总 #3–#12 全部研究与定案。确认后按第 7 节里程碑开实施票。
> 术语见 [GLOSSARY.md](../GLOSSARY.md)；时间令牌语法见 [ADR-0001](adr/0001-inline-time-tokens.md)、
> 重复语义见 [ADR-0002](adr/0002-recurring-time-tokens.md)；视觉令牌见
> [design-tokens-mobile-draft.md](design-tokens-mobile-draft.md)。

## 0. 目标与范围

在 iPhone（先行）与 Android（对齐跟进）上提供 Zakki 的**重度编辑 + 可靠提醒**体验：
浏览、编辑、标签、提醒（含重复）、同步，与桌面端共享同一份 `zakki.json` 与同一套令牌语义。

- 桌面端保持现状（提醒 v1 + 本规格的同步配套改动），桌面重复调度与 OS 级调度**不在本规格**。
- 本规格不含：跨标签搜索、通知深链、应用内今日聚合视图（fast-follow）、iPad 布局、手表、
  Siri/快捷指令、厂商推送通道（另议）。

## 1. 技术栈与工程结构

- **iOS**：SwiftUI 壳 + UIKit 文本栈（`UITextView` + `NSTextAttachment` 行内组件）。
  交互模型已由原型验证（spike v3.2，用户验收）。
- **Android**：Kotlin + Jetpack Compose，**效果对齐 iOS**（一致性由设计令牌 + 对齐验收保证）。
- **共享内核**：`zakki/core`（桌面仓库嵌套 module，纯 stdlib 零依赖），
  经 `core/bind`（gomobile JSON 边界）生成 XCFramework / AAR 供双端消费。
  双端 bind 已实证：iOS XCFramework（真机+模拟器 slice）、Android AAR（4 ABI）。
- **仓库**：新建 `zakki-mobile` 仓库（iOS/Android 工程 + 绑定产物构建脚本）；
  `zakki/core` 留在桌面仓库（单一事实源），移动端以 replace/固定提交引用。
- **CI**：zakki-mobile 仓库自带构建检查；`zakki/core` 的测试随桌面 `make check` 运行。
- **签名与分发**：iOS 免费个人签名（7 天重签，日常侧载流程见实施验证清单）；
  Android APK 侧载（官方一等，无重签死结）；不上架、不进 TestFlight（v1）。

## 2. 数据模型与同步

### 2.1 数据

- 唯一数据文件 `zakki.json`（version 3）：`{version, active, tabs:[{id,name,text,color}], settings}`；
  标签正文为 Markdown，时间语义以行内令牌承载（`@时刻`、`~提前量`、`^重复`，见 ADR-0001/0002）。
- **core 增补 `model` 包**（M1）：从 `internal/notes/file.go` 提取 File/Tab/Settings 类型与
  版本迁移语义，桌面 `internal/notes` 委托之；移动端经 gomobile 消费同一实现。
  迁移时同 commit 更新 architecture.md / engineering-standard.md。

### 2.2 同步协议（定案：可插拔双后端，任一时刻一个激活）

| 后端 | 覆盖 | 并发控制 | 认证 |
|---|---|---|---|
| A. GitHub Contents API | 全平台通用兜底 | `sha` 显式 CAS（官方文档语义） | **OAuth Device Flow**（`github.com/login/device`，无需回调服务器） |
| B. 云盘文件夹 | macOS=iCloud Drive / Windows=OneDrive | 系统冲突副本（OneDrive conflict copy / iCloud NSFileVersion）+ 打开对账 | 系统文件提供方，零凭证 |

- **冲突策略（定案）**：LWW + 输掉版本留档。后端 A 由 app 留档（「来自 iPhone 的副本」类标签）；
  后端 B 依赖系统冲突副本 + 打开时对账提示。
- **同步时机**：前台/聚焦时拉推（iOS 后台限制为物理事实，不做后台承诺）；
  每次同步完成即触发提醒重排（见 3.2 主循环）。
- **备份关系（定案）**：同步开启时接管桌面 `git.autoCommit/autoPush`；关闭时原备份功能保留。
- 桌面配套（本规格覆盖的实现项）：prefs 版本 14 新增 `sync` 节（后端/开关/账号/状态）；
  `git.autoPush` 改造进同步循环；云盘后端需区分「云端触发的外部变更」（无未保存本地修改时
  自动跟随）与「人为外部改动」（维持写保护拒绝）。
- 不做：坚果云 WebDAV（GitHub 可达性实测通过 + OneDrive 覆盖 Windows 云盘诉求）、
  坚果云 ETag 实测、iCloud for Windows 实测（未选该路线）、CloudKit/Syncthing/自建服务/Dropbox。

## 3. 提醒机制

### 3.1 设置交互

与桌面同构：时间令牌弹层（`@时刻` + `~提前量` + `^重复`，档位白名单共用 core `AlertTiers`）；
预览态令牌渲染为行内组件；所有解析/写回经 core，双端与桌面零语义漂移。

### 3.2 调度主循环（双端同构）

**打开/回到前台 → 拉取同步 → 全量重算提醒实例 → 对账重排通知。**

- 实例展开在 core（`Expand`）：桌面地平线 60 天；iOS 在 64 条 pending 配额内填未来实例
  （可整除自然单位的规则可用原生 repeating trigger 省配额）；Android 注册下一个精确闹钟，
  触发后由 receiver 重注册。
- 通道提权：iOS 用 Time Sensitive 级别；Android 用 `setAlarmClock()`（直通 Doze）+
  `SCHEDULE_EXACT_ALARM` 权限引导 + `BOOT_COMPLETED` 重建设（重启后重建闹钟）。
- 漏报应对：iOS 侧漏报场景清单见
  [可靠性研究](reports/2026-10-27-ios-local-notification-reliability.md)
  （64 条上限、重签清零、BGTask 预算衰减）；Android 侧见
  [Android 报告](reports/2026-10-27-android-notification-reliability.md)。
  免费约束下的可靠性分级已被用户接受：iOS 端自建提醒较可靠、桌面端改动「尽力而为」。
- 完成推进（重复任务勾选）：编辑动作改写锚点到当前实例的下一个时刻（从到期日推进、
  补勾不追账）；跨设备完成状态一致性由同步 LWW + 留档覆盖，无额外机制。

### 3.3 通知呈现

纯系统通知（无应用内聚合页）；点按通知仅打开 app（深链 fast-follow）。
通知正文 = 令牌所在条目的可读正文（与桌面 v1 同规则）。

## 4. UI 与视觉规格

### 4.1 信息架构

- **一级**：标签列表，彩色胶囊（对齐 PC 每 tab 颜色；名字在色条内，深色文字对比度收敛）。
- **二级**：编辑器，**单面实时渲染**（所见即所得）：
  - 缓冲中令牌簇 = 单个对象字符（U+FFFC），随时渲染为行内 pill 组件；
  - 键入完整令牌自动收成组件；退格删组件 = 删除整个令牌；点组件 = 完成并推进；
  - 右上毛玻璃 FAB 切换 实时渲染 ⇄ 源码（等宽字体）；
  - 底部 MD 快捷栏（键盘附件）：`H1 H2 • ☐ ❝ B `` @✦`，B/反引号支持选区包裹，
    `@✦` 插入明日令牌；
  - 支持横屏三向。

### 4.2 视觉令牌与主题

- 基线见 [design-tokens-mobile-draft.md](design-tokens-mobile-draft.md)（颜色/组件/排版/间距）。
- **验收总则（用户定案）**：移动端样式与 PC 端保持大致一致；间距/排版以桌面排版主题为准；
  原型占位值不作为验收依据。
- **主题提取方案（定案：分阶段）**：
  - M2（最小主题集）：明暗跟随 + 标准风格核心样式（正文/标题/任务/引用/代码/组件）+
    标签色板，数值从桌面 `view_theme.go` 提取为 core 共享数据（JSON）；
  - 后续里程碑：完整双层主题（经典/手账排版、代码配色方案）提取与对齐；
    手账装饰与 LXGW 字体的移动分发另议。

## 5. MVP 清单

**MVP = 基线 + 两项点选**：

1. 基线：浏览、重度编辑（所见即所得 + 源码双模式）、标签切换、提醒（设置/通知/对账重排）、
   同步（可插拔双后端）。
2. 点选进 MVP：**重复提醒**（ADR-0002 全量，实例展开进 core）；**小组件第一批**
   （今日到期提醒；iOS WidgetKit / Android AppWidget，形态随 M5 细化）。

**明确不做（v1）**：跨标签搜索、通知深链、应用内今日聚合视图（三者 fast-follow）；
iPad 专属布局、手表、Siri/快捷指令、厂商推送通道、桌面 OS 级调度、上架/App Store 分发。

## 6. 桌面端配套改动（实施项，随 M4）

1. prefs 版本 14：新增 `sync` 节（后端选择/开关/账号/最近状态）。
2. `git.autoPush` 并入同步循环；关闭同步时原 Git 自动备份原样保留。
3. 云盘后端的「云端触发外部变更」自动跟随策略（无未保存本地修改时）。
4. 冲突副本在桌面的呈现入口（「来自 iPhone 的副本」提示与合并引导）。

## 7. 里程碑

| 里程碑 | 内容 | 出口标准 |
|---|---|---|
| M1 脚手架 | zakki-mobile 仓库 + iOS 工程可跑 + core/bind XCFramework 集成 + core `model` 包提取 + CI | 模拟器显示真实 zakki.json 的标签列表 |
| M2 编辑器 | 双模式编辑器 + 标签切换 + 最小主题集（明暗） | 长中文文档重度编辑手感达标（用户验收） |
| M3 提醒 | 令牌弹层 + 本地通知调度主循环 + 完成推进 + 重复提醒 + Time Sensitive/setAlarmClock | 双端到点响、重启/重签后恢复、对账重排正确 |
| M4 同步 | GitHub Contents API 后端 + LWW 留档 + 桌面配套（prefs v14/同步循环） | 桌面 ↔ iPhone 双向同步 + 冲突留档可复现 |
| M5 收尾 | 云盘后端（iCloud/OneDrive）+ 小组件 + 分发签名日常化 + 实施验证清单清零 | 全部验收项通过 |

每个里程碑 = 一张实施票（可独立会话执行）；里程碑内发现语义缺口回 core 修（单一事实源）。

## 8. 实施验证清单（随对应里程碑执行）

- [ ] GitHub API 蜂窝网真机复核（M4）
- [ ] OneDrive 冲突副本命名与触发条件实测（M5）
- [ ] iCloud NSFileVersion 恢复 UI 与测试（M5）
- [ ] iOS 免费签名 7 天重签流程演练 + 重签后 pending 通知与授权清零的实际表现（M3）
- [ ] Android 12+ 精确闹钟权限引导路径实测（M3）
- [ ] gomobile 产物对 Go 版本升级的兼容性回归（随 M1 CI）

## 9. 风险与已接受的取舍

- iOS 免费签名 7 天重签（体验摩擦已知并被接受；重签后通知需打开 app 重建——主循环覆盖）。
- 桌面端改动同步到手机是「尽力而为」（少开 app 会漏）——缓解：Time Sensitive + 打开即对账。
- 双原生 = 两套 UI 代码——一致性靠设计令牌 + 对齐验收（已接受，Android 只需「与 iOS 一致」）。
- gomobile 的 Go 运行时会随 core 增长（预估 < 10MB/端，已接受）。
