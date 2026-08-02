# consensus-atlas Makefile —— 仿 go-agent-research 风格
#
# 用法：
#   make tidy        —— 同步 Go 依赖
#   make build       —— 构建单二进制到 bin/
#   make run         —— 前台运行默认 demo（raft）
#   make test        —— 运行全部测试
#   make test-race   —— 竞态检测（需 CGO/gcc）
#   make vet         —— 静态检查
#   make fmt         —— 格式化
#   make clean       —— 清理 bin/

GO        ?= go
BIN_DIR   := bin
EXT       := $(shell go env GOOS | grep -q windows && echo .exe)
VERSION   := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS   := -X main.version=$(VERSION)

.PHONY: tidy build run test test-race vet fmt clean

## 同步依赖
tidy:
	$(GO) mod tidy

## 构建单二进制
build:
	$(GO) build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/consensus-atlas$(EXT) ./cmd/atlas

## 前台运行默认 demo（raft）
run:
	$(GO) run ./cmd/atlas -d raft

## 运行全部测试
test:
	$(GO) test ./...

## 竞态检测（需 CGO）
test-race:
	CGO_ENABLED=1 $(GO) test -race -count=1 ./...

## 静态检查
vet:
	$(GO) vet ./...

## 格式化
fmt:
	$(GO) fmt ./...

## 清理
clean:
	rm -rf $(BIN_DIR)
