"""引擎抽象基类。

每个环节都是一个 async generator 接口：上游产一点、下游立即消费，绝不缓冲完整结果。
这层抽象让 Mock 实现（测试）与真实实现（Ollama/Whisper/MuseTalk）可互换。
"""
from __future__ import annotations

from abc import ABC, abstractmethod
from dataclasses import dataclass
from collections.abc import AsyncIterator


@dataclass
class Message:
    """对话历史中的单条消息（与 Ollama/OpenAI 格式通用）。"""
    role: str          # "system" / "user" / "assistant"
    content: str


# ---------- Stage 1: ASR ----------

class ASREngine(ABC):
    """语音转文字。流式输入 PCM，流式输出完整句文本。

    约定：仅在检测到句尾（VAD 静音或语义完整）时 yield 一条字符串，
    不要逐字 yield——下游 LLM 按完整用户话语触发。
    """

    @abstractmethod
    async def transcribe_stream(
        self, audio_stream: AsyncIterator[bytes]
    ) -> AsyncIterator[str]:
        """喂 PCM chunk 流，产出完整句文本流。"""
        ...
        # 提示类型检查器这是 async generator
        yield ""  # pragma: no cover


# ---------- Stage 2: LLM ----------

class LLMEngine(ABC):
    """大语言模型流式推理。逐 token 产出，不等待完整回复。"""

    @abstractmethod
    async def chat_stream(
        self, prompt: str, history: list[Message] | None = None
    ) -> AsyncIterator[str]:
        """逐 token 产出生成内容。"""
        ...
        yield ""  # pragma: no cover


# ---------- Stage 3: TTS ----------

class TTSEngine(ABC):
    """文字转语音。输入短句，输出 PCM/WAV 片段流。"""

    @abstractmethod
    async def synthesize_stream(self, text: str) -> AsyncIterator[bytes]:
        """合成语音，产出 PCM/WAV 片段。"""
        ...
        yield b""  # pragma: no cover


# ---------- Stage 4: 唇形 ----------

class LipSyncEngine(ABC):
    """音频驱动唇形渲染。输入音频片段流，输出 JPEG 编码的视频帧流。"""

    @abstractmethod
    async def render_stream(
        self,
        portrait: bytes,
        audio_stream: AsyncIterator[bytes],
    ) -> AsyncIterator[bytes]:
        """根据形象图 + 音频流，产出 JPEG 帧（每帧一个 bytes）。"""
        ...
        yield b""  # pragma: no cover


# ---------- Stage 5: 推流 ----------

class Pusher(ABC):
    """把视频帧/音频推送到客户端（WS / WebRTC）。"""

    @abstractmethod
    async def push_frame(self, session_id: str, jpeg: bytes) -> None:
        """推送一帧 JPEG。"""

    @abstractmethod
    async def push_audio(self, session_id: str, pcm: bytes) -> None:
        """推送一段音频。"""

    async def push_text(self, session_id: str, text: str) -> None:
        """推送一段文字（用于前端显示字幕）。默认不实现。"""
        return None

    async def push_latency(self, session_id: str, timing_ms: dict) -> None:
        """推送延迟统计（JSON，单位 ms）。默认不实现。

        timing_ms 形如 {"first_token_ms": 450, "first_frame_ms": 1200}。
        """
        return None

    async def close(self, session_id: str) -> None:
        """会话结束清理。"""
        return None
