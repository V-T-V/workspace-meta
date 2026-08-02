// 本文件实现用户反馈的学习闭环：
//   - FeedbackStats 汇总赞踩分布与满意度
//   - ListCorrections 提取带纠正内容的负面反馈作为 FAQ 候选
//   - PromoteCorrection 把候选纠正提升为正式 FAQ（并标记 feedback 已采纳）
// 对应原计划 7.7 的学习闭环增强。
package storage

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// FeedbackStats 反馈统计。
type FeedbackStats struct {
	Total       int     `json:"total"`
	Positive    int     `json:"positive"`    // rating=1
	Negative    int     `json:"negative"`    // rating=-1
	WithCorrection int  `json:"withCorrection"` // 踩且带纠正
	Satisfaction float64 `json:"satisfaction"` // 正面占比（0~1）
}

// ComputeFeedbackStats 汇总反馈统计。
func ComputeFeedbackStats(ctx context.Context, db *sql.DB) (*FeedbackStats, error) {
	s := &FeedbackStats{}
	err := db.QueryRowContext(ctx, `
		SELECT
			COUNT(*),
			COALESCE(SUM(CASE WHEN rating=1 THEN 1 ELSE 0 END),0),
			COALESCE(SUM(CASE WHEN rating=-1 THEN 1 ELSE 0 END),0),
			COALESCE(SUM(CASE WHEN rating=-1 AND correction IS NOT NULL AND correction<>'' THEN 1 ELSE 0 END),0)
		FROM feedback
	`).Scan(&s.Total, &s.Positive, &s.Negative, &s.WithCorrection)
	if err != nil {
		return nil, fmt.Errorf("[storage] 反馈统计失败: %w", err)
	}
	if s.Total > 0 {
		s.Satisfaction = float64(s.Positive) / float64(s.Total)
	}
	return s, nil
}

// FAQCandidate 是从带纠正的负面反馈派生的 FAQ 候选。
type FAQCandidate struct {
	FeedbackID  string `json:"feedbackId"`
	MessageID   string `json:"messageId"`
	Question    string `json:"question"`    // 取自关联用户消息内容
	Correction  string `json:"correction"` // 用户纠正（拟作为答案）
	Promoted    bool   `json:"promoted"`   // 是否已提升为 FAQ
	CreatedAt   string `json:"createdAt"`
}

// ListCorrectionCandidates 列出带纠正内容的负面反馈（按时间倒序）。
// 关联 messages 表取用户原问题作为候选 question。
// limit<=0 或 >200 钳制为 100。
func ListCorrectionCandidates(ctx context.Context, db *sql.DB, limit int) ([]*FAQCandidate, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	rows, err := db.QueryContext(ctx, `
		SELECT f.id, f.message_id, COALESCE(m.content,''), COALESCE(f.correction,''),
		       EXISTS(SELECT 1 FROM faqs q WHERE q.source_document_id = 'feedback:'||f.id),
		       f.created_at
		FROM feedback f
		LEFT JOIN messages m ON m.id = f.message_id
		WHERE f.rating=-1 AND f.correction IS NOT NULL AND f.correction<>''
		ORDER BY f.created_at DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("[storage] 查询纠正候选失败: %w", err)
	}
	defer rows.Close()
	var out []*FAQCandidate
	for rows.Next() {
		c := &FAQCandidate{}
		if err := rows.Scan(&c.FeedbackID, &c.MessageID, &c.Question, &c.Correction,
			&c.Promoted, &c.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// PromoteCorrectionToFAQ 把一条带纠正的反馈提升为启用 FAQ。
// question/answer 为人工审核后的最终内容；返回新 FAQ id。
// 若该 feedback 已提升过则返回 ErrAlreadyPromoted。
var ErrAlreadyPromoted = fmt.Errorf("该反馈已提升为 FAQ")

// PromoteCorrectionToFAQ 提升纠正候选为 FAQ。
func PromoteCorrectionToFAQ(ctx context.Context, db *sql.DB, feedbackID, question, answer string) (string, error) {
	if question == "" || answer == "" {
		return "", fmt.Errorf("question 和 answer 不能为空")
	}
	// 检查是否已提升（用 source_document_id 标记）
	var existing string
	err := db.QueryRowContext(ctx,
		`SELECT id FROM faqs WHERE source_document_id=? LIMIT 1`,
		"feedback:"+feedbackID).Scan(&existing)
	if err == nil {
		return "", ErrAlreadyPromoted
	}
	if err != nil && err != sql.ErrNoRows {
		return "", fmt.Errorf("[storage] 查询已提升 FAQ 失败: %w", err)
	}

	newID := "faq-fb-" + feedbackID
	if _, err := db.ExecContext(ctx, `
		INSERT INTO faqs(id, category, question, normalized_question, answer, keywords,
		                 source_document_id, enabled, priority)
		VALUES(?, '反馈学习', ?, ?, ?, '', ?, 1, 5)
	`, newID, question, NormalizeFAQText(question), answer, "feedback:"+feedbackID); err != nil {
		return "", fmt.Errorf("[storage] 提升为 FAQ 失败: %w", err)
	}
	return newID, nil
}

// NormalizeFAQText 简单标准化（去首尾空白）。FAQ 的标准化匹配由 chat.Normalize 负责。
func NormalizeFAQText(s string) string {
	return strings.TrimSpace(s)
}
