# 移动端技术栈补充研究：Android 加入后的双端视角（wayfinder #9）

研究性质：事实采集，不含最终决策，不重复 #5 已覆盖的 iOS 侧细节，只补 Android 加入后的增量事实。
GitHub 数据（星数 / 最近推送 / issue 状态 / release 时间）取自 GitHub API，检索日期 2026-10-27；
issue 的 open/closed 状态随时间变化，以检索当日为准。其余事实均附一手来源链接。
前置报告：[2026-10-27-ios-client-stack-options.md](2026-10-27-ios-client-stack-options.md)（下称 #5）、
[2026-10-27-ios-local-notification-reliability.md](2026-10-27-ios-local-notification-reliability.md)（下称 #3）。

## 背景与约束

- 目的地已重画（2026-10-27，[wayfinder #9](https://github.com/gomeen/Zakki/issues/9)）：iOS 与 Android 双端都在范围内；两平台**同时交付还是先后交付是后续决策**，本研究只给成本差异事实。
- 数据模型不变：单文件 `zakki.json`（`internal/notes/file.go` 注释："File 是 zakki.json 的磁盘格式"）；时间功能是行内文本令牌 `@YYYY-MM-DD HH:mm` + `~<提前量>`（[ADR-0001](../adr/0001-inline-time-tokens.md)），解析跳过围栏代码块、行内代码、链接与转义（ADR-0001 Consequences）。
- 硬约束不变（#5）：成本最小化（无付费 Apple 开发者账号）、**重度编辑角色**（WYSIWYG + 源码模式）、与单 JSON + 行内令牌模型贴合。
- 解析内核现状：`internal/markdown` 是"Markdown 解析、格式变换、表格纯函数（无 MyGo 依赖）"（[architecture.md](../architecture.md)）；桌面测试以 `CGO_ENABLED=0` 运行（`Makefile`）。

## 候选逐项事实

### 1. Kotlin Multiplatform / Compose Multiplatform（#5 未覆盖）

**稳定性：双平台都已 Stable，是本次检索最硬的增量事实。**

JetBrains 官方稳定性页（页面更新日期 2025-09-10）给出的现行等级表：

- KMP 核心技术：Android **Stable**、iOS **Stable**（watchOS/tvOS Beta、Kotlin/Wasm Beta）。
- Compose Multiplatform UI 框架：Android **Stable**、iOS **Stable**、Web（Kotlin/Wasm）Beta。

该页对 Stable 的定义原文："you can use it even in the most conservative of scenarios"，且 "API-breaking changes can only be made 2 versions after an official deprecation announcement"（[Supported platforms](https://www.jetbrains.com/help/kotlin-multiplatform-dev/supported-platforms.html)）。

iOS 转 Stable 的时点是 Compose Multiplatform 1.8.0（2025-05），公告标题即 "Compose Multiplatform for iOS Is Stable and Production-Ready"，原文要点："brings Compose for iOS to Stable"、"Feature parity with Jetpack Compose for all popular use cases"、"Text editing with native selection and right-to-left support"、iOS 侧体积代价 "adds only ~9 MB"（[JetBrains 博客](https://blog.jetbrains.com/kotlin/2025/05/compose-multiplatform-1-8-0-released-compose-multiplatform-for-ios-is-stable-and-production-ready/)）。

**Android 侧就是 Jetpack Compose 本尊。**官方关系页原文："Compose Multiplatform is a cross-platform UI toolkit developed by JetBrains. It extends Google's Jetpack Compose toolkit for Android by supporting additional target platforms"；"When you build your Compose Multiplatform application for Android, you use the Jetpack Compose artifacts published by Google"（[Relationship between CMP and Jetpack Compose](https://www.jetbrains.com/help/kotlin-multiplatform-dev/compose-multiplatform-and-jetpack-compose.html)）。即 Android 侧的 UI 栈与"纯 Android 原生开发"用的是同一批 Google 官方组件。

**版本与 iOS 文本打磨节奏**（GitHub Releases API 2026-10-27）：现行 stable v1.12.2（2026-10-15 发布）、v1.13.0-beta01（2026-10-23）。1.12.0 的 iOS 段仍在做文本细节："The selection handles now match the style of iOS"、BasicTextField 在 Android/iOS 报告不一致的修复、Apple Pencil Scribble 在 TextField 中的崩溃修复（[v1.12.0 release notes](https://github.com/JetBrains/compose-multiplatform/releases/tag/v1.12.0)）。

**iOS 编辑问题史**（标题检索 `TextField iOS in:title` 按创建时间倒序，GitHub 2026-10-27）：最新 20 条**全部已关闭**，最新一条创建于 2025-03；早期集中爆发（#3101 韩文无法正常输入、#3416 日文转换候选不显示、#3530 键盘不弹出、#3621 键盘/TextField 问题 19 评论、#3811 BasicTextField 稳定崩溃）均关闭。CMP 的跨端行为差异也有修复记录（#5009 iOS/Android IME padding 偏移，2024-09 关闭）。

**编辑组件生态**（GitHub API 2026-10-27）：

| 组件 | 定位 | 星数/最近推送 | 关键事实 |
|---|---|---|---|
| [compose-rich-editor](https://github.com/MohamedRejeb/compose-rich-editor) | CMP 富文本**编辑** | 1863★，2026-10-09 推送 | README："A Rich text editor library for both Jetpack Compose and Compose Multiplatform…supports HTML and Markdown"、"supports Compose Multiplatform (Android, iOS, Desktop, Web)"、"is a WYSIWYG editor"；Markdown **双向 API**：`setMarkdown` / `toMarkdown` / `insertMarkdown`（[README](https://github.com/MohamedRejeb/compose-rich-editor#readme)） |
| [multiplatform-markdown-renderer](https://github.com/mikepenz/multiplatform-markdown-renderer) | Markdown **渲染**（不编辑） | 1124★，2026-10-15 推送 | "Render Markdown as native Compose UI — one API, every platform"，基于 JetBrains Markdown AST（README） |

compose-rich-editor 自身的 IME/编辑问题已有一批修复记录：#779（Gboard 联想词替换吞掉段落边界）、#640（Android IME 组词态回车丢列表格式）、#595（KMP iOS 构建失败）均已关闭（GitHub 2026-10-27）。

**风险面**：CMP 官方组件清单（Animation/Compiler/Foundation/Material/Runtime/UI 等）中**没有**"编辑态文本内嵌自定义视图"的 API（对比 iOS UIKit 的 `NSTextAttachmentViewProvider`，见 #5）；行内时间组件在编辑态仍需基于 Compose 自绘实现（同桌面 live-preview 的自建路径）。iOS 侧编辑深度上限低于系统 TextKit/UITextView 直改造，这一点无官方文档承诺，属结构推断。

### 2. gomobile：Android 侧与 iOS 侧对照

**同一工具、同一包，双端出产物**（[gomobile 文档](https://pkg.go.dev/golang.org/x/mobile/cmd/gomobile)）：

- `gomobile bind -target android` 产出 **AAR**（原文 "archives the precompiled Java API stub classes, the compiled shared libraries, and all asset files"），Android Studio 1.2+ 可直接导入。
- Apple 目标（ios/iossimulator/macos/maccatalyst）"must be run on an OS X machine with Xcode installed"，bind 产出 **XCFramework**；iOS 最低 SDK 默认 13.0。
- Android 构建要求 javac 1.8+、Android SDK（API 16+）、`ANDROID_NDK_HOME`。**Android 侧无 macOS/Xcode 依赖**，CI 面更宽。

**绑定面双端一致**（[gobind 文档](https://pkg.go.dev/golang.org/x/mobile/cmd/gobind)）：受支持类型为有符号/无符号整型、浮点、string、bool、[]byte（引用传递）、参数与返回值全为受支持类型的函数、导出方法受限的 interface/struct；"Exceptions and panics are not yet supported. If either pass a language boundary, the program will exit."。反向绑定 Java 与 Objective-C **都**支持（"gobind recognizes import statements that start with `Java/` or `ObjC/`"），且官方示例即"继承 Android Activity"。→ #5 的结论"窄接口：bytes in / bytes out，Swift 侧不持有 Go 对象"**在 Android 侧原样成立，无新增限制**。

**维护状态**（GitHub API 2026-10-27）：[golang/mobile](https://github.com/golang/mobile) 6227★，2026-10-11 仍有推送；README 原文 "The Go Mobile project is experimental. Use this at your own risk. … neither Google nor the Go team can provide end-user support."——与 #5 记录一致，工具链风险未变化。

**纯 Go 无 cgo 的可迁移性**：Zakki 逻辑包是纯函数/纯 Go（`internal/markdown` 无 MyGo 依赖，[architecture.md](../architecture.md)），桌面测试 `CGO_ENABLED=0`（`Makefile`）；Go 的 internal 包不能被外部模块 import，需在仓库内加非 internal 薄包装包（Go 1.4 release notes，引 #5 §5）。Android 侧的 JNI 桥由 gomobile 工具链生成，**Zakki 代码侧不引入 cgo**——与 iOS 侧同构。

**Android 特有的新校验点**：Google Play 要求 "all apps targeting Android 15 (API level 35) and higher must support 16 KB memory page sizes on 64-bit devices on Google Play"，且 "Starting February 1, 2027, if your app updates don't support 16 KB memory page sizes, you won't be able to release these updates"；官方明确 "If your app uses any NDK libraries, either directly or indirectly through an SDK, then you will need to rebuild your app"，而 "If your app only uses code written in the Java programming language or in Kotlin… your app already supports 16 KB devices"（[Support 16 KB page sizes](https://developer.android.com/guide/practices/page-sizes)）。gomobile 产物含 Go 编译的 `.so`（native 库），**是否默认满足 16 KB 对齐未在本次检索中确认**，列入开放点。

### 3. Flutter Android 重度编辑（IME / 选区 / 键盘 / CJK）

**引擎/框架层**（flutter/flutter，GitHub 检索 2026-10-27）：

- [#141557](https://github.com/flutter/flutter/issues/141557)：**open**，2024-01-15 创建至今，"[Android] App is crashing in production: `IndexOutOfBoundsException: invalid composing start`"——Android 组词区线上崩溃，开放近三年。
- [#165734](https://github.com/flutter/flutter/issues/165734)：open，"[Android] IME connection closure isn't reported to app when using swipe to go back"。
- [#180887](https://github.com/flutter/flutter/issues/180887)：closed 2026-01-13，"[Android][Samsung] Japanese IME fails to convert symmetric characters"——三星键盘日文转换失败 2026 年才修复。
- [#157092](https://github.com/flutter/flutter/issues/157092)：closed 2024-10-17，"[Android]: no event is triggered after text composing in Korean"。
- 同期 iOS/web 侧也有开放项（#171068 拼音输入焦点切换留隐形字符、#174159 composing 结果在 Web/iOS 错误、#193846 web 无障碍开启后 preedit 重复）——composing 管线是 Flutter **全平台持续摩擦面**，不只是 Android。

**编辑器组件层**（[super_editor](https://github.com/Flutter-Bounty-Hunters/super_editor)，1937★，2026-10-22 推送）open 状态的 CJK/IME 清单：#2993（日文 IME 按空格光标移动）、#2217（SuperTextField 日文转换重复）、#2728（中文组词末尾删除崩溃）、#743（韩文坏文本）、#2757（韩文 Shift+Enter 字符重复）、[#2070](https://github.com/Flutter-Bounty-Hunters/super_editor/issues/2070)（Android：键盘带 OS 联想词时选中非文本节点抛异常，11 评论）、#1333（移动键盘联想词导致 IME 错误）；iOS 韩文组词问题一组已修复（#1825/#1826/#1828）——证明可修，也证明是活跃摩擦面。#5 对 Flutter 编辑器"模型↔Markdown 往返保真"的结论与平台无关，继续成立。

### 4. React Native Android 重度编辑

- 仓库已迁移至 [react/react-native](https://github.com/react/react-native)（GitHub API 2026-10-27，126806★；旧地址 facebook/react-native 重定向）。
- **最重的开放问题在 iOS 侧**：[#56463](https://github.com/react/react-native/issues/56463)（open，2026-04-16）"CJK (Chinese/Japanese/Korean) IME input is fundamentally broken on iOS Fabric — affecting all New Architecture users since 0.76"；#48497（open，2025-01）CJK 转换下划线消失；#30578（open，2020-12 起）中文键盘 onChangeText 触发时机错误，开放近 6 年。同一套 TextInput 核心双端共用，iOS Fabric 的破坏级开放问题是该栈文本输入层脆弱性的直接证据。
- Android 侧标题检索（`android composing in:title`）未返回同级 CJK 开放问题；IME 专项如 #58380（`keyboardType="url"` 的 inputType 问题）已关闭；New Architecture 下 TextInput 是状态同步区，历史案例 #31996（state update 打断 CJK 输入模式）已关闭。
- 编辑器底座不变：#5 已记录 10tap-editor（1194★，2026-10-19 推送）基于 Tiptap/ProseMirror 运行在 `react-native-webview` 内——Android 侧即 Android WebView（Chromium），重度编辑手感风险与 #5 §4 的 webview 分析同构。

### 5. 双原生 UI（SwiftUI + Kotlin/Compose）+ 共享 gomobile 解析内核

- Android 原生 UI 栈即 Jetpack Compose：Google 官方定义 "Jetpack Compose is a declarative UI toolkit for building native Android interfaces"（[CMP/Jetpack Compose 关系页](https://www.jetbrains.com/help/kotlin-multiplatform-dev/compose-multiplatform-and-jetpack-compose.html)）。**"双原生"路线在 Android 侧没有 iOS 26 styled `TextEditor` 式的年轻 API 疑虑**（#5 §1 的 iOS 悬念在 Android 不存在）——编辑底座就是平台主路径。
- 共享内核的增量近零：gomobile bind 用**同一个 Go 包**既产 AAR 又产 XCFramework（§2 引文）；解析、令牌、提醒派生一套实现，两端同一二进制来源。相比 #5 的 iOS-only 场景，Android 加入只增加"AAR 集成 + Compose UI"，不增加第二套解析器。
- 代价结构：UI 层两套（SwiftUI 一套、Compose 一套），桌面已验证的 live-preview 式 WYSIWYG 要**实现两遍**；行内时间组件两套挂载实现（iOS：`NSTextAttachmentViewProvider`，引 #5；Android：Compose 无系统文本附件 API，需自绘，同 §1 风险面）；平台 SDK 节奏两套（iOS 年度大版本 + Android 年度 API level）。
- 对照单跨平台 UI（CMP/Flutter/RN）：UI 一套，但重度编辑组件要么三方库要么自建（§1/§3），且 iOS 侧编辑深度不及原生；Android 侧 CMP=Jetpack Compose 与双原生等底座，Flutter 则是自绘引擎（Skia→Impeller）。

### 6. 交付模型：账号、分发、通知与"同时 vs 先后"的成本差异事实

**账号与分发**：

- Apple：$99/年（引 #5）；免费个人签名 "Provisioning profiles… will expire 7 days from issuance. You'll need to rebuild and reinstall your app to your device after expiration."，且 10 个 App ID / 3 台设备 / 每设备 3 app 均 7 天过期（[Compare memberships](https://developer.apple.com/support/compare-memberships/)）。
- Google Play：注册费 "There is a US$25 one-time registration fee"（[官方注册指南](https://support.google.com/googleplay/android-developer/answer/6112435)）；2023-11-13 后创建的个人账号 "must run a closed test… with a minimum of 12 testers who have been opted in continuously for at least 14 days" 才能申请 production 访问（[Play 测试要求](https://support.google.com/googleplay/android-developer/answer/14151465)）——**Play 商店路径有冷启动门槛**。
- APK 直装（侧载）是 Android 官方一等流程：系统设置项 `ACTION_MANAGE_UNKNOWN_APP_SOURCES`——"Activity Action: Show settings to allow configuration of trusted external sources"（API 26+，[Settings 参考](https://developer.android.com/reference/android/provider/Settings)）。侧载**没有** iOS 描述文件 7 天过期的等价物：官方建议签名密钥 "should be valid for at least 25 years"，Play 要求密钥有效期超过 2033-10-22，调试证书 30 年（[Sign your app](https://developer.android.com/studio/publish/app-signing)）。
- FCM：定价页逐字 "Cloud Messaging (FCM) — **No-cost**"（[Firebase 定价](https://firebase.google.com/pricing)）。本地提醒不依赖它；它是将来"进程被杀后的补投递/跨端通道"的免费底座。

**Android 本地通知/提醒的约束是"权限型"**（对照 #3：iOS 是"配额/生命周期型"——64 条 pending、7 天重签、BGAppRefresh 预算）：

- 精确闹钟默认拒绝："SCHEDULE_EXACT_ALARM, the permission introduced in Android 12… is no longer being pre-granted to most newly installed apps targeting Android 13 and higher (will be set to denied by default)"；无权限调用 `setExact()` / `setExactAndAllowWhileIdle()` 抛 SecurityException（[Android 14 行为变更](https://developer.android.com/about/versions/14/changes/schedule-exact-alarms)，页面标题即 "Schedule exact alarms are denied by default"）。
- 非精确闹钟的实际窗口："On Android 12 (API level 31) and higher, the system invokes the alarm **within one hour** of the supplied trigger time, unless any battery-saving restrictions"（[Schedule alarms 指南](https://developer.android.com/training/scheduling/alarms)）——对 `~10m` 提前量是数量级偏差。同页官方立场："If your app's core functionality depends on a precisely-timed alarm—such as for an alarm clock app or a calendar app—then it's OK to use an exact alarm instead"；Zakki 是便签不是闹钟，能否正当地申请 SCHEDULE_EXACT_ALARM 特殊访问属产品决策。
- ADR-0001 的兜底策略（应用存活时对账重排、打开即重建）对双端同样适用且必要。

**同时 vs 先后的差值定位**（诚实声明：工程工时没有一手公开数据，以下是可引用的结构性事实）：

- 双原生路线的增量 UI 人力是**两套**；先 iOS 后 Android 时第二套可以复用已验证的编辑器交互设计（"翻译"而非"重新设计"）；Go 内核一次到位双端同源（§5）。
- 单 UI 跨平台路线（CMP/Flutter/RN）UI 一套、天然双端可同发；增量转为**每端平台适配与缺陷分叉的验证**（跨端行为差异案例：CMP #5009 已修、Flutter #190974 Android viewInsets 特例已修）。
- 分发节奏事实：Android 侧载无 7 天重建负担，免费迭代节奏比 iOS 免费签名更顺；走 Play 商店则个人账号有 14 天/12 人测试门槛（§6 引文）。iOS 侧免费签名约束见 #3。

## 对比表

| 维度 | 双原生（SwiftUI + Jetpack Compose）+ gomobile | KMP/CMP 单 UI | Flutter | RN / Expo |
|---|---|---|---|---|
| 双端 Stable 背书 | 平台原生（无背书问题） | KMP 核心 + CMP UI 双双 Stable（iOS 2025-05 起） | 移动双端成熟 | 移动双端成熟 |
| 重度 Markdown 编辑成本 | 最高：两套自建 WYSIWYG；但两底座（TextKit / Compose 文本栈）均为平台主路径，定制上限最高 | 一套 UI；编辑器可用 compose-rich-editor（`setMarkdown`/`toMarkdown`）；上限受 CMP 文本栈约束，iOS 细节仍在打磨（1.12 修选区手柄/上下文菜单） | 一套 UI；super_editor 等活跃，但 open CJK/IME 清单最长（§3） | 一套 UI 但编辑器在 WebView；iOS Fabric CJK 破坏级 open 问题（§4） |
| CJK/IME 成熟度（重编辑证据） | 系统输入法管线原生（EditText / Compose TextField） | CMP Android = Jetpack Compose 同左；组件层已知 IME 问题已修（rich-editor #779/#640） | 引擎层 Android 组词崩溃 open 近三年（#141557）；组件层多个 open CJK 问题 | Android 侧未检索到同级 open CJK 问题；iOS 侧 #56463 显示文本层脆弱 |
| 行内时间组件 | 两套实现；挂载点明确（iOS NSTextAttachmentViewProvider / Compose 自绘） | 一套 Compose 自绘（双端共用）；无系统附件 API | 一套 Flutter 自绘 | WebView 内 DOM 实现 |
| 本地通知/提醒派生 | 每端系统 API；Android 需面对精确闹钟默认拒绝（§6） | Kotlin 层直接调系统 API，同左 | flutter_local_notifications + 同样的 Android 权限面 | expo-notifications（Android Expo Go 已不支持通知，引 #5）+ 同样权限面 |
| 免费构建/分发 | iOS 7 天重签；Android 侧载无重签、Play $25 + 个人账号 14 天/12 人门槛 | 同左 + Xcode/Gradle 工具链 | 同左 + Flutter 工具链 | 同左 + Expo 工具链 |
| 与 zakki.json + 行内令牌贴合 | **优**：gomobile 一套解析，AAR+XCFramework 同包产出 | 优（保留 Go 桥）/ 双实现（换 Kotlin 核心） | 需 gomobile 桥或 Dart 重写（#5 判断不变） | 同 Flutter |
| 双端覆盖成本结构 | UI×2、内核×1 | UI×1、内核×1（Go）或 ×2（Kotlin） | UI×1、内核×1（Go 桥） | UI×1、内核×1（Go 桥） |
| 长期维护面 | 两套 UI、两套 SDK 节奏、一套内核 | 一套 UI；CMP iOS 文本栈持续打磨；KMP 双平台 Stable | 一套 UI；IME 摩擦面长期开放 | 一套 UI；WebView 编辑器 + iOS Fabric CJK open 问题 |

## 对 #5 原排序的修订建议（事实驱动，非决策）

#5 排序：原生 SwiftUI + gomobile > Flutter > RN > webview。Android 加入后：

1. **双原生 + gomobile 仍是"重度编辑 + 模型贴合"的上限路线，排序第一的性质不变**；且 Android 侧比 iOS 侧更无悬念（Compose 即官方主路径，无 young-API 悬念），共享内核对 Android 是增量近零。变化的只是成本项：自建 WYSIWYG 从一套变两套。
2. **CMP 从"未参评"直接进入第二名竞争圈**：双端 Stable + Android 侧就是 Jetpack Compose + 编辑组件自带 Markdown 双向 API + 已知 IME 问题已修复。与 Flutter 的第二名之争不能沿用 #5 结论，应通过"Zakki 方言 Markdown 往返保真 spike"决定。
3. **Flutter 退到与 CMP 同档比较**：编辑器生态仍是三跨平台案中最成熟的，但 IME/composing 缺陷流（含 Android 生产崩溃 open 近三年）在"重度编辑"角色下应比 #5 的 iOS 视角评估给更高权重。
4. **RN 仍居末位**：Android 侧没有出现缓解其结构性风险的事实；iOS Fabric CJK 开放破坏问题反而强化了文本层脆弱性证据；webview 编辑器底座不变。
5. **gomobile 定位不变**：共享内核手段而非 UI 候选；绑定面双端一致、Android 无新增限制；新增 16 KB 页大小这一个 Android 特有验证点（见开放点）。
6. **交付顺序与栈选择强相关**：双原生路线与"先 iOS 后 Android"天然匹配（第二套 UI 复用设计、人力串行化）；单 UI 路线与"双端同发"天然匹配。Android 免费测试路径（侧载，无 7 天重签）比 iOS 免费签名更顺，若做 Android 可先用侧载验证再决定是否进 Play（$25 + 14 天门槛）。

## 不替用户拍板的开放点

- **CMP vs Flutter 的名次应由 spike 数据决定**：compose-rich-editor 的 `setMarkdown`/`toMarkdown` 对 Zakki 方言（`@`/`~` 令牌、表格独立路径、围栏块跳过）的往返保真度未实测；CMP iOS 编辑态能否承载行内时间组件（无系统附件 API，需自绘验证）也未实测。
- **gomobile 产物是否满足 Play 16 KB 页大小要求未验证**（2027-02-01 起不满足则不能发更新）；官方文档明确含 native 库的 app 需重建适配，Go `.so` 的默认对齐情况需实测或查证 Go 工具链文档。
- **Android 精确闹钟的产品策略**：引导用户授予 SCHEDULE_EXACT_ALARM 特殊访问、接受非精确闹钟 ≤1 小时窗口、还是以"打开 app 对账重建"为唯一兜底——直接影响 `~10m` 提前量的兑现方式，属产品决策。
- **双端同时 vs 先 iOS 后 Android**：无一手工时数据；差值集中在"第二套 UI 的工程时间"（双原生）或"双端缺陷分叉的验证时间"（跨平台），属排期与人力决策。
- **内核语言**：若走 CMP/KMP 且共享逻辑改写为 Kotlin，桌面 Go 解析与移动 Kotlin 解析将成为双实现（与 #5 "Swift 重写"同型风险）；保留 Go 则 CMP 路线下三语言并存（Go 内核 + Kotlin UI + ObjC/Swift 桥）。两者需单独拍板。
