# ai-safety-atlas Makefile —— 仿 consensus-atlas 风格
GO        ?= go
BIN_DIR   := bin
EXT       := $(shell go env GOOS | grep -q windows && echo .exe)
VERSION   := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS   := -X main.version=$(VERSION)

.PHONY: tidy build run-eval run-detect vet fmt clean

tidy:
	$(GO) mod tidy

build:
	$(GO) build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/ai-safety$(EXT) ./cmd/safety

run-eval:
	$(GO) run ./cmd/safety -d eval

run-detect:
	$(GO) run ./cmd/safety -d detect

vet:
	$(GO) vet ./...

fmt:
	$(GO) fmt ./...

clean:
	rm -rf $(BIN_DIR)
