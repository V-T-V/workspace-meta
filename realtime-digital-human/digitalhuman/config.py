"""配置加载与校验。

用 dataclass 表达全配置，from_yaml 加载 + 校验 + 默认值兜底。
所有 engine 工厂都接收 `Config` 实例，按 `backend` 字段分发。
"""
from __future__ import annotations

import os
from dataclasses import asdict, dataclass, field, fields, is_dataclass
from typing import Any

import yaml


class ConfigError(ValueError):
    """配置错误。"""


# ---------- 各子配置 ----------

@dataclass
class ServerConfig:
    host: str = "127.0.0.1"
    port: int = 8000
    web_dir: str = "web"
    cors_origins: list[str] = field(default_factory=lambda: ["*"])
    # P1：并发 session 上限（单 4060 8GB 建议 2-3，防 OOM）
    max_sessions: int = 3
    # P1：WS 鉴权 token（为空则不鉴权；设值后客户端需 ?token=xxx 才能连）
    auth_token: str = ""
    # #3：对话历史持久化（空字符串=禁用，用内存；设路径=SQLite 持久化）
    history_db: str = "data/digitalhuman.db"


@dataclass
class ASRConfig:
    backend: str = "whisper"            # whisper(本地,GPU) / cloud(线上API,省显存) / groq/openai/deepinfra/siliconflow(预设云) / disabled
    model: str = "small"
    language: str | None = "zh"
    device: str = "auto"                # auto(探测cuda) / cuda / cpu（仅 whisper）
    compute_type: str | None = None     # None=自动（cuda→int8_float16, cpu→int8）仅 whisper
    silence_ms: int = 500
    # P0-1：VAD 自动句尾检测——收到 PCM 实时喂 VAD，连续静音超 silence_ms 自动触发 pipeline
    # 消除"等用户手动发 UTTERANCE_END"的延迟（首帧 -40~60%）
    vad_auto_trigger: bool = True
    vad_threshold: float = 0.01         # RMS 低于此值视为静音
    # 云端 ASR（backend=cloud/groq/... 时生效，把 ASR 上云省 ~1GB 显存给唇形）
    base_url: str = ""                  # 空则按 backend 自动套预设厂商 base_url
    api_key: str = ""                   # 线上必填（环境变量 DH_ASR_API_KEY 优先，key 不进 git/EXE）
    timeout: float = 30.0               # 云端转写超时（秒）


@dataclass
class LLMConfig:
    backend: str = "ollama"             # ollama(本地) / openai(通用云) / deepseek/glm/qwen/kimi(预设云) / gpu_mesh / disabled
    base_url: str = "http://127.0.0.1:11434"
    model: str = "qwen2.5:3b"
    system_prompt: str = "你是友好的数字人助手，回答简短自然。"
    temperature: float = 0.7
    max_tokens: int = 256
    num_ctx: int = 2048                 # 上下文窗口（减小加速 prompt 处理，默认 2048 够短对话）
    # 线上云 API 密钥（环境变量 DH_LLM_API_KEY 优先于此值，key 不进 git/EXE）
    api_key: str = ""


@dataclass
class TTSConfig:
    backend: str = "edge"               # edge / cosyvoice / disabled
    voice: str = "zh-CN-XiaoxiaoNeural"
    rate: str = "+0%"
    cosyvoice_script: str = "scripts/cosyvoice_tts.py"
    voice_sample: str = "assets/voice_sample.wav"


@dataclass
class SentenceSplitterConfig:
    punctuation: str = "。！？!?,.;；、\n"
    max_chars: int = 15


@dataclass
class LipSyncConfig:
    backend: str = "musetalk"           # musetalk / disabled
    portrait: str = "assets/portrait_sample.png"
    musetalk_script: str = "scripts/musetalk_render.py"
    fps: int = 25
    output_width: int = 512
    output_height: int = 512


@dataclass
class PusherConfig:
    backend: str = "ws_mjpeg"           # ws_mjpeg / disabled
    jpeg_quality: int = 75


