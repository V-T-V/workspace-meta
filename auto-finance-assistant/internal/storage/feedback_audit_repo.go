package storage

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// Feedback 对应 feedback 表。对应原计划 7.7。
type Feedback struct {
	ID          string
	MessageID   string
	Rating      int    // 1=赞 -1=踩
	Reason      string
	Correction  string
	CreatedAt   time.Time
}

// CreateFeedback 记录用户反馈。
func CreateFeedback(ctx context.Context, db *sql.DB, f *Feedback) error {
	if _, err := db.ExecContext(ctx, `
		INSERT INTO feedback(id, message_id, rating, reason, correction)
		VALUES(?, ?, ?, ?, ?)
	`, f.ID, f.MessageID, f.Rating, nullString(f.Reason), nullString(f.Correction)); err != nil {
		return fmt.Errorf("[storage] 创建反馈失败: %w", err)
	}
	return nil
}

// ListFeedback 返回反馈列表。
func ListFeedback(ctx context.Context, db *sql.DB, limit int) ([]*Feedback, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := db.QueryContext(ctx, `
		SELECT id, message_id, rating, COALESCE(reason,''), COALESCE(correction,''), created_at
		FROM feedback ORDER BY created_at DESC LIMIT ?
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("[storage] 查询反馈失败: %w", err)
	}
	defer rows.Close()
	var out []*Feedback
	for rows.Next() {
		f := &Feedback{}
		var createdRaw string
		if err := rows.Scan(&f.ID, &f.MessageID, &f.Rating, &f.Reason, &f.Correction, &createdRaw); err != nil {
			return nil, err
		}
		f.CreatedAt = parseTime(createdRaw)
		out = append(out, f)
	}
	return out, rows.Err()
}

// AuditLog 对应 audit_logs 表。对应原计划 7.8。
type AuditLog struct {
	ID         string
	UserID     string
	Action     string
	TargetType string
	TargetID   string
	Detail     string
	IPAddress  string
	CreatedAt  time.Time
}

// CreateAuditLog 记录审计日志。
func CreateAuditLog(ctx context.Context, db *sql.DB, a *AuditLog) error {
	if _, err := db.ExecContext(ctx, `
		INSERT INTO audit_logs(id, user_id, action, target_type, target_id, detail, ip_address)
		VALUES(?, ?, ?, ?, ?, ?, ?)
	`, a.ID, nullString(a.UserID), a.Action, nullString(a.TargetType),
		nullString(a.TargetID), nullString(a.Detail), nullString(a.IPAddress)); err != nil {
		return fmt.Errorf("[storage] 创建审计日志失败: %w", err)
	}
	return nil
}

// ListAuditLogs 返回审计日志列表。
func ListAuditLogs(ctx context.Context, db *sql.DB, action string, limit int) ([]*AuditLog, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	q := `SELECT id, COALESCE(user_id,''), action, COALESCE(target_type,''),
		      COALESCE(target_id,''), COALESCE(detail,''), COALESCE(ip_address,''), created_at
	      FROM audit_logs`
	var args []any
	if action != "" {
		q += ` WHERE action=?`
		args = append(args, action)
	}
	q += ` ORDER BY created_at DESC LIMIT ?`
	args = append(args, limit)
	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("[storage] 查询审计日志失败: %w", err)
	}
	defer rows.Close()
	var out []*AuditLog
	for rows.Next() {
		a := &AuditLog{}
		var createdRaw string
		if err := rows.Scan(&a.ID, &a.UserID, &a.Action, &a.TargetType, &a.TargetID,
			&a.Detail, &a.IPAddress, &createdRaw); err != nil {
			return nil, err
		}
		a.CreatedAt = parseTime(createdRaw)
		out = append(out, a)
	}
	return out, rows.Err()
}

// ListRefusedMessages 返回低置信拒答的消息（供"无答案问题列表"）。
func ListRefusedMessages(ctx context.Context, db *sql.DB, limit int) ([]*Message, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := db.QueryContext(ctx, `
		SELECT id, conversation_id, role, content, COALESCE(intent,''), COALESCE(confidence,0),
		       COALESCE(sources,''), COALESCE(duration_ms,0), COALESCE(prompt_tokens,0),
		       COALESCE(completion_tokens,0), created_at
		FROM messages WHERE intent='refuse' ORDER BY created_at DESC LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Message
	for rows.Next() {
		m := &Message{}
		var createdRaw string
		if err := rows.Scan(&m.ID, &m.ConversationID, &m.Role, &m.Content, &m.Intent, &m.Confidence,
			&m.Sources, &m.DurationMS, &m.PromptTokens, &m.CompletionTokens, &createdRaw); err != nil {
			return nil, err
		}
		m.CreatedAt = parseTime(createdRaw)
		out = append(out, m)
	}
	return out, rows.Err()
}
