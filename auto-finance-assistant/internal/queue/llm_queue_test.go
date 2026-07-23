package queue

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func testQueue(conc, wait int, timeout time.Duration) *LLMQueue {
	return New(conc, wait, timeout, slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})))
}

// TestRun_SingleConcurrency 验证并发为 1 时串行执行。
func TestRun_SingleConcurrency(t *testing.T) {
	q := testQueue(1, 5, 2*time.Second)

	var active, maxActive int32
	var mu sync.Mutex
	var wg sync.WaitGroup

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = q.Run(context.Background(), func(ctx context.Context) error {
				cur := atomic.AddInt32(&active, 1)
				mu.Lock()
				if cur > maxActive {
					maxActive = cur
				}
				mu.Unlock()
				time.Sleep(20 * time.Millisecond)
				atomic.AddInt32(&active, -1)
				return nil
			})
		}()
	}
	wg.Wait()

	if maxActive != 1 {
		t.Errorf("并发=1 时最大并发应为 1，实际 %d", maxActive)
	}
	if q.Active() != 0 {
		t.Errorf("结束后执行中应为 0，实际 %d", q.Active())
	}
}

// TestRun_BusyReject 验证等待队列满时立即返回 ErrBusy。
func TestRun_BusyReject(t *testing.T) {
	q := testQueue(1, 1, 5*time.Second)

	// 占用唯一执行槽位
	rel, err := q.Acquire(context.Background())
	if err != nil {
		t.Fatalf("首次 Acquire 失败: %v", err)
	}
	defer rel()

	// 占满 1 个等待席（会阻塞在 sem 上，直到超时；用 goroutine）
	stopWait := make(chan struct{})
	go func() {
		_, _ = q.Acquire(context.Background())
		close(stopWait)
	}()
	// 等待等待席被占
	time.Sleep(50 * time.Millisecond)
	if q.Waiting() != 1 {
		t.Fatalf("应有 1 个等待者，实际 %d", q.Waiting())
	}

	// 第三次应立即被拒（等待席已满）
	done := make(chan error, 1)
	go func() {
		_, err := q.Acquire(context.Background())
		done <- err
	}()
	select {
	case err := <-done:
		if !errors.Is(err, ErrBusy) {
			t.Errorf("第三次应返回 ErrBusy，实际 %v", err)
		}
	case <-time.After(200 * time.Millisecond):
		t.Error("第三次应立即拒绝，但阻塞了")
	}
	<-stopWait
}

// TestRun_Timeout 验证排队超时。
func TestRun_Timeout(t *testing.T) {
	q := testQueue(1, 1, 100*time.Millisecond)
	rel, _ := q.Acquire(context.Background())
	defer rel()

	// 第二个请求排队，等待席有位但执行槽被占，应超时
	err := q.Run(context.Background(), func(ctx context.Context) error {
		return nil
	})
	if !errors.Is(err, ErrBusy) {
		t.Errorf("排队超时应返回 ErrBusy，实际 %v", err)
	}
}

// TestRun_ContextCancel 验证 ctx 取消释放等待席。
func TestRun_ContextCancel(t *testing.T) {
	q := testQueue(1, 1, 5*time.Second)
	rel, _ := q.Acquire(context.Background())
	defer rel()

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(30 * time.Millisecond)
		cancel()
	}()

	err := q.Run(ctx, func(ctx context.Context) error { return nil })
	if !errors.Is(err, context.Canceled) {
		t.Errorf("ctx 取消应返回 context.Canceled，实际 %v", err)
	}
	// 等待席应已释放
	if q.Waiting() != 0 {
		t.Errorf("取消后等待席应为 0，实际 %d", q.Waiting())
	}
}
