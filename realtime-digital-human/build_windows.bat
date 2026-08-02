@echo off
chcp 65001 >nul
REM 实时数字人 · Windows 完整离线包打包脚本（含 CUDA 13.3 torch）
REM 用法：双击或 build_windows.bat
REM 产出：dist/DigitalHuman/（PyInstaller onedir）+ dist/DigitalHuman-Setup.exe（Inno Setup，若装了）

setlocal

echo.
echo ════════════════════════════════════════════════
echo   实时数字人 · Windows 离线包打包（CUDA 13.3）
echo ════════════════════════════════════════════════
echo.

cd /d "%~dp0"

REM 0. 环境检查
echo [0/6] 环境检查...
python --version >nul 2>&1
if errorlevel 1 (
    echo   ❌ 未找到 python，请装 Python 3.10+
    pause
    exit /b 1
)
for /f "tokens=2" %%v in ('python --version 2^>^&1') do echo   ✓ Python %%v

nvidia-smi >nul 2>&1
if errorlevel 1 (
    echo   ⚠ 未检测到 NVIDIA GPU（将打 CPU 版，4060 机器需重新打）
) else (
    echo   ✓ 检测到 NVIDIA GPU
)

REM 1. 清理旧 venv + 旧产物
echo.
echo [1/6] 清理旧产物...
if exist .venv rmdir /s /q .venv
if exist build rmdir /s /q build
if exist dist rmdir /s /q dist
echo   ✓ 已清理

REM 2. 建 clean venv
echo.
echo [2/6] 创建干净 venv（用于打包）...
python -m venv .venv
if errorlevel 1 (
    echo   ❌ venv 创建失败
    pause
    exit /b 1
)
call .venv\Scripts\activate.bat
python -m pip install --quiet --upgrade pip
echo   ✓ venv 已创建

REM 3. 装 CUDA 13.3 PyTorch（关键：默认 pip 装的是 CPU 版）
echo.
echo [3/6] 安装 CUDA 13.3 PyTorch（约 2.5GB，请耐心等待）...
set HTTP_PROXY=
set HTTPS_PROXY=
pip install --quiet torch torchvision --index-url https://download.pytorch.org/whl/cu133
if errorlevel 1 (
    echo   ⚠ cu133 安装失败，尝试 cu130（PyTorch 当前最高稳定）...
    pip install --quiet torch torchvision --index-url https://download.pytorch.org/whl/cu130
)
if errorlevel 1 (
    echo   ❌ CUDA torch 安装失败，请手动检查 PyTorch 对你 CUDA 版本的支持
    echo      参考：https://pytorch.org/get-started/locally/
    pause
    exit /b 1
)
echo   ✓ PyTorch CUDA 已安装
python -c "import torch; print(f'  torch={torch.__version__}, cuda={torch.cuda.is_available()}, gpu={torch.cuda.get_device_name(0) if torch.cuda.is_available() else \"N/A\"}')"

REM 4. 装其他依赖 + PyInstaller
echo.
echo [4/6] 安装 faster-whisper + opencv + edge-tts + PyInstaller...
pip install --quiet faster-whisper opencv-python edge-tts numpy pyyaml
pip install --quiet fastapi "uvicorn[standard]" aiohttp websockets
pip install --quiet pyinstaller
if errorlevel 1 (
    echo   ❌ 依赖安装失败
    pause
    exit /b 1
)
echo   ✓ 全部依赖已安装

REM 5. PyInstaller 打包
echo.
echo [5/6] PyInstaller 打包（onedir 模式，约 5-15 分钟）...
REM 关键：用环境变量传项目根（spec 内部路径计算在 PyInstaller 执行时不可靠）
set DH_PROJECT_ROOT=%CD%
pyinstaller packaging\digitalhuman.spec --noconfirm --clean
REM 自动复制 Start.bat（启动提示脚本）
copy /Y packaging\Start.bat dist\DigitalHuman\Start.bat >nul 2>&1
if errorlevel 1 (
    echo   ❌ PyInstaller 打包失败
    echo      常见原因：torch/ctranslate2 动态库收集不全，看 spec 的 hooks/
    pause
    exit /b 1
)
echo   ✓ 打包完成

REM 验证产出
if exist dist\DigitalHuman\DigitalHuman.exe (
    for /f %%S in ("dist\DigitalHuman\DigitalHuman.exe") do echo   产出：dist\DigitalHuman\ （%%~zS bytes）
    dir dist\DigitalHuman /s /-c | findstr "个文件" | head -1
) else (
    echo   ⚠ 未找到 dist\DigitalHuman\DigitalHuman.exe
)

REM 6. 可选：Inno Setup 打安装包
echo.
echo [6/6] 检查 Inno Setup（可选，生成 .exe 安装包）...
where iscc >nul 2>&1
if errorlevel 1 (
    echo   ℹ 未安装 Inno Setup，跳过安装包生成
    echo      装 Inno Setup 后运行：iscc packaging\installer.iss
    echo      下载：https://jrsoftware.org/isdl.php
) else (
    echo   ✓ 检测到 Inno Setup，生成安装包...
    iscc packaging\installer.iss
    if errorlevel 1 (
        echo   ⚠ Inno Setup 编译失败，但 PyInstaller 产物已就绪
    ) else (
        echo   ✓ 安装包已生成：dist\DigitalHuman-Setup.exe
    )
)

echo.
echo ════════════════════════════════════════════════
echo   ✓ 打包完成！
echo.
echo   双击启动：dist\DigitalHuman\Start.bat
echo   或直接运行：dist\DigitalHuman\DigitalHuman.exe
echo   或用安装包：dist\DigitalHuman-Setup.exe（若有）
echo.
echo   首次运行会自动检测 Ollama（若未装会提示）
echo   模型需先拉取：ollama pull qwen2.5:3b
echo ════════════════════════════════════════════════
echo.
pause
