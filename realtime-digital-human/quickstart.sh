#!/usr/bin/env bash
# 实时数字人 · 极简部署（一个脚本搞定）
# 用法：./quickstart.sh [tier]
#   tier=0  体验版（纯文字，~30MB，1分钟）
#   tier=1  标准版（+语音TTS，~80MB，2分钟）
#   tier=2  完整版（+ASR+唇形，需GPU，~3GB，15分钟）
#   不传 = 自动探测：有GPU装tier2，否则tier1

set -e
cd "$(dirname "$0")"

TIER="${1:-}"
if [ -z "$TIER" ]; then
    if command -v nvidia-smi >/dev/null 2>&1; then
        TIER=2; echo "检测到 GPU，使用完整版（tier 2）"
    else
        TIER=1; echo "未检测到 GPU，使用标准版（tier 1）"
    fi
fi
echo "目标版本: Tier $TIER"
echo ""

# Python 检查
PY=python3
command -v $PY >/dev/null 2>&1 || PY=python
command -v $PY >/dev/null 2>&1 || { echo "❌ 需要 Python 3.10+"; exit 1; }

# venv
if [ ! -d .venv ]; then
    echo "[1/3] 创建虚拟环境..."
    $PY -m venv .venv
fi
source .venv/bin/activate

# 分层装依赖
echo "[2/3] 安装依赖（Tier $TIER）..."
pip install --quiet --upgrade pip

PKGS="fastapi uvicorn[standard] pyyaml"
if [ "$TIER" -ge 1 ]; then
    PKGS="$PKGS edge-tts miniaudio aiohttp websockets numpy"
fi
if [ "$TIER" -eq 2 ]; then
    echo "  安装 CUDA PyTorch（较大，请等待）..."
    pip install --quiet torch --index-url https://download.pytorch.org/whl/cu133 || \
    pip install --quiet torch --index-url https://download.pytorch.org/whl/cu121
    PKGS="$PKGS faster-whisper opencv-python"
fi

echo "  安装: $PKGS"
pip install --quiet $PKGS
echo "  ✓ 依赖就绪"

# Ollama
echo "[3/3] 检查 Ollama..."
if command -v ollama >/dev/null 2>&1; then
    if ! ollama list 2>/dev/null | grep -q "qwen2.5:3b"; then
        echo "  拉取模型 qwen2.5:3b..."
        ollama pull qwen2.5:3b
    else
        echo "  ✓ 模型就绪"
    fi
else
    echo "  ⚠ 需要 Ollama：https://ollama.com/download"
    echo "  装完运行：ollama pull qwen2.5:3b"
fi

echo ""
echo "════════════════════════════════════════════"
echo "  ✓ 部署完成！"
echo ""
echo "  启动：python -m digitalhuman.server"
echo "  浏览器：http://127.0.0.1:8000"
echo ""
[ "$TIER" = "0" ] && echo "  当前：体验版（文字对话，无语音）"
[ "$TIER" = "1" ] && echo "  当前：标准版（文字+语音TTS）"
[ "$TIER" = "2" ] && echo "  当前：完整版（文字+语音+ASR+唇形）"
echo "  升级：./quickstart.sh 1  或  ./quickstart.sh 2"
echo "════════════════════════════════════════════"

read -p "现在启动？(y/n): " RUN
[ "$RUN" = "y" ] && python -m digitalhuman.server
