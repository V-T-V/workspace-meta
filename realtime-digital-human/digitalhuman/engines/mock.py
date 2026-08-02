"""Mock 引擎实现（测试与离线开发用）。

每个 Mock 都按固定节奏产出 token/音频/帧，不依赖任何外部模型。
让 pipeline 可以在没有 GPU/Ollama/MuseTalk 的情况下验证流式接力。
"""
from __future__ import annotations

import asyncio
from collections.abc import AsyncIterator

from .base import ASREngine, LLMEngine, LipSyncEngine, Message, TTSEngine


class MockASR(ASREngine):
    """把预置的句子按节奏 yield 出来（模拟 VAD 检测到句尾）。"""

    def __init__(self, sentences: list[str], delay: float = 0.05):
        self._sentences = list(sentences)
        self._delay = delay
        # 记录喂入的音频，供测试断言
        self.fed_audio: list[bytes] = []

    async def transcribe_stream(
        self, audio_stream: AsyncIterator[bytes]
    ) -> AsyncIterator[str]:
        # 先消费完音频流（模拟 ASR 必须等用户说完）
        async for chunk in audio_stream:
            self.fed_audio.append(chunk)
        for s in self._sentences:
            await asyncio.sleep(self._delay)
            yield s


class InjectASR(ASREngine):
    """直接注入固定文本的 ASR（绕过真实识别，用于 bench/验证脚本）。

    消费完音频流后 yield 预置文本。scripts 共享，避免重复定义。
    """

    def __init__(self, text: str):
        self._text = text

    async def transcribe_stream(
        self, audio_stream: AsyncIterator[bytes]
    ) -> AsyncIterator[str]:
        async for _ in audio_stream:
            pass
        yield self._text


class MockLLM(LLMEngine):
    """把预置回复按字符逐个 yield（模拟流式 token）。"""

    def __init__(self, reply: str, delay_per_token: float = 0.01):
        self._reply = reply
        self._delay = delay_per_token
        self.received_prompts: list[str] = []

    async def chat_stream(
        self, prompt: str, history: list[Message] | None = None
    ) -> AsyncIterator[str]:
        self.received_prompts.append(prompt)
        for ch in self._reply:
            await asyncio.sleep(self._delay)
            yield ch


class MockTTS(TTSEngine):
    """模拟音频合成：每字产生固定时长 PCM（0.3s/字，16k mono 16-bit）。

    ★ 产生真实时长的 PCM，让下游 MockLipSync 能按 fps 正确算帧数，
    避免长回复时帧数爆炸。
    """

    # 每字音频时长（秒）——Mock 用较小值（0.05）加速测试，
    # 真实场景是 0.3s/字但 Mock 不应真等那么久
    SECS_PER_CHAR = 0.05
    SAMPLE_RATE = 16000
    BYTES_PER_SAMPLE = 2
    CHUNK_MS = 20  # 每 20ms 一个 chunk

    def __init__(self, delay: float = 0.001):
        self._delay = delay
        self.received_texts: list[str] = []

    async def synthesize_stream(self, text: str) -> AsyncIterator[bytes]:
        self.received_texts.append(text)
        if not text.strip():
            return
        # 总时长 = 字数 × 每字时长
        duration_s = max(0.1, len(text) * self.SECS_PER_CHAR)
        total_samples = int(duration_s * self.SAMPLE_RATE)
        total_bytes = total_samples * self.BYTES_PER_SAMPLE
        chunk_bytes = int(self.SAMPLE_RATE * self.BYTES_PER_SAMPLE * self.CHUNK_MS / 1000)
        # 产生静音 PCM（真实场景是有声波，这里用 0 占位）
        data = b"\x00" * total_bytes
        for i in range(0, len(data), chunk_bytes):
            await asyncio.sleep(self._delay)
            yield data[i:i + chunk_bytes]


class MockLipSync(LipSyncEngine):
    """按 fps 限速产假 JPEG 帧。

    ★ 帧数 = 音频时长 × fps（与真实 MuseTalk 契约一致），而非按音频字节数。
    避免长回复时帧数爆炸导致 pipeline 吞吐跟不上。
    """

    # PCM 16k mono 16-bit：每秒 16000*2 = 32000 字节
    SAMPLES_RATE = 16000
    BYTES_PER_SAMPLE = 2

    def __init__(self, fps: int = 25, delay: float = 0.001):
        self._fps = fps
        self._delay = delay
        self.received_portrait: bytes | None = None

    async def render_stream(
        self, portrait: bytes, audio_stream: AsyncIterator[bytes]
    ) -> AsyncIterator[bytes]:
        self.received_portrait = portrait
        async for audio in audio_stream:
            # 按音频时长 × fps 算帧数（PCM16 16k mono）
            duration_s = len(audio) / (self.SAMPLES_RATE * self.BYTES_PER_SAMPLE)
            n_frames = max(1, int(duration_s * self._fps))
            for _ in range(n_frames):
                await asyncio.sleep(self._delay)
                yield b"\xff\xd8\xff\xe0MOCKJPEG"  # 假 JPEG 头
