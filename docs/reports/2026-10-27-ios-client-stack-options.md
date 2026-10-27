# iOS 客户端技术栈候选事实对比（wayfinder #5）

研究性质：事实采集，不含最终决策。所有星数与最近推送时间取自 GitHub API / pub.dev，查询日期 2026-10-27；
系统 API 能力取自 Apple 开发者文档；库能力取自各库官方 README / 文档 / 源码。每条事实后跟来源。

## 背景与约束

- 已定决策（[wayfinder #1](https://github.com/gomeen/Zakki/issues/1)）：完整 iOS 客户端、iOS 优先、**重度编辑角色**（手机上要舒服地写/改正文，不只是瞄一眼）、成本最小化（无付费 Apple 开发者账号，免费个人 7 天签名可接受；若某路线必须花钱须给出金额）、提醒走本地通知。
- 数据模型：单文件 `zakki.json`（`{version, active, tabs:[{id,name,text,color}], settings}`，tab 内容是原始 Markdown；`internal/notes/file.go`）。时间功能是行内文本令牌（[ADR-0001](../adr/0001-inline-time-tokens.md)）：`@YYYY-MM-DD HH:mm` + `~<提前量>`，可附在任意内容行；解析跳过围栏代码块、行内代码、链接与转义（`internal/markdown/timeinline.go`）。
- 桌面编辑器现状参考：Zakki 桌面自研"实时预览块 + 聚焦进入编辑 + 行内时间组件"的 WYSIWYG（`internal/app/live_*`），说明该交互模式在自建路径上已被验证过一次。
- 成本事实：Apple Developer Program 为 $99/年（页面原文 "$99 annual membership"），含 TestFlight/App Store 分发能力（[Apple Developer Program](https://developer.apple.com/programs/)）。免费路径：Apple 官方文档描述了"注册设备 + Xcode 自动管理签名 + Run"的真机运行流程，未提及需要付费账号（[Distributing your app to registered devices](https://developer.apple.com/documentation/xcode/distributing-your-app-to-registered-devices)）；"免费团队签名的应用 7 天过期"是 wayfinder #1 已记录的既定约束，本次检索未在 Apple 公开文档中找到明文出处，按既定约束对待。

## 候选逐项事实

### 1. SwiftUI / UIKit 原生

系统文本编辑能力：

- `TextEditor(text:)` 是**纯文本**编辑器（官方原话 "Creates a plain text editor"），iOS 14+（[Apple](https://developer.apple.com/documentation/swiftui/texteditor/init(text:))）。
- iOS 26 新增 `TextEditor(text:selection:)`，官方原话 **"Creates a styled text editor"**（[Apple](https://developer.apple.com/documentation/swiftui/texteditor/init(text:selection:))），即以 `AttributedString` 为载体的富文本编辑。第三方印证：RichTextKit 作者在 README 写道 "with the new iOS/macOS 26 releases, we can now edit attributed strings with a `TextEditor`"（[RichTextKit README](https://github.com/danielsaidi/RichTextKit)）。
- Markdown → 富文本方向系统有现成 API：`AttributedString(markdown:options:baseURL:)`，iOS 15+（[Apple](https://developer.apple.com/documentation/foundation/attributedstring/init(markdown:options:baseurl:))）；**富文本 → Markdown 的回写方向系统未提供**，需自行序列化。
- 深度定制底座是 `UITextView` + TextKit：`attributedText` 属性（"The styled text that the text view displays"，[Apple](https://developer.apple.com/documentation/uikit/uitextview/attributedtext)）；`NSTextAttachmentViewProvider`（iOS 15+）可将**视图对象**附着到文档指定位置（[Apple](https://developer.apple.com/documentation/uikit/nstextattachmentviewprovider)）——这是行内时间组件（桌面端已有同构交互）的原生挂载点。
- 本地通知：`UNUserNotificationCenter`（"central object for managing notification-related activities"）+ `UNCalendarNotificationTrigger`（"the system delivers [the notification] at a specific date and time"），iOS 10+（[Apple 1](https://developer.apple.com/documentation/usernotifications/unusernotificationcenter)、[Apple 2](https://developer.apple.com/documentation/usernotifications/uncalendarnotificationtrigger)）。本地通知不需要推送类 entitlement，与免费个人签名兼容。
- 系统层缺口：iOS 26 styled TextEditor 文档中**没有**自定义视图/组件嵌入 API（对比 `NSTextAttachmentViewProvider` 只有 UIKit 路径有）——所见即所得里的行内时间组件在纯 SwiftUI 新 API 下如何落，文档未给答案。

现成组件（GitHub API 2026-10-27）：

| 组件 | 定位 | 星数/最近推送 | 维护事实 |
|---|---|---|---|
| [RichTextKit](https://github.com/danielsaidi/RichTextKit) | UITextView 桥接的 SwiftUI 富文本编辑 | 1275★，2026-06 推送 | 活跃；作者自述 iOS 26 后"not yet sure if I will keep this library alive" |
| [Proton](https://github.com/rajdeep/proton) | UITextView 扩展的富文本编辑框架（Apache-2.0） | 1382★，2025-04 推送 | README 自述 1.0.0 之前 API 可能有破坏性变更；`TextProcessor` 官方示例即"边输入边按 markup 语法转换" |
| [Runestone](https://github.com/simonbs/Runestone) | 源码模式候选：iOS 高性能纯文本编辑 + 语法高亮 | 3232★，2026-10 推送 | 活跃 |
| [MarkdownUI](https://github.com/gonzalezreal/swift-markdown-ui) | Markdown **渲染**（非编辑） | 3934★，已进维护模式 | README 标注继任者 [textual](https://github.com/gonzalezreal/textual)（"Render and customize rich attributed text"，仍是渲染） |
| [Down](https://github.com/johnxnguyen/Down) | cmark 渲染 | 2535★ | 2023-07 后停更 |
| [CocoaMarkdown](https://github.com/indragiek/CocoaMarkdown) | 解析/渲染 | 1234★ | 2019 年停更 |
| [Aztec](https://github.com/wordpress-mobile/AztecEditor-iOS) | HTML 管线的富文本编辑 | 665★，2026-10 推送 | 仍活跃但 HTML 不是 Markdown |

小结：iOS 上**不存在**"以 Markdown 为数据模型的现成 WYSIWYG 编辑组件"；所见即所得需自建（参考桌面 live-preview 路径）或在通用富文本框架（Proton/RichTextKit）上加 Markdown 序列化层。免费签名路径零额外工具链（Xcode 自动签名）。

### 2. Flutter

编辑器生态（GitHub API / pub.dev 2026-10-27）：

- [flutter-quill](https://github.com/singerdmx/flutter-quill)：2916★，2026-09 推送，生态里星数最高的 Flutter 富文本编辑器。内部模型是 Quill **Delta**；官方 README 明确建议"storing the Document as **Delta JSON** instead of other formats (e.g., HTML, Markdown…)"，Markdown 往返靠第三方包 [`markdown_quill`](https://pub.dev/packages/markdown_quill)。对"Markdown 是唯一事实源"的应用是**模型错配**：数据要经 Markdown↔Delta 双向转换。
- [super_editor](https://github.com/Flutter-Bounty-Hunters/super_editor)：1937★，2026-10 推送，文档模型编辑器工具包；官方伴生包 [`super_editor_markdown`](https://pub.dev/packages/super_editor_markdown)（pub.dev 描述 "Markdown (de)serialization for super_editor documents"）——三家中与 Markdown 事实源**贴合度最好**。README 自述 mono-repo 多包需整组同步升级。
- [appflowy-editor](https://github.com/AppFlowy-IO/appflowy-editor)：687★，2026-08 推送；官方文档支持从 Markdown / Quill Delta 导入（[importing.md](https://github.com/AppFlowy-IO/appflowy-editor/blob/main/documentation/importing.md)），源码含 Markdown 编码器插件（`lib/src/plugins/markdown/document_markdown.dart` 及测试），双向可走。
- 渲染类：`flutter_markdown` 已被官方** discontinued**（[pub.dev](https://pub.dev/packages/flutter_markdown) 页面标注 "discontinued" 与 "replaced by"，且本就只渲染不编辑）；社区 [markdown_widget](https://github.com/asjqkkkk/markdown_widget) 460★ 活跃，同为渲染。
- 结论：编辑器组件生态**成熟且活跃**；核心风险全部集中在"编辑器内部模型 ↔ 原始 Markdown"的保真往返（super_editor 的 serde 是否覆盖 Zakki 的令牌方言需实测）。

本地通知与构建：

- [flutter_local_notifications](https://github.com/MaikuB/flutter_local_notifications)：2670★，2026-10 推送，支持 iOS 本地通知（文档含权限申请与 iOS pending notifications limit 章节）。
- 构建走标准 Xcode 签名；官方文档明确 "To publish your app to the App Store, you must first enroll in the Apple Developer Program"（[Flutter iOS deployment](https://docs.flutter.dev/deployment/ios)），真机开发签名在其 Xcode Team 配置流程内，免费个人团队可用；额外代价是引入 Flutter 工具链与产物体积。

### 3. React Native / Expo

编辑器生态（GitHub API 2026-10-27）：

- [10tap-editor](https://github.com/10play/10tap-editor)：1194★，2026-10 推送，当前 RN 生态最活跃的富文本编辑器；README 自述 "based on Tiptap and Prosemirror"，安装依赖 `react-native-webview`——**编辑器实际运行在 WKWebView 内**；原生键盘驱动模块在 dev client 可用，在 Expo Go 里退化为 JavaScript fallback（README "Expo Go: the editor works, and custom keyboards use a JavaScript fallback"）。
- 其余均为 WebView/HTML 系且更弱：[wxik/react-native-rich-editor](https://github.com/wxik/react-native-rich-editor) 945★（2025-07 推送，自述 "JavaScript, H5"）、[react-native-cn-quill](https://github.com/imnapo/react-native-cn-quill) 220★（2025-02，Quill.js WebView 包装）；Wix 系编辑器 2023 年起停更。
- 与 Flutter 同类风险：Tiptap/ProseMirror 的文档模型（HTML/JSON）↔ Markdown 需转换层；且重度编辑手感继承 §4 的 webview 风险。

本地通知与构建：

- [expo-notifications](https://docs.expo.dev/versions/latest/sdk/notifications/)：官方功能含 "Schedule a one-off notification for a specific date" 与 iOS 日历匹配的重复调度；可在既有 RN 工程安装。Expo Go 支持面变化：SDK 53 起 "**notifications are no longer supported in Expo Go for Android**"；iOS 的 Expo Go 仍支持（官方原话 "We still support push notifications in Expo Go for iOS…"）（[SDK 53 changelog](https://expo.dev/changelog/sdk-53)）。
- 备选：[Wix react-native-notifications](https://github.com/Wix/react-native-notifications) 3341★，2026-10 推送，仍活跃；[notifee](https://github.com/invertase/notifee) 2173★ 已**归档**（2026-04）。
- 构建：`expo run:ios` 本地用 Xcode 编译（官方 "Build locally: Compile with Android Studio and Xcode using Expo CLI"，[docs](https://docs.expo.dev/develop/development-builds/create-a-build/)），免费个人签名可行；云构建（EAS）与 App Store/TestFlight 侧受付费账号约束（[Apple $99](https://developer.apple.com/programs/)）。

### 4. Tauri 2 mobile / Capacitor（webview 系）

- Tauri 2.0（2024-10）stable 支持 iOS/Android；前端是"编译到 HTML/JavaScript/CSS 的任何前端框架"，跑在系统 webview 里，原生层经 Swift/Kotlin 暴露（[Tauri 2.0 发布公告](https://tauri.app/blog/tauri-20/)）。本地通知有官方插件（[tauri-plugin-notification](https://v2.tauri.app/plugin/notification/)）。
- 重度文本编辑 = WKWebView 里的 contenteditable，其手感由上游 JS 编辑器（ProseMirror/CodeMirror/Tiptap）决定。上游的 iOS 问题史是可查的事实：ProseMirror [#543](https://github.com/ProseMirror/prosemirror/issues/543)（composition/IME 更新粒度，51 评论）、[#982](https://github.com/ProseMirror/prosemirror/issues/982)（open，iOS 联想词延续到下一行）、[#760](https://github.com/ProseMirror/prosemirror/issues/760)（iOS 剪切失效）；CodeMirror（[codemirror/dev](https://github.com/codemirror/dev)）[#379](https://github.com/codemirror/dev/issues/379)（iOS 日文输入）、[#1538](https://github.com/codemirror/dev/issues/1538)（iOS 选区手柄消失）、[#482](https://github.com/codemirror/dev/issues/482)（回车卡住）、[#797](https://github.com/codemirror/dev/issues/797)（IME 失焦）、[#1020](https://github.com/codemirror/dev/issues/1020)（iOS 不支持自动更正开关）。多数已修复——证明可修，也证明是**持续摩擦面**。
- Tauri 自身移动端键盘问题有 open bug：[#9907](https://github.com/tauri-apps/tauri/issues/9907)（"IOS pop-up keyboard causes webview to move outside the screen"）、[#10631](https://github.com/tauri-apps/tauri/issues/10631)（visualViewport 无法反映键盘高度）。
- [Capacitor](https://github.com/ionic-team/capacitor)（16.9k★，活跃）运行时更成熟：官方 [Keyboard API](https://capacitorjs.com/docs/apis/keyboard)（键盘显示/隐藏事件与控制）与 [Local Notifications API](https://capacitorjs.com/docs/apis/local-notifications)（"schedule device notifications locally (i.e. without a server sending push notifications)"）。大量键盘布局 issue 已关闭（如 [#8329](https://github.com/ionic-team/capacitor/issues/8329)）。但文本编辑底座与 Tauri 相同（WKWebView + contenteditable），选区/IME 定制深度到不了原生 UITextView 层。

### 5. gomobile / Go 共享内核

- `gomobile bind -target ios` 生成 ObjC 绑定并编译库；Apple 目标必须在装有 Xcode 的 macOS 上运行（[gomobile 文档](https://pkg.go.dev/golang.org/x/mobile/cmd/gomobile)）。
- **绑定面受限**（[gobind 类型限制](https://pkg.go.dev/golang.org/x/mobile/cmd/gobind)）：仅支持有符号/无符号整型、浮点、string、bool、[]byte（引用传递）、参数与返回值全为受支持类型的函数、导出方法受限的 interface/struct；"Exceptions and panics are not yet supported. If either pass a language boundary, the program will exit."。→ 合理用法是**窄接口**：传入 `zakki.json` 字节，返回解析/提醒派生结果（string/[]byte），Swift 侧不持有 Go 对象。
- Zakki 的解析代码在 `internal/`（`internal/markdown`、`internal/notes`）。Go 的 internal 包**不能被外部模块 import**（[Go 1.4 release notes](https://go.dev/doc/go1.4)）；可行做法是在 Zakki 仓库内加一个非 internal 的薄包装包（同模块内合法），gomobile bind 该包装包。
- Zakki 全部逻辑包为纯 Go（Makefile 测试用 `CGO_ENABLED=0`，见 `Makefile`），无 cgo 依赖，bind 无额外障碍；Zakki 自身不写 cgo。"cgo 约束"实际体现为：Apple 交叉编译由 gomobile 工具链处理，代码侧只需守 gobind 类型限制。
- x/mobile 官方定位（[README](https://github.com/golang/mobile)）："The Go Mobile project is **experimental**. Use this at your own risk. … neither Google nor the Go team can provide end-user support."。仓库仍有提交（2026-10 有推送），但属工具链风险项。
- 收益：`zakki.json` 读写、行内令牌解析、提醒派生规则与桌面**同一实现**（解析单一事实源，避免 Swift 重写第二套解析器）；ADR-0001 的"文本即数据"在两端由同一代码保证。代价：iOS 构建链多一个 xcframework 产物、gomobile 工具版本维护、跨语言调试。
- 关键定位：gomobile **不是独立候选栈**（没有 UI），是与 1/2/3 组合使用的共享内核；webview 系（4）也能经桥复用，但那两案的主要风险不在解析而在编辑器本身。

## 对比表

| 维度 | SwiftUI/UIKit 原生 | Flutter | RN / Expo | Tauri 2 / Capacitor | gomobile 共享内核 |
|---|---|---|---|---|---|
| 重度 Markdown 编辑（WYSIWYG+源码）实现成本 | 高：无现成 markdown-WYSIWYG 组件，需自建/改造；但 UITextView/TextKit 定制深度最高，桌面 live-preview 模式可平移 | 中低：super_editor / appflowy-editor 现成且活跃、带 Markdown serde；风险在模型↔Markdown 往返保真 | 中：10tap 现成活跃，但编辑器跑在 WKWebView，Tiptap 模型需转换层 | 中低（JS 编辑器现成）但重度编辑手感风险最高（见上） | 不适用（无 UI） |
| 行内时间组件挂载 | 原生挂载点存在：`NSTextAttachmentViewProvider`（iOS 15+）；iOS 26 styled TextEditor 未见自定义视图 API | 编辑器自定义块/内嵌 widget 可做 | 10tap 自定义节点，渲染仍经 webview | web 组件（webview 内） | 不适用 |
| 本地通知 | `UNUserNotificationCenter` + `UNCalendarNotificationTrigger` 直接可用（iOS 10+），无 entitlement 负担 | `flutter_local_notifications` 活跃（2670★） | `expo-notifications` / Wix 可用；notifee 已归档 | 双方均有官方插件 | 无关 |
| 免费签名构建/分发体验 | 最好：零额外工具链，Xcode 自动签名+设备注册即真机运行 | 同 Xcode 路径，另需维护 Flutter 工具链 | `expo run:ios` 本地可行；EAS 云构建与商店分发受付费约束 | 最终仍走 Xcode；Tauri mobile 工具链较新 | 增加一步 xcframework 生成 |
| 与 zakki.json + 行内令牌模型贴合度 | 优：纯文本进出；解析可复用 Go 或 Swift 实现 | 良（super_editor）/差（flutter-quill 需 Delta 转换） | 中：HTML/JSON 模型↔Markdown 转换层 | 中：同 RN | **优**：解析与桌面单一事实源 |
| 长期维护面 | 系统 API 稳定；iOS 26 styled TextEditor 很年轻；社区组件有单点风险（RichTextKit 去留未定、Proton pre-1.0） | 三个编辑器组件皆活跃但体量大；Flutter 自身大版本跟随 | RN+Expo 升级链最长；10tap 单仓库维护 | Tauri mobile 年轻且有 open 键盘 bug；Capacitor 运行时成熟但编辑上限在 webview | x/mobile 官方自述 experimental、不提供支持 |

## 排序与理由（事实驱动，非决策）

1. **SwiftUI/UIKit 原生 + gomobile 共享核心**。免费签名与本地通知零摩擦（§1）；行内时间组件有原生挂载点；解析单一事实源经 gomobile 可达。主要代价是编辑器自建——但这是把桌面已验证的 live-preview 交互平移到定制深度最高的平台，且 iOS 26 styled TextEditor 的出现方向与 Zakki 的富文本需求一致（尽管年轻）。
2. **Flutter（super_editor 或 appflowy-editor）+ gomobile**。编辑器生态三案中最成熟、最活跃且唯一原生带 Markdown serde 的路线；代价是文档模型↔Markdown 往返保真需实测、Flutter 工具链长期跟随。flutter-quill 官方立场（存 Delta JSON）与本项目模型直接冲突，若走 Flutter 应排除。
3. **React Native / Expo（10tap）**。编辑器与通知生态可用且活跃；但最活跃的编辑器本质是 WKWebView 里的 ProseMirror，把 webview 系的编辑手感风险引入"重度编辑"这一已定角色，升级链也最长。
4. **Tauri 2 mobile / Capacitor**。构建与插件面可用（Capacitor 运行时成熟），但两项 open 事实直接命中本项目核心场景：WKWebView 重度编辑的键盘/选区/IME 长期摩擦史 + Tauri mobile 键盘 bug 仍开。适合"轻阅读轻编辑"，与已定的"重度编辑"错位。
5. **gomobile 单列为"非候选的候选"**：它是解析单一事实源的手段而非 UI 栈；绑定面窄但够用（bytes in/out），官方 experimental 定位与"不提供支持"是主要风险，建议若采用则以极窄接口封装并接受 xcframework 工具链维护。

## 不替用户拍板的开放点

- iOS 26 styled `TextEditor` 能否承载 Zakki 式行内时间组件：Apple 文档未见 SwiftUI 侧的自定义视图嵌入 API（对比 UIKit 的 `NSTextAttachmentViewProvider`），需要 spike 实测；若不能，WYSIWYG 路线须直接落在 UITextView 上。
- super_editor / appflowy-editor 的 Markdown serde 对 Zakki 方言（`@`/`~` 令牌、无删除线、表格独立路径）的往返保真度：未验证，需往返测试再比较第 1、2 名。
- 解析核心走 gomobile（单一事实源、experimental 工具链）还是 Swift 重写（工具链简单、两套解析器需对齐测试）：两侧风险不同型，值得单独决策。
- 免费签名 7 天重签的日常成本对所有栈相同，但影响 MVP 分发与测试节奏，属运维决策非技术栈决策。
- 若未来取消 "Android out of scope"（wayfinder #1 现明确排除），Flutter/RN 的排序会上升；本排序仅基于 iOS 优先的现状。
- Tauri/Capacitor 排名基于"重度编辑"前提；若该前提改变（例如 iOS 端只做查看+微调），应重新评估。
