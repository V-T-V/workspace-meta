// Package scheduler 实现管道的定时调度（M1 最小版）。
//
// M1：支持简单的间隔触发（每 N 秒跑一次指定管道）。
// M2 候选：cron 表达式 / 事件触发（文件变更）/ 依赖触发。
//
// 单机模式：调度器在 server 进程内跑，到点调用 pipeline.Run。
// M3 分布式时：调度器把任务派发给远程 worker（见 internal/proto）。
package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// Job 是一个定时调度任务。
type Job struct {
	Name         string                                       // 任务名
	PipelineYAML string                                       // 管道 YAML 内容
	Interval     time.Duration                                // 触发间隔（0 表示只跑一次）
	RunFunc      func(ctx context.Context, yaml string) error // 实际执行函数（由调用方注入，避免循环依赖）
}

// Scheduler 管理多个 Job。
type Scheduler struct {
	jobs    map[string]*Job
	cancels map[string]context.CancelFunc
	mu      sync.Mutex
	log     *slog.Logger
	running bool
}

// New 创建调度器。
func New(log *slog.Logger) *Scheduler {
	if log == nil {
		log = slog.Default()
	}
	return &Scheduler{
		jobs:    map[string]*Job{},
		cancels: map[string]context.CancelFunc{},
		log:     log,
	}
}

// Add 注册一个 Job（若同名已存在则覆盖）。
func (s *Scheduler) Add(j Job) error {
	if j.Name == "" {
		return fmt.Errorf("job 缺少 name")
	}
	if j.RunFunc == nil {
		return fmt.Errorf("job %s 缺少 RunFunc", j.Name)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	// 若已存在同名且在跑，先停掉旧的。
	if cancel, ok := s.cancels[j.Name]; ok {
		cancel()
	}
	s.jobs[j.Name] = &j
	return nil
}

// Start 启动所有 Job 的调度循环。返回 cancel 函数用于停止全部。
func (s *Scheduler) Start(ctx context.Context) context.CancelFunc {
	s.mu.Lock()
	s.running = true
	s.mu.Unlock()

	rootCtx, rootCancel := context.WithCancel(ctx)

	for name, job := range s.jobs {
		jobCtx, jobCancel := context.WithCancel(rootCtx)
		s.mu.Lock()
		s.cancels[name] = jobCancel
		s.mu.Unlock()
		go s.runJob(jobCtx, job)
	}

	return func() {
		rootCancel()
		s.mu.Lock()
		s.running = false
		s.mu.Unlock()
		s.log.Info("[scheduler] 已停止全部 job")
	}
}

// runJob 循环执行单个 job。
func (s *Scheduler) runJob(ctx context.Context, job *Job) {
	// 首次立即跑一次
	s.execOnce(ctx, job)
	if job.Interval == 0 {
		return // 一次性任务
	}
	ticker := time.NewTicker(job.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.execOnce(ctx, job)
		}
	}
}

// execOnce 跑一次管道。
func (s *Scheduler) execOnce(ctx context.Context, job *Job) {
	s.log.Info("[scheduler] 触发 job", "name", job.Name, "time", time.Now().Format(time.RFC3339))
	start := time.Now()
	if err := job.RunFunc(ctx, job.PipelineYAML); err != nil {
		s.log.Error("[scheduler] job 失败", "name", job.Name, "err", err, "duration", time.Since(start))
		return
	}
	s.log.Info("[scheduler] job 完成", "name", job.Name, "duration", time.Since(start))
}

// Jobs 返回已注册的 job 名称列表。
func (s *Scheduler) Jobs() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]string, 0, len(s.jobs))
	for k := range s.jobs {
		out = append(out, k)
	}
	return out
}
