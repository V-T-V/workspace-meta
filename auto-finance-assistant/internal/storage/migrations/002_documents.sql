-- 002_documents.sql · M3 文档与知识片段
-- 对应原计划 7.1/7.2。M1 阶段文件存在但不激活（由 migrations.go 按阶段 gating）。

CREATE TABLE IF NOT EXISTS documents (
    id             TEXT PRIMARY KEY,
    name           TEXT NOT NULL,
    original_name  TEXT NOT NULL,
    file_path      TEXT NOT NULL,
    file_type      TEXT NOT NULL,
    file_size      INTEGER NOT NULL,
    file_hash      TEXT NOT NULL,
    version        TEXT,
    institution    TEXT,
    product_code   TEXT,
    region         TEXT,
    customer_type  TEXT,
    effective_date TEXT,
    expiry_date    TEXT,
    status         TEXT NOT NULL DEFAULT 'draft',   -- draft|processing|active|inactive|failed|archived
    metadata       TEXT NOT NULL DEFAULT '{}',
    created_at     TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at     TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_documents_status ON documents(status);
CREATE INDEX IF NOT EXISTS idx_documents_hash ON documents(file_hash);

CREATE TABLE IF NOT EXISTS chunks (
    id            TEXT PRIMARY KEY,
    document_id   TEXT NOT NULL,
    sequence      INTEGER NOT NULL,
    title         TEXT,
    section       TEXT,
    content       TEXT NOT NULL,
    page_number   INTEGER,
    token_count   INTEGER,
    embedding     BLOB,                  -- M6 向量检索时填充
    metadata      TEXT NOT NULL DEFAULT '{}',
    created_at    TEXT NOT NULL DEFAULT (datetime('now')),
    FOREIGN KEY (document_id) REFERENCES documents(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_chunks_doc ON chunks(document_id, sequence);
