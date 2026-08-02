package storage

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/QiuShichang/flow-pipe/internal/pipeline"
)

// Run 是 runs 表的行记录（一次管道执行的持久化结果）。
type Run struct {
	ID           int64
	PipelineID   sql.NullInt64
	PipelineName string
	StartedAt    time.Time
	FinishedAt   time.Time
	Status       string                // ok / error
	Steps        []pipeline.StepResult // 反序列化自 steps_json（可能为 nil）
	Err          string                // error 为空时为 ""，避免 nil 指针序列化问题
}

// stepResultJSON 是 pipeline.StepResult 的可序列化投影（Err error 不能直接 JSON）。
type stepResultJSON struct {
	StepID       string `json:"step_id"`
	Kind         string `json:"kind"`
	RowsIn       int    `json:"rows_in"`
	RowsOut      int    `json:"rows_out"`
	DeadLettered int    `json:"dead_lettered,omitempty"` // 死信行数（retry+死信兜底时 >0）
	Duration     string `json:"duration"`                // time.Duration.String()
	Err          string `json:"err,omitempty"`
	Skipped      bool   `json:"skipped,omitempty"` // 状态恢复时跳过（partial rerun）
}

// SaveRun 把一次 pipeline.RunResult 持久化到 runs 表。
// pipelineName 可能为空（直接跑 YAML 未存库时）。pipelineID <= 0 时存 NULL。
func SaveRun(db *sql.DB, rr *pipeline.RunResult) (int64, error) {
	if rr == nil {
		return 0, fmt.Errorf("RunResult 为空")
	}

	stepsJSON := encodeSteps(rr.Steps)
	status := "ok"
	errMsg := ""
	if rr.Err != nil {
		status = "error"
		errMsg = rr.Err.Error()
	}

	started := rr.StartedAt.UTC().Format(time.RFC3339Nano)
	finished := rr.FinishedAt.UTC().Format(time.RFC3339Nano)

	// 关联同名管道的 id（若存在）。
	var pid sql.NullInt64
	if rr.PipelineName != "" {
		if id, err := pipelineIDByName(db, rr.PipelineName); err == nil && id > 0 {
			pid = sql.NullInt64{Int64: id, Valid: true}
		}
	}

	res, err := db.Exec(
		`INSERT INTO runs(pipeline_id, pipeline_name, started_at, finished_at, status, steps_json, error)
		 VALUES(?, ?, ?, ?, ?, ?, ?)`,
		pid, rr.PipelineName, started, finished, status, stepsJSON, errMsg,
	)
	if err != nil {
		return 0, fmt.Errorf("保存运行记录失败: %w", err)
	}
	id, _ := res.LastInsertId()
	return id, nil
}

