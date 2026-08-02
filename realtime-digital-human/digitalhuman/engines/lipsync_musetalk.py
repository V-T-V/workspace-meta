"""唇形引擎：MuseTalk 实现。

MuseTalk 是腾讯开源的实时音频驱动唇形同步，专为实时设计（30fps+）。
我们用 subprocess 桥接（隔离重依赖）：
- 主进程通过 stdin 喂音频片段
- subprocess（musetalk_render.py）渲染后通过 stdout 输出 JPEG 帧
- 帧协议见 frames.py：[4字节长度][JPEG payload]，0xFFFFFFFF=EOF

★ 性能优化：常驻进程模式
初版每次 render_stream 都 create_subprocess_exec，付 ~1.5s Python 启动 + cv2 import 税。
优化后 subprocess 常驻（--persistent），省去重复启动开销，多轮对话首帧延迟大幅降低。
"""
from __future__ import annotations

import asyncio
import logging
import os
import sys
from collections.abc import AsyncIterator

from .base import LipSyncEngine
from ..frames import iter_frames

log = logging.getLogger(__name__)


class MuseTalkLipSync(LipSyncEngine):
    """subprocess 桥接 MuseTalk（常驻模式，复用进程）。"""

    def __init__(self, musetalk_script: str = "scripts/musetalk_render.py",
                 fps: int = 25, width: int = 512, height: int = 512,
                 persistent: bool = True):
        self.script = musetalk_script
        self.fps = fps
        self.width = width
        self.height = height
        self.persistent = persistent
        if not os.path.isfile(self.script):
            raise RuntimeError(
                f"MuseTalk 渲染脚本不存在: {self.script}。"
                f"请参考 scripts/musetalk_render.py 创建"
            )
        # 常驻模式的进程状态（惰性启动）
        self._proc: asyncio.subprocess.Process | None = None
        self._proc_lock = asyncio.Lock()  # 保护 _proc 的启动/复用
        self._render_lock = asyncio.Lock()  # P1：渲染互斥（常驻 proc 单例，防多 session 帧交错）

    async def _get_or_start_proc(self) -> asyncio.subprocess.Process:
        """获取或启动常驻 subprocess。"""
        async with self._proc_lock:
            if self._proc is not None and self._proc.returncode is None:
                return self._proc
            # 启动新进程
            args = [sys.executable, self.script,
                    "--fps", str(self.fps),
                    "--width", str(self.width),
                    "--height", str(self.height)]
            if self.persistent:
                args.append("--persistent")
            proc = await asyncio.create_subprocess_exec(
                *args,
                stdin=asyncio.subprocess.PIPE,
                stdout=asyncio.subprocess.PIPE,
                stderr=asyncio.subprocess.PIPE,
            )
            log.info("MuseTalk subprocess 启动 pid=%d persistent=%s",
                     proc.pid, self.persistent)
            self._proc = proc
            return proc

    async def render_stream(
        self, portrait: bytes, audio_stream: AsyncIterator[bytes]
    ) -> AsyncIterator[bytes]:
        """喂音频段，收 JPEG 帧。

        常驻模式：复用已启动的 proc，只发音频段 + 段结束标记；
        非常驻模式：每次启动新 proc。
        """
        if self.persistent:
            # P1：渲染互斥——常驻 proc 是单例，多 session 同时渲染会帧交错
            async with self._render_lock:
                async for frame in self._render_persistent(portrait, audio_stream):
                    yield frame
        else:
            async for frame in self._render_oneshot(portrait, audio_stream):
                yield frame

    async def _render_persistent(
        self, portrait: bytes, audio_stream: AsyncIterator[bytes]
    ) -> AsyncIterator[bytes]:
        """常驻模式：复用 proc，发段 + 收帧到段结束标记。"""
        from ..frames import encode_frame, encode_segment_end

        proc = await self._get_or_start_proc()
        if proc.stdout is None or proc.stdin is None:
            raise RuntimeError("MuseTalk subprocess 的 stdio 未就绪")

        async def _feed():
            try:
                if portrait:
                    # m3：portrait 用魔数前缀区分。PCM16 小端音频首字节 \x00 常见，
                    # 但连续 9 字节 "\x00PORTRAIT:" 在真实音频中概率近乎 0，可接受。
                    proc.stdin.write(encode_frame(b"\x00PORTRAIT:" + portrait))
                    await proc.stdin.drain()
                # P1-4：批量喂音频——攒多个 chunk 一次 write+drain，减少 IPC 协调开销
                batch = bytearray()
                batch_count = 0
                BATCH_FLUSH = 4  # 攒 4 个 chunk 或流结束时 flush
                async for audio in audio_stream:
                    batch.extend(encode_frame(audio))
                    batch_count += 1
                    if batch_count >= BATCH_FLUSH:
                        proc.stdin.write(bytes(batch))
                        await proc.stdin.drain()
                        batch = bytearray()
                        batch_count = 0
                # flush 剩余
                if batch_count > 0:
                    proc.stdin.write(bytes(batch))
                    await proc.stdin.drain()
                # 段结束（不是 EOF，让 proc 继续等下一段）
                proc.stdin.write(encode_segment_end())
                await proc.stdin.drain()
            except (BrokenPipeError, ConnectionResetError):
                # proc 可能已崩溃，触发重启
                await self._reset_proc()
            except Exception as e:
                # C3：其他异常（如 loop closed）也要 reset，避免半坏 proc 被复用
                log.warning("_feed 异常，reset proc: %s", e)
                await self._reset_proc()

        feed_task = asyncio.create_task(_feed())

        try:
            # 读帧直到段结束标记（b""）或 EOF（None）
            async for frame in iter_frames(proc.stdout):
                if frame == b"":
                    # 段结束，本段渲染完成
                    break
                yield frame
        finally:
            try:
                await asyncio.wait_for(feed_task, timeout=5.0)
            except asyncio.TimeoutError:
                feed_task.cancel()
            # 检查 proc 是否还活着，死了则清理（下次自动重启）
            if self._proc and self._proc.returncode is not None:
                log.warning("MuseTalk proc 已退出 code=%s，下次 render 将重启",
                            self._proc.returncode)
                await self._drain_and_reset()

    async def _render_oneshot(
        self, portrait: bytes, audio_stream: AsyncIterator[bytes]
    ) -> AsyncIterator[bytes]:
        """非常驻模式（向后兼容）：每次启动新 proc。"""
        from ..frames import encode_frame, encode_segment_end

        args = [sys.executable, self.script,
                "--fps", str(self.fps),
                "--width", str(self.width),
                "--height", str(self.height)]
        proc = await asyncio.create_subprocess_exec(
            *args,
            stdin=asyncio.subprocess.PIPE,
            stdout=asyncio.subprocess.PIPE,
            stderr=asyncio.subprocess.PIPE,
        )
        log.info("MuseTalk subprocess（单次）启动 pid=%d", proc.pid)

        async def _feed_audio():
            try:
                if portrait:
                    # m3：portrait 用魔数前缀区分。PCM16 小端音频首字节 \x00 常见，
                    # 但连续 9 字节 "\x00PORTRAIT:" 在真实音频中概率近乎 0，可接受。
                    proc.stdin.write(encode_frame(b"\x00PORTRAIT:" + portrait))
                    await proc.stdin.drain()
                # P1-4：批量喂音频——攒多个 chunk 一次 write+drain，减少 IPC 协调开销
                batch = bytearray()
                batch_count = 0
                BATCH_FLUSH = 4  # 攒 4 个 chunk 或流结束时 flush
                async for audio in audio_stream:
                    batch.extend(encode_frame(audio))
                    batch_count += 1
                    if batch_count >= BATCH_FLUSH:
                        proc.stdin.write(bytes(batch))
                        await proc.stdin.drain()
                        batch = bytearray()
                        batch_count = 0
                # flush 剩余
                if batch_count > 0:
                    proc.stdin.write(bytes(batch))
                    await proc.stdin.drain()
                proc.stdin.write(encode_segment_end())
                await proc.stdin.drain()
            except (BrokenPipeError, ConnectionResetError):
                pass
            finally:
                try:
                    proc.stdin.close()
                except Exception:
                    pass

        feed_task = asyncio.create_task(_feed_audio())
        try:
            async for frame in iter_frames(proc.stdout):
                if frame:  # 跳过段结束标记
                    yield frame
        finally:
            await self._wait_and_drain(proc, feed_task)

    async def _reset_proc(self):
        """清理死掉的常驻 proc（下次 _get_or_start_proc 会重启）。"""
        if self._proc and self._proc.returncode is None:
            self._proc.kill()
        self._proc = None

    async def _drain_and_reset(self):
        """读 stderr 诊断 + 重置 proc 引用。"""
        if self._proc:
            try:
                stderr = await asyncio.wait_for(self._proc.stderr.read(), timeout=1.0)
                if stderr:
                    log.warning("MuseTalk stderr: %s",
                                stderr.decode("utf-8", errors="ignore")[:300])
            except asyncio.TimeoutError:
                pass
        self._proc = None

    async def _wait_and_drain(self, proc, feed_task):
        """单次模式：等 proc 退出 + 读 stderr。"""
        try:
            await feed_task
        except (asyncio.CancelledError, Exception):
            pass
        try:
            await asyncio.wait_for(proc.wait(), timeout=5.0)
        except asyncio.TimeoutError:
            proc.kill()
            try:
                await proc.wait()
            except Exception:
                pass
        if proc.returncode and proc.returncode != 0:
            try:
                stderr = await asyncio.wait_for(proc.stderr.read(), timeout=1.0)
                log.warning("MuseTalk 退出 code=%d stderr=%s",
                            proc.returncode,
                            stderr.decode("utf-8", errors="ignore")[:300])
            except asyncio.TimeoutError:
                pass

    async def close(self) -> None:
        """关闭常驻 proc（session 结束时调）。"""
        if self._proc and self._proc.returncode is None:
            try:
                # 发 EOF 让常驻 proc 正常退出
                from ..frames import encode_eof
                self._proc.stdin.write(encode_eof())
                await self._proc.stdin.drain()
                try:
                    await asyncio.wait_for(self._proc.wait(), timeout=3.0)
                except asyncio.TimeoutError:
                    self._proc.kill()
            except Exception:
                try:
                    self._proc.kill()
                except Exception:
                    pass
        self._proc = None


def build_lipsync(musetalk_script: str = "scripts/musetalk_render.py",
                  fps: int = 25, width: int = 512, height: int = 512) -> LipSyncEngine:
    """工厂：构造 MuseTalkLipSync（默认常驻模式）。失败抛 RuntimeError 由上层降级。"""
    return MuseTalkLipSync(musetalk_script=musetalk_script, fps=fps,
                           width=width, height=height, persistent=True)
