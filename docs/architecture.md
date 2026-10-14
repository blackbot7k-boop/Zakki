# Tot MD 架构说明

面向后续维护者与 agent：先读本文再改代码。本文只描述稳定事实，具体行为以源码为准
（关键处标注了文件与函数）。项目是 [MyGo](https://github.com/egoist/mygo) 原生 Go UI
桌面应用，无 HTML/JS/webview，纯 Go 单二进制。

## 模块职责

| 文件 | 职责 |
|---|---|
| `main.go` | 应用生命周期：入口 `main()`、单实例锁（`RequestSingleInstanceLock`，第二个实例把参数交给第一个后退出）、日志落盘（`setupLogging`）、macOS 菜单（`buildMenu` / `createMenu`）：应用菜单统一提供「设置… ⌘,」入口，快捷键等页面从设置窗口导航进入，无配置文件入口；「编辑 → Markdown」子菜单收纳格式命令并展示当前绑定；「显示」保留搜索/源码/置顶与「切换标签」子菜单；托盘（`setupTray`：显示/隐藏、设置、退出）、全局热键（`syncGlobalHotkey`，动态改键）、退出收尾（`OnWillQuit` → `shutdown`）。窗口创建 `createWindow`：`Frameless: true` 无边框窗口（无红绿灯），`StateKey: "main"` 记住位置大小（偏好开关控制），关闭窗口驻留托盘（`OnWindowAllClosed` 空实现）。 |
| `prefs_runtime.go` / `prefs_bridge.go` | 偏好运行时集成：`loadPreferences`/`updatePreferences`；写盘委托 `prefs.Store`。类型别名在 `prefs_bridge.go`。 |
| `config_bridge.go` | 旧配置类型别名，以及 `decodeConfig` / `defaultFileConfig`。 |
| `notes_bridge.go` | 笔记磁盘类型别名、`migrateSavedFile` 与 `newNotesStore`。 |
| `internal/legacyconfig` | 旧 config.toml/json 严格解码（仅供迁移）。 |
| `internal/atomicfile` | tmp+rename 原子写（笔记与偏好共用）。 |
| `internal/markdown` | Markdown 解析、格式变换、表格纯函数（无 MyGo 依赖）。 |
| `internal/notes` | `tot-md.json` 磁盘模型、`Store`（载入/防抖保存/备份/写保护）。 |
| `internal/prefs` | `preferences.json` 模型、`Store`（修订号/防抖写盘/异常门禁）、`MigrateLegacy`。 |
| `live_markdown.go` / `view_highlight.go` | 根目录桥接 `internal/markdown`；源码高亮 `editorRanges`（依赖 `ui.TextRange`）。 |
| `menu_commands.go` | 命令注册表 `commandRegistry`（ID/名称/分组/默认绑定，Markdown 命令 + 全局唤起）：菜单、Keymap 设置页与编辑器共用；加速器解析/规范化/显示名（`parseAccelerator`/`accelerator`/`displayName`）、录键校验（`validateBinding`）与冲突检测（`findBindingConflict`，含固定键与系统保留键）。 |
| `live_format.go` | 格式命令执行（`execFormatCommand`）：标题/列表/引用/行内/代码块/插入在实时预览与源码模式共用；无选区切换「待输入格式」意图（`mdPending`，输入/粘贴/IME 确认时经 `pendingWrapFor` 生成有效 Markdown）；命令以独立撤销事务执行（`editCommand`）。 |
| `live_link.go` | 链接浮层 `linkTool`：打开、校验并写回链接。 |
| `settings_window.go` / `settings_ui.go` / `settings_keymap.go` | 原生设置窗口：单实例非模态（`StateKey: "settings"` 独立记忆位置尺寸），左侧导航常规/外观/标签与外观/备份与数据/快捷键五页；控件绑定草稿（`settingsUIState`）+ 统一推入 `updatePreferences`；外观页双主题选择卡片与只读预览；快捷键页搜索、按键录制、解除/恢复，全局热键动态重注册（`syncGlobalHotkey`：先注册新键成功后再注销旧键）。 |
| `view_theme.go` | 双层主题与每点配色：`colorThemes`、`dotThemes`、`markdownLayouts`；`resolveStyle` 合并颜色 + 排版 + 用户覆盖。 |
| `window.go` | 窗口显示与隐藏：`showWindow`、`toggleWindow`、`applyAlwaysOnTop`。 |
| `view_status.go` | 状态栏文案与字数统计：`statusWarn`、`countWords`、`backupLabel`。 |
| `app_state.go` | `app` 由领域子结构体匿名嵌入，另加 `cfg`、`ready`、`now` 与 `notes.Store` / `prefs.Store`。 |
| `state.go` | 笔记载入编排与搜索匹配；写盘委托 `notes.Store`。 |
| `tab_model.go` | 动态标签：`tab`、`DefaultTabCount`（7）、`MaxTabCount`（12），以及当前标签的访问与增删改。 |
| `tab_strip.go` | 标签栏液态动画与重命名几何。 |
| `settings_tabs.go` | 设置窗口「标签与外观」页。 |
| `view.go` | 主窗口骨架：`view` 每帧构建顶栏、查找条、源码编辑区与底栏。 |
| `view_live.go` | 实时预览块列表：`livePane`、焦点块 `activeText` 与非焦点块渲染。 |
| `live_blocks.go` | 按行切块：`liveBlocks` / `parseLines`。焦点块保留排版并编辑可见文字；Markdown 标记留在源文本中。 |
| `live_edit.go` | 草稿提交与焦点：`commitDraft`、`publishDraft`、`focusBlock`、块缓存。 |
| `live_input.go` | 边界键：Enter 分段与列表续行、退格合块、方向键跨块。 |
| `live_widgets.go` | 焦点块输入（`applyProjectedHint`）与表格单元格编辑控件。 |
| `live_projection.go` | 可见文字投影与 rune 映射（`projectBlock`/`replaceSelection`）；文档级撤销事务（`liveEdit` + `editKind`：连续输入合并、粘贴/IME/格式命令独立），`undoLive` 覆盖实时预览与源码模式，恢复文本与选区。 |
| `table_edit.go` | 表格编辑：`createTableOnEnter`、`navigateTable`、`tableEnter`。 |
| `table_actions.go` | 表格工具状态 `tableTools` 与行列增删。 |
| `table_toolbar.go` | 表格悬浮工具栏。 |
| `live_syntax.go` | 块级类型与 `parseBlocks`（测试/预览适配）；行内解析与格式变换见 `internal/markdown`。 |
| `view_icons.go` | 内嵌 Lucide 图标（ISC 许可），24×24 viewBox、2px 圆头描边，`ui.MustParseSVG` 生成。 |
| `tray_icon.go` | 托盘图标：程序化生成透明底白色单圆环 PNG（`trayPNG`），超采样抗锯齿。 |

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
- 退出流程（`shutdown`，主线程）：停防抖定时器 → 置 `stopped` → 最终快照保存并等它完成，
  此后忽略新的保存请求，不会与定时保存并发写两份。

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
退出流程（`shutdown`）在笔记数据最终保存后等待偏好最终保存（`shutdownPreferences`）并报告失败。
完整契约见 [偏好与设置](configuration.md)。

## 测试接缝

- **路径注入**：`notes.Store` 的 `DataFile` / `BackupDir`。生产由 `newApp` 用 `dataDir()`
  （`mygo.App.Path(PathAppData)`，macOS 为 `~/Library/Application Support/Tot MD/`，
  失败回退 `os.UserHomeDir` + `Library/Application Support`）拼出 `tot-md.json` 与 `backups/`。
- **时钟注入**：`app.now func() time.Time`，生产 `time.Now`，测试给假时钟。
  创建后 `now` 只读，后台可安全读取。
- **`newTestApp(t)`**（`main_test.go`）：注入 `t.TempDir()` 与固定时钟（2024-06-01），
  **绝不写真实用户目录**。保存、备份与写保护在 `internal/notes`；`persistence_test.go` 覆盖载入映射与清空标签。
- **`ui.NewTester(a.view, w, h)`**：无窗口跑视图（构建/断言文本、模拟按键），
  `main_test.go`/`tab_style_test.go`/`live_boundary_test.go`/`live_focus_test.go` 用它。
- **偏好测试**：根目录 `prefs_test.go` 覆盖迁移优先级（TOML/JSON/笔记旧设置）、
  损坏与超前版本恢复、写失败报告、修订号防回写；默认值与严格解码在 `internal/prefs`；
  旧配置解码在 `internal/legacyconfig`；笔记版本迁移表在 `internal/notes`；
  `menu_commands_test.go` / `internal/markdown` 的格式与行内边界测试 / `view_theme_test.go`
  分别覆盖加速器与冲突校验、纯格式变换（含 CJK/emoji/光标映射）与全主题对比度；
  `menu_contract_test.go` 验证新菜单结构、设置窗口五页（`ui.NewTester`）与改键流程。
- 测试命令见 `Makefile`：`make test`（`CGO_ENABLED=0 go test ./... -count=1`）、
  `make race`、`make vet`（含 `go tool mygo vet .` 的 UI 生命周期检查）。
