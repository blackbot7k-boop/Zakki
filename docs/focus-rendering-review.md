# 聚焦前后文字外观差异：根因核查

最初核查复现了用户截图中的混排斜体、链接装饰差异，并发现行内代码字体与标题颜色也存在独立问题。
以下保留修复前的根因与验证证据；正式修复及当前验收结果见文末「修复结果」。

## 渲染路径

`internal/app/view_live.go` 的 `livePane` 按 `mdActive` 选择控件：

```text
同一份 Markdown / liveProjection
  ├─ 非活动块 → renderLiveBlock
  │               ├─ 段落、标题、待办、表格：liveProjection.content → Text / RichText
  │               └─ 引用、列表：renderInline → RichText 的行内子元素
  └─ 活动块   → activeBlock → activeText → TextAreaBase + TextRanges
```

切换的是整个块的渲染路径。同一活动块内，将光标移到样式区间的起点、中间、终点或块首，测试均未发现光标之外的文字变化；切走后原预览恢复。
本轮复现未涉及 IME 或文字编辑，正文始终相同，因此能排除编辑写回、隐藏标记被改写等干扰。

## 根因一：预览样式在转换为输入区间时丢失

预览的 `liveProjection.content`（`live_projection.go`）和 `renderInline`（`view_live.go`）分别维护样式映射；编辑的 `textRanges` 又维护一份。
三处没有完整共享样式定义。当前输入 API 本身也只能表达其中一部分：

| 属性 | 预览 | 编辑 | 影响 |
|---|---|---|---|
| 字重 | Span.Weight / Bold | TextRange.Weight | 已保留 |
| 粗体、代码底色 | Span.Background / TextBackground | 上轮新增 TextRange.Background | 已保留，本轮普通段落对照无底色丢失 |
| 局部斜体 | Span.Italic / Italic | 无对应 TextRange 属性 | 混排斜体变正体 |
| 行内代码字体 | Span.Font / Font("monospace") | 无对应 TextRange 属性 | 回退为整块正文的字体，字形、宽度与换行可能改变 |
| 链接下划线 | Underline；手账为 WavyUnderline + DecorationColor | 仅传 Color | 链接只剩颜色，装饰消失 |

`vendor/github.com/egoist/mygo/ui/textinput.go` 的 `TextRange` 目前只有 `Start/End`、`Color`、`Background`、`Weight`。
`editor_layout.go` 的 `spansOf` 同样只向 shaping / paint 转换这些属性。
因此，仅在应用层补一个斜体或链接分支仍不足以传递局部样式。

`activeText` 还对“整个可见块恰好只有一个斜体区间”调用整框 `Italic()`。这解释了整段斜体正常、夹在普通文字中的斜体失败的表面不一致。
它是整框补偿，无法表达同一块中的不同字体或局部装饰。

底层 `text.Span` 已能表达 Family、Italic 等布局属性；`spanPaint` 已能绘制下划线与波浪线。
修复可以沿原生输入布局扩展，保留原生选区、光标和组合文本几何。

## 根因二：内容主题的默认文字颜色被后续 SetTheme 覆盖

`internal/app/view.go` 在内容区调用 `c.SetTheme(&contentTheme)`，构建底栏前又调用 `c.SetTheme(&t)`。
MyGo `ui/context.go` 的 `SetTheme` 会直接改写 `root.ts.color`，不是创建隔离的主题作用域。
未显式设置文字颜色的元素在布局／绘制时沿祖先继承颜色，不会保存创建时的默认主题颜色。

标题预览在 `view_sketch.go` 的 `sketchHeading` 显式调用 `.TextColor(t.Text)`；标题编辑仅调用 `styleHeading`，后者设置字体、字号、行高和字重，没有设置颜色。
因此在本轮浅色手账测试中：

- 标题预览文字：`#2d2a26`，手账墨色。
- 标题编辑文字：`#18181b`，恢复后的默认前景。
- 两态文字矩形、深色字形边界及标题底色均相同，差异来自前景颜色。

普通正文的预览和编辑都未固定正文颜色，所以两态可以“相同但同时偏离所选颜色主题”。
只做两态相等比较仍会漏掉这个问题，还需要检查是否等于主题的预期颜色。

在临时 overlay 中，只给 `livePane` 内容列表添加 `.TextColor(style.text)`，中英文普通标题的像素差异即降为 0；正文两态仍一致。
这是验证内容区需要稳定继承源的证据。

## 可重复证据

测试使用 `newTestApp(t)` 隔离数据，调用真实 `a.view`，固定手账颜色／排版和手写字体。
宽窗口按固定内容区域比较 PNG 像素，排除光标矩形；另用窄窗口比较后续段落的 Y 坐标。
两轮重复得到相同的差异值。

| 单段场景 | 聚焦前后差异像素 |
|---|---:|
| 普通正文 | 0 |
| 粗体及底色 | 0 |
| 整段只有斜体 | 0 |
| 普通文字中夹斜体 | 360 |
| 链接 | 77 |
| 英文行内代码 `code iii WWW` | 2532 |
| 截图中的四种样式组合 | 806 |
| 普通中文一级标题 | 1214 |
| 普通英文一级标题 | 812 |

这些数值仅描述本机固定字体、窗口和截图区域，不是跨平台阈值。

代码宽度的最小复现：窗口宽 280，正文为：

```markdown
支持 `code iii WWW `，尾文。

后续段落
```

仅切入首块，后续段落从 Y=154 移到 Y=126，上移 28，等于一行。
对 45 组宽度／代码长度组合扫描，12 组发生该类高度变化；临时让代码预览继承正文的字体后，这 45 组均无高度变化。

