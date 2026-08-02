// Command obs 是 obs-lite 的入口：跑 metrics/trace demo 或启动 HTTP 服务。
//
// 用法：
//
//	obs -d metrics        # metrics demo（counter/gauge/histogram）
//	obs -d trace          # trace demo（span 树）
//	obs -d all            # 全部
//	obs -version
//	obs -serve -port 9090 # 启动 HTTP 服务，Prometheus 可 scrape http://localhost:9090/metrics
package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/QiuShichang/obs-lite/internal/exporter"
	"github.com/QiuShichang/obs-lite/internal/metrics"
	"github.com/QiuShichang/obs-lite/internal/server"
	"github.com/QiuShichang/obs-lite/internal/trace"
)

var version = "dev"

func main() {
	demo := flag.String("d", "metrics", "demo: metrics|trace|all")
	serve := flag.Bool("serve", false, "启动 HTTP 服务模式（暴露 /metrics、/api/trace 等端点）")
	port := flag.Int("port", 9090, "HTTP 服务监听端口（配合 -serve）")
	showVer := flag.Bool("version", false, "打印版本号")
	flag.Parse()

	if *showVer {
		fmt.Println("obs-lite", version)
		return
	}

	// -serve 模式优先（启动 HTTP 服务，Prometheus 可 scrape）。
	if *serve {
		runServe(*port)
		return
	}

	switch *demo {
	case "metrics":
		runMetricsDemo()
	case "trace":
		runTraceDemo()
	case "all":
		runMetricsDemo()
		fmt.Println()
		runTraceDemo()
	default:
		fmt.Fprintln(os.Stderr, "用法: obs -d metrics|trace|all | obs -serve -port 9090")
		os.Exit(1)
	}
}

// runServe 启动 HTTP 服务模式：注入示例 metric + trace，启动 Server，阻塞直到 Ctrl+C。
func runServe(port int) {
	reg := metrics.NewRegistry()
	tr := trace.NewTracer()

	// 注入示例 metric，让 /metrics 有内容可看。
	injectSamples(reg, tr)

	addr := fmt.Sprintf(":%d", port)
	srv := server.New(addr, reg, tr)
	fmt.Printf("obs-lite HTTP 服务启动 → http://localhost%s\n", addr)
	fmt.Println("端点：")
	fmt.Println("  GET /metrics      Prometheus 文本格式（可被 prometheus scrape）")
	fmt.Println("  GET /api/metrics  metrics 的 JSON")
	fmt.Println("  GET /api/trace    trace 的 JSON")
	fmt.Println("  GET /health       健康检查")
	fmt.Println("按 Ctrl+C 退出。")

	if err := srv.Start(); err != nil {
		fmt.Fprintln(os.Stderr, "HTTP 服务退出:", err)
		os.Exit(1)
	}
}

// injectSamples 注入示例 metric + trace 数据（demo 用，让端点有内容）。
func injectSamples(reg *metrics.Registry, tr *trace.Tracer) {
	// Counter：HTTP 请求计数
	reqTotal := reg.Counter("http_requests_total")
	reqTotal.Inc(map[string]string{"method": "GET", "status": "200"})
	reqTotal.Inc(map[string]string{"method": "GET", "status": "200"})
	reqTotal.Inc(map[string]string{"method": "POST", "status": "500"})

	// Gauge：活跃连接
	reg.Gauge("active_connections").Set(42, nil)

	// Histogram：请求延迟分布
	latency := reg.Histogram("request_duration_seconds", nil)
	for _, v := range []float64{0.003, 0.02, 0.05, 0.1, 0.3, 0.8, 1.5, 3.0} {
		latency.Observe(v, map[string]string{"endpoint": "/api/users"})
	}

	// Trace：示例调用链
	ctx, root := tr.Start("GET /api/users", nil)
	root.SetAttr("http.method", "GET")
	time.Sleep(2 * time.Millisecond)

	_, db := tr.Start("db.query", ctx)
	db.SetAttr("db.system", "postgres")
	db.SetAttr("db.statement", "SELECT * FROM users")
	time.Sleep(15 * time.Millisecond)
	db.End()

	_, auth := tr.Start("auth.verify", ctx)
	time.Sleep(3 * time.Millisecond)
	auth.End()

	root.End()
}

func runMetricsDemo() {
	reg := metrics.NewRegistry()

	// Counter：模拟 HTTP 请求计数
	reqCounter := reg.Counter("http_requests_total")
	reqCounter.Inc(map[string]string{"method": "GET", "status": "200"})
	reqCounter.Inc(map[string]string{"method": "GET", "status": "200"})
	reqCounter.Inc(map[string]string{"method": "POST", "status": "500"})

	// Gauge：模拟活跃连接数
	connsGauge := reg.Gauge("active_connections")
	connsGauge.Set(42, nil)
	connsGauge.Inc(nil) // 新连接
	connsGauge.Dec(nil) // 断开

	// Histogram：模拟请求延迟分布
	latencyHist := reg.Histogram("request_duration_seconds", nil)
	for _, v := range []float64{0.003, 0.02, 0.05, 0.1, 0.3, 0.8, 1.5, 3.0} {
		latencyHist.Observe(v, map[string]string{"endpoint": "/api/users"})
	}

	fmt.Println("=== Metrics demo ===")
	fmt.Print(exporter.FormatMetricsText(reg))
}

func runTraceDemo() {
	tr := trace.NewTracer()

	// 模拟一次 HTTP 请求的调用链
	ctx, root := tr.Start("GET /api/users", nil)
	root.SetAttr("http.method", "GET")
	root.SetAttr("http.url", "/api/users")

	time.Sleep(2 * time.Millisecond)

	// 子 span：DB 查询
	_, db := tr.Start("db.query", ctx)
	db.SetAttr("db.system", "postgres")
	db.SetAttr("db.statement", "SELECT * FROM users")
	time.Sleep(15 * time.Millisecond)
	db.End()

	// 子 span：缓存查询
	_, cache := tr.Start("cache.get", ctx)
	cache.SetAttr("cache.key", "users:all")
	time.Sleep(1 * time.Millisecond)
	cache.SetError() // 缓存未命中
	cache.AddEvent("miss")
	cache.End()

	// 兄弟 span：认证检查
	_, auth := tr.Start("auth.verify", ctx)
	time.Sleep(3 * time.Millisecond)
	auth.End()

	root.End()

	spans := tr.Collect()
	fmt.Println("=== Trace demo ===")
	fmt.Print(exporter.FormatTraceText(spans))
}
