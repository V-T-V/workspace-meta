package exporter

import (
	"math"
	"strings"
	"testing"
	"time"

	"github.com/QiuShichang/obs-lite/internal/metrics"
	"github.com/QiuShichang/obs-lite/internal/types"
)

// ===== FormatMetricsText =====

// TestFormatMetricsTextCounterGauge counter 与 gauge 的输出格式正确（带 kind 标签 + 值）。
func TestFormatMetricsTextCounterGauge(t *testing.T) {
	reg := metrics.NewRegistry()
	reg.Counter("requests_total").Inc(map[string]string{"method": "GET"})
	reg.Gauge("active_conns").Set(7, map[string]string{"host": "a"})

	out := FormatMetricsText(reg)

	// counter 行：name + labels + (counter) 标签 + 整数值
	if !strings.Contains(out, "requests_total") || !strings.Contains(out, "(counter)") {
		t.Errorf("counter 输出缺字段:\n%s", out)
	}
	// 值应为整数 1（counter 增 1），不应出现 1.00 之类
	if !strings.Contains(out, "requests_total{method=\"GET\"} (counter) 1") {
		t.Errorf("counter 行格式不对:\n%s", out)
	}
	// gauge 行：name + labels + (gauge) 标签 + 整数值
	if !strings.Contains(out, "active_conns") || !strings.Contains(out, "(gauge)") {
		t.Errorf("gauge 输出缺字段:\n%s", out)
	}
	if !strings.Contains(out, "active_conns{host=\"a\"} (gauge) 7") {
		t.Errorf("gauge 行格式不对:\n%s", out)
	}
}

// TestFormatMetricsTextNoKindTagForHistogramPoint 非 counter/gauge 的 kind 不输出 kind 标签。
// （Registry 只收 counter/gauge 的 point，这里验证计数格式只对这两种 kind 标注。）
func TestFormatMetricsTextNoKindTagForUnknownKind(t *testing.T) {
	// 直接构造含 histogram kind 的 point 列表绕过 registry 不易，改为验证 formatKindTag 的行为
	if got := formatKindTag(types.MetricHistogram); got != "" {
		t.Errorf("histogram kind 不应有标签，got %q", got)
	}
	if got := formatKindTag(types.MetricCounter); got != " (counter)" {
		t.Errorf("counter 标签应为 ' (counter)'，got %q", got)
	}
	if got := formatKindTag(types.MetricGauge); got != " (gauge)" {
		t.Errorf("gauge 标签应为 ' (gauge)'，got %q", got)
	}
}

// TestFormatMetricsTextDeterministic 多次调用输出完全一致（验证 sortPoints 起作用，不受 map 迭代顺序影响）。
func TestFormatMetricsTextDeterministic(t *testing.T) {
	reg := metrics.NewRegistry()
	// 多 counter 多标签，制造 map 迭代无序的测试场景
	c := reg.Counter("req")
	for _, m := range []string{"GET", "POST", "PUT", "DELETE", "PATCH"} {
		c.Inc(map[string]string{"method": m})
	}
	g := reg.Gauge("temp")
	for _, h := range []string{"a", "b", "c", "d"} {
		g.Set(float64(len(h)), map[string]string{"host": h})
	}

	first := FormatMetricsText(reg)
	// 跑 20 次，每次都必须一致
	for i := 0; i < 20; i++ {
		got := FormatMetricsText(reg)
		if got != first {
			t.Fatalf("第 %d 次输出与首次不一致（确定性破坏）:\nfirst:\n%s\ngot:\n%s", i+1, first, got)
		}
	}
}

// TestFormatMetricsTextEmpty 空注册表输出应为空字符串。
func TestFormatMetricsTextEmpty(t *testing.T) {
	reg := metrics.NewRegistry()
	if got := FormatMetricsText(reg); got != "" {
		t.Errorf("空 registry 应输出空串，got %q", got)
	}
}

// TestFormatMetricsTextHistogramBucketFormat 直方图输出含 bucket/sum/count。
func TestFormatMetricsTextHistogramBucketFormat(t *testing.T) {
	reg := metrics.NewRegistry()
	h := reg.Histogram("latency", []float64{0.1, 0.5})
	h.Observe(0.05, nil)
	h.Observe(0.2, nil)

	out := FormatMetricsText(reg)
	if !strings.Contains(out, "latency") {
		t.Errorf("应包含 histogram 名")
	}
	if !strings.Contains(out, "bucket{le=") {
		t.Errorf("应包含 bucket 行:\n%s", out)
	}
	if !strings.Contains(out, "sum =") || !strings.Contains(out, "count =") {
		t.Errorf("应包含 sum/count 行:\n%s", out)
	}
}

