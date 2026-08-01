// Package types 定义 obs-lite 的共享类型：metrics（counter/gauge/histogram）+ trace span。
package types

import "time"

// MetricKind 标识 metric 类型。
type MetricKind int

const (
	MetricCounter   MetricKind = iota // 单调递增计数器
	MetricGauge                       // 可增可减的瞬时值
	MetricHistogram                   // 直方图（分布）
)

// String 返回 metric 类型的可读名。
func (k MetricKind) String() string {
	switch k {
	case MetricCounter:
		return "counter"
	case MetricGauge:
		return "gauge"
	case MetricHistogram:
		return "histogram"
	}
	return "unknown"
}

// MetricPoint 是单个 metric 数据点。
type MetricPoint struct {
	Name      string
	Kind      MetricKind
	Value     float64           // counter/gauge 用
	Labels    map[string]string // 标签（维度）
	Timestamp time.Time
}

// HistogramBucket 是直方图的一个桶。
type HistogramBucket struct {
	UpperBound float64 // 桶的上界（+Inf 桶用 math.Inf(1)）
	Count      uint64  // 该桶的累计计数
}

// HistogramData 是直方图的完整数据。
type HistogramData struct {
	Name    string
	Labels  map[string]string
	Buckets []HistogramBucket
	Sum     float64
	Count   uint64
}

// Span 是 trace 的一个 span（分布式调用的一个节点）。
type Span struct {
	TraceID    string // 同一次分布式调用共用
	SpanID     string // 本 span 唯一
	ParentID   string // 父 span（根 span 为空）
	Name       string // span 名（如 "db.query"）
	StartTime  time.Time
	EndTime    time.Time
	Attributes map[string]string // 自定义属性
	Events     []SpanEvent       // span 内的事件（如错误、日志）
	Status     SpanStatus
}

// SpanStatus span 的执行状态。
type SpanStatus int

const (
	SpanOK SpanStatus = iota
	SpanError
	SpanCancelled
)

// String 返回状态名。
func (s SpanStatus) String() string {
	switch s {
	case SpanOK:
		return "OK"
	case SpanError:
		return "ERROR"
	case SpanCancelled:
		return "CANCELLED"
	}
	return "UNKNOWN"
}

// Duration 返回 span 持续时间。
func (s Span) Duration() time.Duration {
	return s.EndTime.Sub(s.StartTime)
}

// SpanEvent 是 span 内的一个事件。
type SpanEvent struct {
	Name       string
	Timestamp  time.Time
	Attributes map[string]string
}
