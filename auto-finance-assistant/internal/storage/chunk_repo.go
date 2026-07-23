package storage

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// Chunk 对应 chunks 表。对应原计划 7.2。
type Chunk struct {
	ID           string
	DocumentID   string
	Sequence     int
	Title        string
	Section      string
	Content      string
	PageNumber   int
	TokenCount   int
	Embedding    []byte // M6 向量
	Metadata     string // JSON
	CreatedAt    time.Time
}

// CreateChunk 插入一个知识片段。
func CreateChunk(ctx context.Context, db *sql.DB, c *Chunk) error {
	if _, err := db.ExecContext(ctx, `
		INSERT INTO chunks(id, document_id, sequence, title, section, content, page_number, token_count, metadata)
		VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		c.ID, c.DocumentID, c.Sequence, nullString(c.Title), nullString(c.Section),
		c.Content, nullInt(c.PageNumber), c.TokenCount, c.Metadata,
	); err != nil {
		return fmt.Errorf("[storage] 创建 chunk 失败: %w", err)
	}
	return nil
}

// CreateChunks 批量插入片段（单事务）。
func CreateChunks(ctx context.Context, db *sql.DB, chunks []*Chunk) error {
	if len(chunks) == 0 {
		return nil
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO chunks(id, document_id, sequence, title, section, content, page_number, token_count, metadata)
		VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("[storage] 预编译 chunk 插入失败: %w", err)
	}
	defer stmt.Close()

	for _, c := range chunks {
		if _, err := stmt.ExecContext(ctx, c.ID, c.DocumentID, c.Sequence,
			nullString(c.Title), nullString(c.Section), c.Content,
			nullInt(c.PageNumber), c.TokenCount, c.Metadata); err != nil {
			return fmt.Errorf("[storage] 批量插入 chunk 失败: %w", err)
		}
	}
	return tx.Commit()
}

// DeleteChunksByDocument 删除文档的所有片段。
func DeleteChunksByDocument(ctx context.Context, db *sql.DB, documentID string) error {
	if _, err := db.ExecContext(ctx, `DELETE FROM chunks WHERE document_id=?`, documentID); err != nil {
		return fmt.Errorf("[storage] 删除文档片段失败: %w", err)
	}
	return nil
}

// ListChunksByDocument 返回文档的片段（按 sequence）。
func ListChunksByDocument(ctx context.Context, db *sql.DB, documentID string) ([]*Chunk, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT id, document_id, sequence, COALESCE(title,''), COALESCE(section,''), content,
		       COALESCE(page_number,0), COALESCE(token_count,0), COALESCE(metadata,'{}'), created_at
		FROM chunks WHERE document_id=? ORDER BY sequence
	`, documentID)
	if err != nil {
		return nil, fmt.Errorf("[storage] 查询片段失败: %w", err)
	}
	defer rows.Close()

	var out []*Chunk
	for rows.Next() {
		c := &Chunk{}
		var createdRaw string
		if err := rows.Scan(&c.ID, &c.DocumentID, &c.Sequence, &c.Title, &c.Section, &c.Content,
			&c.PageNumber, &c.TokenCount, &c.Metadata, &createdRaw); err != nil {
			return nil, err
		}
		c.CreatedAt = parseTime(createdRaw)
		out = append(out, c)
	}
	return out, rows.Err()
}

// CountChunksByDocument 统计文档片段数。
func CountChunksByDocument(ctx context.Context, db *sql.DB, documentID string) (int, error) {
	var n int
	if err := db.QueryRowContext(ctx,
		`SELECT count(*) FROM chunks WHERE document_id=?`, documentID).Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}

// nullInt 0 转 NULL。
func nullInt(n int) any {
	if n == 0 {
		return nil
	}
	return n
}
