// Package server 提供 HTTP 端点暴露 metrics 和 trace，兼容 Prometheus scrape。
//
// 端点：
//   - GET /metrics     → Prometheus 文本格式（Content-Type: text/plain; version=0.0.4; charset=utf-8）
//   - GET /api/trace   → trace 的 JSON（Tracer.Peek 的 span 列表）
//   - GET /api/metrics → metrics 的 JSON（MetricPoint + HistogramData）
//   - GET /health      → {"status":"ok"}
//
// 设计目标：零外部依赖（标准库 net/http + encoding/json），可被 Prometheus 直接抓取。
package server

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"net"
	"net/http"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/QiuShichang/obs-lite/internal/exporter"
	"github.com/QiuShichang/obs-lite/internal/metrics"
	"github.com/QiuShichang/obs-lite/internal/trace"
	"github.com/QiuShichang/obs-lite/internal/types"
)

// Prometheus 标准 Content-Type（version=0.0.4 是 Prometheus 文本格式的当前版本）。
const promContentType = "text/plain; version=0.0.4; charset=utf-8"

// metricsJSONResponse 是 /api/metrics 的 JSON 响应结构。
type metricsJSONResponse struct {
	Points     []jsonMetricPoint   `json:"points"`
	Histograms []jsonHistogramData `json:"histograms"`
}

// jsonMetricPoint 是 types.MetricPoint 的 JSON 友好表示（MetricKind 转字符串）。
type jsonMetricPoint struct {
	Name      string            `json:"name"`
	Kind      string            `json:"kind"`
	Value     float64           `json:"value"`
	Labels    map[string]string `json:"labels,omitempty"`
	Timestamp time.Time         `json:"timestamp"`
}

// jsonHistogramData 是 types.HistogramData 的 JSON 友好表示。
type jsonHistogramData struct {
	Name    string                `json:"name"`
	Labels  map[string]string     `json:"labels,omitempty"`
	Buckets []jsonHistogramBucket `json:"buckets"`
	Sum     float64               `json:"sum"`
	Count   uint64                `json:"count"`
}

// jsonHistogramBucket 是单个桶的 JSON 表示（+Inf 上界转字符串 "+Inf"，避免 JSON 编码 inf 出错）。
type jsonHistogramBucket struct {
	UpperBound string `json:"le"`
	Count      uint64 `json:"count"`
}

// jsonSpan 是 types.Span 的 JSON 友好表示（SpanStatus 转字符串）。
type jsonSpan struct {
	TraceID    string            `json:"trace_id"`
	SpanID     string            `json:"span_id"`
	ParentID   string            `json:"parent_id,omitempty"`
	Name       string            `json:"name"`
	StartTime  time.Time         `json:"start_time"`
	EndTime    time.Time         `json:"end_time"`
	Attributes map[string]string `json:"attributes,omitempty"`
	Events     []jsonSpanEvent   `json:"events,omitempty"`
	Status     string            `json:"status"`
}

// jsonSpanEvent 是 types.SpanEvent 的 JSON 表示。
type jsonSpanEvent struct {
	Name       string            `json:"name"`
	Timestamp  time.Time         `json:"timestamp"`
	Attributes map[string]string `json:"attributes,omitempty"`
}

// Server 持有 Registry + Tracer，提供 HTTP 端点。
type Server struct {
	Addr     string // 监听地址 "host:port"
	Registry *metrics.Registry
	Tracer   *trace.Tracer
	server   *http.Server
	ln       net.Listener // StartInBackground 绑定的 listener（暴露实际端口）
}

// New 创建 Server。
func New(addr string, reg *metrics.Registry, tr *trace.Tracer) *Server {
	return &Server{Addr: addr, Registry: reg, Tracer: tr}
}

// ListenerAddr 返回实际监听地址（StartInBackground 后可用，含 OS 分配的端口）。
// 未启动时返回空串。
func (s *Server) ListenerAddr() string {
	if s.ln == nil {
		return ""
	}
	return s.ln.Addr().String()
}

// Handler 返回 http.Handler（路由）。
// 用 Go 1.22+ 的方法路由（{Method} {Path}）。
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /metrics", s.handleMetrics)
	mux.HandleFunc("GET /api/trace", s.handleTrace)
	mux.HandleFunc("GET /api/metrics", s.handleMetricsJSON)
	mux.HandleFunc("GET /health", s.handleHealth)
	return mux
}

// handleMetrics 输出 Prometheus 文本格式（可被 prometheus 抓取）。
func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", promContentType)
	// 空指针保护：未注入 Registry 时输出空（仍是合法的 Prometheus 响应）。
	if s.Registry == nil {
		return
	}
	body := exporter.FormatMetricsPrometheus(s.Registry)
	_, _ = w.Write([]byte(body))
}

