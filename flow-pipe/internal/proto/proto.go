// Package proto 定义 flow-pipe 的 worker 协议（M1 仅定义，M3 实装）。
//
// 设计目标（M3 分布式）：多个 worker 通过反向 WebSocket 连接中心调度器（server），
// server 把管道的某些步骤（source/sink 的 IO 密集部分）分发给远程 worker 执行。
// 协议参考 go-rmm 的 relay/agent 反向连接模式。
//
// M3 路线图：
//
//	┌─────────┐   反向 WS    ┌─────────┐
//	│ worker  │ ───────────▶ │  server │
//	│ (N 个)  │ ◀─────────── │ (调度器) │
//	└─────────┘   task_assign └─────────┘
//
//	 1. worker 启动 → 反向 WS 连接 server → 发 MsgRegister
//	 2. server 收到管道任务 → 按 worker 负载分发 MsgTaskAssign
//	 3. worker 执行步骤（source.Read / transform.Transform / sink.Write）
//	 4. worker 回报 MsgTaskResult
//	 5. server 聚合结果，推进管道
//
// M1 状态：管道在 server 进程内单机执行（pipeline.Run），本包只有协议结构定义，
// 不连真实网络，避免过度设计。M3 时实装 transport 层（真 WS 传输）。
package proto

// MessageType 标识 worker 与 server 之间的消息类型。
type MessageType string

const (
	MsgRegister   MessageType = "register"    // worker 上线注册（携带 ID/能力）
	MsgHeartbeat  MessageType = "heartbeat"   // worker 心跳（保活 + 负载上报）
	MsgTaskAssign MessageType = "task_assign" // server 给 worker 派发任务
	MsgTaskResult MessageType = "task_result" // worker 回报任务结果
	MsgTaskCancel MessageType = "task_cancel" // server 取消任务
)

// Envelope 是消息信封（所有消息的通用外壳）。
type Envelope struct {
	Type     MessageType `json:"type"`
	TaskID   string      `json:"task_id,omitempty"`
	WorkerID string      `json:"worker_id,omitempty"`
	Payload  any         `json:"payload,omitempty"`
}

// TaskPayload 是 server 派发给 worker 的任务负载（M3 用）。
// 一个任务 = 一个管道步骤的执行（如 source.Read 或 transform.Transform）。
type TaskPayload struct {
	PipelineName string           `json:"pipeline_name"`
	StepID       string           `json:"step_id"`
	StepKind     string           `json:"step_kind"` // source/transform/sink
	Connector    string           `json:"connector"` // csv/filter/sqlite
	Config       map[string]any   `json:"config"`
	InputRows    []map[string]any `json:"input_rows,omitempty"` // transform/sink 的输入
}

// TaskResultPayload 是 worker 回报的任务结果。
type TaskResultPayload struct {
	TaskID     string           `json:"task_id"`
	Success    bool             `json:"success"`
	OutputRows []map[string]any `json:"output_rows,omitempty"`
	Error      string           `json:"error,omitempty"`
	DurationMs int64            `json:"duration_ms"`
}

// WorkerInfo 是 worker 注册时上报的自身信息。
type WorkerInfo struct {
	ID            string   `json:"id"`
	Capabilities  []string `json:"capabilities"` // 支持的连接器类型
	MaxConcurrent int      `json:"max_concurrent"`
}
