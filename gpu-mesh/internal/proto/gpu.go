package proto

// GPUSnapshot 一张 GPU 在某时刻的状态快照。
//
// 采集自 nvidia-smi（零 CGO，命令解析）。这是 gpu-mesh 的核心数据结构：
//   - 调度器据 UtilGPU / MemUsedMB 选节点（Phase 3）
//   - 仪表盘据它做可视化（Phase 1）
//   - 让位检测器结合它判断是否有人在用（Phase 1 埋点 / Phase 3 执行）
type GPUSnapshot struct {
	Index    int     `json:"index"`              // GPU 序号（从 0 开始）
	Name     string  `json:"name"`               // 如 "NVIDIA GeForce RTX 4060"
	UtilGPU  float64 `json:"util_gpu"`           // GPU 计算利用率 %（0-100）★调度核心指标
	UtilMem  float64 `json:"util_mem"`           // 显存控制器利用率 %
	MemUsedMB int    `json:"mem_used_mb"`        // 已用显存 MB
	MemTotalMB int   `json:"mem_total_mb"`       // 显存总量 MB（4060 = 8192）
	TempC    int     `json:"temp_c"`             // 温度 ℃
	PowerW   float64 `json:"power_w"`            // 实时功耗 W
	PowerLimitW float64 `json:"power_limit_w"`   // 功耗墙 W
	Models   []string `json:"models,omitempty"`  // 显存中已加载的模型（Phase 3 模型路由用）
	TS       int64   `json:"ts"`                 // 采集时间戳 Unix 毫秒
}

// 让位档位常量。
const (
	// YieldIDLE：机器空闲（idle>5min 且外部 GPU<20%），算力上限 100%，全力跑。
	YieldIDLE = "idle"
	// YieldACTIVE：机器有轻微活动（idle 60s~5min），算力上限 50%，降并发/降量化。
	YieldACTIVE = "active"
	// YieldBUSY：机器被人主动使用（idle<60s 或外部 GPU>50%），算力上限 10%，只跑轻量或暂停。
	YieldBUSY = "busy_human"
)

// BudgetForLevel 按档位返回算力配额百分比。
func BudgetForLevel(level string) int {
	switch level {
	case YieldIDLE:
		return 100
	case YieldACTIVE:
		return 50
	case YieldBUSY:
		return 10
	default:
		return 100
	}
}

// YieldState 让位状态——决定本机当前能给多少算力给集群任务。
//
// 设计为 Agent 本地自治（本地反应最快，不依赖 Relay 往返）：
//   - Agent 周期采样 GetLastInputInfo（用户空闲时间）+ nvidia-smi 外部占用 + 前台窗口
//   - 据综合活跃度算出 Level（IDLE/ACTIVE/BUSY）和 Budget（算力配额 %）
//   - Phase 1：只采集上报（仪表盘可见），不执行降级
//   - Phase 3：据 Budget 调整引擎并发/上下文；进 BUSY 主动 NACK 低优先级任务触发重调度
type YieldState struct {
	IdleSeconds    int     `json:"idle_seconds"`     // 用户空闲秒数（GetLastInputInfo）
	HumanActivity  float64 `json:"human_activity"`   // 综合活跃度 0-1（窗口抖动+进程命中+外部GPU）
	ExternalUtilGPU float64 `json:"external_util_gpu"` // 扣除本 Agent 进程后的外部 GPU 占用 %
	Budget         int     `json:"budget"`           // 当前算力配额 %（100/50/10）
	Level          string  `json:"level"`            // IDLE/ACTIVE/BUSY_HUMAN
}

// AgentRegister Agent 连接建立后发送，登记身份与能力。
type AgentRegister struct {
	AgentID  string            `json:"agent_id"`  // Agent 唯一 ID
	Hostname string            `json:"hostname"`  // 主机名
	OS       string            `json:"os"`        // runtime.GOOS/runtime.GOARCH
	Version  string            `json:"version"`   // Agent 二进制版本
	GPUs     []GPUSnapshot     `json:"gpus"`      // ★ 多卡支持
	Engines  []string          `json:"engines"`   // ★ 探测到的引擎 ["ollama","llamacpp"]
	Models   []string          `json:"models"`    // ★ 已下载的模型列表
	Yield    YieldState        `json:"yield"`     // ★ 让位状态
	Tags     map[string]string `json:"tags,omitempty"` // 业务标签 region=bj / gpu=4060
}

// AgentHeartbeat 心跳载荷，携带最新 GPU 快照与让位状态（每次心跳都刷新）。
type AgentHeartbeat struct {
	AgentID string        `json:"agent_id"`
	GPUs    []GPUSnapshot `json:"gpus"` // ★ 最新 GPU 快照
	Yield   YieldState    `json:"yield"` // ★ 最新让位状态
	Seq     int64         `json:"seq"`   // 递增序号，检测丢包
}
