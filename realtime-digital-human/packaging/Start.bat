@echo off
title Digital Human
cd /d "%~dp0"

REM ★ 项目根 = 本目录的上一级（离线包布局：app/ 同级有 models/ 与 ollama/）
REM 让程序离线加载 models/whisper-small 模型（不再联网下载）
set DH_PROJECT_ROOT=%~dp0..
set HF_HUB_OFFLINE=1

echo.
echo ==================================================
echo   Digital Human Starting...
echo   First launch takes 10-30 seconds, please wait.
echo.
echo   Open in browser: http://127.0.0.1:8000
echo   Press Ctrl+C to stop
echo ==================================================
echo.

REM Ensure Ollama is running
where ollama >nul 2>&1
if not errorlevel 1 (
    tasklist /fi "imagename eq ollama.exe" 2>nul | findstr /i "ollama" >nul
    if errorlevel 1 (
        echo [Starting Ollama...]
        start "" /b ollama serve >nul 2>&1
        timeout /t 2 /nobreak >nul
    )
)

echo [Starting Digital Human...]
DigitalHuman.exe %*
pause
