@echo off
chcp 65001 >nul
REM 实时数字人 · 极简部署（一个脚本搞定）
REM 用法：quickstart.bat [tier]
REM   tier=0  体验版（纯文字，~30MB，1分钟）
REM   tier=1  标准版（+语音TTS，~80MB，2分钟）
REM   tier=2  完整版（+ASR+唇形，需GPU，~3GB，15分钟）
REM   不传 = 自动探测：有GPU装tier2，否则tier1

setlocal EnableDelayedExpansion
cd /d "%~dp0"

echo.
echo ════════════════════════════════════════════
echo   实时数字人 · 极简部署
echo ════════════════════════════════════════════
echo.

REM 参数解析
set TIER=%~1
if "%TIER%"=="" (
    REM 自动探测 GPU
    nvidia-smi >nul 2>&1
    if errorlevel 1 (
        set TIER=1
        echo 未检测到 GPU，使用标准版（tier 1）
    ) else (
        set TIER=2
        echo 检测到 GPU，使用完整版（tier 2）
    )
)
echo 目标版本: Tier %TIER%
echo.

REM Python 检查
python --version >nul 2>&1
if errorlevel 1 (
    echo ❌ 需要Python 3.10+，请先安装：https://www.python.org/downloads/
    pause & exit /b 1
)

REM venv（若没有则创建）
if not exist .venv (
    echo [1/3] 创建虚拟环境...
    python -m venv .venv
)
call .venv\Scripts\activate.bat

REM 绕过代理
set HTTP_PROXY=
set HTTPS_PROXY=

REM 分层装依赖
echo [2/3] 安装依赖（Tier %TIER%）...
python -m pip install --quiet --upgrade pip >nul 2>&1

REM Tier 0：核心（最小）
set PKGS=fastapi "uvicorn[standard]" pyyaml
REM Tier 1：+TTS（语音）
if "%TIER%" geq "1" set PKGS=%PKGS% edge-tts miniaudio aiohttp websockets numpy
REM Tier 2：+ASR+唇形（GPU 重依赖）
if "%TIER%"=="2" (
    echo   安装 CUDA PyTorch（较大，请等待）...
    pip install --quiet torch --index-url https://download.pytorch.org/whl/cu133 >nul 2>&1
    if errorlevel 1 pip install --quiet torch --index-url https://download.pytorch.org/whl/cu121 >nul 2>&1
    set PKGS=%PKGS% faster-whisper opencv-python
)

echo   安装: %PKGS%
pip install --quiet %PKGS% 2>nul
if errorlevel 1 (
    echo   主源失败，尝试镜像...
    pip install --quiet -i http://mirrors.aliyun.com/pypi/simple/ --trusted-host mirrors.aliyun.com %PKGS%
)
if errorlevel 1 (
    echo ❌ 依赖安装失败
    pause & exit /b 1
)
echo   ✓ 依赖就绪

REM Ollama（所有 tier 都需要）
echo [3/3] 检查 Ollama...
where ollama >nul 2>&1
if errorlevel 1 (
    echo   ⚠ 数字人需要 Ollama 提供大脑
    echo   下载：https://ollama.com/download
    echo   装完运行：ollama pull qwen2.5:3b
) else (
    ollama list 2>nul | findstr "qwen2.5:3b" >nul
    if errorlevel 1 (
        echo   拉取模型 qwen2.5:3b...
        ollama pull qwen2.5:3b
    ) else (
        echo   ✓ 模型就绪
    )
)

echo.
echo ════════════════════════════════════════════
echo   ✓ 部署完成！
echo.
echo   启动：python -m digitalhuman.server
echo   浏览器：http://127.0.0.1:8000
echo.
if "%TIER%"=="0" echo   当前：体验版（文字对话，无语音）
if "%TIER%"=="1" echo   当前：标准版（文字+语音TTS）
if "%TIER%"=="2" echo   当前：完整版（文字+语音+ASR+唇形）
echo   升级：quickstart.bat 1  或  quickstart.bat 2
echo ════════════════════════════════════════════
echo.

REM 可选：直接启动
set /p RUN="现在启动？(y/n): "
if /i "%RUN%"=="y" python -m digitalhuman.server
pause
