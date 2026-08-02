#!/usr/bin/env bash
# 实时数字人 · 一键安装（Linux/macOS）
# 用法：./install.sh

set -e

echo ""
echo "════════════════════════════════════════════"
echo "  实时数字人 · 一键安装"
echo "════════════════════════════════════════════"
echo ""

# 切到脚本所在目录（项目根）
cd "$(dirname "$0")"

# 1. 检查 Python
echo "[1/4] 检查 Python..."
if command -v python3 >/dev/null 2>&1; then
    PY=python3
elif command -v python >/dev/null 2>&1; then
    PY=python
else
    echo "  ❌ 未找到 python，请先安装 Python 3.10+"
    exit 1
fi
PYVER=$($PY --version 2>&1 | awk '{print $2}')
echo "  ✓ Python $PYVER"

# 2. 创建虚拟环境
echo ""
echo "[2/4] 创建虚拟环境 .venv..."
if [ ! -d .venv ]; then
    $PY -m venv .venv
    echo "  ✓ .venv 已创建"
else
    echo "  ✓ .venv 已存在"
fi
# shellcheck disable=SC1091
source .venv/bin/activate

# 3. 安装核心依赖
echo ""
echo "[3/4] 安装核心依赖..."
pip install --quiet --upgrade pip
pip install --quiet fastapi "uvicorn[standard]" pyyaml aiohttp websockets edge-tts numpy opencv-python
echo "  ✓ 核心依赖已安装"

# 4. 检查 Ollama
echo ""
echo "[4/4] 检查 Ollama 与模型..."
if command -v ollama >/dev/null 2>&1; then
    echo "  ✓ ollama 已安装"
    if ! ollama list 2>/dev/null | grep -q "qwen2.5:3b"; then
        echo "  拉取模型 qwen2.5:3b（约 1.9GB，首次较慢）..."
        ollama pull qwen2.5:3b
    else
        echo "  ✓ 模型 qwen2.5:3b 已就绪"
    fi
else
    echo "  ⚠ 未找到 ollama"
    echo "     请安装：https://ollama.com/download"
    echo "     安装后运行：ollama pull qwen2.5:3b"
fi

# 完成
echo ""
echo "════════════════════════════════════════════"
echo "  ✓ 安装完成！"
echo ""
echo "  启动服务：./run.sh"
echo "  或手动：python -m digitalhuman.server"
echo ""
echo "  浏览器打开：http://127.0.0.1:8000"
echo "════════════════════════════════════════════"
