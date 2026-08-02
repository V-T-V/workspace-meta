"""帧编码协议测试。"""
import asyncio
import struct

import pytest

from digitalhuman import frames


def test_encode_frame_basic():
    payload = b"\xff\xd8\xff\xe0" + b"jpegdata"  # 假 JPEG 头
    enc = frames.encode_frame(payload)
    assert len(enc) == frames.HEADER_SIZE + len(payload)
    (n,) = struct.unpack(">I", enc[:4])
    assert n == len(payload)
    assert enc[4:] == payload


def test_encode_eof_and_segment_end():
    eof = frames.encode_eof()
    (n,) = struct.unpack(">I", eof)
    assert n == frames.EOF_MARKER

    seg = frames.encode_segment_end()
    (n,) = struct.unpack(">I", seg)
    assert n == frames.SEGMENT_END


def test_encode_frame_too_large():
    # 不真构造 4GB（MemoryError），只验证边界判断逻辑
    # EOF_MARKER = 0xFFFFFFFF = 4294967295，encode_frame 检查 len >= EOF_MARKER
    # 用 mock 验证函数对超大输入抛 ValueError
    import unittest.mock
    with unittest.mock.patch.object(frames, 'EOF_MARKER', 100):
        big = b"x" * 100
        with pytest.raises(ValueError):
            frames.encode_frame(big)


def test_ws_pack_unpack():
    msg = frames.ws_pack(frames.WS_MSG_FRAME, b"\x01\x02\x03")
    t, p = frames.ws_unpack(msg)
    assert t == frames.WS_MSG_FRAME
    assert p == b"\x01\x02\x03"

    txt = frames.ws_pack_text("你好")
    t, p = frames.ws_unpack(txt)
    assert t == frames.WS_MSG_TEXT
    assert p.decode() == "你好"


async def _pipe_frames():
    """构造一个 StreamReader，写入 3 帧 + EOF。"""
    r = asyncio.StreamReader()
    r.feed_data(frames.encode_frame(b"frame1"))
    r.feed_data(frames.encode_frame(b"frame2"))
    r.feed_data(frames.encode_segment_end())
    r.feed_data(frames.encode_frame(b"frame3"))
    r.feed_data(frames.encode_eof())
    r.feed_eof()
    return r


@pytest.mark.asyncio
async def test_read_frame():
    r = await _pipe_frames()
    assert await frames.read_frame(r) == b"frame1"
    assert await frames.read_frame(r) == b"frame2"
    assert await frames.read_frame(r) == b""        # 段结束
    assert await frames.read_frame(r) == b"frame3"
    assert await frames.read_frame(r) is None       # EOF


@pytest.mark.asyncio
async def test_iter_frames():
    r = await _pipe_frames()
    out = []
    async for f in frames.iter_frames(r):
        out.append(f)
    assert out == [b"frame1", b"frame2", b"", b"frame3"]


@pytest.mark.asyncio
async def test_iter_frames_handles_incomplete_read():
    """★ C4：subprocess 崩溃时 stdout 断流，iter_frames 应捕获 IncompleteReadError
    并正常结束，而非让异常冒泡。
    """
    r = asyncio.StreamReader()
    # 发一帧完整数据
    r.feed_data(frames.encode_frame(b"good_frame"))
    # 发不完整的头（只有 2 字节，不够 4 字节头）然后 EOF
    r.feed_data(b"\x00\x00")
    r.feed_eof()

    out = []
    # 不应抛异常
    async for f in frames.iter_frames(r):
        out.append(f)
    # 应能读到第一帧，然后正常结束
    assert out == [b"good_frame"]


@pytest.mark.asyncio
async def test_iter_frames_empty_stream():
    """空流（立即 EOF）应正常结束，不抛异常。"""
    r = asyncio.StreamReader()
    r.feed_eof()
    out = []
    async for f in frames.iter_frames(r):
        out.append(f)
    assert out == []
