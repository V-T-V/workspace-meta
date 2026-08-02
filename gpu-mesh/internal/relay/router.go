package relay

import (
	"encoding/json"
	"net/http"
	"runtime"
	"sync/atomic"
	"time"
)

// Metrics Relay 运行指标（原子计数器，无锁）。
type Metrics struct {
	AgentsOnline int64
	TasksTotal   int64
	AgentsSeen   int64 // 累计上线过的 Agent 数（含已离线）
	EventsDropped int64
	StartedAt    time.Time
	// 业务指标（Phase 6+ 优化）
	InferenceTotal   int64 // 累计推理请求数
	InferenceSuccess int64 // 推理成功数
	InferenceFail    int64 // 推理失败数
	InferenceMicros  int64 // 推理累计耗时（微秒，用于算平均延迟）
	YieldNacks       int64 // 让位 NACK 次数（任务被让位重调度）
	BatchJobs        int64 // 累计批量作业数
}

var metrics = &Metrics{StartedAt: time.Now()}

func (m *Metrics) snapshot() map[string]any {
	infTotal := atomic.LoadInt64(&m.InferenceTotal)
	infMicros := atomic.LoadInt64(&m.InferenceMicros)
	avgLatencyMs := int64(0)
	if infTotal > 0 {
		avgLatencyMs = infMicros / infTotal / 1000
	}
	return map[string]any{
		"agents_online":     atomic.LoadInt64(&m.AgentsOnline),
		"tasks_total":       atomic.LoadInt64(&m.TasksTotal),
		"agents_seen":       atomic.LoadInt64(&m.AgentsSeen),
		"events_dropped":    atomic.LoadInt64(&m.EventsDropped),
		"uptime_seconds":    int(time.Since(m.StartedAt).Seconds()),
		"goroutines":        runtime.NumGoroutine(),
		"inference_total":   infTotal,
		"inference_success": atomic.LoadInt64(&m.InferenceSuccess),
		"inference_fail":    atomic.LoadInt64(&m.InferenceFail),
		"inference_avg_ms":  avgLatencyMs,
		"yield_nacks":       atomic.LoadInt64(&m.YieldNacks),
		"batch_jobs":        atomic.LoadInt64(&m.BatchJobs),
	}
}

// 业务指标打点辅助。
func incInference(success bool, durationMs int64) {
	atomic.AddInt64(&metrics.InferenceTotal, 1)
	if success {
		atomic.AddInt64(&metrics.InferenceSuccess, 1)
	} else {
		atomic.AddInt64(&metrics.InferenceFail, 1)
	}
	atomic.AddInt64(&metrics.InferenceMicros, durationMs*1000)
}

func incYieldNack() { atomic.AddInt64(&metrics.YieldNacks, 1) }
func incBatchJob()  { atomic.AddInt64(&metrics.BatchJobs, 1) }

// handleListAgents 返回所有在线 Agent 的完整状态（GPU/引擎/让位）。
//
// 这是 Phase 1 仪表盘的数据源：每台机器的利用率、显存、温度、引擎、让位状态一目了然。
func (r *Relay) handleListAgents(w http.ResponseWriter, req *http.Request) {
	if !r.authorize(req) {
		writeJSON(w, http.StatusUnauthorized, errMap("unauthorized"))
		return
	}
	list := r.agents.Snapshot()
	// 同步 metrics
	atomic.StoreInt64(&metrics.AgentsOnline, int64(len(list)))
	writeJSON(w, http.StatusOK, map[string]any{
		"agents": list,
		"count":  len(list),
	})
}

// handleMetrics Prometheus 风格指标（Phase 6 扩展为 /metrics 格式）。
func (r *Relay) handleMetrics(w http.ResponseWriter, _ *http.Request) {
	atomic.StoreInt64(&metrics.AgentsOnline, int64(len(r.agents.Snapshot())))
	atomic.StoreInt64(&metrics.EventsDropped, r.console.Dropped())
	writeJSON(w, http.StatusOK, metrics.snapshot())
}

// handleListTasks 任务历史（Phase 1 基本占位，Phase 2+ 调度器填充）。
func (r *Relay) handleListTasks(w http.ResponseWriter, req *http.Request) {
	if !r.authorize(req) {
		writeJSON(w, http.StatusUnauthorized, errMap("unauthorized"))
		return
	}
	if r.store == nil {
		writeJSON(w, http.StatusOK, map[string]any{"tasks": []any{}, "enabled": false})
		return
	}
	tasks, err := r.store.ListTasks(50)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errMap(err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"tasks": tasks, "count": len(tasks), "enabled": true})
}

// handleSSE SSE 推送实时事件给 Web 控制台（agent 上下线 / 让位变化 / 任务结果）。
func (r *Relay) handleSSE(w http.ResponseWriter, req *http.Request) {
	if !r.authorize(req) {
		writeJSON(w, http.StatusUnauthorized, errMap("unauthorized"))
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // Nginx 关闭缓冲

	subID, ch := r.console.Subscribe(128)
	defer r.console.Unsubscribe(subID)

	// hello 事件
	writeSSE(w, flusher, "hello", map[string]any{"sub_id": subID})
	flusher.Flush()

	notify := req.Context().Done()
	for {
		select {
		case <-notify:
			return
		case ev, ok := <-ch:
			if !ok {
				return
			}
			writeSSE(w, flusher, ev.Kind, map[string]any{
				"agent_id": ev.AgentID,
				"ts":       ev.TS,
				"data":     ev.Payload,
			})
			flusher.Flush()
		}
	}
}

// --- HTTP 辅助 ---

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func errMap(msg string) map[string]any { return map[string]any{"error": msg} }

func writeSSE(w http.ResponseWriter, _ http.Flusher, event string, data any) {
	payload, _ := json.Marshal(data)
	// SSE 帧格式：event: <name>\ndata: <json>\n\n
	w.Write([]byte("event: " + event + "\n"))
	w.Write([]byte("data: " + string(payload) + "\n\n"))
}
