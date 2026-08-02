package storage

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// ComplianceLog 合规日志记录。
// 记录每次输入/输出/拦截的完整证据链，满足 GB/T 45654-2025 安全评估要求。
type ComplianceLog struct {
	ID               string    `json:"id"`
	TraceID          string    `json:"traceId"`
	EventType        string    `json:"eventType"`        // input | output | guard_block | compliance_block | model_invoke | rag_refuse
	ConversationID   string    `json:"conversationId"`
	RawInput         string    `json:"rawInput"`         // 脱敏后的原始输入
	RawOutput        string    `json:"rawOutput"`        // 脱敏后的原始输出
	Intent           string    `json:"intent"`           // 判定意图
	ActionTaken      string    `json:"actionTaken"`      // pass | block | replace | refuse | answer
	Reason           string    `json:"reason"`           // 拦截原因
	DurationMS       int64     `json:"durationMs"`
	PromptTokens     int       `json:"promptTokens"`
	CompletionTokens int       `json:"completionTokens"`
	IPAddress        string    `json:"ipAddress"`
	CreatedAt        time.Time `json:"createdAt"`
}

// CreateComplianceLog 写入合规日志。
func CreateComplianceLog(ctx context.Context, db *sql.DB, log *ComplianceLog) error {
	if log.ID == "" {
		return fmt.Errorf("[storage] compliance log id 不能为空")
	}
	_, err := db.ExecContext(ctx, `
		INSERT INTO compliance_logs(id, trace_id, event_type, conversation_id,
			raw_input, raw_output, intent, action_taken, reason,
			duration_ms, prompt_tokens, completion_tokens, ip_address)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, log.ID, log.TraceID, log.EventType, nullString(log.ConversationID),
		log.RawInput, log.RawOutput, log.Intent, log.ActionTaken, log.Reason,
		log.DurationMS, log.PromptTokens, log.CompletionTokens, log.IPAddress)
	if err != nil {
		return fmt.Errorf("[storage] 写入合规日志失败: %w", err)
	}
	return nil
}

// ListComplianceLogs 查询合规日志。
// eventType 为空时查全部。limit 上限 500。
func ListComplianceLogs(ctx context.Context, db *sql.DB, eventType string, limit int) ([]*ComplianceLog, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	q := `SELECT id, trace_id, event_type, COALESCE(conversation_id,''),
		      COALESCE(raw_input,''), COALESCE(raw_output,''),
		      COALESCE(intent,''), COALESCE(action_taken,''), COALESCE(reason,''),
		      duration_ms, prompt_tokens, completion_tokens, COALESCE(ip_address,''), created_at
	      FROM compliance_logs`
	var args []any
	if eventType != "" {
		q += ` WHERE event_type = ?`
		args = append(args, eventType)
	}
	q += ` ORDER BY created_at DESC LIMIT ?`
	args = append(args, limit)

	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("[storage] 查询合规日志失败: %w", err)
	}
	defer rows.Close()

	var out []*ComplianceLog
	for rows.Next() {
		var l ComplianceLog
		var createdRaw string
		if err := rows.Scan(&l.ID, &l.TraceID, &l.EventType, &l.ConversationID,
			&l.RawInput, &l.RawOutput, &l.Intent, &l.ActionTaken, &l.Reason,
			&l.DurationMS, &l.PromptTokens, &l.CompletionTokens, &l.IPAddress, &createdRaw); err != nil {
			return nil, err
		}
		l.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdRaw)
		out = append(out, &l)
	}
	return out, nil
}

// PurgeComplianceLogs 清理超过指定天数的合规日志。
func PurgeComplianceLogs(ctx context.Context, db *sql.DB, retainDays int) (int, error) {
	if retainDays <= 0 {
		retainDays = 180
	}
	cutoff := time.Now().AddDate(0, 0, -retainDays)
	r, err := db.ExecContext(ctx, `DELETE FROM compliance_logs WHERE created_at < ?`, cutoff)
	if err != nil {
		return 0, err
	}
	n, _ := r.RowsAffected()
	return int(n), nil
}
