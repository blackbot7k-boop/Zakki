# Zakki — 原生 Go 的 Markdown 速记便签

Zakki 是一个 macOS / Windows 桌面速记应用：多个彩色标签各存一段 Markdown，
Typora 式所见即所得编辑，纯 [MyGo](https://github.com/egoist/mygo) 原生 Go UI
实现——无 HTML/JS/webview，单二进制，GPU 绘制，界面语言内置英文与简体中文
（默认跟随系统）。

## 功能

- **彩色标签速记**：默认 7 个标签（可 1–12 个），当前标签是带标题的彩色胶囊，
  其余是可配置形状的圆点；只有一个标签时顶栏保持干净，只留关闭与置顶。
- **所见即所得编辑**：焦点块保留标题/段落/列表/引用排版，Markdown 标记只留在
  内部源文本；表格带悬浮工具栏（调行列、对齐、删除），待办可直接勾选；
  `⌘⇧P` 随时切全文源码模式。
- **长文档流畅**：预览块列表只构建可见块，几千块的笔记滚动与按键依然轻快。
- **自动备份**：启动备份 + 编辑节流滚动备份 + `⌘S` 立即备份；可选镜像到
  云盘同步文件夹，或 Git 仓库自动提交/推送；默认保留最近 20 份。
- **界面语言**：英文 / 简体中文，跟随系统，可在首启向导、设置或菜单随时切换。
- **顶栏个性化**：液态切换动画可关闭；非当前标签形状可选
  圆点 / 圆环 / 方点 / 圆角方 / 胶囊 / 菱形 / 六边形 / 三角 / 星形 / 水滴 / 叶片。
- **原生体验**：全局热键呼出、窗口置顶、关窗驻留托盘、单实例、
  深浅色跟随系统、全套快捷键可录键重绑定。

## 安装

从 [GitHub Releases](https://github.com/gomeen/Zakki/releases/latest) 下载：

- **macOS**：通用 DMG（Intel 与 Apple Silicon），拖入「应用程序」。当前为
  ad-hoc 签名（无 Developer ID 与公证），首次打开需在「系统设置 → 隐私与
  安全性」里允许。
- **Windows 10/11 x64**：`Zakki Setup <版本> amd64.exe`，按用户安装、无需管理员；
  未做 Authenticode 签名，SmartScreen 可能提示，确认来源后运行。

应用内 **Zakki → 检查更新…** 可手动检查并安装签名更新（默认不后台检查）。

## 快捷键速览

| 按键 | 作用 |
|---|---|
| `⌘1`–`⌘9` | 切换标签 |
| `⌘T` / `⌘R` | 新建 / 重命名标签 |
| `⌘⌥←` / `⌘⌥→` | 上一个 / 下一个标签 |
| `⌘B` / `⌘I` / `⌘K` | 加粗 / 斜体 / 链接 |
| `⌘⌥1`–`⌘⌥6` / `⌘0` | 标题 1–6 / 正文 |
| `⌘⇧8` / `⌘⇧7` / `⌘⇧9` | 无序 / 有序 / 任务列表 |
| `⌘⇧Q` / `⌘⌥C` | 引用 / 代码块 |
| `⌘F` / `⌘G` / `⇧⌘G` | 查找 / 下一个 / 上一个 |
| `⌘⇧P` | 源码模式 |
| `⌘S` | 立即备份 |
| `⌘⇧C` | 复制标签全文 |
| `⌘⇧空格` | 全局显示/隐藏（可改键） |
| `⌥⌘T` | 窗口置顶 |
| `⌘,` | 设置 |

Markdown 命令与全局唤起都可在 **设置 → 快捷键** 里录制改键。

## 数据与备份

- 数据目录：macOS `~/Library/Application Support/Zakki/`，
  Windows `%APPDATA%\Zakki\`。
- 数据文件 `zakki.json`：全部标签文本与编辑会话状态，tmp+rename 原子写入；
  偏好文件 `preferences.json` 是所有设置的唯一来源。
- 备份在 `backups/` 子目录；更早版本的数据目录与文件会在升级后首次启动时
  自动迁移到新命名，原样搬入不丢内容。
- 数据文件损坏时隔离原件、优先从备份恢复；无备份可用时进入写保护，
  新改动暂存 `.pending`，确认后才写回。

## 开发

```sh
go tool mygo dev      # 开发（热重载）
go test               # 无窗口视图测试 + markdown 引擎测试 + 持久化测试
make check            # gofmt + vet（含 mygo vet）+ test + race + arch 检查（CI 同款）
make build            # macOS 通用 .app / .dmg
make build-windows    # Windows x64 .exe
```

界面文案在 `locales/*.json`（ICU MessageFormat，ARB 格式）；
`go tool mygo generate` 生成 `locales/catalog_gen.go`，键位一致性有测试保障。
工程标准与模块职责见 [docs/architecture.md](docs/architecture.md)。

## 性能

输入管线块级增量（基准 180KB / 2401 行 / 1200 块）：单次按键约 0.2ms / 0.4MB
垃圾；源码高亮 163ms → 1.6ms；搜索命中 10.7ms → 0.54ms。预览块列表虚拟化后，
帧构建成本只随可见块数增长（4001 块笔记每帧 ≈ 0.28ms），与文档长度无关。

## 许可

[MIT](LICENSE)。内嵌资源各自遵循其许可：霞鹜文楷 Lite 字体（OFL，见
`resources/OFL-LXGW-WenKai-Lite.txt`）、Lucide 图标（ISC）。
