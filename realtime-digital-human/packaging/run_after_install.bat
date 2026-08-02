@echo off
chcp 65001 >nul
REM 实时数字人 · 首次运行配置（安装后自动启动）
REM 检测 Ollama，缺失则提示安装；检测模型，缺失则提示拉取

setlocal
title 实时数字人 · 首次配置

echo.
echo ════════════════════════════════════════════════
echo   实时数字人 · 首次运行配置
echo ════════════════════════════════════════════════
echo.

REM 1. 检测 Ollama
echo [1/2] 检测 Ollama...
where ollama >nul 2>&1
if errorlevel 1 (
    echo   ❌ 未检测到 Ollama（数字人需要 Ollama 提供大脑）
    echo.
    echo   请先安装 Ollama：
    echo     下载：https://ollama.com/download/windows
    echo     装完后双击桌面的"实时数字人"图标即可
    echo.
    echo   按任意键打开 Ollama 下载页...
    pause >nul
    start https://ollama.com/download/windows
    exit /b 1
) else (
    echo   ✓ Ollama 已安装
)

REM 2. 检测模型
echo.
echo [2/2] 检测模型 qwen2.5:3b...
ollama list 2>nul | findstr /c:"qwen2.5:3b" >nul
if errorlevel 1 (
    echo   模型未拉取，现在拉取 qwen2.5:3b（约 1.9GB，首次较慢）...
    ollama pull qwen2.5:3b
    if errorlevel 1 (
        echo   ❌ 模型拉取失败，请手动运行：ollama pull qwen2.5:3b
        pause
        exit /b 1
    )
    echo   ✓ 模型已就绪
) else (
    echo   ✓ 模型 qwen2.5:3b 已就绪
)

REM 3. 启动服务
echo.
echo ════════════════════════════════════════════════
echo   ✓ 配置完成！启动数字人...
echo.
echo   浏览器将打开 http://127.0.0.1:8000
echo   按 Ctrl+C 或关闭窗口停止
echo ════════════════════════════════════════════════
echo.

cd /d "%~dp0"
DigitalHuman.exe
pause
