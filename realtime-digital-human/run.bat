@echo off
chcp 65001 >nul
REM 实时数字人 · 一键启动（Windows）
REM 用法：双击或 run.bat [端口号]

setlocal

REM 切到脚本所在目录
cd /d "%~dp0"

REM 激活 venv（若存在）
if exist .venv\Scripts\activate.bat call .venv\Scripts\activate.bat

REM 端口参数（默认 8000）
set PORT=8000
if not "%~1"=="" set PORT=%~1

REM 确保 ollama 在跑（后台启动，若已跑则无害）
where ollama >nul 2>&1
if not errorlevel 1 (
    echo 检查 Ollama 服务...
    tasklist /fi "imagename eq ollama.exe" 2>nul | findstr /i "ollama.exe" >nul
    if errorlevel 1 (
        echo 启动 Ollama 后台服务...
        start "" /b ollama serve >nul 2>&1
        timeout /t 2 /nobreak >nul
    )
)

echo.
echo ════════════════════════════════════════════
echo   实时数字人启动中（端口 %PORT%）
echo   浏览器打开：http://127.0.0.1:%PORT%
echo   按 Ctrl+C 停止
echo ════════════════════════════════════════════
echo.

python -m digitalhuman.server --port %PORT%
pause
