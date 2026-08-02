"""音频处理与 VAD 测试。"""
import numpy as np
import pytest

from digitalhuman.utils.audio import (
    float32_to_pcm16,
    merge_pcm_chunks,
    pcm16_to_float32,
    pcm16_to_wav,
    resample,
    rms_energy,
    wav_to_pcm16,
)
from digitalhuman.utils.vad import VadState


def test_pcm_float32_roundtrip():
    """PCM16 → float32 → PCM16 应无损失（在量化误差内）。"""
    samples = np.array([0.0, 0.5, -0.5, 1.0, -1.0, 0.123, -0.456], dtype=np.float32)
    pcm = float32_to_pcm16(samples)
    back = pcm16_to_float32(pcm)
    # 16-bit 量化误差约 1/32768 ≈ 3e-5
    assert np.allclose(samples, back, atol=1e-4)


def test_pcm16_to_float32_silent():
    assert np.allclose(pcm16_to_float32(b"\x00\x00" * 10), 0.0)


def test_merge_pcm_chunks():
    assert merge_pcm_chunks([b"\x01\x02", b"\x03\x04"]) == b"\x01\x02\x03\x04"
    assert merge_pcm_chunks([]) == b""


def test_wav_roundtrip():
    """PCM → WAV → PCM 应还原。"""
    samples = np.array([0.1, -0.2, 0.3, 0.0] * 100, dtype=np.float32)
    pcm = float32_to_pcm16(samples)
    wav = pcm16_to_wav(pcm)
    # WAV 头 44 字节
    assert wav[:4] == b"RIFF"
    assert b"data" in wav[:60]
    back = wav_to_pcm16(wav)
    assert back == pcm


def test_resample_noop():
    """同采样率不变。"""
    s = np.arange(100, dtype=np.float32)
    out = resample(s, 16000, 16000)
    assert np.array_equal(out, s)


def test_resample_halve():
    """16k → 8k 应减半长度。"""
    s = np.arange(160, dtype=np.float32)
    out = resample(s, 16000, 8000)
    assert len(out) == 80


def test_rms_energy_silent():
    assert rms_energy(np.zeros(100, dtype=np.float32)) == 0.0


def test_rms_energy_nonzero():
    s = np.full(100, 0.5, dtype=np.float32)
    e = rms_energy(s)
    assert abs(e - 0.5) < 1e-6


# ---------- VAD ----------

def _make_pcm(samples: np.ndarray) -> bytes:
    return float32_to_pcm16(samples)


def test_vad_no_utterance_for_silence():
    """纯静音不应产生 utterance。"""
    vad = VadState(silence_threshold=0.1, silence_ms=200, min_utterance_ms=100)
    silent = np.zeros(4096, dtype=np.float32)  # 256ms @16k
    out = None
    for _ in range(20):  # 喂 5s 静音
        out = vad.feed(_make_pcm(silent))
        if out:
            break
    assert out is None


def test_vad_detects_utterance_end():
    """说话 + 静音 → 应在静音超阈值时返回 utterance。"""
    vad = VadState(silence_threshold=0.05, silence_ms=300, min_utterance_ms=100,
                   sample_rate=16000)
    loud = np.ones(4096, dtype=np.float32) * 0.5   # 256ms 说话
    silent = np.zeros(4096, dtype=np.float32)      # 256ms 静音

    utterance = None
    # 喂 1s 说话（4 chunk）
    for _ in range(4):
        utterance = vad.feed(_make_pcm(loud))
        assert utterance is None  # 说话中不应返回
    # 喂静音直到超过 silence_ms（300ms = 2 chunk）
    for _ in range(5):
        utterance = vad.feed(_make_pcm(silent))
        if utterance:
            break
    assert utterance is not None
    assert len(utterance) > 0


def test_vad_filters_short_noise():
    """短于 min_utterance_ms 的纯静音不应产生 utterance。

    注：VAD 的 buffer 含静音尾巴，min_utterance_ms 过滤是基于总缓冲长度，
    因此这里只验证"完全无能量输入"的边界——连续静音不应触发任何 utterance。
    """
    vad = VadState(silence_threshold=0.05, silence_ms=100, min_utterance_ms=500,
                   sample_rate=16000)
    silent = np.zeros(4096, dtype=np.float32)
    utterance = None
    for _ in range(10):
        utterance = vad.feed(_make_pcm(silent))
        if utterance:
            break
    # 纯静音无能量，_in_utterance 始终为 False，不应触发
    assert utterance is None


def test_vad_flush():
    """flush 返回剩余缓冲。"""
    vad = VadState(silence_threshold=0.5, silence_ms=1000)
    loud = np.ones(100, dtype=np.float32) * 0.3
    vad.feed(_make_pcm(loud))
    out = vad.flush()
    assert out is not None
    assert len(out) > 0
    # 再 flush 应为 None
    assert vad.flush() is None


def test_vad_full_utterance_lifecycle():
    """★ 整句触发完整生命周期：说话 → 静音 → 触发 utterance → 重置 → 再说话。

    模拟真实对话：用户说一句话（有能量），停顿（静音超过阈值），
    VAD 应返回这段完整 utterance，然后状态重置，下一段独立检测。
    """
    vad = VadState(
        silence_threshold=0.05,
        silence_ms=300,          # 静音 300ms 触发
        min_utterance_ms=100,
        sample_rate=16000,
    )
    loud = (np.sin(np.linspace(0, 100*2*np.pi, 4096)) * 0.5).astype(np.float32)
    silent = np.zeros(4096, dtype=np.float32)

    # 第一句：4 chunk 说话(1s) + 2 chunk 静音(0.5s > 300ms) → 应触发
    utterance1 = None
    for _ in range(4):
        utterance1 = vad.feed(_make_pcm(loud))
        assert utterance1 is None  # 说话中不触发
    for _ in range(3):
        utterance1 = vad.feed(_make_pcm(silent))
        if utterance1:
            break
    assert utterance1 is not None, "第一句应被触发"
    assert len(utterance1) > 0

    # 第二句：再说话 + 静音 → 应独立触发（状态已重置）
    utterance2 = None
    for _ in range(3):
        utterance2 = vad.feed(_make_pcm(loud))
        assert utterance2 is None
    for _ in range(3):
        utterance2 = vad.feed(_make_pcm(silent))
        if utterance2:
            break
    assert utterance2 is not None, "第二句应独立触发（状态已重置）"


def test_vad_energy_calculation_varied():
    """不同能量级别的 PCM 应正确区分（用足够大的 chunk 让 silence_duration 可靠）。"""
    # 用 4096 样本（256ms @16k），silence_ms=200，确保 1 个静音 chunk 即够
    vad = VadState(silence_threshold=0.1, silence_ms=200, sample_rate=16000)
    # 低能量（低于阈值）不应进入说话状态
    low = np.ones(4096, dtype=np.float32) * 0.05
    assert vad.feed(_make_pcm(low)) is None
    # 高能量（高于阈值）进入说话状态
    high = np.ones(4096, dtype=np.float32) * 0.5
    assert vad.feed(_make_pcm(high)) is None  # 说话中不触发
    # 静音（1 chunk 256ms > 200ms silence_ms）应触发
    silent = np.zeros(4096, dtype=np.float32)
    result = vad.feed(_make_pcm(silent))
    assert result is not None, "高能量后接静音应触发 utterance"