@dataclass
class MemoryConfig:
    """长期记忆配置（★ 创新：摘要压缩，防长对话失忆 + 控制上下文窗口）。

    当 history 超过 summary_threshold 条时，把最旧的消息用 LLM 压成一条摘要，
    而非直接丢弃——保留长期记忆（用户姓名/偏好/关键事实），同时控制 token 数。
    """
    enabled: bool = True                # 总开关（False 则退回直接丢弃旧消息的旧行为）
    # 触发摘要的消息条数阈值（history 超过此数才压缩最旧的部分）
    summary_threshold: int = 10
    # 摘要后保留的近期对话轮数（user+assistant 对，这些不压缩，保持原文）
    keep_recent_pairs: int = 4          # 保留最近 4 轮原文 = 8 条消息
    # 摘要消息的 system prompt 指令
    summary_prompt: str = (
        "请把以下对话历史压缩成一段简洁的中文摘要（不超过150字），"
        "重点保留：用户的称呼/姓名、偏好、关键事实、未完成的待办。"
        "只输出摘要内容，不要加'摘要：'之类前缀。"
    )


@dataclass
class PersonaConfig:
    """单个数字人角色（人设 + 音色 + 形象）。"""
    id: str = "default"
    name: str = "默认助手"
    voice: str = "zh-CN-XiaoxiaoNeural"
    system_prompt: str = "你是友好的数字人助手，回答简短自然。"
    portrait: str = ""                  # 可选：该角色专用形象图
    greeting: str = ""                  # 可选：连接时的欢迎语


@dataclass
class PersonasConfig:
    """多角色配置。list 为空时用默认单角色。"""
    list: list = field(default_factory=list)
    default_id: str = "default"


@dataclass
class QueueConfig:
    text_queue: int = 16
    token_queue: int = 64
    sentence_queue: int = 8
    audio_queue: int = 32


@dataclass
class Config:
    server: ServerConfig = field(default_factory=ServerConfig)
    asr: ASRConfig = field(default_factory=ASRConfig)
    llm: LLMConfig = field(default_factory=LLMConfig)
    tts: TTSConfig = field(default_factory=TTSConfig)
    sentence_splitter: SentenceSplitterConfig = field(default_factory=SentenceSplitterConfig)
    lipsync: LipSyncConfig = field(default_factory=LipSyncConfig)
    pusher: PusherConfig = field(default_factory=PusherConfig)
    personas: PersonasConfig = field(default_factory=PersonasConfig)
    queue: QueueConfig = field(default_factory=QueueConfig)
    memory: MemoryConfig = field(default_factory=MemoryConfig)
    # GPU 显存预算（MB）。当前由 server.max_sessions 间接管控并发防 OOM；
    # 此字段保留供未来"运行时动态降级 asr.model"使用。
    gpu_memory_budget_mb: int = 8000
    # 加载时收集的未知配置键（拼写错误等），供 validate 报警（C12）
    unknown_keys: list[str] = field(default_factory=list)


# ---------- 加载 ----------

def _build_sub(cls: type, data: dict[str, Any] | None) -> tuple[Any, list[str]]:
    """从 dict 构造子 dataclass。

    返回 (instance, unknown_keys)：未知字段不传入构造，但收集起来供上层报警。
    缺失字段用 dataclass 默认值。
    """
    if data is None:
        return cls(), []
    if not isinstance(data, dict):
        raise ConfigError(f"配置段应为字典，得到 {type(data).__name__}")
    valid = {f.name for f in fields(cls)}
    unknown = []
    filtered = {}
    for k, v in data.items():
        if k in valid:
            filtered[k] = v
        else:
            unknown.append(k)
    return cls(**filtered), unknown


