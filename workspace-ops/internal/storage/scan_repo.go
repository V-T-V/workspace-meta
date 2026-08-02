package storage

import (
	"database/sql"
	"fmt"
	"time"
)

// Scan 记录一次扫描。
type Scan struct {
	ID           int64
	StartedAt    time.Time
	FinishedAt   *time.Time
	ProjectCount int
	Status       string // running / done / failed
}

// StartScan 创建一条 status=running 的 scan 记录，返回其 ID。
func StartScan(db *sql.DB) (int64, error) {
	res, err := db.Exec(
		`INSERT INTO scans(started_at, status) VALUES (?, 'running')`,
		time.Now().UTC().Format(time.RFC3339),
	)
	if err != nil {
		return 0, fmt.Errorf("创建 scan 记录失败: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	return id, nil
}

// FinishScan 把 scan 标记为完成，写入 project_count 和 status。
func FinishScan(db *sql.DB, scanID int64, projectCount int, status string) error {
	_, err := db.Exec(
		`UPDATE scans SET finished_at = ?, project_count = ?, status = ? WHERE id = ?`,
		time.Now().UTC().Format(time.RFC3339),
		projectCount, status, scanID,
	)
	return err
}

// LatestScan 返回最近一次 scan（按 id 降序）。没有则返回 nil。
func LatestScan(db *sql.DB) (*Scan, error) {
	row := db.QueryRow(`
		SELECT id, started_at, finished_at, project_count, status
		FROM scans ORDER BY id DESC LIMIT 1`)
	var s Scan
	var started, finished sql.NullString
	err := row.Scan(&s.ID, &started, &finished, &s.ProjectCount, &s.Status)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if started.Valid {
		s.StartedAt, _ = time.Parse(time.RFC3339, started.String)
	}
	if finished.Valid {
		t, err := time.Parse(time.RFC3339, finished.String)
		if err == nil {
			s.FinishedAt = &t
		}
	}
	return &s, nil
}

// AllScans 返回全部 scan 记录（按 id 降序）。
func AllScans(db *sql.DB) ([]Scan, error) {
	rows, err := db.Query(`
		SELECT id, started_at, finished_at, project_count, status
		FROM scans ORDER BY id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Scan
	for rows.Next() {
		var s Scan
		var started, finished sql.NullString
		if err := rows.Scan(&s.ID, &started, &finished, &s.ProjectCount, &s.Status); err != nil {
			return nil, err
		}
		if started.Valid {
			s.StartedAt, _ = time.Parse(time.RFC3339, started.String)
		}
		if finished.Valid {
			t, err := time.Parse(time.RFC3339, finished.String)
			if err == nil {
				s.FinishedAt = &t
			}
		}
		out = append(out, s)
	}
	return out, rows.Err()
}
