// Package proto 定义 gpu-mesh 三组件（relay / agent / web 控制台）共享的通信协议。
//
// 所有 WebSocket 与 HTTP 消息都使用统一的 Envelope 信封包装，通过 Type 字段区分种类，
// Payload 为原始 JSON，由收件方按 Type 反序列化为对应结构体。
//
// 消息流向：
//
//	Agent  ──register/heartbeat/task_result/task_progress──►  Relay
//	Agent  ◄──────────────── task_request ──────────────────  Relay
//	Web    ──(HTTP POST /api/tasks) task_request────────────►  Relay
//	Web    ◄───(SSE) task_result/task_progress/agent_*──────  Relay
package proto

import (
	"encoding/json"
	"time"
)

// 协议版本
const Version = 1

// 消息类型常量。
const (
	TypeRegister     = "register"      // Agent→Relay：连接建立后首条消息，登记身份/GPU/引擎/让位状态
	TypeHeartbeat    = "heartbeat"     // Agent→Relay：周期心跳，携带最新 GPU 快照与让位状态
	TypeTaskRequest  = "task_request"  // Relay→Agent：下发任务
	TypeTaskResult   = "task_result"   // Agent→Relay：任务终态结果
	TypeTaskProgress = "task_progress" // Agent→Relay：长任务流式进度
	TypeTaskCancel   = "task_cancel"   // Relay→Agent：取消任务
	TypeTaskNack     = "task_nack"     // Agent→Relay：拒绝任务（让位降级/显存不足等），Relay 应重调度
	TypeAgentOnline  = "agent_online"  // Relay→Web：Agent 上线通知
	TypeAgentOffline = "agent_offline" // Relay→Web：Agent 离线通知
)

// 任务类型常量。Phase 1 暂只用 powershell（诊断）；Phase 2+ 扩展。
const (
	TaskInference = "inference" // Phase 2 LLM 推理
	TaskBatch     = "batch"     // Phase 4 批量离线
	TaskTrain     = "train"     // Phase 5 训练/微调
	TaskDiag      = "diag"      // 诊断命令（Phase 1 占位，用于验证链路）
)

// ValidTaskTypes 已知任务类型集合（Relay 侧 type 校验用）。
var ValidTaskTypes = map[string]bool{
	TaskInference: true,
	TaskBatch:     true,
	TaskTrain:     true,
	TaskDiag:      true,
}

// IsValidTaskType 判断是否为已知任务类型。
func IsValidTaskType(t string) bool { return ValidTaskTypes[t] }

// DefaultTaskTimeout 默认任务超时（秒）。
const DefaultTaskTimeout = 300

// Envelope 是所有消息的统一信封。
//
// 设计要点：
//   - Payload 用 json.RawMessage 延迟解码，允许 Relay 仅做路由而不关心具体载荷。
//   - To 为 "*" 时表示广播（仅用于 agent_online/offline 等通知）。
//   - ID 由发送方生成（UUIDv4），ReplyTo 用于结果/进度回填对应请求。
type Envelope struct {
	Version int             `json:"v"`                 // 协议版本，当前 1
	Type    string          `json:"type"`              // 消息类型，取 Type* 常量
	From    string          `json:"from"`              // 发送方 ID（agent-xxx / relay）
	To      string          `json:"to,omitempty"`      // 接收方 ID；"*" 表示广播
	ID      string          `json:"id"`                // 消息 ID
	ReplyTo string          `json:"reply_to,omitempty"` // 对应请求的 ID
	Payload json.RawMessage `json:"payload,omitempty"` // 按 Type 反序列化
	TS      int64           `json:"ts"`                // Unix 毫秒时间戳
}

// NewEnvelope 构造一个带默认字段的信封。payload 可为 nil。
func NewEnvelope(typ, from, to string, payload any) (Envelope, error) {
	env := Envelope{
		Version: Version,
		Type:    typ,
		From:    from,
		To:      to,
		ID:      uuidString(),
		TS:      time.Now().UnixMilli(),
	}
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			return Envelope{}, err
		}
		env.Payload = raw
	}
	return env, nil
}

// Decode 把信封的 Payload 反序列化到 v。
func (e Envelope) Decode(v any) error {
	if len(e.Payload) == 0 {
		return nil
	}
	return json.Unmarshal(e.Payload, v)
}
