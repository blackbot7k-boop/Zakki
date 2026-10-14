# Typora 主题迁移与表格编辑（Sonnet / 表格自动创建）

两条需求：借用本机 Typora 的主题观感；输入 `|A|B|C|` 回车自动建表。两者都已完成实现。

## 主题（Sonnet / Sonnet Dark）

**结论：Typora 的 CSS 不能直接加载**——Tot MD 是 MyGo 原生 Go UI（无 DOM/webview/CSS 引擎），
但可以把 `sonnet.css` / `sonnet-dark.css` 的视觉参数移植进原生代码：

- 颜色 token 复刻 `:root` 变量：浅色 `#faf9f5` 底 / `#141413` 字 / `#cc785c` 强调，
  深色 `#262624` / `#faf9f5` / `#d9916f`；行内代码、引用条、分隔线、代码块底色同源。
- 排版复刻：正文字号 16 / 行高 1.6；标题字号 28/24/20/17/15、代码块 14。
- 字体直接复用本机已装的「Sonnet 主题同款栈」：`JetBrains Mono, LXGW WenKai...`
  （MyGo 的 `Font()` 支持逗号栈并按序回退）。
- 实现位置：`markdown_theme.go`（`sonnetMarkdownTheme` + `markdownStyle`），
  在 `view.go` 中只作用于内容区（编辑器 + 实时预览），托盘顶栏/底栏保持 Tot 原色。
- 菜单：显示 → **Markdown 主题** → Sonnet（Typora，默认）/ Tot 点配色（旧行为）。
  持久化在 `tots-md.json` 的 `settings.markdownTheme`。
- 不能移植的部分：`#write`、`md-meta`/`md-expand`「标记常驻+CSS 显隐」行内混排、
  CodeMirror 专属规则、Typora 菜单/侧栏样式——原生 UI 结构不同，无对应 DOM。

## 表格编辑（Typora 既有行为的移植）

以此前 Typora 取证结论为准：其块树文档模型 + Enter=insertParagraph。
本项目在原生 UI 下的等价实现（`table_edit.go`，接入 `live_edit.go` 输入管线）：

| 行为 | 触发 | 说明 |
|---|---|---|
| 自动建表 | 输入 `\|A\|B\|C\|`（含首尾竖线、≥2 格）回车 | 补分隔行 + 空数据行，光标进入首个数据单元格 |
| 立即撤销 | 建表后 ⌘Z 或一次退格 | 恢复原输入行，金钱可继续打字 |
| 单元格导航 | Tab / Shift+Tab | 依次选各格；末格 Tab 自动加数据行 |
| 表格内回车 | header 上回车跳到首数据格；数据行回车后追加同行列宽新行 | 与 Typora 行为一致 |
| 空表尾回车 | 光标在空数据行回车 | 删掉该空行退出表格，回到普通段落 |
| 表格渲染 | 非焦点块显示网格 | 首行强调底色，支持 `:---:/---:` 对齐（左/右/中）|
| 防误判 | 不满足首尾竖线/分隔行/≥2 格 | 保持普通段落分段；代码块内竖线不受影响 |

### 关键约束（本仓库输入管线）

- `focusBlock` 会先 `commitDraft` 再切块：程序化写入 `a.dots` 后必须先过一帧
  再定位光标，否则空草稿会把文档覆盖。
- `applyDocEdit` 在「焦点块索引未变且内容一致」时早退跳过选区同步，
  因此表格这类块级转换后必须显式设置 `selSyncStart/End`。
- 菜单的 edit command（菜单项 undo）不经过 `HandleInput` 的 `InputCommand`；
  撤销检测用 `KeyDown` 键事件（macOS `Super+Z`）。
- 撤销锚点：`autoTableBefore`（转换前文档）+ `autoTableLine/Col`（原光标位置），
  立即退格/⌘Z 只在光标未动过时触发。

### 测试

- `table_test.go`：建表/撤销（⌘Z、退格）/Tab 导航/末格加行/误判防护共 5 组。
- 既有回归按新语义更新：`TestLiveEnterBackspaceRoundTrip` 用 `a|b|c`
  （无首尾竖线）验证普通分段往返；`TestEnterMovesOneLine` 改用中文质点文本。
