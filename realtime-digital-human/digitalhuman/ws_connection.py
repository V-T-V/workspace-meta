"""WS 连接处理器（从 server.py 的 ws_endpoint 拆出）。

将 240 行的 ws_endpoint 单函数拆成可测试的 WSConnection 类。
职责：鉴权、限流、session 生命周期、音频缓冲、VAD、pump、消息分发、清理。
"""
from __future__ import annotations

import asyncio
import logging
import time

from fastapi import WebSocket, WebSocketDisconnect

from .config import Config
from .engines.base import ASREngine, LLMEngine, LipSyncEngine, Pusher, TTSEngine
from .frames import WS_MSG_AUDIO, WS_MSG_END, WS_MSG_TEXT, WS_MSG_UTTERANCE_END, ws_pack, ws_unpack
from .pusher.base import WSMjpegPusher
from .session import Session, SessionRegistry
from .store import SQLiteStore

log = logging.getLogger(__name__)

MAX_AUDIO_BUFFER_BYTES = 30 * 1024 * 1024  # 30MB（约 16 分钟 PCM@16k）


class WSConnection:
    """单路 WS 连接的完整生命周期管理。

    从 server.ws_endpoint 拆出，每个方法对应原函数的一个逻辑块：
    - check_access：鉴权 + 并发限流
    - setup_session：创建 session + 加载历史 + 欢迎语
    - run：主循环（启动 pump + 接收消息 + 分发）
    - handle_audio / handle_utterance_end / handle_set_persona：消息处理
    - maybe_barge_in：打断检测
    - flush_utterance / pump_utterances / run_pipeline：音频→pipeline
    - cleanup：清理
    """

    def __init__(self, ws: WebSocket, session_id: str, cfg: Config,
                 asr: ASREngine, llm: LLMEngine, tts: TTSEngine,
                 lipsync: LipSyncEngine, pusher: Pusher, portrait: bytes,
                 registry: SessionRegistry, store: SQLiteStore | None):
        self.ws = ws
        self.session_id = session_id
        self.cfg = cfg
        self.asr = asr
        self.llm = llm
        self.tts = tts
        self.lipsync = lipsync
        self.pusher = pusher
        self.portrait = portrait
        self.registry = registry
        self.store = store

        self.sess: Session | None = None
        self.audio_buffer: list[bytes] = []
        self.audio_buffer_total = 0
        self.utterance_queue: asyncio.Queue = asyncio.Queue()
        self.pump_alive = True
        self.vad_state = None
        self.pump_task: asyncio.Task | None = None

    async def ws_send(self, data: bytes) -> None:
        """安全发送 WS 二进制数据。"""
        try:
            await self.ws.send_bytes(data)
        except Exception as e:
            log.debug("ws send 失败: %s", e)

    async def handle(self) -> None:
        """主入口：鉴权 → session → 接收循环 → 清理。"""
        t0 = time.monotonic()
        client = self.ws.client
        client_desc = f"{client[0]}:{client[1]}" if client else "?"
        log.info("WS 连接开始 session=%s client=%s", self.session_id, client_desc)
        if not await self._check_access():
            return
        self._setup_session()
        if self.sess is None:
            return
        self._bind_pusher()
        await self._send_engine_status()
        self._init_vad()
        self.pump_task = asyncio.create_task(self._pump_utterances(), name=f"pump-{self.session_id}")
        try:
            await self._receive_loop()
        except WebSocketDisconnect:
            log.info("WS 断开（客户端关闭）session=%s 时长=%.1fs 收音频=%.1fKB",
                     self.session_id, time.monotonic() - t0,
                     getattr(self, "_audio_bytes", 0) / 1024)
        except Exception as e:
            log.exception("WS 异常 session=%s: %s", self.session_id, e)
        finally:
            await self._cleanup()
            log.info("WS 会话结束 session=%s 总时长=%.1fs 收音频=%.1fKB 管线轮次=%d",
                     self.session_id, time.monotonic() - t0,
                     getattr(self, "_audio_bytes", 0) / 1024,
                     getattr(self, "_pipeline_runs", 0))

    async def _check_access(self) -> bool:
        """鉴权 + 并发限流。失败返回 False（已关闭 WS）。"""
        # 鉴权
        if self.cfg.server.auth_token:
            token = self.ws.query_params.get("token", "")
            if token != self.cfg.server.auth_token:
                await self.ws.accept()
                await self.ws.close(code=1008, reason="鉴权失败：token 错误")
                log.warning("WS 鉴权失败 session=%s", self.session_id)
                return False
        # 并发上限
        if self.registry.is_full():
            await self.ws.accept()
            await self.ws.close(code=1013, reason=f"服务繁忙（上限 {self.registry.max_sessions} 路）")
            log.warning("WS 拒绝（满 %d/%d）session=%s", len(self.registry), self.registry.max_sessions, self.session_id)
            return False
        await self.ws.accept()
        log.info("WS 接入 session=%s（%d/%d）", self.session_id, len(self.registry) + 1, self.registry.max_sessions)
        return True

    def _setup_session(self) -> None:
        """创建 session + 加载历史 + 触发欢迎语。"""
        self.sess = Session(
            session_id=self.session_id, cfg=self.cfg,
            asr=self.asr, llm=self.llm, tts=self.tts,
            lipsync=self.lipsync, pusher=self.pusher,
            portrait=self.portrait, store=self.store,
        )
        # 加载历史
        if self.store is not None:
            try:
                loaded = self.store.load_history(self.session_id, limit=12)
                if loaded:
                    self.sess.history = loaded
                    log.info("session=%s 恢复 %d 条历史", self.session_id, len(loaded))
            except Exception as e:
                log.warning("加载历史失败: %s", e)
        # 欢迎语
        try:
            if self.cfg.personas.list:
                dp = next((p for p in self.cfg.personas.list if p.id == self.cfg.personas.default_id),
                          self.cfg.personas.list[0])
                if dp.greeting:
                    asyncio.create_task(self.sess.greet(dp.greeting))
        except Exception as e:
            log.debug("欢迎语未触发: %s", e)
        # 加入 registry
        if not self.registry.add(self.sess):
            asyncio.ensure_future(self.ws.close(code=1013, reason="并发已满"))
            self.sess = None

    def _bind_pusher(self) -> None:
        if isinstance(self.pusher, WSMjpegPusher):
            self.pusher.bind(self.session_id, self.ws_send)

    async def _send_engine_status(self) -> None:
        """推送引擎状态（哪些是 Mock 降级），让前端显示提示。"""
        import json
        engines = {
            "asr": type(self.asr).__name__,
            "llm": type(self.llm).__name__,
            "tts": type(self.tts).__name__,
            "lipsync": type(self.lipsync).__name__,
        }
        # 检测降级（Mock 开头的都是降级）
        warnings = []
        for key, name in engines.items():
            if name.startswith("Mock"):
                warnings.append(f"{key.upper()} 降级为 Mock（{name}）")
        # LLM 降级最关键（数字人不会思考）
        if "MockLLM" in engines["llm"]:
            warnings.append("⚠ 数字人大脑未就绪——请检查 Ollama 是否已启动 + 模型是否已拉取")
        status = {"engines": engines, "warnings": warnings}
        await self.ws_send(ws_pack(0x14, json.dumps(status).encode("utf-8")))

    def _init_vad(self) -> None:
        if self.cfg.asr.vad_auto_trigger:
            from .utils.vad import VadState
            self.vad_state = VadState(
                silence_threshold=self.cfg.asr.vad_threshold,
                silence_ms=self.cfg.asr.silence_ms,
                min_utterance_ms=300, sample_rate=16000,
            )

    async def _receive_loop(self) -> None:
        """主接收循环：分发消息。"""
        self._audio_bytes = 0     # 本次连接累计音频字节（会话统计）
        self._pipeline_runs = 0   # 本次连接 pipeline 轮次
        while True:
            msg = await self.ws.receive()
            if msg["type"] == "websocket.disconnect":
                break
            if "bytes" in msg and msg["bytes"] is not None:
                await self._handle_bytes(msg["bytes"])
            elif "text" in msg and msg["text"]:
                if msg["text"] == "stop":
                    log.info("收到 stop 指令 session=%s", self.session_id)
                    await self.sess.cancel()
                    self.audio_buffer.clear()
                    self.audio_buffer_total = 0

    async def _handle_bytes(self, data: bytes) -> None:
        """处理二进制消息（音频/控制信令）。"""
        try:
            msg_type, payload = ws_unpack(data)
        except ValueError as e:
            log.warning("非法 WS 帧: %s", e)
            return
        if msg_type == WS_MSG_AUDIO:
            self._audio_bytes = getattr(self, "_audio_bytes", 0) + len(payload)
            await self._handle_audio(payload)
        elif msg_type == WS_MSG_UTTERANCE_END:
            await self._flush_utterance()
        elif msg_type == 0x12:  # SET_PERSONA
            self._handle_set_persona(payload)
        elif msg_type == 0x13:  # SET_TTS_RATE
            self._handle_set_tts_rate(payload)
        elif msg_type == 0x15:  # TEXT_INPUT
            await self._handle_text_input(payload)
        elif msg_type == 0x16:  # CLEAR_HISTORY
            await self._handle_clear_history(payload)
        else:
            log.debug("忽略未知 WS 消息 0x%02x", msg_type)

    async def _handle_audio(self, payload: bytes) -> None:
        """处理音频 chunk：缓冲 + barge-in + VAD 句尾。"""
        # OOM 保护
        if self.audio_buffer_total > MAX_AUDIO_BUFFER_BYTES:
            log.warning("audio_buffer 超限，丢弃旧数据")
            self.audio_buffer.clear()
            self.audio_buffer_total = 0
        self.audio_buffer.append(payload)
        self.audio_buffer_total += len(payload)
        # Barge-in 打断
        await self._maybe_barge_in(payload)
        # VAD 句尾检测
        if self.vad_state is not None:
            utterance = self.vad_state.feed(payload)
            if utterance is not None:
                await self._flush_utterance()

    async def _maybe_barge_in(self, payload: bytes) -> None:
        """检测用户开口 → 打断当前回复。"""
        if self.vad_state is None or self.sess._current is None:
            return
        from .utils.audio import pcm16_to_float32, rms_energy
        samples = pcm16_to_float32(payload)
        if rms_energy(samples) > self.vad_state.silence_threshold:
            log.info("Barge-in：用户打断")
            try:
                from .metrics import get_metrics
                get_metrics().record_barge_in()
            except Exception:
                pass
            await self.sess.cancel()
            await self.ws_send(ws_pack(0x11, b""))  # INTERRUPT

    def _handle_set_persona(self, payload: bytes) -> None:
        """热切换角色（set_persona 是同步方法）。"""
        persona_id = payload.decode("utf-8", errors="ignore").strip()
        matched = [p for p in self.cfg.personas.list if p.id == persona_id]
        if matched:
            self.sess.set_persona(matched[0])
            log.info("切换角色 session=%s persona=%s", self.session_id, persona_id)
        else:
            log.warning("未知角色 id: %s", persona_id)

    def _handle_set_tts_rate(self, payload: bytes) -> None:
        """运行时调整 TTS 语速（如 "+20%" 加速 / "-10%" 减速）。"""
        rate = payload.decode("utf-8", errors="ignore").strip()
        setattr(self.sess.tts, "rate", rate)
        log.info("TTS 语速调整 session=%s rate=%s", self.session_id, rate)

    async def _handle_text_input(self, payload: bytes) -> None:
        """文字输入：绕过 ASR，直接把文字当用户发言喂 pipeline。"""
        text = payload.decode("utf-8", errors="ignore").strip()
        if not text:
            return
        log.info("文字输入 session=%s: %s", self.session_id, text[:50])
        # 推 user 字幕
        from .frames import ws_pack, WS_MSG_TEXT
        await self.ws_send(ws_pack(WS_MSG_TEXT, f"[user] {text}".encode("utf-8")))
        # 把文字作为 mic_stream 的替代，直接触发 pipeline
        # 用一个假音频流（空），通过 InjectASR 风格注入
        async def text_mic():
            yield b"\x00" * 10  # 占位音频（ASR 不会处理，因为我们直接注入）

        # 临时替换 ASR 为注入式（这次调用用完即弃）
        from .engines.mock import InjectASR
        original_asr = self.sess.asr
        self.sess.asr = InjectASR(text)
        try:
            await self._run_pipeline_with_text()
        finally:
            self.sess.asr = original_asr

    async def _run_pipeline_with_text(self) -> None:
        """文字输入的 pipeline 执行（复用 run_pipeline 的逻辑）。"""
        async def mic_stream():
            yield b"\x00" * 10
        try:
            await self.sess.handle_utterance(mic_stream())
        except asyncio.CancelledError:
            pass
        except Exception as e:
            log.exception("文字输入 pipeline 异常: %s", e)
            await self.ws_send(ws_pack(0x03, "[error:fatal] 处理失败".encode()))
        finally:
            await self.ws_send(ws_pack(0x04, b""))  # END

    async def _handle_clear_history(self, payload: bytes) -> None:
        """清空对话历史（遗忘）。"""
        self.sess.history.clear()
        if self.store is not None:
            try:
                # ★ H-3：同步 sqlite DELETE 放 executor，避免阻塞事件循环
                loop = asyncio.get_running_loop()
                await loop.run_in_executor(None, self.store.clear_session, self.session_id)
            except Exception as e:
                log.warning("清空持久化历史失败: %s", e)
        log.info("对话已清空 session=%s", self.session_id)
        await self.ws_send(ws_pack(0x03, "[system] 对话已重置".encode("utf-8")))

    async def _flush_utterance(self) -> None:
        """快照音频 + 入队（不阻塞接收循环）。"""
        if not self.audio_buffer:
            return
        chunks = list(self.audio_buffer)
        self.audio_buffer.clear()
        self.audio_buffer_total = 0
        await self.utterance_queue.put(chunks)

    async def _pump_utterances(self) -> None:
        """串行消费 utterance_queue，执行 pipeline。"""
        while self.pump_alive:
            try:
                chunks = await asyncio.wait_for(self.utterance_queue.get(), timeout=0.5)
            except asyncio.TimeoutError:
                continue
            if chunks is None:
                break
            await self._run_pipeline(chunks)

    async def _run_pipeline(self, chunks: list[bytes]) -> None:
        """执行一次 pipeline，推 END。"""
        async def mic_stream():
            for c in chunks:
                yield c
        self._pipeline_runs = getattr(self, "_pipeline_runs", 0) + 1
        t0 = time.monotonic()
        log.info("pipeline 开始 session=%s 轮次=%d 音频=%.1fKB",
                 self.session_id, self._pipeline_runs, sum(len(c) for c in chunks) / 1024)
        try:
            await self.sess.handle_utterance(mic_stream())
        except asyncio.CancelledError:
            raise
        except Exception as e:
            log.exception("pipeline 异常: %s", e)
            await self.ws_send(ws_pack(WS_MSG_TEXT, "[error:fatal] 处理失败".encode()))
        finally:
            log.info("pipeline 完成 session=%s 耗时=%.2fs", self.session_id,
                     time.monotonic() - t0)
            await self.ws_send(ws_pack(WS_MSG_END, b""))

    async def _cleanup(self) -> None:
        """清理：停 pump + cancel + unbind + remove。"""
        self.pump_alive = False
        await self.utterance_queue.put(None)
        if self.pump_task and not self.pump_task.done():
            self.pump_task.cancel()
            try:
                await self.pump_task
            except asyncio.CancelledError:
                pass  # 正常取消
            except Exception as e:
                # ★ M1：真实 BUG 不再静默吞（原 except 把 CancelledError 和真实异常混着 pass）
                log.debug("pump task 异常退出 session=%s: %s", self.session_id, e)
        if self.sess:
            try:
                await self.sess.cancel()
            except Exception as e:
                log.debug("cancel 异常: %s", e)
        if isinstance(self.pusher, WSMjpegPusher):
            self.pusher.unbind(self.session_id)
        self.registry.remove(self.session_id)
        log.info("WS 清理完成 session=%s", self.session_id)
