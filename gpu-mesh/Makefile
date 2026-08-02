.PHONY: all relay agent tidy test vet clean run-relay run-agent

# Go 版本要求 1.22+（开发机已 1.25.6）
GOFLAGS := -trimpath

all: relay agent

# 编译中继节点（公网 VPS 部署，通常 Linux amd64）
relay:
	$(MAKE) relay-linux

# 编译 Agent（Windows 服务，每台 4060 机器部署）
agent:
	$(MAKE) agent-windows

relay-linux:
	GOOS=linux GOARCH=amd64 go build $(GOFLAGS) -o bin/gpu-mesh-relay ./cmd/relay

relay-windows:
	GOOS=windows GOARCH=amd64 go build $(GOFLAGS) -o bin/gpu-mesh-relay.exe ./cmd/relay

# Windows Agent（带文件版本信息，便于运维识别）
agent-windows:
	GOOS=windows GOARCH=amd64 go build $(GOFLAGS) \
		-ldflags "-X github.com/QiuShichang/gpu-mesh/internal/agent.Version=0.1.0" \
		-o bin/gpu-mesh-agent.exe ./cmd/agent

# 本机调试编译（用当前 OS）
dev:
	go build $(GOFLAGS) -o bin/gpu-mesh-relay$(EXT) ./cmd/relay
	go build $(GOFLAGS) -o bin/gpu-mesh-agent$(EXT) ./cmd/agent

# Windows 下 .exe 后缀
ifeq ($(OS),Windows_NT)
EXT := .exe
endif

tidy:
	go mod tidy

test:
	go test ./...

test-race:
	go test -race ./...

vet:
	go vet ./...

clean:
	rm -rf bin/ gpu-mesh.db gpu-mesh-relay.db

# 本地三端联调（两个终端）
run-relay:
	go run ./cmd/relay -addr :7780

run-agent:
	go run ./cmd/agent run -relay ws://127.0.0.1:7780

# === 部署 ===

# 打包 Linux Relay 部署包（含二进制 + 部署脚本，传到 VPS 解压即用）
deploy-pack-relay:
	mkdir -p dist/relay
	GOOS=linux GOARCH=amd64 go build $(GOFLAGS) -o dist/relay/gpu-mesh-relay ./cmd/relay
	cp deploy/relay-init.sh deploy/systemd/gpu-mesh-relay.service dist/relay/
	cp deploy/nginx/gpu-mesh-nginx.conf dist/relay/
	cd dist && tar czf gpu-mesh-relay-deploy.tar.gz relay/
	@echo "=== 部署包: dist/gpu-mesh-relay-deploy.tar.gz ==="
	@echo "上传到 VPS 后：tar xzf gpu-mesh-relay-deploy.tar.gz && cd relay && sudo bash relay-init.sh"

# 打包 Windows Agent 部署包（含 exe + 安装脚本，分发给每台机器）
deploy-pack-agent:
	mkdir -p dist/agent
	GOOS=windows GOARCH=amd64 go build $(GOFLAGS) \
		-ldflags "-X github.com/QiuShichang/gpu-mesh/internal/agent.Version=$$(git describe --tags 2>/dev/null || echo dev)" \
		-o dist/agent/gpu-mesh-agent.exe ./cmd/agent
	cp scripts/install-agent.ps1 dist/agent/
	cp deploy/playbook/batch-install-agent.ps1 deploy/playbook/agents.csv.example dist/agent/
	cd dist && zip -r gpu-mesh-agent-deploy.zip agent/
	@echo "=== 部署包: dist/gpu-mesh-agent-deploy.zip ==="

# Docker 构建 Relay 镜像
deploy-docker:
	docker build -t gpu-mesh-relay:latest -f deploy/docker/Dockerfile .

# 一键打包所有部署资产
deploy-all: deploy-pack-relay deploy-pack-agent
	@echo "=== 全部部署包已生成于 dist/ ==="
	@ls -lh dist/*.tar.gz dist/*.zip 2>/dev/null
