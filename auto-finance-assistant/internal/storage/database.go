// Package storage 封装 SQLite 持久化层。
// 使用 modernc.org/sqlite（纯 Go，免 CGO，Windows 无 gcc 可编译）。
package storage

import (
	"database/sql"
	"fmt"
	"log/slog"
	"net/url"
	"path/filepath"

	_ "modernc.org/sqlite" // 注册 sqlite 驱动
)

// OpenDB 打开 SQLite 数据库并应用关键 PRAGMA。
// dbPath 相对路径基于进程工作目录解析。
func OpenDB(dbPath string, log *slog.Logger) (*sql.DB, error) {
	abs, err := filepath.Abs(dbPath)
	if err != nil {
		return nil, fmt.Errorf("[storage] 解析数据库路径失败: %w", err)
	}

	// modernc.org/sqlite 用 DSN 配置：WAL 模式 + 外键 + 忙等待。
	// _pragma 参数通过 query string 传递。
	dsn := buildDSN(abs)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("[storage] 打开数据库失败: %w", err)
	}

	// 验证连接 + 应用 PRAGMA。
	if _, err := db.Exec(`
		PRAGMA journal_mode=WAL;
		PRAGMA foreign_keys=ON;
		PRAGMA busy_timeout=5000;
		PRAGMA synchronous=NORMAL;
	`); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("[storage] 应用 PRAGMA 失败: %w", err)
	}

	// 读连接放开（WAL 支持并发读），配合 _txlock=immediate 保证写串行。
	// 避免 SetMaxOpenConns(1) 导致慢查询阻塞全局。
	db.SetMaxOpenConns(8)
	db.SetMaxIdleConns(4)

	log.Info("[storage] 数据库已打开", "path", abs, "mode", "WAL")
	return db, nil
}

// buildDSN 构造 modernc.org/sqlite 的 DSN。
// _txlock=immediate：写事务开始即获锁，避免升级死锁（配合多读连接）。
func buildDSN(absPath string) string {
	q := url.Values{}
	q.Add("_pragma", "busy_timeout(5000)")
	q.Add("_pragma", "journal_mode(WAL)")
	q.Add("_pragma", "foreign_keys(ON)")
	q.Add("_pragma", "synchronous(NORMAL)")
	q.Add("_txlock", "immediate")
	return "file:" + filepath.ToSlash(absPath) + "?" + q.Encode()
}
