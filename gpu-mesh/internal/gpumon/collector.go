package gpumon

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/QiuShichang/gpu-mesh/internal/proto"
)

// Collector 周期性采集 GPU 快照，缓存最新结果供 Agent 心跳读取。
//
// 设计要点：
//   - 后台 goroutine 按 interval 周期调用 SnapshotOnce。
//   - Snapshot() 返回最近一次成功的快照（无锁读原子指针）。
//   - 采集失败（无 GPU）时缓存空切片，调用方据此判断本机是否有 GPU。
type Collector struct {
	interval time.Duration

	mu     sync.RWMutex
	last   []proto.GPUSnapshot // 最近一次成功快照
	lastAt time.Time
	ok     bool // 是否曾采集成功

	cancel context.CancelFunc
	done   chan struct{}
}

// NewCollector 构造采集器。interval 建议为心跳周期的 1/2（如心跳 5s → 采集 2s）。
func NewCollector(interval time.Duration) *Collector {
	return &Collector{interval: interval, done: make(chan struct{})}
}

// Start 启动后台采集循环。
func (c *Collector) Start() {
	ctx, cancel := context.WithCancel(context.Background())
	c.cancel = cancel
	// 启动时立即采集一次，避免首次心跳拿不到数据
	c.collectOnce(ctx)
	go c.loop(ctx)
}

// Stop 停止采集。
func (c *Collector) Stop() {
	if c.cancel != nil {
		c.cancel()
	}
	<-c.done
}

func (c *Collector) loop(ctx context.Context) {
	defer close(c.done)
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.collectOnce(ctx)
		}
	}
}

func (c *Collector) collectOnce(ctx context.Context) {
	snap, err := SnapshotOnce(ctx)
	if err != nil {
		// 无 GPU 是常态（开发机/虚拟机），降级为空，不打错误日志避免噪音
		if err == ErrNoGPU {
			c.mu.Lock()
			c.last = nil
			c.ok = false
			c.mu.Unlock()
			return
		}
		log.Printf("[gpumon] 采集失败: %v", err)
		return
	}
	c.mu.Lock()
	c.last = snap
	c.lastAt = time.Now()
	c.ok = true
	c.mu.Unlock()
}

// Snapshot 返回最近一次成功的 GPU 快照。无 GPU 返回 nil。
func (c *Collector) Snapshot() []proto.GPUSnapshot {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.last
}

// Available 本机是否存在可用的 NVIDIA GPU。
func (c *Collector) Available() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.ok
}