// handleTrace 输出 trace 的 JSON（Tracer.Peek 的 span 列表，含未结束 span）。
func (s *Server) handleTrace(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	var spans []*jsonSpan
	if s.Tracer != nil {
		for _, sp := range s.Tracer.Peek() {
			spans = append(spans, toJSONSpan(sp))
		}
	}
	// 即使为空也输出 []，保证客户端解析稳定。
	if spans == nil {
		spans = []*jsonSpan{}
	}
	writeJSON(w, spans)
}

// handleMetricsJSON 输出 metrics 的 JSON（MetricPoint + HistogramData 列表）。
func (s *Server) handleMetricsJSON(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	resp := metricsJSONResponse{Points: []jsonMetricPoint{}, Histograms: []jsonHistogramData{}}
	if s.Registry != nil {
		for _, p := range s.Registry.AllPoints() {
			resp.Points = append(resp.Points, jsonMetricPoint{
				Name: p.Name, Kind: p.Kind.String(), Value: p.Value,
				Labels: p.Labels, Timestamp: p.Timestamp,
			})
		}
		for _, h := range s.Registry.AllHistograms() {
			resp.Histograms = append(resp.Histograms, toJSONHistogramData(h))
		}
	}
	writeJSON(w, resp)
}

// handleHealth 健康检查端点。
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	writeJSON(w, map[string]string{"status": "ok"})
}

// Start 启动 HTTP 服务（阻塞），收到 SIGINT/SIGTERM 时优雅关闭。
func (s *Server) Start() error {
	s.server = &http.Server{
		Addr:    s.Addr,
		Handler: s.Handler(),
	}
	// 监听 Ctrl+C / kill，触发优雅关闭。
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		if err := s.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		stop()
		// 给在途请求 5 秒收尾时间。
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return s.server.Shutdown(shutdownCtx)
	}
}

// StartInBackground 后台启动 HTTP 服务，返回关闭函数。
// 关闭函数会优雅关闭服务器并返回关闭过程中的错误。
// 先用 net.Listen 绑定端口（同步、确定成功/失败），再用 Serve 在后台服务，
// 这样 ListenerAddr() 可拿到 OS 分配的实际端口。
func (s *Server) StartInBackground() (func() error, error) {
	ln, err := net.Listen("tcp", s.Addr)
	if err != nil {
		return nil, err
	}
	s.ln = ln
	s.server = &http.Server{
		Handler: s.Handler(),
	}
	errCh := make(chan error, 1)
	go func() {
		if err := s.server.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}
		errCh <- nil
	}()
	closer := func() error {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		err := s.server.Shutdown(shutdownCtx)
		// 排空后台 goroutine 的退出错误，避免 goroutine 泄漏。
		<-errCh
		return err
	}
	return closer, nil
}

// ===== 辅助：types → JSON 友好结构 =====

// toJSONSpan 把 *types.Span 转成 JSON 友好结构（Status 转字符串）。
func toJSONSpan(sp *types.Span) *jsonSpan {
	out := &jsonSpan{
		TraceID:    sp.TraceID,
		SpanID:     sp.SpanID,
		ParentID:   sp.ParentID,
		Name:       sp.Name,
		StartTime:  sp.StartTime,
		EndTime:    sp.EndTime,
		Attributes: sp.Attributes,
		Status:     sp.Status.String(),
	}
	for _, ev := range sp.Events {
		out.Events = append(out.Events, jsonSpanEvent{
			Name: ev.Name, Timestamp: ev.Timestamp, Attributes: ev.Attributes,
		})
	}
	return out
}

// toJSONHistogramData 把 types.HistogramData 转成 JSON 友好结构（+Inf 桶上界转字符串）。
func toJSONHistogramData(h types.HistogramData) jsonHistogramData {
	out := jsonHistogramData{
		Name: h.Name, Labels: h.Labels, Sum: h.Sum, Count: h.Count,
	}
	for _, b := range h.Buckets {
		out.Buckets = append(out.Buckets, jsonHistogramBucket{
			UpperBound: formatBound(b.UpperBound), Count: b.Count,
		})
	}
	return out
}

// formatBound 把桶上界渲染成字符串（+Inf 用 "+Inf"，其余用 %g 保留精度）。
// 与 exporter 包的 formatBound 同逻辑；这里单独实现避免引入未导出依赖。
func formatBound(b float64) string {
	if math.IsInf(b, 1) {
		return "+Inf"
	}
	// 用 %g 保留原始精度，避免 0.005/0.025 被渲染成同值。
	return strconv.FormatFloat(b, 'g', -1, 64)
}

// writeJSON 把 v 序列化成 JSON 写入 ResponseWriter，失败时写 500。
// SetEscapeHTML(false) 避免把 label 里的 <> 转义（Prometheus 数据可能含）。
func writeJSON(w http.ResponseWriter, v any) {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		http.Error(w, "json encode failed: "+err.Error(), http.StatusInternalServerError)
	}
}
