"""LLM token 流 → 短句切分器。

实时数字人的延迟生死线：不等 LLM 说完整句，遇标点或累积到阈值就立即喂 TTS。

策略：
- 维护一个 buffer，每来一个 token 追加
- 检测 buffer 尾部是否含切分标点（中英），或长度 ≥ max_chars
- 满足条件就把 buffer 作为一句 yield 出去，并清空

用法（async generator）：
    async for sentence in split_tokens(token_stream, cfg):
        ...

也提供纯函数 next_boundary(buf, punctuation, max_chars) 用于单元测试。
"""
from __future__ import annotations

from collections.abc import AsyncIterator

from .config import SentenceSplitterConfig

# 默认切分标点（中英）
DEFAULT_PUNCTUATION = "。！？!?,.;；、\n"
DEFAULT_MAX_CHARS = 15


def next_boundary(buf: str, punctuation: str, max_chars: int) -> int:
    """返回 buf 中下一个切分点（不含），返回 0 表示尚未到边界。

    切分点 = 第一个切分标点的"下一位置"，或 buf 长度（当超 max_chars 时）。
    返回值表示 buf[:n] 应作为一句切出。
    """
    if not buf:
        return 0
    # 1. 标点切分：找第一个切分字符的下一位置
    for i, ch in enumerate(buf):
        if ch in punctuation:
            return i + 1
    # 2. 长度切分：超阈值强制切到 max_chars 处（避免 LLM 长输出阻塞 TTS）
    if len(buf) >= max_chars:
        return max_chars
    return 0


def split_text(text: str, punctuation: str = DEFAULT_PUNCTUATION,
               max_chars: int = DEFAULT_MAX_CHARS) -> list[str]:
    """纯函数：把一段完整文本切成短句列表。用于测试与非流式场景。"""
    out: list[str] = []
    buf = text
    while buf:
        n = next_boundary(buf, punctuation, max_chars)
        if n == 0:
            # 剩余部分既无标点也未超长——作为最后一句
            out.append(buf)
            break
        out.append(buf[:n])
        buf = buf[n:]
    # 过滤纯空白句
    return [s for s in out if s.strip()]


async def split_token_stream(
    token_stream: AsyncIterator[str],
    cfg: SentenceSplitterConfig | None = None,
) -> AsyncIterator[str]:
    """流式：消费 token，产出短句。

    ★ P2：时间维度强制切分——LLM 回复"你好"（短、无标点）时，不等整句流完，
    超过 max_idle_ms 没新 token 即切（降短回复的首句延迟）。
    token 流结束时，buffer 有剩余作为最后一句 yield。
    """
    import asyncio

    punctuation = cfg.punctuation if cfg else DEFAULT_PUNCTUATION
    max_chars = cfg.max_chars if cfg else DEFAULT_MAX_CHARS
    max_idle_ms = 500  # 短回复快速切分阈值（可调）
    buf = ""
    last_token_time = None

    # 用 __anext__ 显式取，配合超时检测空闲
    while True:
        try:
            # 若 buffer 有内容且空闲超时，先切（不等下个 token）
            if buf and last_token_time is not None:
                idle_ms = (asyncio.get_running_loop().time() - last_token_time) * 1000
                timeout_left = max(0.001, (max_idle_ms - idle_ms) / 1000)
            else:
                timeout_left = None  # buffer 空时不切，无限等首个 token
            token = await asyncio.wait_for(token_stream.__anext__(), timeout=timeout_left)
            buf += token
            last_token_time = asyncio.get_running_loop().time()
        except asyncio.TimeoutError:
            # 空闲超时，buffer 有内容则切（P2 核心）
            if buf.strip():
                yield buf
                buf = ""
            last_token_time = None
            continue
        except StopAsyncIteration:
            break
        # 循环切：一个 token 可能让 buffer 凑齐多句
        while True:
            n = next_boundary(buf, punctuation, max_chars)
            if n == 0:
                break
            sentence = buf[:n]
            buf = buf[n:]
            if sentence.strip():
                yield sentence
            last_token_time = asyncio.get_running_loop().time()
    # 流结束，flush 剩余
    if buf.strip():
        yield buf
