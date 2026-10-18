# MyGo 依赖补丁

本仓库保留 `go.mod` 锁定的 MyGo v0.5.1，并通过 Go 标准 `vendor/` 机制携带最小补丁。
`vendor/` 包含构建所需依赖及其许可证；普通 `go test`、`go build` 和 `go tool mygo` 自动使用它。
不要用 `-mod=mod` 绕过补丁。

## 输入区间样式

`mygo-text-range-background.patch` 为 `ui.TextRange` 补齐实时预览实际使用的属性：

- `Background`：粗体荧光底色与行内代码底色。
- `Font`、`Italic`：局部等宽字体与混排斜体。
- `Underline`、`WavyUnderline`、`DecorationColor`：普通链接下划线与手账波浪线。

非零属性独立合并，重叠时各取最后一个有效值；布尔属性与 RichText 一样只开启样式。
字体、斜体与字重进入布局和 shaping key；合并缓存区间时比较完整布局属性。
背景、文字颜色与装饰只参与绘制，改变或拆分这些属性不改变 shaping key。
复用原生输入框的 `spanPaint`：底色 → 选区 → 文字与装饰，换行、字体回退、双向文字、滚动沿用控件的真实几何。
IME 组合文本保持无区间样式，原区间在其两侧拆分／移动，确认或取消后恢复。

应用层在 `internal/app/live_inline_style.go` 统一预览和编辑的样式定义。
标题装饰仍由两态共用同一容器；列表固定内容文字颜色，避免恢复窗口主题时覆盖继承。
应用回归在 `internal/app/view_live_focus_test.go`、`view_sketch_focus_test.go` 与 `mygo_text_background_test.go`，
覆盖像素一致、墨色、换行、光标边界、表格、选区及 IME。
底层合并、布局 key 与组合区间测试在 vendor 的 `ui/textinput_background_test.go` 和 `ui/textinput_styles_test.go`；
`make test` / `make race` 会显式执行这个包（普通 `go test ./...` 不遍历 vendor）。

## 刷新与升级

```sh
make vendor       # go mod vendor 后重新应用补丁；冲突时直接失败，需人工适配
make check
```

不要单独执行 `go mod vendor` 后提交，否则会丢失补丁。升级 MyGo 后，先检查上游是否已支持
对应原生区间属性；若接口和行为等价，移除补丁并保留回归测试，再评估是否仍需提交 `vendor/`。
补丁只修改 `ui/textinput.go`、`ui/editor_layout.go` 及同步后的 `ui/api_gen.go` 文档，
另附上述两个底层测试文件；不批量改第三方格式。
