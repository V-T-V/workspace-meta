"""WSConnection 单元测试（从 server.ws_endpoint 拆出后可测试）。

验证 WSConnection 的核心逻辑：鉴权、限流、消息分发、音频缓冲、清理。
"""
import asyncio
import pytest

from digitalhuman.config import Config
from digitalhuman.ws_connection import WSConnection
from digitalhuman.session import SessionRegistry


class MockWebSocket:
    """模拟 FastAPI WebSocket（足够测试 WSConnection）。"""
    def __init__(self, accepted=True):
        self._accepted = False
        self._closed = None
        self._sent = []
        self._query_params = {}
        self._messages = []  # 待接收的消息
        self._disconnected = False

    @property
    def query_params(self):
        return self._query_params

    async def accept(self):
        self._accepted = True

    async def close(self, code=1000, reason=""):
        self._closed = (code, reason)

    async def send_bytes(self, data):
        self._sent.append(data)

    async def receive(self):
        if self._disconnected:
            return {"type": "websocket.disconnect"}
        if self._messages:
            msg = self._messages.pop(0)
            if msg == "disconnect":
                self._disconnected = True
                return {"type": "websocket.disconnect"}
            return msg
        # 默认阻塞（模拟等消息）
        await asyncio.sleep(0.1)
        return {"type": "websocket.disconnect"}


def _make_conn(ws=None, cfg=None, registry=None):
    """构造 WSConnection（用 Mock 引擎）。"""
    from digitalhuman.engines.mock import MockASR, MockLLM, MockTTS, MockLipSync
    from digitalhuman.pusher.base import DisabledPusher
    cfg = cfg or Config()
    ws = ws or MockWebSocket()
    registry = registry or SessionRegistry(max_sessions=3)
    conn = WSConnection(
        ws=ws, session_id="test", cfg=cfg,
        asr=MockASR([]), llm=MockLLM("回复"), tts=MockTTS(delay=0.001),
        lipsync=MockLipSync(delay=0.001), pusher=DisabledPusher(),
        portrait=b"", registry=registry, store=None,
    )
    return conn, ws


@pytest.mark.asyncio
async def test_check_access_rejects_bad_token():
    """鉴权：错误 token 应拒绝（close 1008）。"""
    cfg = Config()
    cfg.server.auth_token = "secret"
    ws = MockWebSocket()
    ws._query_params = {"token": "wrong"}
    conn, ws = _make_conn(ws, cfg)
    ok = await conn._check_access()
    assert not ok
    assert ws._closed is not None
    assert ws._closed[0] == 1008


@pytest.mark.asyncio
async def test_check_access_accepts_correct_token():
    """鉴权：正确 token 应通过。"""
    cfg = Config()
    cfg.server.auth_token = "secret"
    ws = MockWebSocket()
    ws._query_params = {"token": "secret"}
    conn, ws = _make_conn(ws, cfg)
    ok = await conn._check_access()
    assert ok
    assert ws._accepted


@pytest.mark.asyncio
async def test_check_access_rejects_when_full():
    """限流：registry 满 should 拒绝（close 1013）。"""
    registry = SessionRegistry(max_sessions=1)
    # 占满
    from digitalhuman.engines.base import Pusher
    from digitalhuman.session import Session
    cfg = Config()
    sess = Session(session_id="other", cfg=cfg, asr=None, llm=None, tts=None,
                   lipsync=None, pusher=None)
    registry.add(sess)
    assert registry.is_full()

    ws = MockWebSocket()
    conn, ws = _make_conn(ws, cfg, registry)
    ok = await conn._check_access()
    assert not ok
    assert ws._closed[0] == 1013


