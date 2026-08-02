"""TTS 引擎：edge-tts 实现（默认）。

edge-tts 调微软在线 TTS，零本地显存，质量高。

★ 音频格式归一化（P0 修复）：
  edge-tts 原生输出 MP3，但下游 MuseTalk 按 PCM16 解读字节（len//2 算采样点）。
  MP3 是压缩格式，直接喂会导致乱码音频 + 错误帧数（真实 bug，前几轮验证因占位渲染未暴露）。
  本引擎在 yield 前用 miniaudio 把 MP3 解码成 PCM16（16k mono），下游统一处理 PCM。
  前端播放也用 PCM16（copyToChannel，比 decodeAudioData 更高效）。
"""
from __future__ import annotations

import asyncio
import logging
from collections.abc import AsyncIterator

from .base import TTSEngine

log = logging.getLogger(__name__)


async def _decode_mp3_to_pcm16_async(mp3_data: bytes, sample_rate: int = 16000) -> bytes:
    """MP3 bytes → PCM16 16k mono bytes（异步，解码在线程池避免阻塞事件循环）。"""
    loop = asyncio.get_running_loop()
    return await loop.run_in_executor(None, _decode_mp3_to_pcm16_sync, mp3_data, sample_rate)


def _decode_mp3_to_pcm16_sync(mp3_data: bytes, sample_rate: int = 16000) -> bytes:
    """同步解码（在线程池里跑）。"""
    try:
        import miniaudio
        decoded = miniaudio.decode(mp3_data, nchannels=1, sample_rate=sample_rate,
                                   output_format=miniaudio.SampleFormat.SIGNED16)
        return bytes(decoded.samples)
    except Exception as e:
        log.warning("MP3→PCM16 解码失败: %s", e)
        return b""


# ★ P0-1 流式解码阈值：累积到此字节数就解码一次（约 0.5s 音频的 MP3）
#   MP3 ~128kbps → 16KB/s，8KB ≈ 0.5s。平衡首音频延迟与解码次数。
_STREAM_DECODE_THRESHOLD = 8 * 1024


async def _decode_mp3_chunk_async(mp3_data: bytes, sample_rate: int = 16000) -> bytes:
    """分块解码：对已收 MP3 部分做一次解码（流式增量，不要求完整 MP3）。"""
    return await _decode_mp3_to_pcm16_async(mp3_data, sample_rate)


# 向后兼容（测试用）
def _decode_mp3_to_pcm16(mp3_data: bytes, sample_rate: int = 16000) -> bytes:
    return _decode_mp3_to_pcm16_sync(mp3_data, sample_rate)


class EdgeTTS(TTSEngine):
    """edge-tts 流式合成。

    yield MP3 二进制片段（每片几百字节到几 KB）。

    ★ 自动重试（深度优化）：edge-tts 在线服务偶发 NoAudioReceived（限流/抖动），
    自动重试最多 max_retries 次，提高稳定性。
    """

    def __init__(self, voice: str = "zh-CN-XiaoxiaoNeural", rate: str = "+0%",
                 max_retries: int = 2):
        self.voice = voice
        self.rate = rate
        self.max_retries = max_retries

    async def synthesize_stream(self, text: str) -> AsyncIterator[bytes]:
        # 惰性 import：缺 edge_tts 时给清晰错误
        try:
            import edge_tts
        except ImportError as e:
            raise RuntimeError(
                "未安装 edge-tts。请 `pip install edge-tts` 或在 config 里换 tts.backend"
            ) from e

        if not text.strip():
            return
        log.debug("EdgeTTS synthesize: voice=%s text=%r", self.voice, text[:40])

        # 20ms PCM chunk 大小（16k mono 16-bit）
        chunk_bytes = 16000 * 2 * 20 // 1000  # 640 bytes

        # ★ 自动重试：偶发 NoAudioReceived 时重试
        last_err = None
        for attempt in range(self.max_retries + 1):
            try:
                communicate = edge_tts.Communicate(text, self.voice, rate=self.rate)
                # ★ P0-1 流式增量解码：边收 MP3 chunk 边解码出 PCM，不再等整段 MP3 下完。
                #   原实现整段缓存（首音频等 0.8-1.2s）；现每攒 _STREAM_DECODE_THRESHOLD 字节
                #   就解码一次，首音频可降至 0.2-0.4s。
                mp3_buffer = bytearray()
                pending_pcm = bytearray()
                async for chunk in communicate.stream():
                    if chunk["type"] == "audio":
                        mp3_buffer.extend(chunk["data"])
                        # 攒够阈值就解码一次（增量出 PCM）
                        if len(mp3_buffer) >= _STREAM_DECODE_THRESHOLD:
                            pcm = await _decode_mp3_chunk_async(bytes(mp3_buffer))
                            if pcm:
                                pending_pcm.extend(pcm)
                                # 立即按 640B chunk yield 已就绪的 PCM
                                while len(pending_pcm) >= chunk_bytes:
                                    yield bytes(pending_pcm[:chunk_bytes])
                                    del pending_pcm[:chunk_bytes]
                            mp3_buffer.clear()
                # 收尾：解码最后不足阈值的部分
                if mp3_buffer:
                    pcm = await _decode_mp3_chunk_async(bytes(mp3_buffer))
                    if pcm:
                        pending_pcm.extend(pcm)
                # yield 剩余 PCM
                while len(pending_pcm) >= chunk_bytes:
                    yield bytes(pending_pcm[:chunk_bytes])
                    del pending_pcm[:chunk_bytes]
                if pending_pcm:
                    yield bytes(pending_pcm)
                return  # 成功
            except Exception as e:
                last_err = e
                if attempt < self.max_retries:
                    log.warning("edge-tts 第 %d 次失败（%s），重试...", attempt + 1, e)
                    await asyncio.sleep(0.3 * (attempt + 1))  # 退避
                continue
        # 所有重试失败，抛出（由 pipeline 的 per-sentence 容错捕获）
        raise last_err if last_err else RuntimeError("edge-tts 未知失败")


def build_tts(backend: str, **kwargs) -> TTSEngine:
    if backend == "edge":
        # edge 只用 voice/rate，剔除 cosyvoice 专用参数（script/voice_sample）
        edge_kwargs = {k: v for k, v in kwargs.items() if k in ("voice", "rate")}
        return EdgeTTS(**edge_kwargs)
    if backend in ("mock", "disabled", "none"):
        from .mock import MockTTS
        return MockTTS()
    if backend == "cosyvoice":
        # CosyVoice 本地克隆音色（占 ~1.5GB 显存）；只用 script/voice_sample，剔除 edge 专用参数
        try:
            from .tts_cosyvoice import build_cosyvoice
            cosy_kwargs = {k: v for k, v in kwargs.items()
                           if k in ("script", "voice_sample", "sample_rate")}
            return build_cosyvoice(**cosy_kwargs)
        except ImportError as e:
            raise RuntimeError(
                "CosyVoice 实现未就绪。请先完成 engines/tts_cosyvoice.py"
            ) from e
    raise ValueError(f"未知 tts backend: {backend}")
