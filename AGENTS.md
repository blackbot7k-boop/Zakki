# AGENTS.md

Tot MD：基于 [MyGo](https://github.com/egoist/mygo) 原生 UI 的 macOS Markdown 便签应用，纯 Go 单二进制。

## 必读

1. [docs/engineering-standard.md](docs/engineering-standard.md)：工程与架构标准、目标布局、依赖规则、迁移阶段。
2. [docs/architecture.md](docs/architecture.md)：各模块职责、线程模型、持久化生命周期。
3. [.agents/skills/mygo-maintenance/SKILL.md](.agents/skills/mygo-maintenance/SKILL.md)：改 UI 前必读。
4. [docs/configuration.md](docs/configuration.md)：改偏好/设置相关代码前必读。

## 布局

- 根目录 `package main`：MyGo UI 与应用装配（入口、菜单、托盘、视图、实时预览、标签、设置）。
- `internal/`：纯逻辑与持久化（`markdown`、`notes`、`prefs`、`legacyconfig`、`atomicfile`）。
- 当前处于迁移期，进度与验收标准见标准第 7 节。只做被分配的阶段，不要顺手搬动其他代码。

## 硬规则

- `internal/*` 禁止 import `github.com/egoist/mygo` 及其子包；internal 包之间只允许标准第 3 节列出的依赖。
- 禁止往 `app` 顶层新增字段；新状态放进对应的领域子结构体或 Store。
- 禁止把 `ui.Element`、`*ui.Context` 存入状态或传给后台；持久身份用 `ui.Handle`。
- 后台 goroutine 只碰不可变快照，回主线程只用 `Window.Update`；锁只存在于 Store 类型内部。
- 不改 `tot-md.json`、`preferences.json` 的格式与迁移语义，除非任务明确要求（需递增版本并补迁移测试）。
- 持久化失败必须如实上报，不得吞掉错误。
- 测试绝不写真实用户目录：用 `newTestApp(t)` / `t.TempDir()` 和注入的时钟。
- 结构迁移不夹带行为变更；测试只能迁移，不能删除或放宽。
- 未经用户允许不做 lint 类修改。
- 文件迁移或职责变化时，在同一次提交里更新 `docs/architecture.md`。
- 非测试文件超过 800 行必须拆分；禁止 `util`/`common`/`helpers` 包名。
- 注释用中文，只写代码本身表达不出的约束。

## 命令

```bash
make check          # 提交前必须全绿：fmt-check + vet（含 mygo vet）+ test + race（阶段 1 起含 arch-check）
make test           # CGO_ENABLED=0 go test ./... -count=1
make race           # go test -race ./... -count=1
make vet            # go vet ./... && go tool mygo vet .
make build          # go tool mygo build，产物在 build/
go list -m -f '{{.Dir}}' github.com/egoist/mygo   # 查看 MyGo 源码与示例，API 以该版本源码为准
```

## 提交

- 中文 commitlint：`git commit -m "<type>: <描述>"`，type 取 `feat` / `fix` / `refactor` / `style` / `docs` / `chore` / `test` / `perf`。
- 一次提交只做一件事；纯结构迁移用 `refactor:`。
- 禁止提交 `build/`、`.mygo/`、根目录二进制 `tot-md-mygo`。
