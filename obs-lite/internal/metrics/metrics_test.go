package metrics

import (
	"runtime"
	"testing"
)

func TestCounter(t *testing.T) {
	c := NewCounter("requests_total")
	c.Inc(map[string]string{"method": "GET"})
	c.Inc(map[string]string{"method": "GET"})
	c.Inc(map[string]string{"method": "POST"})
	if c.Get(map[string]string{"method": "GET"}) != 2 {
		t.Error("GET counter 应为 2")
	}
	if c.Get(map[string]string{"method": "POST"}) != 1 {
		t.Error("POST counter 应为 1")
	}
}

func TestCounterNoDecrease(t *testing.T) {
	c := NewCounter("c")
	c.Add(5, nil)
	c.Add(-1, nil) // 应被忽略
	if c.Get(nil) != 5 {
		t.Errorf("counter 不应减少，实际 %f", c.Get(nil))
	}
}

func TestGauge(t *testing.T) {
	g := NewGauge("active_conns")
	g.Set(10, nil)
	g.Inc(nil)    // 11
	g.Dec(nil)    // 10
	g.Add(5, nil) // 15
	if g.Get(nil) != 15 {
		t.Errorf("gauge 应为 15，实际 %f", g.Get(nil))
	}
}

func TestGaugeCanDecrease(t *testing.T) {
	g := NewGauge("g")
	g.Set(10, nil)
	g.Add(-3, nil)
	if g.Get(nil) != 7 {
		t.Errorf("gauge 应能减少到 7，实际 %f", g.Get(nil))
	}
}

func TestHistogram(t *testing.T) {
	h := NewHistogram("latency", []float64{0.1, 0.5, 1.0})
	for _, v := range []float64{0.05, 0.2, 0.6, 1.5} {
		h.Observe(v, nil)
	}
	data := h.Data()
	if len(data) != 1 {
		t.Fatalf("应有 1 组数据，实际 %d", len(data))
	}
	d := data[0]
	if d.Count != 4 {
		t.Errorf("count 应为 4，实际 %d", d.Count)
	}
	// 桶 le=0.1 包含 0.05（1 个）
	if d.Buckets[0].Count != 1 {
		t.Errorf("le=0.1 桶应为 1，实际 %d", d.Buckets[0].Count)
	}
	// 桶 le=0.5 包含 0.05+0.2（2 个）
	if d.Buckets[1].Count != 2 {
		t.Errorf("le=0.5 桶应为 2，实际 %d", d.Buckets[1].Count)
	}
	// +Inf 桶包含全部（4 个）
	if d.Buckets[3].Count != 4 {
		t.Errorf("+Inf 桶应为 4，实际 %d", d.Buckets[3].Count)
	}
}

func TestRegistry(t *testing.T) {
	r := NewRegistry()
	c := r.Counter("requests")
	c.Inc(nil)
	g := r.Gauge("conns")
	g.Set(5, nil)
	points := r.AllPoints()
	if len(points) != 2 {
		t.Errorf("应有 2 个 point，实际 %d", len(points))
	}
}

func TestRegistrySameNameReturnsSame(t *testing.T) {
	r := NewRegistry()
	c1 := r.Counter("x")
	c2 := r.Counter("x")
	if c1 != c2 {
		t.Error("同名 counter 应返回同一实例")
	}
}

func TestLabelsIsolation(t *testing.T) {
	c := NewCounter("c")
	c.Inc(map[string]string{"a": "1"})
	c.Inc(map[string]string{"a": "2"})
	if c.Get(map[string]string{"a": "1"}) != 1 {
		t.Error("不同标签应隔离")
	}
}

// ===== CallbackGauge =====

