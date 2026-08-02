# realtime-digital-human · AGENTS.md

单机实时数字人服务。**核心是流式管线重叠**，不是各环节的简单拼接。

## 一句话
浏览器开页面 → 对着麦克风说话 → ≤1.5s 看到数字人开口回话。单机 4060 8GB 可跑。

## 技术栈
- **Python 3.10+** / FastAPI / uvicorn / asyncio（实时管线生态全在 Python）
- ASR：faster-whisper（GPU，int8，~1GB）
- LLM：Ollama qwen2.5:3b（~1.9GB，通过 `/api/chat stream=true`）
- TTS：edge-tts（默认，在线零显存）/ CosyVoice（可选，本地可克隆）
- 唇形：MuseTalk（~1.5GB，subprocess 桥接）
- 推流：WebSocket + MJPEG（默认）/ WebRTC（预留）
- 测试：pytest + pytest-asyncio

## 关键架构决策
1. **流式管线重叠（生死线）**：4 阶段（ASR/LLM/TTS/唇形）均为独立 asyncio Task，靠 `asyncio.Queue` 接力。绝不串行等待，否则 4s+ 延迟。
2. **句切分器（Stage 2.5）**：LLM token 累积遇标点或 >15 字即切，立即喂 TTS。不等整句说完，这是降延迟的关键。
3. **引擎抽象**：`ASREngine/LLMEngine/TTSEngine/LipSyncEngine` 四个抽象基类（参照 gpu-mesh `engine.Engine`），Mock 与真实实现可互换。
4. **Pusher 抽象**：MVP 用 WS+MJPEG，预留 WebRTC 升级，业务层无感。
5. **帧编码协议**：MuseTalk subprocess 通信用 4 字节长度前缀帧协议（`frames.py`），解决"JPEG 长度不固定如何分帧"问题。

## 显存预算（MVP 默认）
```
whisper-small  ~1.0GB
qwen2.5:3b     ~1.9GB
edge-tts       0
MuseTalk       ~1.5GB
合计           ~4.4GB  ✅ 留足余量
```
升级路径：qwen2.5:7b + CosyVoice + whisper-large-v3 ≈ 8.9GB（需 12GB+ 卡或分时加载）。

## 目录约定
- `digitalhuman/engines/`：四环节抽象 + 实现（每环节一文件，工厂分发）
- `digitalhuman/pusher/`：推流抽象 + 实现
- `digitalhuman/pipeline.py`：管线协调器（4 阶段 Task 调度 + `_put_timed` 流式节奏）
- `digitalhuman/session.py`：单会话状态机（Queue 生命周期 + ctx 取消 + history 裁剪 + set_persona 热切换 + greet 欢迎语）
- `digitalhuman/store.py`：SQLite 持久化（对话历史，stdlib 零依赖）
- `digitalhuman/metrics.py`：Prometheus 指标注册表（延迟分位/失败率/session 数）
- `digitalhuman/frames.py`：帧编码协议 + WS 消息类型（含 INTERRUPT/SET_PERSONA）
- `digitalhuman/config.py`：dataclass 配置（含 PersonasConfig 多角色 + history_db + auth_token）
- `web/`：前端（原生 HTML/JS，无框架；createImageBitmap + Web Audio + 角色选择器 + 历史加载）
- `tests/`：pytest（108 测试），**改任何引擎/管线前先跑一遍**
- `scripts/`：验证/基准/模拟（verify_e2e / bench_multi_turn / scenarios_simulator）
- `packaging/`：PyInstaller spec + Inno Setup（Windows 安装包）

## 产品级特性
- **对话记忆**：SQLite 持久化，重启恢复历史（`store.py`）
- **Barge-in 打断**：VAD 检测用户开口 → cancel 当前回复 + 前端停音频
- **多角色热切换**：`Session.set_persona()` 运行时切 voice/system_prompt（`config.py` PersonasConfig）
- **主动欢迎语**：`Session.greet()` 连接后主动开口
- **Prometheus Metrics**：`/metrics` 端点（延迟/失败率/session 数）
- **安全限流**：`max_sessions` 并发上限 + `auth_token` WS 鉴权

## 复用工作区资产
- **gpu-mesh** `engine.Engine` 抽象模式（非代码）+ 可选 LLM 网关 `/v1/chat/completions`
- **auto-finance-assistant** `AnswerStream`（buffered channel + 后台 Task + ctx 取消）+ `LLMQueue` 两级信号量 + `sseHeaders` 禁缓冲
- **lightai** pyproject 范式（核心依赖 + optional-dependencies 分组）

## 跑测试
```bash
pip install -r requirements-dev.txt
pytest tests/ -v
```

## 启动（MVP 达成后）
```bash
python -m digitalhuman.server -c config.dev.yaml
# 浏览器开 http://127.0.0.1:8000
```

## 不做（边界）
- 不改 gpu-mesh / auto-finance 源码（工作区约定）
- 不做多机分布式（单机已稳，多机是 gpu-mesh 的领域）
- 不做 aiortc WebRTC（WS+MJPEG 在 LAN 够用，预留 Pusher 抽象）
- 不做形象生成/克隆前端 UI（用静态照片 + config 配置）
- 不做 Pluggable Stage 链（YAGNI，5 个固定 stage 够用）
