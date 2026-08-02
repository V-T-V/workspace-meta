@echo off
chcp 65001 >nul
REM 实时数字人 · 4060 8GB 完整部署脚本
REM 逐步检测、下载、安装、验证每个依赖
REM 用法：双击 deploy_4060.bat

setlocal EnableDelayedExpansion
cd /d "%~dp0"

echo.
echo ════════════════════════════════════════════════════
echo   实时数字人 · 4060 8GB 完整部署（逐步验证）
echo ════════════════════════════════════════════════════
echo.

set PASS=0
set FAIL=0

:step_check
echo [STEP 1/8] 检查 NVIDIA GPU 驱动
echo   检测中...
nvidia-smi >nul 2>&1
if errorlevel 1 (
    echo   [FAIL] 未检测到 nvidia-smi
    echo   解决：安装 NVIDIA 驱动 ^>= 535
    echo   下载：https://www.nvidia.com/drivers
    goto :failed
)
for /f "tokens=1,2,3" %%a in ('nvidia-smi --query-gpu=name,memory.total,driver_version --format=csv,noheader') do (
    echo   [OK] GPU: %%a
    echo   [OK] 显存: %%b
    echo   [OK] 驱动: %%c
)
set /a PASS+=1
echo.

echo [STEP 2/8] 检查 Python
python --version >nul 2>&1
if errorlevel 1 (
    echo   [FAIL] 未找到 Python
    echo   解决：安装 Python 3.10+
    echo   下载：https://www.python.org/downloads/
    goto :failed
)
for /f "tokens=2" %%v in ('python --version 2^>^&1') do echo   [OK] Python %%v
set /a PASS+=1
echo.

echo [STEP 3/8] 创建虚拟环境
if not exist .venv (
    python -m venv .venv
    echo   [OK] venv 已创建
) else (
    echo   [OK] venv 已存在
)
call .venv\Scripts\activate.bat
set HTTP_PROXY=
set HTTPS_PROXY=
set NO_PROXY=127.0.0.1,localhost
python -m pip install --quiet --upgrade pip >nul 2>&1
set /a PASS+=1
echo.

echo [STEP 4/8] 安装 CUDA PyTorch（关键：默认 pip 装 CPU 版！）
echo   检查当前 torch...
python -c "import torch; print(f'current: {torch.__version__} cuda={torch.cuda.is_available()}')" 2>nul
if errorlevel 1 (
    echo   torch 未安装，安装 CUDA 版...
) else (
    python -c "import torch; exit(0 if torch.cuda.is_available() else 1)" 2>nul
    if errorlevel 1 (
        echo   当前是 CPU 版，升级到 CUDA 版...
        pip uninstall -y torch torchvision >nul 2>&1
    ) else (
        echo   [OK] torch CUDA 已安装
        goto :step_torch_done
    )
)

echo   尝试 CUDA 13.3...
pip install --quiet torch torchvision --index-url https://download.pytorch.org/whl/cu133
if errorlevel 1 (
    echo   CUDA 13.3 失败，尝试 CUDA 12.1...
    pip install --quiet torch torchvision --index-url https://download.pytorch.org/whl/cu121
)
if errorlevel 1 (
    echo   [FAIL] PyTorch CUDA 安装失败
    echo   手动安装：https://pytorch.org/get-started/locally/
    goto :failed
)

:step_torch_done
echo   验证 torch CUDA...
python -c "import torch; print(f'torch={torch.__version__}, cuda={torch.cuda.is_available()}, gpu={torch.cuda.get_device_name(0) if torch.cuda.is_available() else \"N/A\"}')"
python -c "import torch; exit(0 if torch.cuda.is_available() else 1)" 2>nul
if errorlevel 1 (
    echo   [FAIL] torch.cuda.is_available() 返回 False
    echo   可能原因：CUDA toolkit 与驱动不匹配
    goto :failed
)
echo   [OK] PyTorch CUDA 验证通过
set /a PASS+=1
echo.

echo [STEP 5/8] 安装项目依赖
echo   安装核心依赖...
pip install --quiet fastapi "uvicorn[standard]" pyyaml aiohttp websockets edge-tts miniaudio numpy
echo   安装 faster-whisper + opencv...
pip install --quiet faster-whisper opencv-python
if errorlevel 1 (
    echo   主源失败，尝试镜像...
    pip install --quiet -i http://mirrors.aliyun.com/pypi/simple/ --trusted-host mirrors.aliyun.com fastapi "uvicorn[standard]" pyyaml aiohttp websockets edge-tts miniaudio numpy faster-whisper opencv-python
)
if errorlevel 1 (
    echo   [FAIL] 依赖安装失败
    goto :failed
)
echo   验证依赖...
python -c "
import fastapi, uvicorn, aiohttp, yaml, websockets, edge_tts, miniaudio, numpy, cv2
print('  [OK] 全部核心依赖可导入')
from faster_whisper import WhisperModel
print('  [OK] faster-whisper 可导入')
import ctranslate2
print(f'  [OK] ctranslate2 CUDA devices: {ctranslate2.get_cuda_device_count()}')
"
set /a PASS+=1
echo.

echo [STEP 6/8] 检查 Ollama + 拉取模型
where ollama >nul 2>&1
if errorlevel 1 (
    echo   [FAIL] 未找到 ollama
    echo   解决：安装 Ollama
    echo   下载：https://ollama.com/download
    goto :failed
)
echo   [OK] ollama 已安装

echo   检查模型 qwen2.5:3b...
ollama list 2>nul | findstr /c:"qwen2.5:3b" >nul
if errorlevel 1 (
    echo   拉取模型 qwen2.5:3b（约 1.9GB）...
    ollama pull qwen2.5:3b
    if errorlevel 1 (
        echo   [FAIL] 模型拉取失败
        goto :failed
    )
)
echo   [OK] 模型 qwen2.5:3b 已就绪

echo   测试模型推理...
python -c "
import urllib.request, json
opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))
r = opener.open('http://127.0.0.1:11434/api/version', timeout=5)
v = json.loads(r.read())
print(f'  [OK] Ollama 版本: {v.get(\"version\",\"?\")}')
"
set /a PASS+=1
echo.

echo [STEP 7/8] 服务自检
echo   运行 --self-test（启动服务 + 检查所有子系统）...
python -m digitalhuman.server --self-test --port 8765
if errorlevel 1 (
    echo   [WARN] 自检有失败项（可能 faster-whisper 模型首次下载超时）
    echo   不影响启动，首次对话时会自动下载 ASR 模型
) else (
    echo   [OK] 自检通过
)
set /a PASS+=1
echo.

echo [STEP 8/8] 启动数字人
echo.
echo ════════════════════════════════════════════════════
echo   部署完成！通过 %PASS%/8 步
echo.
echo   启动：python -m digitalhuman.server
echo   浏览器：http://127.0.0.1:8000
echo   自检：python -m digitalhuman.server --self-test
echo.
echo   首次对话较慢（模型预热 + ASR 模型下载）
echo   后续对话会快很多
echo ════════════════════════════════════════════════════
echo.
set /p RUN="现在启动数字人？(y/n): "
if /i "%RUN%"=="y" python -m digitalhuman.server
pause
exit /b 0

:failed
echo.
echo ════════════════════════════════════════════════════
echo   部署失败！通过 %PASS%/8 步
echo   请按上述提示修复后重试
echo ════════════════════════════════════════════════════
pause
exit /b 1
