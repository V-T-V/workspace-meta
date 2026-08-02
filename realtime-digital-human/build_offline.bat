@echo off
chcp 65001 >nul
REM 实时数字人 · 完全离线包构建脚本
REM 产出：dist/DigitalHuman-Offline/ （含全部依赖+模型，无需任何网络）
REM
REM 完全离线包包含：
REM   - PyInstaller onedir（Python + 所有依赖）
REM   - faster-whisper small 模型（~500MB）
REM   - Ollama 安装包（~500MB）
REM   - qwen2.5:3b 模型文件（~1.8GB）
REM   - Start.bat（一键启动，自动配置 Ollama）
REM
REM 总大小预计：~5-6GB（含全部模型+依赖）

setlocal EnableDelayedExpansion
cd /d "%~dp0"

echo.
echo ════════════════════════════════════════════════════
echo   实时数字人 · 完全离线包构建
echo ════════════════════════════════════════════════════
echo.

set OFFLINE_DIR=dist\DigitalHuman-Offline

REM ===== 1. PyInstaller 打包 =====
echo [1/5] PyInstaller 打包（onedir，含全部 Python 依赖）...
if not exist .venv (
    python -m venv .venv
)
call .venv\Scripts\activate.bat
set HTTP_PROXY=
set HTTPS_PROXY=
set NO_PROXY=127.0.0.1,localhost
python -m pip install --quiet --upgrade pip >nul 2>&1

pip install --quiet fastapi "uvicorn[standard]" pyyaml aiohttp websockets edge-tts miniaudio numpy faster-whisper opencv-python pyinstaller
if errorlevel 1 (
    pip install --quiet -i http://mirrors.aliyun.com/pypi/simple/ --trusted-host mirrors.aliyun.com fastapi "uvicorn[standard]" pyyaml aiohttp websockets edge-tts miniaudio numpy faster-whisper opencv-python pyinstaller
)

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
    echo   [FAIL] PyInstaller 打包失败
    pause & exit /b 1
)
echo   [OK] PyInstaller 打包完成
echo.

REM ===== 2. 整理目录结构 =====
echo [2/5] 创建离线包目录结构...
if exist "%OFFLINE_DIR%" rmdir /s /q "%OFFLINE_DIR%"
mkdir "%OFFLINE_DIR%"
mkdir "%OFFLINE_DIR%\app"
mkdir "%OFFLINE_DIR%\models"
mkdir "%OFFLINE_DIR%\ollama"

REM 移动 PyInstaller 产出
xcopy /E /I /Q dist\DigitalHuman "%OFFLINE_DIR%\app" >nul
copy /Y packaging\Start.bat "%OFFLINE_DIR%\app\Start.bat" >nul
echo   [OK] app/ 目录就绪
echo.

REM ===== 3. 复制 faster-whisper 模型 =====
echo [3/5] 复制 faster-whisper small 模型（~500MB）...
REM ★ 优先用项目内完整 snapshot（含 config.json/tokenizer.json/vocabulary.txt，
REM   faster-whisper 才能完全离线加载；HF 缓存往往只有 model.bin 会触发联网下载）
if exist "models\whisper-small\model.bin" (
    mkdir "%OFFLINE_DIR%\models\whisper-small"
    xcopy /E /I /Q "models\whisper-small" "%OFFLINE_DIR%\models\whisper-small" >nul
    echo   [OK] whisper-small 完整 snapshot 已复制（含 tokenizer/vocabulary）
) else (
    set HF_CACHE=%USERPROFILE%\.cache\huggingface\hub\models--Systran--faster-whisper-small
    if exist "%HF_CACHE%" (
        mkdir "%OFFLINE_DIR%\models\whisper-small"
        xcopy /E /I /Q "%HF_CACHE%" "%OFFLINE_DIR%\models\whisper-small" >nul
        echo   [OK] whisper-small 模型已复制（HF 缓存，可能不完整）
    ) else (
        echo   [WARN] faster-whisper 模型不存在
        echo   请先把完整 snapshot 放到 models/whisper-small/（含 model.bin+config.json+tokenizer.json+vocabulary.txt）
    )
)
echo.

REM ===== 4. 复制 Ollama + 模型 =====
echo [4/5] 复制 Ollama + qwen2.5:3b 模型（~2GB）...

