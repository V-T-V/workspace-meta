# 4060 8GB 完整部署指南

## 一键部署

双击 `deploy_4060.bat`，脚本自动执行 8 步检测+安装+验证。

## 逐步手动部署（每步验证）

### STEP 1：检查 NVIDIA GPU

```bat
nvidia-smi
```
应显示：
- GPU 名称（如 NVIDIA GeForce RTX 4060）
- 显存 8192MB（8GB）
- 驱动版本 >= 535

**如果失败**：安装 NVIDIA 驱动 https://www.nvidia.com/drivers

### STEP 2：检查 Python

```bat
python --version
```
应显示 Python 3.10+

**如果失败**：安装 Python https://www.python.org/downloads/

### STEP 3：创建虚拟环境

```bat
python -m venv .venv
call .venv\Scripts\activate.bat
set NO_PROXY=127.0.0.1,localhost
python -m pip install --upgrade pip
```

### STEP 4：安装 CUDA PyTorch（关键！）

**默认 `pip install torch` 装的是 CPU 版！必须指定 CUDA：**

```bat
:: 先卸载可能的 CPU 版
pip uninstall -y torch torchvision

:: 装 CUDA 13.3（推荐）
pip install torch torchvision --index-url https://download.pytorch.org/whl/cu133

:: 或 CUDA 12.1（兼容老驱动）
pip install torch torchvision --index-url https://download.pytorch.org/whl/cu121
```

**验证（必须 True）：**
```bat
python -c "import torch; print(f'CUDA: {torch.cuda.is_available()}, GPU: {torch.cuda.get_device_name(0)}')"
```

### STEP 5：安装项目依赖

```bat
pip install fastapi "uvicorn[standard]" pyyaml aiohttp websockets edge-tts miniaudio numpy faster-whisper opencv-python
```

**验证：**
```bat
python -c "import fastapi, uvicorn, aiohttp, yaml, websockets, edge_tts, miniaudio, numpy, cv2; print('核心依赖 OK')"
python -c "from faster_whisper import WhisperModel; print('faster-whisper OK')"
python -c "import ctranslate2; print(f'CTranslate2 CUDA: {ctranslate2.get_cuda_device_count()}')"
```

### STEP 6：检查 Ollama + 拉模型

```bat
:: 安装 Ollama（如果没装）
:: 下载：https://ollama.com/download

:: 拉模型
ollama pull qwen2.5:3b

:: 验证
ollama list
:: 应显示 qwen2.5:3b

:: 测试推理
python -c "import urllib.request; r=urllib.request.urlopen('http://127.0.0.1:11434/api/version'); print('Ollama OK:', r.read().decode())"
```

### STEP 7：服务自检

```bat
python -m digitalhuman.server --self-test --port 8765
```

应显示：
```
[OK] Python 3.12.x
[OK] fastapi / uvicorn / aiohttp / yaml / websockets
[OK] ASR(WhisperASR)          ← GPU 版
[OK] LLM(OllamaLLM)
[OK] TTS(EdgeTTS)
[OK] LipSync(MuseTalkLipSync)
[OK] Ollama reachable
[OK] Model qwen2.5:3b
[OK] Port 8765
[OK] Health / OpenAI / Personas / Dashboard / Metrics

[OK] All passed!
```

### STEP 8：启动

```bat
python -m digitalhuman.server
```

浏览器打开 http://127.0.0.1:8000

## 显存预算

```
whisper-small     ~1.0 GB   (ASR)
qwen2.5:3b        ~1.9 GB   (LLM，Ollama 管理)
edge-tts          0         (在线)
MuseTalk          ~1.5 GB   (唇形)
─────────────────────────────
合计              ~4.4 GB   ✅ 8GB 余量 3.6GB
```

## 故障排查

| 问题 | 解决 |
|------|------|
| `torch.cuda.is_available()` = False | 装 CUDA 版 torch（不是默认 pip install） |
| faster-whisper 模型下载超时 | 设置镜像：`set HF_ENDPOINT=https://hf-mirror.com` |
| `model not found` | `ollama pull qwen2.5:3b` |
| 端口被占 | `python -m digitalhuman.server --port 8001` |
| ASR 识别不准 | config 里 `asr.model: medium`（占 2.5GB） |
| 首次启动慢 | 正常（模型预热 + ASR 模型下载），后续会快 |
