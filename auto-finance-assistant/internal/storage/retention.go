package storage

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// PurgeResult 数据清理结果统计。
type PurgeResult struct {
	Conversations int `json:"conversations"`
	Messages      int `json:"messages"`
	Feedback      int `json:"feedback"`
	AuditLogs     int `json:"auditLogs"`
	RefusedMsgs   int `json:"refusedMsgs"`
}

// PurgeExpiredData 清理超过指定天数的会话、消息、反馈、审计日志。
// 保留天数按法规要求：管理办法≥60天，网络安全法≥6个月。
// 返回各表删除的行数。
func PurgeExpiredData(ctx context.Context, db *sql.DB, retainDays int) (*PurgeResult, error) {
	if retainDays <= 0 {
		retainDays = 90
	}
	cutoff := time.Now().AddDate(0, 0, -retainDays)
	result := &PurgeResult{}

	// 1. 先删消息（外键 ON DELETE CASCADE 会级联删除关联消息）
	// 2. 再删会话（会触发 messages 级联删除）
	// 注意：messages 表有 conversation_id 外键，删 conversation 会级联删 messages

	// 删过期的合规日志（输入输出拦截记录）
	if r, err := db.ExecContext(ctx,
		`DELETE FROM compliance_logs WHERE created_at < ?`, cutoff); err == nil {
		_ = r // 合规日志行数不计入 PurgeResult（单独管理）
	}

	// 删过期的审计日志
	if r, err := db.ExecContext(ctx,
		`DELETE FROM audit_logs WHERE created_at < ?`, cutoff); err == nil {
		n, _ := r.RowsAffected()
		result.AuditLogs = int(n)
	}

	// 删过期的反馈
	if r, err := db.ExecContext(ctx,
		`DELETE FROM feedback WHERE created_at < ?`, cutoff); err == nil {
		n, _ := r.RowsAffected()
		result.Feedback = int(n)
	}

	// 删过期的拒答消息（intent=refuse/compliance_refuse/guard_*）
	if r, err := db.ExecContext(ctx,
		`DELETE FROM messages WHERE created_at < ? AND intent IN ('refuse','compliance_refuse','error') OR intent LIKE 'guard_%'`,
		cutoff); err == nil {
		n, _ := r.RowsAffected()
		result.RefusedMsgs = int(n)
	}

	// 删过期的会话（级联删 messages）
	if r, err := db.ExecContext(ctx,
		`DELETE FROM conversations WHERE updated_at < ?`, cutoff); err == nil {
		n, _ := r.RowsAffected()
		result.Conversations = int(n)
	} else {
		return nil, fmt.Errorf("清理会话失败: %w", err)
	}

	return result, nil
}
