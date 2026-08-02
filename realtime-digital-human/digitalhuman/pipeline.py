"""管线协调器：4 阶段流式接力（核心）。

设计参照 auto-finance-assistant 的 AnswerStream：buffered Queue + 后台 Task + ctx 取消。
绝不串行等待——每阶段独立 Task，靠 asyncio.Queue 接力。

阶段：
    ASR(text) → LLM(token) → 句切分(sentence) → TTS(audio) → 唇形(frame) → pusher

输入：async generator 产出 PCM chunk（来自浏览器麦克风）
输出：通过 pusher 推送帧/音频/文字到客户端
"""
from __future__ import annotations

import asyncio
import logging
from dataclasses import dataclass, field
from collections.abc import AsyncIterator, Callable

from .config import Config
from .engines.base import (
    ASREngine,
    LLMEngine,
    LipSyncEngine,
    Message,
    Pusher,
    TTSEngine,
)
from .sentence_splitter import split_token_stream
from .utils.timing import StageTiming

log = logging.getLogger(__name__)

# 音频 chunk 类型（来自麦克风或喂给 TTS 的 PCM）
AudioStream = AsyncIterator[bytes]


@dataclass
class PipelineParts:
    """pipeline 的可插拔部件。便于测试注入 Mock。"""
    asr: ASREngine
    llm: LLMEngine
    tts: TTSEngine
    lipsync: LipSyncEngine
    pusher: Pusher
    portrait: bytes = b""            # 数字人形象图二进制
    session_id: str = "default"
    history: list[Message] = field(default_factory=list)
    on_timing: Callable[[StageTiming], None] | None = None


