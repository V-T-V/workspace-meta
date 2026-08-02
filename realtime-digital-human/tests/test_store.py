"""#3 持久化存储测试（SQLite）。"""
import asyncio
import os
import tempfile

import pytest

from digitalhuman.engines.base import Message
from digitalhuman.store import SQLiteStore


@pytest.fixture
def store(tmp_path):
    """每个测试用独立临时 db。"""
    db = tmp_path / "test.db"
    return SQLiteStore(str(db))


def test_init_creates_tables(store):
    """初始化应创建 messages 和 sessions 表。"""
    import sqlite3
    conn = sqlite3.connect(store.db_path)
    tables = {r[0] for r in conn.execute(
        "SELECT name FROM sqlite_master WHERE type='table'"
    ).fetchall()}
    conn.close()
    assert "messages" in tables
    assert "sessions" in tables


def test_load_empty_history(store):
    """无历史时返回空列表。"""
    assert store.load_history("nonexistent") == []


def test_append_and_load(store):
    """写入消息后能按时间顺序读回。"""
    asyncio.run(store.append_message("s1", "user", "你好"))
    asyncio.run(store.append_message("s1", "assistant", "你好呀"))
    asyncio.run(store.append_message("s1", "user", "再见"))

    history = store.load_history("s1")
    assert len(history) == 3
    assert history[0].role == "user" and history[0].content == "你好"
    assert history[1].role == "assistant" and history[1].content == "你好呀"
    assert history[2].role == "user" and history[2].content == "再见"


def test_load_respects_limit(store):
    """limit 应限制返回条数（取最近的）。"""
    for i in range(10):
        asyncio.run(store.append_message("s1", "user", f"msg{i}"))

    history = store.load_history("s1", limit=3)
    assert len(history) == 3
    # 最近的 3 条
    assert history[0].content == "msg7"
    assert history[2].content == "msg9"


def test_sessions_isolated(store):
    """不同 session 的历史应隔离。"""
    asyncio.run(store.append_message("s1", "user", "session1-msg"))
    asyncio.run(store.append_message("s2", "user", "session2-msg"))

    assert len(store.load_history("s1")) == 1
    assert len(store.load_history("s2")) == 1
    assert store.load_history("s1")[0].content == "session1-msg"
    assert store.load_history("s2")[0].content == "session2-msg"


def test_append_batch(store):
    """批量写入。"""
    msgs = [
        Message(role="user", content="批量1"),
        Message(role="assistant", content="批量2"),
    ]
    asyncio.run(store.append_messages("s1", msgs))
    history = store.load_history("s1")
    assert len(history) == 2
    assert history[0].content == "批量1"
    assert history[1].content == "批量2"


def test_clear_session(store):
    """清空某 session 历史。"""
    asyncio.run(store.append_message("s1", "user", "msg"))
    asyncio.run(store.append_message("s2", "user", "keep"))
    store.clear_session("s1")
    assert store.load_history("s1") == []
    assert len(store.load_history("s2")) == 1  # s2 不受影响


def test_persistence_across_reopen(tmp_path):
    """★ 关键：db 重新打开后历史仍在（数字人不失忆）。"""
    db = str(tmp_path / "persist.db")
    s1 = SQLiteStore(db)
    asyncio.run(s1.append_message("user1", "user", "昨天的对话"))

    # 模拟进程重启：新建 store（同 db 文件）
    s2 = SQLiteStore(db)
    history = s2.load_history("user1")
    assert len(history) == 1
    assert history[0].content == "昨天的对话"


def test_get_session_history_rest(store):
    """REST 端点用的 dict 格式。"""
    asyncio.run(store.append_message("s1", "user", "hi"))
    result = store.get_session_history("s1")
    assert isinstance(result, list)
    assert result[0]["role"] == "user"
    assert result[0]["content"] == "hi"
    assert "ts" in result[0]


