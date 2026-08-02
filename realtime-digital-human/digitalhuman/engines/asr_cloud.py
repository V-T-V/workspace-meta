"""ASR 引擎：云端语音识别（OpenAI 兼容 /v1/audio/transcriptions）。

★ 设计目标：把语音识别从本地 GPU（faster-whisper 占 ~1GB 显存）上云，
  4060 显存全留给数字人形象（MuseTalk 唇形，必须本地实时渲染帧）。

★ 协议：OpenAI Whisper API 兼容（multipart 上传 WAV，返回 JSON {"text": "..."}）。
  兼容厂商：OpenAI / Groq（whisper-large-v3）/ DeepInfra / SiliconFlow / 自建 whisper-server 等。

★ 接口契约（与 WhisperASR 对称）：
  - 输入：AsyncIterator[bytes]（PCM16 16k mono LE，上游 VAD 已切成完整 utterance）
  - 输出：AsyncIterator[str]（完整句文本）
  - 先消费完整段音频 → 打包 WAV → 一次 HTTP POST → yield text

★ 与 WhisperASR 的区别：
  - WhisperASR 同步阻塞（faster-whisper）需线程池；云端用 aiohttp 原生异步
  - WhisperASR 按 segment yield 多句；云端一次请求返回整段文本，yield 一条
"""
from __future__ import annotations

import asyncio
import io
import logging
import os
import wave
from collections.abc import AsyncIterator

import aiohttp

from .base import ASREngine
from ..utils.audio import pcm16_to_wav

log = logging.getLogger(__name__)

# 预设云厂商 → 默认 base_url（写 backend: groq 即自动套用）。
# 均为 OpenAI 兼容（/v1/audio/transcriptions）。
ASR_CLOUD_PRESETS: dict[str, str] = {
    "groq":        "https://api.groq.com/openai/v1",       # Groq（whisper-large-v3，极快）
    "openai":      "https://api.openai.com/v1",
    "deepinfra":   "https://api.deepinfra.com/v1/openai",
    "siliconflow": "https://api.siliconflow.cn/v1",
}


class CloudASR(ASREngine):
    """云端 ASR（OpenAI 兼容 /v1/audio/transcriptions）。

    复用引擎级 ClientSession（连接池），与 LLM 的 _BaseHTTPLLM 模式一致。
    """

    def __init__(self, base_url: str, model: str = "whisper-large-v3",
                 language: str | None = "zh", api_key: str | None = None,
                 timeout: float = 30.0, backend_tag: str = "cloud"):
        self.base_url = base_url.rstrip("/")
        self.model = model
        self.language = language
        self.api_key = api_key
        self.timeout = timeout
        self.backend_tag = backend_tag
        self._session: aiohttp.ClientSession | None = None
        # ★ H1：保护懒初始化的并发安全（多 session 并发转写会竞态）
        self._session_lock = asyncio.Lock()

    async def _get_session(self) -> aiohttp.ClientSession:
        """获取或创建常驻 ClientSession。

        ★ H1：double-check + Lock，避免并发首请求各自建 session 导致旧 session 泄漏。
        ★ H3：加 sock_read 超时，防半开连接导致读永久挂起。
        """
        if self._session is None or self._session.closed:
            async with self._session_lock:
                if self._session is None or self._session.closed:
                    self._session = aiohttp.ClientSession(
                        timeout=aiohttp.ClientTimeout(total=self.timeout, sock_read=self.timeout),
                        trust_env=False,
                        connector=aiohttp.TCPConnector(limit=5, force_close=False),
                    )
        return self._session

    async def close(self) -> None:
        if self._session and not self._session.closed:
            await self._session.close()
            self._session = None

    async def transcribe_stream(
        self, audio_stream: AsyncIterator[bytes]
    ) -> AsyncIterator[str]:
        """累积 PCM chunk → 打包 WAV → POST /v1/audio/transcriptions → yield text。"""
        chunks: list[bytes] = []
        async for chunk in audio_stream:
            chunks.append(chunk)

        if not chunks:
            return

        pcm = b"".join(chunks)
        text = await self._transcribe(pcm)
        if text and text.strip():
            yield text.strip()

    async def _transcribe(self, pcm: bytes) -> str:
        """POST WAV 到云端，返回识别文本。"""
        wav_bytes = pcm16_to_wav(pcm)
        path = "/audio/transcriptions" if self.base_url.endswith("/v1") else "/v1/audio/transcriptions"
        url = f"{self.base_url}{path}"
        headers = {"Authorization": f"Bearer {self.api_key}"} if self.api_key else {}
        # multipart：file 字段必须是 (filename, fileobj, content_type) 三元组
        form = aiohttp.FormData()
        form.add_field("file", wav_bytes, filename="audio.wav", content_type="audio/wav")
        form.add_field("model", self.model)
        if self.language:
            form.add_field("language", self.language)
        form.add_field("response_format", "json")

        log.debug("[%s] ASR POST %s model=%s bytes=%d", self.backend_tag, url, self.model, len(wav_bytes))
        sess = await self._get_session()
        try:
            async with sess.post(url, data=form, headers=headers) as resp:
                if resp.status != 200:
                    body = await resp.text()
                    raise RuntimeError(f"[{self.backend_tag}] ASR HTTP {resp.status}: {body[:200]}")
                data = await resp.json()
                return data.get("text", "")
        except aiohttp.ClientError as e:
            raise RuntimeError(f"[{self.backend_tag}] ASR 网络错误: {e}") from e


def build_asr_cloud(backend: str, **kwargs) -> ASREngine:
    """工厂：按 backend 构造云 ASR。

    backend 取值：
      - cloud / openai   通用（base_url 必须在 kwargs 给）
      - groq/deepinfra/siliconflow  预设厂商（自动套 ASR_CLOUD_PRESETS 的 base_url）
    """
    # ★ api_key：环境变量 DH_ASR_API_KEY 优先于 yaml（key 不进 git/EXE）
    env_key = os.environ.get("DH_ASR_API_KEY")
    if env_key:
        kwargs["api_key"] = env_key

    if backend in ASR_CLOUD_PRESETS:
        if not kwargs.get("base_url"):
            kwargs["base_url"] = ASR_CLOUD_PRESETS[backend]
            log.info("[%s] ASR 使用预设 base_url: %s", backend, ASR_CLOUD_PRESETS[backend])
        return CloudASR(backend_tag=backend, **kwargs)
    if backend in ("cloud", "openai"):
        return CloudASR(backend_tag="cloud", **kwargs)
    raise ValueError(f"未知 cloud asr backend: {backend}")
