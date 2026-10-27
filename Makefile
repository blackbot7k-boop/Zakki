# Zakki 本地/CI 统一检查入口。Makefile 必须用 tab 缩进，勿改空格。

# 只检查仓库源码；vendor 保留第三方源码格式，补丁单独维护。
GO_FILES := $(shell find . -name '*.go' -not -path './build/*' -not -path './.mygo/*' -not -path './vendor/*')

# 纯逻辑包 → 允许依赖的 internal 包（空格分隔，空表示无）。
# 这些包禁止依赖 MyGo，也禁止反向依赖 internal/app（UI 层）。
ARCH_RULES := \
	markdown: \
	codehl: \
	atomicfile: \
	fonts: \
	sysnotify: \
	notes:atomicfile \
	prefs:atomicfile,fonts

.PHONY: fmt-check vet test race arch-check check build build-windows vendor

fmt-check:
	@echo "==> gofmt 检查：仓库源码（排除 build/、.mygo/ 与 vendor/）"
	@out="$$(gofmt -l $(GO_FILES))"; \
	if [ -n "$$out" ]; then echo "未通过 gofmt 的文件:"; echo "$$out"; exit 1; fi
	@echo "    gofmt 通过"

vet:
	@echo "==> go vet：Go 标准静态检查"
	go vet ./...
	@echo "==> mygo vet：MyGo 原生 UI 生命周期静态检查"
	go tool mygo vet .

test:
	@echo "==> go test：全量测试（CGO 关闭，-count=1 不用缓存）"
	CGO_ENABLED=0 go test ./... -count=1
	CGO_ENABLED=0 go test ./vendor/github.com/egoist/mygo/ui -count=1

race:
	@echo "==> go test -race：竞态检测"
	go test -race ./... -count=1
	go test -race ./vendor/github.com/egoist/mygo/ui -count=1

arch-check:
	@echo "==> arch-check：纯逻辑包不得依赖 MyGo 与 internal/app，且只允许约定的 internal 依赖"
	@fail=0; for rule in $(ARCH_RULES); do \
		pkg=$${rule%%:*}; allow=$${rule#*:}; dir=./internal/$$pkg; \
		[ -d "$$dir" ] || continue; \
		if go list -deps "$$dir" | grep -q '^github.com/egoist/mygo'; then \
			echo "  internal/$$pkg 依赖了 MyGo"; fail=1; fi; \
		for dep in $$(go list -f '{{join .Imports "\n"}}' "$$dir" | grep '^zakki/internal/' | sed 's|zakki/internal/||'); do \
			case ",$$allow," in *",$$dep,"*) ;; *) echo "  internal/$$pkg 不允许依赖 internal/$$dep"; fail=1;; esac; \
		done; \
	done; for dep in $$(go list -f '{{join .Imports "\n"}}' ./internal/app | grep '^zakki/internal/' | sed 's|zakki/internal/||'); do \
		case " markdown codehl atomicfile notes prefs fonts sysnotify " in *" $$dep "*) ;; *) echo "  internal/app 依赖了非约定包 internal/$$dep"; fail=1;; esac; \
	done; [ $$fail -eq 0 ] && echo "    arch-check 通过"

check: fmt-check vet test race arch-check
	@echo "==> make check 全部通过"

vendor:
	go mod vendor
	git apply patches/mygo-text-range-background.patch
	git apply patches/mygo-textinput-objects.patch
	git apply patches/mygo-shortcut-claimed-keys.patch
	git apply patches/mygo-calendar-spacing.patch

build:
	@echo "==> go tool mygo build：打包通用 .app / .dmg 到 build/darwin-universal/"
	go tool mygo build -platform darwin/universal

build-windows:
	@echo "==> go tool mygo build：打包 Windows x64 到 build/windows-amd64/（安装器需要 NSIS）"
	go tool mygo build -platform windows/amd64
