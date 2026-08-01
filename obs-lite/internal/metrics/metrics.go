// Package metrics 实现 counter / gauge / histogram 三种 metric + registry。
//
// 设计对齐 Prometheus 数据模型（但零依赖，无 HTTP 端点）：
//   - Counter：单调递增（如 requests_total）
//   - Gauge：可增可减（如 active_connections）
//   - Histogram：分布（如 request_duration_seconds）
//
// 用 Registry 收集所有 metric，可导出为文本/Prometheus 格式。
package metrics

import (
	"fmt"
	"math"
	"sort"
	"sync"
	"time"

	"github.com/QiuShichang/obs-lite/internal/types"
)

// Counter 是单调递增计数器。
type Counter struct {
	mu     sync.Mutex
	name   string
	values map[string]float64 // labelsHash → value
}

// NewCounter 创建 Counter。
func NewCounter(name string) *Counter {
	return &Counter{name: name, values: map[string]float64{}}
}

// Inc 增加 1（带标签）。
func (c *Counter) Inc(labels map[string]string) { c.Add(1, labels) }

// Add 增加指定值（必须 >= 0）。
func (c *Counter) Add(v float64, labels map[string]string) {
	if v < 0 {
		return // counter 不允许减
	}
	c.mu.Lock()
	c.values[labelsKey(labels)] += v
	c.mu.Unlock()
}

// Get 返回某标签组合的当前值。
func (c *Counter) Get(labels map[string]string) float64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.values[labelsKey(labels)]
}

// Points 导出为 MetricPoint 切表（每个标签组合一个点）。
func (c *Counter) Points() []types.MetricPoint {
	c.mu.Lock()
	defer c.mu.Unlock()
	var out []types.MetricPoint
	for k, v := range c.values {
		out = append(out, types.MetricPoint{
			Name: c.name, Kind: types.MetricCounter, Value: v,
			Labels: parseLabelsKey(k), Timestamp: time.Now(),
		})
	}
	return out
}

// Gauge 是可增可减的瞬时值。
type Gauge struct {
	mu     sync.Mutex
	name   string
	values map[string]float64
}

func NewGauge(name string) *Gauge { return &Gauge{name: name, values: map[string]float64{}} }

func (g *Gauge) Set(v float64, labels map[string]string) {
	g.mu.Lock()
	g.values[labelsKey(labels)] = v
	g.mu.Unlock()
}

func (g *Gauge) Inc(labels map[string]string) { g.Add(1, labels) }
func (g *Gauge) Dec(labels map[string]string) { g.Add(-1, labels) }

func (g *Gauge) Add(delta float64, labels map[string]string) {
	g.mu.Lock()
	g.values[labelsKey(labels)] += delta
	g.mu.Unlock()
}

func (g *Gauge) Get(labels map[string]string) float64 {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.values[labelsKey(labels)]
}

func (g *Gauge) Points() []types.MetricPoint {
	g.mu.Lock()
	defer g.mu.Unlock()
	var out []types.MetricPoint
	for k, v := range g.values {
		out = append(out, types.MetricPoint{
			Name: g.name, Kind: types.MetricGauge, Value: v,
			Labels: parseLabelsKey(k), Timestamp: time.Now(),
		})
	}
	return out
}

// CallbackGauge 是“每次导出即调用”的 Gauge：值不手动 Set，
// 而是在每次 Points() 调用时执行回调动态获取当前值。
//
// 适用场景：采集系统运行时指标——内存占用、goroutine 数、
// 打开的文件句柄数、CPU 负载等——这些值在导出瞬间才有意义，
// 没必要也无法持续 Set。CallbackGauge 把"何时采样"的决策权交给 Registry。
//
// 与 Gauge 的区别：Gauge 是"写入型"（业务代码 Set/Add），CallbackGauge 是
// "拉取型"（导出时回调）；Prometheus 中对应的 client_golang 同样提供
// GaugeFunc / Collector 接口实现该模式。
type CallbackGauge struct {
	name     string
	callback func() float64
	labels   map[string]string
}

// NewCallbackGauge 创建回调 Gauge。
//
// callback 必须非空且线程安全（Points 可能在并发导出时被调用）。
// 若 callback 为 nil，Points 返回空切片（避免 panic）。
func NewCallbackGauge(name string, callback func() float64) *CallbackGauge {
	return &CallbackGauge{name: name, callback: callback}
}

// WithLabels 给本回调 Gauge 绑定一组固定标签，每次导出都带上。
// 返回 g 自身以便链式调用。labels 为 nil 表示无标签。
func (g *CallbackGauge) WithLabels(labels map[string]string) *CallbackGauge {
	g.labels = labels
	return g
}

// Points 导出当前回调值：执行 callback 取一次瞬时值，包装成单个 MetricPoint。
//
// 与 Counter/Gauge 的 Points 不同，这里每次返回的 Value 都可能不同
// （取决于 callback 此刻读到什么）。callback 为 nil 时返回 nil。
func (g *CallbackGauge) Points() []types.MetricPoint {
	if g.callback == nil {
		return nil
	}
	return []types.MetricPoint{{
		Name: g.name, Kind: types.MetricGauge, Value: g.callback(),
		Labels: g.labels, Timestamp: time.Now(),
	}}
}

// Histogram 是分布直方图。
type Histogram struct {
	mu      sync.Mutex
	name    string
	buckets []float64 // 桶上界（排序）
	// 每个 labelsHash 一组数据
	counts map[string][]uint64 // 每桶计数
	sums   map[string]float64
	total  map[string]uint64
}

