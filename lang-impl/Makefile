# lang-impl Makefile —— 仿 go-agent-research / consensus-atlas 风格
GO        ?= go
BIN_DIR   := bin
EXT       := $(shell go env GOOS | grep -q windows && echo .exe)
VERSION   := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS   := -X main.version=$(VERSION)

.PHONY: tidy build run repl test test-race vet fmt clean bench

tidy:
	$(GO) mod tidy

## 构建单二进制
build:
	$(GO) build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/lang-impl$(EXT) ./cmd/langimpl

## 前台运行默认 demo（lexer）
run:
	$(GO) run ./cmd/langimpl -d lex

## 交互式 REPL
repl:
	$(GO) run ./cmd/langimpl -d repl

## 运行全部测试
test:
	$(GO) test ./...

## 竞态检测
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