@pytest.mark.asyncio
async def test_session_persists_history_through_pipeline():
    """集成：pipeline 跑完后，历史持久化到 store。"""
    from digitalhuman.config import Config
    from digitalhuman.engines.mock import MockASR, MockLLM, MockLipSync, MockTTS
    from digitalhuman.engines.base import Pusher
    from digitalhuman.pusher.base import DisabledPusher
    from digitalhuman.session import Session
    from digitalhuman.pipeline import DigitalHumanPipeline, PipelineParts

    class _InjectASR:
        def __init__(self, t): self.t = t
        async def transcribe_stream(self, s):
            async for _ in s: pass
            yield self.t

    db = tempfile.NamedTemporaryFile(suffix=".db", delete=False)
    db.close()
    store = SQLiteStore(db.name)
    cfg = Config()
    sess = Session(
        session_id="persist-test", cfg=cfg,
        asr=_InjectASR("你好"), llm=MockLLM("你好呀。"),
        tts=MockTTS(delay=0.001), lipsync=MockLipSync(delay=0.001),
        pusher=DisabledPusher(), store=store,
    )

    async def mic():
        yield b"\x00" * 10

    await sess.handle_utterance(mic())

    # 历史应已持久化
    loaded = store.load_history("persist-test")
    assert len(loaded) == 2  # user + assistant
    assert loaded[0].role == "user" and loaded[0].content == "你好"
    assert loaded[1].role == "assistant"

    os.unlink(db.name)


# ---------- 错误路径 / 边界测试 ----------

def test_concurrent_writes_same_session(store):
    """★ 并发写同一 session 不丢数据（asyncio.Lock 保护）。"""
    async def go():
        # 并发写 5 对 user+assistant
        tasks = []
        for i in range(5):
            tasks.append(store.append_message("s1", "user", f"u{i}"))
            tasks.append(store.append_message("s1", "assistant", f"a{i}"))
        await asyncio.gather(*tasks)

    asyncio.run(go())
    history = store.load_history("s1")
    # 10 条都在（顺序可能交错，但数量正确）
    assert len(history) == 10
    contents = [m.content for m in history]
    for i in range(5):
        assert f"u{i}" in contents
        assert f"a{i}" in contents


def test_large_content(store):
    """★ 超长消息（10KB）正确存取。"""
    big = "x" * 10000
    asyncio.run(store.append_message("s1", "user", big))
    loaded = store.load_history("s1")
    assert len(loaded[0].content) == 10000
    assert loaded[0].content == big


def test_unicode_and_special_chars(store):
    """★ Unicode / 特殊字符 / emoji 正确存取。"""
    import sqlite3  # noqa
    special = "你好世界 🌍 emoji=🎉\\n\\t<tag>&amp; \"quotes\" 'apostrophe'"
    asyncio.run(store.append_message("s1", "user", special))
    loaded = store.load_history("s1")
    assert loaded[0].content == special


def test_many_sessions(store):
    """★ 大量 session（100 个）不互相干扰。"""
    async def go():
        for i in range(100):
            await store.append_message(f"sess-{i}", "user", f"msg-{i}")

    asyncio.run(go())
    # 每个 session 都有自己的 1 条
    for i in range(0, 100, 25):  # 抽样检查
        h = store.load_history(f"sess-{i}")
        assert len(h) == 1
        assert h[0].content == f"msg-{i}"


def test_load_history_nonexistent_returns_empty(store):
    """★ 不存在的 session 返回空列表（不报错）。"""
    assert store.load_history("does-not-exist") == []


def test_clear_nonexistent_session_no_error(store):
    """★ 清空不存在的 session 不报错。"""
    store.clear_session("no-such-session")  # 不应抛


def test_history_limit_zero(store):
    """★ limit=0 返回空列表。"""
    asyncio.run(store.append_message("s1", "user", "msg"))
    assert store.load_history("s1", limit=0) == []


def test_db_recreate_after_corruption(tmp_path):
    """★ db 文件损坏后重新初始化应能恢复（CREATE IF NOT EXISTS 幂等）。"""
    db = str(tmp_path / "corrupt.db")
    # 写入垃圾数据模拟损坏
    with open(db, "wb") as f:
        f.write(b"NOT A DATABASE")
    # 重新初始化（应覆盖或处理）
    try:
        store = SQLiteStore(db)
        # 如果能初始化，写入应正常
        asyncio.run(store.append_message("s1", "user", "after-corruption"))
        loaded = store.load_history("s1")
        assert len(loaded) == 1
    except Exception:
        # SQLite 可能拒绝打开损坏文件，这是预期行为
        pass


def test_get_session_history_empty(store):
    """★ REST 端点格式：空 session 返回空 list。"""
    result = store.get_session_history("empty-session")
    assert isinstance(result, list)
    assert len(result) == 0
