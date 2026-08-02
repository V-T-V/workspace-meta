@echo off
chcp 65001 >nul
REM 实时数字人 · 4060 GPU 启动
REM 用 config.gpu.yaml（真实 ASR + 真实唇形）
REM 用法：run_gpu.bat [端口号]

setlocal
cd /d "%~dp0"

if exist .venv\Scripts\activate.bat call .venv\Scripts\activate.bat

set PORT=8000
if not "%~1"=="" set PORT=%~1

REM 确保 ollama 在跑
where ollama >nul 2>&1
if not errorlevel 1 (
    tasklist /fi "imagename eq ollama.exe" 2>nul | findstr /i "ollama.exe" >nul
    if errorlevel 1 (
        start "" /b ollama serve >nul 2>&1
        timeout /t 2 /nobreak >nul
    )
)

REM 优先用 GPU 配置（不存在则回退 config.dev.yaml / 零配置）
set CFG=config.gpu.yaml
if not exist %CFG% set CFG=config.dev.yaml

echo.
echo ════════════════════════════════════════════
echo   实时数字人 · GPU 模式（端口 %PORT%）
echo   配置: %CFG%
echo   浏览器: http://127.0.0.1:%PORT%
echo   按 Ctrl+C 停止
echo ════════════════════════════════════════════
echo.

python -m digitalhuman.server -c %CFG% --port %PORT%
pause
