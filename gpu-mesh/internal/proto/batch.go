package proto

import "encoding/json"

// === Phase 4：批量离线推理（Map-Reduce）schema ===

// BatchTask 批量任务载荷（分发给单个 Agent 执行一个分片）。
type BatchTask struct {
	BatchID  string        `json:"batch_id"`  // 整个批量作业的 ID（多个分片共享）
	ShardID  string        `json:"shard_id"`  // 本分片 ID
	Engine   string        `json:"engine,omitempty"`
	Model    string        `json:"model"`
	TaskType string        `json:"task_type"` // "chat" / "embed"
	// chat 任务：每条输入转成一个 messages（user 角色放 content）
	Items []string `json:"items"`
	// embed 任务的嵌入模型（可与 chat model 不同）
	EmbedModel string `json:"embed_model,omitempty"`
	// 采样参数（仅 chat）
	Options *GenOptions `json:"options,omitempty"`
	MaxTokens int        `json:"max_tokens,omitempty"`
}

// BatchShardResult 一个分片的结果。
type BatchShardResult struct {
	BatchID string `json:"batch_id"`
	ShardID string `json:"shard_id"`
	// chat: results[i] 对应 items[i] 的生成文本
	// embed: embeddings[i] 对应 items[i] 的向量
	Results    []string    `json:"results,omitempty"`
	Embeddings [][]float32 `json:"embeddings,omitempty"`
	Succeeded  int         `json:"succeeded"`
	Failed     int         `json:"failed"`
	Errors     []string    `json:"errors,omitempty"` // 每个失败项的错误
}

// BatchSpec 批量作业定义（提交给 Relay，由编排器分片）。
type BatchSpec struct {
	BatchID   string `json:"batch_id"`            // 空 = 自动生成
	Engine    string `json:"engine,omitempty"`
	Model     string `json:"model"`               // chat 模型
	TaskType  string `json:"task_type"`           // "chat"/"embed"
	Items     []string `json:"items"`             // 全量输入
	ShardSize int    `json:"shard_size,omitempty"` // 每分片条数，默认 20
	MaxTokens int    `json:"max_tokens,omitempty"`
	// MinBudget=100 表示批量任务只在 IDLE 节点跑（让位友好）
	MinBudget int `json:"min_budget,omitempty"`
}

// BatchStatus 批量作业进度。
type BatchStatus struct {
	BatchID    string `json:"batch_id"`
	Total      int    `json:"total"`       // 总分片数
	Completed  int    `json:"completed"`   // 已完成分片
	Failed     int    `json:"failed"`      // 失败分片
	Processing int    `json:"processing"`  // 执行中
	Pending    int    `json:"pending"`     // 待分发
	TotalItems int    `json:"total_items"` // 总条目数
	DoneItems  int    `json:"done_items"`  // 已处理条目
	Status     string `json:"status"`      // pending/running/completed/failed
	CreatedAt  int64  `json:"created_at"`
	FinishedAt int64  `json:"finished_at,omitempty"`
	// Reduce 结果
	Results    []string    `json:"results,omitempty"`     // chat 聚合结果
	Embeddings [][]float32 `json:"embeddings,omitempty"`  // embed 聚合向量
}

// MarshalBatchPayload 把 BatchTask 序列化为 TaskRequest.Payload。
func MarshalBatchPayload(b BatchTask) json.RawMessage {
	out, _ := json.Marshal(b)
	return out
}
