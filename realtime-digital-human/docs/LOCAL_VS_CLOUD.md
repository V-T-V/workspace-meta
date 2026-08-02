# 本地 vs 云端：四环节可配置混搭指南

数字人管线有 4 个 AI 环节，**每个都既支持本地也支持云、可独立配置**。
按需混搭：GPU 富余就全本地（隐私+零延迟），GPU 紧张就把 AI 能力上云（显存全留唇形）。

## 四环节总览

每个环节都有 4 类后端：**本地 / 云端 / mock / disabled**。

| 环节 | 本地选项 | 云端选项 | mock（纯演示） | GPU 占用 |
|------|---------|---------|--------------|---------|
| **ASR** | `whisper`（faster-whisper） | `cloud`/`groq`/`openai`/`deepinfra`/`siliconflow` | `mock`（预置回复） | 本地 ~1GB / 云 0 / mock 0 |
| **LLM** | `ollama`（qwen2.5 等） | `openai`/`deepseek`/`glm`/`qwen`/`kimi` | `mock`（预置回复） | 本地 ~1.9GB / 云 0 / mock 0 |
| **TTS** | `cosyvoice`（可克隆音色） | `edge`（微软云，零显存） | `mock`（静音 PCM） | 本地 ~1.5GB / 云 0 / mock 0 |
| **唇形** | `musetalk`（实时帧渲染） | ❌ 无（必须本地） | `mock`（占位帧） | 本地 ~1.5GB / mock 0 |

> **mock vs disabled 区别**：mock 产生合理的占位输出（预置回复/静音音频/占位帧），让管线完整跑通用于演示；disabled 产生空输出。
> 适合场景：无 GPU / 无网络 / 纯演示 / 单元测试 / CI。

## 四种典型配置方案

### 方案 A：全本地（隐私优先，零网络依赖）
适合：有 12GB+ 显存、内网/离线、数据不出域
```yaml
asr:     { backend: whisper, model: small, device: auto }
llm:     { backend: ollama, base_url: http://127.0.0.1:11434, model: qwen2.5:3b }
tts:     { backend: cosyvoice, voice_sample: assets/voice_sample.wav }
lipsync: { backend: musetalk }
```
显存：~4.4GB（ASR 1 + LLM 1.9 + TTS 1.5 + 唇形 1.5... 实际唇形含在 musetalk）

### 方案 B：全云 AI（GPU 全留唇形，4060 8GB 首选）★推荐
适合：4060 8GB、显存紧张、可联网
```yaml
asr:     { backend: groq, model: whisper-large-v3, api_key: "" }   # ASR 上云省 1GB
llm:     { backend: deepseek, model: deepseek-chat, api_key: "" }  # LLM 上云省 1.9GB
tts:     { backend: edge }                                          # 本就是云
lipsync: { backend: musetalk }                                      # 唯一本地 GPU
```
显存：~1.5GB（仅唇形）。环境变量设 key：
```cmd
set DH_ASR_API_KEY=sk-你的groq-key
set DH_LLM_API_KEY=sk-你的deepseek-key
```

### 方案 C：混合（ASR/LLM 本地，TTS 上云）
适合：要本地推理质量但不在乎 TTS
```yaml
asr:     { backend: whisper, model: small, device: cuda }
llm:     { backend: ollama, model: qwen2.5:3b }
tts:     { backend: edge }
lipsync: { backend: musetalk }
```
显存：~2.9GB（ASR 1 + LLM 1.9）

### 方案 D：全 mock（纯演示，0 GPU 0 网络）★演示/测试用
适合：无 GPU / 无网络 / 纯演示 / CI 单元测试
```yaml
asr:     { backend: mock }      # 预置固定回复，不等识别
llm:     { backend: mock }      # 预置固定回复，不等推理
tts:     { backend: mock }      # 静音 PCM，不等合成
lipsync: { backend: mock }      # 占位帧，不渲染真帧
```
显存：**0 GB**。不依赖任何模型、网络、GPU——管线完整跑通，用于演示 UI / 验证流程 / CI。
> 注意：mock 模式下数字人不会"真的"说话（无真实识别/回复/语音/口型），只是按节奏走完整条管线。

## 密钥安全（重要）

**线上云 API 的 key 一律用环境变量，不要写进 config.yaml**（避免进 git/EXE）：

| 环节 | 环境变量 | 优先级 |
|------|---------|--------|
| LLM | `DH_LLM_API_KEY` | 环境变量 > yaml 的 `api_key` |
| ASR | `DH_ASR_API_KEY` | 环境变量 > yaml 的 `api_key` |

```powershell
# PowerShell（当前会话）
$env:DH_LLM_API_KEY = "sk-xxx"
$env:DH_ASR_API_KEY = "sk-yyy"

# 或系统级永久（重开终端生效）
[Environment]::SetEnvironmentVariable("DH_LLM_API_KEY", "sk-xxx", "User")
```

## 预设云厂商（写 backend 名即自动套 base_url）

**LLM**（OpenAI 兼容 `/v1/chat/completions`）：
| backend | 厂商 | 默认 base_url |
|---------|------|--------------|
| `deepseek` | DeepSeek | https://api.deepseek.com |
| `glm` | 智谱 GLM | https://open.bigmodel.cn/api/paas/v4 |
| `openai` | OpenAI | https://api.openai.com/v1 |
| `qwen` | 通义千问 | https://dashscope.aliyuncs.com/compatible-mode/v1 |
| `kimi` | Kimi | https://api.moonshot.cn/v1 |

**ASR**（OpenAI 兼容 `/v1/audio/transcriptions`）：
| backend | 厂商 | 默认 base_url |
|---------|------|--------------|
| `groq` | Groq（whisper-large-v3，极快） | https://api.groq.com/openai/v1 |
| `openai` | OpenAI | https://api.openai.com/v1 |
| `deepinfra` | DeepInfra | https://api.deepinfra.com/v1/openai |
| `siliconflow` | SiliconFlow | https://api.siliconflow.cn/v1 |

> 自建/代理服务：`base_url` 显式填写即可覆盖预设。

## 验证当前配置的 GPU 占用

启动后看 `data/digitalhuman.log` 的引擎装配行：
```
[INFO] 装配完成: asr=WhisperASR llm=OllamaLLM tts=EdgeTTS lipsync=MuseTalkLipSync ...
```
- `WhisperASR` = 本地 ASR（占 ~1GB）
- `CloudASR` = 云端 ASR（0 显存）
- `OllamaLLM` = 本地 LLM（占 ~1.9GB）
- `OpenAICompatLLM` = 云端 LLM（0 显存）
- `EdgeTTS` = 云端 TTS（0 显存）
- `CosyVoiceTTS` = 本地 TTS（占 ~1.5GB）
- `MuseTalkLipSync` = 本地唇形（占 ~1.5GB，必须）