def from_dict(data: dict[str, Any]) -> Config:
    if not isinstance(data, dict):
        raise ConfigError(f"顶层配置应为字典，得到 {type(data).__name__}")
    cfg = Config()
    mapping = {
        "server": ServerConfig,
        "asr": ASRConfig,
        "llm": LLMConfig,
        "tts": TTSConfig,
        "sentence_splitter": SentenceSplitterConfig,
        "lipsync": LipSyncConfig,
        "pusher": PusherConfig,
        "personas": PersonasConfig,
        "queue": QueueConfig,
        "memory": MemoryConfig,
    }
    for key, cls in mapping.items():
        if key in data:
            inst, unknown = _build_sub(cls, data[key])
            setattr(cfg, key, inst)
            for u in unknown:
                cfg.unknown_keys.append(f"{key}.{u}")
    # #2：personas 特殊处理——list of dict → list of PersonaConfig
    if "personas" in data and isinstance(data["personas"], dict):
        p_list = data["personas"].get("list", [])
        cfg.personas.list = [_build_sub(PersonaConfig, p)[0] for p in p_list if isinstance(p, dict)]
        if "default_id" in data["personas"]:
            cfg.personas.default_id = data["personas"]["default_id"]
    # 顶层未知键
    for top_key in data:
        if top_key not in mapping and top_key != "gpu_memory_budget_mb":
            cfg.unknown_keys.append(top_key)
    if "gpu_memory_budget_mb" in data:
        # m6：容错强转（用户可能写 "8GB" 等非纯数字）
        try:
            cfg.gpu_memory_budget_mb = int(data["gpu_memory_budget_mb"])
        except (ValueError, TypeError):
            cfg.unknown_keys.append("gpu_memory_budget_mb(值非法)")
    return cfg


def from_yaml(path: str) -> Config:
    if not os.path.isfile(path):
        raise ConfigError(f"配置文件不存在: {path}")
    with open(path, encoding="utf-8") as f:
        data = yaml.safe_load(f)
    if data is None:
        data = {}
    return from_dict(data)


