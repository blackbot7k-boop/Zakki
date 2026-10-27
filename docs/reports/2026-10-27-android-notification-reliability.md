# Android 提醒可靠性研究（最小成本约束下）

对应 wayfinder 总图 [#1](https://github.com/gomeen/Zakki/issues/1) 的研究票
[#10](https://github.com/gomeen/Zakki/issues/10)：在「最小成本」约束下，Android 客户端的
行内令牌提醒（`@YYYY-MM-DD HH:mm` + `~<lead>`）实际能达到什么可靠性水平；与 iOS 免费路径
（见 [iOS 本地通知可靠性研究](2026-10-27-ios-local-notification-reliability.md)）对比，
双端各自的漏提醒边界在哪里。

前置决策（不在本研究范围内重开）：做完整移动客户端，iOS 与 Android 都在范围内，整体最小
成本。Android 侧新增的问题：Google 侧没有 iOS 那种"免费账号 7 天重签"的死结，那成本省在
哪里、可靠性又在哪里流失，是本报告的主线。

## 背景与约束

- Zakki 数据是单个 `zakki.json`；提醒是行内令牌 `@YYYY-MM-DD HH:mm` + `~<lead>`
  （见 [ADR-0001](../adr/0001-inline-time-tokens.md)）。移动端只需把这些令牌翻译成
  系统级定时事件，没有服务端也能自洽。
- 硬约束：最小成本。即不买 Apple Developer Program（99 美元/年），Android 侧也默认不上
  付费服务；自建常驻推送 provider 与现有"无自建服务器"偏好冲突，仅在必要时重估。
- 证据分级（沿用 iOS 报告的标注习惯，并对 Android 生态补充两档）：
  - 【文档】：Google 官方文档（developer.android.com、firebase.google.com）逐字可查。
  - 【官方页面】：Google 官方帮助/政策页（support.google.com 等）。
  - 【厂商文档】：厂商开发者平台页面（小米澎湃OS开发者平台、vivo 开放平台等）。
  - 【第一方工具】：TickTick 官方帮助中心。
  - 【实测共识】：无逐字文档、但平台长期实测一致的行为；按最坏情况设计。
  - 【平台公认事实】：厂商公开发布、被行业普遍接受的平台事实；本报告未能取到逐字原文，
    设计上不单独押注。

## 逐项事实（带来源）

### 1. 精确闹钟：能力存在，但 Android 12 起权限全面收紧

**Android 12+ 设置精确闹钟必须持有"闹钟和提醒"特殊访问权限。**

> "apps that target Android 12 and higher and set exact alarms must have access to the
> 'Alarms & reminders' capability that appears within the Special app access screen in
> system settings. To obtain this special app access, request the SCHEDULE_EXACT_ALARM
> permission in the manifest."
> —— [Android 12 行为变更](https://developer.android.com/about/versions/12/behavior-changes-12)【文档】

没有权限时是硬失败而非降级：

> "Note: If your app targets Android 12 or higher, you must declare one of the
> 'Alarms & reminders' permissions. Otherwise, a SecurityException occurs."
> —— [Schedule alarms](https://developer.android.com/develop/background-work/background-tasks/services/alarms)【文档】

`setExact()`、`setExactAndAllowWhileIdle()`、`setAlarmClock()` 三个精确 API 都需要
SCHEDULE_EXACT_ALARM，"否则抛出 SecurityException"
（[Android 14 精确闹钟变更](https://developer.android.com/about/versions/14/changes/schedule-exact-alarms)）【文档】。

**两个权限的分工（对 Zakki 这类"提醒"应用只有一条路）：**

> "USE_EXACT_ALARM — Granted automatically / Cannot be revoked by the user / Subject to an
> upcoming Google Play policy / Limited use cases.
> SCHEDULE_EXACT_ALARM — Granted by the user / Broader set of use cases / Apps should confirm
> that the permission has not been revoked."
> —— [Schedule alarms](https://developer.android.com/develop/background-work/background-tasks/services/alarms)【文档】

USE_EXACT_ALARM 是"自动授予、用户不可撤销"的 normal permission，但 Play 政策只允许
日历/闹钟类核心功能应用使用；Android 14 文档原话：

> "Calendar or alarm clock apps need to send calendar reminders, wake-up alarms, or alerts
> when the app is no longer running. These apps can request the USE_EXACT_ALARM normal
> permission. … Apps will not be able to publish a version of their app with this permission
> in the manifest unless they qualify based on the policy language."
> —— [Android 14 行为变更](https://developer.android.com/about/versions/14/changes/schedule-exact-alarms)【文档】
> （"policy language" 链接到 Play 的
> [Policy Deadlines](https://support.google.com/googleplay/android-developer/answer/12253906#exact_alarm_preview)
> 页 exact_alarm_preview 锚点【官方页面】；该页为 JS 渲染，正文按日期滚动归档。）

Zakki 是便签提醒、不是闹钟/日历应用 ⇒ **只能走 SCHEDULE_EXACT_ALARM 用户授权路径**，
USE_EXACT_ALARM 属于政策违规风险。

**Android 13/14 起，这个权限默认不给新安装：**

> "SCHEDULE_EXACT_ALARM … is no longer being pre-granted to most newly installed apps
> targeting Android 13 and higher (will be set to denied by default). If the user transfers
> app data to a device running Android 14 through a backup-and-restore operation, the
> permission will still be denied."
> —— [Android 14 行为变更](https://developer.android.com/about/versions/14/changes/schedule-exact-alarms)【文档】

即首启必须引导用户去系统设置（`ACTION_REQUEST_SCHEDULE_EXACT_ALARM`）授权，且授权是可以被
用户随时收回的；收回的后果很重：

> "When the SCHEDULE_EXACT_ALARM permission is revoked for your app, your app stops, and all
> future exact alarms are canceled."
> —— [Schedule alarms](https://developer.android.com/develop/background-work/background-tasks/services/alarms)【文档】

### 2. Doze / App Standby / 重启：闹钟的物理边界

**Doze 会把普通闹钟推迟到维护窗口；三类"放行"通道各有代价。**

> "Defers standard AlarmManager alarms, including setExact() and setWindow(), to the next
> maintenance window. If you need to set alarms that fire while in Doze, use
> setAndAllowWhileIdle() or setExactAndAllowWhileIdle(). Alarms set with setAlarmClock()
> continue to fire normally. The system exits Doze shortly before those alarms fire."
> —— [Optimize for Doze and App Standby](https://developer.android.com/training/monitoring-device-state/doze-standby)【文档】

allow-while-idle 系列有节流上限：

> "Note: Neither setAndAllowWhileIdle() nor setExactAndAllowWhileIdle() can fire alarms more
> than once per nine minutes, per app."
> —— 同上【文档】

**重启清空是逐字文档，不是实测推测：**

> "By default, all alarms are canceled when a device shuts down. To prevent this from
> happening, you can design your application to automatically restart a repeating alarm if
> the user reboots the device."
> —— [Schedule alarms](https://developer.android.com/develop/background-work/background-tasks/services/alarms)【文档】

重建依赖 `RECEIVE_BOOT_COMPLETED`，且文档明确加了一个限定："this only works if the app has
already been launched by the user at least once"（同页）【文档】——即 app 处于 stopped 状态
（从未启动或被 force-stop）时收不到开机广播。

**用户主动限制：行为差的 app 会被用户授权系统限制，连闹钟都停。**

> "If an app exhibits some of the bad behaviors described in Android vitals, the system
> prompts the user to restrict that app's access to system resources. … The precise
> restrictions imposed are determined by the device manufacturer. For example, on AOSP
> builds, restricted apps cannot run jobs, trigger alarms, or use the network, except when
> the app is in the foreground."
> —— [System restrictions on background tasks](https://developer.android.com/develop/background-work/background-tasks/bg-work-restrictions)【文档】

### 3. WorkManager：可靠，但只有"分钟级窗口"，不是提醒原语

WorkManager 的定位是"保证执行、可推迟"，官方用它对照闹钟：

> "WorkManager features a robust scheduling mechanism that lets tasks persist across app
> restarts and device reboots. … Deferrable: Scheduled tasks that start at a later time and
> can run periodically."
> —— [Background tasks: persistent work](https://developer.android.com/develop/background-work/background-tasks/persistent)【文档】

周期下限 15 分钟（"You can provide a repeat interval and flexInterval (15 minutes minimum)"，
[Android 14 精确闹钟变更](https://developer.android.com/about/versions/14/changes/schedule-exact-alarms)）【文档】。
Doze 期间完全停摆：

> "JobScheduler run: No. WorkManager uses JobScheduler internally, so WorkManager tasks don't
> run."
> —— [Optimize for Doze and App Standby](https://developer.android.com/training/monitoring-device-state/doze-standby)【文档】

官方对两者的分工建议：

> "Unlike WorkManager's regular workers, AlarmManager's exact alarms wake a device from Doze
> mode. … Only use it for precise alarms or notifications such as calendar events, not for
> recurring background work."
> —— [Background tasks: persistent work](https://developer.android.com/develop/background-work/background-tasks/persistent)【文档】

对 Zakki 的含义：WorkManager 适合做"打开前的数据同步兜底"，**不能**承担 `@HH:mm` 提醒；
提醒必须走精确闹钟（要授权）或 FCM（要 GMS），没有中间路线。另注意
`setRepeating()` 自 API 19 起全部是非精确闹钟（"all repeating alarms are inexact alarms"，
[Schedule alarms](https://developer.android.com/develop/background-work/background-tasks/services/alarms)）【文档】，
循环提醒不能用它实现分钟级准时。

### 4. FCM 推送：免费属实，通道强依赖 GMS，自建 provider 是"便宜但常驻"

**FCM 本身免费——逐字：**

> "Cloud Messaging (FCM) — No-cost"
> —— [Firebase pricing](https://firebase.google.com/pricing)【文档】

Google Play 开发者账号也只是一次性费用：

> "There is a US$25 one-time registration fee."
> —— [Register for a Google Play Developer account](https://support.google.com/googleplay/android-developer/answer/6112435)【官方页面】

且 **FCM 不要求 Google Play 分发**——这是与 iOS 完全不同的开放度：

> "FCM clients require devices running Android 7.0 or higher that also have the Google Play
> Store app installed, or an emulator running Android 7.0 with Google APIs. Note that you are
> not limited to deploying your Android apps through Google Play Store."
> —— [Set up a Firebase Cloud Messaging client app on Android](https://firebase.google.com/docs/cloud-messaging/android/client)【文档】

（若走 Play 分发，2023-11-13 之后注册的个人账号还有冷启动门槛："must run a closed test for
their app with a minimum of 12 testers who have been opted in continuously for at least
14 days"，见 [App testing requirements](https://support.google.com/googleplay/android-developer/answer/14151465)【官方页面】。）

**通道依赖是排他的：有 Play Store（GMS）才有 FCM。** 国行 ROM 的现实含义见第 5 节。

**自建 provider 的最低要求与成本结构：**
发消息走 HTTP v1 API，服务端凭据是"service account JSON + 由其派生的短时 OAuth 2.0
access token"（
[Authenticate FCM send requests](https://firebase.google.com/docs/cloud-messaging/auth-server)）【文档】；
每台设备的注册 token 需要服务端持有并在卸载/重装后维护。金钱成本可以为零（一个 serverless
函数 + 免费额度即可承载 Zakki 的提醒量级），但它和 Apple APNs provider 一样是**常驻组件**：
Zakki 桌面端"提醒时间到了"要有人替它调 FCM。与 iOS 的差别是门槛费：FCM 路径 0 元/年 + 一次性
25 美元（可选），APNs 路径 99 美元/年（见 iOS 报告第 3 节）。

**FCM 的送达语义不是"必达"，两层折扣要计入设计：**

- Doze 下普通优先级消息可能延迟："Normal priority messages are delivered immediately when the
  device is not sleeping. When the device is in Doze mode, delivery may be delayed to conserve
  battery until the device exits doze."；高优先级则 "attempts to deliver high priority
  messages immediately, allowing FCM to wake a sleeping device when necessary"
  （[Message priority](https://firebase.google.com/docs/cloud-messaging/android/message-priority)）【文档】。
- 高优先级被降级机制："If FCM detects a pattern in which messages don't result in
  user-facing notifications, your messages may be deprioritized to normal priority … FCM uses
  7 days of message behavior when determining whether to deprioritize or proxy messages"
  （[Message deprioritization on Android](https://firebase.google.com/docs/cloud-messaging/android-message-priority)）【文档】。
  即用户长期不看到通知时，FCM 会把提醒静默降级——对提醒类应用必须监控这一状态。

### 5. 国行 ROM 现实：后台查杀是常态，厂商通道是"合规买路钱"

**第一块硬事实：华为/荣耀的 HarmonyOS 生态没有 GMS，FCM 整条路不存在。**
上面 FCM 客户端要求"设备装有 Google Play Store"【文档】，而华为 2020 年后国行设备预装的是
HMS 而非 GMS；HarmonyOS NEXT 更是完全不运行 Android APK【平台公认事实】。Android 版 Zakki
在这些设备上连"收到推送"这一格都是空的。

**厂商为什么自建推送通道——vivo 的官方表述就是证词：**

> "OS系统级通道，提供稳定、可靠、高效的推送服务。"
> "vivo系统级通道，稳定长连接，实现推送速度和到达率双重提升。"
> —— [vivo 消息推送](https://developers.vivo.com/product/d/messagePush)【厂商文档】

翻译过来：普通应用自己保持的长连接在 vivo ROM 上到达率不行，所以要"系统级通道"。小米、
OPPO、华为、荣耀同理（各家推送服务的存在本身即是平台承认默认链路不可靠）【厂商文档 + 实测共识】。

**厂商通道对"不在其商店分发"的应用不关门，但设置了多重门槛（以小米为例，规则全部逐字可查）：**

- **接入即审核**：启用推送须先同意《技术服务协议》等四份文件，然后创建通知类别（最多 8 个）
  提交审核，"我们会在 3-5 个工作日内完成审核"
  （[小米推送服务启用指南](https://dev.mi.com/xiaomihyperos/documentation/detail?pId=1542)）【厂商文档】。
- **配额按通道分级**：默认通道"单个应用单个设备单日 1 条"；公信消息"单个应用单个设备单日
  5-8 条"且总量=通知开启数×倍数；私信消息"不限量"，但需按《小米推送2026年消息分类新规》申请
  通道，且"2026年07月01日起接入小米推送的应用需使用模板下发私信消息"
  （[推送消息限制说明](https://dev.mi.com/xiaomihyperos/documentation/detail?pId=1656)）【厂商文档】。
  提醒类内容（日程管理、行程提醒等）落在私信/公信场景表内，即 Zakki 必须走申请制通道才能
  逃避"每设备每天 1 条"的默认配额。
- **内容安全审计前置**："MiPush禁止推送包含敏感词的消息，建议App的运营人员在推送消息的时候
  基于三方平台（推必安）的审核结果来进行消息推送"，推送请求需附带审计结果，不通过即拦截
  （[基于第三方审核结果的消息推送](https://dev.mi.com/xiaomihyperos/documentation/detail?pId=1563)）【厂商文档】。
  一个个人便签应用的"提醒"要过第三方内容审核，这是国行通道的隐性运营成本。

**后台查杀对提醒的实际影响——TickTick 官方 FAQ 是最好的一手证词**（iOS 报告已引用同一来源）：

> "Has TickTick already got the notification permission? … Does the phone system have its own
> security service? If so, please allow TickTick to start itself and keep it running in the
> background. … Do you have a third-party security application installed? If so, please make
> sure that TickTick is added to the startup whitelist and memory acceleration whitelist in
> every security software."
> —— [TickTick FAQ: What should I do if I am not reminded when a task is due?](https://help.ticktick.com/articles/7055792921664028672)【第一方工具】

即最大的待办应用对国行 ROM 的应对方式是**引导用户手动放开自启动/后台/白名单**，而不是某种
程序化 API；这是当前生态下可复现的最优实践。社区侧的厂商差异汇总见
[dontkillmyapp.com](https://dontkillmyapp.com/)（第三方聚合站，非一手来源，仅作场景索引，
不作为本报告的事实依据）。

**闹钟通道在国行 ROM 的存活率**：`AlarmManager` 闹钟由系统调度，进程被普通上滑清理后仍应
触发【实测共识】；但 force-stop（含部分 ROM 的"一键清理"，行为等同 force-stop）会清空闹钟，
且 stopped 状态收不到 BOOT_COMPLETED【文档 + 实测共识】。厂商"自启动"开关被关闭时，重启后
重建与 FCM 回调都可能失效【实测共识，与 TickTick 指引一致】。

## Android 漏提醒场景清单（最小成本下的诚实清单）

| # | 场景 | 后果 | 证据依据 |
|---|---|---|---|
| 1 | 无 GMS 设备（华为/荣耀 HarmonyOS 等） | FCM 不可用；厂商通道未接入则服务器提醒全空 | FCM 客户端要求【文档】+【平台公认事实】 |
| 2 | 用户 force-stop，或 ROM"一键清理"等同 force-stop | 已排定闹钟清零；收不到 BOOT_COMPLETED，重启后不再重建 | 【文档】+【实测共识】 |
| 3 | 厂商"自启动"关闭、后台被查杀（MIUI/HarmonyOS/ColorOS/OriginOS 常态） | 重启后无法重建闹钟；FCM 回调无法落地通知 | 【实测共识】+ TickTick FAQ【第一方工具】 |
| 4 | Android 13+ 用户未授予"闹钟和提醒" | 精确闹钟抛 SecurityException，`@HH:mm` 完全无法排定 | 【文档】 |
| 5 | 用户随后收回 SCHEDULE_EXACT_ALARM | app 进程被系统停止，未来精确闹钟全部取消 | 【文档】 |
| 6 | 设备重启且未及时打开 app | 闹钟已清空；BOOT 重建依赖 RECEIVE_BOOT_COMPLETED + 自启动存活 | 【文档】+【实测共识】 |
| 7 | Doze 深休眠 + 未用 allow-while-idle/setAlarmClock | 普通闹钟推迟到维护窗口，提醒迟到数小时 | 【文档】 |
| 8 | FCM 消息 7 天"无用户可见通知"模式 | 高优先级被降级为普通优先级，Doze 下延迟 | 【文档】 |
| 9 | 走小米等厂商通道但未申请私信类通道 | 默认通道 1 条/设备/日，提醒被配额拦截；公信 5-8 条/日 | 【厂商文档】 |
| 10 | 通知权限关闭/免打扰（与 iOS 相同的用户侧开关） | 提醒被抑制 | 平台事实【实测共识】 |

**结构性结论**：Android 端"**app 内创建的、闹钟权限齐全的提醒**"在 GMS+原生 ROM 上可以做到
很高可靠性（闹钟由系统持有、无 64 条上限、无 7 天重签）；**可靠性流失集中在两端**——一端是
权限与生命周期（重启、force-stop、自启动），另一端是"桌面改、设备响"的同步链路（无 GMS 时
没有免费的服务器通道；有 GMS 时 FCM 免费但受 Doze/降级规则约束）。

## 双端分级可靠性表

"提醒何时能响"按能力分级（✅ 可依赖 / 🟡 尽力而为 / ❌ 不可用），成本列为该级别的最小门票：

| 能力/场景 | iOS（免费账号） | Android（最小成本） |
|---|---|---|
| 本地精确触发（`@HH:mm`） | 🟡 系统投递可靠，但受 64 条上限、免费签名 7 天过期约束 | 🟡 需用户授予 SCHEDULE_EXACT_ALARM（13+ 默认不给）；授权后系统持有闹钟，无条数上限 |
| 权限有效期 | ❌ 免费 profile 7 天过期，过期后 pending 清零、须重装 | ✅ APK 安装长期有效，无重签死结 |
| 数量上限 | 🟡 64 条 pending，超出静默丢弃 | ✅ 无系统条数上限（厂商通道另有日配额，见下） |
| 重启后存续 | 🟡 实测保留但无文档承诺 | 🟡 文档明确清空，靠 BOOT_COMPLETED 重建（依赖自启动） |
| 进程被清理后触发 | ✅ 系统投递，进程无关 | 🟡 普通清理不影响；force-stop 等同清零【实测共识】 |
| 桌面改动到达设备 | 🟡 BGAppRefresh 预算随使用衰减，长期不开 app 不到达 | 🟡 同步时机同 iOS；有 GMS 时可加 FCM 兜底，无 GMS 时与 iOS 同级或更差 |
| 服务器实时推送 | ❌ APNs = 99 美元/年 + 常驻 provider | 🟡 FCM 免费 + 常驻 provider（金钱近零、运维常驻），但受 Doze 延迟与 7 天降级规则约束 |
| 国行 ROM 推送 | （无对应问题） | ❌→🟡 FCM 无 GMS 不可用；厂商通道逐家接入：注册+审核+通道申请+模板+内容审计，成本是合规运营而非金钱 |
| 官方推荐的替代原语 | —— | WorkManager（保执行、15 分钟窗口、Doze 停摆）——只能做同步兜底，做不了提醒 |

## 对后续决策的含义

- **Android 的提醒架构应当是三层**：① 打开即对账 + 全量重排（与 iOS 报告的缓解 1 相同，
  双端第一优先级）；② 精确闹钟（首启引导 SCHEDULE_EXACT_ALARM 授权、BOOT_COMPLETED 重建、
  `setAlarmClock()` 走 Doze 直通）；③ FCM 作为"桌面改、设备响"的免费兜底（仅 GMS 设备），
  服务端 provider 与将来 iOS 推送共用 Zakki 同步设施。WorkManager 只做数据同步，不做提醒。
- **"最小成本"在 Android 侧的真实价格签**：金钱近零（FCM 免费、Play 一次性 25 美元且可
  侧载绕开），但把成本转移给了三处——用户授权摩擦（精确闹钟权限引导）、生命周期脆弱性
  （force-stop/重启依赖对账兜底）、国行合规运营（厂商通道逐家接入与内容审计）。没有
  "免费且省心"的 Android 推送；只有"免费但要把不可靠如实告知"的方案。
- **与 iOS 合并看，双端可靠性排名**：GMS+原生 ROM 上的"app 内创建提醒" > iOS 免费路径 >
  国行 ROM 无厂商通道。产品文案应按此分级声明，不做统一承诺（延续"持久化失败必须如实上报"
  的仓库原则）。
- **国行决策推迟到有真实用户信号后再做**：厂商通道接入是每家一套的注册/审核/模板/审计流程
  （小米规则见第 5 节），属于运营投入而非代码投入；建议作为独立后续票评估，不阻塞 Android
  客户端 v1（v1 用 ①+②+③ 已能覆盖大部分场景）。
- **FCM 高优先级降级监控是提醒类应用的硬要求**：7 天"通知不可见"即被降级（第 4 节引文），
  Android 端需要保证通知到达即呈现，并把降级状态纳入诊断。

## 引用与可得性说明

- 所有 Google 文档链接为 2026-10-27 实取内容，引文为原文摘录；Android 文档站当前将闹钟指南
  置于 `developer/develop/background-work/background-tasks/services/alarms`（原
  `/training/scheduling/alarms` 重定向同页）。
- `support.google.com` 的 Play 政策页（exact alarm declaration）原页面（answer/13316128）已
  不可访问；本报告引用 Android 14 官方文档中对 Play 政策的转述及其指向的 Policy Deadlines
  锚点，未引用第三方转述。
- 【厂商文档】各项取自小米澎湃OS开发者平台与 vivo 开放平台（SPA 页面经渲染代理抓取正文）；
  OPPO/华为/荣耀的对应页面未能取得稳定正文，本报告对未核验的厂商仅做存在性陈述并标注
  【实测共识】，不引用具体配额数字。
- 【平台公认事实】（HarmonyOS NEXT 不兼容 APK、国行设备无 GMS）无逐字引用，按行业公认处理，
  设计上不单独押注。
- TickTick 引文取自其官方帮助中心 FAQ 正文（SPA 经渲染代理抓取），与 iOS 报告引用的
  "Effective Reminder"帮助同源。
