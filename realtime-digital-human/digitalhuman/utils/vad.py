"""简单 VAD：基于能量的句尾检测。

faster-whisper 本身能分段，但实时场景需要"用户说完了"的快速判断。
策略：累积 PCM，检测到连续静音超过 silence_ms 即认为一句结束，
把缓冲的音频交给 ASR。

复杂 VAD（如 Silero）留待后续；MVP 用能量法足够。
"""
from __future__ import annotations

from dataclasses import dataclass, field


from .audio import pcm16_to_float32, rms_energy


@dataclass
class VadState:
    """VAD 状态机。每次 feed_audio 喂一段 PCM，返回完整 utterance（若有）。"""
    silence_threshold: float = 0.01    # RMS 低于此值视为静音
    silence_ms: int = 500              # 连续静音多久算句尾
    min_utterance_ms: int = 300        # 短于此长度的 utterance 丢弃（噪声）
    sample_rate: int = 16000

    _buffer: list[bytes] = field(default_factory=list)
    _silence_chunks: int = 0           # 连续静音 chunk 计数
    _in_utterance: bool = False
    # chunk 时长（ms）：假设每 chunk = 256 samples @16k = 16ms（前端 ScriptProcessor 4096 @16k=256ms）
    _chunk_ms: int = 256

    def feed(self, pcm: bytes) -> bytes | None:
        """喂一段 PCM chunk，返回完整 utterance（句尾触发时）或 None。"""
        self._buffer.append(pcm)
        samples = pcm16_to_float32(pcm)
        energy = rms_energy(samples)
        chunk_ms = int(len(samples) / self.sample_rate * 1000)
        if chunk_ms > 0:
            self._chunk_ms = chunk_ms

        if energy < self.silence_threshold:
            self._silence_chunks += 1
        else:
            self._silence_chunks = 0
            self._in_utterance = True

        # 检测句尾：正在说话 + 连续静音达阈值
        silence_duration_ms = self._silence_chunks * self._chunk_ms
        if (self._in_utterance and
                silence_duration_ms >= self.silence_ms):
            utterance = b"".join(self._buffer)
            self._buffer.clear()
            self._silence_chunks = 0
            self._in_utterance = False
            # 过滤过短（噪声）
            if len(utterance) / 2 * 1000 / self.sample_rate < self.min_utterance_ms:
                return None
            return utterance
        return None

    def flush(self) -> bytes | None:
        """强制结束（如客户端断开），返回剩余缓冲。"""
        if not self._buffer:
            return None
        utterance = b"".join(self._buffer)
        self._buffer.clear()
        return utterance
