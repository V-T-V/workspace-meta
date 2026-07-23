package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// Document 对应 documents 表。对应原计划 7.1。
type Document struct {
	ID            string
	Name          string
	OriginalName  string
	FilePath      string
	FileType      string // 扩展名，如 .docx
	FileSize      int64
	FileHash      string
	Version       string
	Institution   string
	ProductCode   string
	Region        string
	CustomerType  string
	EffectiveDate string
	ExpiryDate    string
	Status        string // draft|processing|active|inactive|failed|archived
	Metadata      string // JSON
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// 文档状态常量。
const (
	DocStatusDraft      = "draft"
	DocStatusProcessing = "processing"
	DocStatusActive     = "active"
	DocStatusInactive   = "inactive"
	DocStatusFailed     = "failed"
	DocStatusArchived   = "archived"
)

// CreateDocument 插入文档记录。
func CreateDocument(ctx context.Context, db *sql.DB, d *Document) error {
	if _, err := db.ExecContext(ctx, `
		INSERT INTO documents(id, name, original_name, file_path, file_type, file_size, file_hash,
			version, institution, product_code, region, customer_type, effective_date, expiry_date,
			status, metadata)
		VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		d.ID, d.Name, d.OriginalName, d.FilePath, d.FileType, d.FileSize, d.FileHash,
		nullString(d.Version), nullString(d.Institution), nullString(d.ProductCode),
		nullString(d.Region), nullString(d.CustomerType), nullString(d.EffectiveDate),
		nullString(d.ExpiryDate), d.Status, d.Metadata,
	); err != nil {
		return fmt.Errorf("[storage] 创建文档失败: %w", err)
	}
	return nil
}

// GetDocument 按 ID 查询。
func GetDocument(ctx context.Context, db *sql.DB, id string) (*Document, error) {
	row := db.QueryRowContext(ctx, selectDocCols+` FROM documents WHERE id=?`, id)
	return scanDocument(row)
}

// GetDocumentByHash 按 hash 查询（去重用）。
func GetDocumentByHash(ctx context.Context, db *sql.DB, hash string) (*Document, error) {
	row := db.QueryRowContext(ctx, selectDocCols+` FROM documents WHERE file_hash=?`, hash)
	return scanDocument(row)
}

// ListDocuments 返回文档列表。
func ListDocuments(ctx context.Context, db *sql.DB, status string, limit int) ([]*Document, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	q := selectDocCols + ` FROM documents`
	var args []any
	if status != "" {
		q += ` WHERE status=?`
		args = append(args, status)
	}
	q += ` ORDER BY created_at DESC LIMIT ?`
	args = append(args, limit)
	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("[storage] 查询文档列表失败: %w", err)
	}
	defer rows.Close()
	return scanDocuments(rows)
}

// UpdateDocumentStatus 更新文档状态。
func UpdateDocumentStatus(ctx context.Context, db *sql.DB, id, status string) error {
	res, err := db.ExecContext(ctx,
		`UPDATE documents SET status=?, updated_at=datetime('now') WHERE id=?`, status, id)
	if err != nil {
		return fmt.Errorf("[storage] 更新文档状态失败: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// UpdateDocumentMeta 更新文档元数据字段。
func UpdateDocumentMeta(ctx context.Context, db *sql.DB, d *Document) error {
	res, err := db.ExecContext(ctx, `
		UPDATE documents SET name=?, version=?, institution=?, product_code=?, region=?,
			customer_type=?, effective_date=?, expiry_date=?, metadata=?, updated_at=datetime('now')
		WHERE id=?
	`,
		d.Name, nullString(d.Version), nullString(d.Institution), nullString(d.ProductCode),
		nullString(d.Region), nullString(d.CustomerType), nullString(d.EffectiveDate),
		nullString(d.ExpiryDate), d.Metadata, d.ID,
	)
	if err != nil {
		return fmt.Errorf("[storage] 更新文档元数据失败: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteDocument 删除文档（ON DELETE CASCADE 会连带删除 chunks）。
func DeleteDocument(ctx context.Context, db *sql.DB, id string) error {
	res, err := db.ExecContext(ctx, `DELETE FROM documents WHERE id=?`, id)
	if err != nil {
		return fmt.Errorf("[storage] 删除文档失败: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

const selectDocCols = `SELECT id, name, original_name, file_path, file_type, file_size, file_hash,
	COALESCE(version,''), COALESCE(institution,''), COALESCE(product_code,''), COALESCE(region,''),
	COALESCE(customer_type,''), COALESCE(effective_date,''), COALESCE(expiry_date,''),
	status, COALESCE(metadata,'{}'), created_at, updated_at`

func scanDocument(row *sql.Row) (*Document, error) {
	d := &Document{}
	var createdRaw, updatedRaw string
	if err := row.Scan(&d.ID, &d.Name, &d.OriginalName, &d.FilePath, &d.FileType, &d.FileSize,
		&d.FileHash, &d.Version, &d.Institution, &d.ProductCode, &d.Region, &d.CustomerType,
		&d.EffectiveDate, &d.ExpiryDate, &d.Status, &d.Metadata, &createdRaw, &updatedRaw); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("[storage] 扫描文档失败: %w", err)
	}
	d.CreatedAt = parseTime(createdRaw)
	d.UpdatedAt = parseTime(updatedRaw)
	return d, nil
}

func scanDocuments(rows *sql.Rows) ([]*Document, error) {
	var out []*Document
	for rows.Next() {
		d := &Document{}
		var createdRaw, updatedRaw string
		if err := rows.Scan(&d.ID, &d.Name, &d.OriginalName, &d.FilePath, &d.FileType, &d.FileSize,
			&d.FileHash, &d.Version, &d.Institution, &d.ProductCode, &d.Region, &d.CustomerType,
			&d.EffectiveDate, &d.ExpiryDate, &d.Status, &d.Metadata, &createdRaw, &updatedRaw); err != nil {
			return nil, err
		}
		d.CreatedAt = parseTime(createdRaw)
		d.UpdatedAt = parseTime(updatedRaw)
		out = append(out, d)
	}
	return out, rows.Err()
}
