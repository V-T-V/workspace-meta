# 架构设计

## 核心理念：流式管线重叠

实时数字人的生死线不是单环节延迟，而是**管线重叠**——绝不让下游等上游完整产出。

### 错误做法（串行，4-6 秒延迟）
```
用户说"你好呀"
  → ASR 识别完整句 (0.4s)
  → LLM 生成完整回复 (1.5s)
  → TTS 合成完整音频 (0.8s)
  → 唇形生成完整视频 (2s)
总延迟：4.7s ❌
```

### 正确做法（流式接力，<1.5s 首响应）
```
t=0.0s  ASR 检测句尾 → 喂 LLM
t=0.3s  LLM 吐首 token
t=0.6s  句切分器凑齐第一短句 → TTS
t=0.9s  TTS 出首段音频 → MuseTalk
t=1.2s  MuseTalk 出首帧 → WS 推送 → 用户看到开口
```

## 管线时序图

```
浏览器麦克风 ─WS─► server.py
                    │
              [session.feed_audio]
                    │
        ┌───────────┴───────────┐
        │ Stage 1: ASR           │  faster-whisper + VAD 分句
        │   audio_chunk ─► text  │  检测句尾 → put(text_queue)
        └───────────┬───────────┘
                    │ text_queue (buffer=16)
        ┌───────────┴───────────┐
        │ Stage 2: LLM           │  Ollama /api/chat stream=true
        │   text ─► token 流     │  逐 token → put(token_queue)
        └───────────┬───────────┘
                    │ token_queue (buffer=64)
        ┌───────────┴───────────┐
        │ Stage 2.5: 句切分器    │  ★ token 累积遇标点或 >15 字即切
        │   tokens ─► sentence   │  → put(sentence_queue)
        └───────────┬───────────┘
                    │ sentence_queue (buffer=8)
        ┌───────────┴───────────┐
        │ Stage 3: TTS           │  edge-tts / CosyVoice
        │   sentence ─► 音频流   │  → put(audio_queue)
        └───────────┬───────────┘
                    │ audio_queue (buffer=32)
        ┌───────────┴───────────┐
        │ Stage 4: 唇形          │  MuseTalk (subprocess)
        │   音频 ─► JPEG 帧流    │  → pusher.push_frame()
        └───────────┬───────────┘
                    │
                    ▼
              [WS + MJPEG] ─► 浏览器 Canvas
```

每个 Stage 是独立的 `asyncio.Task`，靠 `asyncio.Queue` 接力。
参照 auto-finance-assistant 的 `AnswerStream` 范式：buffered Queue + 后台 Task + ctx 取消。

## 显存预算（4060 8GB）

### MVP 默认配置（~4.4GB）
```
faster-whisper (small)   ~1.0GB
qwen2.5:3b (Ollama)      ~1.9GB
edge-tts                 0       (在线)
MuseTalk                 ~1.5GB
─────────────────────────────────
合计                     ~4.4GB  ✅ 留足余量
```

### 升级路径（~8.9GB，需 12GB+ 卡或分时加载）
```
whisper-large-v3         ~2.5GB
qwen2.5:7b               ~4.4GB
CosyVoice                ~1.5GB
MuseTalk                 ~1.5GB
─────────────────────────────────
合计                     ~9.9GB  ⚠️ 超 8GB
```

## 关键设计决策

### 1. 引擎抽象（参照 gpu-mesh engine.Engine）
四个抽象基类 `ASREngine/LLMEngine/TTSEngine/LipSyncEngine`，每个有 Mock + 真实实现。
缺依赖时自动降级 Mock，服务仍可启动。

### 2. 句切分器（Stage 2.5）
LLM 流式 token 不等整句，遇标点（。！？!?,.;）或累积 >15 字就立即喂 TTS。
这是降延迟的关键——否则 TTS 要等 LLM 说完整句才开工。

### 3. 帧编码协议（解决 MuseTalk subprocess 通信）
MuseTalk 的 subprocess 通过 stdin/stdout 与主进程通信。JPEG 帧长度不固定，
无法用 `read(N)` 分帧。自定义协议：
```
[4字节 big-endian 长度][payload]
长度=0xFFFFFFFF  → EOF
长度=0           → 段结束（一组帧/音频的边界）
```

### 4. Pusher 抽象（预留 WebRTC 升级）
MVP 用 WS + MJPEG。`Pusher` 抽象基类让后续 `WebRTCPusher`（aiortc）可无缝替换。

### 5. 会话串行化（参照 auto-finance LLMQueue）
单 GPU 防并发抢占：`Session` 用 `asyncio.Lock` 保证同一会话同时只跑一个 pipeline。

## 协议：WebSocket 消息

