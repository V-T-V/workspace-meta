package storage

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// ProjectRecord 是 projects 表的一行。
type ProjectRecord struct {
	ID           int64
	Slug         string
	Path         string
	StackPrimary string
	HasAgentsMD  bool
	GitBranch    string
	GitDirty     bool
	LastScanID   int64
	LastScanAt   *time.Time
}

// queryExecer 是 *sql.DB 与 *sql.Tx 都满足的最小接口（Exec / QueryRow）。
// 便于 getProjectID 在事务内外复用。
type queryExecer interface {
	Exec(query string, args ...any) (sql.Result, error)
	QueryRow(query string, args ...any) *sql.Row
}

// SaveFacts 把一个项目的 inspector.Facts 写入 projects + project_facts。
// 同 slug 的项目做 UPSERT。facts 是 KV map。
//
// 整个写入（1 UPSERT + 1 DELETE + N INSERT）在一个事务内完成，
// 中途任何一步失败都会回滚，避免留下半成品数据。
func SaveFacts(db *sql.DB, scanID int64, slug, path string, facts map[string]string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("开启事务失败: %w", err)
	}
	// defer rollback：Commit 成功后 rollback 返回 sql.ErrTxDone，安全忽略。
	defer func() { _ = tx.Rollback() }()

	// 提取 projects 表的独立列（便于查询/索引）
	stack := facts["stack_primary"]
	hasAgents := facts["has_agents_md"] == "true"
	gitBranch := facts["git_branch"]
	gitDirty := facts["git_dirty"] == "true"

	// UPSERT project
	res, err := tx.Exec(`
		INSERT INTO projects(slug, path, stack_primary, has_agents_md, git_branch, git_dirty, last_scan_id, last_scan_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(slug) DO UPDATE SET
			path = excluded.path,
			stack_primary = excluded.stack_primary,
			has_agents_md = excluded.has_agents_md,
			git_branch = excluded.git_branch,
			git_dirty = excluded.git_dirty,
			last_scan_id = excluded.last_scan_id,
			last_scan_at = excluded.last_scan_at`,
		slug, path, sql.NullString{String: stack, Valid: stack != ""}, hasAgents,
		sql.NullString{String: gitBranch, Valid: gitBranch != ""}, gitDirty,
		scanID, time.Now().UTC().Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("upsert project %s 失败: %w", slug, err)
	}
	// 拿 project id（INSERT 时是新的；CONFLICT 时查出来）
	projectID, err := getProjectID(tx, slug)
	if err != nil {
		return err
	}
	_ = res

	// 先删该 project + scan 的旧 facts（重跑 scan 时清理），再批量插入。
	if _, err := tx.Exec(
		`DELETE FROM project_facts WHERE project_id = ? AND scan_id = ?`,
		projectID, scanID,
	); err != nil {
		return fmt.Errorf("清理旧 facts 失败: %w", err)
	}
	for k, v := range facts {
		if _, err := tx.Exec(
			`INSERT OR IGNORE INTO project_facts(project_id, scan_id, fact_key, fact_value) VALUES (?, ?, ?, ?)`,
			projectID, scanID, k, v,
		); err != nil {
			return fmt.Errorf("插入 fact %s=%s 失败: %w", k, v, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("提交事务失败: %w", err)
	}
	return nil
}

func getProjectID(qe queryExecer, slug string) (int64, error) {
	var id int64
	err := qe.QueryRow(`SELECT id FROM projects WHERE slug = ?`, slug).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("查询 project id 失败 slug=%s: %w", slug, err)
	}
	return id, nil
}

// GetProjectIDBySlug 按 slug 查项目 id（导出版，供 cmd 的 testrunner 入库用）。
// 不存在返回 (0, error)。
func GetProjectIDBySlug(db *sql.DB, slug string) (int64, error) {
	return getProjectID(db, slug)
}

// AllProjects 返回全部项目记录（按 slug 升序）。
func AllProjects(db *sql.DB) ([]ProjectRecord, error) {
	rows, err := db.Query(`
		SELECT id, slug, path, COALESCE(stack_primary,''), has_agents_md,
		       COALESCE(git_branch,''), git_dirty, COALESCE(last_scan_id,0), last_scan_at
		FROM projects ORDER BY slug`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ProjectRecord
	for rows.Next() {
		var p ProjectRecord
		var lastScanAt sql.NullString
		if err := rows.Scan(&p.ID, &p.Slug, &p.Path, &p.StackPrimary, &p.HasAgentsMD,
			&p.GitBranch, &p.GitDirty, &p.LastScanID, &lastScanAt); err != nil {
			return nil, err
		}
		if lastScanAt.Valid {
			t, err := time.Parse(time.RFC3339, lastScanAt.String)
			if err == nil {
				p.LastScanAt = &t
			}
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// ProjectFacts 返回某项目 + 某次 scan 的全部 facts（KV）。
// scanID <= 0 时取该项目最近一次 scan（last_scan_id）。
func ProjectFacts(db *sql.DB, projectID, scanID int64) (map[string]string, error) {
	if scanID <= 0 {
		err := db.QueryRow(`SELECT last_scan_id FROM projects WHERE id = ?`, projectID).Scan(&scanID)
		if err != nil {
			return nil, err
		}
	}
	rows, err := db.Query(
		`SELECT fact_key, fact_value FROM project_facts WHERE project_id = ? AND scan_id = ?`,
		projectID, scanID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		out[k] = v
	}
	return out, rows.Err()
}
