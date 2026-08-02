"""ASR 引擎：faster-whisper 实现。

faster-whisper 是 CTranslate2 后端的 Whisper，比 openai/whisper 快 4x，显存占用低。

输入：浏览器 PCM16 16k mono chunk 流（上游已由 VAD 切成完整句）
输出：完整句文本流（每次 yield 一句）

设计要点：
1. ★ 解码必须在线程池（C1）：transcribe + 迭代都放进 executor。
2. ★ 整句契约（C11）：上游由 VAD 喂整句，本引擎一次 transcribe 一段完整音频。
3. ★ 设备自动探测：device="auto" 时探测 CUDA，不可用回退 CPU（4060 直接跑，无 GPU 也能用）。
4. ★ 离线优先（P0）：模型名先探测本地目录（DH_PROJECT_ROOT/models/<model>/），
   存在完整 snapshot（model.bin+config.json+tokenizer.json）则用本地路径 + local_files_only，
   绝不联网；否则退回 HF hub 联网加载。
"""
from __future__ import annotations

import asyncio
import logging
import os
import time
from collections.abc import AsyncIterator

from .base import ASREngine
from ..utils.audio import pcm16_to_float32

log = logging.getLogger(__name__)

# faster-whisper 加载本地目录所需的关键文件（缺任一则视为不完整，退回 hub）
_LOCAL_SNAPSHOT_KEYS = ("model.bin", "config.json", "tokenizer.json")


def _resolve_model_path(model: str) -> tuple[str, bool]:
    """解析模型来源，返回 (model_path, local_only)。

    - model 本身是目录 → 直接用（local_only=True）
    - DH_PROJECT_ROOT/models/<model>/、models/whisper-<model>/ 或 cwd 同级存在完整
      snapshot → 本地（local_only=True）。离线包布局是 models/whisper-small/。
    - 否则原样返回模型名（走 HF hub 联网，local_only=False）
    """
    if os.path.isdir(model):
        return model, True

    roots = []
    env_root = os.environ.get("DH_PROJECT_ROOT")
    if env_root:
        roots.append(env_root)
    roots.append(os.getcwd())
    # app/ 里直接跑 EXE 时模型在上一级（离线包布局 app/ 与 models/ 同级）
    roots.append(os.path.dirname(os.getcwd()))

    # 候选目录：models/<model> 与 models/whisper-<model>（离线包布局）
    candidates = []
    for root in roots:
        if not root:
            continue
        candidates.append(os.path.join(root, "models", model))
        candidates.append(os.path.join(root, "models", f"whisper-{model}"))
    for cand in candidates:
        if os.path.isdir(cand) and all(
            os.path.isfile(os.path.join(cand, f)) for f in _LOCAL_SNAPSHOT_KEYS
        ):
            return cand, True
    return model, False


def _detect_device(requested: str) -> tuple[str, str]:
    """探测可用设备，返回 (device, compute_type)。

    requested="auto"：优先 cuda（int8_float16），无 GPU 回退 cpu（int8）。
    显式指定 device 时按指定值，compute_type 仍自适应。

    ★ 检测顺序（不依赖 torch——faster-whisper 用 CTranslate2 后端，独立于 torch）：
    1. CTranslate2 的 get_cuda_device_count()
    2. nvidia-smi 命令存在
    3. torch.cuda.is_available()（兼容旧检测）
    """
    if requested == "auto":
        # 方法 1：CTranslate2 原生 CUDA 检测（最可靠）
        try:
            import ctranslate2
            if ctranslate2.get_cuda_device_count() > 0:
                log.info("CTranslate2 检测到 CUDA，ASR 使用 GPU (int8_float16)")
                return "cuda", "int8_float16"
        except Exception:
            pass

        # 方法 2：nvidia-smi 存在（说明有 NVIDIA GPU 驱动）
        try:
            import shutil
            if shutil.which("nvidia-smi"):
                log.info("检测到 nvidia-smi，ASR 尝试 GPU (int8_float16)")
                return "cuda", "int8_float16"
        except Exception:
            pass

        # 方法 3：torch（兼容旧检测，但 faster-whisper 不依赖 torch）
        try:
            import torch
            if torch.cuda.is_available():
                log.info("torch 检测到 CUDA，ASR 使用 GPU (int8_float16)")
                return "cuda", "int8_float16"
        except ImportError:
            pass

        log.info("未检测到 CUDA，ASR 回退 CPU (int8)")
        return "cpu", "int8"
    # 显式指定：cuda 配 float16，cpu 配 int8
    compute = "int8_float16" if requested == "cuda" else "int8"
    return requested, compute


