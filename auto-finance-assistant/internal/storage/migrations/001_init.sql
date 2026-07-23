-- 001_init.sql · M1 基础表
-- 会话、消息、设置、迁移版本表。对应原计划 7.5/7.6/7.9。

CREATE TABLE IF NOT EXISTS schema_migrations (
    version     TEXT PRIMARY KEY,
    applied_at  TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS conversations (
    id          TEXT PRIMARY KEY,
    user_id     TEXT,
    title       TEXT,
    summary     TEXT,
    created_at  TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at  TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS messages (
    id               TEXT PRIMARY KEY,
    conversation_id  TEXT NOT NULL,
    role             TEXT NOT NULL,            -- user | assistant | system
    content          TEXT NOT NULL,
    intent           TEXT,
    confidence       REAL,
    sources          TEXT,                     -- JSON array, M4 填充
    duration_ms      INTEGER,
    prompt_tokens    INTEGER,
    completion_tokens INTEGER,
    created_at       TEXT NOT NULL DEFAULT (datetime('now')),
    FOREIGN KEY (conversation_id) REFERENCES conversations(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_messages_conv ON messages(conversation_id, created_at);

CREATE TABLE IF NOT EXISTS settings (
    key         TEXT PRIMARY KEY,
    value       TEXT NOT NULL,
    updated_at  TEXT NOT NULL DEFAULT (datetime('now'))
);

