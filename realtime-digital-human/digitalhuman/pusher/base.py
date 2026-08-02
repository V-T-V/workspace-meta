"""Pusher 工厂与默认实现。

ws_mjpeg: 把帧/音频通过 WebSocket 推给浏览器（默认）。
disabled: 不推流（测试用）。

WebRTC 实现留待 Phase 2：实现 pusher.webrtc_aiortc.WebRTCPusher，
在 build_pusher 里加一个 elif 分支即可。
"""
from __future__ import annotations

import asyncio
import logging
from typing import Any

from ..engines.base import Pusher as PusherBase

log = logging.getLogger(__name__)


class WSMjpegPusher(PusherBase):
    """WS + MJPEG 推流器。

    持有一个 session_id → WebSocket 的映射。
    pipeline 通过 push_frame/push_audio 调用，
    本对象把数据打包成 WS 二进制消息写回对应连接。

    ★ P1-2：每会话一个有界发送队列 + 独立发送 task，解耦渲染与网络。
      慢客户端时队列满则丢旧帧（视频流可丢，保最新），push_frame 立即返回 <1ms，
      不再反压 lipsync 导致渲染串行卡顿。
    """

    # 每会话帧队列上限（视频流可丢，保最新 N 帧）
    FRAME_QUEUE_MAX = 8

    def __init__(self, send_timeout: float = 2.0) -> None:
        # session_id -> 可发送协程的包装对象（server 注入）
        self._conns: dict[str, Any] = {}
        self._send_timeout = send_timeout  # ★ 背压保护：单次 send 超时
        # P1-2：每会话帧发送队列 + 发送 task（仅视频帧解耦，音频/文字仍直发保时序）
        self._frame_queues: dict[str, asyncio.Queue] = {}
        self._frame_senders: dict[str, asyncio.Task] = {}

    def bind(self, session_id: str, ws_send) -> None:
        """server 在 WS 接入时调用，绑定发送回调。"""
        self._conns[session_id] = ws_send
        # 建独立帧发送队列 + task
        q: asyncio.Queue = asyncio.Queue(maxsize=self.FRAME_QUEUE_MAX)
        self._frame_queues[session_id] = q
        self._frame_senders[session_id] = asyncio.create_task(
            self._frame_sender_loop(session_id, q), name=f"frame-snd-{session_id}")

    def unbind(self, session_id: str) -> None:
        self._conns.pop(session_id, None)
        sender = self._frame_senders.pop(session_id, None)
        if sender and not sender.done():
            sender.cancel()
        self._frame_queues.pop(session_id, None)

    async def _frame_sender_loop(self, session_id: str, q: asyncio.Queue) -> None:
        """独立 task：从队列取帧发送，慢客户端超时 unbind。"""
        from ..frames import ws_pack_frame
        while True:
            try:
                jpeg = await q.get()
            except asyncio.CancelledError:
                return
            if jpeg is None:
                return
            await self._safe_send(session_id, ws_pack_frame(jpeg))

    async def _safe_send(self, session_id: str, data: bytes) -> None:
        """★ 背压保护：带超时的 send，慢客户端/断连不冻住 pipeline。

        超时或异常时 unbind 该 session（视为断连），后续推送静默丢弃。
        """
        send = self._conns.get(session_id)
        if send is None:
            return
        try:
            await asyncio.wait_for(send(data), timeout=self._send_timeout)
        except (asyncio.TimeoutError, Exception) as e:
            # 慢客户端或连接已断 —— unbind 避免后续重复超时
            log.debug("pusher send 失败/超时，unbind session=%s: %s", session_id, e)
            self.unbind(session_id)

    async def push_frame(self, session_id: str, jpeg: bytes) -> None:
        """★ P1-2：帧入队即返回（<1ms），慢客户端不反压 lipsync。队列满丢旧帧保最新。"""
        q = self._frame_queues.get(session_id)
        if q is None:
            return  # 无绑定，静默丢
        if q.full():
            try:
                q.get_nowait()  # 丢最旧的帧（视频流可丢，保最新）
            except asyncio.QueueEmpty:
                pass
        try:
            q.put_nowait(jpeg)
        except asyncio.QueueFull:
            pass  # 极端情况仍丢，保渲染不阻塞

    async def push_audio(self, session_id: str, pcm: bytes) -> None:
        from ..frames import ws_pack_audio
        await self._safe_send(session_id, ws_pack_audio(pcm))

    async def push_text(self, session_id: str, text: str) -> None:
        from ..frames import ws_pack_text
        await self._safe_send(session_id, ws_pack_text(text))

    async def push_latency(self, session_id: str, timing_ms: dict) -> None:
        """推送延迟统计（M1：接通可观测性链路）。"""
        import json
        from ..frames import WS_MSG_LATENCY, ws_pack
        await self._safe_send(session_id, ws_pack(WS_MSG_LATENCY, json.dumps(timing_ms).encode("utf-8")))

    async def close(self, session_id: str) -> None:
        self.unbind(session_id)


class DisabledPusher(PusherBase):
    """丢弃所有推送（测试/disabled 配置用）。"""

    async def push_frame(self, session_id: str, jpeg: bytes) -> None:
        return None

    async def push_audio(self, session_id: str, pcm: bytes) -> None:
        return None


def build_pusher(backend: str) -> PusherBase:
    if backend == "ws_mjpeg":
        return WSMjpegPusher()
    if backend in ("disabled", "none", ""):
        return DisabledPusher()
    raise ValueError(f"未知 pusher backend: {backend}")