REM ★ Ollama 完整运行时：ollama.exe + lib/ollama/（llama-server.exe + ggml DLL + CUDA 后端）
REM   只复制 ollama.exe 会导致 "llama-server binary not found / 0xc0000135" 加载失败！
set OLLAMA_INSTALL=%LOCALAPPDATA%\Programs\Ollama
if exist "%OLLAMA_INSTALL%\ollama.exe" (
    copy /Y "%OLLAMA_INSTALL%\ollama.exe" "%OFFLINE_DIR%\ollama\ollama.exe" >nul 2>&1
    mkdir "%OFFLINE_DIR%\ollama\lib\ollama"
    xcopy /E /I /Q "%OLLAMA_INSTALL%\lib\ollama\ggml*.dll" "%OFFLINE_DIR%\ollama\lib\ollama" >nul 2>&1
    xcopy /E /I /Q "%OLLAMA_INSTALL%\lib\ollama\lib*.dll" "%OFFLINE_DIR%\ollama\lib\ollama" >nul 2>&1
    copy /Y "%OLLAMA_INSTALL%\lib\ollama\llama-server.exe" "%OFFLINE_DIR%\ollama\lib\ollama" >nul 2>&1
    REM CUDA v12 后端（4060 等 NVIDIA 显卡）；rocm 是 AMD 可省略
    if exist "%OLLAMA_INSTALL%\lib\ollama\cuda_v12" (
        xcopy /E /I /Q "%OLLAMA_INSTALL%\lib\ollama\cuda_v12" "%OFFLINE_DIR%\ollama\lib\ollama\cuda_v12" >nul 2>&1
    )
    if exist "%OFFLINE_DIR%\ollama\ollama.exe" (
        echo   [OK] ollama.exe + 完整运行时已复制（含 llama-server + CUDA v12）
    ) else (
        echo   [WARN] ollama.exe 复制失败
    )
) else (
    echo   [WARN] Ollama 未安装（%OLLAMA_INSTALL% 不存在，需要用户自行安装）
)

REM Ollama 模型
set OLLAMA_MODELS=%OLLAMA_MODELS%
if "%OLLAMA_MODELS%"=="" set OLLAMA_MODELS=%USERPROFILE%\.ollama\models

if exist "%OLLAMA_MODELS%\manifests\registry.ollama.ai\library\qwen2.5" (
    mkdir "%OFFLINE_DIR%\ollama\models"
    xcopy /E /I /Q "%OLLAMA_MODELS%\manifests" "%OFFLINE_DIR%\ollama\models\manifests" >nul
    xcopy /E /I /Q "%OLLAMA_MODELS%\blobs" "%OFFLINE_DIR%\ollama\models\blobs" >nul
    echo   [OK] qwen2.5:3b 模型已复制
) else (
    echo   [WARN] Ollama 模型未找到（检查 OLLAMA_MODELS 环境变量）
)
echo.

REM ===== 5. 生成离线启动脚本 =====
echo [5/5] 生成离线启动脚本...

REM 主启动脚本
(
echo @echo off
echo title Digital Human - Offline
echo cd /d "%%~dp0"
echo.
echo echo.
echo echo ==================================================
echo echo   Digital Human - Offline Mode
echo echo   Starting... ^(~20 seconds first time^)
echo echo.
echo echo   Open browser: http://127.0.0.1:8000
echo echo   Press Ctrl+C to stop
echo echo ==================================================
echo echo.
echo REM Set Ollama to use bundled models
echo set OLLAMA_MODELS=%%~dp0ollama\models
echo.
echo REM Start Ollama if bundled
echo if exist "%%~dp0ollama\ollama.exe" (
echo     tasklist /fi "imagename eq ollama.exe" 2^>nul ^| findstr /i "ollama" ^>nul
echo     if errorlevel 1 (
echo         echo [Starting Ollama...]
echo         start "" /b "%%~dp0ollama\ollama.exe" serve
echo         timeout /t 3 /nobreak ^>nul
echo     )
echo )
echo.
echo REM Set faster-whisper to use bundled model
echo set HF_HUB_OFFLINE=1
echo set HF_HOME=%%~dp0models
echo set DH_PROJECT_ROOT=%%~dp0
echo.
echo echo [Starting Digital Human...]
echo "%%~dp0app\DigitalHuman.exe" %%*
echo pause
) > "%OFFLINE_DIR%\启动.bat"

REM English version
(
echo @echo off
echo title Digital Human - Offline
echo cd /d "%%~dp0"
echo echo Starting Digital Human ^(~20 seconds^)...
echo set OLLAMA_MODELS=%%~dp0ollama\models
echo set HF_HUB_OFFLINE=1
echo set HF_HOME=%%~dp0models
echo set DH_PROJECT_ROOT=%%~dp0
echo if exist "%%~dp0ollama\ollama.exe" (
echo     start "" /b "%%~dp0ollama\ollama.exe" serve
echo     timeout /t 3 /nobreak ^>nul
echo )
echo "%%~dp0app\DigitalHuman.exe" %%*
echo pause
) > "%OFFLINE_DIR%\Start.bat"

echo   [OK] 启动脚本已生成
echo.

REM ===== 完成 =====
echo ════════════════════════════════════════════════════
echo   完全离线包构建完成！
echo.
echo   位置：%OFFLINE_DIR%
echo.
echo   目录结构：
echo   DigitalHuman-Offline/
echo     ├── Start.bat / 启动.bat    ← 双击启动
echo     ├── app/                    ← 数字人程序（Python+依赖）
echo     ├── models/whisper-small/   ← ASR 模型（~500MB）
echo     └── ollama/                 ← Ollama + qwen2.5:3b（~2GB）
echo.
echo   使用：双击 启动.bat 或 Start.bat
echo   无需网络、无需 Python、无需 pip install
echo ════════════════════════════════════════════════════
pause
exit /b 0
