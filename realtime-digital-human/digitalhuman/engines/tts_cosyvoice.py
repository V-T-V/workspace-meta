"""TTS 引擎：CosyVoice 实现（可选，本地可克隆音色）。

通过 subprocess 调 scripts/cosyvoice_tts.py，隔离重依赖。
与 tts_edge.py 的区别：本地推理（占显存 1.5-2GB），可克隆自定义音色。

★ subprocess 安全要点（C5）：
- stdout/stderr 并发读（避免 stderr 灌满 pipe buffer 死锁）
- finally 里 proc.kill() + await proc.wait()（避免僵尸进程）
- 用 sys.executable 而非 "python"（Windows/venv 正确性，M2）
"""
from __future__ import annotations

import asyncio
import logging
import os
import sys
from collections.abc import AsyncIterator

from .base import TTSEngine

log = logging.getLogger(__name__)


class CosyVoiceTTS(TTSEngine):
    """subprocess 桥接 CosyVoice。"""

    def __init__(self, script: str = "scripts/cosyvoice_tts.py",
                 voice_sample: str | None = None, sample_rate: int = 16000):
        self.script = script
        self.voice_sample = voice_sample
        self.sample_rate = sample_rate
        if not os.path.isfile(self.script):
            raise RuntimeError(
                f"CosyVoice 脚本不存在: {self.script}。请参考 scripts/cosyvoice_tts.py"
            )

    async def synthesize_stream(self, text: str) -> AsyncIterator[bytes]:
        if not text.strip():
            return
        # ★ sys.executable：用与主进程相同的解释器，确保 venv/依赖可见（M2）
        args = [sys.executable, self.script]
        if self.voice_sample:
            args += ["--voice", self.voice_sample]
        args += ["--sample-rate", str(self.sample_rate)]

        proc = await asyncio.create_subprocess_exec(
            *args,
            stdin=asyncio.subprocess.PIPE,
            stdout=asyncio.subprocess.PIPE,
            stderr=asyncio.subprocess.PIPE,
        )
        try:
            # 把文本写到 stdin
            proc.stdin.write(text.encode("utf-8"))
            await proc.stdin.drain()
            proc.stdin.close()

            # ★ 并发读 stdout/stderr（C5）：避免子进程崩溃时 stderr 灌满 pipe buffer
            # 阻塞写，导致 stdout.read() 永久挂死
            stdout_task = asyncio.create_task(proc.stdout.read())
            stderr_task = asyncio.create_task(proc.stderr.read())
            wav, stderr = await asyncio.gather(stdout_task, stderr_task)
            await proc.wait()

            if proc.returncode and proc.returncode != 0:
                raise RuntimeError(
                    f"CosyVoice 失败 code={proc.returncode}: "
                    f"{stderr.decode('utf-8', errors='ignore')[:300]}"
                )
        except Exception:
            # 异常路径：确保子进程被 kill（C5）
            if proc.returncode is None:
                proc.kill()
                try:
                    await asyncio.wait_for(proc.wait(), timeout=2.0)
                except asyncio.TimeoutError:
                    pass
            raise

        # 分块 yield（模拟流式）
        chunk_size = 4096
        for i in range(0, len(wav), chunk_size):
            yield wav[i:i + chunk_size]


def build_cosyvoice(script: str = "scripts/cosyvoice_tts.py",
                    voice_sample: str | None = None,
                    sample_rate: int = 16000) -> TTSEngine:
    return CosyVoiceTTS(script=script, voice_sample=voice_sample,
                        sample_rate=sample_rate)
