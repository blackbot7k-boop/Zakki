# iOS 本地通知可靠性研究（免费账号约束下）

对应 wayfinder 总图 [#1](https://github.com/gomeen/Zakki/issues/1) 的研究票
[#3](https://github.com/gomeen/Zakki/issues/3)：在「无付费 Apple 开发者账号、无自建推送服务器」
的硬约束下，iOS 客户端用本地通知（UNUserNotificationCenter + UNCalendarNotificationTrigger）
加后台刷新调度提醒，实际能达到什么可靠性水平。

前置决策（已定，不在本研究范围内重开）：做完整 iOS 客户端（查看 + 重度编辑），iOS 优先；
桌面 v1 维持现状（应用内定时器 + 系统通知投递，见
[ADR-0001](../adr/0001-inline-time-tokens.md)）。

## 背景与约束

- Zakki 数据是单个 `zakki.json`：`{version, active, tabs:[{id,name,text,color}], settings}`，
  每个标签是原始 Markdown 文本；提醒是行内令牌 `@YYYY-MM-DD HH:mm` + `~<lead>`
  （`~0m|~10m|~1h|~1d`，见 [ADR-0001](../adr/0001-inline-time-tokens.md)）。
- 硬约束：免费 Apple 账号（Personal Team）签名安装，不自建推送服务器，不用付费服务。
- 证据分级（本报告对每条事实标注）：
  - 【文档】：Apple 官方文档（developer.apple.com/documentation）逐字可查。
  - 【官方页面】：Apple 官网支持/程序页（developer.apple.com/support、/programs 等）。
  - 【会话】：Apple WWDC 官方视频转录（developer.apple.com/videos）。
  - 【第一方工具】：AltStore 官方 README（签名刷新链路的事实来源）。
  - 【实测共识】：无 Apple 逐字文档、但 iOS 平台长期实测一致的行为；设计上按最坏情况处理。

## 逐项事实（带来源）

### 1. 本地通知的调度上限与寿命

**64 条 pending 上限——属实。**
Apple 官方文档（现行唯一写明数字的位置是已被标记废弃的 `UILocalNotification` 页，数字语义
延续到 UserNotifications API）：

> "An app can have only a limited number of scheduled notifications; the system keeps the
> soonest-firing 64 notifications (with automatically rescheduled notifications counting as a
> single notification) and discards the rest."
> —— [`UILocalNotification`](https://developer.apple.com/documentation/uikit/uilocalnotification)【文档】

要点：超出部分**不是报错，而是被系统静默丢弃**；`repeats: true` 的自动重复通知只算 1 条。
对 Zakki 的含义：全量重排时每个提醒占 1 槽（一次性 `UNCalendarNotificationTrigger`），
预警冗余（一条提醒排多条）会数倍消耗 64 槽预算。

**投递由系统负责，应用进程不在场也能响。**

> "The operating system is responsible for delivering local notifications at their scheduled
> times; the app does not have to be running for this to happen."
> —— [`UILocalNotification`](https://developer.apple.com/documentation/uikit/uilocalnotification)【文档】

> "Upon calling this method, the system begins tracking the trigger conditions associated with
> your request. When the trigger condition is met, the system delivers your notification."
> —— [`UNUserNotificationCenter.add(_:withCompletionHandler:)`](https://developer.apple.com/documentation/usernotifications/unusernotificationcenter/add(_:withcompletionhandler:))【文档】

> "Once scheduled, a notification request remains active until its trigger condition is met, or
> you explicitly cancel it."
> —— [Scheduling a notification locally from your app](https://developer.apple.com/documentation/usernotifications/scheduling-a-notification-locally-from-your-app)【文档】

**设备重启：文档未逐字承诺，按实测保留、按最坏情况兜底。**现行 Apple 文档只承诺"排定后保持
活跃直到触发或显式取消""由系统负责投递"，没有一句话直接写"跨重启保留"。平台长期实测是保留
（排定状态由系统持有，与应用进程无关）【实测共识】。设计中不应把任何提醒的存续押注在这一点上，
而应以"打开 app 即对账重建"为兜底（见缓解机制）。

**免费个人签名：7 天过期、过期必须重建重装——属实。**
Apple 官方对 Personal Team 的限额原文：

> "You can register up to 10 App IDs, which expire after 7 days. … Provisioning profiles that
> enable apps to be installed on a device will expire 7 days from issuance. **You'll need to
> rebuild and reinstall your app to your device after expiration.**"
> —— [Compare memberships](https://developer.apple.com/support/compare-memberships/)【官方页面】

同页还列出：最多 3 台设备（7 天过期）、每设备最多 3 个 app、App ID 7 天过期。Xcode 文档确认
免费账号在 Xcode 里显示为 Personal Team
（[Running your app on simulated or physical devices](https://developer.apple.com/documentation/xcode/running-your-app-on-simulated-or-physical-devices)）【文档】。

**重签/重装是否会清掉已排定通知：Apple 无逐字文档，按"会清空"设计。**拆开说：

- Xcode 免费路径：profile 过期后 Apple 的原话就是"rebuild and **reinstall**"——是一次全新安装。
  iOS 平台公认行为：卸载 app 即清除其全部 pending 本地通知，通知授权也随之重置（重装后必须
  重新走 `requestAuthorization`）【实测共识】。Apple 文档只写"调度前必须请求授权"
  （[`requestAuthorization`](https://developer.apple.com/documentation/usernotifications/unusernotificationcenter/requestauthorization(options:completionhandler:))【文档】），
  未逐字写明"卸载即重置"，但这是所有本地通知依赖方的统一前提。
- AltStore 路径：AltStore 用你的个人证书重签 app，并在同一 WiFi 下周期性后台刷新以防过期：
  > "AltStore resigns apps with your personal development certificate … To prevent apps from
  > expiring, AltStore will also periodically refresh your apps in the background when on the
  > same WiFi as AltServer."
  > —— [AltStore README](https://github.com/rileytestut/AltStore#readme)【第一方工具】

  原地刷新（refresh）保留 app 数据，已排定通知理论上保留【实测共识】；但这条链路依赖
  "AltServer 在运行 + 设备与它在同一 WiFi + iOS 后台刷新可用"，本身就是新的不可靠单点。
- 过期未刷新时已排定通知是否仍投递：无任何官方文档承诺；此时 app 已无法启动，设计中必须按
  **不可依赖**处理【实测共识】。

### 2. 桌面端改动何时能到达设备并重排通知（BGAppRefresh 的真实预算）

**BGAppRefreshTask 每次启动约 30 秒，触发时机跟随用户使用习惯。**

> "your app will get 30 seconds of run time, each time it's launched, to fetch new content and
> keep itself up to date throughout the day. How often your app is launched and at what times
> depends on how the user has historically used your app. … This also means that an app that
> isn't used as frequently may not launch as frequently as well."
> —— [Advances in App Background Execution（WWDC19 Session 707）](https://developer.apple.com/videos/play/wwdc2019/707/)【会话】

**资格与预算进一步受限：**

> "we'll make sure that you're eligible to run these tasks as long as you request them when your
> app is foreground or if your app has been recently used in the foreground"
> —— 同上【会话】

调度器还综合"battery level, app usage, network connectivity, and more"决定启动时机（同上）。
使用 BGAppRefreshTask 需要声明 Background Modes 能力
（[`BGAppRefreshTask`](https://developer.apple.com/documentation/backgroundtasks/bgapprefreshtask)【文档】）。

**用户授权了通知但长期不打开 app 会漏掉什么：**

- 设备上**已经排定**的旧提醒仍会响（见上节，系统负责投递）。
- 桌面端**新改/新增/删除**的提醒全部迟到或丢失：BGAppRefresh 的启动时机按历史使用模式学习，
  低频使用的 app 启动频率随之下降，"recently used"资格不满足时预算趋近于零；两次打开 app
  之间设备端排定与最新数据之间的偏差无人纠正。
- 若用户在系统设置里关闭 Background App Refresh（全局或针对本 app），BGAppRefreshTask 不再
  运行（该能力的开关语义即"是否允许后台刷新"，见同会话与
  [`BGTaskScheduler`](https://developer.apple.com/documentation/backgroundtasks/bgtaskscheduler)【文档】）。
- 通知授权与后台刷新是两个独立开关：授权了通知完全不代表后台刷新可用。

### 3. 对比产品与推送成本

**滴答清单（TickTick）等商业产品把提醒建立在多端同步（服务器为准）之上。**
TickTick 官网首页宣称"Sync across all platforms — real-time sync"
（[ticktick.com](https://ticktick.com/)）。其官方帮助对"提醒没响"的处理是：iOS 检查系统通知
权限；Android 要求"allow TickTick to start itself and keep it running in the background"并加
厂商白名单、开"Alert Mode"（[Effective Reminder Feature 与 FAQ：What should I do if I am not
reminded when a task is due?，support.ticktick.com](https://support.ticktick.com/)）。即：
iOS 上跨设备的提醒到达依赖服务器端数据经 APNs 下发；iOS 系统里远端通知的**唯一**系统级通道
就是 APNs（"Process the payloads from remote notifications the system delivers by Apple Push
Notification service (APNs)"，
[`UNUserNotificationCenter`](https://developer.apple.com/documentation/usernotifications/unusernotificationcenter)【文档】）。

**若将来上推送，最低年成本与 APNs 门槛：**

- Apple Developer Program：**99 美元/年**（
  [programs 页](https://developer.apple.com/programs/)、
  [中文页"年会费 99 美元"](https://developer.apple.com/cn/programs/)）【官方页面】。
  这是使用推送的硬前提：能力的可用性"whether you're a member of the Apple Developer Program"
  有关（[Adding capabilities to your app](https://developer.apple.com/documentation/xcode/adding-capabilities-to-your-app)【文档】），
  Personal Team 无法为 app 启用 Push Notifications 能力【实测共识，与上一条一致】。
- APNs 侧需要一个 provider 服务器向 APNs 发请求，并用从开发者账号申请的 token 签名密钥
  （.p8 + Key ID）做 token-based 认证
  （[Establishing a token-based connection to APNs](https://developer.apple.com/documentation/usernotifications/establishing-a-token-based-connection-to-apns)【文档】）。
  即"无自建推送服务器"约束与 APNs 推送**互斥**：要么买账号 + 部署常驻 provider（或信任第三方
  中继服务），要么接受纯本地通知的可靠性上限。

### 4. 结论（先给答案）

免费约束下，iOS 端本地通知是一条"尽力而为"链路：**设备上已排定、签名链路有效、权限开启、
不撞 64 上限的提醒可以可靠触发；其余一切（桌面端改动、签名过期、低频使用）都会漏。**
逐项回答：64 条上限属实（重复触发占 1 条）；跨重启实测保留但无逐字文档承诺；免费 profile
7 天过期、Apple 原文要求过期后 rebuild + reinstall，重装后应假设 pending 通知与授权清零；
桌面端改动依赖 BGAppRefresh，其预算随使用频率衰减，授权通知但少开 app 的用户会系统性漏掉
"桌面端改的提醒"；TickTick 类产品依赖服务器同步 + APNs 兜底，上推送最低 99 美元/年且需要
provider 服务器（与当前约束冲突）。

## 漏提醒场景清单（免费约束下的诚实清单）

| # | 场景 | 后果 | 证据依据 |
|---|---|---|---|
| 1 | app 未安装 / 已被 iOS 卸载（含系统自动清理长期未用 app 的可能） | 无任何提醒 | 平台事实【实测共识】 |
| 2 | 免费 profile 超 7 天未刷新（出差、无 AltServer 同 WiFi、忘记刷新） | app 过期不可启动；已排定通知不可依赖；重装后 pending 与授权清零，全部丢失 | [Compare memberships](https://developer.apple.com/support/compare-memberships/)【官方页面】+【实测共识】 |
| 3 | 桌面端新增/修改/删除提醒，设备端低频打开 app | 新数据数天到数周不生效甚至永不生效：BGAppRefresh 预算随使用频率衰减，长期不开 app 无资格 | [WWDC19-707](https://developer.apple.com/videos/play/wwdc2019/707/)【会话】 |
| 4 | 用户关闭 Background App Refresh（全局或针对本 app） | BGAppRefreshTask 不再运行，桌面端改动永不自动到达 | 同上 + [BGAppRefreshTask](https://developer.apple.com/documentation/backgroundtasks/bgapprefreshtask)【文档】 |
| 5 | 通知权限被用户关闭、专注模式/勿扰、通知摘要改期 | 提醒被抑制或延迟（Time Sensitive 可部分缓解，见下） | [interruption levels](https://developer.apple.com/documentation/usernotifications/unnotificationinterruptionlevel)【文档】 |
| 6 | 提醒数 × 预警冗余超 64 条 pending | 超出部分被系统**静默丢弃**（保留最先触发的 64 条） | [`UILocalNotification`](https://developer.apple.com/documentation/uikit/uilocalnotification)【文档】 |
| 7 | 设备关机/重启错过触发时刻 | 跨重启保留是实测行为但无文档承诺，错过的 past-due 补投同样无承诺 |【实测共识】 |
| 8 | AltStore 链路故障（AltServer 不在、WiFi 不通、iOS 后台刷新被杀） | 刷新失败 → 场景 2 必然发生 | [AltStore README](https://github.com/rileytestut/AltStore#readme)【第一方工具】 |

注意对称性：**在 iOS app 内创建的提醒**只要满足"签名有效 + 权限开 + 槽位够"，可靠性较高；
**桌面端改动的提醒**天生低一档。可靠性上限由"最后一次打开 app 时能拿到的数据"决定。

## 缓解机制候选（按证据支撑排序）

1. **打开 app 即对账重建（最高优先）**：每次进入前台拉取最新数据，用
   [`getPendingNotificationRequests`](https://developer.apple.com/documentation/usernotifications/unusernotificationcenter/getpendingnotificationrequests(completionhandler:))【文档】
   对账后 `removeAllPendingNotificationRequests` + 全量重排（文档明示"conditions change"时
   应取消重排，
   [Scheduling a notification locally](https://developer.apple.com/documentation/usernotifications/scheduling-a-notification-locally-from-your-app)【文档】）。
   漏报窗口收敛为"两次打开 app 之间"。授权检查用 `requestAuthorization`
   （重装后必须重新请求）【文档】。
2. **前台到达的通知当场补投**：实现
   [`willPresent`](https://developer.apple.com/documentation/usernotifications/unusernotificationcenterdelegate/usernotificationcenter(_:willpresent:withcompletionhandler:))【文档】，
   app 在前台时改横幅/应用内呈现，避免"响的时候人正在 app 里却看不到"。
3. **64 槽预算管理 + 预警冗余**：每条提醒在 64 槽内排 2~3 个触发点（lead 提醒 + 远端引导
   提醒），超预算时按"最先触发优先"裁剪（系统的丢弃规则就是 soonest-firing 64）【文档】；
   循环类提醒用 `repeats: true` 只占 1 槽
   （[`UNCalendarNotificationTrigger`](https://developer.apple.com/documentation/usernotifications/uncalendarnotificationtrigger)【文档】）。
4. **Time Sensitive 中断级别**：`timeSensitive` "can break through system controls such as
   Notification Summary and Focus"，但用户可关闭
   （[`UNNotificationInterruptionLevel/timeSensitive`](https://developer.apple.com/documentation/usernotifications/unnotificationinterruptionlevel/timesensitive)【文档】）。
   对 `@…` 截止类提醒值得默认采用。**Critical alerts 不可用**：需要 Apple 专项审批的
   entitlement（[`…/critical`](https://developer.apple.com/documentation/usernotifications/unnotificationinterruptionlevel/critical)【文档】）。
5. **签名有效期运维**：把"7 天内至少一次 AltStore refresh（同一 WiFi）"写进 iOS 客户端的
   用户文档与首启引导；启动时检测 profile 剩余有效期并提示刷新（工程实现候选：读取
   embedded.mobileprovision 的过期时间）【工程候选，非文档承诺】。
6. **Notification Service Extension 不适用**：NSE 只能改写**远程**通知内容
   （[`UNNotificationServiceExtension`](https://developer.apple.com/documentation/usernotifications/unnotificationserviceextension)【文档】），
   对本地通知提醒无帮助——排除该候选。
7. **将来若上推送（推翻两条约束之一）**：99 美元/年会员
   （[programs](https://developer.apple.com/programs/)【官方页面】）+ Push 能力 + .p8 密钥 +
   常驻 provider 服务器（或第三方中继）
   （[token-based APNs](https://developer.apple.com/documentation/usernotifications/establishing-a-token-based-connection-to-apns)【文档】）。
   之后"桌面改、设备响"变成即达，NSE 也才有用武之地。

## 对后续决策的含义

- **接受免费约束 = 接受分级可靠性**：产品文案与发布说明必须如实声明"桌面端改的提醒不保证
  及时到达 iOS；7 天不刷新签名一切失效"。持久化失败要如实上报是本仓库的既有原则
  （[engineering-standard](../engineering-standard.md)），通知可靠性同理，不做静默承诺。
- **架构上把本地通知当"缓存"不当"事实源"**：iOS 端排定的通知只是"最后一次同步到的数据"的
  投影；打开即对账重建（缓解 1）应是 iOS 客户端的第一块实现，而不是附加优化。这与 Zakki
  "文本即数据"的模型一致（[ADR-0001](../adr/0001-inline-time-tokens.md)）。
- **iOS 端创建的提醒与桌面端改动的提醒应区分对待**：前者可靠、后者尽力；UI 可标注提醒的
  同步状态（已排定/待同步）。
- **99 美元/年是"尽力而为 → 服务器为准"的最低门票**，但不是全部成本：还需常驻 provider
  （与"无自建服务器"约束冲突，届时需一并重估）。wayfinder 后续票若讨论推送，应以本报告
  第 3 节为成本基线。
- 免费签名链路（AltStore refresh）本身就是 iOS 客户端最大的运维风险源，任何"自动可靠"的
  宣传都不成立；这可能反过来影响"完整 iOS 客户端"优先于其他平台的决策权重。

## 引用与可得性说明

- 所有 Apple 文档链接为 2026-10-27 实取内容（`developer.apple.com/tutorials/data/...` JSON
  端点与官方页面正文），引文为原文摘录。
- `UILocalNotification`（含 64 条上限原文）在现行文档中标记为废弃，但它是 Apple 现行可访问
  文档中唯一写明该数字与丢弃规则的页面；UserNotifications 文档未重述该限制。
- 【实测共识】各项（跨重启保留、卸载清空 pending/授权、过期不可投递、Personal Team 无推送
  能力）无 Apple 逐字文档，本报告明确标注并按最坏情况设计，不以实测为承诺。
- TickTick 官方帮助为 SPA 渲染，引文取自 [support.ticktick.com](https://support.ticktick.com/)
  页面内嵌的官方文章正文（Effective Reminder Feature、漏提醒 FAQ）。
