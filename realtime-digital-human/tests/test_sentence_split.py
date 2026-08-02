"""句切分器测试。"""
import pytest

from digitalhuman.config import SentenceSplitterConfig
from digitalhuman.sentence_splitter import (
    DEFAULT_MAX_CHARS,
    DEFAULT_PUNCTUATION,
    next_boundary,
    split_text,
    split_token_stream,
)


def test_next_boundary_empty():
    assert next_boundary("", DEFAULT_PUNCTUATION, DEFAULT_MAX_CHARS) == 0


def test_next_boundary_no_punct_under_limit():
    assert next_boundary("你好呀", DEFAULT_PUNCTUATION, 15) == 0


def test_next_boundary_chinese_punct():
    # "你好。" → 切到 。后一位
    assert next_boundary("你好。", DEFAULT_PUNCTUATION, 15) == 3


def test_next_boundary_english_punct():
    assert next_boundary("hi!", DEFAULT_PUNCTUATION, 15) == 3
    assert next_boundary("a,b", DEFAULT_PUNCTUATION, 15) == 2


def test_next_boundary_max_chars():
    # 无标点但超阈值 → 强制切到 max_chars 处（不是整个长度）
    assert next_boundary("abcdefghijklmnop", DEFAULT_PUNCTUATION, 5) == 5
    # 恰好等于阈值也切
    assert next_boundary("abcde", DEFAULT_PUNCTUATION, 5) == 5
    # 未到阈值不切
    assert next_boundary("abcd", DEFAULT_PUNCTUATION, 5) == 0


def test_split_text_with_punct():
    out = split_text("你好。很高兴认识你！")
    assert out == ["你好。", "很高兴认识你！"]


def test_split_text_max_chars():
    # 18 字无标点，max_chars=5 → 应切成 4 段（5+5+5+3）
    out = split_text("abcdefghijklmnopqr", max_chars=5)
    assert len(out) == 4
    assert out[0] == "abcde"
    assert out[-1] == "pqr"


def test_split_text_filters_whitespace():
    out = split_text("你好。   。世界")
    # 中间纯空白的句子被过滤
    assert all(s.strip() for s in out)
    assert "你好。" in out


def test_split_text_empty():
    assert split_text("") == []
    assert split_text("   ") == []


def test_split_text_multi_punct_in_one_token():
    """一个 token 含多个标点应切多句。"""
    out = split_text("你好。再见！")
    assert out == ["你好。", "再见！"]


# ---------- 流式版本 ----------

async def _token_gen(tokens):
    for t in tokens:
        yield t


@pytest.mark.asyncio
async def test_split_token_stream_basic():
    """模拟 LLM 逐字吐 token，验证切分。"""
    tokens = list("你好。很高兴认识你！")
    out = []
    async for s in split_token_stream(_token_gen(tokens)):
        out.append(s)
    assert out == ["你好。", "很高兴认识你！"]


@pytest.mark.asyncio
async def test_split_token_stream_token_with_multi_punct():
    """一个 token 包含完整多句。"""
    out = []
    async for s in split_token_stream(_token_gen(["你好。", "再见！"])):
        out.append(s)
    assert out == ["你好。", "再见！"]


@pytest.mark.asyncio
async def test_split_token_stream_max_chars():
    """token 流无标点但超 max_chars，应强制切。"""
    cfg = SentenceSplitterConfig(punctuation="。", max_chars=5)
    out = []
    async for s in split_token_stream(_token_gen(list("abcdefghijklmnop")), cfg):
        out.append(s)
    # 每 5 字一段
    assert out == ["abcde", "fghij", "klmno", "p"]


@pytest.mark.asyncio
async def test_split_token_stream_flush_on_end():
    """流结束时 buffer 有残留应 flush。"""
    out = []
    async for s in split_token_stream(_token_gen(list("你好呀"))):
        out.append(s)
    # 无标点，结束时整体 flush
    assert out == ["你好呀"]
