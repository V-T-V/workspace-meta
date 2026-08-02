package api

import (
	"net/http"
	"sync"
	"time"
)

// rateLimiter 基于 IP 的滑动窗口频率限制器。
// 每个窗口内允许 maxRequests 次请求，超出的返回 429。
type rateLimiter struct {
	mu       sync.Mutex
	buckets  map[string]*bucket
	maxReqs  int
	window   time.Duration
}

type bucket struct {
	count    int
	resetAt  time.Time
}

func newRateLimiter(maxReqs int, window time.Duration) *rateLimiter {
	return &rateLimiter{
		buckets: make(map[string]*bucket),
		maxReqs: maxReqs,
		window:  window,
	}
}

// allow 检查是否允许请求。返回 true 表示放行。
func (rl *rateLimiter) allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	b, exists := rl.buckets[key]
	if !exists || now.After(b.resetAt) {
		rl.buckets[key] = &bucket{count: 1, resetAt: now.Add(rl.window)}
		// 惰性清理过期 bucket
		if len(rl.buckets) > 1000 {
			for k, v := range rl.buckets {
				if now.After(v.resetAt) {
					delete(rl.buckets, k)
				}
			}
		}
		return true
	}
	if b.count >= rl.maxReqs {
		return false
	}
	b.count++
	return true
}

// RateLimitMiddleware 返回频率限制中间件。
func (s *Server) RateLimitMiddleware(rl *rateLimiter, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ip := clientIP(r)
		if !rl.allow(ip) {
			w.Header().Set("Retry-After", "60")
			writeError(w, http.StatusTooManyRequests, "rate_limited", "请求过于频繁，请稍后再试")
			return
		}
		next(w, r)
	}
}

// 全局限流器实例（在 Register 时初始化）
var (
	chatLimiter   = newRateLimiter(30, time.Minute)   // 聊天 30/min（允许连续测试）
	adminLimiter  = newRateLimiter(30, time.Minute)   // 管理 30/min
)
