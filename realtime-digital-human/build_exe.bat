@echo off
chcp 65001 >nul
REM 实时数字人 · 单 EXE 打包（Lite 版）
REM 产出：dist/DigitalHuman-Lite.exe（~50MB，双击即用）
REM 用法：build_exe.bat

setlocal
cd /d "%~dp0"

echo.
echo ════════════════════════════════════════════════
echo   单 EXE 打包（Lite 版，~50MB）
echo ════════════════════════════════════════════════
echo.

REM venv（若没有则创建 + 装 Lite 依赖）
if not exist .venv (
    echo [1/3] 创建 venv + 装依赖...
    python -m venv .venv
)
call .venv\Scripts\activate.bat
set HTTP_PROXY=
set HTTPS_PROXY=
python -m pip install --quiet --upgrade pip >nul 2>&1
pip install --quiet fastapi "uvicorn[standard]" pyyaml aiohttp websockets edge-tts miniaudio numpy pyinstaller
if errorlevel 1 (
    pip install --quiet -i http://mirrors.aliyun.com/pypi/simple/ --trusted-host mirrors.aliyun.com fastapi "uvicorn[standard]" pyyaml aiohttp websockets edge-tts miniaudio numpy pyinstaller
)
if errorlevel 1 (
    echo ❌ 依赖安装失败
    pause & exit /b 1
)
echo   ✓ 依赖就绪

REM 打包
echo [2/3] PyInstaller 打包（onefile，约 3-5 分钟）...
set DH_PROJECT_ROOT=%CD%
pyinstaller packaging\digitalhuman.onefile.spec --noconfirm --clean
if errorlevel 1 (
    echo ❌ 打包失败
    pause & exit /b 1
)

REM 验证
echo [3/3] 验证产出...
if exist dist\DigitalHuman-Lite.exe (
    for %%S in ("dist\DigitalHuman-Lite.exe") do (
        set SIZE=%%~zS
        set /a SIZEMB=!SIZE! / 1048576
        echo   ✓ 产出：dist\DigitalHuman-Lite.exe（!SIZEMB! MB）
    )
) else (
    echo   ❌ 未找到 dist\DigitalHuman-Lite.exe
    pause & exit /b 1
)

echo.
echo ════════════════════════════════════════════════
echo   ✓ 单 EXE 打包完成！
echo.
echo   dist\DigitalHuman-Lite.exe
echo   双击运行 → 浏览器开 http://127.0.0.1:8000
echo.
echo   需先装 Ollama + 拉模型：ollama pull qwen2.5:3b
echo   ASR/唇形降级占位（Lite 版无 GPU 重依赖）
echo   完整 GPU 版用 build_windows.bat
echo ════════════════════════════════════════════════
pause
