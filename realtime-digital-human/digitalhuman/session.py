"""会话状态机：管理单客户端的多次往返。

一个 Session 对应一路浏览器 WS 连接。
- 持有 4 个引擎实例（或共享）
- 每次"用户说完一段话"触发一次 pipeline.run()
- 维护对话历史给 LLM 做上下文
- ctx 取消时清理所有正在跑的 Task（防泄漏，参照 auto-finance AnswerStream 范式）
"""
from __future__ import annotations

import asyncio
import logging
from dataclasses import dataclass, field
from typing import TYPE_CHECKING
from collections.abc import AsyncIterator

if TYPE_CHECKING:
    from .config import PersonaConfig
    from .store import SQLiteStore

from .config import Config
from .engines.base import ASREngine, LLMEngine, LipSyncEngine, Message, Pusher, TTSEngine
from .pipeline import DigitalHumanPipeline, PipelineParts

log = logging.getLogger(__name__)


class _FixedASR(ASREngine):
    """不产生任何文本的 ASR（用于欢迎语，绕过真实识别）。"""
    async def transcribe_stream(self, audio_stream):
        async for _ in audio_stream:
            pass
        return
        yield ""  # 让它成为 async generator


class _FixedLLM(LLMEngine):
    """固定输出的 LLM（用于欢迎语，绕过真实推理）。"""
    def __init__(self, text: str):
        self._text = text
    async def chat_stream(self, prompt, history=None):
        yield self._text
        return


