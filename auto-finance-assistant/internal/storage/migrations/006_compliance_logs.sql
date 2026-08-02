-- 006_compliance_logs.sql
-- 合规日志表：记录每次输入输出拦截的完整证据链。
-- 满足 GB/T 45654-2025 安全评估 + 《生成式AI管理办法》日志留存要求。
CREATE TABLE IF NOT EXISTS compliance_logs (
    id TEXT PRIMARY KEY,
    trace_id TEXT NOT NULL,              -- 关联请求的 trace ID
    event_type TEXT NOT NULL,            -- input | output | guard_block | compliance_block | model_invoke | rag_refuse
    conversation_id TEXT,                -- 会话 ID（可为空）
    raw_input TEXT,                      -- 原始输入（脱敏后）
    raw_output TEXT,                     -- 原始输出（脱敏后）
    intent TEXT,                         -- 判定意图（guard_shortcut / guard_reject:xxx / compliance_refuse / model / refuse）
    action_taken TEXT,                   -- 处理动作（pass / block / replace / refuse / answer）
    reason TEXT,                         -- 拦截原因（如 prompt_injection / profanity / off_topic）
    duration_ms INTEGER DEFAULT 0,       -- 处理耗时
    prompt_tokens INTEGER DEFAULT 0,     -- 模型输入 tokens
    completion_tokens INTEGER DEFAULT 0, -- 模型输出 tokens
    ip_address TEXT,                     -- 请求来源 IP
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_compliance_event ON compliance_logs(event_type, created_at);
CREATE INDEX IF NOT EXISTS idx_compliance_trace ON compliance_logs(trace_id);
CREATE INDEX IF NOT EXISTS idx_compliance_conv ON compliance_logs(conversation_id);
