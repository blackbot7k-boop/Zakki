# Tot MD — Tot 风格的速记便签（MyGo 原生 UI）

模仿 [Tot](https://tot.rocks)（The Iconfactory）的 7 点速记应用，用
[MyGo](https://github.com/egoist/mygo) 的**原生 Go UI** 实现：无 HTML/JS/webview，
纯 Go 单二进制，GPU 绘制。

## 功能

| Tot 的功能 | 本项目实现 |
|---|---|
| 单窗口 + 7 个彩色点导航 | ✅ 顶栏居中 7 个圆点环（激活实心），⌘1–⌘7 / 点击切换 |
| 每点独立主题色与底色 | ✅ Tot 2.1.1 真实配色：内容区背景 + 顶/底栏 status 色 + accent，浅深两套 |
| 自绘顶栏 | ✅ TitleBarHiddenInset，红绿灯嵌在染色顶栏上，顶栏可拖动窗口；✕ 清空当前点，⌖ 置顶 |
| Markdown 支持 | ✅ 编辑器实时语法高亮 + ⌘⇧P 预览（标题/列表/待办/引用/代码块/行内标记） |
| Smart Bullets 智能圆点 | ✅ ⌘⇧B 行首插入/取消 `- `；`- [ ]` 待办可在预览里直接勾选回写源文本 |
| 键盘操作 + 字数统计 | ✅ 底栏 `● N 行 · N 词 · N 字符 · 备份时间`，右侧 ✱ 圆点 / Aa 预览 / ⌕ 搜索（Tot 同布局） |
| 深浅色模式 | ✅ 跟随系统，可强制 浅色/深色（显示菜单） |
| Esc 隐藏窗口 | ✅ `disableEscape` 在 Tot 是偏好项，这里是"按 Esc 隐藏窗口"开关 |
| 全局热键呼出 | ✅ ⌘⇧空格 全局显示/隐藏（MASShortcut 在 Tot 内置同类能力） |
| 窗口置顶（浮动便签） | ✅ ⌥⌘T / 工具栏"置顶" |
| 自动备份 | ✅ 启动备份 + 每 5 分钟滚动备份，保留 20 份；⌘S 立即备份；可从访达打开备份目录 |
| 菜单栏（托盘）图标 | ✅ 程序化生成的 7 点图标，左键显示/隐藏，右键菜单 |
| Shortcuts / URL scheme | ❌ 未实现（Tot 走 INIntent + `tot` scheme） |
| iCloud 同步 / Share 扩展 | ❌ 未实现（本应用为本地单机版） |

Tot 的其他细节也一并移植了精神：单实例运行、关窗驻留托盘、危险操作前强制备份
（Tot 的 "Remove All Data and Quit" 需要 Shift 确认）。

## 取证记录（REA）

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

## 开发

```sh
go tool mygo dev      # 开发（热重载）
go test               # 无窗口视图测试 + markdown 引擎测试
go tool mygo vet      # mygo 静态检查
go tool mygo build    # 打包 .app / .dmg
```

## 数据

- 数据文件：`~/Library/Application Support/Tot MD/tot-md.json`（7 点文本 + 设置，原子写入）
- 备份：`~/Library/Application Support/Tot MD/backups/`（保留最近 20 份）

## 许可

MIT。Tot 是 The Iconfactory 的产品，本项目与其无关联；
配色数据来自用户本机已安装的应用包，仅作个人复刻研究用途。
