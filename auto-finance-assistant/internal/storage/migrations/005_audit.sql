-- 005_audit.sql · M7 反馈与审计
-- 对应原计划 7.7/7.8。M1 阶段存在但不激活。

CREATE TABLE IF NOT EXISTS feedback (
    id          TEXT PRIMARY KEY,
    message_id  TEXT NOT NULL,
    rating      INTEGER NOT NULL,        -- 1=赞 -1=踩
    reason      TEXT,
    correction  TEXT,
    created_at  TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_feedback_msg ON feedback(message_id);

CREATE TABLE IF NOT EXISTS audit_logs (
    id           TEXT PRIMARY KEY,
    user_id      TEXT,
    action       TEXT NOT NULL,
    target_type  TEXT,
    target_id    TEXT,
    detail       TEXT,
    ip_address   TEXT,
    created_at   TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_audit_action ON audit_logs(action, created_at);
