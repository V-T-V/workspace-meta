# 4060 GPU 部署指南

本指南让数字人在 **NVIDIA 4060（8GB）** 上跑真实 ASR + 真实唇形（MuseTalk），而非降级占位。

## 显存预算

```
faster-whisper (small)   ~1.0 GB   (ASR)
qwen2.5:3b (Ollama)      ~1.9 GB   (LLM)
edge-tts                 0         (在线)
MuseTalk                 ~1.5 GB   (唇形)
───────────────────────────────────
合计                     ~4.4 GB   ✅ 4060 8GB 充裕
```

升级路径（12GB+ 卡）：
```
whisper-medium + qwen2.5:7b + CosyVoice + MuseTalk ≈ 9.5 GB
```

## 一键部署（Windows 4060）

```bat
deploy_gpu.bat    :: 装 CUDA torch + faster-whisper + 拉模型
run.bat           :: 启动（用 config.gpu.yaml）
```

或手动指定 GPU 配置启动：
```bat
python -m digitalhuman.server -c config.gpu.yaml
```

## 手动部署步骤

### 1. 确认 GPU 驱动
```bat
nvidia-smi
:: 应显示 4060 + 驱动版本 >= 535
```

### 2. 安装 CUDA 版 PyTorch（关键）
默认 `pip install torch` 装的是 **CPU 版**，必须指定 CUDA：
```bat
pip install torch torchvision --index-url https://download.pytorch.org/whl/cu121
:: 或 CUDA 11.8：
pip install torch torchvision --index-url https://download.pytorch.org/whl/cu118
```
验证：
```bat
python -c "import torch; print(torch.cuda.is_available())"
:: 应输出 True
```

### 3. 安装 faster-whisper + 核心依赖
```bat
pip install faster-whisper fastapi "uvicorn[standard]" pyyaml aiohttp websockets edge-tts numpy opencv-python
```

### 4. Ollama + 模型
```bat
:: 装 ollama：https://ollama.com/download
ollama pull qwen2.5:3b
```

### 5. 启动（ASR 自动用 GPU）
```bat
python -m digitalhuman.server -c config.gpu.yaml
```
日志应显示：`ASR 使用 GPU (int8_float16)` + `whisper 模型就绪`

## MuseTalk 真实唇形（可选，提升口型真实度）

默认 `musetalk_render.py` 在缺 MuseTalk 时用占位渲染（灰阶 + 嘴部亮度动画）。
要真实唇形，需 clone 官方 MuseTalk：

### 安装 MuseTalk
```bat
:: 在项目父目录 clone
cd ..
git clone https://github.com/TMElyralab/MuseTalk
cd MuseTalk
pip install -r requirements.txt
:: 按官方文档下载模型到 models/musetalkV15/
```

### 配置环境变量
```bat
:: 启动前设置（或在 run_gpu.bat 里加）
set MUSETALK_HOME=..\MuseTalk
set MUSETALK_MODEL=..\MuseTalk\models\musetalkV15
```

### 验证
启动后日志应显示 `[musetalk] 加载 MuseTalk 模型（device=cuda）...`，
而非 `MuseTalk 不可用，使用占位渲染`。

> MuseTalk 接口可能随版本变动。若渲染失败，对照官方 `scripts/realtime_inference.py`
> 调整 `scripts/musetalk_render.py` 的 `_render_with_musetalk_api`。

## 性能调优

| 现象 | 调整 |
|------|------|
| 显存不足（OOM） | `asr.model`: small→tiny；或 `tts.backend`: cosyvoice→edge |
| 首帧延迟高 | `sentence_splitter.max_chars`: 12→8（更早切句喂 TTS） |
| 唇形不真实 | 装 MuseTalk（见上），换真实人像图 |
| GPU 利用率低 | `llm.max_tokens` 调高（让 LLM 多说） |
| 想要克隆音色 | 装 CosyVoice，`tts.backend: cosyvoice`（+1.5G 显存） |

## 常见问题

### Q: `torch.cuda.is_available()` 返回 False
A: 装的是 CPU 版 torch。卸载后重装 CUDA 版：
```bat
pip uninstall -y torch torchvision
pip install torch torchvision --index-url https://download.pytorch.org/whl/cu121
```

### Q: faster-whisper 报 `no CUDA-capable device`
A: CUDA toolkit 与驱动版本不匹配。驱动 >= 535 配 CUDA 12.1，或换 cu118。

### Q: MuseTalk 渲染失败降级占位
A: 检查 `MUSETALK_HOME` 路径正确、模型已下载。看日志的具体错误。

### Q: 多人同时访问卡顿
A: 单 4060 同时只能流畅服务 1-2 路会话（显存 + 算力限制）。
   多路需 gpu-mesh 多机调度（见根目录 gpu-mesh 项目）。