class WhisperASR(ASREngine):
    """faster-whisper 流式 ASR（VAD 驱动的整句识别）。"""

    def __init__(self, model: str = "small", language: str | None = "zh",
                 device: str = "auto", compute_type: str | None = None,
                 silence_ms: int = 500):
        try:
            from faster_whisper import WhisperModel
        except ImportError as e:
            raise RuntimeError(
                "未安装 faster-whisper。请 `pip install faster-whisper`"
                "（GPU 用户用 deploy_gpu.bat 装 CUDA 版）"
            ) from e

        # ★ 设备自动探测（4060 直接跑，无 GPU 回退 CPU）
        actual_device, actual_compute = _detect_device(device)
        if compute_type:
            actual_compute = compute_type  # 显式覆盖

        # ★ 离线优先：本地 snapshot 存在则绝不联网（P0：离线包无网时不再 30s 超时降级）
        model_path, local_only = _resolve_model_path(model)
        source_desc = f"本地目录 {model_path}" if local_only else f"模型名 {model}（联网模式）"
        log.info("加载 whisper 模型: %s (%s/%s)，来源=%s, local_files_only=%s",
                 model, actual_device, actual_compute, source_desc, local_only)
        t0 = time.monotonic()
        self._model = WhisperModel(
            model_path, device=actual_device, compute_type=actual_compute,
            local_files_only=local_only,
        )
        log.info("whisper 模型就绪，耗时 %.2fs", time.monotonic() - t0)
        self._language = language
        self._silence_ms = silence_ms

    async def transcribe_stream(
        self, audio_stream: AsyncIterator[bytes]
    ) -> AsyncIterator[str]:
        """累积上游 PCM（一段完整 utterance），一次性 transcribe，逐 segment yield。"""
        buffer: list[bytes] = []
        async for chunk in audio_stream:
            buffer.append(chunk)

        if not buffer:
            return

        segments = await self._transcribe_buffer(buffer)
        for seg in segments:
            if seg.strip():
                yield seg

    async def _transcribe_buffer(self, buffer: list[bytes]) -> list[str]:
        """把 PCM buffer 喂给 whisper，返回 segment 文本列表。

        ★ transcribe + 迭代都在线程池里跑，避免阻塞事件循环。
        """
        pcm = b"".join(buffer)
        samples = pcm16_to_float32(pcm)
        loop = asyncio.get_running_loop()
        return await loop.run_in_executor(None, self._run_transcribe, samples)

    def _run_transcribe(self, samples) -> list[str]:
        """同步函数：在 executor 线程里跑 transcribe + 迭代。"""
        segments, _info = self._model.transcribe(
            samples,
            language=self._language,
            vad_filter=True,
            beam_size=1,  # 实时优先
        )
        return [seg.text for seg in segments]


def build_asr(model: str = "small", language: str | None = "zh",
              device: str = "auto", compute_type: str | None = None,
              silence_ms: int = 500) -> ASREngine:
    """工厂：构造 WhisperASR。device="auto" 自动探测。失败抛 RuntimeError 由上层降级。"""
    return WhisperASR(model=model, language=language, device=device,
                      compute_type=compute_type, silence_ms=silence_ms)
