# auto-finance-assistant Makefile —— 仿 go-rmm 风格
#
# 用法：
#   make tidy        —— 同步 Go 依赖
#   make web-deps    —— 安装前端依赖（pnpm 或 npm）
#   make web-build   —— 构建 Vue 前端到 internal/web/dist
#   make build       —— 构建单 EXE（先前端后 Go，嵌入 dist）
#   make run         —— 前台运行（默认 config.yaml）
#   make run-dev     —— 前台运行（config.dev.yaml，CPU 验证）
#   make test        —— 运行测试
#   make vet         —— 静态检查
#   make fmt         —— 格式化
#   make clean       —— 清理 bin/ 与前端 dist/

GO        ?= go
BIN_DIR   := bin
WEB_DIR   := web
EXT       := $(shell go env GOOS | grep -q windows && echo .exe)
VERSION   := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS   := -X main.version=$(VERSION)
NPM       := $(shell command -v pnpm 2>/dev/null || echo npm)

.PHONY: tidy web-deps web-build build run run-dev test vet fmt clean

tidy:
	$(GO) mod tidy

## 安装前端依赖
web-deps:
	cd $(WEB_DIR) && $(NPM) install

## 构建前端到 internal/web/dist（供 go:embed）
web-build:
	cd $(WEB_DIR) && $(NPM) run build

## 构建单二进制（先前端后 Go，嵌入完整 UI）
build: web-build
	$(GO) build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/auto-finance-assistant$(EXT) ./cmd/server

## 仅构建 Go（跳过前端，用于占位页快速迭代后端）
build-go:
	$(GO) build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/auto-finance-assistant$(EXT) ./cmd/server

## 前台运行（生产配置）
run:
	$(GO) run ./cmd/server -config config.yaml

## 前台运行（CPU 开发配置）
run-dev:
	$(GO) run ./cmd/server -config config.dev.yaml

## 运行全部测试
test:
	$(GO) test ./...

## 静态检查
vet:
	$(GO) vet ./...

## 格式化
fmt:
	$(GO) fmt ./...

## 清理产物
clean:
	rm -rf $(BIN_DIR) internal/web/dist web/dist
