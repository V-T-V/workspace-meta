package agent

import (
	"context"
	"sync"
	"time"

	"github.com/QiuShichang/gpu-mesh/internal/proto"
)

// YieldDetector 让位检测器——决定本机当前能给集群多少算力。
//
// 这是 gpu-mesh 的核心运维约束："机器有人使用时，主动降低算力占用占比"。
// 设计为 Agent 本地自治（本地反应最快，不依赖 Relay 往返）：
//
// 检测维度（三路信号融合）：
//  1. 用户空闲时间：GetLastInputInfo() —— 人多久没动键鼠（最直接信号）
//  2. 外部 GPU 占用：GPU 总利用率 - 本 Agent 进程占用（有人开游戏/渲染/本机推理）
//  3. 前台窗口抖动：周期采样 GetForegroundWindow() 变化（人在主动切窗口）
//
// 输出三档状态：
//   - IDLE      (idle>5min 且 外部GPU<20%)  → Budget 100% 全力跑
//   - ACTIVE    (idle 60s~5min)             → Budget 50%  降并发
//   - BUSY_HUMAN(idle<60s 或 外部GPU>50%)   → Budget 10%  只跑轻量
//
// Phase 1：只采集上报（仪表盘可见），不执行降级动作。
// Phase 3：据 Budget 调整引擎 num_parallel；进 BUSY 主动 NACK 低优先级任务。
type YieldDetector struct {
	interval time.Duration

	mu   sync.RWMutex
	last proto.YieldState

	cancel context.CancelFunc
	done   chan struct{}
}

// NewYieldDetector 构造让位检测器。interval 建议与心跳同步（5s）。
func NewYieldDetector(interval time.Duration) *YieldDetector {
	return &YieldDetector{interval: interval, done: make(chan struct{})}
}

// Start 启动后台检测循环。
func (y *YieldDetector) Start() {
	ctx, cancel := context.WithCancel(context.Background())
	y.cancel = cancel
	y.detectOnce(ctx) // 立即采集一次
	go y.loop(ctx)
}

// Stop 停止检测。
func (y *YieldDetector) Stop() {
	if y.cancel != nil {
		y.cancel()
	}
	<-y.done
}

func (y *YieldDetector) loop(ctx context.Context) {
	defer close(y.done)
	ticker := time.NewTicker(y.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			y.detectOnce(ctx)
		}
	}
}

// detectOnce 采集一次让位信号，融合计算 YieldState。
func (y *YieldDetector) detectOnce(ctx context.Context) {
	idleSec := getIdleSeconds()
	foregroundChanged := foregroundChangedSinceLast()
	externalGPU := externalGPUUtil()

	// 综合活跃度：空闲越久活动越低；前台窗口在变 + 有外部 GPU 占用 → 活动越高
	activity := 0.0
	if idleSec < 30 {
		activity += 0.5 // 近半分钟内动过键鼠
	}
	if foregroundChanged {
		activity += 0.2
	}
	if externalGPU > 20 {
		activity += float64(externalGPU) / 100.0
	}
	if activity > 1 {
		activity = 1
	}

	level := classifyLevel(idleSec, externalGPU, activity)
	st := proto.YieldState{
		IdleSeconds:     idleSec,
		HumanActivity:   activity,
		ExternalUtilGPU: externalGPU,
		Budget:          proto.BudgetForLevel(level),
		Level:           level,
	}
	y.mu.Lock()
	y.last = st
	y.mu.Unlock()
}

// State 返回最近一次让位状态快照。
func (y *YieldDetector) State() proto.YieldState {
	y.mu.RLock()
	defer y.mu.RUnlock()
	return y.last
}

// classifyLevel 据三路信号算档位。
//
// 判定优先级（最保守优先）：
//  1. externalGPU > 50 → BUSY_HUMAN（明确有人在用 GPU，如游戏/渲染）
//  2. idle < 60s      → BUSY_HUMAN（刚动过键鼠）
//  3. idle < 5min     → ACTIVE（近 5 分钟有活动迹象）
//  4. 其余            → IDLE
func classifyLevel(idleSec int, externalGPU float64, activity float64) string {
	if externalGPU > 50 {
		return proto.YieldBUSY
	}
	if idleSec < 60 {
		return proto.YieldBUSY
	}
	if idleSec < 300 {
		return proto.YieldACTIVE
	}
	return proto.YieldIDLE
}

// —— 平台相关原语，在 idle_windows.go / idle_other.go 实现 ——

// getIdleSeconds 返回用户最后输入至今的秒数（键鼠空闲时间）。
var getIdleSeconds = func() int { return 0 }

// foregroundChangedSinceLast 检测前台窗口自上次调用以来是否变化。
var foregroundChangedSinceLast = func() bool { return false }

// externalGPUUtil 返回扣除本 Agent 进程后的"外部" GPU 利用率 %。
// 默认实现：取 GPU 总利用率（保守判定——把本 Agent 自己的占用也算外部，偏保守）。
// 由 Agent 启动时注入真实实现（见 agent.go 的 setExternalGPUUtil）。
var externalGPUUtil = func() float64 { return 0 }

// setExternalGPUUtil 注入真实的 GPU 利用率采集函数（Agent 启动时调用）。
func setExternalGPUUtil(fn func() float64) {
	if fn != nil {
		externalGPUUtil = fn
	}
}
