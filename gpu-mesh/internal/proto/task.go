package proto

import "encoding/json"

// TaskRequest Relay→Agent：下发任务。
type TaskRequest struct {
	TaskID   string          `json:"task_id"`          // 任务唯一 ID
	Type     string          `json:"type"`             // 任务类型，取 Task* 常量
	AgentID  string          `json:"agent_id"`         // 目标 Agent ID
	Payload  json.RawMessage `json:"payload"`          // 按 Type 反序列化
	Timeout  int             `json:"timeout"`          // 秒，0 用 DefaultTaskTimeout
	Priority int             `json:"priority,omitempty"` // 0-9，Phase 3 调度用
	// MinBudget 要求的最低算力配额 %。Agent 当前 Yield.Budget < MinBudget 时应 NACK。
	// 让位协作：低优先级批量任务设 MinBudget=100（只在 IDLE 跑），
	// 高优先级推理设 MinBudget=10（IDLE/ACTIVE/BUSY 都可跑）。
	MinBudget int `json:"min_budget,omitempty"`
}

// TaskResult 任务终态结果。
type TaskResult struct {
	TaskID   string          `json:"task_id"`
	Success  bool            `json:"success"`
	Stdout   string          `json:"stdout,omitempty"`
	Stderr   string          `json:"stderr,omitempty"`
	ExitCode int             `json:"exit_code"`
	Data     json.RawMessage `json:"data,omitempty"` // 结构化结果
	Duration int             `json:"duration_ms"`
	Error    string          `json:"error,omitempty"`
}

// TaskProgress 长任务流式进度。
type TaskProgress struct {
	TaskID  string `json:"task_id"`
	Step    string `json:"step"`
	Message string `json:"message"`
	Percent int    `json:"percent"`
}

// TaskNack Agent 拒绝任务。Relay 收到后应重调度到其他节点。
type TaskNack struct {
	TaskID string `json:"task_id"`
	Reason string `json:"reason"` // yield_busy / vram_insufficient / unknown
}

// TaskCancel 取消任务。
type TaskCancel struct {
	TaskID string `json:"task_id"`
	Reason string `json:"reason,omitempty"`
}

// --- Phase 1 诊断任务载荷（验证链路用）---

// DiagTask 诊断命令载荷。Phase 1 用于验证任务闭环（如执行 nvidia-smi/hostname）。
type DiagTask struct {
	Command string `json:"command"` // shell 命令
}
