@echo off
chcp 65001 >nul
REM 实时数字人 · 完整离线包打包（含全部依赖，目标机无需装任何东西）
REM 产出：dist/DigitalHuman/（onedir，双击 DigitalHuman.exe 即用）
REM 用法：build_full.bat

setlocal
cd /d "%~dp0"

echo.
echo ════════════════════════════════════════════════
echo   完整离线包打包（全部依赖内嵌，~400MB）
echo ════════════════════════════════════════════════
echo.

REM 1. venv + 依赖
if not exist .venv (
    echo [1/3] 创建 venv...
    python -m venv .venv
)
call .venv\Scripts\activate.bat
set HTTP_PROXY=
set HTTPS_PROXY=

echo [2/3] 装全部依赖...
python -m pip install --quiet --upgrade pip >nul 2>&1
pip install --quiet fastapi "uvicorn[standard]" pyyaml aiohttp websockets edge-tts miniaudio numpy faster-whisper opencv-python pyinstaller
if errorlevel 1 (
    pip install --quiet -i http://mirrors.aliyun.com/pypi/simple/ --trusted-host mirrors.aliyun.com fastapi "uvicorn[standard]" pyyaml aiohttp websockets edge-tts miniaudio numpy faster-whisper opencv-python pyinstaller
)
if errorlevel 1 (
    echo ❌ 依赖安装失败
    pause & exit /b 1
)
echo   ✓ 依赖就绪

REM 2. 打包（命令行参数方式，比 spec 更高效）
echo [3/3] PyInstaller 打包（约 5-10 分钟）...
set DH_PROJECT_ROOT=%CD%
pyinstaller digitalhuman\__main__.py ^
    --name DigitalHuman ^
    --onedir --noconfirm --clean ^
    --collect-all ctranslate2 ^
    --collect-all faster_whisper ^
    --collect-all cv2 ^
    --collect-all edge_tts ^
    --collect-all miniaudio ^
    --collect-all aiohttp ^
    --add-data "web;web" ^
    --add-data "scripts/musetalk_render.py;scripts" ^
    --add-data "scripts/cosyvoice_tts.py;scripts" ^
    --add-data "assets;assets" ^
    --add-data "config.example.yaml;." ^
    --add-data "config.dev.yaml;." ^
    --hidden-import uvicorn.lifespan.on ^
    --hidden-import pyyaml ^
    --exclude-module torch ^
    --exclude-module torchvision ^
    --exclude-module matplotlib ^
    --exclude-module scipy ^
    --exclude-module pandas ^
    --exclude-module tkinter ^
    --exclude-module tensorboard ^
    --icon packaging\digitalhuman.ico
if errorlevel 1 (
    echo ❌ 打包失败
    pause & exit /b 1
)

REM 3. 验证
if exist dist\DigitalHuman\DigitalHuman.exe (
    REM 自动复制 Start.bat（启动提示脚本）
    copy /Y packaging\Start.bat dist\DigitalHuman\Start.bat >nul 2>&1
    echo.
    echo ════════════════════════════════════════════════
    echo   ✓ 完整包打包完成！
    echo.
    echo   dist\DigitalHuman\Start.bat        ← 双击启动
    echo   dist\DigitalHuman\DigitalHuman.exe  ← 或直接运行
    echo   浏览器开 http://127.0.0.1:8000
    echo   自检：dist\DigitalHuman\DigitalHuman.exe --self-test
    echo.
    echo   注：不含 torch（需 GPU ASR 时运行时 pip install torch）
    echo   或用 build_windows.bat 打含 CUDA torch 的完整 GPU 版
    echo ════════════════════════════════════════════════
) else (
    echo ❌ 未找到产出
)
pause
