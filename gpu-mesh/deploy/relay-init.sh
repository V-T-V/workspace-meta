#!/usr/bin/env bash
# gpu-mesh Relay 一键初始化脚本（Linux VPS）
#
# 用法（root 或 sudo）：
#   curl -fsSL https://your-host/deploy/relay-init.sh | sudo bash
#   或本地：
#   sudo bash deploy/relay-init.sh
#
# 做的事：
#   1. 建系统用户 gpu-mesh + 工作目录
#   2. 下载/编译 gpu-mesh-relay 到 /usr/local/bin
#   3. 生成强随机 token 并注入 systemd unit
#   4. 安装并启动 systemd 服务
#   5. 打印控制台地址 + Agent 安装命令
#
# 可选环境变量：
#   RELAY_TOKEN   自定义 token（默认随机生成）
#   RELAY_BIN     已编译的二进制路径（默认从源码编译）

set -euo pipefail

GREEN='\033[0;32m'; YELLOW='\033[1;33m'; RED='\033[0;31m'; CYAN='\033[0;36m'; NC='\033[0m'
info()  { echo -e "${GREEN}[INFO]${NC} $*"; }
warn()  { echo -e "${YELLOW}[WARN]${NC} $*"; }
err()   { echo -e "${RED}[ERR]${NC} $*" >&2; }

# 必须 root
if [[ $EUID -ne 0 ]]; then
  err "需要 root 权限运行：sudo bash $0"
  exit 1
fi

# 检查 systemd
if ! command -v systemctl &>/dev/null; then
  err "未检测到 systemd，本脚本仅支持 systemd 发行版（CentOS7+/Ubuntu16.04+/Debian8+）"
  exit 1
fi

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

info "=== GPU Mesh Relay 初始化 ==="

# 1. 建用户和目录
info "创建系统用户与目录..."
if ! id gpu-mesh &>/dev/null; then
  useradd -r -s /sbin/nologin -d /var/lib/gpu-mesh -M gpu-mesh
fi
mkdir -p /var/lib/gpu-mesh /var/log/gpu-mesh
chown -R gpu-mesh:gpu-mesh /var/lib/gpu-mesh /var/log/gpu-mesh

# 2. 准备二进制
BIN_PATH=/usr/local/bin/gpu-mesh-relay
if [[ -n "${RELAY_BIN:-}" && -f "$RELAY_BIN" ]]; then
  info "从 $RELAY_BIN 安装二进制..."
  cp "$RELAY_BIN" "$BIN_PATH"
elif [[ -f "$PROJECT_DIR/bin/gpu-mesh-relay" ]]; then
  info "从项目 bin/ 安装二进制..."
  cp "$PROJECT_DIR/bin/gpu-mesh-relay" "$BIN_PATH"
elif command -v go &>/dev/null; then
  info "从源码编译（需要网络下载依赖）..."
  cd "$PROJECT_DIR"
  GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -o "$BIN_PATH" ./cmd/relay
else
  err "找不到二进制也找不到 go 编译器。请先 make relay 或设 RELAY_BIN 环境变量"
  exit 1
fi
chmod +x "$BIN_PATH"
info "二进制已就位: $BIN_PATH ($($BIN_PATH -h 2>&1 | head -1 || echo 'installed'))"

# 3. 生成 token
if [[ -z "${RELAY_TOKEN:-}" ]]; then
  RELAY_TOKEN=$(openssl rand -hex 32)
  info "已生成随机 token（保存好，Agent 和控制台都要用）"
else
  info "使用自定义 token"
fi

# 4. 安装 systemd unit（注入 token）
info "安装 systemd 服务..."
TMP_UNIT=$(mktemp)
sed "s|CHANGE_ME_TO_STRONG_TOKEN|${RELAY_TOKEN}|" "$SCRIPT_DIR/systemd/gpu-mesh-relay.service" > "$TMP_UNIT"
install -m 644 "$TMP_UNIT" /etc/systemd/system/gpu-mesh-relay.service
rm -f "$TMP_UNIT"

systemctl daemon-reload
systemctl enable gpu-mesh-relay
systemctl restart gpu-mesh-relay

# 5. 等待启动 + 健康检查
info "等待服务启动..."
sleep 2
for i in 1 2 3 4 5; do
  if curl -sf http://127.0.0.1:7780/healthz >/dev/null 2>&1; then
    info "服务健康检查通过"
    break
  fi
  sleep 1
done

# 6. 防火墙提示
PUBLIC_IP=$(curl -s --max-time 3 ifconfig.me 2>/dev/null || hostname -I 2>/dev/null | awk '{print $1}' || echo "YOUR_SERVER_IP")

echo ""
echo -e "${CYAN}========================================${NC}"
echo -e "${GREEN}✓ GPU Mesh Relay 部署完成${NC}"
echo -e "${CYAN}========================================${NC}"
echo ""
echo -e "控制台:    ${GREEN}http://${PUBLIC_IP}:7780/?token=${RELAY_TOKEN}${NC}"
echo -e "Agent 接入: ${GREEN}ws://${PUBLIC_IP}:7780/agent${NC}"
echo -e "Token:     ${YELLOW}${RELAY_TOKEN}${NC}"
echo ""
echo -e "${YELLOW}每台 4060 机器上执行（PowerShell 管理员）:${NC}"
echo -e "  .\\install-agent.ps1 -Relay ws://${PUBLIC_IP}:7780 -Token ${RELAY_TOKEN} -AgentID gpu-XXX-001"
echo ""
echo -e "${YELLOW}生产加固建议:${NC}"
echo "  1. 配置 Nginx 反代 + TLS（见 deploy/nginx/gpu-mesh-nginx.conf）"
echo "  2. 开启防火墙: ufw allow 7780/tcp（或 iptables 对应规则）"
echo "  3. token 保存到密码管理器，不要泄露"
echo ""
echo -e "日志: journalctl -u gpu-mesh-relay -f"
echo -e "状态: systemctl status gpu-mesh-relay"
