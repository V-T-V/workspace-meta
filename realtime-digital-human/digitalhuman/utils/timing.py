"""延迟埋点。

记录各阶段首字节/首帧时间戳，供延迟统计与 bench_latency 使用。
"""
from __future__ import annotations

import time
from dataclasses import dataclass


@dataclass
class StageTiming:
    """单次回复的各阶段时间戳（相对于 reply_start 的秒数）。"""
    reply_start: float = 0.0          # 用户句尾被检测到的时刻（基准 0）
    first_token: float | None = None  # LLM 吐第一个 token
    first_sentence: float | None = None  # 句切分器凑齐第一短句
    first_audio: float | None = None  # TTS 出第一段音频
    first_frame: float | None = None  # 唇形出第一帧
    reply_end: float | None = None    # 整段回复结束

    def mark_start(self) -> None:
        self.reply_start = time.monotonic()

    def mark_first_token(self) -> None:
        if self.first_token is None:
            self.first_token = time.monotonic()

    def mark_first_sentence(self) -> None:
        if self.first_sentence is None:
            self.first_sentence = time.monotonic()

    def mark_first_audio(self) -> None:
        if self.first_audio is None:
            self.first_audio = time.monotonic()

    def mark_first_frame(self) -> None:
        if self.first_frame is None:
            self.first_frame = time.monotonic()

    def mark_end(self) -> None:
        self.reply_end = time.monotonic()

    def summary(self) -> dict[str, float]:
        """返回各阶段相对基准的延迟（秒）。None 字段不出现。"""
        base = self.reply_start or 0.0
        out: dict[str, float] = {}
        for name in ("first_token", "first_sentence", "first_audio", "first_frame", "reply_end"):
            v = getattr(self, name)
            if v is not None:
                out[name] = round(v - base, 4)
        return out
