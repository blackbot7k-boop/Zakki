# Tot MD 发布清单

## 版本号

- **唯一来源**：`mygo.json` 的 `version` 字段（当前 `0.1.0`）。
  打包产物的 dmg 名（`Tot MD <version> <arch>.dmg`）从这里取，不手写第二处版本号。
- **git tag 对齐**：发布时打同号 tag（如 `v0.1.0`），tag 与 `mygo.json` 必须一致。

## 发布前检查

1. 全量测试为准，不做测试排除：`make check` 必须全绿
   （`fmt-check` → `vet`（含 `go tool mygo vet .`）→ `test` → `race`）。
2. `go mod verify` 通过（CI 同样跑）。
3. 更新 README 需要同步的段落（见下）。

## 打包

```sh
go tool mygo build    # 打包 .app / .dmg 到 build/
```

- 产物按平台-架构分目录：`build/darwin-amd64/`、`build/darwin-arm64/`，
  内含 `Tot MD.app` 与 `Tot MD <version> <arch>.dmg`。
- **darwin/amd64 与 arm64**：MyGo 无 cgo，任一平台可交叉编译另一架构；
  需要双架构时分别构建，或用 `darwin/universal` 合并（此时 dmg 不带架构名）。
  当前开发机（x86_64）默认产出 `build/darwin-amd64/`。

## 代码签名 / 公证（未配置）

- **当前未配置**：`mygo.json` 尚无 `macos.signingIdentity` / `macos.notarize`，
  产物是未做 Developer ID 签名与公证的本地构建。
- 待办（发布对外版本前完成）：
  - [ ] 在 `mygo.json` 配置 `macos.signingIdentity`（Developer ID），或 `go tool mygo build -sign …`
  - [ ] 配置 `macos.notarize` 公证 dmg
  - [ ] 在干净账户上验证 Gatekeeper 打开无告警

## 产物 checksum

对每个 dmg 出校验和，随发布附上：

```sh
shasum -a 256 "build/darwin-amd64/Tot MD 0.1.0 amd64.dmg"
```

## README 更新点

每次发布检查 README 与现状一致：

- 功能表：新增/变更的功能行（含备份行为、顶栏/窗口行为）。
- 「开发」一节：命令列表（`make check` / `go test` / `go tool mygo …`）与实际 Makefile 一致。
- 「数据」一节：数据目录（macOS `~/Library/Application Support/Tot MD/`，
  其他平台遵循系统用户配置目录）与备份策略（启动备份、5 分钟滚动备份、
  手动立即备份无条件创建、清空当前点前强制备份、保留 20 份）。
- 版本号不写死在 README（以 `mygo.json` 为准）。
