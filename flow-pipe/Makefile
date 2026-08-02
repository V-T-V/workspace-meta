# flow-pipe Makefile —— 仿 generic-admin / workspace-ops 风格

GO        ?= go
BIN_DIR   := bin
EXT       := $(shell go env GOOS | grep -q windows && echo .exe)
VERSION   := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS   := -X main.version=$(VERSION)
CONFIG    ?= config.dev.yaml

.PHONY: build run-server run-worker run-example run-example-csv run-schedule test vet fmt release clean

## 构建 server 二进制
build:
	$(GO) build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/flow-pipe$(EXT) ./cmd/server
	$(GO) build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/flow-pipe-worker$(EXT) ./cmd/worker

## 前台启动 server
run-server:
	$(GO) run ./cmd/server -config $(CONFIG)

## 启动 worker（M3 占位）
run-worker:
	$(GO) run ./cmd/worker

## 跑示例管道（generate → stdout，最简验证）
run-example:
	$(GO) run ./cmd/server -config $(CONFIG) -run examples/generate-to-stdout.yaml

## 跑 csv→sqlite 示例
run-example-csv:
	$(GO) run ./cmd/server -config $(CONFIG) -run examples/csv-to-sqlite.yaml

## 定时循环跑示例管道（每 60s 一次，Ctrl+C 退出；改 -interval 调间隔）
run-schedule:
	$(GO) run ./cmd/server -config $(CONFIG) -schedule examples/generate-to-stdout.yaml -interval 60

## 运行测试
test:
	$(GO) test ./...

## 静态检查
vet:
	$(GO) vet ./...

## 格式化
fmt:
	$(GO) fmt ./...

## 跨平台构建
release:
	mkdir -p dist
	GOOS=windows GOARCH=amd64 $(GO) build -ldflags "$(LDFLAGS)" -o dist/flow-pipe-windows-amd64.exe ./cmd/server
	GOOS=linux GOARCH=amd64 $(GO) build -ldflags "$(LDFLAGS)" -o dist/flow-pipe-linux-amd64 ./cmd/server
	GOOS=darwin GOARCH=arm64 $(GO) build -ldflags "$(LDFLAGS)" -o dist/flow-pipe-darwin-arm64 ./cmd/server

## 清理
clean:
	rm -rf $(BIN_DIR) dist flow-pipe.db *.db
