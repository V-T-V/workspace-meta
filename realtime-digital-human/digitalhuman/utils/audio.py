"""音频处理：PCM 格式转换、重采样、能量计算。

浏览器通过 ScriptProcessor 采集的是 16-bit signed PCM, mono, 16kHz（前端约定）。
ASR (faster-whisper) 接受 16k mono 的 float32 numpy 数组或 raw bytes。
TTS (edge-tts) 输出 MP3，前端直接播。
MuseTalk 需要 wav。

本模块提供纯函数转换，不依赖 torch（仅 numpy）。
"""
from __future__ import annotations

import io
import wave
from collections.abc import Iterable

import numpy as np

# 约定的音频格式（前端 ScriptProcessor 配置）
SAMPLE_RATE = 16000
SAMPLE_WIDTH = 2      # 16-bit
CHANNELS = 1          # mono


# ---------- PCM <-> numpy ----------

def pcm16_to_float32(pcm: bytes) -> np.ndarray:
    """16-bit signed LE PCM bytes → float32 numpy array (-1.0 .. 1.0)。"""
    arr = np.frombuffer(pcm, dtype="<i2").astype(np.float32)
    return arr / 32768.0


def float32_to_pcm16(samples: np.ndarray) -> bytes:
    """float32 (-1..1) → 16-bit signed LE PCM bytes。"""
    clipped = np.clip(samples, -1.0, 1.0)
    return (clipped * 32767).astype("<i2").tobytes()


def merge_pcm_chunks(chunks: Iterable[bytes]) -> bytes:
    """合并多个 PCM chunk 为一个连续 bytes。"""
    return b"".join(chunks)


# ---------- WAV 封装 ----------

def pcm16_to_wav(pcm: bytes, sample_rate: int = SAMPLE_RATE,
                 sample_width: int = SAMPLE_WIDTH, channels: int = CHANNELS) -> bytes:
    """raw PCM16 → WAV bytes（含头）。MuseTalk 等需要 WAV 输入。"""
    buf = io.BytesIO()
    with wave.open(buf, "wb") as w:
        w.setnchannels(channels)
        w.setsampwidth(sample_width)
        w.setframerate(sample_rate)
        w.writeframes(pcm)
    return buf.getvalue()


def wav_to_pcm16(wav_bytes: bytes) -> bytes:
    """WAV bytes → raw PCM16（去头）。"""
    with wave.open(io.BytesIO(wav_bytes), "rb") as w:
        return w.readframes(w.getnframes())


# ---------- 重采样（线性插值，无外部依赖） ----------

def resample(samples: np.ndarray, src_rate: int, dst_rate: int) -> np.ndarray:
    """线性插值重采样（mono float32）。够用，非最高质量。"""
    if src_rate == dst_rate:
        return samples
    ratio = dst_rate / src_rate
    n_out = int(len(samples) * ratio)
    if n_out < 1:
        return np.zeros(1, dtype=np.float32)
    idx = np.arange(n_out) / ratio
    idx_floor = np.floor(idx).astype(int)
    idx_ceil = np.minimum(idx_floor + 1, len(samples) - 1)
    frac = idx - idx_floor
    return samples[idx_floor] * (1 - frac) + samples[idx_ceil] * frac


# ---------- 能量 / VAD ----------

def rms_energy(samples: np.ndarray) -> float:
    """计算 RMS 能量（float32 输入）。"""
    if len(samples) == 0:
        return 0.0
    return float(np.sqrt(np.mean(samples ** 2)))


def is_silence(samples: np.ndarray, threshold: float = 0.01) -> bool:
    """简单能量 VAD：低于阈值视为静音。"""
    return rms_energy(samples) < threshold
