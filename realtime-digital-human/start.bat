@echo off
chcp 65001 >nul
REM 实时数字人 · 一键启动（零参数，自动探测最优配置）
REM 用法：双击 start.bat 或 start.bat [端口]

setlocal
cd /d "%~dp0"

REM 激活 venv（若有）
if exist .venv\Scripts\activate.bat call .venv\Scripts\activate.bat

REM 端口（默认 8000）
set PORT=8000
if not "%~1"=="" set PORT=%~1

REM 确保 ollama 在跑
where ollama >nul 2>&1
if not errorlevel 1 (
    tasklist /fi "imagename eq ollama.exe" 2>nul | findstr /i "ollama" >nul
    if errorlevel 1 (
        start "" /b ollama serve >nul 2>&1
        timeout /t 2 /nobreak >nul
    )
)

echo.
echo ════════════════════════════════════════════
echo   实时数字人（端口 %PORT%）
echo   浏览器：http://127.0.0.1:%PORT%
echo   按 Ctrl+C 停止
echo ════════════════════════════════════════════
echo.

python -m digitalhuman.server --port %PORT%
pause
