package storage

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"log/slog"
	"path"
	"sort"
	"strings"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// migrationsTable 创建版本追踪表的 DDL。
const migrationsTableDDL = `
CREATE TABLE IF NOT EXISTS schema_migrations (
  version TEXT PRIMARY KEY,
  applied_at TEXT NOT NULL DEFAULT (datetime('now'))
);
`

// Migrate 跑全部未应用的 migrations/*.sql（按文件名排序）。
func Migrate(db *sql.DB) error {
	if _, err := db.Exec(migrationsTableDDL); err != nil {
		return fmt.Errorf("创建 schema_migrations 失败: %w", err)
	}
	applied, err := loadAppliedVersions(db)
	if err != nil {
		return err
	}
	names, err := fsNames()
	if err != nil {
		return err
	}
	for _, name := range names {
		if applied[name] {
			continue
		}
		if err := applyOne(db, name); err != nil {
			return fmt.Errorf("migration %s 失败: %w", name, err)
		}
		slog.Info("[migration] applied", "version", name)
	}
	return nil
}

func loadAppliedVersions(db *sql.DB) (map[string]bool, error) {
	rows, err := db.Query(`SELECT version FROM schema_migrations`)
	if err != nil {
		return nil, fmt.Errorf("查询已应用版本失败: %w", err)
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		out[v] = true
	}
	return out, rows.Err()
}

func fsNames() ([]string, error) {
	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	return names, nil
}

func applyOne(db *sql.DB, name string) error {
	raw, err := migrationsFS.ReadFile(path.Join("migrations", name))
	if err != nil {
		return err
	}
	stmts := splitSQLStatements(string(raw))

	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	for _, s := range stmts {
		if strings.TrimSpace(s) == "" {
			continue
		}
		if _, err := tx.Exec(s); err != nil {
			return fmt.Errorf("exec 失败: %w\nSQL: %s", err, s)
		}
	}
	if _, err := tx.Exec(`INSERT INTO schema_migrations(version) VALUES(?)`, name); err != nil {
		return err
	}
	return tx.Commit()
}

// splitSQLStatements 按 ; 分号分割（migration 无触发器/过程，够用）。
func splitSQLStatements(s string) []string {
	parts := strings.Split(s, ";")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		t := strings.TrimSpace(p)
		if t != "" {
			out = append(out, p)
		}
	}
	return out
}