class DigitalHumanPipeline:
    """单次用户发言 → 数字人回复 的完整管线。

    一次 run() 处理一段完整的用户音频流。
    多轮对话由 session 层多次调用 run() 实现。
    """

    def __init__(self, cfg: Config, parts: PipelineParts):
        self.cfg = cfg
        self.parts = parts
        # 四条 Queue（背压控制，大小来自 config）
        self.text_q = asyncio.Queue(cfg.queue.text_queue)
        self.token_q = asyncio.Queue(cfg.queue.token_queue)
        self.sentence_q = asyncio.Queue(cfg.queue.sentence_queue)
        self.audio_q = asyncio.Queue(cfg.queue.audio_queue)
        # 延迟埋点
        self.timing = StageTiming()
        # 历史消息（每轮累积，给 LLM 做上下文）
        self._history = list(parts.history)
        # 本轮统计（汇总日志用）
        self._token_count = 0
        self._sentence_count = 0
        self._audio_bytes = 0
        self._frame_count = 0

    async def run(self, mic_stream: AudioStream) -> None:
        """处理一段麦克风音频流：ASR → ... → pusher。

        阻塞直到整段回复推完。ctx 取消会传播到所有 Task。
        ★ H3：总超时保护——单条请求超 120s（含长回复）视为卡死，取消释放并发槽位。
        """
        self.timing.mark_start()
        tasks = [
            asyncio.create_task(self._stage_asr(mic_stream), name="asr"),
            asyncio.create_task(self._stage_llm(), name="llm"),
            asyncio.create_task(self._stage_split(), name="split"),
            asyncio.create_task(self._stage_tts(), name="tts"),
            asyncio.create_task(self._stage_lipsync(), name="lipsync"),
        ]
        try:
            # ★ return_exceptions=True：避免一个 stage 异常打断其他 stage 的取消流程。
            # 每个 stage 的 finally 都会发哨兵，保证下游不永久阻塞（C2）。
            # ★ H3：总超时 120s——LLM sock_read 30s + 各阶段接力，正常回复远低于此；
            #   超时则取消所有 stage，释放 max_sessions 并发槽位，防网络抖动卡死服务。
            await asyncio.wait_for(
                asyncio.gather(*tasks, return_exceptions=True),
                timeout=120.0,
            )
        except asyncio.TimeoutError:
            log.warning("pipeline 总超时（120s），强制取消所有 stage session=%s",
                        self.parts.session_id)
        finally:
            # 确保所有 task 都结束（cancel 未完成的）
            for t in tasks:
                if not t.done():
                    t.cancel()
            # drain 取消的 task，避免"task was never awaited"警告
            for t in tasks:
                if not t.done():
                    try:
                        await t
                    except (asyncio.CancelledError, Exception):
                        pass
            self.timing.mark_end()
            if self.parts.on_timing:
                try:
                    self.parts.on_timing(self.timing)
                except Exception:
                    log.exception("on_timing 回调异常")

        # 检查结果：若有非 CancelledError 的异常，抛出第一个（保留诊断）
        for t, name in zip(tasks, ["asr", "llm", "split", "tts", "lipsync"]):
            if t.done() and not t.cancelled():
                exc = t.exception()
                if exc and not isinstance(exc, asyncio.CancelledError):
                    raise RuntimeError(f"stage {name} 失败: {exc}") from exc

        # ★ 单条汇总日志（诊断延迟/断链的关键）：各阶段耗时 + 吞吐
        st = self.timing.summary()
        log.info("回复管线完成 session=%s 总耗时=%.2fs 阶段延迟=%s 统计: token=%d 句=%d 音频=%.1fKB 帧=%d",
                 self.parts.session_id, st.get("reply_end", 0.0),
                 {k: round(v, 2) for k, v in st.items()},
                 getattr(self, "_token_count", 0), getattr(self, "_sentence_count", 0),
                 getattr(self, "_audio_bytes", 0) / 1024,
                 getattr(self, "_frame_count", 0))

    async def _safe_put(self, q: asyncio.Queue, item) -> None:
        """容错 put：Queue 满或 loop 关闭时不让 finally 抛异常打断清理（C2）。

        用于 finally 发哨兵——哨兵丢了也无妨（下游终会结束）。
        """
        try:
            q.put_nowait(item)
        except asyncio.QueueFull:
            pass

    async def _put_or_drop(self, q: asyncio.Queue, item, *, stage: str = "",
                          drop_counter: list | None = None) -> bool:
        """非阻塞 put：queue 满立即丢弃（用于 token/sentence，丢了无所谓）。"""
        try:
            q.put_nowait(item)
            return True
        except asyncio.QueueFull:
            if drop_counter is not None:
                drop_counter[0] += 1
            log.debug("queue 满，丢弃 item（stage=%s）", stage)
            return False

    async def _put_timed(self, q: asyncio.Queue, item, timeout: float = 0.05) -> None:
        """带超时的 put：让下游有机会消费（流式节奏），超时才丢。

        用于 audio（TTS→lipsync）：真实 lipsync 慢但没死时，短暂等待让帧渲染跟上，
        而非立即丢弃（之前的 put_nowait 会突发压满 + 误判退出）。
        下游真死了时，每个 chunk 最多等 timeout，不会永久阻塞。
        """
        try:
            await asyncio.wait_for(q.put(item), timeout=timeout)
        except asyncio.TimeoutError:
            # 下游消费不及，丢该 chunk（音频丢一点不影响整体，反正帧率有限）
            log.debug("audio put 超时 %.0fms，丢弃 1 chunk", timeout * 1000)

    # ---------- Stage 1: ASR ----------

    async def _stage_asr(self, mic_stream: AudioStream) -> None:
        """麦克风 PCM → ASR → 完整句文本。"""
        try:
            async for text in self.parts.asr.transcribe_stream(mic_stream):
                if text.strip():
                    # 把用户发言推给字幕
                    await self.parts.pusher.push_text(
                        self.parts.session_id, f"[user] {text}"
                    )
                    # ★ 非阻塞 put：下游 LLM 失败后 text_q 可能满，立即丢弃避免死锁
                    await self._put_or_drop(self.text_q, text, stage="asr")
        except Exception as e:
            log.exception("ASR stage error: %s", e)
            raise
        finally:
            # ASR 结束 = 用户说完，发哨兵让 LLM 阶段知道没有更多输入
            await self._safe_put(self.text_q, None)

    # ---------- Stage 2: LLM ----------

    async def _stage_llm(self) -> None:
        """完整句文本 → LLM 流式 token。"""
        try:
            while True:
                text = await self.text_q.get()
                if text is None:             # ASR 段结束哨兵
                    break
                self._history.append(Message(role="user", content=text))
                full_reply = []
                async for token in self.parts.llm.chat_stream(text, list(self._history)):
                    self.timing.mark_first_token()
                    self._token_count += 1
                    full_reply.append(token)
                    await self._put_or_drop(self.token_q, token, stage="llm")
                # 一句用户输入对应的 LLM 回复结束，记录历史
                self._history.append(Message(role="assistant", content="".join(full_reply)))
        except Exception as e:
            log.exception("LLM stage error: %s", e)
            raise
        finally:
            # ★ 无论正常/异常都发哨兵，避免 split stage 永久阻塞（C2）
            await self._safe_put(self.token_q, None)

    # ---------- Stage 2.5: 句切分 ----------

    async def _stage_split(self) -> None:
        """LLM token 流 → 短句。"""
        async def _token_gen():
            while True:
                tok = await self.token_q.get()
                if tok is None:
                    return
                yield tok

        try:
            async for sentence in split_token_stream(_token_gen(), self.cfg.sentence_splitter):
                self.timing.mark_first_sentence()
                self._sentence_count += 1
                await self._put_or_drop(self.sentence_q, sentence, stage="split")
        except Exception as e:
            log.exception("split stage error: %s", e)
            raise
        finally:
            await self._safe_put(self.sentence_q, None)

    # ---------- Stage 3: TTS ----------

    async def _stage_tts(self) -> None:
        """短句 → TTS → 音频片段流。

        ★ 单句容错（深度优化）：单句 TTS 失败（如 edge-tts 偶发 NoAudioReceived）
        不中断整个 pipeline，跳过该句的音频/唇形，继续下一句。
        字幕已推送，用户至少能看到文字。
        """
        try:
            while True:
                sentence = await self.sentence_q.get()
                if sentence is None:
                    break
                # P2-6：字幕推送改为 fire-and-forget，不阻塞首音频
                asyncio.create_task(self.parts.pusher.push_text(
                    self.parts.session_id, f"[assistant] {sentence}"
                ))
                # ★ per-sentence 容错：单句 TTS 失败跳过，不中断
                try:
                    async for audio in self.parts.tts.synthesize_stream(sentence):
                        self.timing.mark_first_audio()
                        self._audio_bytes += len(audio)
                        # ★ 带短超时的 put：让下游 lipsync 有机会消费（形成流式节奏），
                        # 超时（50ms）才丢该 chunk。避免"一次性 yield 60 chunk 压满"，
                        # 也避免"lipsync 死了 TTS 永远阻塞"。
                        await self._put_timed(self.audio_q, audio, timeout=0.05)
                except Exception as tts_err:
                    # 偶发 TTS 错误（edge-tts 抖动）——跳过这句的音频，继续下一句
                    log.warning("TTS 单句失败，跳过（不中断 pipeline）: %s", tts_err)
                    # #6：埋点 TTS 失败
                    try:
                        from .metrics import get_metrics
                        get_metrics().record_tts_fail()
                    except Exception:
                        pass
                    # m7：错误分级——TTS 单句失败是 warn 级（非致命）
                    await self.parts.pusher.push_text(
                        self.parts.session_id, "[error:warn] 该句语音合成失败，已跳过"
                    )
        except Exception as e:
            # 非单句级的致命错误（如 Queue 损坏）才中断
            log.exception("TTS stage 致命错误: %s", e)
            raise
        finally:
            await self._safe_put(self.audio_q, None)

    # ---------- Stage 4: 唇形 ----------

    async def _stage_lipsync(self) -> None:
        """音频片段 → 唇形 → JPEG 帧 → pusher。"""
        async def _audio_gen():
            while True:
                a = await self.audio_q.get()
                if a is None:
                    return
                yield a

        try:
            async for frame in self.parts.lipsync.render_stream(
                self.parts.portrait, _audio_gen()
            ):
                self.timing.mark_first_frame()
                self._frame_count += 1
                await self.parts.pusher.push_frame(self.parts.session_id, frame)
        except Exception as e:
            log.exception("lipsync stage error: %s", e)
            raise