func TestCallbackGaugeFixedValue(t *testing.T) {
	// 回调返回固定值 → Points 返回正确。
	g := NewCallbackGauge("mem_bytes", func() float64 { return 1024 })
	pts := g.Points()
	if len(pts) != 1 {
		t.Fatalf("应有 1 个 point，实际 %d", len(pts))
	}
	if pts[0].Value != 1024 {
		t.Errorf("值应为 1024，实际 %f", pts[0].Value)
	}
	if pts[0].Name != "mem_bytes" {
		t.Errorf("name 应为 mem_bytes，实际 %s", pts[0].Name)
	}
	if pts[0].Kind.String() != "gauge" {
		t.Errorf("kind 应为 gauge，实际 %s", pts[0].Kind)
	}
}

func TestCallbackGaugeDynamicValue(t *testing.T) {
	// 回调每次返回不同值 → 每次导出不同。
	val := 1.0
	g := NewCallbackGauge("counter_dyn", func() float64 {
		v := val
		val += 10
		return v
	})
	// 第一次：1
	if p := g.Points()[0].Value; p != 1 {
		t.Errorf("第一次应为 1，实际 %f", p)
	}
	// 第二次：11
	if p := g.Points()[0].Value; p != 11 {
		t.Errorf("第二次应为 11，实际 %f", p)
	}
	// 第三次：21
	if p := g.Points()[0].Value; p != 21 {
		t.Errorf("第三次应为 21，实际 %f", p)
	}
}

func TestCallbackGaugeLabels(t *testing.T) {
	g := NewCallbackGauge("g", func() float64 { return 0 }).
		WithLabels(map[string]string{"host": "node-1"})
	pts := g.Points()
	if pts[0].Labels["host"] != "node-1" {
		t.Errorf("标签丢失：%v", pts[0].Labels)
	}
}

func TestCallbackGaugeNilCallbackSafe(t *testing.T) {
	// callback 为 nil 不应 panic，返回 nil。
	g := NewCallbackGauge("nil_cb", nil)
	if pts := g.Points(); pts != nil {
		t.Errorf("nil callback 应返回 nil，实际 %v", pts)
	}
}

func TestCallbackGaugeRealRuntime(t *testing.T) {
	// 真实场景：采集 goroutine 数（应随导出时刻变化）。
	g := NewCallbackGauge("goroutines", func() float64 { return float64(runtime.NumGoroutine()) })
	p1 := g.Points()[0].Value
	// 起 5 个 goroutine，下次导出应增大。
	done := make(chan struct{})
	for i := 0; i < 5; i++ {
		go func() { <-done }()
	}
	p2 := g.Points()[0].Value
	if p2 <= p1 {
		t.Errorf("起 goroutine 后数值应增大：%f -> %f", p1, p2)
	}
	close(done)
}

func TestRegistryCallbackGauge(t *testing.T) {
	r := NewRegistry()
	cg := r.CallbackGauge("goroutines", func() float64 { return 7 })
	if cg == nil {
		t.Fatal("CallbackGauge 应返回非 nil")
	}
	// 同名应返回同一实例。
	cg2 := r.CallbackGauge("goroutines", func() float64 { return 999 })
	if cg != cg2 {
		t.Error("同名 callbackGauge 应返回同一实例")
	}
	// AllPoints 应包含回调值。
	pts := r.AllPoints()
	var found bool
	for _, p := range pts {
		if p.Name == "goroutines" && p.Value == 7 {
			found = true
		}
	}
	if !found {
		t.Error("AllPoints 应包含 callbackGauge 的点（值 7）")
	}
}

func TestRegistryMixedMetrics(t *testing.T) {
	// counter + gauge + callbackGauge 同时存在 → AllPoints 全部导出。
	r := NewRegistry()
	r.Counter("req").Inc(nil)
	r.Gauge("conns").Set(3, nil)
	r.CallbackGauge("cpu_load", func() float64 { return 0.42 })
	pts := r.AllPoints()
	if len(pts) != 3 {
		t.Errorf("应有 3 个 point（c+g+cg），实际 %d", len(pts))
	}
}