// AllRuns 返回最近的运行历史（按 started_at 倒序，limit<=0 时默认 50）。
func AllRuns(db *sql.DB, limit int) ([]Run, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := db.Query(
		`SELECT id, pipeline_id, pipeline_name, started_at, finished_at, status, steps_json, error
		 FROM runs ORDER BY started_at DESC LIMIT ?`,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("查询运行历史失败: %w", err)
	}
	defer rows.Close()

	out := make([]Run, 0)
	for rows.Next() {
		var r Run
		var started, finished, stepsJSON, errStr sql.NullString
		if err := rows.Scan(&r.ID, &r.PipelineID, &r.PipelineName, &started, &finished, &r.Status, &stepsJSON, &errStr); err != nil {
			return nil, err
		}
		if started.Valid {
			r.StartedAt = parseTime(started.String)
		}
		if finished.Valid {
			r.FinishedAt = parseTime(finished.String)
		}
		if stepsJSON.Valid && stepsJSON.String != "" {
			r.Steps = decodeSteps(stepsJSON.String)
		}
		if errStr.Valid {
			r.Err = errStr.String
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// encodeSteps 把 StepResult 切片序列化成 JSON（duration/err 转成可读字符串）。
func encodeSteps(steps []pipeline.StepResult) string {
	if len(steps) == 0 {
		return "[]"
	}
	out := make([]stepResultJSON, 0, len(steps))
	for _, s := range steps {
		errStr := ""
		if s.Err != nil {
			errStr = s.Err.Error()
		}
		out = append(out, stepResultJSON{
			StepID:       s.StepID,
			Kind:         string(s.Kind),
			RowsIn:       s.RowsIn,
			RowsOut:      s.RowsOut,
			DeadLettered: s.DeadLettered,
			Duration:     s.Duration.String(),
			Err:          errStr,
			Skipped:      s.Skipped,
		})
	}
	b, _ := json.Marshal(out)
	return string(b)
}

// decodeSteps 把 steps_json 反序列化回 StepResult 切片（duration 从字符串解析）。
func decodeSteps(raw string) []pipeline.StepResult {
	var js []stepResultJSON
	if err := json.Unmarshal([]byte(raw), &js); err != nil {
		return nil
	}
	out := make([]pipeline.StepResult, 0, len(js))
	for _, s := range js {
		d, _ := time.ParseDuration(s.Duration)
		sr := pipeline.StepResult{
			StepID:       s.StepID,
			Kind:         pipeline.StepKind(s.Kind),
			RowsIn:       s.RowsIn,
			RowsOut:      s.RowsOut,
			DeadLettered: s.DeadLettered,
			Duration:     d,
			Skipped:      s.Skipped,
		}
		if s.Err != "" {
			sr.Err = fmt.Errorf("%s", s.Err)
		}
		out = append(out, sr)
	}
	return out
}

// parseTime 容错解析 RFC3339（含 Nano）字符串。
func parseTime(s string) time.Time {
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t
		}
	}
	return time.Time{}
}

// LatestSuccessfulSteps 返回某管道最近一次成功完成的步骤 ID 列表。
// 用于状态恢复：重跑时跳过这些已完成的步骤。
//
// 判定逻辑：取该管道最近的若干条 run（按 started_at 倒序），逐条扫描其 steps_json。
// 返回**最近一条包含至少一个成功步骤**的 run 里，所有未报错（Err 为空）且未被跳过
// （Skipped=false）的步骤 ID。这保证恢复的步骤集来自同一次连贯的运行，
// 而非多次拼凑；若最近一次整体失败但前几步成功，则用前几步成功的那次。
// 若所有 run 都无成功步骤，返回 nil（无可恢复状态）。
//
// pipelineName 为空时返回 nil。
func LatestSuccessfulSteps(db *sql.DB, pipelineName string) ([]string, error) {
	if pipelineName == "" {
		return nil, nil
	}
	// 拉足够多的历史（500 条）覆盖长期运行的管道；按时间倒序。
	runs, err := runsByName(db, pipelineName, 500)
	if err != nil {
		return nil, fmt.Errorf("查询 %s 运行历史失败: %w", pipelineName, err)
	}

	// 找最近一条"有任何成功步骤"的 run，返回它全部的成功步骤。
	// 这保证恢复的步骤集来自同一次连贯的运行，符合"从上次成功的地方继续"的直觉。
	for _, r := range runs { // runs 已按 started_at 倒序
		var ok []string
		for _, s := range r.Steps {
			if !s.Skipped && s.Err == nil {
				ok = append(ok, s.StepID)
			}
		}
		if len(ok) > 0 {
			return ok, nil
		}
	}
	return nil, nil
}

// runsByName 查询某管道最近的 limit 条 run（按 started_at 倒序）。
// 复用 AllRuns 的行解析，但按 pipeline_name 过滤。
func runsByName(db *sql.DB, pipelineName string, limit int) ([]Run, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := db.Query(
		`SELECT id, pipeline_id, pipeline_name, started_at, finished_at, status, steps_json, error
		 FROM runs WHERE pipeline_name = ? ORDER BY started_at DESC LIMIT ?`,
		pipelineName, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]Run, 0)
	for rows.Next() {
		var r Run
		var started, finished, stepsJSON, errStr sql.NullString
		if err := rows.Scan(&r.ID, &r.PipelineID, &r.PipelineName, &started, &finished, &r.Status, &stepsJSON, &errStr); err != nil {
			return nil, err
		}
		if started.Valid {
			r.StartedAt = parseTime(started.String)
		}
		if finished.Valid {
			r.FinishedAt = parseTime(finished.String)
		}
		if stepsJSON.Valid && stepsJSON.String != "" {
			r.Steps = decodeSteps(stepsJSON.String)
		}
		if errStr.Valid {
			r.Err = errStr.String
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
