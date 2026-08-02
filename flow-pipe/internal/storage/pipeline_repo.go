package storage

import (
	"database/sql"
	"fmt"
	"time"
)

// Pipeline 是 pipelines 表的行记录（持久化的管道定义）。
type Pipeline struct {
	ID             int64
	Name           string
	DefinitionYAML string
	CreatedAt      time.Time
}

// SavePipeline 保存（或覆盖）一个管道定义。按 name 唯一：已存在则更新 definition_yaml 与 created_at。
func SavePipeline(db *sql.DB, name, definitionYAML string) (int64, error) {
	if name == "" {
		return 0, fmt.Errorf("管道名不能为空")
	}
	now := time.Now().UTC().Format(time.RFC3339)

	// 先尝试插入；name 冲突时更新已有行。
	res, err := db.Exec(
		`INSERT INTO pipelines(name, definition_yaml, created_at) VALUES(?, ?, ?)
		 ON CONFLICT(name) DO UPDATE SET definition_yaml=excluded.definition_yaml, created_at=excluded.created_at`,
		name, definitionYAML, now,
	)
	if err != nil {
		return 0, fmt.Errorf("保存管道失败 name=%s: %w", name, err)
	}
	id, _ := res.LastInsertId()
	// ON CONFLICT 更新时 LastInsertId 返回旧行 id（sqlite 重用），改为按 name 查回准确 id。
	if id == 0 {
		id, _ = pipelineIDByName(db, name)
	}
	return id, nil
}

// GetPipeline 按名字读取一个管道定义。不存在返回 (nil, nil)。
func GetPipeline(db *sql.DB, name string) (*Pipeline, error) {
	row := db.QueryRow(
		`SELECT id, name, definition_yaml, created_at FROM pipelines WHERE name = ?`, name,
	)
	var p Pipeline
	var created string
	if err := row.Scan(&p.ID, &p.Name, &p.DefinitionYAML, &created); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("查询管道失败 name=%s: %w", name, err)
	}
	p.CreatedAt = parseTime(created)
	return &p, nil
}

// AllPipelines 返回全部管道定义（按 name 升序）。
func AllPipelines(db *sql.DB) ([]Pipeline, error) {
	rows, err := db.Query(
		`SELECT id, name, definition_yaml, created_at FROM pipelines ORDER BY name ASC`,
	)
	if err != nil {
		return nil, fmt.Errorf("查询管道列表失败: %w", err)
	}
	defer rows.Close()

	out := make([]Pipeline, 0)
	for rows.Next() {
		var p Pipeline
		var created string
		if err := rows.Scan(&p.ID, &p.Name, &p.DefinitionYAML, &created); err != nil {
			return nil, err
		}
		p.CreatedAt = parseTime(created)
		out = append(out, p)
	}
	return out, rows.Err()
}

func pipelineIDByName(db *sql.DB, name string) (int64, error) {
	var id int64
	err := db.QueryRow(`SELECT id FROM pipelines WHERE name = ?`, name).Scan(&id)
	return id, err
}
