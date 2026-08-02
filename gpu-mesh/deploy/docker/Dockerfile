# gpu-mesh Relay Dockerfile
# 多阶段构建：builder 编译，runtime 仅含二进制（镜像小）
#
# 构建：docker build -t gpu-mesh-relay:latest -f deploy/docker/Dockerfile .
# 运行：见 deploy/docker/docker-compose.yml

# === 阶段1：编译 ===
FROM golang:1.25-alpine AS builder

WORKDIR /src
# 先拷依赖（利用层缓存）
COPY go.mod go.sum ./
RUN go mod download

# 拷源码编译（CGO 禁用，纯静态）
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -trimpath -ldflags="-s -w" \
    -o /out/gpu-mesh-relay ./cmd/relay

# === 阶段2：运行时（极简镜像）===
FROM alpine:3.19

# 装 ca-certificates（HTTPS）+ wget（健康检查）+ tzdata（时区）
RUN apk add --no-cache ca-certificates wget tzdata && \
    adduser -D -u 1000 gpu-mesh

# 拷二进制
COPY --from=builder /out/gpu-mesh-relay /usr/local/bin/gpu-mesh-relay

# 数据目录
RUN mkdir -p /var/lib/gpu-mesh /var/log/gpu-mesh && \
    chown -R gpu-mesh:gpu-mesh /var/lib/gpu-mesh /var/log/gpu-mesh

USER gpu-mesh
WORKDIR /var/lib/gpu-mesh
VOLUME ["/var/lib/gpu-mesh", "/var/log/gpu-mesh"]

EXPOSE 7780

# 健康检查（容器编排用）
HEALTHCHECK --interval=30s --timeout=5s --retries=3 \
    CMD wget -qO- http://127.0.0.1:7780/healthz || exit 1

ENTRYPOINT ["/usr/local/bin/gpu-mesh-relay"]
CMD ["-addr", ":7780", "-store", "-store-path", "/var/lib/gpu-mesh/relay.db", "-audit", "/var/log/gpu-mesh/audit.jsonl"]