单变量对照结果：

- 仅在临时预览中取消局部斜体：斜体用例差异 360 → 0。
- 仅取消链接波浪线：链接用例 77 → 0。
- 仅取消代码的 monospace 字体：代码用例 2532 → 0，换行跳动消失。
- 仅给内容列表固定正文颜色：中英文普通标题差异均 → 0。

前三项通过使预览退化到输入当前支持的效果来确定缺失属性，不能作为产品修复。
正式修复需要保留预览效果，并补齐编辑能力。

手账／Sonnet、浅／深色的范围扫描还观察到段落、引用、列表和待办中的同类样式差异。
表格聚焦会出现工具栏、引起结构位移，因此未把其整图像素差当作样式丢失的定量证据；其单元格编辑复用同一 `activeText`／`textRanges` 缺失路径。

## 上轮测试为何仍然通过

`view_sketch_focus_test.go` 的 `TestSketchFocusPreservesDecorations` 统计目标底色像素，并允许编辑态保留至少预览态的 80%。
它没有混排斜体和链接用例，也不检查字形、字体、文字颜色或后续块位置。
代码底色仍在，即使代码字体变化，该类测试仍可能通过。

本轮重新运行现有三组 Sketch 回归全部通过，同时新的外观／换行诊断稳定失败。
这表明需要补齐断言范围，现有测试通过不能证明全部外观一致。

## 修复边界与验收

1. **固定内容区继承样式。** 为内容容器显式绑定解析后的正文颜色与字体；保留引用、代码、链接等显式覆盖。断言颜色符合主题值，同时检查聚焦前后相同。
2. **共享行内样式定义。** 将四类行内格式解析成一套样式属性，再分别转换为预览 Span 和输入 TextRange；让普通段落、列表、引用、表格复用相同定义。
3. **补齐原生输入区间能力。** 添加当前预览实际使用的局部 Font、Italic、Underline／WavyUnderline、DecorationColor。同步区间合并、IME 拆分、shape / paint 转换。Font／Italic 进入布局与缓存键；颜色和装饰只进入绘制。缓存规范化必须比较完整布局属性。
4. **用完整视觉契约验收。** 覆盖混排斜体、整段斜体、中文／英文代码、普通／波浪链接、各级标题文字颜色，比较文字几何、样式、换行和后续块位置。额外覆盖浅深色、主题、字体、表格、选区、滚动及 IME；继续保留上一轮底色和输入回归。

本报告初次核查时只新增文档，没有修改生产源码。上述诊断通过临时 Go overlay 执行，图像、代码和日志均在 `/tmp/zakki-focus-analysis/`。
未做原生桌面窗口操作；渲染证据来自 MyGo Tester 和用户提供的截图。

当前会话可重放的失败命令：

```sh
CGO_ENABLED=0 go test -overlay /tmp/zakki-focus-analysis/overlay.json ./internal/app \
  -run '^Test(SketchFocusAppearanceProbe|FocusInlineCodeWrapParityProbe)$' -count=1 -v
```

日志：`final-repro.log` 记录原始缺陷；`final-counterfactual.log` 记录各原因在临时对照中消除后的结果。
`*-comparison.png` 从上到下为预览、编辑、红色差异像素（最后一行包含光标，量化统计另行排除了它）。
临时目录不会随仓库长期保存。

## 修复结果

生产代码现已补齐两条路径：

- `livePane` 列表显式设置解析后的正文颜色，标题编辑与正文都从稳定的内容容器继承墨色；保留引用、链接和代码的显式颜色覆盖。
- `live_inline_style.go` 的 `inlineStyle` 统一定义粗体、斜体、代码和链接样式，投影预览、列表／引用与输入区间共用；移除整段斜体的临时补偿。
- 原生 `TextRange` 增加 `Font`、`Italic`、`Underline`、`WavyUnderline`、`DecorationColor`，沿用逐属性合并与 IME 区间拆分。布局 key 比较完整字体、斜体与字重；背景、颜色和装饰不改变 key。补丁与上游 v0.5.1 的差异保存在 `patches/`，`make vendor` 重放后 5 个文件逐字节一致。

正式回归在 `internal/app/view_live_focus_test.go`：

| 范围 | 验收结果 |
|---|---|
| 手账／Sonnet × 浅深色 × 默认／手写字体 × 17 类内容 | 136 组预览／编辑像素比较均为 0（排除光标）；块内移动、切走恢复及后续段落位置保持一致 |
| 3 个窄窗口宽度 × 3 种代码文本 | 9 组均不再因聚焦改变换行或后续段落位置 |
| 正文与 1–6 级标题 × 浅深色 | 14 组两态均使用主题预期墨色 |
| 表头／正文 × 左中右对齐 × 两种排版 | 12 组网格内单元格样式一致；独立渲染真实网格以固定工具栏位移 |
| 局部斜体／代码／链接 | IME 取消后图像恢复，输入／撤销及带格式选区替换保留 Markdown 包装 |

原始诊断 overlay 在修复后重放通过：十个截图场景的聚焦差异均为 0，窄窗复现也已通过。
图像在 `/tmp/zakki-focus-analysis/fixed/`，日志在 `/tmp/zakki-focus-original-replay.log`；长期验收以仓库内正式测试为准。
本轮渲染验证使用 MyGo Tester，没有操作真实桌面窗口；macOS 与 Windows amd64 构建均通过。
`make check` 全部通过，包含 gofmt、Go／MyGo vet、全量测试、vendor 补丁测试、两者的 race 检查及架构依赖校验。

