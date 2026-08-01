package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QiuShichang/obs-lite/internal/metrics"
	"github.com/QiuShichang/obs-lite/internal/trace"
)

// newTestServer 构造一个带示例数据的 Server（Registry + Tracer）。
func newTestServer(t *testing.T) (*Server, *httptest.Server) {
	t.Helper()
	reg := metrics.NewRegistry()
	reg.Counter("http_requests_total").Inc(map[string]string{"method": "GET", "status": "200"})
	reg.Gauge("active_connections").Set(5, nil)
	reg.Histogram("request_duration_seconds", []float64{0.1, 0.5}).
		Observe(0.05, map[string]string{"endpoint": "/api"})

	tr := trace.NewTracer()
	ctx, root := tr.Start("GET /api/users", nil)
	root.SetAttr("http.method", "GET")
	_, child := tr.Start("db.query", ctx)
	child.SetAttr("db.system", "postgres")
	child.End()
	root.End()

	srv := New("127.0.0.1:0", reg, tr)
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)
	return srv, ts
}

// get 是测试用的 HTTP GET 封装，返回响应体字符串 + 响应对象。
func get(t *testing.T, ts *httptest.Server, path string) (string, *http.Response) {
	t.Helper()
	resp, err := http.Get(ts.URL + path)
	if err != nil {
		t.Fatalf("GET %s 出错: %v", path, err)
	}
	t.Cleanup(func() { resp.Body.Close() })
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("读响应体出错: %v", err)
	}
	return string(body), resp
}

// ===== TestMetricsEndpoint =====

// TestMetricsEndpoint GET /metrics 返回 Prometheus 文本格式，含 metric 名 + 正确 Content-Type。
func TestMetricsEndpoint(t *testing.T) {
	_, ts := newTestServer(t)
	body, resp := get(t, ts, "/metrics")

	// Content-Type 必须是 Prometheus 标准（含 version=0.0.4）。
	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(ct, "text/plain") || !strings.Contains(ct, "version=0.0.4") {
		t.Errorf("Content-Type 不符 Prometheus 标准, got %q", ct)
	}

	// 应包含 counter 名 + TYPE 头。
	if !strings.Contains(body, "# TYPE http_requests_total counter") {
		t.Errorf("缺 counter TYPE 头:\n%s", body)
	}
	// 应包含数据行（带 labels）。
	if !strings.Contains(body, `http_requests_total{method="GET",status="200"}`) &&
		!strings.Contains(body, `http_requests_total{status="200",method="GET"}`) {
		t.Errorf("缺带 labels 的 counter 数据行:\n%s", body)
	}
	// 应包含 gauge 名。
	if !strings.Contains(body, "active_connections") {
		t.Errorf("缺 gauge metric:\n%s", body)
	}
	// 应为 200。
	if resp.StatusCode != http.StatusOK {
		t.Errorf("状态码 = %d, want 200", resp.StatusCode)
	}
}

// TestMetricsEndpointEmptyRegistry 空 Registry 时 /metrics 返回 200 + 空 body（不 panic）。
func TestMetricsEndpointEmptyRegistry(t *testing.T) {
	srv := New("127.0.0.1:0", metrics.NewRegistry(), trace.NewTracer())
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	body, resp := get(t, ts, "/metrics")
	if resp.StatusCode != http.StatusOK {
		t.Errorf("状态码 = %d, want 200", resp.StatusCode)
	}
	if body != "" {
		t.Errorf("空 registry 应输出空 body, got %q", body)
	}
}

// ===== TestHealthEndpoint =====

// TestHealthEndpoint GET /health 返回 {"status":"ok"} + JSON Content-Type。
func TestHealthEndpoint(t *testing.T) {
	_, ts := newTestServer(t)
	body, resp := get(t, ts, "/health")

	if resp.StatusCode != http.StatusOK {
		t.Errorf("状态码 = %d, want 200", resp.StatusCode)
	}
	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Errorf("Content-Type 应为 JSON, got %q", ct)
	}

	var got map[string]string
	if err := json.Unmarshal([]byte(body), &got); err != nil {
		t.Fatalf("健康响应不是合法 JSON: %v\nbody: %s", err, body)
	}
	if got["status"] != "ok" {
		t.Errorf("status = %q, want \"ok\"", got["status"])
	}
}

// ===== TestTraceEndpoint =====

// TestTraceEndpoint GET /api/trace 返回 span 列表的 JSON。
func TestTraceEndpoint(t *testing.T) {
	_, ts := newTestServer(t)
	body, resp := get(t, ts, "/api/trace")

	if resp.StatusCode != http.StatusOK {
		t.Errorf("状态码 = %d, want 200", resp.StatusCode)
	}
	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Errorf("Content-Type 应为 JSON, got %q", ct)
	}

	var spans []jsonSpan
	if err := json.Unmarshal([]byte(body), &spans); err != nil {
		t.Fatalf("trace 响应不是合法 JSON 数组: %v\nbody: %s", err, body)
	}
	// 应有 2 个 span（root + child）。
	if len(spans) != 2 {
		t.Fatalf("span 数 = %d, want 2", len(spans))
	}

	// 找到 db.query span 验证属性 + parent。
	var dbSpan *jsonSpan
	for i := range spans {
		if spans[i].Name == "db.query" {
			dbSpan = &spans[i]
		}
	}
	if dbSpan == nil {
		t.Fatalf("缺 db.query span, got: %#v", spans)
	}
	if dbSpan.Attributes["db.system"] != "postgres" {
		t.Errorf("db.query 属性 db.system = %q, want \"postgres\"", dbSpan.Attributes["db.system"])
	}
	if dbSpan.ParentID == "" {
		t.Error("db.query 应有 parent_id（它是子 span）")
	}
	if dbSpan.Status != "OK" {
		t.Errorf("db.query status = %q, want \"OK\"", dbSpan.Status)
	}
}