// ===== FormatMetricsPrometheus =====

// TestFormatMetricsPrometheusTypeHeader 每个 metric name 只输出一次 # TYPE 头，kind 正确。
func TestFormatMetricsPrometheusTypeHeader(t *testing.T) {
	reg := metrics.NewRegistry()
	c := reg.Counter("requests_total")
	c.Inc(map[string]string{"method": "GET"})
	c.Inc(map[string]string{"method": "POST"})
	g := reg.Gauge("active_conns")
	g.Set(3, map[string]string{"host": "a"})

	out := FormatMetricsPrometheus(reg)

	// TYPE 头计数：counter 1 个、gauge 1 个，总共恰好 2 个
	if n := strings.Count(out, "# TYPE "); n != 2 {
		t.Errorf("应输出 2 个 TYPE 头，实际 %d:\n%s", n, out)
	}
	if !strings.Contains(out, "# TYPE requests_total counter") {
		t.Errorf("缺 counter TYPE 头:\n%s", out)
	}
	if !strings.Contains(out, "# TYPE active_conns gauge") {
		t.Errorf("缺 gauge TYPE 头:\n%s", out)
	}
}

// TestFormatMetricsPrometheusSameNameSingleType 同一 metric 多个标签组合时只输出一个 TYPE 头。
func TestFormatMetricsPrometheusSameNameSingleType(t *testing.T) {
	reg := metrics.NewRegistry()
	c := reg.Counter("req")
	for _, m := range []string{"GET", "POST", "PUT"} {
		c.Inc(map[string]string{"method": m})
	}
	out := FormatMetricsPrometheus(reg)
	// 同名只应有一个 TYPE 头
	if strings.Count(out, "# TYPE req counter") != 1 {
		t.Errorf("同名 metric 只应一个 TYPE 头:\n%s", out)
	}
	// 应有 3 行数据
	if strings.Count(out, "req{") != 3 {
		t.Errorf("应有 3 行数据行:\n%s", out)
	}
}

// TestFormatMetricsPrometheusFormatCompliant 每个数据行格式为 "name{labels} value"，
// 且出现在对应 TYPE 头之后。
func TestFormatMetricsPrometheusFormatCompliant(t *testing.T) {
	reg := metrics.NewRegistry()
	reg.Counter("hits").Inc(nil)
	out := FormatMetricsPrometheus(reg)

	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) < 2 {
		t.Fatalf("输出行数太少:\n%s", out)
	}
	// 第一行应是 TYPE 头
	if !strings.HasPrefix(lines[0], "# TYPE hits counter") {
		t.Errorf("第一行应是 TYPE 头，got %q", lines[0])
	}
	// 第二行应是数据行
	if !strings.HasPrefix(lines[1], "hits ") {
		t.Errorf("第二行应是数据行，got %q", lines[1])
	}
}

// TestFormatMetricsPrometheusDeterministic 同上，多次输出一致。
func TestFormatMetricsPrometheusDeterministic(t *testing.T) {
	reg := metrics.NewRegistry()
	c := reg.Counter("r")
	for _, m := range []string{"a", "b", "c", "d", "e", "f"} {
		c.Inc(map[string]string{"m": m})
	}
	first := FormatMetricsPrometheus(reg)
	for i := 0; i < 20; i++ {
		if got := FormatMetricsPrometheus(reg); got != first {
			t.Fatalf("Prometheus 输出确定性破坏（第 %d 次）", i+1)
		}
	}
}

// ===== FormatTraceText =====

// TestFormatTraceTextTree 树形输出根 span + 子 span（缩进表示层级）。
func TestFormatTraceTextTree(t *testing.T) {
	now := time.Now()
	spans := []*types.Span{
		{
			TraceID: "t1", SpanID: "root", ParentID: "",
			Name: "http.request", StartTime: now, EndTime: now.Add(50 * time.Millisecond),
			Status: types.SpanOK,
		},
		{
			TraceID: "t1", SpanID: "child", ParentID: "root",
			Name: "db.query", StartTime: now, EndTime: now.Add(10 * time.Millisecond),
			Status: types.SpanError,
		},
	}
	out := FormatTraceText(spans)
	// 应包含 trace 头
	if !strings.Contains(out, "=== Trace t1 (2 spans) ===") {
		t.Errorf("缺 trace 头:\n%s", out)
	}
	// 应包含两个 span 名
	if !strings.Contains(out, "http.request") || !strings.Contains(out, "db.query") {
		t.Errorf("缺 span 名:\n%s", out)
	}
	// 状态应可见
	if !strings.Contains(out, "[OK]") || !strings.Contains(out, "[ERROR]") {
		t.Errorf("缺状态标签:\n%s", out)
	}
}

