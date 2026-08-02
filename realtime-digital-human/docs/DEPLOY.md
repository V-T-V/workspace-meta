# 部署指南

## 单机部署（Windows + 4060 8GB）

### 1. 环境准备

#### Python 依赖
```bash
cd realtime-digital-human
pip install -r requirements.txt
```

#### GPU 驱动与 CUDA
- NVIDIA 驱动 ≥ 535
- CUDA 11.8 或 12.1（torch 对应版本）
- 验证：`python -c "import torch; print(torch.cuda.is_available())"`

#### Ollama + 模型
```bash
# 安装：见 ollama.com
ollama pull qwen2.5:3b
# 验证：curl http://127.0.0.1:11434/api/tags
```

#### MuseTalk（可选，重度）
MuseTalk 依赖较重，按官方文档安装：
```bash
# 见 github.com/TMElyralab/MuseTalk
git clone https://github.com/TMElyralab/MuseTalk
cd MuseTalk
pip install -r requirements.txt
```
> MVP 阶段若不装 MuseTalk，唇形自动降级为占位渲染（需 opencv-python）。

#### CosyVoice（可选，音色克隆）
```bash
# 见 github.com/FunAudioLLM/CosyVoice
# 默认用 edge-tts（在线，零显存），无需安装
```

### 2. 配置

```bash
cp config.example.yaml config.yaml
# 编辑 config.yaml，按本机环境调整
```

关键项：
- `asr.device`: 有 GPU 用 `cuda`，无则 `cpu`（慢但能跑）
- `llm.base_url`: Ollama 地址，默认本机 11434
- `lipsync.portrait`: 数字人形象图路径（512x512 正脸免冠照）
- `tts.voice`: edge-tts 音色（`edge-tts --list-voices` 查全量）

#### 产品级特性配置（v0.2 新增）

- `server.history_db`: 对话历史持久化路径（`data/digitalhuman.db`）。空则禁用（纯内存）
- `server.max_sessions`: 并发上限（单 4060 建议 2，防 OOM）
- `server.auth_token`: WS 鉴权 token（局域网部署建议设，客户端 `?token=xxx` 连接）
- `personas`: 多角色配置（list of `{id, name, voice, system_prompt, greeting}`）
- `asr.vad_auto_trigger`: VAD 自动句尾 + Barge-in 打断（默认 true）

#### Prometheus 监控接入

```yaml
# prometheus.yml
scrape_configs:
  - job_name: 'digitalhuman'
    static_configs:
      - targets: ['localhost:8000']
    metrics_path: '/metrics'
```

指标：`dh_first_frame_ms`（首帧延迟 P50/P95）、`dh_pipeline_total`、`dh_tts_fail_total` 等。

#### 多角色配置示例（config.gpu.yaml）

```yaml
personas:
  default_id: "xiaoxiao"
  list:
    - id: "xiaoxiao"
      name: "晓晓（温柔客服）"
      voice: "zh-CN-XiaoxiaoNeural"
      system_prompt: "你是温柔的数字人客服。"
      greeting: "你好，很高兴见到你。"
    - id: "yunxi"
      name: "云希（活力助手）"
      voice: "zh-CN-YunxiNeural"
      system_prompt: "你是活力四射的数字人助手。"
```

### 3. 准备形象素材
```
assets/
├── portrait_sample.png   # 数字人形象（512x512，正脸，免冠，纯色背景）
└── voice_sample.wav      # CosyVoice 音色样本（5-10s 清晰人声，可选）
```

### 4. 检查环境
```bash
python scripts/setup_models.py
```
缺的依赖会标 ❌，服务仍可启动（自动降级 Mock）。

### 5. 启动
```bash
python -m digitalhuman.server -c config.yaml
# 浏览器开 http://127.0.0.1:8000
# 点麦克风按钮，说话
```

DEBUG 日志：
```bash
python -m digitalhuman.server -c config.yaml -v
```

### 6. 延迟基准
```bash
python scripts/bench_latency.py -c config.yaml
```

## Windows 服务化（开机自启）

MVP 不内建服务化。推荐用 NSSM：
```bash
# 下载 nssm.exe
nssm install DigitalHuman "C:\path\to\python.exe" "-m digitalhuman.server -c C:\path\to\config.yaml"
nssm start DigitalHuman
```

或用 Python 的 `pywin32`：
```python
# 自行实现，参考 auto-finance-assistant cmd/server/service.go 的思路
```

## 常见问题

### Q: 浏览器连不上 / 503
A: 系统代理拦截了 localhost。设置 `NO_PROXY=127.0.0.1,localhost`。

### Q: Ollama 调用超时
A: 首次推理模型加载慢（~5s）。预热：`curl http://127.0.0.1:11434/api/generate -d '{"model":"qwen2.5:3b","prompt":"hi"}'`。

### Q: 显存不足（OOM）
A: 降级配置：
- `asr.model`: small → tiny
- `llm.model`: qwen2.5:3b → qwen2.5:1.5b
- `tts.backend`: cosyvoice → edge（省 1.5GB）

### Q: 麦克风没声音
A: 浏览器需 HTTPS 或 localhost 才允许麦克风权限。本机用 localhost 即可。

### Q: MuseTalk subprocess 报错
A: 检查 `scripts/musetalk_render.py` 是否可独立运行：
```bash
python -c "import struct,sys; sys.stdout.buffer.write(struct.pack('>I',16000)+b'\x00'*32000+struct.pack('>I',0))" | python scripts/musetalk_render.py --fps 5 | python -c "import sys,struct; d=sys.stdin.buffer.read(); print('frames bytes:', len(d))"
```

## 性能调优

| 目标 | 调整 | 效果 |
|------|------|------|
| 降首 token 延迟 | `llm.model` 换更小（1.5b/0.5b） | -200ms |
| 降首帧延迟 | `sentence_splitter.max_chars` 减到 10 | 句切分更早，TTS 更快开工 |
| 降总回复时长 | `llm.max_tokens` 减到 64 | LLM 说更短，整体更快 |
| 提升唇形质量 | MuseTalk 换真实模型（非占位） | 口型更准 |
| 降显存 | `tts.backend: edge` | 省 1.5GB（CosyVoice） |