@pytest.mark.asyncio
async def test_handle_audio_buffers_chunks():
    """音频消息应累积到 audio_buffer。"""
    conn, _ = _make_conn()
    conn._bind_pusher()  # 不绑也不影响 audio
    conn.sess = None  # 跳过 barge_in
    # 模拟 _handle_audio（无 sess 时不触发 barge-in）
    from digitalhuman.engines.mock import MockASR, MockLLM, MockTTS, MockLipSync
    from digitalhuman.engines.base import Pusher
    from digitalhuman.pusher.base import DisabledPusher
    from digitalhuman.session import Session
    conn.sess = Session(session_id="t", cfg=conn.cfg, asr=MockASR([]),
                        llm=MockLLM("r"), tts=MockTTS(), lipsync=MockLipSync(),
                        pusher=DisabledPusher())

    await conn._handle_audio(b"\x00" * 100)
    assert len(conn.audio_buffer) == 1
    assert conn.audio_buffer_total == 100

    await conn._handle_audio(b"\x00" * 200)
    assert len(conn.audio_buffer) == 2
    assert conn.audio_buffer_total == 300


@pytest.mark.asyncio
async def test_flush_utterance_snapshots_and_clears():
    """flush 应快照音频 + 清空 buffer + 入队。"""
    conn, _ = _make_conn()
    conn.audio_buffer = [b"\x01", b"\x02"]
    conn.audio_buffer_total = 2
    await conn._flush_utterance()
    assert conn.audio_buffer == []
    assert conn.audio_buffer_total == 0
    # 队列应有 1 项
    chunks = conn.utterance_queue.get_nowait()
    assert chunks == [b"\x01", b"\x02"]


@pytest.mark.asyncio
async def test_flush_empty_buffer_noop():
    """空 buffer flush 不应入队。"""
    conn, _ = _make_conn()
    await conn._flush_utterance()
    assert conn.utterance_queue.empty()


@pytest.mark.asyncio
async def test_cleanup_removes_from_registry():
    """cleanup 应从 registry 移除 session。"""
    conn, ws = _make_conn()
    conn._setup_session()
    assert len(conn.registry) == 1
    await conn._cleanup()
    assert len(conn.registry) == 0


@pytest.mark.asyncio
async def test_handle_set_persona():
    """set_persona 消息应触发角色切换。"""
    from digitalhuman.config import PersonaConfig, PersonasConfig
    cfg = Config()
    cfg.personas = PersonasConfig(list=[
        PersonaConfig(id="a", name="A", voice="voice-a"),
    ])
    conn, _ = _make_conn(cfg=cfg)
    conn._setup_session()
    conn._handle_set_persona(b"a")
    assert conn.sess.tts.voice == "voice-a"


def test_handle_set_tts_rate():
    """set_tts_rate 消息应调整语速。"""
    conn, _ = _make_conn()
    conn._setup_session()
    conn._handle_set_tts_rate(b"+20%")
    assert hasattr(conn.sess.tts, "rate")
    assert conn.sess.tts.rate == "+20%"

    conn._handle_set_tts_rate(b"-10%")
    assert conn.sess.tts.rate == "-10%"


@pytest.mark.asyncio
async def test_handle_text_input():
    """文字输入应触发 pipeline（绕过 ASR，直接注入文字）。"""
    conn, ws = _make_conn()
    conn._bind_pusher()
    conn._setup_session()
    # 发送文字输入
    await conn._handle_text_input("你好呀".encode("utf-8"))
    # 应执行完 pipeline（MockLLM 会回复）
    # history 应含 user + assistant
    await asyncio.sleep(0.5)  # 等 pipeline 完成
    assert len(conn.sess.history) >= 2
    assert conn.sess.history[-2].role == "user"
    assert conn.sess.history[-2].content == "你好呀"


@pytest.mark.asyncio
async def test_handle_text_input_empty_ignored():
    """空文字输入应被忽略。"""
    conn, _ = _make_conn()
    conn._setup_session()
    history_len_before = len(conn.sess.history)
    await conn._handle_text_input(b"   ")  # 空白
    assert len(conn.sess.history) == history_len_before


@pytest.mark.asyncio
async def test_handle_clear_history():
    """清空对话应清空 session history。"""
    from digitalhuman.engines.base import Message
    conn, _ = _make_conn()
    conn._setup_session()
    # 先填充 history
    conn.sess.history = [
        Message(role="user", content="旧对话1"),
        Message(role="assistant", content="旧回复1"),
    ]
    await conn._handle_clear_history(b"")
    assert len(conn.sess.history) == 0
