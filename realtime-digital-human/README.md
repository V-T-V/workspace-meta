# realtime-digital-human

单机实时数字人服务。对着麦克风说话，数字人开口回话，首响应延迟 <1.5s。

## 核心特性

### 对话体验
- **流式管线重叠**：ASR → LLM → 句切分 → TTS → 唇形，4 阶段独立 asyncio Task 靠 Queue 接力，绝不串行等待
- **Barge-in 打断**：用户开口即停止当前回复，对话流畅如真人
- **对话记忆**：SQLite 持久化，进程重启后数字人仍记得你
- **多角色切换**：3 个预置角色（温柔客服/活力助手/可爱少女），运行时热切换音色+人设
- **主动开口**：连接后数字人主动欢迎，不冷场

### 性能（4060 GPU 预期首帧 ~1.2s）
- **首响应 <1.5s**：句切分器让 TTS 在 LLM 还没说完时就开工
- **MuseTalk 常驻**：多轮对话省去重复启动（第 2 轮首帧 -2s）
- **VAD 自动句尾**：不等用户手动停止，说完即触发
- **4060 8GB 可跑**：MVP 默认配置显存占用 ~4.4GB

### 工程化
- **引擎可插拔**：四环节抽象基类 + 工厂分发，Mock/真实实现可互换
- **全链路容错**：TTS/Lipsync/WS 单点失败不崩溃，无死锁
- **OpenAI 兼容 API**：`/v1/chat/completions` + `/v1/models`，可被任何 OpenAI 客户端接入
- **Prometheus Metrics**：`/metrics` 端点暴露延迟分位/失败率/session 数
- **Web 仪表盘**：首页底部实时显示 sessions/延迟 P50/P95/错误率/运行时长
- **WS 鉴权 + 并发限流**：`auth_token` + `max_sessions` 防 OOM
- **Windows 安装包**：PyInstaller + Inno Setup 一键打包（CUDA 13.3）

## 快速开始（极简部署）

### 3 个 tier，按需选择

| Tier | 功能 | 依赖大小 | 命令 |
|------|------|---------|------|
| **0 体验版** | 文字对话（无语音/唇形） | ~30MB | `quickstart.bat 0` |
| **1 标准版** | +语音 TTS（edge-tts 在线） | ~80MB | `quickstart.bat 1` |
| **2 完整版** | +ASR +唇形（需 GPU） | ~3GB | `quickstart.bat 2` |

不传参数自动探测：有 GPU 装 tier 2，否则 tier 1。

### Windows（极简）
```bat
quickstart.bat    :: 自动探测 + 分层装依赖 + 拉 Ollama 模型
start.bat         :: 启动（零参数），浏览器开 http://127.0.0.1:8000
```

### Linux / macOS
```bash
./quickstart.sh   # 自动探测 + 分层装依赖 + 拉 Ollama 模型
python -m digitalhuman.server  # 启动
```

### 手动 2 步（最简）
```bash
pip install fastapi "uvicorn[standard]" pyyaml edge-tts miniaudio aiohttp
python -m digitalhuman.server    # 零配置启动
```

### 手动方式（3 步）
```bash
pip install fastapi "uvicorn[standard]" pyyaml aiohttp websockets edge-tts numpy opencv-python
ollama pull qwen2.5:3b            # 拉模型（需先装 ollama.com）
python -m digitalhuman.server     # 零配置启动（无 config.yaml 也能跑）
```

> 💡 **零配置**：不传 `-c` 时自动用内置默认配置，缺 GPU/ASR/MuseTalk 时自动降级占位实现，服务始终能启动。

### 环境检查（可选）
```bash
python scripts/setup_models.py --all    # 检查 + 自动装缺失依赖 + 拉模型
```

## 打包成单 EXE（独立部署）

### Lite 单 EXE（推荐快速体验：38MB，双击即用）
```bat
build_exe.bat    :: 一键打包，产出 dist/DigitalHuman-Lite.exe（~38MB，3-5 分钟）
```
双击 `DigitalHuman-Lite.exe` → 浏览器开 http://127.0.0.1:8000。无需 Python/依赖/config。

### 完整离线包（含全部依赖，目标机无需装任何东西）
```bat
build_full.bat   :: 一键打包（torch+faster-whisper+opencv 全内嵌，~1GB，5-15 分钟）
```
产出 `dist/DigitalHuman/DigitalHuman.exe`（onedir，双击即用）。

### 完整 GPU 版（CUDA 13.3，4060 机器上打）
```bat
build_windows.bat    :: CUDA torch + 完整依赖（~1.5GB）
```

详细见 [`docs/DEPLOY_PACKAGING.md`](./docs/DEPLOY_PACKAGING.md)。


## 架构（一句话）
```
麦克风 ─► ASR ─► LLM ─► 句切分 ─► TTS ─► 唇形 ─► WS ─► 浏览器
         (流)   (token流) (短句流)  (音频流) (JPEG帧流)
         每一段都是流式接力，不等上游完整产出
```
详见 `docs/ARCHITECTURE.md`。

## 运行日志（诊断黑屏/无声/无响应）

详细运行日志默认写入 **`data/digitalhuman.log`**（5MB×3 轮转，与控制台同步输出）：

| 阶段 | 日志内容 |
|------|---------|
| 启动 | 日志文件路径、配置来源、ASR 模型来源（本地目录/联网）、各引擎装配耗时与降级原因、web 目录存在性、形象图大小 |
| WS | 连接开始（client IP）、鉴权、接入/断开、会话时长、收音频量、管线轮次 |
| 对话 | 用户输入、回复摘要、管线各阶段延迟（first_token/first_sentence/first_audio/first_frame）、token 数/句数/帧数 |
| 错误 | 完整堆栈 + 上下文（如 ASR 降级原因、TTS 单句失败、Ollama 500 详情） |

常见黑屏排查：
1. 页面全黑/残缺 → 看日志是否报 `Too much data for declared Content-Length`（旧版 favicon 204 带 body 的 bug，已修复）与 web 目录是否存在
2. 形象区黑但 UI 正常 → WS 未连上，看 WS 会话日志
3. 有语音输入但无回复 → 看 ASR 是否降级 Mock（日志有"来源=本地目录/联网模式"）与 LLM 是否 Mock（Ollama 不可达）

## 配置项速查
| 段 | 关键字段 | 默认 | 说明 |
|----|---------|------|------|
| asr | backend / model | whisper / small | tiny/base/small/medium/large-v3 |
| llm | backend / model | ollama / qwen2.5:3b | 可选 gpu_mesh |
| tts | backend / voice | edge / zh-CN-XiaoxiaoNeural | 可选 cosyvoice |
| lipsync | backend | musetalk | 可选 disabled |
| pusher | backend | ws_mjpeg | 可选 disabled |
| sentence_splitter | max_chars | 15 | 越小延迟越低但 TTS 调用越频繁 |

## 测试
```bash
pip install -r requirements-dev.txt
pytest tests/ -v
```

## 延迟基准
```bash
python scripts/bench_latency.py -c config.yaml
```

## 相关文档
- [`AGENTS.md`](./AGENTS.md) — 项目约定与架构决策
- [`docs/ARCHITECTURE.md`](./docs/ARCHITECTURE.md) — 管线时序图与选型理由
- [`docs/DEPLOY.md`](./docs/DEPLOY.md) — 单机部署