// NewHistogram 创建 Histogram。buckets 必须排序且不含 +Inf（自动加）。
// 默认用 Prometheus 推荐的 10 个桶。
func NewHistogram(name string, buckets []float64) *Histogram {
	if len(buckets) == 0 {
		buckets = []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10}
	}
	// 确保排序
	sorted := append([]float64{}, buckets...)
	sort.Float64s(sorted)
	return &Histogram{
		name: name, buckets: sorted,
		counts: map[string][]uint64{}, sums: map[string]float64{}, total: map[string]uint64{},
	}
}

// Observe 记录一个观测值。
func (h *Histogram) Observe(v float64, labels map[string]string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	key := labelsKey(labels)
	if _, ok := h.counts[key]; !ok {
		h.counts[key] = make([]uint64, len(h.buckets)+1) // +1 是 +Inf 桶
	}
	// 累加到所有上界 >= v 的桶
	for i, b := range h.buckets {
		if v <= b {
			h.counts[key][i]++
		}
	}
	h.counts[key][len(h.buckets)]++ // +Inf 桶总是 +1
	h.sums[key] += v
	h.total[key]++
}

// Data 导出直方图数据。
func (h *Histogram) Data() []types.HistogramData {
	h.mu.Lock()
	defer h.mu.Unlock()
	var out []types.HistogramData
	for key, counts := range h.counts {
		var buckets []types.HistogramBucket
		for i, b := range h.buckets {
			buckets = append(buckets, types.HistogramBucket{UpperBound: b, Count: counts[i]})
		}
		buckets = append(buckets, types.HistogramBucket{UpperBound: math.Inf(1), Count: counts[len(h.buckets)]})
		out = append(out, types.HistogramData{
			Name: h.name, Labels: parseLabelsKey(key),
			Buckets: buckets, Sum: h.sums[key], Count: h.total[key],
		})
	}
	return out
}

// Registry 收集所有 metric（counter/gauge/histogram/callbackGauge）。
type Registry struct {
	mu             sync.Mutex
	counters       map[string]*Counter
	gauges         map[string]*Gauge
	histograms     map[string]*Histogram
	callbackGauges map[string]*CallbackGauge
}

// NewRegistry 创建空 Registry。
func NewRegistry() *Registry {
	return &Registry{
		counters: map[string]*Counter{}, gauges: map[string]*Gauge{}, histograms: map[string]*Histogram{},
		callbackGauges: map[string]*CallbackGauge{},
	}
}

// Counter 获取或创建 counter（按 name）。
func (r *Registry) Counter(name string) *Counter {
	r.mu.Lock()
	defer r.mu.Unlock()
	if c, ok := r.counters[name]; ok {
		return c
	}
	c := NewCounter(name)
	r.counters[name] = c
	return c
}

// Gauge 获取或创建 gauge。
func (r *Registry) Gauge(name string) *Gauge {
	r.mu.Lock()
	defer r.mu.Unlock()
	if g, ok := r.gauges[name]; ok {
		return g
	}
	g := NewGauge(name)
	r.gauges[name] = g
	return g
}

// Histogram 获取或创建 histogram。
func (r *Registry) Histogram(name string, buckets []float64) *Histogram {
	r.mu.Lock()
	defer r.mu.Unlock()
	if h, ok := r.histograms[name]; ok {
		return h
	}
	h := NewHistogram(name, buckets)
	r.histograms[name] = h
	return h
}

// CallbackGauge 注册一个回调 Gauge（按 name，重复注册返回已存在的实例）。
// callback 在每次导出（AllPoints）时被调用以获取瞬时值。
// 适合采集 runtime 指标（goroutine 数、内存占用等），无需手动 Set。
func (r *Registry) CallbackGauge(name string, cb func() float64) *CallbackGauge {
	r.mu.Lock()
	defer r.mu.Unlock()
	if g, ok := r.callbackGauges[name]; ok {
		return g
	}
	g := NewCallbackGauge(name, cb)
	r.callbackGauges[name] = g
	return g
}

// AllPoints 导出所有 counter/gauge/callbackGauge 的点。
func (r *Registry) AllPoints() []types.MetricPoint {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []types.MetricPoint
	for _, c := range r.counters {
		out = append(out, c.Points()...)
	}
	for _, g := range r.gauges {
		out = append(out, g.Points()...)
	}
	for _, cg := range r.callbackGauges {
		out = append(out, cg.Points()...)
	}
	return out
}

// AllHistograms 导出所有 histogram 数据。
func (r *Registry) AllHistograms() []types.HistogramData {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []types.HistogramData
	for _, h := range r.histograms {
		out = append(out, h.Data()...)
	}
	return out
}

// ===== 辅助：labels 序列化 =====

// labelsKey 把 labels map 转成确定性字符串（key 排序）。
func labelsKey(labels map[string]string) string {
	if len(labels) == 0 {
		return ""
	}
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := ""
	for _, k := range keys {
		out += k + "=" + labels[k] + ","
	}
	return out
}

// parseLabelsKey 反序列化 labelsKey（简化：不处理值里的 =/,）。
func parseLabelsKey(s string) map[string]string {
	if s == "" {
		return nil
	}
	out := map[string]string{}
	for _, pair := range split(s, ',') {
		if pair == "" {
			continue
		}
		kv := split(pair, '=')
		if len(kv) == 2 {
			out[kv[0]] = kv[1]
		}
	}
	return out
}

func split(s string, sep byte) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == sep {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	out = append(out, s[start:])
	return out
}

// FormatPoints 把 MetricPoints 格式化成可读文本（调试用）。
func FormatPoints(points []types.MetricPoint) string {
	var out string
	for _, p := range points {
		out += fmt.Sprintf("%s{%s} = %.2f [%s]\n", p.Name, formatLabels(p.Labels), p.Value, p.Kind)
	}
	return out
}

func formatLabels(labels map[string]string) string {
	if len(labels) == 0 {
		return ""
	}
	return labelsKey(labels)
}
