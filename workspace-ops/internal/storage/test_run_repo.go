package storage

import (
	"database/sql"
	"fmt"
	"time"
)

// TestRunRecord 是 test_runs 表的一行。
type TestRunRecord struct {
	ID         int64
	ProjectID  int64
	ScanID     int64
	Status     string // pass / fail / skipped / timeout / error
	Command    string
	DurationMs int64
	OutputTail string
	RanAt      time.Time
}

// SaveTestRun 保存一条测试运行结果。
func SaveTestRun(db *sql.DB, projectID, scanID int64, status, command string, duration time.Duration, outputTail string) (int64, error) {
	res, err := db.Exec(`
		INSERT INTO test_runs(project_id, scan_id, status, command, duration_ms, output_tail, ran_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		projectID, scanID, status, command, duration.Milliseconds(), outputTail, time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		return 0, fmt.Errorf("保存 test_run 失败: %w", err)
	}
	return res.LastInsertId()
}

// AllTestRuns 返回全部测试运行记录（按 id 降序，限制 limit 条）。
func AllTestRuns(db *sql.DB, limit int) ([]TestRunRecord, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := db.Query(`
		SELECT tr.id, tr.project_id, tr.scan_id, tr.status, COALESCE(tr.command,''),
		       COALESCE(tr.duration_ms,0), COALESCE(tr.output_tail,''), tr.ran_at
		FROM test_runs tr
		ORDER BY tr.id DESC
		LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []TestRunRecord
	for rows.Next() {
		var r TestRunRecord
		var ranAt string
		if err := rows.Scan(&r.ID, &r.ProjectID, &r.ScanID, &r.Status, &r.Command,
			&r.DurationMs, &r.OutputTail, &ranAt); err != nil {
			return nil, err
		}
		r.RanAt, _ = time.Parse(time.RFC3339, ranAt)
		out = append(out, r)
	}
	return out, rows.Err()
}

// LatestTestRunForProject 返回某项目最近一次测试运行（按 id 降序）。无则返回 nil。
func LatestTestRunForProject(db *sql.DB, projectID int64) (*TestRunRecord, error) {
	row := db.QueryRow(`
		SELECT tr.id, tr.project_id, tr.scan_id, tr.status, COALESCE(tr.command,''),
		       COALESCE(tr.duration_ms,0), COALESCE(tr.output_tail,''), tr.ran_at
		FROM test_runs tr
		WHERE tr.project_id = ?
		ORDER BY tr.id DESC LIMIT 1`, projectID)
	var r TestRunRecord
	var ranAt string
	err := row.Scan(&r.ID, &r.ProjectID, &r.ScanID, &r.Status, &r.Command,
		&r.DurationMs, &r.OutputTail, &ranAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	r.RanAt, _ = time.Parse(time.RFC3339, ranAt)
	return &r, nil
}
