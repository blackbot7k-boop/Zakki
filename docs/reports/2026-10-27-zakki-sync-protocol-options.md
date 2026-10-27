# zakki.json 同步协议候选研究（wayfinder #4）

对应 [工单 #4](https://github.com/gomeen/Zakki/issues/4)，父图 [#1](https://github.com/gomeen/Zakki/issues/1)。
问题：单文件 `zakki.json` 如何可靠到达 iPhone 并双向写回？约束：尽量不花钱、Windows 与 macOS 桌面共存、单人使用。
本文只做事实梳理与排序，不替用户拍板。除 Zakki 仓库文档外，全部断言给出一手来源链接；个别无法固定一手来源的事实单独标注。

## 背景与约束

- Zakki 桌面是纯 Go + MyGo 应用，数据是**一个 JSON 文件** `zakki.json`（含全部标签，`version: 3`），编辑停顿默认 2 秒落盘；应用自带「迁址」功能，可把文件移动到任意新文件夹。[architecture.md](../architecture.md)、[configuration.md](../configuration.md)
- 桌面已有可选 Git 自动提交/推送：只提交这一个文件，默认关，**不做 pull/rebase/force push**，定位是云备份而非同步。[configuration.md](../configuration.md)
- 提醒令牌是行内纯文本：`@YYYY-MM-DD HH:mm` 时刻 + `~<提前量>`（如 `~10m`），「文本即数据」，天然可 Git diff，不依赖任何结构化字段。[ADR-0001](../adr/0001-inline-time-tokens.md)
- 已定决策：完整 iOS 客户端（浏览 + 重度编辑）、iOS 优先、成本尽量为零（免费 Apple 个人签名，无付费服务；若必须花钱须给出具体金额交用户拍板）。[#1](https://github.com/gomeen/Zakki/issues/1)
- 平台共性事实（影响所有候选）：
  - **iOS 上无法 shell out 到 git**：Foundation 的 `Process` 类平台可用性仅 macOS 与 Mac Catalyst，不含 iOS（[developer.apple.com/documentation/foundation/process](https://developer.apple.com/documentation/foundation/process)）。
  - **iOS 后台由系统机会主义调度**：BGTask 文档原文 "the system executes BGTask subclasses on behalf of your app, while your app is in the background"（[developer.apple.com/documentation/backgroundtasks/bgtask](https://developer.apple.com/documentation/backgroundtasks/bgtask)）；Syncthing 官方 FAQ 同样指出 "iOS has significant restrictions on background processing that make it very hard to run Syncthing reliably"（[docs.syncthing.net/users/faq.html](https://docs.syncthing.net/users/faq.html)）。结论：**任何候选的"双向实时"都应按「前台/聚焦时同步」设计**，后台持续同步不可依赖。
  - 免费个人团队签名：App 需每 7 天用 Xcode 重签后才能继续在设备上运行。官方帮助页在本环境为 JS 渲染、未能固定引用；#1 已把「7 天重签的日常管理」列为已知成本，实施前以 Xcode 实测为准。

## 候选逐项事实（带来源）

### A. Git 私有仓库

**A0. 宿主成本**：GitHub Free 为 $0/月，"Unlimited public/private repositories"（[github.com/pricing](https://github.com/pricing)）。

**A1. iOS 端 Git 库现状（libgit2 路线）**

| 项目 | 状态 | 来源 |
|---|---|---|
| libgit2（C 库） | 活跃：最新 release v1.9.7，仓库 2026-10-16 仍有提交，约 10.6k star | [api.github.com/repos/libgit2/libgit2](https://api.github.com/repos/libgit2/libgit2) 及 releases |
| SwiftGit2（Swift 封装） | 未归档、MIT、705 star；但最后一次正式 release **0.6.0 停在 2019-05**，近两年提交均为构建/CI 修复（2025-04 "Fix build with XCode 16"、2025-09 CI bump），44 个 open issue | [api.github.com/repos/SwiftGit2/SwiftGit2](https://api.github.com/repos/SwiftGit2/SwiftGit2) |
| ObjectiveGit | 最后提交 2023-09，实质停更 | [api.github.com/repos/libgit2/objective-git](https://api.github.com/repos/libgit2/objective-git) |

解读：libgit2 本体健康，但 iOS 侧的现成 Swift 绑定处于"能用但维护薄"状态；接入 libgit2 需要自己扛构建链与 API 演进。工程量：clone/fetch/commit/push 全套 + 二进制依赖管理，在六条路线里最重。

**A2. 不引 Git 库的变体：GitHub REST Contents API（研究认为值得单独评估）**

- 「Create or update file contents」端点用 Base64 提交整个文件内容；更新时必须携带 `sha`（原文："sha — Required if you are updating a file. The blob SHA of the file being replaced."），即**天然的 Compare-and-Swap：远端已变时旧 `sha` 会被拒绝**；文档同时明确并行 create/delete 会冲突报错、需串行调用；fine-grained token 需 Contents 写权限。[docs.github.com/en/rest/repos/contents?apiVersion=2022-11-28](https://docs.github.com/en/rest/repos/contents?apiVersion=2022-11-28)
- 语义：冲突表现为显式拒绝（HTTP 4xx），客户端重新 GET、与本地合并后重试。对"单 JSON 文件、单人双端"这是最干净的并发模型——**不做自动合并**，冲突永远显式交给应用层。
- iOS 端只需 URLSession + JSON，零第三方依赖；桌面端已有 git 提交/推送可继续复用同一仓库。
- 交互风险：桌面 `git.autoPush` 不做 pull（[configuration.md](../configuration.md)），若 iOS 频繁经 REST 产生新提交，桌面 push 会因 non-fast-forward 失败——选此路线则桌面 Git 备份要么改为也走 REST、要么补 `pull --ff-only`，属于 #1 声明的"改造范畴届时另议"。

**A3. 外包给 Working Copy（App）**：免费下载，但"unlock pro features such as the ability to push commits and manage more than 5 repositories"（push 与 >5 仓库需内购解锁）；支持 Shortcuts 与 x-callback-url 自动化；更新至 2026-10-19，维护活跃（[App Store/iTunes API，id 896694807](https://apps.apple.com/us/app/working-copy-git-client/id896694807)、[workingcopy.app](https://workingcopy.app/)）。缺点： Zakki 变成第三方 app 的"附属"，自动化依赖 URL 回跳，体验割裂且解锁需花钱（App Store 内显示，非本报告固定）。

**A4. 合并与提醒令牌**：令牌是单行内纯文本（[ADR-0001](../adr/0001-inline-time-tokens.md)），任何"按行"的合并在不拆行时都不会破坏单个令牌；但 JSON 是整文件结构（tabs 数组增删、`active` 指针、settings），两端并发改动的行级 git merge 极易产生语义损坏的 JSON。**研究结论：Git 路线（无论 A1/A2）都应禁用自动合并，用 CAS 拒绝 + 应用层重放（以"重读最新文件 + 用户选择保留哪份"收场）**。这样令牌不存在"被合并破坏"的问题，因为合并不发生。

### B. iCloud

**B1. CloudKit**

- 免费 5GB iCloud 配额：iCloud "with 5 GB of storage for free"（[support.apple.com/en-us/108047](https://support.apple.com/en-us/108047)）。
- 开发者侧免存储费：公共数据 "up to 1PB of storage for your app's public data"；私有数据存在**用户自己的 iCloud 账户**里（[developer.apple.com/icloud/cloudkit/](https://developer.apple.com/icloud/cloudkit/)）。
- 数据模型：public / private / shared 三库；private 读写必须有 iCloud 账户，public 无账户只能读（[developer.apple.com/documentation/cloudkit/ckdatabase](https://developer.apple.com/documentation/cloudkit/ckdatabase)）。CloudKit 存的是 **CKRecord 记录**而非文件——单文件 JSON 要用，就得把 zakki.json 的内容拆成记录建模，或整包塞进一个 CKAsset，两法都与"文本即数据"的本地模型有落差。
- 免费个人团队可用性：capability 清单中 "iCloud: CloudKit" 与 "Push notifications" 是两个独立能力（[developer.apple.com/help/account/reference/supported-capabilities-ios](https://developer.apple.com/help/account/reference/supported-capabilities-ios)）；但**免费签名下能否使用 CloudKit 推送唤醒，官方一手文档在本环境未能固定引用**——若推送不可用，CloudKit 订阅只剩"打开 app 时拉取"，与文件方案的实时性无差。实施前须 Xcode 实测。
- 判断：单文件 + 单用户场景下 CloudKit 的记录模型是错配，工程量高于收益。

**B2. iCloud Drive（ubiquity container / CloudDocs）**

- iOS 侧冲突语义有明确 API：NSFileVersion 文档原文——"two different computers attempt to save the file to the cloud at the same time. In that case, one file is chosen as the current version and any other versions are tagged as being in conflict"（[developer.apple.com/documentation/foundation/nsfileversion](https://developer.apple.com/documentation/foundation/nsfileversion)）。即**系统不丢数据、保留冲突版本、由 app（或用户）解决**，这是所有候选里唯一有系统级冲突 API 的。
- Windows 侧现实路径是 iCloud for Windows：官方指南要求先在 iPhone/iPad/Mac 上设置 iCloud，然后在文件资源管理器里访问 iCloud Drive，含 "Keep files downloaded"、"View download and sharing status" 等按需下载语义（[support.apple.com/guide/icloud-windows/set-up-icloud-for-windows-icwee190673a](https://support.apple.com/guide/icloud-windows/set-up-icloud-for-windows-icwee190673a/icloud)）。**整份指南没有出现 "conflict" 字样**（本次检索未命中）：并发编辑时 Windows 端如何改名/合并，Apple 无公开规范，只能实测。同步质量（延迟、丢事件、按需下载）同样无 SLA。
- 结构性限制：iOS app 的 ubiquity container 是 app 专属目录，iCloud for Windows 只同步用户的 iCloud Drive（CloudDocs），**不同步第三方 app 的 container**；反过来，桌面直接写 CloudDocs 时，iOS app 要无交互读写该文件需 document picker 授权 + security-scoped bookmark（系统机制，见 [Apple Document Picker 文档](https://developer.apple.com/documentation/uikit/view_controllers/displaying_and_managing_documents_with_the_document_browser)）。两条子路径各有一层摩擦。
- 桌面端改动量：≈0——Zakki 已支持迁址，把 `zakki.json` 放进 iCloud Drive 目录即可（macOS 原生、Windows 装 iCloud for Windows）。
- 免费签名的影响（7 天重签）对所有路线相同；重签不丢 iCloud 数据。

### C. WebDAV（坚果云为主）

- 免费版额度：**上传 1 GB/月、下载 3 GB/月**，历史版本保留 1 个月（[jianguoyun.com/s/pricing](https://www.jianguoyun.com/s/pricing)）。zakki.json 为 KB 级文本，额度余量数个量级；1 个月文件历史是免费版自带的兜底回滚。
- WebDAV 接入：官方要求生成"第三方应用密码"；**免费版频率限制每 30 分钟 600 次请求**（付费 1500 次），单次列目录 750 项需分页，单文件上传上限 500 MB（[help.jianguoyun.com/?p=2064](https://help.jianguoyun.com/?p=2064)）。同步-on-open 模式每轮只需个位数请求，额度与频率都远够。
- 协议本体：RFC 4918 —— "a set of methods, headers, and content-types ancillary to HTTP/1.1 ... and resource locking (collision avoidance)"（[rfc-editor.org/rfc/rfc4918](https://www.rfc-editor.org/rfc/rfc4918.html)）。即 iOS 用 URLSession、桌面用 Go 标准库即可自实现最小动词集（PROPFIND/GET/PUT，可选 LOCK）；GitHub 搜索未见主流维护的 Swift WebDAV 客户端库（前列均为小体量/新项目），自实现反而是常态。
- 并发控制短板：坚果云是否支持 ETag/If-Match 未见于公开文档（本次检索未命中）——乐观并发需要自己拿"修改时间/大小/自定义修订"做写前校验，弱于 GitHub `sha` 的显式 CAS；好在免费版 1 个月历史版本提供了最坏情况的兜底。
- 桌面改动：在「保存与同步」框架内新增一个 WebDAV 同步模式（纯 HTTP，无新依赖）；与现有 Git 备份互不干扰。

### D. 自建微型同步服务

- 成本锚点（均为官方页面，价格随时变动，下单前以页面为准）：
  - Oracle Cloud Free Tier：$300/30 天试用 + **Always Free 服务无限期**（含 Arm Ampere A1 Compute）（[oracle.com/cloud/free](https://www.oracle.com/cloud/free/)）——真 ¥0，但账号风控与回收条款以官方 FAQ 为准。
  - 腾讯云轻量应用服务器：新用户 4核4G3M **首年 ¥109/年**；常规 2核2G 约 ¥45/月 / ¥459/年（[cloud.tencent.com/product/lighthouse](https://cloud.tencent.com/product/lighthouse)）。
  - Hetzner：Cost-Optimized 自 **€5.99/月** 起（不含 VAT）（[hetzner.com/cloud](https://www.hetzner.com/cloud/)）。
- 必须自己实现的面：单用户鉴权（token）、HTTPS 传输、并发控制（版本号/ETag 的 CAS）、冲突保留策略、（可选）推送。即一条"私有小 GitHub Contents API"。
- 持续维护面：系统安全更新、证书续期、服务可用性；境外 VPS 在大陆的可达性波动属用户环境风险，本次不作为一手结论。
- 判断：完全可控、延迟可做到最好，但工程量与长期维护在所有路线中最大；单人单文件场景下收益/代价比不高。

### E. Syncthing 与同类

- 官方立场：**"There are no plans by the current Syncthing team to officially support iOS in the foreseeable future"**，并直接归因于 iOS 后台限制（[docs.syncthing.net/users/faq.html](https://docs.syncthing.net/users/faq.html)）。
- iOS 现状两选项：
  - **Synctrain**（开源，pixelspark/sushitrain）：免费（[App Store](https://apps.apple.com/us/app/synctrain/id6553985316)，iTunes API 显示 Free、更新 2026-08-17）；仓库活跃：2116 star，2026-10-26 仍有提交，最新 release v2.7.83（2026-08-17）（[api.github.com/repos/pixelspark/sushitrain](https://github.com/pixelspark/sushitrain)）。
  - **Möbius Sync**（商业）：免费 + 内购——"Usage within the Möbius Sync sandbox is free up to 20MB. Unlimited file sync ... is available for a one-time purchase"；"访问其他 app 沙箱"是**付费且标注 EXPERIMENTAL** 的功能；App Store 更新停在 2025-01-27，评分 3.8（140 评分）（[App Store](https://apps.apple.com/us/app/mobius-sync/id1539203216)、[mobiussync.com](https://www.mobiussync.com)）。
- 冲突行为有明确规范：两台设备同时修改且内容不同时，**修改时间较旧的一方被改名为 `<filename>.sync-conflict-<date>-<time>-<modifiedBy>.<ext>`**，冲突副本此后作为普通文件继续传播（[docs.syncthing.net/users/syncing.html](https://docs.syncthing.net/users/syncing.html) "Conflicting Changes"）。
- 对本场景的结构性缺陷：Zakki iOS 无法直接读写另一个 app（Synctrain/Möbius）沙箱里的文件，需 Files app 授权/security-scoped bookmark 间接访问；`.sync-conflict` 改名对"一个被 app 持续读写的 JSON 文件"是坏模式——app 不知道冲突文件的存在，除非专门适配。桌面两端装 Syncthing 倒是零成本（官方二进制）。

### F. 其他有一手来源的选项

- **Dropbox Basic**：免费 2 GB，"For 1 person"、"Connect up to 3 devices"——Windows + macOS + iPhone 恰好 3 台，**踩线**（[dropbox.com/en/basic](https://www.dropbox.com/en/basic)）。冲突副本（conflicted copy）帮助文档本次未能定位（404），冲突语义未固定；大陆可达性属用户环境风险。iOS 集成需 Files app 授权或官方 SDK，均有一层封装。
- InfiniCLOUD 等其他免费 WebDAV：官方文档未能在本环境抓取，未验证，不展开。

## 对比表

| 候选 | 金钱成本 | 双向实时感 | 单 JSON 并发写冲突 | iOS 后台下表现 | 工程量（含 Windows 端） |
|---|---|---|---|---|---|
| **C. WebDAV 坚果云** | ¥0（免费版 1GB/3GB 每月，KB 级文件富余） | 打开/聚焦时拉推，秒级（坚果云国内可达） | 无文档化 ETag → 自设修订校验，弱 CAS；免费版 1 个月历史版本兜底 | 前台同步，符合 iOS 限制 | 两端各一个小同步模块，纯标准库；Windows 端无额外软件 |
| **A2. Git 仓库 + Contents API** | ¥0（GitHub Free 私有库无限） | 打开时拉推；GitHub 大陆可达性波动（用户环境风险） | `sha` 显式 CAS，冲突=显式拒绝，最干净；不做自动合并 | 同上 | iOS 纯 HTTP/JSON；**桌面 git.autoPush 需改造**（加 pull 或改走 REST） |
| **A1. Git 仓库 + libgit2/SwiftGit2** | ¥0 | 同上 | 同上（但需自己实现 fetch/commit/push 循环） | 同上 | 最重：二进制依赖 + SwiftGit2 维护薄（release 停 2019） |
| **B2. iCloud Drive（CloudDocs 路径）** | ¥0（5GB 免费配额） | 系统后台代同步，app 不在前台也推进；体感最好 | 系统保留冲突版本，NSFileVersion 显式 API；**Windows 端冲突行为无公开规范** | 系统服务同步，受 iOS 限制最小 | 桌面 ≈0（迁址即可）；iOS 需 document picker + bookmark；Windows 装 iCloud for Windows，质量须实测 |
| **B1. CloudKit** | ¥0（公共库 1PB、私有库记用户配额） | 若推送可用则接近实时；免费签名下推送可用性未验证 | 记录级（需把单文件建模为记录）或整包 CKAsset，冲突语义自建 | 订阅+推送可后台唤醒（若推送可用） | 大：数据模型重构，与"文本即数据"错配 |
| **E. Syncthing（Synctrain 免费 / Möbius）** | ¥0（Synctrain）；Möbius 解锁为一次性买断 | 设备同在线时快；iOS 端只在前台 | 规范明确但形式是 `.sync-conflict-` 改名副本，对单文件 app 不友好 | 官方明言 iOS 后台限制下难可靠 | iOS 靠第三方 app + 沙箱授权，Zakki 无法直接读写；桌面零成本 |
| **D. 自建微服务** | ¥0（Oracle Always Free）或 ¥109/年起（腾讯轻量首年） | 可做到最好的实时（自定协议 + 可选推送） | 完全自定义（版本号 CAS） | 可选推送（受签名限制约束） | 最大：服务端 + 鉴权 + 双端协议 + 长期运维 |
| **F. Dropbox Basic** | ¥0（2GB） | 打开时同步 | 冲突副本文档未能定位，语义未固定 | Files app/SDK 路径，前台同步 | 中；**3 设备限制恰好踩线**；大陆可达性存疑 |

## 排序与理由

在「尽量不花钱 + Windows/macOS 共存 + 单文件 + 单人 + iOS 前台同步即可」的约束下：

1. **WebDAV（坚果云免费版）**——额度/频率对单人单文件严重富余（1GB/月、600 次/30 分钟），国内网络可达性最好；两端都是纯 HTTP 自实现，不引入二进制依赖，不动现有 Git 备份；免费版 1 个月文件历史给最坏情况兜底。代价：并发控制需自设修订校验（弱 CAS）。
2. **Git 私有仓库（GitHub Contents API 变体，非完整 git 客户端）**——`sha` CAS 语义一手文档明确，是所有候选里最干净的显式冲突模型；iOS 工程量与 WebDAV 相当（全 JSON/HTTP）。代价：GitHub 大陆可达性需用户实测；选它必然触发桌面 `git.autoPush` 的改造议题（#1 已预留）。libgit2/SwiftGit2 完整客户端路线因 SwiftGit2 维护薄而不推荐。
3. **iCloud Drive（CloudDocs 路径）**——桌面改动唯一 ≈0 的方案（迁址即可），iOS 侧系统级同步与唯一系统级冲突 API（NSFileVersion）；体感上限最高。代价：Windows 端依赖 iCloud for Windows 且冲突行为无公开规范、iOS 需 security-scoped bookmark、深度绑定苹果生态。CloudKit 变体因记录模型错配 + 免费签名下推送可用性未验证而不推荐。
4. **Syncthing（Synctrain）**——生态健康（官方文档诚实标注 iOS 限制；Synctrain 活跃且免费），但 Zakki iOS 与同步器分属两个 app，沙箱隔离 + `.sync-conflict` 改名对单文件应用是持续摩擦，仅当走"通用文件池"思路才合适。
5. **自建微型同步服务**——可控性与实时性上限最高，但工程与运维成本最大；只有当以上候选全部不满足（例如要求真后台推送）才值得，Oracle Always Free 可把金钱成本压到 ¥0。
6. **Dropbox/其他**——免费但 3 设备限制踩线、冲突语义未固定、可达性存疑，列为备查。

## 不替用户拍板的开放点

- **GitHub 大陆可达性**：路线 2 的前提，需在用户真实网络环境实测（本报告无一手数据）。
- **坚果云 ETag/If-Match 是否可用**：决定路线 1 的并发控制精度；未见于公开文档，需实测（最坏退路：写前校验修订 + 1 个月历史版本恢复）。
- **免费签名下 CloudKit/APNs 推送可用性**：官方一手文档未能固定引用；影响所有路线的"后台到货提醒"上限与路线 3/5 的实时性预期，须 Xcode 实测后回填。
- **iCloud for Windows 实际质量**（同步延迟、按需下载对桌面读写的影响、并发编辑时的实际产物）：无公开规范，选路线 3 前应双端实测。
- **桌面 Git 备份与同步协议的关系**：若选 Git 路线，`git.autoPush` 不做 pull 的现状必须改造（范畴 #1 已声明另议）；若选 WebDAV/iCloud，现有 Git 备份可保留共存或停用，由用户取舍。
- **是否接受一次性付费解锁**（Working Copy push、Möbius 无限同步）：报告按 ¥0 口径排序，付费解锁的价值判断留给用户。
