"""对话历史持久化（SQLite）。

让数字人"记住"用户——进程重启后仍能恢复对话上下文。
零依赖（Python stdlib sqlite3）。

设计：
- messages 表：存每条 user/assistant 消息（含时间戳）
- sessions 表：存 session 元信息（persona 等）
- Session 启动时 load_history，pipeline 成功后 append_message

并发安全：sqlite3 连接默认 check_same_thread=True，asyncio 单线程下 OK。
若要多线程访问，用 lock 或 check_same_thread=False + WAL。
"""
from __future__ import annotations

import asyncio
import logging
import sqlite3
import time
from pathlib import Path

from .engines.base import Message

log = logging.getLogger(__name__)


class SQLiteStore:
    """SQLite 持久化存储（对话历史 + session 元信息）。"""

    def __init__(self, db_path: str = "data/digitalhuman.db"):
        self.db_path = db_path
        # 确保目录存在
        Path(db_path).parent.mkdir(parents=True, exist_ok=True)
        self._lock = asyncio.Lock()
        self._init_db()

    def _init_db(self) -> None:
        """初始化表结构（幂等）。"""
        conn = sqlite3.connect(self.db_path)
        try:
            conn.executescript("""
                CREATE TABLE IF NOT EXISTS messages (
                    id INTEGER PRIMARY KEY AUTOINCREMENT,
                    session_id TEXT NOT NULL,
                    role TEXT NOT NULL,
                    content TEXT NOT NULL,
                    ts REAL NOT NULL
                );
                CREATE INDEX IF NOT EXISTS idx_messages_session ON messages(session_id, id);

                CREATE TABLE IF NOT EXISTS sessions (
                    id TEXT PRIMARY KEY,
                    persona TEXT DEFAULT 'default',
                    created_at REAL NOT NULL,
                    last_active REAL NOT NULL
                );
            """)
            conn.commit()
        finally:
            conn.close()

    def load_history(self, session_id: str, limit: int = 12) -> list[Message]:
        """加载某 session 的历史消息（最近 limit 条）。"""
        conn = sqlite3.connect(self.db_path)
        try:
            rows = conn.execute(
                "SELECT role, content FROM messages WHERE session_id = ? "
                "ORDER BY id DESC LIMIT ?",
                (session_id, limit)
            ).fetchall()
            # 倒序取的，反转回时间顺序
            return [Message(role=r[0], content=r[1]) for r in reversed(rows)]
        finally:
            conn.close()

    async def append_message(self, session_id: str, role: str, content: str) -> None:
        """追加一条消息（异步，加锁防并发写）。"""
        async with self._lock:
            # sqlite3 是同步，用 run_in_executor 避免阻塞事件循环
            loop = asyncio.get_event_loop()
            await loop.run_in_executor(None, self._append_sync,
                                       session_id, role, content)

    def _append_sync(self, session_id: str, role: str, content: str) -> None:
        conn = sqlite3.connect(self.db_path)
        try:
            now = time.time()
            conn.execute(
                "INSERT INTO messages(session_id, role, content, ts) VALUES (?, ?, ?, ?)",
                (session_id, role, content, now)
            )
            # upsert session
            conn.execute(
                "INSERT INTO sessions(id, persona, created_at, last_active) "
                "VALUES (?, 'default', ?, ?) "
                "ON CONFLICT(id) DO UPDATE SET last_active = ?",
                (session_id, now, now, now)
            )
            conn.commit()
        finally:
            conn.close()

    async def append_messages(self, session_id: str, messages: list[Message]) -> None:
        """批量追加（一次写多轮对话）。"""
        async with self._lock:
            loop = asyncio.get_event_loop()
            await loop.run_in_executor(None, self._append_batch_sync,
                                       session_id, messages)

    def _append_batch_sync(self, session_id: str, messages: list[Message]) -> None:
        conn = sqlite3.connect(self.db_path)
        try:
            now = time.time()
            for m in messages:
                conn.execute(
                    "INSERT INTO messages(session_id, role, content, ts) VALUES (?, ?, ?, ?)",
                    (session_id, m.role, m.content, now)
                )
            conn.execute(
                "INSERT INTO sessions(id, persona, created_at, last_active) "
                "VALUES (?, 'default', ?, ?) "
                "ON CONFLICT(id) DO UPDATE SET last_active = ?",
                (session_id, now, now, now)
            )
            conn.commit()
        finally:
            conn.close()

    def get_session_history(self, session_id: str, limit: int = 50) -> list[dict]:
        """REST 端点用：返回历史消息列表（dict 形式）。"""
        conn = sqlite3.connect(self.db_path)
        try:
            rows = conn.execute(
                "SELECT role, content, ts FROM messages WHERE session_id = ? "
                "ORDER BY id DESC LIMIT ?",
                (session_id, limit)
            ).fetchall()
            return [{"role": r[0], "content": r[1], "ts": r[2]} for r in reversed(rows)]
        finally:
            conn.close()

    def clear_session(self, session_id: str) -> None:
        """清空某 session 的历史（遗忘）。"""
        conn = sqlite3.connect(self.db_path)
        try:
            conn.execute("DELETE FROM messages WHERE session_id = ?", (session_id,))
            conn.commit()
        finally:
            conn.close()


# 全局单例（惰性创建）
_store: SQLiteStore | None = None


def get_store(db_path: str = "data/digitalhuman.db") -> SQLiteStore:
    """获取全局 store 单例。"""
    global _store
    if _store is None:
        _store = SQLiteStore(db_path)
    return _store
