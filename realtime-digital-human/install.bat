@echo off
chcp 65001 >nul
REM 实时数字人 · 一键安装（Windows）
REM 用法：双击或 install.bat

setlocal

echo.
echo ════════════════════════════════════════════
echo   实时数字人 · 一键安装
echo ════════════════════════════════════════════
echo.

REM 切到脚本所在目录（项目根）
cd /d "%~dp0"

REM 1. 检查 Python
echo [1/4] 检查 Python...
python --version >nul 2>&1
if errorlevel 1 (
    echo   ❌ 未找到 python，请先安装 Python 3.10+ 并加入 PATH
    echo      下载：https://www.python.org/downloads/
    pause
    exit /b 1
)
for /f "tokens=2" %%v in ('python --version 2^>^&1') do set PYVER=%%v
echo   ✓ Python %PYVER%

REM 2. 创建虚拟环境（可选，推荐）
if not exist .venv (
    echo.
    echo [2/4] 创建虚拟环境 .venv...
    python -m venv .venv
    if errorlevel 1 (
        echo   ⚠ 创建 venv 失败，将用系统 Python
    ) else (
        echo   ✓ .venv 已创建
    )
) else (
    echo.
    echo [2/4] 虚拟环境 .venv 已存在
)

REM 激活 venv
if exist .venv\Scripts\activate.bat call .venv\Scripts\activate.bat

REM 3. 安装核心依赖
echo.
echo [3/4] 安装核心依赖（fastapi/uvicorn/aiohttp/edge-tts/opencv）...
python -m pip install --quiet --upgrade pip >nul 2>&1
REM 绕过可能的系统代理
set HTTP_PROXY=
set HTTPS_PROXY=
python -m pip install --quiet fastapi "uvicorn[standard]" pyyaml aiohttp websockets edge-tts numpy opencv-python
if errorlevel 1 (
    echo   ⚠ pip install 部分失败，尝试用国内镜像...
    python -m pip install --quiet -i http://mirrors.aliyun.com/pypi/simple/ --trusted-host mirrors.aliyun.com fastapi "uvicorn[standard]" pyyaml aiohttp websockets edge-tts numpy opencv-python
)
if errorlevel 1 (
    echo   ❌ 依赖安装失败，请手动运行：pip install -r requirements.txt
    pause
    exit /b 1
)
echo   ✓ 核心依赖已安装

REM 4. 检查 Ollama + 拉模型
echo.
echo [4/4] 检查 Ollama 与模型...
where ollama >nul 2>&1
if errorlevel 1 (
    echo   ⚠ 未找到 ollama 命令
    echo      请安装：https://ollama.com/download
    echo      安装后运行：ollama pull qwen2.5:3b
) else (
    echo   ✓ ollama 已安装
    REM 检查模型是否已拉取
    ollama list 2>nul | findstr /c:"qwen2.5:3b" >nul
    if errorlevel 1 (
        echo   拉取模型 qwen2.5:3b（约 1.9GB，首次较慢）...
        ollama pull qwen2.5:3b
    ) else (
        echo   ✓ 模型 qwen2.5:3b 已就绪
    )
)

REM 完成
echo.
echo ════════════════════════════════════════════
echo   ✓ 安装完成！
echo.
echo   启动服务：run.bat
echo   或手动：python -m digitalhuman.server
echo.
echo   浏览器打开：http://127.0.0.1:8000
echo ════════════════════════════════════════════
echo.
echo 注：ASR（faster-whisper）和唇形（MuseTalk）需 GPU 环境
echo     缺失时会自动降级为占位实现，服务仍可运行
echo.
pause
