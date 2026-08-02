package api

// 健康检查与监控增强（第八轮）。
// 新增 Kubernetes 风格探针：
//   - GET /api/health/livez  存活探针（进程存活 + DB 可 ping），低开销
//   - GET /api/health/readyz 就绪探针（DB + 模型双就绪才 200）
//   - GET /api/health/ping   延迟测量（DB ping + 模型探测耗时，毫秒）

import (
	"context"
	"net/http"
	"time"
)

// dbPing 对 DB 做一次轻量查询，返回耗时(ms) 与是否成功。
func (s *Server) dbPing() (latencyMs float64, ok bool) {
	start := time.Now()
	var n int
	err := s.db.QueryRow(`SELECT count(*) FROM conversations`).Scan(&n)
	latencyMs = float64(time.Since(start).Microseconds()) / 1000.0
	return latencyMs, err == nil
}

// handleLivez 存活探针：只要 DB 能 ping 通即 200（模型不参与，避免误重启）。
func (s *Server) handleLivez(w http.ResponseWriter, r *http.Request) {
	dbLatency, dbOK := s.dbPing()
	status := "alive"
	code := http.StatusOK
	if !dbOK {
		status = "dead"
		code = http.StatusServiceUnavailable
	}
	writeJSON(w, code, map[string]any{
		"status":      status,
		"database":    map[string]any{"ok": dbOK, "latencyMs": round2(dbLatency)},
		"uptimeSec":   int(time.Since(startTime).Seconds()),
		"version":     version,
	})
}

// handleReadyz 就绪探针：DB + 模型双就绪才 200（决定是否能接流量）。
func (s *Server) handleReadyz(w http.ResponseWriter, r *http.Request) {
	dbLatency, dbOK := s.dbPing()
	hs := s.cachedHealth()
	modelOK := hs.Reachable && hs.HasModel

	status := "ready"
	code := http.StatusOK
	if !dbOK || !modelOK {
		status = "not_ready"
		code = http.StatusServiceUnavailable
	}
	components := map[string]any{
		"database": map[string]any{"ok": dbOK, "latencyMs": round2(dbLatency)},
	}
	if hs.Reachable {
		components["model"] = map[string]any{"reachable": true, "hasModel": hs.HasModel}
	} else {
		components["model"] = map[string]any{"reachable": false}
	}
	writeJSON(w, code, map[string]any{
		"status":     status,
		"components": components,
	})
}

// handlePing 延迟测量端点：返回 DB 与模型探测的耗时。
func (s *Server) handlePing(w http.ResponseWriter, r *http.Request) {
	dbLatency, dbOK := s.dbPing()

	// 模型探测耗时（实时，非缓存）
	modelStart := time.Now()
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	hs := s.model.Health(ctx)
	modelLatency := float64(time.Since(modelStart).Microseconds()) / 1000.0

	writeJSON(w, http.StatusOK, map[string]any{
		"database": map[string]any{
			"ok":        dbOK,
			"latencyMs": round2(dbLatency),
		},
		"model": map[string]any{
			"reachable": hs.Reachable,
			"hasModel":  hs.HasModel,
			"latencyMs": round2(modelLatency),
		},
		"timestamp": time.Now().Format(time.RFC3339),
	})
}

// round2 保留 2 位小数。
func round2(f float64) float64 {
	return float64(int(f*100+0.5)) / 100
}

// startTime 进程启动时间（main 或 init 时设置；测试用零值不影响）。
var startTime = time.Now()

// SetStartTime 供 main 注入真实启动时间。
func SetStartTime(t time.Time) { startTime = t }