// TestFormatTraceTextEmpty 空 span 列表返回占位文本。
func TestFormatTraceTextEmpty(t *testing.T) {
	out := FormatTraceText(nil)
	if out == "" {
		t.Error("空 span 不应返回空串")
	}
	if !strings.Contains(out, "无 span") {
		t.Errorf("空 span 应返回占位提示，got %q", out)
	}
	out2 := FormatTraceText([]*types.Span{})
	if out2 != out {
		t.Errorf("nil 和空切片应同样处理")
	}
}

// TestFormatTraceTextMultipleTraces 多 trace 分组输出，每组一个头。
func TestFormatTraceTextMultipleTraces(t *testing.T) {
	now := time.Now()
	spans := []*types.Span{
		{TraceID: "t1", SpanID: "a", ParentID: "", Name: "x", StartTime: now, EndTime: now, Status: types.SpanOK},
		{TraceID: "t2", SpanID: "b", ParentID: "", Name: "y", StartTime: now, EndTime: now, Status: types.SpanOK},
	}
	out := FormatTraceText(spans)
	if strings.Count(out, "=== Trace ") != 2 {
		t.Errorf("应有两个 trace 头:\n%s", out)
	}
}

// TestFormatTraceTextAttributes span 带 Attributes 时输出大括号属性。
func TestFormatTraceTextAttributes(t *testing.T) {
	now := time.Now()
	spans := []*types.Span{
		{
			TraceID: "t", SpanID: "s", ParentID: "", Name: "n",
			StartTime: now, EndTime: now, Status: types.SpanOK,
			Attributes: map[string]string{"k": "v"},
		},
	}
	out := FormatTraceText(spans)
	if !strings.Contains(out, "{k=v}") {
		t.Errorf("应输出属性 {k=v}:\n%s", out)
	}
}

// ===== formatBound 回归测试 =====

// TestFormatBound005Vs025NotConflict 回归测试：0.005 与 0.025 在输出上必须可区分。
// 这是之前修复的 bug：用 %.2f 时两者都被渲染成 "0.01"，导致 Prometheus le 标签冲突。
// 现在 formatBound 用 %g 保留原始精度，二者必须不同。
func TestFormatBound005Vs025NotConflict(t *testing.T) {
	got005 := formatBound(0.005)
	got025 := formatBound(0.025)
	if got005 == got025 {
		t.Errorf("0.005 与 0.025 输出冲突（两者都=%q）—— 回归 bug 复现！", got005)
	}
	if got005 != "0.005" {
		t.Errorf("formatBound(0.005) 应为 \"0.005\"，got %q", got005)
	}
	if got025 != "0.025" {
		t.Errorf("formatBound(0.025) 应为 \"0.025\"，got %q", got025)
	}
}

// TestFormatBoundInf +Inf 桶输出 "+Inf"。
func TestFormatBoundInf(t *testing.T) {
	if got := formatBound(math.Inf(1)); got != "+Inf" {
		t.Errorf("+Inf 桶应为 \"+Inf\"，got %q", got)
	}
}

// TestFormatBoundOtherBounds 其他默认 Prometheus 桶边界渲染正确（不丢精度）。
func TestFormatBoundOtherBounds(t *testing.T) {
	cases := map[float64]string{
		0.01: "0.01",
		0.05: "0.05",
		0.1:  "0.1",
		0.25: "0.25",
		0.5:  "0.5",
		1:    "1",
		2.5:  "2.5",
		5:    "5",
		10:   "10",
	}
	for in, want := range cases {
		if got := formatBound(in); got != want {
			t.Errorf("formatBound(%v) = %q, want %q", in, got, want)
		}
	}
}

// ===== sortPoints / sortHistograms 确定性 =====

