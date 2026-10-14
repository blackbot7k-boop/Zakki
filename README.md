# Tot MD — Tot 风格的速记便签（MyGo 原生 UI）

模仿 [Tot](https://tot.rocks)（The Iconfactory）的 7 点速记应用，用
[MyGo](https://github.com/egoist/mygo) 的**原生 Go UI** 实现：无 HTML/JS/webview，
纯 Go 单二进制，GPU 绘制。

## 功能

| Tot 的功能 | 本项目实现 |
|---|---|
| 单窗口 + 彩色标签导航 | ✅ 顶栏动态标签栏（默认 7 个，最多 12 个）：当前标签是带标题的彩色胶囊，其余是 18pt 圆点（空内容灰色）；切换时胶囊液态融合滑向目标；⌘1–⌘9 / 点击切换，⌘T 新建、⌘R 重命名、双击胶囊重命名 |
| 每标签独立主题色 | ✅ Tot 2.1.1 真实 7 色 + 5 个扩展色，可在「设置 → 标签与外观」改色；内容区为窗口级毛玻璃上的中性背景，不再随标签换底色 |
| 自绘顶栏 | ✅ 无边框窗口（Frameless + 透明 + VibrancyContent），自绘顶栏可拖动窗口；✕ 清空当前标签，⌖ 置顶 |
| Markdown 支持 | ✅ Typora 式所见即所得块编辑：光标聚焦时保留标题/段落排版、列表/待办前缀和表格网格；标记只留在内部源文本；⌘⇧P 切全文源码模式 |
| 表格编辑 | ✅ 点击单元格后显示工具栏：网格调整行列数（快捷选择最多 8 列 × 10 行，含表头）、整表所有列统一左/中/右对齐、删除整表；非空裁剪先确认，支持撤销/重做；Tab / Enter 可继续增行 |
| 待办列表 | ✅ 手动输入 `- [ ]`，可在实时预览里勾选并回写源文本 |
| 键盘操作 + 字数统计 | ✅ 底栏 `● N 行 · N 词 · N 字符 · 备份时间`，右侧眼睛切换源码 / 放大镜搜索 |
| 深浅色模式 | ✅ 跟随系统，可在设置窗口强制 浅色/深色 |
| Esc 隐藏窗口 | ✅ `disableEscape` 在 Tot 是偏好项，这里是设置窗口中的「按 Esc 隐藏窗口」开关 |
| 全局热键呼出 | ✅ 默认 ⌘⇧空格 全局显示/隐藏，在「设置 → 快捷键」里改键或解除（MASShortcut 在 Tot 内置同类能力） |
| 窗口置顶（浮动便签） | ✅ ⌥⌘T / 工具栏"置顶" |
| 自动备份 | ✅ 默认启动备份 + 自动 5 分钟滚动备份 + ⌘S「立即备份」无条件创建 + 清空当前点前强制备份，默认保留 20 份；间隔、数量和启动备份在设置窗口调整 |
| 原生设置窗口 | ✅ 应用菜单「设置… ⌘,」：常规 / 外观 / 备份与数据 / 快捷键四页，所有设置即时生效并自动保存，每页可恢复默认值 |
| 颜色 × 排版双主题 | ✅ 颜色主题（Tot / 纸白 / 暖砂 / 雾青 / 石墨）与 Markdown 排版主题（Sonnet / Tot / 清晰 / 书页 / 紧凑）相互独立、自由组合，附只读示例预览 |
| Markdown 快捷键编辑 | ✅ 「设置 → 快捷键」按键录制：加粗/斜体/行内代码/链接/标题/列表/引用/代码块/分隔线/表格及全局唤起；冲突检测、解除绑定、恢复默认 |
| 菜单栏（托盘）图标 | ✅ 程序化生成的透明底白色单圆环图标，左键显示/隐藏，右键菜单 |
| Shortcuts / URL scheme | ❌ 未实现（Tot 走 INIntent + `tot` scheme） |
| iCloud 同步 / Share 扩展 | ❌ 未实现（本应用为本地单机版） |

Tot 的其他细节也一并移植了精神：单实例运行、关窗驻留托盘、危险操作前强制备份
（Tot 的 "Remove All Data and Quit" 需要 Shift 确认）。

## 实时预览的实现（Typora 取证）

「Typora 式实时预览」的原理，证据来自本机 Typora 1.14.10
（`/Applications/Typora.app/Contents/Resources/TypeMark/`，未打包 JS 可直接检索）
与公开资料：

- **编辑内核是自研 contenteditable DOM 编辑器**，不是 CodeMirror 定制：块渲染函数
  `toHTML` 按 `node.get("type")` 分派（paragraph/heading/blockquote/list/fences/table…），
  每块带 `cid`/`mdtype`，叶子块才 `contenteditable`。CodeMirror 5 只用于两类例外：
  每个代码块内嵌一个 CM 实例、整页源码模式（`sourceView.inSourceMode`；
  typora-issues#3538 开发者回复同证）。
- **文档模型是块树**：每块存原始 markdown 文本（`node.get("text")`），
  序列化即拼树，无损往返。
- **所见即所得靠「标记常驻、CSS 显隐」**：`**b**` 渲染为
  `<span class='md-meta md-before'>**</span><strong>b</strong><span class='md-meta md-after'>**</span>`，
  `.md-meta{display:none}` 默认隐藏，光标进入行内元素时 JS 加 `md-expand` 显示
  （`style/base-control.css`；官方主题文档公开 `md-expand`/`md-meta`/`md-content`）。
- **焦点块**：`File.editor.focusCid`；`toHTML` 对焦点块走 `brushContent`
  （标记可见 + `cursorDiff` 光标偏移映射），其余块走 `parseInline`；
  编辑后只重刷焦点块（`brushNode`），**增量单位是块**。
- **光标按（块 id, 文本偏移）存取**，undo 命令携带块 id+区间；
  Enter = `insertParagraph` 切分重解析，Backspace = `backspaceHandler` 块首向前合并，
  自动配对在 `handlePair`。
- 同构参考：MarkText 的 Muya 引擎（自研块编辑器 + CM 仅源码模式）、
  Yuyz0112/typora-web（ProseMirror 复刻，文档明言「textblock 保留源码分隔符、
  可见性是按选区派生的 decoration」）。

本项目在原生 Go UI（无 DOM/CSS）下的等价实现（`liveedit.go`）：

- 文档仍是单个 markdown 字符串（无损往返）；`liveBlocks` 按行切块。
- 焦点块编辑的是**可见文字投影**：标题、段落、列表、待办、引用和行内格式保持排版；Markdown 标记只留在内部源文本。表格焦点时仍显示网格，只在活动单元格内输入，竖线自动转义。
- 输入管线单块增量：打字 → 可见文字映射回草稿 → 重切变化区域 → 光标按（块, 行, 列）重映射。Enter 插入 `\n\n` 硬分段，列表自动续行，块首退格一次合并整个段落边界，方向键跨块。
- `⌘⇧P` 仍可切换全文源码模式；它是批量调整 Markdown 的备用入口，不是实时预览中的焦点编辑方式。
- 与 Typora 的差距：原生 TextArea 的光标/选区仍由平台控件实现，块内标记显隐行为有意简化；代码围栏和表格边界是结构，不是可编辑字符。

## 取证记录（REA，Tot 本体）

证据来自两条路径，全部在本机完成：

**公开文档**：tot.rocks 官网功能清单；Iconfactory 官方知识库
（"Can I add more than seven dots?" 证实 7 点上限、"Use a floating window"、
"Show and Hide Tot At Will with a Hotkey" 等文章标题）。

**REA 成品工件分析**（`rea-agents@6.4.0` MCP + Hopper，目标
`/Applications/Tot.app`，Mach-O x86_64，sha256 `876e17e8…`）：

- 字符串表：`BackupManager performBackupWithError:`、`disableSmartBullets`、
  `configureHotKeys`、`disableEscape`、`statisticsForCharacterCount:wordCount:lineCount:`、
  `MarkdownStyleEmphasisDouble/Link`、10 万字上限警告文案、
  分隔线付费提示、`ToutWindowController`（Tot 2.1 的 Tout 浮层）。
- ObjC 类：`MASShortcut/MASShortcutBinder`（全局热键）、`INIntent` 系（Shortcuts）、
  `ThemeColors initWithDotColor:textColor:accentColor:linkColor:…`（每点主题）。
- 反编译 `-[Theme loadTheme]` 发现默认主题读取**应用包内 `theme.json`**，
  随即提取 8 套主题的 dot/accent/背景色，本项目 `state.go` 中的 `dotThemes`
  即逐字来自该文件（红 `#d4253a`、橙 `#f67500`、黄 `#ffd01c`、绿 `#65c802`、
  蓝 `#0f75e9`、紫 `#a40bd8`）。

## 性能

实时预览的输入管线做了块级增量优化（基准为 180KB / 2401 行 / 1200 块的极限笔记，
`go test -bench .` 可复现）：

| 热点 | 优化前 | 优化后 | 手段 |
|---|---|---|---|
| 每次按键的块切分 | 全量重切 2401 行 | 只重切 ~2 行 | `spliceBlocks` 增量拼接：按行找公共前缀/后缀，复用未变块（含行内 spans），随机化测试保障与全量解析一致 |
| 每帧的块列表获取 | 全量重切 ×2 | 缓存命中（51ns / 0 分配） | 文档串作键，变更自然失效 |
| 源码模式高亮 | **163ms** | 1.6ms | 修 `add()` 内重复全文 rune 计数 + O(n²) 排序改 `sort.Slice` |
| 搜索命中 | 10.7ms | 0.54ms | 小写副本缓存 + rune 偏移递增（O(n)） |
| 草稿拼回文档 | 全文 Split/Join 中转 | 字节偏移直拼 | `lineByteOffset` 零分配扫描 |
| 状态栏统计 | 每帧 `strings.Fields` 全文分配 | 零分配扫描 |  |

一条按键的总成本从 ~5-8ms / ~2MB 垃圾 降到 ~0.2ms / ~0.4MB（不含 MyGo 渲染层）。

## 开发

```sh
go tool mygo dev      # 开发（热重载）
go test               # 无窗口视图测试 + markdown 引擎测试 + 持久化测试
go tool mygo vet      # mygo 静态检查
go tool mygo build    # 打包 .app / .dmg
make check            # gofmt + vet + test + race 全套检查（CI 同款，见 Makefile）
```

## 设置与偏好

所有用户偏好保存在数据目录中的 `preferences.json`，**不再需要手工编辑配置文件**。
通过应用菜单 **Tot MD → 设置…（⌘,）** 打开设置窗口：

- **常规**：窗口置顶、按 Esc 隐藏、记忆主窗口位置与尺寸（从下次创建主窗口起生效）。
- **外观**：跟随系统/浅色/深色；颜色主题与 Markdown 排版主题两组独立选择卡片
  （Tot 随七点配色，纸白/暖砂/雾青/石墨有固定明暗底色）；正文字体、代码字体、
  字号、行高可微调，清空即「跟随排版主题」；附只读示例预览。
- **备份与数据**：编辑后自动备份最小间隔（节流，非定时备份）、保留数量、
  启动备份；立即备份、打开备份目录、显示数据位置；数据文件写保护时的恢复写回。
- **快捷键**：Markdown 格式命令与全局唤起的按键录制，支持搜索、解除绑定、
  恢复单项/全部默认；与固定快捷键和系统保留键冲突时显示原因，不自动抢占。

所有设置即时生效并自动保存；每页提供「恢复本页默认值」（需确认，不清除笔记或备份）。
全局热键变更先注册新键、成功后再注销旧键，失败保留原绑定并提示。

### 从旧配置迁移（一次性）

首次没有 `preferences.json` 时，应用按 **默认值 → 笔记内旧设置 → 旧配置显式值**
的优先级执行一次迁移（优先读 `config.toml`，仅当不存在时读旧 `config.json`），
结果原子写入 `preferences.json`；旧文件原样保留，之后不再读取。
旧配置损坏时保留原件并提示，确认后可用已有偏好继续；偏好文件本身损坏或版本
超前时不会退回旧配置、不会覆盖原文件，确认后另存原件并恢复默认。
无需再手工编辑任何配置文件。

macOS 数据目录：`~/Library/Application Support/Tot MD/`。

## 数据

- 数据目录：macOS 为 `~/Library/Application Support/Tot MD/`，其他平台遵循系统用户配置目录（os.UserConfigDir）
- 数据文件：`tot-md.json`（7 点文本 + 编辑会话状态，tmp+rename 原子写入）
- 偏好文件：`preferences.json`（全部用户偏好的唯一来源；损坏时原件隔离另存）
- 旧配置：`config.toml` / `config.json` 仅供首次迁移读取，迁移后原样保留，不再读取
- 备份：`backups/`（默认启动备份 + 自动 5 分钟滚动备份 + 手动「立即备份」无条件创建 + 清空当前点前强制备份，默认保留最近 20 份；策略在设置窗口调整）

## 许可

MIT。Tot 是 The Iconfactory 的产品，本项目与其无关联；
配色数据来自用户本机已安装的应用包，仅作个人复刻研究用途。