def validate(cfg: Config) -> list[str]:
    """返回告警列表（含未知键、值越界等）；空列表表示无问题。

    不抛异常（宽松策略），但 server 启动时会 log.warning 每一条。
    """
    warns: list[str] = []
    # 未知键（拼写错误等）——C12：不再静默
    for k in cfg.unknown_keys:
        warns.append(f"未知配置项: {k}（可能是拼写错误，将使用默认值）")
    # M4：backend 合法性校验（白名单）
    from .engines.llm_ollama import CLOUD_PRESETS
    from .engines.asr_cloud import ASR_CLOUD_PRESETS
    VALID_ASR = ({"whisper", "cloud", "mock", "disabled", "none"}
                 | set(ASR_CLOUD_PRESETS.keys()))
    # 线上云 backend：openai（通用）+ 预设厂商 + gpu_mesh（旧兼容）+ 本地 ollama + mock + disabled
    VALID_LLM = ({"ollama", "openai", "gpu_mesh", "mock", "disabled", "none"}
                 | set(CLOUD_PRESETS.keys()))
    VALID_TTS = {"edge", "cosyvoice", "mock", "disabled", "none"}
    # lipsync 用户要求"除了数字人都可 mock"——唇形是数字人核心，必须本地/真引擎，
    # 但保留 mock 供纯演示（不渲染真帧，只占位）。disabled=完全无唇形。
    VALID_LIPSYNC = {"musetalk", "mock", "disabled", "none"}
    VALID_PUSHER = {"ws_mjpeg", "disabled", "none"}
    if cfg.asr.backend not in VALID_ASR:
        warns.append(f"asr.backend 非法: {cfg.asr.backend}，合法值 {VALID_ASR}")
    if cfg.llm.backend not in VALID_LLM:
        warns.append(f"llm.backend 非法: {cfg.llm.backend}，合法值 {VALID_LLM}")
    if cfg.tts.backend not in VALID_TTS:
        warns.append(f"tts.backend 非法: {cfg.tts.backend}，合法值 {VALID_TTS}")
    if cfg.lipsync.backend not in VALID_LIPSYNC:
        warns.append(f"lipsync.backend 非法: {cfg.lipsync.backend}，合法值 {VALID_LIPSYNC}")
    if cfg.pusher.backend not in VALID_PUSHER:
        warns.append(f"pusher.backend 非法: {cfg.pusher.backend}，合法值 {VALID_PUSHER}")
    # 值越界
    if cfg.server.port <= 0 or cfg.server.port > 65535:
        warns.append(f"server.port 非法: {cfg.server.port}")
    if cfg.asr.silence_ms < 100:
        warns.append(f"asr.silence_ms 过小（{cfg.asr.silence_ms}ms），可能频繁误切句")
    if cfg.sentence_splitter.max_chars < 5:
        warns.append(f"sentence_splitter.max_chars 过小（{cfg.sentence_splitter.max_chars}），TTS 调用过频")
    if cfg.lipsync.fps <= 0 or cfg.lipsync.fps > 60:
        warns.append(f"lipsync.fps 异常: {cfg.lipsync.fps}")
    for q in (cfg.queue.text_queue, cfg.queue.token_queue,
              cfg.queue.sentence_queue, cfg.queue.audio_queue):
        if q <= 0:
            warns.append(f"queue 缓冲非法（{q}），会导致死锁")
    # M3：云端 backend 的 api_key/base_url 前置校验（配错运行时才暴露，体验差）
    import os as _os
    # 通用云端（cloud/openai）必须显式配 base_url；预设厂商（groq/deepseek 等）自动套
    if cfg.asr.backend in ("cloud", "openai") and not cfg.asr.base_url:
        warns.append(f"asr.backend={cfg.asr.backend} 需配 base_url（预设厂商 groq/openai 等可免）")
    if cfg.llm.backend == "openai" and not cfg.llm.base_url:
        warns.append("llm.backend=openai 需配 base_url（预设厂商 deepseek/glm 等可免）")
    # 云端 backend 缺 api_key 且无环境变量 → 401 必然失败（仅预设厂商/通用云端，不含 ollama/whisper/mock）
    _CLOUD_ASR = set(ASR_CLOUD_PRESETS.keys()) | {"cloud"}
    _CLOUD_LLM = set(CLOUD_PRESETS.keys())
    if (cfg.asr.backend in _CLOUD_ASR and not cfg.asr.api_key
            and not _os.environ.get("DH_ASR_API_KEY")):
        warns.append(f"asr.backend={cfg.asr.backend} 缺 api_key（设 DH_ASR_API_KEY 环境变量或填 asr.api_key）")
    if (cfg.llm.backend in _CLOUD_LLM and not cfg.llm.api_key
            and not _os.environ.get("DH_LLM_API_KEY")):
        warns.append(f"llm.backend={cfg.llm.backend} 缺 api_key（设 DH_LLM_API_KEY 环境变量或填 llm.api_key）")
    return warns


def is_dataclass_instance(obj: Any) -> bool:
    return is_dataclass(obj) and not isinstance(obj, type)


# ---------- 回写（管理台用）----------

# 序列化时排除的内部字段（非用户配置）
_INTERNAL_KEYS = {"unknown_keys", "gpu_memory_budget_mb"}


def to_yaml_dict(cfg: Config) -> dict[str, Any]:
    """Config dataclass → 可写回 YAML 的 dict（排除内部字段）。"""
    raw = asdict(cfg)
    out: dict[str, Any] = {}
    for key, val in raw.items():
        if key in _INTERNAL_KEYS:
            continue
        out[key] = val
    return out


def save_yaml(cfg: Config, path: str) -> None:
    """把 Config 序列化回写 config.yaml（全量重写，注释会丢失）。

    ★ 管理台保存配置用。用 yaml.dump 全量重写——简单可靠。
    若需保留注释，后续可换 ruamel.yaml（当前项目无此依赖，不引入）。
    """
    data = to_yaml_dict(cfg)
    with open(path, "w", encoding="utf-8") as f:
        yaml.dump(data, f, allow_unicode=True, default_flow_style=False, sort_keys=False)
