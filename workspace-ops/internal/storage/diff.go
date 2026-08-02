package storage

import (
	"database/sql"
	"fmt"
)

// DiffResult 是两次 scan 之间的差异。
type DiffResult struct {
	ScanAID int64
	ScanBID int64
	Added   []string // B 有 A 没有的项目 slug
	Removed []string // A 有 B 没有的项目 slug
	Changed []ProjectChange
}

// ProjectChange 是某个项目在两次 scan 间的变化。
type ProjectChange struct {
	Slug   string
	Field  string // 变化的字段名（如 git_branch / stack_primary / has_agents_md）
	OldVal string
	NewVal string
}

// DiffScans 比较两次 scan 的项目差异。
// scanAID 是旧的，scanBID 是新的。
func DiffScans(db *sql.DB, scanAID, scanBID int64) (*DiffResult, error) {
	// 取两次 scan 各自的项目 slug 集合 + 关键字段
	aProjects, err := scanProjects(db, scanAID)
	if err != nil {
		return nil, fmt.Errorf("查询 scanA=%d 项目失败: %w", scanAID, err)
	}
	bProjects, err := scanProjects(db, scanBID)
	if err != nil {
		return nil, fmt.Errorf("查询 scanB=%d 项目失败: %w", scanBID, err)
	}

	result := &DiffResult{ScanAID: scanAID, ScanBID: scanBID}

	// 新增：B 有 A 没有
	for slug := range bProjects {
		if _, ok := aProjects[slug]; !ok {
			result.Added = append(result.Added, slug)
		}
	}
	// 删除：A 有 B 没有
	for slug := range aProjects {
		if _, ok := bProjects[slug]; !ok {
			result.Removed = append(result.Removed, slug)
		}
	}
	// 变化：两边都有但关键字段不同
	for slug, aInfo := range aProjects {
		bInfo, ok := bProjects[slug]
		if !ok {
			continue
		}
		// 比较关键字段
		fields := []struct{ name, a, b string }{
			{"stack_primary", aInfo.stack, bInfo.stack},
			{"git_branch", aInfo.branch, bInfo.branch},
			{"has_agents_md", aInfo.hasAgents, bInfo.hasAgents},
		}
		for _, f := range fields {
			if f.a != f.b {
				result.Changed = append(result.Changed, ProjectChange{
					Slug: slug, Field: f.name, OldVal: f.a, NewVal: f.b,
				})
			}
		}
	}

	return result, nil
}

// projectInfo 是 diff 用的项目快照（简化）。
type projectInfo struct {
	stack     string
	branch    string
	hasAgents string
}

// scanProjects 取某次 scan 的项目集合（slug → info）。
func scanProjects(db *sql.DB, scanID int64) (map[string]projectInfo, error) {
	rows, err := db.Query(`
		SELECT p.slug, COALESCE(p.stack_primary,''), COALESCE(p.git_branch,''),
		       CASE WHEN p.has_agents_md THEN 'true' ELSE 'false' END
		FROM projects p
		WHERE p.last_scan_id = ?`, scanID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]projectInfo{}
	for rows.Next() {
		var slug, stack, branch, hasAgents string
		if err := rows.Scan(&slug, &stack, &branch, &hasAgents); err != nil {
			return nil, err
		}
		out[slug] = projectInfo{stack: stack, branch: branch, hasAgents: hasAgents}
	}
	return out, rows.Err()
}

// FormatDiff 把 diff 结果格式化成可读文本。
func FormatDiff(d *DiffResult) string {
	if d == nil {
		return "(无 diff 数据)"
	}
	out := fmt.Sprintf("=== Scan %d → %d 差异 ===\n\n", d.ScanAID, d.ScanBID)
	if len(d.Added) > 0 {
		out += fmt.Sprintf("🆕 新增项目 (%d):\n", len(d.Added))
		for _, s := range d.Added {
			out += fmt.Sprintf("  + %s\n", s)
		}
	}
	if len(d.Removed) > 0 {
		out += fmt.Sprintf("\n🗑️ 删除项目 (%d):\n", len(d.Removed))
		for _, s := range d.Removed {
			out += fmt.Sprintf("  - %s\n", s)
		}
	}
	if len(d.Changed) > 0 {
		out += fmt.Sprintf("\n📝 变更 (%d):\n", len(d.Changed))
		for _, c := range d.Changed {
			out += fmt.Sprintf("  ~ %-20s %s: %q → %q\n", c.Slug, c.Field, c.OldVal, c.NewVal)
		}
	}
	if len(d.Added) == 0 && len(d.Removed) == 0 && len(d.Changed) == 0 {
		out += "(无差异)\n"
	}
	return out
}
