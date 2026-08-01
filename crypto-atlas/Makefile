# crypto-atlas Makefile —— 仿 consensus-atlas / go-agent-research 风格
GO        ?= go
BIN_DIR   := bin
EXT       := $(shell go env GOOS | grep -q windows && echo .exe)
VERSION   := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS   := -X main.version=$(VERSION)

.PHONY: tidy build run test test-race vet fmt clean

tidy:
	$(GO) mod tidy

build:
	$(GO) build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/crypto-atlas$(EXT) ./cmd/atlas

run:
	$(GO) run ./cmd/atlas -d caesar

test:
	$(GO) test ./...

test-race:
	CGO_ENABLED=1 $(GO) test -race -count=1 ./...

vet:
	$(GO) vet ./...

fmt:
	$(GO) fmt ./...

clean:
	rm -rf $(BIN_DIR)
