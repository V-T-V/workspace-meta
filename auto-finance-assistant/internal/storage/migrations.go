package storage

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"log/slog"
	"sort"
	"strings"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// migrationFSRoot 是 embed 内 migration SQL 所在目录。
const migrationFSRoot = "migrations"

// Migrate 按文件名顺序执行未应用的 migration。
// 通过 schema_migrations 表追踪已应用版本，保证幂等。
// activeVersions 限制只应用指定版本集合（按阶段 gating 用）；
// 传 nil 表示应用全部。
func Migrate(ctx context.Context, db *sql.DB, activeVersions []string, log *slog.Logger) error {
	// 确保版本表存在（001_init.sql 会创建，但首次启动前需保障）。
	if _, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version     TEXT PRIMARY KEY,
			applied_at  TEXT NOT NULL DEFAULT (datetime('now'))
		);
	`); err != nil {
		return fmt.Errorf("[migrate] 创建版本表失败: %w", err)
	}

	applied, err := loadApplied(ctx, db)
	if err != nil {
		return err
	}

	files, err := listMigrationFiles()
	if err != nil {
		return err
	}

	// 过滤：若指定 activeVersions，只执行其中的。
	activeSet := toSet(activeVersions)

	for _, fname := range files {
		version := strings.TrimSuffix(fname, ".sql")
		if applied[version] {
			continue
		}
		if activeSet != nil && !activeSet[version] {
			log.Debug("[migrate] 跳过未激活的 migration（后续 Milestone）", "version", version)
			continue
		}

		content, err := migrationsFS.ReadFile(migrationFSRoot + "/" + fname)
		if err != nil {
			return fmt.Errorf("[migrate] 读取 %s 失败: %w", fname, err)
		}

		if err := execMigration(ctx, db, version, string(content)); err != nil {
			return fmt.Errorf("[migrate] 执行 %s 失败: %w", fname, err)
		}
		log.Info("[migrate] 已应用", "version", version)
	}

	return nil
}

// execMigration 在单个事务中执行 SQL 并登记版本。
func execMigration(ctx context.Context, db *sql.DB, version, sqlText string) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, sqlText); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO schema_migrations(version) VALUES (?)`, version); err != nil {
		return err
	}
	return tx.Commit()
}

func loadApplied(ctx context.Context, db *sql.DB) (map[string]bool, error) {
	rows, err := db.QueryContext(ctx, `SELECT version FROM schema_migrations`)
	if err != nil {
		return nil, fmt.Errorf("[migrate] 查询已应用版本失败: %w", err)
	}
	defer rows.Close()
	m := map[string]bool{}
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		m[v] = true
	}
	return m, rows.Err()
}

func listMigrationFiles() ([]string, error) {
	entries, err := migrationsFS.ReadDir(migrationFSRoot)
	if err != nil {
		return nil, fmt.Errorf("[migrate] 列举 migration 目录失败: %w", err)
	}
	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files) // 001 < 002 < ... 保证顺序
	return files, nil
}

func toSet(items []string) map[string]bool {
	if items == nil {
		return nil
	}
	m := make(map[string]bool, len(items))
	for _, v := range items {
		m[v] = true
	}
	return m
}

// M1ActiveVersions 是 Milestone 1 激活的 migration 版本。
// 后续 Milestone 推进时追加，避免提前建表。
func M1ActiveVersions() []string {
	return []string{"001_init"}
}

// M2ActiveVersions 是 Milestone 2 激活的 migration 版本（M1 + FAQ 表）。
func M2ActiveVersions() []string {
	return []string{"001_init", "004_faqs"}
}

// M3ActiveVersions 是 Milestone 3 激活的 migration 版本（+ documents + chunks）。
func M3ActiveVersions() []string {
	return []string{"001_init", "002_documents", "004_faqs"}
}

// M4ActiveVersions 是 Milestone 4 激活的 migration 版本（+ FTS5）。
func M4ActiveVersions() []string {
	return []string{"001_init", "002_documents", "003_fts", "004_faqs"}
}

// AllActiveVersions 激活全部 migration（M7+）。
func AllActiveVersions() []string {
	return []string{"001_init", "002_documents", "003_fts", "004_faqs", "005_audit"}
}