// TestTraceEndpointEmpty 空 Tracer 时 /api/trace 返回空数组（不是 null）。
func TestTraceEndpointEmpty(t *testing.T) {
	srv := New("127.0.0.1:0", metrics.NewRegistry(), trace.NewTracer())
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	body, resp := get(t, ts, "/api/trace")
	if resp.StatusCode != http.StatusOK {
		t.Errorf("状态码 = %d, want 200", resp.StatusCode)
	}
	// 必须是 []，不是 null（客户端类型稳定）。
	if strings.TrimSpace(body) != "[]" {
		t.Errorf("空 tracer 应返回 [], got %q", body)
	}
}

// ===== TestMetricsJSONEndpoint =====

// TestMetricsJSONEndpoint GET /api/metrics 返回 MetricPoint + Histogram 的 JSON。
func TestMetricsJSONEndpoint(t *testing.T) {
	_, ts := newTestServer(t)
	body, resp := get(t, ts, "/api/metrics")

	if resp.StatusCode != http.StatusOK {
		t.Errorf("状态码 = %d, want 200", resp.StatusCode)
	}
	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Errorf("Content-Type 应为 JSON, got %q", ct)
	}

	var got metricsJSONResponse
	if err := json.Unmarshal([]byte(body), &got); err != nil {
		t.Fatalf("metrics 响应不是合法 JSON: %v\nbody: %s", err, body)
	}

	// 应有 counter + gauge 两个 point。
	if len(got.Points) != 2 {
		t.Errorf("points 数 = %d, want 2", len(got.Points))
	}
	// 应有 1 个 histogram。
	if len(got.Histograms) != 1 {
		t.Errorf("histograms 数 = %d, want 1", len(got.Histograms))
	}

	// 验证 counter 的 kind 是字符串 "counter"（不是数字）。
	var foundCounter bool
	for _, p := range got.Points {
		if p.Name == "http_requests_total" {
			foundCounter = true
			if p.Kind != "counter" {
				t.Errorf("counter kind = %q, want \"counter\"", p.Kind)
			}
			if p.Value != 1 {
				t.Errorf("counter value = %v, want 1", p.Value)
			}
		}
	}
	if !foundCounter {
		t.Errorf("缺 http_requests_total point, got: %#v", got.Points)
	}

	// 验证 histogram 的 +Inf 桶上界转成了字符串 "+Inf"。
	h := got.Histograms[0]
	var hasInfBucket bool
	for _, b := range h.Buckets {
		if b.UpperBound == "+Inf" {
			hasInfBucket = true
		}
	}
	if !hasInfBucket {
		t.Errorf("histogram 应有 +Inf 桶, got: %#v", h.Buckets)
	}
}

// ===== TestMethodRouting =====

// TestMethodRouting 非 GET 方法访问端点应返回 405（验证 Go 1.22 方法路由生效）。
func TestMethodRouting(t *testing.T) {
	_, ts := newTestServer(t)

	// POST /metrics 应被拒绝（只允许 GET）。
	resp, err := http.Post(ts.URL+"/metrics", "text/plain", strings.NewReader(""))
	if err != nil {
		t.Fatalf("POST /metrics 出错: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("POST /metrics 状态码 = %d, want 405", resp.StatusCode)
	}
}

// ===== TestStartInBackground =====

// TestStartInBackground StartInBackground 能正常监听、响应请求、并被优雅关闭。
// （真实端口绑定，非 httptest，覆盖 StartInBackground 的端口绑定逻辑。）
func TestStartInBackground(t *testing.T) {
	reg := metrics.NewRegistry()
	reg.Counter("bg_total").Inc(nil)
	tr := trace.NewTracer()
	srv := New("127.0.0.1:0", reg, tr)

	closer, err := srv.StartInBackground()
	if err != nil {
		t.Fatalf("StartInBackground 出错: %v", err)
	}

	// 用带超时的 client，避免任何意外挂起。
	client := &http.Client{Timeout: 3 * time.Second}

	// 用 OS 分配的实际端口访问端点（验证真实监听 + 端口暴露正确）。
	addr := srv.ListenerAddr()
	if addr == "" {
		t.Fatal("ListenerAddr() 返回空串，StartInBackground 未暴露监听地址")
	}
	resp, err := client.Get("http://" + addr + "/health")
	if err != nil {
		t.Fatalf("GET /health 出错: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("状态码 = %d, want 200", resp.StatusCode)
	}
	var got map[string]string
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("健康响应不是合法 JSON: %v\nbody: %s", err, body)
	}
	if got["status"] != "ok" {
		t.Errorf("status = %q, want \"ok\"", got["status"])
	}

	// 调 closer 验证能优雅关闭（不 panic、返回 nil）。
	if err := closer(); err != nil {
		t.Errorf("closer() 返回错误: %v", err)
	}

	// 关闭后再请求应失败（连接拒绝 / 超时）。
	// 用短超时 client 限制等待时间，跨平台稳定。
	postClient := &http.Client{Timeout: 1 * time.Second}
	if _, err := postClient.Get("http://" + addr + "/health"); err == nil {
		t.Error("关闭后仍能连接，closer 未生效")
	}
}
