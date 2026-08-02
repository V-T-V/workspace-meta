# workspace-ops Makefile —— 仿 generic-admin / auto-finance-assistant 风格
#
# 用法：
#   make build       —— 构建 CLI 到 bin/
#   make run-scan    —— 扫描工作区
#   make run-report  —— 输出 text 报告
#   make run-serve   —— 启动 Web 看板
#   make test        —— 运行测试
#   make vet         —— 静态检查
#   make fmt         —— 格式化
#   make release     —— 跨平台构建到 dist/
#   make clean       —— 清理

GO        ?= go
BIN_DIR   := bin
DIST_DIR  := dist
EXT       := $(shell go env GOOS | grep -q windows && echo .exe)
VERSION   := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS   := -X main.version=$(VERSION)
CONFIG    ?= config.dev.yaml

.PHONY: build run-scan run-report run-serve test vet fmt release clean web-build

## 构建 CLI 到 bin/
build:
	$(GO) build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/workspace-ops$(EXT) ./cmd/ops

## 扫描工作区（结果入库）
run-scan:
	$(GO) run ./cmd/ops scan -config $(CONFIG)

## 输出 text 报告
run-report:
	$(GO) run ./cmd/ops report -config $(CONFIG) -format text

## 输出 markdown 报告
run-report-md:
	$(GO) run ./cmd/ops report -config $(CONFIG) -format markdown

## 启动 Web 看板
run-serve:
	$(GO) run ./cmd/ops serve -config $(CONFIG)

## 构建前端（Vue 3 + Vite，输出到 internal/web/dist/）
web-build:
	cd web && npm install && npm run build && cp -r dist/* ../internal/web/dist/

## 前端开发模式（热更新）
web-dev:
	cd web && npm run dev

## 运行测试
test:
	$(GO) test ./...

## 静态检查
vet:
	$(GO) vet ./...

## 格式化
fmt:
	$(GO) fmt ./...

## 跨平台构建到 dist/（5 平台）
release:
	mkdir -p $(DIST_DIR)
	GOOS=windows GOARCH=amd64 $(GO) build -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/workspace-ops-windows-amd64.exe ./cmd/ops
	GOOS=linux GOARCH=amd64 $(GO) build -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/workspace-ops-linux-amd64 ./cmd/ops
	GOOS=linux GOARCH=arm64 $(GO) build -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/workspace-ops-linux-arm64 ./cmd/ops
	GOOS=darwin GOARCH=amd64 $(GO) build -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/workspace-ops-darwin-amd64 ./cmd/ops
	GOOS=darwin GOARCH=arm64 $(GO) build -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/workspace-ops-darwin-arm64 ./cmd/ops

## 清理
clean:
	rm -rf $(BIN_DIR) $(DIST_DIR) workspace-ops.db
