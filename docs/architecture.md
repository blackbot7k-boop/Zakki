# Zakki 架构说明

面向后续维护者与 agent：先读本文再改代码。本文只描述稳定事实，具体行为以源码为准
（关键处标注了文件与函数）。项目是 [MyGo](https://github.com/egoist/mygo) 原生 Go UI
桌面应用，无 HTML/JS/webview，纯 Go 单二进制。

项目分两层：`internal/app` 是 UI 与应用装配（唯一允许依赖 MyGo 的 internal 包）；
`internal/` 其余包是纯逻辑与持久化，禁止依赖 MyGo 与 `internal/app`。根目录只有入口 `main.go`。

## 模块职责

| 文件 | 职责 |
|---|---|
| `main.go` / `internal/app/lifecycle.go` | 入口 `main()` 只调 `app.Run()`；生命周期：单实例锁（`RequestSingleInstanceLock`，第二个实例把参数交给第一个后退出）、日志落盘（`setupLogging`）、macOS 菜单（`buildMenu` / `createMenu`，Typora 式分工）：应用菜单统一提供「设置… ⌘,」入口，快捷键等页面从设置窗口导航进入，无配置文件入口；「段落/格式」两个顶层菜单分别收块级与行内 Markdown 命令并展示当前绑定；「编辑」收系统编辑操作、复制标签全文与查找组（查找/查找下一个/查找上一个）；「显示」保留源码模式/置顶与上/下一个标签、「切换标签」子菜单；「窗口」除系统项外提供「显示主窗口」（关闭窗口驻留托盘）；托盘（`setupTray`：显示/隐藏、设置、退出）、全局热键（`syncGlobalHotkey`，动态改键）、退出收尾（`OnWillQuit` → `finishQuit` → `shutdownForQuit`；保存失败阻止退出/更新重启并重开主窗口）。装配在 `internal/app`；窗口创建 `createWindow`：`Frameless: true` 无边框窗口（无红绿灯），`StateKey: "main"` 记住位置大小（偏好开关控制），关闭窗口驻留托盘（`OnWindowAllClosed` 空实现）。 |
| `internal/app/updates.go` | 注册 MyGo `plugins/updater/native` 原生更新窗口（中文、默认手动检查）；应用菜单与托盘提供「检查更新…」。`finishQuit` 让更新重启也经过最终保存，失败则取消退出。更新公钥与 `gomeen/Zakki` 分发地址在 `mygo.json`；tag 经全量检查后分别构建 macOS universal 与 Windows x64，双平台均成功再汇总到草稿 Release；流程在 `.github/workflows/release.yml`，操作见 [release.md](release.md)。 |
| `internal/app/prefs_runtime.go` / `internal/app/prefs_bridge.go` | 偏好运行时集成：`loadPreferences`/`updatePreferences`；写盘委托 `prefs.Store`。类型别名在 `prefs_bridge.go`。 |
| `internal/app/config_bridge.go` | 旧配置类型别名，以及 `decodeConfig` / `defaultFileConfig`。 |
| `internal/app/notes_bridge.go` | 笔记磁盘类型别名、`migrateSavedFile` 与 `newNotesStore`（内含生产 git runner 装配）。 |
| `internal/app/backup_mirror.go` | 外部备份目的地：`validateMirrorDir`（拒绝数据目录/备份目录自身 + 可写性探测）、`dirIsGitRepo`、Git 档位展示映射、`chooseMirrorDirAsync`（后台 goroutine 弹 `mygo.Dialog.Open{Directory: true}`，经窗口 `Update` 回主线程）；设置页「外部备份」卡片的选/换/断开与 Git 分段入口也在这里。 |
| `internal/app/onboarding.go` | 首启初始化向导（仅全新安装，`firstRun` = 数据文件不存在）：五步（欢迎/备份文件夹/初始标签数/Git 可选/完成）草稿式确认；`finishOnboarding` 写偏好后经 `commitFirstRunData` 落盘，失败留在向导；跳过/关窗经 `persistFirstRunSnapshot` 写默认快照（关窗落盘失败只提示）；欢迎文案随标签数生成（`welcomeTextFor`）；`StateKey: "onboarding"` 独立窗口，主窗口等向导结束再创建。 |
| `internal/legacyconfig` | 旧 config.toml/json 严格解码（仅供迁移）。 |
| `internal/atomicfile` | tmp+rename 原子写（笔记与偏好共用）。 |
| `internal/markdown` | Markdown 解析、格式变换、表格纯函数（无 MyGo 依赖）。 |
| `internal/notes` | `tot-md.json` 磁盘模型、`Store`（载入/防抖保存/备份/写保护）。 |
| `internal/prefs` | `preferences.json` 模型、`Store`（修订号/防抖写盘/异常门禁）、`MigrateLegacy`。 |
| `internal/app/live_markdown.go` / `internal/app/view_highlight.go` | 桥接 `internal/markdown`；源码高亮 `editorRanges`（依赖 `ui.TextRange`）。 |
| `internal/app/menu_commands.go` | 命令注册表 `commandRegistry`（ID/名称/分组/默认绑定，Markdown 命令 + 全局唤起）：菜单、Keymap 设置页与编辑器共用；加速器解析/规范化/显示名（`parseAccelerator`/`accelerator`/`displayName`）、录键校验（`validateBinding`）与冲突检测（`findBindingConflict`，含固定键与系统保留键）。 |
| `internal/app/menu_edit.go` | 编辑命令路由：正文聚焦时，菜单栏与右键菜单的撤销、重做及剪贴板操作共用文档事务；搜索、标签重命名和设置中的输入保留系统编辑角色。焦点切换时 `refreshEditMenuRouting` 只刷新编辑子菜单项（系统 Role ↔ 应用 Click），不全量 `buildMenu`。右键命令在主视图帧末执行，避免控件构建期间重复记录历史。 |
| `internal/app/live_format.go` | 格式命令执行（`execFormatCommand`）：标题/列表/引用/行内/代码块/插入在实时预览与源码模式共用；无选区切换「待输入格式」意图（`mdPending`，输入/粘贴/IME 确认时经 `pendingWrapFor` 生成有效 Markdown）；命令以独立撤销事务执行（`editCommand`）。 |
| `internal/app/live_link.go` | 链接浮层 `linkTool`：打开、校验并写回链接。 |
| `internal/app/settings_window.go` / `internal/app/settings_ui.go` / `internal/app/settings_keymap.go` | 原生设置窗口：单实例非模态（`StateKey: "settings"` 独立记忆位置尺寸），左侧导航常规/外观/标签与外观/备份与数据/快捷键五页；控件绑定草稿（`settingsUIState`）+ 统一推入 `updatePreferences`；外观页双主题选择卡片与只读预览；快捷键页搜索、按键录制、解除/恢复，全局热键动态重注册（`syncGlobalHotkey`：先注 |册新键成功后再注销旧键）。 |
| `internal/app/view_theme.go` | 双层主题与每点配色：`colorThemes`、`dotThemes`、`markdownLayouts`；`resolveStyle` 合并颜色 + 排版 + 用户覆盖；排版可带装饰尺寸（引用条宽、代码圆角/描边，手账主题使用）。 |
| `internal/app/view_sketch.go` | 手账排版的手绘装饰：种子确定的抖动铅笔线（`sketchLine`/`sketchRect`）、波浪分隔线、荧光笔标题底（一/二级）与铅笔下划线（三级及以下）、点阵纸与页边红线、马克笔引用条、手绘勾选框（`sketchCheckbox`，保留复选框语义）与列表小圈；表格格线与代码框为铅笔线；手绘颜色主题内容区不透明（`paperBackground`）。 |
| `internal/app/window.go` | 窗口显示与隐藏：`showWindow`、`toggleWindow`、`applyAlwaysOnTop`。 |
| `internal/app/view_status.go` | 状态栏文案与字数统计：`statusWarn`、`countWords`、`backupLabel`、`mirrorLabel`。 |
| `internal/app/app_state.go` | `app` 由领域子结构体匿名嵌入，另加 `cfg`、`ready`、`now` 与 `notes.Store` / `prefs.Store`。 |
| `internal/app/state.go` | 笔记载入编排与搜索匹配；写盘委托 `notes.Store`。 |
| `internal/app/tab_model.go` | 动态标签：`tab`、`DefaultTabCount`（7）、`MaxTabCount`（12），以及当前标签的访问与增删改。 |
| `internal/app/tab_strip.go` | 标签栏液态动画与重命名几何。 |
| `internal/app/settings_tabs.go` | 设置窗口「标签与外观」页。 |
| `internal/app/view.go` | 主窗口骨架：`view` 每帧构建顶栏、查找条、源码编辑区与底栏；顶栏左侧「关闭窗口」调用 `Window.Close`，保留笔记与标签，关闭后可由托盘或全局热键重新打开。 |
| `internal/app/view_live.go` | 实时预览块列表：`livePane`、焦点块 `activeText` 与非焦点块渲染。 |
| `internal/app/live_blocks.go` | 按行切块：`liveBlocks` / `parseLines`。焦点块保留排版并编辑可见文字；Markdown 标记留在源文本中。 |
| `internal/app/live_edit.go` | 草稿提交与焦点：`commitDraft`、`publishDraft`、`focusBlock`、块缓存。 |
| `internal/app/live_input.go` | 边界键：Enter 分段与列表续行、退格合块、方向键跨块。 |
| `internal/app/live_widgets.go` | 焦点块输入（`applyProjectedHint`）与表格单元格编辑控件。 |
| `internal/app/live_projection.go` | 可见文字投影与 rune 映射（`projectBlock`/`replaceSelection`）；文档级撤销事务（`liveEdit` + `editKind`：连续输入合并、粘贴/IME/格式命令独立），`undoLive` 覆盖实时预览与源码模式，恢复文本与选区。 |
| `internal/app/table_edit.go` | 表格编辑：`createTableOnEnter`、`navigateTable`、`tableEnter`。 |
| `internal/app/table_actions.go` | 表格工具状态 `tableTools` 与行列增删。 |
| `internal/app/table_toolbar.go` | 表格悬浮工具栏。 |
| `internal/app/live_syntax.go` | 块级类型与 `parseBlocks`（测试/预览适配）；行内解析与格式变换见 `internal/markdown`。 |
| `internal/app/view_icons.go` | 内嵌 Lucide 图标（ISC 许可），24×24 viewBox、2px 圆头描边，`ui.MustParseSVG` 生成。 |
| `internal/app/tray_icon.go` / `internal/app/assets/tray.png` | 托盘图标：内嵌「墨 Z」手写字母 PNG（`trayPNG`，36×36px，与应用图标配套）；macOS 以模板自动适配菜单栏明暗，其他平台保留白色。`assets/tray.svg` 保留矢量源。 |
| `internal/app/font_hand.go` / `resources/LXGWWenKaiLite-Regular.ttf` | 内置手写体：霞鹜文楷 Lite（OFL 许可，许可见 `resources/OFL-LXGW-WenKai-Lite.txt`）。字体以单份资源随包分发（macOS 在 Contents/Resources，Windows 在 exe 旁），不 go:embed——嵌入 universal 二进制会每架构各带一份。`registerHandFont`（sync.Once 幂等）只调用一次 `ui.RegisterFont`（每次注册都会整份拷贝字体进 CoreText），由手写字体栈解析路径懒触发，手写主题用户在 WhenReady 开窗前预热；非手写用户不付这份内存。字体栈只允许引用已注册家族名或已知安全的回退名，否则无头环境的文本布局会阻塞在系统按需字体下载上（`TestHandFontCoversStacks` 兼守）。 |
| `resources/icon.png` / `resources/icon.svg` | 「墨 Z」应用图标：米色纸上一笔手写 Z 加朱红句点（1024×1024 PNG 与 SVG 源文件），MyGo 构建时生成 macOS ICNS 与 Windows ICO。设计原型保留在 `docs/demo/app-icon/`，不参与构建。 |

## 线程模型（MyGo 硬约束，违反会在运行期出错或被 `go tool mygo vet` 拦下）

- **app 状态只在主线程改**。`view` 每帧在主线程读状态构建 UI；菜单/托盘/热键回调经
  `withWindow` 回主线程改状态并触发重绘。
- **后台写盘只接触不可变快照**。笔记经 `snapshotFile`、偏好经 `prefs.Store.TakeSnapshot`，
  均在主线程拍值拷贝；后台定时器只写快照，绝不读写 `app` 的可变字段；
  协调状态在 `notes.Store` / `prefs.Store` 内部 mutex 中，偏好另用修订号防旧覆盖新。
- **每帧构建的 UI 元素（`ui.*` 控件）不存入 `app`**。`app` 里只保留窗口/菜单这类桌面集成句柄
  （`*mygo.Window`、`*mygo.Menu`、`*mygo.MenuItem`），普通跨帧状态一律存普通字段。
- **跨线程刷新用 `Window.Update`**。`scheduleSave` 把 `win.Update(func(){})` 放进写完后的 `after` 回调，
  回主线程刷新状态栏（备份时间/错误提示）；窗口失效时 Update 自然空转。
- 退出流程（`finishQuit` → `shutdownForQuit`，主线程）：停防抖定时器 → 置 `stopped` → 最终快照保存并等它完成。
  笔记与偏好均成功后才退出（更新器的重启也经过此流程）；任一保存失败则取消退出、恢复笔记 Store 接收保存，
  重开主窗口显示错误，允许继续编辑与重试。

## 持久化生命周期

数据文件：`<数据目录>/tot-md.json`（目录见下节）。

### 保存

1. **防抖**：每次改动调 `scheduleSave`，默认 800ms（`Advanced.AutosaveDelayMS`）内没有新改动才落盘（`time.AfterFunc`）；
   防抖到期由 `notes.Store.flushPending` 在后台取走最新快照写盘（新快照取代旧的排队快照）。
2. **原子写**：`notes.Store.writeFileLocked` 经 `atomicfile.WriteBytes` 先写 `tot-md.json.tmp` 再 `os.Rename` 替换；
   rename 失败如实上报，不装作保存成功。目录自动 `MkdirAll`。
3. **写保护**：数据文件损坏且无可用备份时 `notes.Store.saveBlocked = true`，此后写入转到暂存文件
   `tot-md.json.pending`，绝不覆盖原路径；用户在菜单点「写回数据文件」（`confirmWriteBack`）
   确认后才写回原路径并清理暂存文件（写回失败则维持写保护）。

### 备份（`backups/` 目录，文件名 `tot-md-<时间戳>.json`，同秒多次加序号）

| 入口 | 行为 |
|---|---|
| 启动备份 `backupOnLaunch` | 默认每次启动、数据文件存在时写一份；可通过 `backupOnLaunch = false` 关闭。 |
| 自动备份 `maybeWritePeriodicBackup` | 距上次备份超过配置间隔（`Backup.IntervalMinutes`，默认 **5 分钟**，常量 `backupInterval`）才写一份，随防抖保存顺带触发。 |
| 手动备份 `backupNow`（菜单/托盘「立即备份」、⌘S） | **无条件**创建一份（绕过周期节流），先保存当前数据再备份。 |
| 清空当前标签 `clearActiveTab` | 危险操作：**先强制备份**（`backupNow`），备份失败则取消清空。 |
| 外部镜像 `writeMirrorLocked` | 本地备份写成功后，同一份内容镜像到 `Backup.MirrorDir`（云盘同步文件夹或 Git 仓库 clone），按同一 `Keep` 修剪且只动 `tot-md-*.json`；**镜像或 Git 失败只记警告与状态，绝不影响本地备份成功**。Git 档位 `Backup.GitMode`（off/commit/commitPush）非 off 且选了仓库目录时，后台 `git add tot-md-*.json` + commit（可选 push，`runGitJob`，runner 经 `Store.RunGit` 注入），忙碌期间合并为一次补跑；`LoadMirrorTime` 启动时从镜像目录恢复最近镜像时间。 |

保留策略：`notes.Store.pruneBackupsLocked` 按 `Backup.Keep` 修剪，默认只保留最新 **20 份**；修剪失败只记警告（备份本体已写成功）。

### 损坏隔离与恢复（`load` → `recoverFromBrokenFile`）

- 文件不存在：首次运行，写入欢迎文本。
- 解析失败 / 读取失败 / 版本过高：原件**复制隔离**为 `tot-md.json.corrupt-<时间戳>`
  （`notes.Store.QuarantineCorruptFile`，绝不删除或覆盖原件）→ 从备份目录新→旧找第一个可用备份恢复
  （`notes.Store.LoadNewestBackup`）→ 都失败则用默认值并进入写保护（见上）。
- 即使已从备份恢复，`load` 也如实返回错误，状态栏/日志给出提示（`notice`/`slog`）。

### 版本迁移

- `currentDataVersion = 2`（`notes.CurrentDataVersion`）。磁盘 `version` 低于 2 且没有 `tabs` 时，
  `migrateSavedFile`（包装 `notes.Migrate`）把旧 `dots` 迁成标签：补齐缺省标签、收敛 `active`、
  空 `themeSource` 按 `system`、旧字段 `preview` 并入 `source`。
- `version` 高于当前支持：返回 `versionError`，**绝不覆盖原文件**（走损坏隔离流程，
  暂存 .pending 等用户确认）。

### hideOnEscape 缺键默认 true

设置里其他项默认值都是 false/空串，唯独 `hideOnEscape` 默认 **true**。JSON 的 bool 分不清
「缺键」与显式 `false`，故 `hasHideOnEscapeKey` 用 `map[string]json.RawMessage` 探测键是否存在：
缺键（含 settings 整体缺失）按 true 处理，显式 false 保留。

## 数据文件格式（`savedFile`）

```json
{
  "version": 2,
  "active": 0,
  "tabs": [{ "id": 1, "text": "…", "color": "#…" }],
  "settings": {
    "themeSource": "system",
    "alwaysOnTop": false,
    "hideOnEscape": true,
    "source": false
  }
}
```

| 字段 | 类型 | 含义 |
|---|---|---|
| `version` | int | 数据版本，当前 `currentDataVersion = 2`；0/1 视为固定 7 点格式；更高拒载不覆盖 |
| `active` | int | 当前激活的标签下标（载入时按标签数 clamp） |
| `tabs` | [{id,name,text,color}] | 动态标签（1–12 个）：稳定 ID、可选名称、markdown 纯文本、颜色（空/非法回退调色板）；v1 的 `dots` 字段仅为读取保留，新保存不写 |
| `settings.themeSource` | string | `system` \| `light` \| `dark`，空串按 `system` |
| `settings.markdownTheme` | string | 空 / `sonnet` 为 Sonnet，`tot` 为 Tot 主题 |
| `settings.bodyFont` / `settings.monoFont` | string | 正文 / 代码字体预设，空为主题默认 |
| `settings.bodySize` / `settings.lineHeight` | float32 | 正文字号 / 行高，0 为主题默认 |
| `settings.alwaysOnTop` | bool | 窗口置顶 |
| `settings.hideOnEscape` | bool | 按 Esc 隐藏窗口，**缺键默认 true**（见上） |
| `settings.source` | bool | 源码模式（false = Typora 式实时预览） |
| `settings.preview` | bool | 旧版字段，仅读取时并入 `source`，不再写出新值 |

## 偏好生命周期

`newApp` 内置默认偏好 → `load` 恢复笔记数据（含编辑会话状态）→ `loadPreferences`
读取 `preferences.json`（缺失时执行一次性迁移：默认值 → 笔记内旧设置 →
旧 config.toml / config.json 显式值，TOML 优先；结果原子写入，旧文件原样保留）→
`WhenReady` 里创建窗口、应用主题并注册全局热键（`a.ready` 之后偏好变更才触达原生 API）。
运行期所有偏好变更走 `updatePreferences` 统一入口：校验 → 应用运行时 → 同步菜单/窗口/热键 →
防抖自动保存；磁盘写经 `prefs.Store.WriteSnapshot` 内部的 `writeMu` 串行化、tmp/rename 原子替换，修订号防旧快照覆盖新值。
`preferences.json` 损坏或版本超前时进入待恢复状态：不退回旧配置、不覆盖原文件，
设置页提供「另存原件并恢复默认」；旧配置损坏时保留原件并提示，
「使用已有偏好继续」后以当前内存偏好建立新文件。
偏好版本当前为 **3**（`prefs.CurrentVersion`）：版本 2 起 `appearance` 分开记录中文字体
`bodyFont` 与西文字体 `latinFont`（空串 = 跟随中文字体）；版本 3 起 `backup` 记录
外部备份目的地 `mirrorDir`（空串 = 未配置）与 Git 自动化档位 `gitMode`
（off/commit/commitPush，缺键按 off）。旧版本文件在 `Decode` 里升级：缺键按未配置处理，
行为与升级前一致；写回后即为版本 3。升级带来的兼容代价：旧版本程序读到版本 3 会按
「版本超前」拒绝并保留原文件。
退出流程（`shutdownForQuit`）在笔记数据最终保存后等待偏好最终保存（`shutdownPreferences`），
失败返回 `finishQuit`，阻止退出或更新重启。
完整契约见 [偏好与设置](configuration.md)。MyGo 更新器另外管理 `updater.json`（检查时间、跳过版本、自动下载选择），
不改变应用偏好的 JSON 契约。

## 测试接缝

- **路径注入**：`notes.Store` 的 `DataFile` / `BackupDir`。生产由 `newApp` 用 `dataDir()`
  （`mygo.App.Path(PathAppData)` + `dataDirectoryName`，macOS 为 `~/Library/Application Support/Tot MD/`，Windows 为 `%APPDATA%\Tot MD\`，
  失败回退 `os.UserHomeDir` + `Library/Application Support`）。改名 Zakki 后保留旧数据目录、文件名与 bundle identifier；
  启动时 `PathUserData` 也指向旧目录，MyGo 窗口状态与 `updater.json` 更新辅助状态仍在这里。由此拼出 `tot-md.json` 与 `backups/`。
- **时钟注入**：`app.now func() time.Time`，生产 `time.Now`，测试给假时钟。
  创建后 `now` 只读，后台可安全读取。
- **`newTestApp(t)`**（`main_test.go`）：注入 `t.TempDir()` 与固定时钟（2024-06-01），
  **绝不写真实用户目录**。保存、备份与写保护在 `internal/notes`；`persistence_test.go` 覆盖载入映射与清空标签。
- **`ui.NewTester(a.view, w, h)`**：无窗口跑视图（构建/断言文本、模拟按键），
  `main_test.go`/`tab_style_test.go`/`live_boundary_test.go`/`live_focus_test.go` 用它。
- **偏好测试**：`internal/prefs` 的测试覆盖迁移优先级（TOML/JSON/笔记旧设置）、
  损坏与超前版本恢复、写失败报告、修订号防回写；默认值与严格解码在 `internal/prefs`；
  旧配置解码在 `internal/legacyconfig`；笔记版本迁移表在 `internal/notes`；
  `menu_commands_test.go` / `internal/markdown` 的格式与行内边界测试 / `view_theme_test.go`
  分别覆盖加速器与冲突校验、纯格式变换（含 CJK/emoji/光标映射）与全主题对比度；
  `menu_contract_test.go` 验证新菜单结构、设置窗口五页（`ui.NewTester`）与改键流程。
- 测试命令见 `Makefile`：`make test`（`CGO_ENABLED=0 go test ./... -count=1`）、
  `make race`、`make vet`（含 `go tool mygo vet .` 的 UI 生命周期检查）。
