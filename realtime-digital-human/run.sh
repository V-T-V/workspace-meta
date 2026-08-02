#!/usr/bin/env bash
# 实时数字人 · 一键启动（Linux/macOS）
# 用法：./run.sh [端口号]

set -e

# 切到脚本所在目录
cd "$(dirname "$0")"

# 激活 venv（若存在）
if [ -f .venv/bin/activate ]; then
    # shellcheck disable=SC1091
    source .venv/bin/activate
fi

# 端口参数（默认 8000）
PORT=${1:-8000}

# 确保 ollama 在跑
if command -v ollama >/dev/null 2>&1; then
    if ! curl -s http://127.0.0.1:11434/api/version >/dev/null 2>&1; then
        echo "启动 Ollama 后台服务..."
        ollama serve >/dev/null 2>&1 &
        sleep 2
    fi
fi

echo ""
echo "════════════════════════════════════════════"
echo "  实时数字人启动中（端口 $PORT）"
echo "  浏览器打开：http://127.0.0.1:$PORT"
echo "  按 Ctrl+C 停止"
echo "════════════════════════════════════════════"
echo ""

exec python -m digitalhuman.server --port "$PORT"