// TestSortPointsDeterministic sortPoints 多次排序结果一致（确定性）。
func TestSortPointsDeterministic(t *testing.T) {
	mk := func(name, lk string) types.MetricPoint {
		return types.MetricPoint{Name: name, Labels: map[string]string{"k": lk}, Value: 1}
	}
	// 故意乱序 + 多个同名不同 label
	input := []types.MetricPoint{
		mk("z", "1"), mk("a", "3"), mk("a", "1"), mk("m", "2"), mk("a", "2"), mk("z", "0"),
	}
	// 排一次作为基准
	first := append([]types.MetricPoint{}, input...)
	sortPoints(first)
	// 再排 10 次（每次用新拷贝），都应相等
	for i := 0; i < 10; i++ {
		clone := append([]types.MetricPoint(nil), input...)
		sortPoints(clone)
		for j := range first {
			if first[j].Name != clone[j].Name || first[j].Labels["k"] != clone[j].Labels["k"] {
				t.Fatalf("第 %d 次 sort 结果与首次不同（idx %d）", i+1, j)
			}
		}
	}
}

// TestSortPointsByNameThenLabels 排序键：先 name 字典序，再 labelsKey。
func TestSortPointsByNameThenLabels(t *testing.T) {
	points := []types.MetricPoint{
		{Name: "b", Labels: map[string]string{"k": "2"}},
		{Name: "a", Labels: map[string]string{"k": "2"}},
		{Name: "a", Labels: map[string]string{"k": "1"}},
		{Name: "b", Labels: map[string]string{"k": "1"}},
	}
	sortPoints(points)
	want := []struct{ name, label string }{
		{"a", "1"}, {"a", "2"}, {"b", "1"}, {"b", "2"},
	}
	for i, w := range want {
		if points[i].Name != w.name || points[i].Labels["k"] != w.label {
			t.Errorf("idx %d: got {%s,%s} want {%s,%s}", i, points[i].Name, points[i].Labels["k"], w.name, w.label)
		}
	}
}

// TestSortHistogramsDeterministic sortHistograms 多次结果一致。
func TestSortHistogramsDeterministic(t *testing.T) {
	mk := func(name string, labels map[string]string) types.HistogramData {
		return types.HistogramData{Name: name, Labels: labels}
	}
	input := []types.HistogramData{
		mk("h3", map[string]string{"a": "1"}),
		mk("h1", map[string]string{"a": "2"}),
		mk("h2", map[string]string{"a": "1"}),
		mk("h1", map[string]string{"a": "1"}),
	}
	first := append([]types.HistogramData{}, input...)
	sortHistograms(first)
	for i := 0; i < 10; i++ {
		clone := append([]types.HistogramData(nil), input...)
		sortHistograms(clone)
		for j := range first {
			if first[j].Name != clone[j].Name || first[j].Labels["a"] != clone[j].Labels["a"] {
				t.Fatalf("第 %d 次 sortHistograms 结果与首次不同（idx %d）", i+1, j)
			}
		}
	}
}

// ===== formatValue / formatLabels 辅助函数覆盖 =====

// TestFormatValue 整数输出无小数点，非整数保留两位。
func TestFormatValue(t *testing.T) {
	cases := map[float64]string{
		0:     "0",
		1:     "1",
		-3:    "-3",
		1.5:   "1.50",
		2.555: "2.56", // %.2f 四舍五入
		100:   "100",
	}
	for v, want := range cases {
		if got := formatValue(v); got != want {
			t.Errorf("formatValue(%v) = %q, want %q", v, got, want)
		}
	}
}

// TestFormatLabelsEmpty 空标签返回空串。
func TestFormatLabelsEmpty(t *testing.T) {
	if got := formatLabels(nil); got != "" {
		t.Errorf("nil labels 应返回空串，got %q", got)
	}
	if got := formatLabels(map[string]string{}); got != "" {
		t.Errorf("空 map 应返回空串，got %q", got)
	}
}

// TestFormatLabelsNonEmpty 非空标签应输出 {k="v"} 格式。
func TestFormatLabelsNonEmpty(t *testing.T) {
	got := formatLabels(map[string]string{"method": "GET"})
	if !strings.HasPrefix(got, "{") || !strings.HasSuffix(got, "}") {
		t.Errorf("标签应被 {} 包裹，got %q", got)
	}
	if !strings.Contains(got, "method=\"GET\"") {
		t.Errorf("应含 method=\"GET\"，got %q", got)
	}
}
