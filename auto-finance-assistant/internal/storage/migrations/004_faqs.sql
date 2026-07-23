-- 004_faqs.sql · M2 FAQ 表
-- 对应原计划 7.4。M1 阶段存在但不激活。

CREATE TABLE IF NOT EXISTS faqs (
    id                  TEXT PRIMARY KEY,
    category            TEXT,
    question            TEXT NOT NULL,
    normalized_question TEXT NOT NULL,
    answer              TEXT NOT NULL,
    keywords            TEXT NOT NULL DEFAULT '',
    source_document_id  TEXT,
    enabled             INTEGER NOT NULL DEFAULT 1,
    priority            INTEGER NOT NULL DEFAULT 0,
    created_at          TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at          TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_faqs_enabled ON faqs(enabled, priority DESC);