二进制消息，首字节为类型：
| 方向 | 类型 | 值 | 含义 |
|------|------|----|------|
| → 服务端 | AUDIO | 0x02 | PCM 音频 chunk |
| → 服务端 | UTTERANCE_END | 0x10 | 用户说完一句，触发 pipeline |
| ← 浏览器 | FRAME | 0x01 | JPEG 视频帧 |
| ← 浏览器 | AUDIO | 0x02 | MP3 音频（TTS 输出） |
| ← 浏览器 | TEXT | 0x03 | 字幕（[user]/[assistant]） |
| ← 浏览器 | END | 0x04 | 一段回复结束 |
| ← 浏览器 | LATENCY | 0x05 | 延迟统计 JSON |

## 复用工作区资产

| 来源 | 复用 | 方式 |
|------|------|------|
| gpu-mesh `engine.Engine` | 引擎抽象模式 | 模式参考 |
| gpu-mesh OpenAI 网关 | LLM 可选 backend | 运行时 HTTP |
| auto-finance `AnswerStream` | buffered Queue + ctx 取消 | 模式参考 |
| auto-finance `LLMQueue` | 两级信号量串行化 | 模式参考 |
| auto-finance `sseHeaders` | 禁缓冲头 | 模式参考 |
| lightai `pyproject.toml` | 核心依赖 + optional 分组 | 直接套用 |

## 不做（边界）
- 不改 gpu-mesh / auto-finance 源码（工作区约定）
- 不做多机分布式（MVP 单机，Phase 2 再说）
- 不做 aiortc WebRTC（WS+MJPEG 够用，预留 Pusher 抽象）

---

## 产品级特性（v0.2 新增）

### 对话历史持久化（SQLite）
数字人"记住"用户——进程重启后恢复对话上下文。
- `digitalhuman/store.py`：SQLiteStore（stdlib sqlite3，零依赖）
- `Session` 启动时 `load_history`，pipeline 成功后 `append_messages`
- `GET /sessions/{id}/history`：REST 端点供前端加载历史字幕
- 配置：`server.history_db: "data/digitalhuman.db"`（空则禁用，纯内存）

### Barge-in 打断
用户开口即停止当前回复（对话流畅如真人）。
- VAD 检测到用户说话（能量超阈值）→ `sess.cancel()` 停止当前 pipeline
- 推 `WS_MSG_INTERRUPT(0x11)` → 前端 `interruptPlayback()` 立即停音频 + 清队列
- 复用成熟的 cancel 机制（history 不被半截回复污染）

### 多角色热切换
用户选数字人性格 + 声音（商业化核心）。
- `PersonaConfig`（id/name/voice/system_prompt/portrait/greeting）
- `Session.set_persona()`：热切换 LLM system_prompt + TTS voice（运行时，不重连）
- `WS_MSG_SET_PERSONA(0x12)`：前端切换 → 服务端热更新
- `GET /personas`：列出可选角色
- config.gpu.yaml 预置 3 角色（晓晓温柔/云希活力/晓伊可爱）

### 主动开口欢迎语
连接后数字人主动开口（不冷场）。
- `Session.greet(greeting)`：跑 mini-pipeline（_FixedASR + _FixedLLM 绕过真实识别/推理）
- persona 的 `greeting` 字段触发

### Prometheus Metrics
生产运维可观测性。
- `GET /metrics`：Prometheus exposition format
- 指标：`dh_first_token_ms`/`dh_first_audio_ms`/`dh_first_frame_ms`（延迟分位 P50/P95）
- 计数：`dh_pipeline_total`/`dh_pipeline_cancel_total`/`dh_tts_fail_total`/`dh_barge_in_total`
- 实时：`dh_sessions_active`/`dh_audio_buffer_bytes`/`dh_uptime_seconds`

### 安全与限流
- `server.max_sessions`：并发上限（单 4060 建议 2，防 OOM）
- `server.auth_token`：WS 鉴权（客户端 `?token=xxx`）
- MuseTalk 常驻 proc 渲染互斥锁（防多 session 帧交错）

## 性能优化记录

### 已验证的延迟数据（CPU 环境）
```
LLM 首 token ~2.0s（Ollama qwen2.5:3b CPU；4060 GPU 预期 ~0.4s）
TTS ~1.9s（edge-tts 在线；本地 CosyVoice 预期 ~0.3s）
句切分 ~0.4s（凑够 max_chars 字）
唇形 ~0.015s（占位；真实 MuseTalk GPU 预期 ~0.1s）
```

### 4060 GPU 预期首帧 ~1.2s（达标 <1.5s）

### 关键优化项
- MuseTalk 常驻 subprocess（多轮首帧 -2s）
- LLM ClientSession 连接复用 + keep_alive + num_ctx 调优
- VAD 自动句尾触发（不等用户手动停止）
- 音频格式归一化（MP3→PCM16，修复真实 bug）
- audio_q 带超时 put（流式节奏，防误中断）
- MP3→PCM 解码异步化（不阻塞事件循环）
- 句切分器时间维度（短回复 500ms 无新 token 即切）
- 前端 createImageBitmap + Web Audio 无缝拼接 + rAF 节流
