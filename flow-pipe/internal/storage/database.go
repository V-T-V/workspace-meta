// Package storage 实现数据持久化：SQLite（modernc.org/sqlite，纯 Go 免 CGO）+ WAL + embed migration。
// 对齐 workspace-ops / generic-admin 的 storage 范式。
package storage

import (
	"database/sql"
	"fmt"
	"net/url"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite" // 纯 Go SQLite 驱动
)

// Open 打开 SQLite 数据库，配置 WAL/FK/busy_timeout PRAGMA，跑 migration。
func Open(path string) (*sql.DB, error) {
	dsn := buildDSN(path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("打开 sqlite 失败: %w", err)
	}
	db.SetMaxOpenConns(8)
	db.SetMaxIdleConns(4)
	db.SetConnMaxLifetime(time.Hour)
	db.SetConnMaxIdleTime(30 * time.Minute)

	if err := Migrate(db); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

// buildDSN 构造带 PRAGMA 的 SQLite DSN。
func buildDSN(absPath string) string {
	q := url.Values{}
	q.Add("_pragma", "busy_timeout(5000)")
	q.Add("_pragma", "journal_mode(WAL)")
	q.Add("_pragma", "foreign_keys(ON)")
	q.Add("_pragma", "synchronous(NORMAL)")
	q.Add("_txlock", "immediate")
	return "file:" + filepath.ToSlash(absPath) + "?" + q.Encode()
}
