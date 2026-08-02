// Package agent 实现 gpu-mesh 的 Windows Agent（受控端）。
//
// 职责：
//   - 注册为 Windows 服务，开机自启、崩溃自恢复（service.go）
//   - 经反向 WS 连接 Relay，穿透 NAT（connection.go）
//   - 周期采集 GPU 快照 + 让位状态，随心跳上报（heartbeat.go / gpumon / yield.go）
//   - 接收并执行 Relay 下发的任务（tasks.go + executors/）
//
// 依赖方向（单向）：agent → {proto, gpumon, engine}。
package agent

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/QiuShichang/gpu-mesh/internal/engine"
	"github.com/QiuShichang/gpu-mesh/internal/gpumon"
)

// Agent gpu-mesh 受控端主结构。
type Agent struct {
	cfg   Config
	gpu   *gpumon.Collector
	yield *YieldDetector

	// engines 缓存本机可用引擎实例（启动时探测一次，执行器复用）。
	enginesMu sync.RWMutex
	engines   []engine.Engine

	// running 跟踪正在执行的任务，支持取消。
	running *runningTasks

	cancel context.CancelFunc
}

// New 构造 Agent。
func New(cfg Config) *Agent {
	cfg.applyDefaults()
	return &Agent{
		cfg:     cfg,
		gpu:     gpumon.NewCollector(cfg.GPUCollectInterval),
		yield:   NewYieldDetector(cfg.HeartbeatInterval),
		running: newRunningTasks(),
	}
}

// Run 启动 Agent（阻塞，直到 ctx 取消）。
//
// 启动顺序：
//  1. GPU 采集器（立即采集一次，后续周期刷新）
//  2. 让位检测器（立即采集一次，后续周期刷新）
//  3. 探测本机引擎（缓存到 a.engines）
//  4. 反向连接循环（注册 → 心跳 → 任务分发）
func (a *Agent) Run(ctx context.Context) {
	innerCtx, cancel := context.WithCancel(ctx)
	a.cancel = cancel

	// 1. GPU 采集
	a.gpu.Start()
	defer a.gpu.Stop()
	log.Printf("[agent] GPU 采集器已启动 (interval=%s) available=%v",
		a.cfg.GPUCollectInterval, a.gpu.Available())

	// 2. 让位检测（注入 GPU 利用率来源，使 externalGPUUtil 能反映真实占用）
	setExternalGPUUtil(func() float64 {
		gpus := a.gpu.Snapshot()
		if len(gpus) == 0 {
			return 0
		}
		// 取所有 GPU 的平均利用率作为外部占用估计（保守：含本 Agent 自身推理占用）
		sum := 0.0
		for _, g := range gpus {
			sum += g.UtilGPU
		}
		return sum / float64(len(gpus))
	})
	a.yield.Start()
	defer a.yield.Stop()
	log.Printf("[agent] 让位检测器已启动，当前状态: %s (budget=%d%%)",
		a.yield.State().Level, a.yield.State().Budget)

	// 3. 引擎探测（缓存）
	a.probeAndCacheEngines()

	// 4. 反向连接循环（阻塞）
	a.reconnectLoop(innerCtx)
}

// probeAndCacheEngines 探测本机引擎并缓存（供 buildRegister 与执行器复用）。
func (a *Agent) probeAndCacheEngines() {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	engs, models := engine.ProbeAll(ctx)
	a.enginesMu.Lock()
	a.engines = engs
	a.enginesMu.Unlock()
	log.Printf("[agent] 引擎探测完成: 引擎=%v 模型=%d 个", engine.EngineNames(engs), len(models))
}

// Engines 返回缓存的本机引擎实例。
func (a *Agent) Engines() []engine.Engine {
	a.enginesMu.RLock()
	defer a.enginesMu.RUnlock()
	return a.engines
}

// FindEngine 按名查找引擎；name 为空时返回第一个可用引擎。
func (a *Agent) FindEngine(name string) engine.Engine {
	engs := a.Engines()
	if name == "" {
		if len(engs) > 0 {
			return engs[0]
		}
		return nil
	}
	return engine.Find(engs, name)
}

// buildRegister 构造注册载荷（每次重连重新探测模型列表，保证最新）。
func (a *Agent) buildRegister() (engines []string, models []string) {
	engs := a.Engines()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var allModels []engine.ModelInfo
	for _, e := range engs {
		if ms, err := e.ListModels(ctx); err == nil {
			allModels = append(allModels, ms...)
		}
	}
	return engine.EngineNames(engs), engine.ModelNames(allModels)
}
