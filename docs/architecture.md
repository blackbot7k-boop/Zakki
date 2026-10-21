# Zakki 架构说明

面向后续维护者与 agent：先读本文再改代码。本文只描述稳定事实，具体行为以源码为准
（关键处标注了文件与函数）。项目是 [MyGo](https://github.com/egoist/mygo) 原生 Go UI
桌面应用，无 HTML/JS/webview，纯 Go 单二进制。

项目分两层：`internal/app` 是 UI 与应用装配（唯一允许依赖 MyGo 的 internal 包）；
`internal/` 其余包是纯逻辑与持久化，禁止依赖 MyGo 与 `internal/app`。根目录只有入口 `main.go`。

依赖通过标准 `vendor/` 固定。MyGo v0.5.1 的原生文字区间样式补丁在
`patches/mygo-text-range-background.patch`，刷新依赖使用 `make vendor`，维护约束见
[依赖补丁](../patches/README.md)。字体／斜体／字重进入原生布局，底色／颜色／下划线仅参与绘制；
区间合并、IME、选区和滚动仍复用输入控件，不另建文本布局。

## 模块职责

| 文件 | 职责 |
|---|---|
| `main.go` / `internal/app/lifecycle.go` | 入口 `app.Run()`：单实例、日志、macOS 菜单（应用/段落/格式/编辑/显示/窗口）、托盘、全局热键、退出与更新收尾；主窗 mac 为无边框自绘顶栏，Windows 用隐藏标题栏模式（`TitleBarStyle: hidden`）保留原生最小化/最大化/关闭（`StateKey: "main"`）、关窗驻留托盘、`OnActivate` 复用 `showWindow`。菜单项与快捷键细节见 `menu_commands.go`、`menu_edit.go` 与 [configuration.md](configuration.md)。 |
| `internal/app/startup_jobs.go` | 首个窗口（含首启向导）完成首帧后仅启动一次后台任务：备份时间恢复与启动备份按序执行，快照预取在主线程，完成后经活窗口 `Window.Update` 刷新。 |
| `internal/app/updates.go` | 注册 MyGo `plugins/updater/native` 原生更新窗口（中文、默认手动检查）；应用菜单与托盘提供「检查更新…」。`finishQuit` 让更新重启也经过最终保存，失败则取消退出。更新公钥与 `gomeen/Zakki` 分发地址在 `mygo.json`；tag 经全量检查后分别构建 macOS universal 与 Windows x64，双平台均成功再汇总到草稿 Release；流程在 `.github/workflows/release.yml`，操作见 [release.md](release.md)。 |
| `internal/app/prefs_runtime.go` / `internal/app/prefs_bridge.go` | 偏好运行时集成：`loadPreferences`/`updatePreferences`；写盘委托 `prefs.Store`。类型别名在 `prefs_bridge.go`。 |
| `internal/app/config_bridge.go` | 旧配置类型别名，以及 `decodeConfig` / `defaultFileConfig`。 |
| `internal/app/notes_bridge.go` | 笔记磁盘类型别名、`migrateSavedFile` 与 `newNotesStore`。 |
| `internal/app/backup_folder.go` | 备份文件夹（唯一备份地址）：`validateBackupDir`（拒绝应用数据目录及子目录 + 可写性探测）、`chooseBackupDirAsync`（后台 goroutine 弹 `mygo.Dialog.Open{Directory: true}`，经窗口 `Update` 回主线程）；设置页「备份文件夹」卡片（settings_backup.go 渲染）的选/换/断开与首启向导共用。 |
| `internal/app/onboarding.go` / `internal/app/onboarding_view.go` | 首启初始化向导（仅全新安装，`firstRun` = 数据文件不存在）：640×560 独立窗口（`StateKey: "onboarding-v2"`），两页草稿式确认——① 语言（English/简体中文，选择即时 `App.SetLocale` 并写偏好）、明暗、风格（标准/手账，写 `appearance.style`，不强制字体）、中文字体、英文字体（预选 JetBrains Mono）；② 初始标签数 1–9 与备份文件夹（预填 `defaultMirrorDir()` 即 `~/Documents/Zakki 备份`，可注入、可换，始终可见）。选择不触发下载；主按钮显示「完成设置」或「下载字体」，提交后冻结草稿，缺失字体经 `prepareFonts` 统一下载（总进度+单字体进度），完成后停在就绪页，点「开始使用」才写偏好与初始数据（`commitFirstRunData` 语义，失败留在向导）并进入主窗口；失败可重试、返回修改（作废旧下载批次）或改用系统字体。关窗等同接受默认（默认备份路径 + 默认快照，落盘失败只提示）；欢迎文案随标签数生成（`welcomeTextFor`）；主窗口等向导结束再创建。 |
| `internal/legacyconfig` | 旧 config.toml/json 严格解码（仅供迁移）。 |
| `internal/atomicfile` | tmp+rename 原子写（笔记与偏好共用）。 |
| `internal/markdown` | Markdown 解析、格式变换、表格纯函数（无 MyGo 依赖）。 |
| `internal/codehl` | 代码块逐行词法着色纯函数：`Kind`（注释/字符串/数字/关键字/函数/键）× 行内 `Span`，按围栏 info 归一化语言（go/js/ts/py/sh/json/yaml/toml/rust/c/java 家族，未知语言走通用规则）；不跨行追踪，块注释/多行字符串只着色起始行（无 MyGo 依赖）。 |
| `internal/notes` | `zakki.json` 磁盘模型、`Store`（载入/防抖保存/备份/写保护/Git 自动化）；`store.go` 负责载入与备份直写单一目的地，`save.go` 负责笔记快照保存、防抖合并和退出保存，`backup_status.go` 负责最近备份时间的后台恢复，`git_backup.go` 负责备份文件夹的 Git 自动提交与推送（探测、提交路径规范、推送节流，runner 可注入）。数据版本 3：排版主题旧 ID 更名为 `note`（旧字面量见 `notes` 包常量），随载入迁移。 |
| `internal/prefs` | `preferences.json` 模型、`Store`（修订号/防抖写盘/异常门禁）、`MigrateLegacy`。偏好版本 4：主题 ID 更名（旧字面量见 `prefs` 包常量，映射为 `classic`/`note`）随 `migrate` 迁移；`appearance.language` 记录界面语言（空 = 跟随系统）。偏好版本 5：代码配色默认「柔和」。版本 7：`appearance.codeIndent` 记录代码块缩进档位（2 / 4 空格，默认 2），旧文件补默认。版本 9：`backup.gitAutoPush`（Git 自动化开关，默认关，旧文件迁移不替用户开启）与 `backup.gitPushMinutes`（推送最小间隔，默认 10）。版本 10：`appearance.style`（标准 / 手账）；手账下生效的颜色与排版由 `Appearance.EffectiveColorTheme` / `EffectiveMarkdownTheme` 给出，渲染一律读这两个方法，`colorTheme` / `markdownTheme` 只记标准风格下的选择。版本 11：`advanced.windowColumns` / `windowRows` 取代像素宽高，换算见 `window.go`（`WindowPixels` / `WindowGrid`，旧像素按默认排版迁移）。 |
| `locales/` | MyGo i18n 目录：`en.json`（源）+ `zh-Hans.json`，ICU MessageFormat；`go tool mygo generate` 生成 `catalog_gen.go`。`internal/app` 统一经 `locales.*` 取文案；语言选择在首启向导第一步、设置通用页与应用菜单，变更经 `App.SetLocale` 生效并由 `OnLocaleChanged` 重建菜单。 |
| `internal/app/datamigrate.go` | 数据目录一次性迁移（更名后的旧目录 → 新目录）：复制到 staging、校验后发布，数据/暂存/备份文件换新命名；旧目录是根符号链接时复制其目标内容并保留链接与目标，路径探测状态不确定时失败关闭；`resolveDataDir()` 内 `sync.Once` 保证每进程只跑一次。迁移失败保留源文件并把错误交给 `Run`，Store 不会构造；修复后由下一进程重试。开发运行（`mygo.IsDev`，即 `go tool mygo dev` / `go run`）改用 `Zakki Dev` 目录且不做迁移，正式目录留给安装版与本地 `make build` 产物。 |
| `internal/app/live_markdown.go` / `internal/app/view_highlight.go` | 桥接 `internal/markdown`；源码高亮 `editorRanges`（依赖 `ui.TextRange`），用每 rune 一位的占用位图判交，保留搜索优先与重叠候选整段拒绝；代码内容行按外观「代码配色」方案套 `internal/codehl` 区间。 |
| `internal/app/view_codehl.go` | 代码配色方案表 `codeSchemes`（柔和/明快 × 浅深）与解析 `codePaletteFor`；`codeLineSpans`（实时预览 RichText）与 `codeSourceRanges`（源码 TextRange）把 `internal/codehl` 词法片段套色。 |
| `internal/app/menu_commands.go` | 命令注册表 `commandRegistry`（ID/名称/分组/默认绑定，Markdown 命令 + 全局唤起）：菜单、Keymap 设置页、窗口兑底与编辑器共用；加速器解析/规范化/显示名（`parseAccelerator`/`accelerator`/`displayName`）、固定绑定表（`fixedBindingsForPlatform`）。 |
| `internal/app/keymap_bindings.go` | 改键校验与写入：平台默认键（`defaultBindingForPlatform`：标题退到次级修饰键 mac ⌃N / 其他平台 Alt+N）、录键校验（`validateBinding`）、冲突检测（`findBindingConflict`，含固定键与系统保留键）、单项写入/恢复与全部恢复。 |
| `internal/app/menu_edit.go` | 编辑命令路由：正文聚焦时，菜单栏与右键菜单的撤销、重做及剪贴板操作共用文档事务；搜索、标签重命名和设置中的输入保留系统编辑角色。焦点切换时 `refreshEditMenuRouting` 只刷新编辑子菜单项（系统 Role ↔ 应用 Click），不全量 `buildMenu`。右键命令在主视图帧末执行，避免控件构建期间重复记录历史。`copyTabTextToClipboard` 供菜单与快捷键兑底共用。 |
| `internal/app/live_format.go` | 格式命令执行（`execFormatCommand`）：标题/列表/引用/行内/代码块/插入在实时预览与源码模式共用；无选区切换「待输入格式」意图（`mdPending`，输入/粘贴/IME 确认时经 `pendingWrapFor` 生成有效 Markdown）；命令以独立撤销事务执行（`editCommand`）。 |
| `internal/app/live_link.go` | 链接浮层 `linkTool`：打开、校验并写回链接。 |
| `internal/app/settings_window.go` / `internal/app/settings_ui.go` / `internal/app/settings_backup.go` / `internal/app/settings_keymap.go` | 原生设置窗口：单实例非模态（`StateKey: "settings"`），四页导航（通用、标签、备份与数据、快捷键）与草稿绑定；变更经 `updatePreferences`；「备份与数据」页在 settings_backup.go（备份文件夹卡片按所选文件夹形态双形态展示，见下）；快捷键页点击徽标录键、冲突就地「改用到这里」（录键期间经 `setKeymapRecording` 临时摘除菜单与全局热键，退出时重建；校验与写入在 keymap_bindings.go），全局热键先注册新键再注销旧键。字段契约见 [configuration.md](configuration.md)。 |
| `internal/app/settings_general.go` | 「通用」页骨架：常规（语言、窗口）在前，随后由 `settings_ui.go` 的 `appearanceSections` 构建主题、排版、代码、顶栏分区，整页共用受窗口高度约束的滚动区与单一重置入口；重置同时恢复窗口与外观，保留备份、快捷键和笔记。默认窗口大小按每行字数 × 行数输入，附当前字号换算的像素。迁移提示与偏好恢复入口保留在页顶。 |
| `internal/app/settings_select.go` | 设置页下拉：`boundedSelect` 与 `ui.Select` 同外观与键盘行为，弹层限高 300 并在内部滚动（`ui.Select` 不限高，选项多时被窗口裁掉）；`selectTrigger` / `selectPanel` 供字体选择器共用。 |
| `internal/app/view_theme.go` | 双层主题：`colorThemes`、`markdownLayouts`；经典颜色主题在主窗口与其他界面一样走中性调色板（`classicNeutralPalette`），`dotThemes` 只提供标签圆点、焦点 accent 与状态栏的标签色；`resolveStyle` 合并颜色 + 排版 + 用户覆盖，结果按（外观偏好、圆点、明暗、是否覆盖、本机字体列表身份）缓存最近 4 份（`fontRuntimeState.styles`），字体栈拼接与本机字体查找不再每帧重复；排版可带装饰尺寸（引用条宽、代码圆角/描边，手账主题使用）。字体栈在 Apple 字体之后列 Windows 回退（微软雅黑、宋体、Segoe UI、Consolas），macOS 渲染不变。 |
| `internal/app/view_sketch.go` | 手账排版的手绘装饰：种子确定的抖动铅笔线（`sketchLine`/`sketchRect`）、波浪分隔线、荧光笔标题底（一/二级）与铅笔下划线（三级及以下）；标题装饰容器 `sketchHeadingFrame` 在预览态与编辑态共用。点阵纸与页边红线、马克笔引用条、手绘勾选框（`sketchCheckbox`，保留复选框语义）与列表小圈；表格格线与代码框为铅笔线；手绘颜色主题内容区不透明（`paperBackground`）。 |
| `internal/app/window.go` | 窗口显示与隐藏：`showWindow`、`toggleWindow`、`applyAlwaysOnTop`；`defaultWindowSize` 按当前生效排版（`windowMetrics`：正文字号 / 行高覆盖或排版默认、排版内边距、平台顶栏 + 状态栏）把偏好里的每行字数 × 行数换算成新建主窗口的像素；创建主窗口时触发本机字体枚举（`loadFontFamilies`）；`resetWindowUI` 在创建新原生窗口前清理旧 `ListState` 和剪贴板服务，并恢复编辑器选区同步，文档与撤销历史保留。 |
| `internal/app/view_status.go` | 状态栏文案与字数统计：`statusWarn`、`countWords`、`backupLabel`；`docStats` 按文档串缓存，同一标签 32KB 以上的文档连续输入时每 250ms 至多重算一次，节流期沿用旧值并用 `time.AfterFunc` + `Window.Update` 预约一帧补算；切换标签立即重算。 |
| `internal/app/app_state.go` | `app` 由领域子结构体匿名嵌入，另加 `cfg`、`ready`、`now` 与 `notes.Store` / `prefs.Store`。 |
| `internal/app/state.go` | 笔记载入编排与搜索匹配；写盘委托 `notes.Store`。 |
| `internal/app/tab_model.go` | 动态标签：`tab`、`DefaultTabCount`（7）、`MaxTabCount`（9，数字快捷键全覆盖），以及当前标签的访问与增删改。 |
| `internal/app/tab_export.go` | 「导出为 Markdown…」（文件菜单与标签右键菜单）：主线程取快照（实时预览先 `commitDraft`），后台弹原生保存对话框，建议文件名取标签名并替换文件系统保留字符；经 `atomicfile` 原样写出，失败弹原生错误框，成功在底栏提示。 |
| `internal/app/tab_strip.go` | 标签栏液态动画与重命名几何；标记只有圆点与圆环。未命名时全部同大，当前标签只靠颜色区分；有名字时名字画在当前标签的色条里。圆环描边且不参与液态桥接。`TabMotionOff` 关闭补间（与系统 ReduceMotion 同路径）。顶栏拖拽排序：圆点按钮 `Drag(tabDrag)`，标签条为落点，经过时画插入指示线，松手记入 `dropPending`、帧末经 `reorderTab` 重排（按下不动仍是点击，双击重命名与右键菜单不受影响）。标签右键菜单（重命名/导出/删除，与菜单栏和固定快捷键同一实现）：菜单动作记为待执行、主视图帧末按稳定 ID 重定位后执行（`runTabMenuPending`），不在构建期改标签集合；重命名先切到被右键的标签，与双击同路径。 |
| `internal/app/settings_tabs.go` | 设置窗口「标签」页：标签列表是可拖拽排序的 List（`ListState.Reorder` → `reorderTab`，active 按稳定 ID 跟随）：行首为与顶栏同形的小圆点（点开 `ui.Popover` + `ui.ColorPicker` 取色，含常用色板；草稿以 `tabWellSeeded` 基线防 `Changed` 迟到误回播种）、双击行内重命名（会话状态独立于顶栏胶囊重命名）与删除图标。顶栏外观项（形状/标题宽/动画速度/动效/玻璃强度）在「通用」页的顶栏分区，见 settings_ui.go 的 `tabAppearanceSection`。 |
| `internal/app/view.go` | 主窗口骨架：`view` 每帧构建顶栏、查找条、源码编辑区与底栏；顶栏左侧「关闭窗口」调用 `Window.Hide`，保留笔记、标签和原生渲染资源，避免每次唤起都重建 Metal 管线；Dock、托盘或全局热键可重新显示。菜单关闭（⌘W）仍销毁窗口，后续由 `showWindow` 重建；退出流程不被隐藏逻辑拦截。顶栏按 `c.TitleBar()` 避让系统窗口按钮：右侧内边距加按钮带宽，Windows 上顶栏降到按钮带同高（32），内容与按钮 glyph 同轴垂直居中，无按钮带时保持 38。只有一个标签且不在重命名时隐藏标签区（`tabStripHidden`），并复位标签几何避免恢复多标签时残留补间。顶栏、底栏与查找条的图标按钮统一走 `chromeIcon`（16pt 图形、24pt 点击区、悬停浅底、名称兼作无障碍标签与悬停提示）；顶栏圆点悬停显示标签名、切换快捷键与正文首行（`tabTooltip`）。底栏文字单行截断并带悬停全文，窄窗口下切换图标不被挤出。 |
| `internal/app/find_bar.go` | 查找条、替换与命中跳转：替换开关 + 输入框 + 计数 + 跨标签开关 + 上/下/关闭图标，展开后第二行是替换框与「替换 / 全部替换」；`openFind(replace, allTabs)` 由 ⌘F / ⌥⌘F / ⇧⌘F 共用。替换走 `runAsCommand`（一次可撤销事务，实时预览先 `commitDraft`）；跨标签模式下计数按标签缓存（`findState.tabHits`，键为文档内容 + 查询），上/下一个越界时切到下一个有命中的标签；跨标签全部替换先弹原生确认，再 `backupNow`，备份失败即中止并提示，活动标签进撤销历史、其余标签直接改写后统一 `scheduleSave`。Enter 下一个、Shift+Enter 上一个（`findStep`，菜单查找下一个/上一个经 `findOpenOrStep` 共用）；`highlightQuery` 只在查找条打开时返回查询，源码区与实时预览（标题、段落、任务项，以及含命中的单行引用/列表）共用它高亮命中，关闭查找条即清除高亮。 |
| `internal/app/view_live.go` | 实时预览块列表：`livePane`（`ui.List` 虚拟化，只构建可见块；外层 Column 兜底左右留白与行间空隙的空白点击）、焦点块 `activeText` 与非焦点块渲染；列表显式绑定解析后的正文颜色、字体与行高，避免帧末恢复外框主题时覆盖内容继承。 |
| `internal/app/live_blocks.go` | 按行切块：`liveBlocks` / `parseLines`。焦点块保留排版并编辑可见文字；Markdown 标记留在源文本中。 |
| `internal/app/live_edit.go` | 草稿提交与焦点：`commitDraft`、`publishDraft`、`focusBlock`、块缓存与 rune 首偏移表。文档重切换代时清理预览派生缓存；偏移表重建分配新切片，不覆写当前构建帧持有的旧代。 |
| `internal/app/live_preview_cache.go` | 当前文档块代际的有限缓存（最多 64 块）：可见文字投影、表格 cell/偏移与代码 token spans；只保存纯数据，不保存 UI 元素/Context，不保留历史版本。切标签、重切和编辑失效，代码颜色随完整 palette 失效；仍由原控件整体布局单个大块。 |
| `internal/app/live_input.go` | 边界键：Enter 分段与列表续行、退格合块、方向键跨块；代码块内 Enter 委托 `applyCodeEnter`。 |
| `internal/app/live_codeblock.go` | 围栏代码块专属编辑：Tab/Shift+Tab 缩进（`applyCodeTab`，单位取通用页代码分区的 `appearance.codeIndent`，退化围栏落到合成内容行）、Enter 续行缩进（`applyCodeEnter`，继承行首缩进、开括号再加一级）、右下角语言徽标（`codeLangBadge`：FocusBind 聚焦、失焦/Enter 提交、Esc 取消，改写围栏 info 串不经 `applyDocEditAt` 以免抢主编辑器焦点；会话态 `codeLang*` 存 `liveEditorState`）。 |
| `internal/app/live_virtual.go` | 块内虚拟化：200 行以上的代码块预览与表格（含正在编辑的表格）只构建视口上下各一屏的行，其余用占位撑开；位置取上一帧 `Bounds` 并按滚动量校正，行高按构建过的行实测缓存（`liveVirtState`，键为块源码 + 块类型，帧末清掉未用条目），未测行用实测均值；正在编辑的表格行总会构建；窗口依据过期（首帧或刚滚动）时再要一帧校正。单个超长段落仍整段排版。 |
| `internal/app/live_widgets.go` | 焦点块输入（`applyProjectedHint`）与表格单元格编辑控件。 |
| `internal/app/live_inline_style.go` | 行内样式唯一映射 `inlineStyle`：粗体、斜体、代码字体／底色、链接颜色／下划线；投影预览、列表／引用 RichText 子元素与原生编辑区间共用定义。 |
| `internal/app/live_projection.go` | 可见文字投影与 rune 映射（`projectBlock`/`replaceSelection`）；`textRanges` 将共用行内样式转换为原生编辑区间，搜索命中（`searchHit`：强调色 + 加粗 + 强调色淡底）覆盖颜色、字重与底色，保留字体、斜体与装饰；文档级撤销事务（`liveEdit` + `editKind`：连续输入合并、粘贴/IME/格式命令独立），`undoLive` 覆盖实时预览与源码模式，恢复文本与选区。 |
| `internal/app/live_selection.go` | 文档级全选（`selectDocWide`/`menuSelectAll`）：跨块选区高亮与复制/剪切/删除/输入在 app 层处理；表格单元格内保留单元格原生全选。 |
| `internal/app/live_history.go` | 大文档撤销差分：64KiB 起将非栈顶的小改动存为克隆的字节差分；整篇替换仍共享原快照，避免反向放大内存。撤销/重做各自栈顶保留完整快照做精确文本校验，用已验证的相邻事务展开下一个栈顶；不靠整文档 hash、不减少 200 条历史、不扩大输入合并范围。差分发布后不可变；弹栈清零底层槽以释放全文引用。撤销历史只属于当前标签的这段编辑：切换到别的标签（含新增标签、删除当前标签）即清空，不按标签保留，也不写盘；删除其他标签或排序不影响当前历史。源码模式的标签会话（`sourceSession`）只记光标与选区。 |
| `internal/app/table_edit.go` | 表格编辑：`createTableOnEnter`、`navigateTable`、`tableEnter`。 |
| `internal/app/table_actions.go` | 表格工具状态 `tableTools` 与行列增删。 |
| `internal/app/table_toolbar.go` | 表格悬浮工具栏。 |
| `internal/app/live_syntax.go` | 块级类型与 `parseBlocks`（测试/预览适配）；行内解析与格式变换见 `internal/markdown`。 |
| `internal/app/view_icons.go` | 内嵌 Lucide 图标（ISC 许可），24×24 viewBox、2px 圆头描边，`ui.MustParseSVG` 生成。 |
| `internal/app/tray_icon.go` / `internal/app/assets/tray.png` / `internal/app/assets/tray-black.png` | 托盘图标：内嵌「墨 Z」手写字母 PNG（`trayPNG`，36×36px，与应用图标配套，白墨/黑墨两版）；macOS 以模板自动适配菜单栏明暗，其他平台系统不管模板，按桌面明暗选墨色并随 `Theme.OnUpdated` 切换，避免图标隐入同色任务栏。`assets/tray.svg` 保留矢量源。 |
| `internal/app/shortcut_fallback.go` | 窗口级快捷键兑底：每帧从统一键位表（命令绑定 + 固定键 + 用户覆盖）注册 `Context.Shortcut`，动作与菜单 Click 同一实现。macOS 菜单 accelerator 在系统层先消费、兑底不触发；Windows 原生 surface 不触发菜单 accelerator，兑底即实际触发路径。编辑角色六键由 ui 引擎内建处理不重复注册。 |
| `internal/app/shortcut_text.go` | 静态快捷键文案的平台转换：locale 字符串以 macOS 符号书写，非 macOS 展示前经 `shortcutText` 转成 Ctrl/Alt/Shift（⌃ 即 mac 标题键映射为 Alt）；动态键位展示走 `keyBinding.displayNameForPlatform`。 |
| `internal/app/font_runtime.go` / `internal/app/font_picker.go` | 字体运行时与选择控件：后台 `ui.FontFamilies()` 枚举本机字体并与远程目录去重（本机字体以 `local:<家族名>` 键入选项，经 `fonts.ClassifyFamily` 按家族名归入中文 / 英文字体，代码字体只列等宽家族）；`fontPicker` 是设置页与向导共用的字体下拉（搜索、推荐 / 本机字体分组、每项样字、限高滚动），键直接绑定，默认字体别名只在显示时对齐（`canonicalFontKey`，空串不触发下载，不能写回）；`prepareFonts` 是唯一准备入口——缺失的可下载字体按固定目录逐个 `fonts.Store.Ensure`（SHA256/字节校验、可取消、进度经存活窗口 `Window.Update` 回主线程）后 `ui.RegisterFont` 私有注册，完成后只回调 `settle(ok)`。字体应用私有注册，不写入操作系统字体库；本机字体卸载后保留偏好，渲染时回退并提示（`usableFontKey`）。老用户升级后首次开窗由 `preparePreferredFontInBackground` 静默补齐已有偏好引用的缺失字体，失败只记日志。向导与设置页共用选项与准备流程；失败重试、返回修改或换选会作废旧批次（generation 门禁）。 |
| `internal/fonts` | 纯逻辑字体资源模块（无 MyGo 依赖）：固定远程目录（LXGW WenKai Lite / JetBrains Mono / Fira Code 5.2，均为官方 GitHub 固定 commit 的原始 TTF，OFL-1.1，见 `catalog.go`）；`Store.Ensure` 先验证缓存（`<应用数据>/fonts/<资源ID>.ttf`，长度+SHA256），命中不触网，缺失才下载：HTTP 200、严格等长、临时文件同目录随机名 0600、Sync+Rename 原子发布，失败/取消不留半截文件；`LocalKey`/`LocalFamily` 校验 `local:` 键防字体栈注入；`ClassifyFamily`（script.go）按家族名推断中文 / 西文 / 其他文字 / 符号，供字体下拉归类。Dank Mono 等商业字体不在目录，仅本机已装时可选。 |
| `internal/app/testdata/LXGWWenKaiLite-Regular.ttf` | 手写字体栈的无头测试夹具（原 `resources/` 内置字体迁入），仅 `go test` 读取，不随包分发；`font_hand_test.go` 用它验证注册路径与字体栈安全回退。 |
| `resources/icon.png` / `resources/icon.svg` | 「墨 Z」应用图标：米色纸上一笔手写 Z 加朱红句点（1024×1024 PNG 与 SVG 源文件），MyGo 构建时生成 macOS ICNS 与 Windows ICO。设计原型保留在 `docs/demo/app-icon/`，不参与构建。 |

## 线程模型（MyGo 硬约束，违反会在运行期出错或被 `go tool mygo vet` 拦下）

- **app 状态只在主线程改**。`view` 每帧在主线程读状态构建 UI；菜单/托盘/热键回调经
  `withWindow` 回主线程改状态并触发重绘；后台完成回调只经原子发布的活窗口 `Window.Update` 投递，窗口关闭后直接丢弃。
- **后台写盘只接触不可变快照**。笔记经 `snapshotFile`、偏好经 `prefs.Store.TakeSnapshot`，
  均在主线程拍值拷贝；后台定时器只写快照，绝不读写 `app` 的可变字段；
  协调状态在 `notes.Store` / `prefs.Store` 内部 mutex 中，偏好另用修订号防旧覆盖新。
- **每帧构建的 UI 元素（`ui.*` 控件）不存入 `app`**。`app` 里只保留窗口/菜单这类桌面集成句柄
  （`*mygo.Window`、`*mygo.Menu`、`*mygo.MenuItem`），普通跨帧状态一律存普通字段。
- **跨线程刷新用 `Window.Update`**。`runOnMain` 只读取原子发布的活窗口并投递；窗口不存在时不执行回调，绝不退回后台 goroutine 直接改 `app` 状态。
- 退出流程（`finishQuit` → `shutdownForQuit`，主线程）：停防抖定时器 → 置 `stopped` → 最终快照保存并等它完成。
  笔记与偏好均成功后才退出（更新器的重启也经过此流程）；任一保存失败则取消退出、恢复笔记 Store 接收保存，
  重开主窗口显示错误，允许继续编辑与重试。

## 持久化生命周期

数据文件：`<数据目录>/zakki.json`（目录见下节）。

### 保存

1. **防抖**：每次改动调 `scheduleSave`，默认 800ms（`Advanced.AutosaveDelayMS`）内没有新改动才落盘（`time.AfterFunc`）；
   防抖到期由 `notes.Store.flushPending` 的单 worker 在后台取走最新快照写盘；忙碌时只保留一个最新排队快照，且须等该快照自身的防抖到期，旧定时回调不能提前触发新快照。
2. **原子写**：`notes.Store.writeFile` 经 `atomicfile.WriteBytes` 先写 `zakki.json.tmp` 再 `os.Rename` 替换；
   rename 失败如实上报，不装作保存成功。目录自动 `MkdirAll`。
   `writeMu` 串行自动/直接/最终保存及写保护切换，保存修订号防止排队的旧直接保存覆盖已接受的新版本（新版本写失败也不会被旧请求掩盖）；状态 `mu` 不覆盖 JSON 编码和磁盘写入；慢盘时 UI 状态查询和提交新快照不等待 I/O。最终保存先拒绝新请求、清理排队快照，再等待在途写入并写最终快照；`shutdownMu` 防止并发退出调用提前报告完成。
3. **写保护**：数据文件损坏、版本高于当前支持，或恢复到这两类文件关联的暂存内容时，
   `notes.Store.saveBlocked = true`；此后写入转到暂存文件 `zakki.json.pending`，绝不覆盖原路径。启动时先验证并恢复
   `.pending`，仍保持写保护；用户在设置页点「写回数据文件」（`confirmWriteBack`）确认后才写回原路径并清理暂存文件
   （写回失败则维持写保护）。

### 备份（唯一目的地：所选备份文件夹，未选时内置 `backups/`；文件名 `zakki-<时间戳>.json`，同秒多次加序号）

备份文件夹是唯一的备份存储地址：选择后每次备份直写该文件夹（云盘同步文件夹即云备份），
不再有第二份本地副本；未选择时写入内置目录（应用数据 `backups/`）。备份用 tmp+rename
原子写，避免云盘同步到半截文件。

备份时间在首帧后后台恢复。`backupMu` 串行一次性时间初始化与备份；周期备份若先到达，
也必须在同一串行区内恢复历史时间再判断节流。初始化不持状态锁、不阻塞快照保存或最终保存，
已完成的新备份不再被迟到扫描覆盖。状态扫描只认本应用备份命名，用户自选文件夹里
混入的其他文件不影响最近备份时间。

| 入口 | 行为 |
|---|---|
| 启动备份 `backupOnLaunch` | 默认每次启动、数据文件存在时写一份；可通过 `backupOnLaunch = false` 关闭。 |
| 自动备份 `maybeWritePeriodicBackup` | 距上次备份超过配置间隔（`Backup.IntervalMinutes`，默认 **5 分钟**，常量 `backupInterval`）才写一份，随防抖保存顺带触发。 |
| 手动备份 `backupNow`（菜单/托盘「立即备份」、⌘S） | **无条件**创建一份（绕过周期节流），先保存当前数据再备份。 |
| 清空当前标签 `clearActiveTab` | 危险操作：**先强制备份**（`backupNow`），备份失败则取消清空。 |

备份直写在备份路径（防抖保存 worker / 启动任务 goroutine）上同步进行，不占主线程；
写入或修剪失败经 `ReportPersist` 如实上报状态栏。

保留策略：按 `Backup.Keep` 修剪当前备份文件夹，默认只保留最新 **20 份**；修剪失败只记警告（备份本体已写成功）。

#### Git 自动化（备份文件夹是 git 仓库时，设置页可开关）

备份文件夹探测到 `.git`（含 worktree 的文件形态，`notes.DirIsGitRepo`；设置页按文件夹缓存
探测结果，切换文件夹或重新同步草稿时才重新 Stat）时，
设置页展示 Git 自动化区块：总开关（`Backup.GitAutoPush`，默认关）与推送间隔
（`Backup.GitPushMinutes`，默认 10 分钟，1–1440）。普通文件夹不出现该区块。

| 步骤 | 行为 |
|---|---|
| 线程 | 备份写入后只在 `mu` 下登记一条 Git 请求（文件夹、备份名、文件夹代号），由 Store 内部单个后台 worker 执行；排队期间新请求覆盖旧请求（commit 的路径集合是整个备份命名空间，合并不丢变更），备份调用（含主线程的 ⌘S、删除前安全备份）不等 git。更换备份文件夹会递增代号并丢弃排队请求，旧文件夹的迟到结果不写入状态。退出不等待 git（推送可能长达超时上限），未完成的提交在下次备份时补上；`WaitGitIdle` 供测试等待 |
| 提交（随备份） | 每次实际备份写入并修剪后，由 Git worker 把备份文件的变更 commit（`add -- <备份文件>` → `status --porcelain` 无变更则跳过 → `commit --only -m "backup: zakki-<时间戳>"`）；只提交本应用备份命名空间（含旧前缀与修剪产生的删除），不碰仓库里用户的其他文件 |
| 推送（按间隔节流） | 距上次推送**尝试**超过 `Backup.GitPushMinutes` 才 `git push`；失败也计一次尝试，避免无网时每次备份硬撞；提交失败不推送，等下次备份重试 |
| 实现 | 调系统 git CLI（`notes.DefaultGitRunner`，单条命令超时 1 分钟，超时后最多再等 5 秒回收子进程管道），找不到可直接运行的 git 时返回 `ErrGitUnavailable`：macOS 的 `/usr/bin/git` 只是垫片，未装命令行工具时运行会弹系统安装对话框，所以先经 `xcode-select -p` 确认开发者目录里有 git；不可用时静默跳过（不记失败，`GitMissing` 供设置页提示一句），装好后下一次备份自动恢复。以非交互方式运行（`GIT_TERMINAL_PROMPT=0`、`GCM_INTERACTIVE=never`：凭据缺失直接失败并上报，不挂起等输入），认证复用用户既有 ssh-agent/凭据配置；不做 pull/rebase/force push，远端分叉的推送失败如实记录 |
| 状态 | 最近提交/推送时间与最近错误在 `Store` 内部锁下发布（`GitCommitTime`/`GitPushTime`/`GitError`），设置页卡片状态行展示；更换备份文件夹或关闭开关时状态作废/清错误 |

### 损坏隔离与恢复（`load` → `recoverFromBrokenFile`）

- 文件不存在：首次运行，写入欢迎文本。
- 解析失败 / 读取失败 / 版本过高：原件**复制隔离**为 `zakki.json.corrupt-<时间戳>`
  （`notes.Store.QuarantineCorruptFile`，绝不删除或覆盖原件）→ 从备份文件夹按时间戳及同秒序号数值由新到旧找第一个可用备份恢复
  （`notes.Store.LoadNewestBackup`；所选文件夹没有时回退内置目录，照顾历史版本的存量备份）→ 都失败则用默认值并进入写保护（见上）。
- 即使已从备份恢复，`load` 也如实返回错误，状态栏/日志给出提示（`notice`/`slog`）。

### 版本迁移

- `currentDataVersion = 3`（`notes.CurrentDataVersion`）。磁盘 `version` 低于 2 且没有 `tabs` 时，
  `migrateSavedFile`（包装 `notes.Migrate`）把旧 `dots` 迁成标签：补齐缺省标签、收敛 `active`、
  空 `themeSource` 按 `system`、旧字段 `preview` 并入 `source`；
  `version` 低于 3 时排版主题旧 ID 随载入映射为 `note`（字面量见 `notes` 包常量）。
- `version` 高于当前支持：返回 `versionError`，**绝不覆盖原文件**（走损坏隔离流程，
  暂存 .pending 等用户确认）。

### hideOnEscape 缺键默认 true

设置里其他项默认值都是 false/空串，唯独 `hideOnEscape` 默认 **true**。JSON 的 bool 分不清
「缺键」与显式 `false`，故 `hasHideOnEscapeKey` 用 `map[string]json.RawMessage` 探测键是否存在：
缺键（含 settings 整体缺失）按 true 处理，显式 false 保留。

## 数据文件格式（`savedFile`）

```json
{
  "version": 3,
  "active": 0,
  "tabs": [{ "id": 1, "text": "…", "color": "#…" }],
  "settings": {
    "themeSource": "system",
    "alwaysOnTop": false,
    "hideOnEscape": true,
    "source": false,
    "tabMotionOff": false,
    "tabDotShape": "dot"
  }
}
```

| 字段 | 类型 | 含义 |
|---|---|---|
| `version` | int | 数据版本，当前 `currentDataVersion = 3`；0/1 视为固定 7 点格式；更高拒载不覆盖 |
| `active` | int | 当前激活的标签下标（载入时按标签数 clamp） |
| `tabs` | [{id,name,text,color}] | 动态标签（1–9 个）：稳定 ID、可选名称、markdown 纯文本、颜色（空/非法回退调色板）；v1 的 `dots` 字段仅为读取保留，新保存不写 |
| `settings.themeSource` | string | `system` \| `light` \| `dark`，空串按 `system` |
| `settings.markdownTheme` | string | 空 / `sonnet` 为 Sonnet，`note` 为便签（版本 2 文件里的更名前旧 ID 随载入迁移到新 ID） |
| `settings.bodyFont` / `settings.monoFont` | string | 正文 / 代码字体预设，空为主题默认 |
| `settings.bodySize` / `settings.lineHeight` | float32 | 正文字号 / 行高，0 为主题默认 |
| `settings.alwaysOnTop` | bool | 窗口置顶 |
| `settings.hideOnEscape` | bool | 按 Esc 隐藏窗口，**缺键默认 true**（见上） |
| `settings.source` | bool | 源码模式（false = Typora 式实时预览） |
| `settings.preview` | bool | 旧版字段，仅读取时并入 `source`，不再写出新值 |

## 偏好生命周期

`Run` 先调用 `resolveDataDir()` 完成目录迁移；成功后才把同一个显式目录传给 `newAppAt` 构造 Store，失败则不构造 Store，在 `WhenReady` 显示原生错误后退出。随后 `load` 恢复笔记数据（含编辑会话状态）→ `loadPreferences`
读取 `preferences.json`（缺失时执行一次性迁移：默认值 → 笔记内旧设置 →
旧 config.toml / config.json 显式值，TOML 优先；结果原子写入，旧文件原样保留）→
`WhenReady` 里创建窗口、应用主题并注册全局热键（`a.ready` 之后偏好变更才触达原生 API），
目录状态恢复与启动备份等到首个窗口的 `OnReadyToShow` 后分别在后台执行。
运行期所有偏好变更走 `updatePreferences` 统一入口：校验 → 应用运行时 → 同步菜单/窗口/热键 →
防抖自动保存；磁盘写经 `prefs.Store.WriteSnapshot` 内部的 `writeMu` 串行化、tmp/rename 原子替换，修订号防旧快照覆盖新值。
`preferences.json` 损坏或版本超前时进入待恢复状态：不退回旧配置、不覆盖原文件，
设置页提供「另存原件并恢复默认」；旧配置损坏时保留原件并提示，
「使用已有偏好继续」后以当前内存偏好建立新文件。
偏好版本当前为 **11**（`prefs.CurrentVersion`）：版本 11 把默认窗口尺寸改存每行字数 × 行数；版本 10 新增整体风格 `appearance.style`（手绘 + 手账整套迁为手账风格，见 configuration.md）；版本 9 补 Git 自动化默认。版本 8 仅扩展字体键（三个字体字段允许 `local:<真实家族名>`，`latinFont` / `monoFont` 另增 `fira`；不新增字段、不改默认、不触发下载）。版本 2 起 `appearance` 分开记录中文字体
`bodyFont` 与西文字体 `latinFont`（空串 = 跟随中文字体）；版本 3 起 `backup` 记录
外部备份目的地 `mirrorDir`；版本 4 把更名前的主题 ID 映射为 `classic` / `note`；
版本 5 起代码块配色默认「柔和」，旧文件补默认，`codeTheme` 始终落盘（空串 = 显式跟随文本）；
版本 6 起备份只有一个文件夹：`backup.mirrorDir` 更名为 `backup.backupDir` 并成为唯一备份地址，
`gitMode` 随 Git 自动化移除，旧键由 `Decode` 在严格解码前摘除后平移；
版本 7 起 `appearance.codeIndent` 只有 `space2` / `space4` 两档（默认 2 空格），旧文件从未写过该键，升级时补默认。
旧版本文件在 `Decode` 里升级，缺键保持升级前行为；写回后即为版本 8。旧版本程序读到版本 8 会按
「版本超前」拒绝并保留原文件。
退出流程（`shutdownForQuit`）在笔记数据最终保存后等待偏好最终保存（`shutdownPreferences`），
失败返回 `finishQuit`，阻止退出或更新重启。
完整契约见 [偏好与设置](configuration.md)。MyGo 更新器另外管理 `updater.json`（检查时间、跳过版本、自动下载选择），
不改变应用偏好的 JSON 契约。

## 测试接缝

- **路径注入**：生产 `Run` 只调用一次 `resolveDataDir()`；迁移成功后把结果同时传给 `mygo.App.SetPath(PathUserData, dir)` 与 `newAppAt(dir)`，由此构造 `notes.Store` 的 `DataFile` / `BackupDir`。迁移失败不会构造 Store。系统数据根目录由 `mygoAppDataBase` 解析（macOS 为 `~/Library/Application Support/`，Windows 为 `%APPDATA%`），`datamigrate.go` 用 staging 复制、校验并发布旧目录内容，源文件保留供重试或人工确认。
- **时钟注入**：`app.now func() time.Time`，生产 `time.Now`，测试给假时钟。
  创建后 `now` 只读，后台可安全读取。
- **`newTestApp(t)`**（`main_test.go`）：注入 `t.TempDir()` 与固定时钟（2024-06-01），
  **绝不写真实用户目录**。保存、备份与写保护在 `internal/notes`；`persistence_test.go` 覆盖载入映射与清空标签。
- **原生生命周期回归**：`ZAKKI_NATIVE_TEST=1 MYGO_LOCALE=zh-Hans go test ./internal/app -run '^TestNativeDockReopen$' -count=1 -v` 在 macOS 图形会话中构建独立 bundle、使用临时数据目录，验证关闭重建、隐藏复用、最小化恢复和可见时不重复开窗。默认 `MYGO_GPU=0` 隔离驱动初始化；显式 `MYGO_GPU=1` 可复测 Metal 路径，耗时与稳定性受驱动影响。
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
