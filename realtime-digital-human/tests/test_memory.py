"""长期记忆（摘要压缩）测试。

验证：超阈值时旧消息被 LLM 压成摘要、近期原文保留、system 人设保留；
未超阈值不动；memory 禁用退回旧裁剪；摘要失败兜底。
"""
import asyncio

import pytest

from digitalhuman.config import Config, MemoryConfig
from digitalhuman.engines.base import Message
from digitalhuman.engines.mock import MockLLM
from digitalhuman.session import Session


def _make_session(history: list[Message], cfg: Config = None,
                  llm: MockLLM = None) -> Session:
    """构造测试 Session（最小依赖，跳过真实引擎装配）。"""
    cfg = cfg or Config()
    s = Session(
        session_id="test", cfg=cfg,
        asr=None, llm=llm or MockLLM("(摘要)用户叫张三，喜欢科幻。"),
        tts=None, lipsync=None, pusher=None,
        history=list(history),
    )
    return s


def _dialog(n_pairs: int, user_prefix: str = "用户", asst_prefix: str = "助手") -> list[Message]:
    """生成 n 轮 user+assistant 对话消息。"""
    msgs = []
    for i in range(n_pairs):
        msgs.append(Message(role="user", content=f"{user_prefix}{i}说句话"))
        msgs.append(Message(role="assistant", content=f"{asst_prefix}{i}回复"))
    return msgs


@pytest.mark.asyncio
async def test_compress_triggers_when_exceed_threshold():
    """超阈值时触发压缩：旧消息→摘要，近期原文保留。"""
    cfg = Config()
    cfg.memory = MemoryConfig(enabled=True, summary_threshold=6, keep_recent_pairs=2)
    # 8 轮 = 16 条对话消息 > 阈值 6；保留最近 2 轮 = 4 条
    history = [Message(role="system", content="你是助手")] + _dialog(8)
    s = _make_session(history, cfg)

    await s._maybe_compress_history()

    # 应产生摘要消息
    summaries = [m for m in s.history if m.role == "system" and "对话摘要" in m.content]
    assert len(summaries) >= 1, "应有摘要消息"
    assert "张三" in summaries[-1].content  # MockLLM 的固定摘要

    # 近期 2 轮原文保留（最后 4 条是 user/assistant 原文，索引 6、7）
    recent = s.history[-4:]
    assert all(m.role in ("user", "assistant") for m in recent)
    assert "用户6" in recent[0].content  # 第 7 轮（索引 6）的原文保留

    # system 人设保留
    assert any(m.role == "system" and m.content == "你是助手" for m in s.history)


@pytest.mark.asyncio
async def test_no_compress_when_below_threshold():
    """未超阈值时不压缩（history 不变）。"""
    cfg = Config()
    cfg.memory = MemoryConfig(enabled=True, summary_threshold=20, keep_recent_pairs=2)
    history = [Message(role="system", content="你是助手")] + _dialog(3)  # 6 条 < 20
    s = _make_session(history, cfg)
    before = list(s.history)

    await s._maybe_compress_history()

    assert s.history == before  # 完全没动


@pytest.mark.asyncio
async def test_memory_disabled_falls_back_to_truncate():
    """memory 禁用时退回旧的直接裁剪逻辑。"""
    cfg = Config()
    cfg.memory = MemoryConfig(enabled=False)
    s = _make_session(_dialog(10), cfg)  # 20 条 > max_history_messages(12)
    s.max_history_messages = 4

    await s._maybe_compress_history()

    # 退回裁剪：只保留最近 4 条
    assert len(s.history) == 4
    # 无摘要消息
    assert not any("对话摘要" in m.content for m in s.history)


@pytest.mark.asyncio
async def test_summarize_failure_falls_back_safely():
    """LLM 摘要失败时不崩，退回直接裁剪。"""
    cfg = Config()
    cfg.memory = MemoryConfig(enabled=True, summary_threshold=4, keep_recent_pairs=1)

    class BrokenLLM(MockLLM):
        async def chat_stream(self, prompt, history=None):
            raise RuntimeError("LLM 挂了")
            yield  # 让它是 async generator（raise 后不可达，但满足类型）

    history = _dialog(5)  # 10 条 > 阈值 4
    s = _make_session(history, cfg, llm=BrokenLLM("x"))
    s.max_history_messages = 4

    await s._maybe_compress_history()

    # 退回裁剪，不崩
    assert len(s.history) <= s.max_history_messages + 1  # +1 容 system（本例无）


@pytest.mark.asyncio
async def test_summary_accumulates_with_previous():
    """多次压缩累积：已有摘要 + 新摘要共存（长期记忆累积）。"""
    cfg = Config()
    cfg.memory = MemoryConfig(enabled=True, summary_threshold=4, keep_recent_pairs=1)
    # 已有一条旧摘要 + 6 条新对话（> 阈值 4）
    history = [
        Message(role="system", content="[对话摘要] 用户叫李四"),
    ] + _dialog(3)
    s = _make_session(history, cfg)

    await s._maybe_compress_history()

    # 旧摘要 + 新摘要都在
    summaries = [m for m in s.history if "对话摘要" in m.content]
    assert len(summaries) >= 2  # 旧的 + 新的
    assert any("李四" in m.content for m in summaries)  # 旧摘要保留