@dataclass
class Session:
    """单会话状态。"""
    session_id: str
    cfg: Config
    asr: ASREngine
    llm: LLMEngine
    tts: TTSEngine
    lipsync: LipSyncEngine
    pusher: Pusher
    portrait: bytes = b""
    history: list[Message] = field(default_factory=list)
    # P0：history 最大保留消息数（含 user+assistant），超出按"留 system + 取最近 N"裁剪
    max_history_messages: int = 12  # 约 6 轮对话
    # #3：持久化存储（None=纯内存；SQLiteStore=持久化，重启不丢历史）
    store: SQLiteStore | None = None
    # 当前正在跑的 pipeline task（同一会话串行化，防并发抢占 GPU）
    _current: asyncio.Task | None = None
    # asyncio.Lock，惰性创建（不能用 field(default_factory=asyncio.Lock)，会在模块加载时建错 loop）
    _lock: object = None
    # 上一轮 pipeline 的延迟统计（ms），由 on_timing 回调写入
    _last_timing_ms: dict | None = None

    def __post_init__(self):
        if self._lock is None:
            self._lock = asyncio.Lock()

    def set_persona(self, persona: PersonaConfig | object) -> None:
        """#2/#4 热切换角色：更新 LLM system_prompt + TTS voice + portrait。

        在 _lock 内调用（确保不打断正在跑的 pipeline）。
        persona: PersonaConfig（或 duck-typed 对象，有 voice/system_prompt/portrait 属性）。
        """
        # 更新 LLM 人设（直接 setattr，引擎无该属性也动态加）
        if hasattr(persona, "system_prompt"):
            setattr(self.llm, "system_prompt", persona.system_prompt)
        # 更新 TTS 音色
        if hasattr(persona, "voice"):
            setattr(self.tts, "voice", persona.voice)
        # 更新形象（若有专用 portrait：bytes 直接用，字符串路径则读取文件）
        if hasattr(persona, "portrait") and persona.portrait:
            if isinstance(persona.portrait, bytes):
                self.portrait = persona.portrait
            elif isinstance(persona.portrait, str):
                # 字符串视为文件路径，读取成 bytes
                try:
                    import os
                    path = persona.portrait
                    if not os.path.isabs(path):
                        path = os.path.join(os.getcwd(), path)
                    if os.path.isfile(path):
                        with open(path, "rb") as f:
                            self.portrait = f.read()
                except Exception as e:
                    log.warning("persona portrait 读取失败: %s", e)
        log.info("session=%s 切换角色: voice=%s", self.session_id,
                 getattr(persona, "voice", "?"))

    async def handle_utterance(self, mic_stream: AsyncIterator[bytes]) -> None:
        """处理一段用户发言。

        串行化：同一会话同一时刻只跑一个 pipeline（单 GPU 防并发）。
        不同会话之间由 server 层调度。

        ★ 仅 pipeline 正常结束时才把 history 回写——避免 cancel/异常时半截
        assistant 回复污染下一轮 LLM 上下文（C3）。
        """
        async with self._lock:
            # M1：on_timing 回调把各阶段延迟推给前端（秒→毫秒）
            def _on_timing(timing):
                # timing.summary() 返回秒，转成 _ms 后缀的字段（与前端约定一致）
                # 仅记录，由 handle_utterance 的调用方在 END 前推送（保证顺序）
                self._last_timing_ms = {f"{k}_ms": int(v * 1000) for k, v in timing.summary().items()}
                # #6：埋点到全局 metrics
                try:
                    from .metrics import get_metrics
                    get_metrics().record_pipeline_complete(self._last_timing_ms)
                except Exception:
                    pass

            parts = PipelineParts(
                asr=self.asr,
                llm=self.llm,
                tts=self.tts,
                lipsync=self.lipsync,
                pusher=self.pusher,
                portrait=self.portrait,
                session_id=self.session_id,
                history=list(self.history),
                on_timing=_on_timing,
            )
            pipeline = DigitalHumanPipeline(self.cfg, parts)
            self._current = asyncio.current_task()
            self._last_timing_ms = None
            try:
                await pipeline.run(mic_stream)
                # ★ 对话摘要日志（诊断对话链路：输入→输出）
                user_texts = [m.content for m in pipeline._history if m.role == "user"]
                last_reply = (pipeline._history[-1].content
                              if pipeline._history and pipeline._history[-1].role == "assistant"
                              else "")
                log.info("对话轮完成 session=%s 用户=%s 回复=%s",
                         self.session_id,
                         (user_texts[-1][:40] if user_texts else "（空/ASR 未识别）"),
                         last_reply[:60])
                # 仅成功才回写 history（含本轮 user + assistant 完整消息）
                prev_len = len(self.history)
                self.history = pipeline._history
                # #3：持久化本轮新增的消息（user + assistant）
                if self.store is not None:
                    new_msgs = self.history[prev_len:] if prev_len < len(self.history) else []
                    if new_msgs:
                        try:
                            await self.store.append_messages(self.session_id, new_msgs)
                        except Exception as e:
                            log.warning("history 持久化失败（不影响对话）: %s", e)
                # ★ 长期记忆（创新）：history 超阈值时，把最旧的消息压成摘要而非直接丢弃
                # 解决长对话失忆（用户姓名/偏好丢失）+ 控制上下文窗口（防推理线性变慢）
                await self._maybe_compress_history()

                # M1：pipeline 成功后，在 server 推 END 之前推 LATENCY（保证前端先收到延迟统计）
                if self._last_timing_ms:
                    try:
                        await self.pusher.push_latency(self.session_id, self._last_timing_ms)
                    except Exception as e:
                        log.debug("push_latency 失败: %s", e)
            except asyncio.CancelledError:
                # 被取消时不回写脏 history，直接抛
                raise
            except Exception:
                # 其他异常也不回写（避免半截 reply 污染），但记录日志
                log.exception("pipeline 异常，history 未回写")
                raise
            finally:
                self._current = None

    async def _maybe_compress_history(self) -> None:
        """★ 创新：长期记忆压缩——超阈值时把最旧消息压成摘要，保留长期记忆。

        策略（LangChain/MemGPT 经典做法）：
        - history 未超 summary_threshold → 不动
        - 超阈值 → 取最旧的（除 system 外）消息调 LLM 压成一条摘要，
          保留最近 keep_recent_pairs 轮原文 + 摘要消息 + system
        - 压缩失败（LLM 不可用/mock/超时）→ 退回旧的"直接丢弃"策略，不影响对话

        效果：用户 10 轮前说的"我叫张三、喜欢科幻"仍被记住，且上下文 token 数恒定。
        """
        mem = getattr(self.cfg, "memory", None)
        # 未配置 memory 或未启用 → 退回旧逻辑（直接裁剪到 max_history_messages）
        if mem is None or not mem.enabled:
            self._truncate_history_legacy()
            return

        # 统计对话消息（排除 system/摘要）
        dialog = [m for m in self.history if m.role in ("user", "assistant")]
        if len(dialog) <= mem.summary_threshold:
            return  # 未超阈值，无需压缩

        keep_count = mem.keep_recent_pairs * 2  # user+assistant 对
        # 要压缩的旧消息（dialog 前段，排除保留的近期对话）
        to_summarize = dialog[:-keep_count] if keep_count < len(dialog) else []
        if not to_summarize:
            return

        # 收集旧摘要（压缩时并入新摘要，避免摘要无限累积导致上下文反向膨胀）
        old_summaries = [m.content for m in self.history
                         if m.role == "system" and "对话摘要" in m.content]
        # 调 LLM 压缩（旧摘要 + 旧对话 → 一条新摘要，合并而非追加）
        summary_text = await self._summarize_messages(to_summarize, old_summaries)

        # 重建 history：system 人设(若有) + 新摘要（唯一，合并了旧的） + 近期原文
        new_history: list[Message] = []
        # 保留首个 system 消息（人设）——但跳过摘要消息（摘要已并入新摘要）
        if (self.history and self.history[0].role == "system"
                and "对话摘要" not in self.history[0].content):
            new_history.append(self.history[0])
        # 新摘要（★ M-6：不再保留旧摘要，它们已并入新摘要，防累积膨胀）
        if summary_text:
            new_history.append(Message(
                role="system",
                content=f"[对话摘要] {summary_text}",
            ))
        # 近期原文
        new_history.extend(dialog[-keep_count:] if keep_count < len(dialog) else dialog)

        old_len = len(self.history)
        self.history = new_history
        log.info("记忆压缩 session=%s: %d 条 → %d 条（压缩 %d 条旧消息 + %d 条旧摘要 → 1 条新摘要）",
                 self.session_id, old_len, len(new_history), len(to_summarize), len(old_summaries))

        # ★ 摘要持久化：把新摘要存进 SQLite，重启后 load_history 能恢复长期记忆
        # （否则压缩只在内存，重启后摘要丢失 = 压缩白做）
        if summary_text and self.store is not None:
            try:
                await self.store.append_message(
                    self.session_id, "system", f"[对话摘要] {summary_text}")
            except Exception as e:
                log.debug("摘要持久化失败（不影响对话）: %s", e)

    def _truncate_history_legacy(self) -> None:
        """旧的直接裁剪逻辑（memory 禁用或压缩失败时兜底）。"""
        if len(self.history) <= self.max_history_messages:
            return
        kept = []
        if self.history and self.history[0].role == "system":
            kept.append(self.history[0])
        kept.extend(self.history[-self.max_history_messages:])
        self.history = kept

    async def _summarize_messages(self, messages: list[Message],
                                  old_summaries: list[str] | None = None) -> str:
        """调 LLM 把消息列表（+ 旧摘要）压成一条新摘要。失败返回空串（不影响对话）。

        ★ M-6：old_summaries 并入 prompt，让新摘要包含旧摘要的关键信息，
          避免多次压缩产生多条摘要导致上下文反向膨胀。
        """
        mem = self.cfg.memory
        parts = []
        # 旧摘要（之前的长期记忆）并入，让 LLM 合并进新摘要
        if old_summaries:
            parts.append("--- 已有摘要（请合并进新摘要）---\n" + "\n".join(old_summaries))
        # 要压缩的对话文本
        dialog_text = "\n".join(
            f"{'用户' if m.role == 'user' else '助手'}: {m.content}" for m in messages
        )
        parts.append(f"--- 对话历史 ---\n{dialog_text}\n--- 结束 ---")
        prompt = f"{mem.summary_prompt}\n\n" + "\n\n".join(parts)

        try:
            tokens = []
            # 复用会话的 LLM 引擎（带人设 + 历史上下文），但传空 history 避免递归
            async for tok in self.llm.chat_stream(prompt, []):
                tokens.append(tok)
                if sum(len(t) for t in tokens) > 200:  # 摘要不宜过长，截断
                    break
            return "".join(tokens).strip()
        except Exception as e:
            log.warning("记忆摘要失败，退回直接裁剪: %s", e)
            self._truncate_history_legacy()
            return ""

    async def greet(self, greeting: str | None = None) -> None:
        """#5：主动开口欢迎语（WS 连接后调用）。

        跑一次 mini-pipeline（跳过 ASR/LLM，直接 TTS+LipSync）。
        greeting 为空则不发。
        """
        if not greeting or not greeting.strip():
            return
        # 用一个只含 assistant 消息的 pipeline（绕过 ASR）
        parts = PipelineParts(
            asr=_FixedASR([]),  # 不触发 LLM
            llm=_FixedLLM(greeting),  # 固定输出 greeting
            tts=self.tts,
            lipsync=self.lipsync,
            pusher=self.pusher,
            portrait=self.portrait,
            session_id=self.session_id,
        )
        pipeline = DigitalHumanPipeline(self.cfg, parts)
        try:
            async def empty_mic():
                return
                yield  # 空 async generator
            await asyncio.wait_for(pipeline.run(empty_mic()), timeout=15.0)
        except Exception as e:
            log.warning("欢迎语播放失败（不影响后续）: %s", e)

    async def cancel(self) -> None:
        """取消当前正在跑的 pipeline（客户端断开时调用）。

        ★ 只捕 CancelledError——其他异常应向上传播，不吞真实 BUG（C3）。
        """
        if self._current is not None and not self._current.done():
            # #6：埋点 cancel
            try:
                from .metrics import get_metrics
                get_metrics().record_pipeline_cancel()
            except Exception:
                pass
            self._current.cancel()
            try:
                await self._current
            except asyncio.CancelledError:
                pass  # 预期的取消
            # 其他异常不在这里吞，让调用方决定（server 的 finally 会处理）


class SessionRegistry:
    """session_id → Session 的注册表。server 层用。

    P1：max_sessions 并发上限（防单 4060 多 session OOM）。
    """

    def __init__(self, max_sessions: int = 3) -> None:
        self._sessions: dict[str, Session] = {}
        self.max_sessions = max_sessions

    def add(self, s: Session) -> bool:
        """加入 session。超限时返回 False（调用方应拒绝 WS）。"""
        if len(self._sessions) >= self.max_sessions:
            return False
        self._sessions[s.session_id] = s
        return True

    def is_full(self) -> bool:
        return len(self._sessions) >= self.max_sessions

    def get(self, session_id: str) -> Session | None:
        return self._sessions.get(session_id)

    def remove(self, session_id: str) -> Session | None:
        return self._sessions.pop(session_id, None)

    def __len__(self) -> int:
        return len(self._sessions)

    def all(self) -> list[Session]:
        return list(self._sessions.values())
