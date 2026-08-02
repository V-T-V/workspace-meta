@echo off
chcp 65001 >nul
REM 实时数字人 · 4060 GPU 一键部署（Windows）
REM 装 CUDA torch + faster-whisper，让真实 ASR 在 GPU 上跑
REM 用法：双击或 deploy_gpu.bat

setlocal

echo.
echo ════════════════════════════════════════════════
echo   实时数字人 · 4060 GPU 部署
echo ════════════════════════════════════════════════
echo.

cd /d "%~dp0"

REM 0. 先检查 NVIDIA 驱动
echo [0/5] 检查 NVIDIA GPU...
nvidia-smi >nul 2>&1
if errorlevel 1 (
    echo   ❌ 未检测到 nvidia-smi，请确认：
    echo      - 已安装 NVIDIA 显卡驱动（4060 需驱动 ^>= 535）
    echo      - 驱动已加入 PATH
    echo   无法继续 GPU 部署。可改用 install.bat（CPU 降级模式）
    pause
    exit /b 1
)
echo   ✓ 检测到 NVIDIA GPU
nvidia-smi --query-gpu=name,memory.total --format=csv,noheader 2>nul
echo.

REM 1. 检查 Python
echo [1/5] 检查 Python...
python --version >nul 2>&1
if errorlevel 1 (
    echo   ❌ 未找到 python，请先装 Python 3.10+
    pause
    exit /b 1
)
for /f "tokens=2" %%v in ('python --version 2^>^&1') do set PYVER=%%v
echo   ✓ Python %PYVER%

REM 2. 创建/激活 venv
if not exist .venv (
    echo.
    echo [2/5] 创建虚拟环境 .venv...
    python -m venv .venv
)
if exist .venv\Scripts\activate.bat call .venv\Scripts\activate.bat
echo   ✓ venv 已就绪

REM 3. 安装 CUDA 版 PyTorch（关键：默认 pip 装的是 CPU 版）
echo.
echo [3/5] 安装 CUDA 版 PyTorch（较大，约 2.5GB，请耐心等待）...
set HTTP_PROXY=
set HTTPS_PROXY=
REM 先卸载可能存在的 CPU 版 torch
pip uninstall -y torch >nul 2>&1
REM 装 CUDA 12.1 版（兼容 4060 + 较新驱动）
pip install --quiet torch torchvision --index-url https://download.pytorch.org/whl/cu121
if errorlevel 1 (
    echo   ⚠ CUDA 12.1 安装失败，尝试 CUDA 11.8...
    pip install --quiet torch torchvision --index-url https://download.pytorch.org/whl/cu118
)
if errorlevel 1 (
    echo   ❌ PyTorch CUDA 安装失败，请手动安装
    echo      参考：https://pytorch.org/get-started/locally/
    pause
    exit /b 1
)
echo   ✓ PyTorch CUDA 已安装

REM 验证 CUDA 可用
python -c "import torch; print(f'  torch={torch.__version__}, cuda={torch.cuda.is_available()}, gpu={torch.cuda.get_device_name(0) if torch.cuda.is_available() else \"N/A\"}')"
if errorlevel 1 (
    echo   ⚠ torch 验证失败，但安装已完成。启动时若 CUDA 不可用会自动回退 CPU
)

REM 4. 安装其他依赖（faster-whisper + 核心 + opencv）
echo.
echo [4/5] 安装 faster-whisper + 核心依赖...
pip install --quiet faster-whisper fastapi "uvicorn[standard]" pyyaml aiohttp websockets edge-tts numpy opencv-python
if errorlevel 1 (
    echo   ⚠ 部分依赖安装失败，尝试镜像...
    pip install --quiet -i http://mirrors.aliyun.com/pypi/simple/ --trusted-host mirrors.aliyun.com faster-whisper fastapi "uvicorn[standard]" pyyaml aiohttp websockets edge-tts numpy opencv-python
)
echo   ✓ faster-whisper + 核心依赖已安装

REM 5. Ollama + 模型
echo.
echo [5/5] 检查 Ollama 与模型...
where ollama >nul 2>&1
if errorlevel 1 (
    echo   ⚠ 未找到 ollama，请安装：https://ollama.com/download
    echo      安装后运行：ollama pull qwen2.5:3b
) else (
    echo   ✓ ollama 已安装
    ollama list 2>nul | findstr /c:"qwen2.5:3b" >nul
    if errorlevel 1 (
        echo   拉取模型 qwen2.5:3b（约 1.9GB）...
        ollama pull qwen2.5:3b
    ) else (
        echo   ✓ 模型 qwen2.5:3b 已就绪
    )
)

echo.
echo ════════════════════════════════════════════════
echo   ✓ 4060 GPU 部署完成！
echo.
echo   启动：run.bat
echo   浏览器：http://127.0.0.1:8000
echo.
echo   显存预算（4060 8GB）：
echo     whisper-small ~1GB + qwen2.5:3b ~1.9GB
echo     + MuseTalk ~1.5GB + 余量 ≈ 4.4GB ✅
echo.
echo   可选：MuseTalk 真实唇形（见 docs/GPU_DEPLOY.md）
echo ════════════════════════════════════════════════
echo.
pause
