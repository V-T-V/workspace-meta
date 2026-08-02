"""引擎抽象与 Pusher 测试。"""
import asyncio

import pytest

from digitalhuman.engines.base import (
    ASREngine,
    LLMEngine,
    LipSyncEngine,
    Message,
    Pusher,
    TTSEngine,
)
from digitalhuman.pusher import build_pusher


def test_message_dataclass():
    m = Message(role="user", content="hello")
    assert m.role == "user"
    assert m.content == "hello"


def test_engines_are_abstract():
    for cls in (ASREngine, LLMEngine, TTSEngine, LipSyncEngine, Pusher):
        with pytest.raises(TypeError):
            cls()  # type: ignore[abstract]


def test_build_pusher_ws_mjpeg():
    p = build_pusher("ws_mjpeg")
    assert isinstance(p, Pusher)
    # 默认无连接，push 不抛错
    import asyncio
    asyncio.run(p.push_frame("s1", b"jpeg"))
    asyncio.run(p.push_audio("s1", b"pcm"))
    asyncio.run(p.push_text("s1", "hi"))


def test_build_pusher_disabled():
    p = build_pusher("disabled")
    assert isinstance(p, Pusher)
    import asyncio
    asyncio.run(p.push_frame("s1", b"jpeg"))


def test_build_pusher_unknown():
    with pytest.raises(ValueError):
        build_pusher("nonexistent")


@pytest.mark.asyncio
async def test_wsmjpeg_pusher_bind_send():
    """绑定连接后，push_frame 能把数据送到 send 回调。"""
    from digitalhuman.pusher.base import WSMjpegPusher
    from digitalhuman import frames

    received: list[bytes] = []

    async def fake_send(data: bytes):
        received.append(data)

    p = WSMjpegPusher()
    p.bind("sess1", fake_send)
    await p.push_frame("sess1", b"\xff\xd8fakejpeg")
    await p.push_text("sess1", "你好")
    # ★ P1-2：push_frame 入队即返回，后台 sender task 需一个 tick 才发送
    await asyncio.sleep(0.05)

    assert len(received) == 2
    # ★ P1-2：frame 走队列（异步），text 直发，顺序不保证——按类型查找
    by_type = {}
    for raw in received:
        t, payload = frames.ws_unpack(raw)
        by_type[t] = payload
    assert frames.WS_MSG_FRAME in by_type, f"应收到帧消息，实际类型: {list(by_type)}"
    assert by_type[frames.WS_MSG_FRAME] == b"\xff\xd8fakejpeg"
    assert frames.WS_MSG_TEXT in by_type, f"应收到文本消息，实际类型: {list(by_type)}"
    assert by_type[frames.WS_MSG_TEXT].decode() == "你好"

    p.unbind("sess1")
    await p.push_frame("sess1", b"ignored")  # 解绑后再 push 不报错
    await asyncio.sleep(0.05)
    assert len(received) == 2
