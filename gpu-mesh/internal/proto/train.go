package proto

// === Phase 5：训练/微调 schema ===

// TrainTask 训练任务载荷。
//
// 8GB 显存约束：只支持 LoRA/QLoRA 小模型微调（如 Qwen2.5-7B Q4 + LoRA）。
// Agent 执行器封装训练框架命令（PEFT/unsloth/axolotl），通过框架名+配置驱动。
type TrainTask struct {
	JobID      string `json:"job_id"`
	Framework  string `json:"framework"`  // "peft" / "unsloth" / "axolotl"
	BaseModel  string `json:"base_model"` // 基座模型（HF id 或本地路径）
	Dataset    string `json:"dataset"`    // 数据集路径（Agent 本地）
	OutputDir  string `json:"output_dir"` // 输出目录（LoRA 权重 + checkpoint）
	// LoRA 参数
	Method     string `json:"method"`     // "lora" / "qlora"，默认 qlora（8GB 友好）
	Rank       int    `json:"rank"`       // LoRA r，默认 8
	Alpha      int    `json:"alpha"`      // LoRA alpha，默认 16
	// 训练超参
	Epochs     int    `json:"epochs"`     // 训练轮数
	BatchSize  int    `json:"batch_size"` // 批大小，默认 1（显存约束）
	LearningRate float64 `json:"learning_rate"`
	MaxSeqLen  int    `json:"max_seq_len"` // 最大序列长度，默认 512
	// 断点续训
	ResumeFrom string `json:"resume_from,omitempty"` // checkpoint 目录，空=从头训
	// 量化（QLoRA 用）
	QuantBits int    `json:"quant_bits,omitempty"` // 4 表示 4bit 量化
}

// TrainResult 训练结果。
type TrainResult struct {
	JobID         string  `json:"job_id"`
	OutputDir     string  `json:"output_dir"`
	FinalLoss     float64 `json:"final_loss"`
	Steps         int     `json:"steps"`
	Duration      int     `json:"duration_s"`
	Paused        bool    `json:"paused"`         // 是否因让位暂停
	CheckpointDir string  `json:"checkpoint_dir"` // 暂停/完成时的 checkpoint
}

// TrainStatus 训练作业进度。
type TrainStatus struct {
	JobID      string  `json:"job_id"`
	Status     string  `json:"status"` // queued/running/paused/completed/failed
	AgentID    string  `json:"agent_id"`
	Step       int     `json:"step"`
	TotalSteps int     `json:"total_steps"`
	Loss       float64 `json:"loss"`
	Lr         float64 `json:"lr"`
	Message    string  `json:"message,omitempty"`
	CreatedAt  int64   `json:"created_at"`
	UpdatedAt  int64   `json:"updated_at"`
}
