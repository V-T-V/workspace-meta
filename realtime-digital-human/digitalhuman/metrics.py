"""生产可观测性指标（Prometheus 格式）。

收集各 stage 延迟、TTS 失败率、session 数等，通过 /metrics 端点暴露。
零依赖（手写 Prometheus exposition format，纯文本）。

指标：
  dh_first_token_ms / dh_first_audio_ms / dh_first_frame_ms（延迟样本）
  dh_pipeline_total / dh_pipeline_cancel_total / dh_tts_fail_total（计数）
  dh_sessions_active（当前活跃 session 数）
  dh_audio_buffer_bytes（音频缓冲水位）
"""
from __future__ import annotations

import threading
import time
from collections import deque


class MetricsRegistry:
    """全局指标注册表（线程安全，asyncio 单线程下也安全）。

    延迟类指标用滑动窗口（最近 N 个样本）算 P50/P95。
    计数器单调递增。
    """

    def __init__(self, window_size: int = 100):
        self._lock = threading.Lock()
        self._window = window_size
        # 延迟样本（毫秒）
        self._first_token_ms: deque = deque(maxlen=window_size)
        self._first_audio_ms: deque = deque(maxlen=window_size)
        self._first_frame_ms: deque = deque(maxlen=window_size)
        # 计数器
        self._pipeline_total = 0
        self._pipeline_cancel_total = 0
        self._tts_fail_total = 0
        self._barge_in_total = 0
        # 实时值
        self._sessions_active = 0
        self._audio_buffer_bytes = 0
        # 启动时间
        self._start_time = time.time()

    def record_pipeline_complete(self, timing_ms: dict) -> None:
        """pipeline 完成时记录延迟。"""
        with self._lock:
            self._pipeline_total += 1
            if "first_token_ms" in timing_ms:
                self._first_token_ms.append(timing_ms["first_token_ms"])
            if "first_audio_ms" in timing_ms:
                self._first_audio_ms.append(timing_ms["first_audio_ms"])
            if "first_frame_ms" in timing_ms:
                self._first_frame_ms.append(timing_ms["first_frame_ms"])

    def record_pipeline_cancel(self) -> None:
        with self._lock:
            self._pipeline_cancel_total += 1

    def record_tts_fail(self) -> None:
        with self._lock:
            self._tts_fail_total += 1

    def record_barge_in(self) -> None:
        with self._lock:
            self._barge_in_total += 1

    def set_sessions_active(self, n: int) -> None:
        with self._lock:
            self._sessions_active = n

    def set_audio_buffer_bytes(self, n: int) -> None:
        with self._lock:
            self._audio_buffer_bytes = n

    def snapshot(self) -> dict:
        """★ M-3：线程安全地返回延迟样本拷贝（供 dashboard/管理台曲线图用）。

        在锁内 list() 拷贝 deque，避免迭代时被写侧 append 导致 RuntimeError。
        """
        with self._lock:
            return {
                "first_token_ms": list(self._first_token_ms),
                "first_audio_ms": list(self._first_audio_ms),
                "first_frame_ms": list(self._first_frame_ms),
                "pipeline_total": self._pipeline_total,
                "pipeline_cancel_total": self._pipeline_cancel_total,
                "tts_fail_total": self._tts_fail_total,
                "barge_in_total": self._barge_in_total,
                "sessions_active": self._sessions_active,
                "uptime_seconds": int(time.time() - self._start_time),
            }

    @staticmethod
    def _percentile(samples: list, p: float) -> float:
        if not samples:
            return 0.0
        s = sorted(samples)
        k = int(len(s) * p / 100)
        k = min(k, len(s) - 1)
        return float(s[k])

    def render_prometheus(self) -> str:
        """输出 Prometheus exposition format 文本。"""
        with self._lock:
            uptime = time.time() - self._start_time
            lines = []

            # 延迟分位（摘要 + 直方图替代）
            for name, samples in [
                ("first_token_ms", list(self._first_token_ms)),
                ("first_audio_ms", list(self._first_audio_ms)),
                ("first_frame_ms", list(self._first_frame_ms)),
            ]:
                p50 = self._percentile(samples, 50)
                p95 = self._percentile(samples, 95)
                lines.append(f"# HELP dh_{name} {name.replace('_', ' ')} (ms)")
                lines.append(f"# TYPE dh_{name} summary")
                lines.append(f'dh_{name}{{quantile="0.5"}} {p50}')
                lines.append(f'dh_{name}{{quantile="0.95"}} {p95}')
                lines.append(f"dh_{name}_count {len(samples)}")

            # 计数器
            lines.append("# HELP dh_pipeline_total Total pipelines completed")
            lines.append("# TYPE dh_pipeline_total counter")
            lines.append(f"dh_pipeline_total {self._pipeline_total}")

            lines.append("# HELP dh_pipeline_cancel_total Total pipelines cancelled (barge-in/error)")
            lines.append("# TYPE dh_pipeline_cancel_total counter")
            lines.append(f"dh_pipeline_cancel_total {self._pipeline_cancel_total}")

            lines.append("# HELP dh_tts_fail_total Total TTS failures")
            lines.append("# TYPE dh_tts_fail_total counter")
            lines.append(f"dh_tts_fail_total {self._tts_fail_total}")

            lines.append("# HELP dh_barge_in_total Total user interruptions")
            lines.append("# TYPE dh_barge_in_total counter")
            lines.append(f"dh_barge_in_total {self._barge_in_total}")

            # 实时值
            lines.append("# HELP dh_sessions_active Active sessions")
            lines.append("# TYPE dh_sessions_active gauge")
            lines.append(f"dh_sessions_active {self._sessions_active}")

            lines.append("# HELP dh_audio_buffer_bytes Current audio buffer size (bytes)")
            lines.append("# TYPE dh_audio_buffer_bytes gauge")
            lines.append(f"dh_audio_buffer_bytes {self._audio_buffer_bytes}")

            # 运行时长
            lines.append("# HELP dh_uptime_seconds Service uptime")
            lines.append("# TYPE dh_uptime_seconds gauge")
            lines.append(f"dh_uptime_seconds {uptime:.0f}")

            return "\n".join(lines) + "\n"


# 全局单例
_metrics: MetricsRegistry | None = None


def get_metrics() -> MetricsRegistry:
    global _metrics
    if _metrics is None:
        _metrics = MetricsRegistry()
    return _metrics
