package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// GetSetting 读取单个配置项，不存在返回空串与 ErrNotFound。
func GetSetting(ctx context.Context, db *sql.DB, key string) (string, error) {
	var v string
	err := db.QueryRowContext(ctx, `SELECT value FROM settings WHERE key = ?`, key).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("[storage] 读取设置 %s 失败: %w", key, err)
	}
	return v, nil
}

// SetSetting upsert 配置项。
func SetSetting(ctx context.Context, db *sql.DB, key, value string) error {
	if _, err := db.ExecContext(ctx, `
		INSERT INTO settings(key, value, updated_at) VALUES(?, ?, datetime('now'))
		ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = datetime('now')
	`, key, value); err != nil {
		return fmt.Errorf("[storage] 写入设置 %s 失败: %w", key, err)
	}
	return nil
}
