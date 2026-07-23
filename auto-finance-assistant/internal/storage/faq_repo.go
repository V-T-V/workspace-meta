package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// FAQ 对应 faqs 表。对应原计划 7.4。
// normalized_question 由 caller（chat.faq_match.Normalize）计算后存入。
type FAQ struct {
	ID                 string
	Category           string
	Question           string
	NormalizedQuestion string
	Answer             string
	Keywords           string // 空格分隔
	SourceDocumentID   string
	Enabled            bool
	Priority           int
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// CreateFAQ 插入一条 FAQ。
func CreateFAQ(ctx context.Context, db *sql.DB, f *FAQ) error {
	if _, err := db.ExecContext(ctx, `
		INSERT INTO faqs(id, category, question, normalized_question, answer, keywords, source_document_id, enabled, priority)
		VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		f.ID, nullString(f.Category), f.Question, f.NormalizedQuestion, f.Answer,
		f.Keywords, nullString(f.SourceDocumentID), boolToInt(f.Enabled), f.Priority,
	); err != nil {
		return fmt.Errorf("[storage] 创建 FAQ 失败: %w", err)
	}
	return nil
}

// UpdateFAQ 全量更新一条 FAQ（除 id 外）。
func UpdateFAQ(ctx context.Context, db *sql.DB, f *FAQ) error {
	res, err := db.ExecContext(ctx, `
		UPDATE faqs SET category=?, question=?, normalized_question=?, answer=?, keywords=?,
		               source_document_id=?, enabled=?, priority=?, updated_at=datetime('now')
		WHERE id=?
	`,
		nullString(f.Category), f.Question, f.NormalizedQuestion, f.Answer,
		f.Keywords, nullString(f.SourceDocumentID), boolToInt(f.Enabled), f.Priority, f.ID,
	)
	if err != nil {
		return fmt.Errorf("[storage] 更新 FAQ 失败: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteFAQ 删除一条 FAQ。
func DeleteFAQ(ctx context.Context, db *sql.DB, id string) error {
	res, err := db.ExecContext(ctx, `DELETE FROM faqs WHERE id=?`, id)
	if err != nil {
		return fmt.Errorf("[storage] 删除 FAQ 失败: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// GetFAQ 按 ID 查询。
func GetFAQ(ctx context.Context, db *sql.DB, id string) (*FAQ, error) {
	row := db.QueryRowContext(ctx, selectFAQCols+` FROM faqs WHERE id=?`, id)
	return scanFAQ(row)
}

// ListFAQs 返回 FAQ 列表（可选只看启用的）。
func ListFAQs(ctx context.Context, db *sql.DB, enabledOnly bool, limit int) ([]*FAQ, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	q := selectFAQCols + ` FROM faqs`
	if enabledOnly {
		q += ` WHERE enabled=1`
	}
	q += ` ORDER BY priority DESC, created_at DESC LIMIT ?`
	rows, err := db.QueryContext(ctx, q, limit)
	if err != nil {
		return nil, fmt.Errorf("[storage] 查询 FAQ 列表失败: %w", err)
	}
	defer rows.Close()
	return scanFAQs(rows)
}

// ListEnabledFAQsForMatch 返回所有启用 FAQ 用于匹配。
// 只取匹配所需字段，按优先级降序。
func ListEnabledFAQsForMatch(ctx context.Context, db *sql.DB) ([]*FAQ, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT id, COALESCE(category,''), question, normalized_question, answer, keywords, priority
		FROM faqs WHERE enabled=1 ORDER BY priority DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("[storage] 查询启用 FAQ 失败: %w", err)
	}
	defer rows.Close()

	var out []*FAQ
	for rows.Next() {
		f := &FAQ{}
		if err := rows.Scan(&f.ID, &f.Category, &f.Question, &f.NormalizedQuestion,
			&f.Answer, &f.Keywords, &f.Priority); err != nil {
			return nil, err
		}
		f.Enabled = true
		out = append(out, f)
	}
	return out, rows.Err()
}

const selectFAQCols = `SELECT id, COALESCE(category,''), question, normalized_question, answer,
       COALESCE(keywords,''), COALESCE(source_document_id,''), enabled, COALESCE(priority,0),
       created_at, updated_at`

func scanFAQ(row *sql.Row) (*FAQ, error) {
	f := &FAQ{}
	var createdRaw, updatedRaw string
	var enabledInt int
	if err := row.Scan(&f.ID, &f.Category, &f.Question, &f.NormalizedQuestion, &f.Answer,
		&f.Keywords, &f.SourceDocumentID, &enabledInt, &f.Priority, &createdRaw, &updatedRaw); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("[storage] 扫描 FAQ 失败: %w", err)
	}
	f.Enabled = enabledInt != 0
	f.CreatedAt = parseTime(createdRaw)
	f.UpdatedAt = parseTime(updatedRaw)
	return f, nil
}

func scanFAQs(rows *sql.Rows) ([]*FAQ, error) {
	var out []*FAQ
	for rows.Next() {
		f := &FAQ{}
		var createdRaw, updatedRaw string
		var enabledInt int
		if err := rows.Scan(&f.ID, &f.Category, &f.Question, &f.NormalizedQuestion, &f.Answer,
			&f.Keywords, &f.SourceDocumentID, &enabledInt, &f.Priority, &createdRaw, &updatedRaw); err != nil {
			return nil, err
		}
		f.Enabled = enabledInt != 0
		f.CreatedAt = parseTime(createdRaw)
		f.UpdatedAt = parseTime(updatedRaw)
		out = append(out, f)
	}
	return out, rows.Err()
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// CountFAQs 返回 FAQ 总数（供指标展示）。
func CountFAQs(ctx context.Context, db *sql.DB) (int, error) {
	var n int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM faqs`).Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}
