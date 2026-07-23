// Package queue 实现单并发 LLM 请求队列。
// 对应原计划第十七节：执行中 1 / 等待 10 / 超出返回系统繁忙。
// ctx 取消时释放槽位。
package queue

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"
)

// ErrBusy 表示等待队列已满，拒绝新请求。
var ErrBusy = errors.New("system_busy")

// LLMQueue 限制同时执行的生成请求数，并维护有限等待队列。
type LLMQueue struct {
	concurrency int
	sem         chan struct{} // 并发信号量
	waiting     chan struct{} // 等待席位
	timeout     time.Duration
	log         *slog.Logger
}

// New 构造队列。concurrency 为并发上限（CPU/单 GPU 建议 1），
// maxWaiting 为等待席上限，timeout 为排队+执行总超时。
func New(concurrency, maxWaiting int, timeout time.Duration, log *slog.Logger) *LLMQueue {
	if concurrency < 1 {
		concurrency = 1
	}
	if maxWaiting < 0 {
		maxWaiting = 0
	}
	return &LLMQueue{
		concurrency: concurrency,
		sem:         make(chan struct{}, concurrency),
		waiting:     make(chan struct{}, maxWaiting),
		timeout:     timeout,
		log:         log,
	}
}

// Acquire 获取一个执行槽位。队列满时返回 ErrBusy。
// 成功后调用方必须在 fn 结束或 ctx 取消时调用 release。
// Run 封装了 acquire/release/timeout，优先使用 Run。
func (q *LLMQueue) Acquire(ctx context.Context) (release func(), err error) {
	// 先抢等待席。满了立即拒绝（不阻塞）。
	select {
	case q.waiting <- struct{}{}:
	default:
		return nil, ErrBusy
	}

	// 抢执行信号量，带总超时。
	timer := time.NewTimer(q.timeout)
	defer timer.Stop()

	select {
	case q.sem <- struct{}{}:
		// 拿到执行权，释放等待席。
		<-q.waiting
		released := false
		return func() {
			if released {
				return
			}
			released = true
			<-q.sem
		}, nil
	case <-ctx.Done():
		<-q.waiting
		return nil, ctx.Err()
	case <-timer.C:
		<-q.waiting
		return nil, fmt.Errorf("%w: 排队超时", ErrBusy)
	}
}

// Run 在队列保护下执行 fn。自动管理 acquire/release/ctx。
// fn 收到的 ctx 带 timeout，取消后应尽快返回。
func (q *LLMQueue) Run(ctx context.Context, fn func(ctx context.Context) error) error {
	release, err := q.Acquire(ctx)
	if err != nil {
		return err
	}
	defer release()

	runCtx, cancel := context.WithTimeout(ctx, q.timeout)
	defer cancel()
	return fn(runCtx)
}

// Concurrency 返回并发上限（供健康/指标展示）。
func (q *LLMQueue) Concurrency() int { return q.concurrency }

// Active 返回当前执行中的请求数。
func (q *LLMQueue) Active() int { return len(q.sem) }

// Waiting 返回当前等待中的请求数。
func (q *LLMQueue) Waiting() int { return len(q.waiting) }
