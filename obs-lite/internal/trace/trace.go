// Package trace 实现分布式 trace：span 树 + 上下文传播 + 收集。
//
// 设计对齐 OpenTelemetry 数据模型（但零依赖）：
//   - Span：一次操作（如 HTTP 请求、DB 查询）
//   - Trace：一组 span 组成的调用链（共享 TraceID）
//   - ParentID：形成 span 树（分布式调用的父子关系）
//
// 用法：
//
//	tracer := trace.NewTracer()
//	ctx, span := tracer.Start("root", nil)
//	span.SetAttr("user", "alice")
//	defer span.End()
//	// 子 span
//	childCtx, child := tracer.Start("child", ctx)
//	defer child.End()
//	spans := tracer.Collect() // 获取所有 span
package trace

import (
	"math/rand"
	"sync"
	"time"

	"github.com/QiuShichang/obs-lite/internal/types"
)

// Tracer 是 trace 收集器，记录所有 span。
//
// 采样：SampleRate 控制是否记录每个 span（head sampling，采样头）：
//   - 1.0（默认 / 零值视为全采样）：每次 Start 都创建真实 span。
//   - 0.0：永不采样，每次 Start 返回 noop span（End 不写入收集器）。
//   - (0,1)：按概率采样，用固定种子的 *rand.Rand 保证确定性。
//
// 确定性说明：每个 Tracer 持有独立的 *rand.Rand（种子固定），
// 故同一程序里同样的 Start 调用序列在多次运行间产生相同的采样决策。
// （注意：跨进程的"全局唯一性"不是本简化实现的目标。）
type Tracer struct {
	mu     sync.Mutex
	spans  []*types.Span
	idSeq  uint64 // 用于生成 SpanID
	tidSeq uint64 // 用于生成 TraceID

	// SampleRate 控制采样率（0-1）。零值（未设置）等同于全采样（1.0），
	// 以保证 NewTracer 的默认行为兼容旧调用方。
	SampleRate float64

	// rng 是采样用的伪随机源（每个 Tracer 独立、固定种子），保证确定性。
	// 懒初始化：首次 Start 涉及随机决策时创建。
	rng *rand.Rand
}

// sampleRandSeed 是 Tracer 采样随机源的固定种子，保证多次运行结果一致。
const sampleRandSeed int64 = 42

// NewTracer 创建 Tracer，默认全采样（SampleRate=1.0）。
func NewTracer() *Tracer {
	return &Tracer{SampleRate: 1.0}
}

// effectiveRate 返回归一化后的采样率：负数视为 0、未设置(0 由 NewTracer 设为 1)按当前值、>1 视为 1。
// 注意 NewTracer 已把默认设为 1.0，故显式置 0 才是"不采样"。
func (t *Tracer) effectiveRate() float64 {
	switch {
	case t.SampleRate <= 0:
		return 0
	case t.SampleRate >= 1:
		return 1
	default:
		return t.SampleRate
	}
}

// sampled 基于 SampleRate 决定本次 Start 是否采样。
// 用固定种子 rng 保证确定性。rate>=1 恒采样、rate<=0 恒不采样，跳过随机。
func (t *Tracer) sampled() bool {
	rate := t.effectiveRate()
	if rate >= 1 {
		return true
	}
	if rate <= 0 {
		return false
	}
	// 懒初始化 rng（在 mu 保护下，因 Start 调用方已持锁）。
	if t.rng == nil {
		t.rng = rand.New(rand.NewSource(sampleRandSeed))
	}
	return t.rng.Float64() < rate
}

// Start 开始一个新 span。
// parentCtx 为 nil 表示根 span（新 TraceID）；非 nil 继承父 span 的 TraceID + 设 ParentID。
// 返回新的 context（含 span 信息）和 Span 句柄。
//
// 采样：若 Tracer.SampleRate 决定不采样本 span，返回的 Span 是 noop
// （其 End/AddEvent/SetAttr 可安全调用，但 End 不会写入收集器），
// 且 Context 仍携带正常的 TraceID/SpanID，以便下游传播不断链。
func (t *Tracer) Start(name string, parentCtx *Context) (*Context, *Span) {
	t.mu.Lock()
	t.idSeq++
	spanID := formatID(t.idSeq)
	var traceID, parentID string
	if parentCtx != nil {
		traceID = parentCtx.TraceID
		parentID = parentCtx.SpanID
	} else {
		t.tidSeq++
		traceID = formatID(t.tidSeq)
	}
	sampled := t.sampled()
	span := &Span{
		s: &types.Span{
			TraceID:    traceID,
			SpanID:     spanID,
			ParentID:   parentID,
			Name:       name,
			StartTime:  time.Now(),
			Attributes: map[string]string{},
		},
		tracer: t,
		noop:   !sampled,
	}
	t.mu.Unlock()
	ctx := &Context{TraceID: traceID, SpanID: spanID}
	return ctx, span
}

// Collect 返回所有已结束的 span（按开始时间排序），并清空内部存储。
func (t *Tracer) Collect() []*types.Span {
	t.mu.Lock()
	defer t.mu.Unlock()
	out := t.spans
	t.spans = nil
	// 按开始时间排序
	sortByStart(out)
	return out
}

// Peek 返回所有 span（含未结束的），不清空（调试用）。
func (t *Tracer) Peek() []*types.Span {
	t.mu.Lock()
	defer t.mu.Unlock()
	out := make([]*types.Span, len(t.spans))
	copy(out, t.spans)
	return out
}

func sortByStart(spans []*types.Span) {
	// 简单插入排序（span 数通常不多）
	for i := 1; i < len(spans); i++ {
		for j := i; j > 0 && spans[j].StartTime.Before(spans[j-1].StartTime); j-- {
			spans[j], spans[j-1] = spans[j-1], spans[j]
		}
	}
}

// Context 传播 trace 上下文（跨函数/跨服务）。
type Context struct {
	TraceID string
	SpanID  string
}

// Span 是 span 的句柄（操作封装 *types.Span + 自动 End 注册到 Tracer）。
//
// 线程安全说明（对齐 OpenTelemetry Go SDK 的约定）：
//   - Tracer 本身（Start/Collect/Peek）线程安全，用 sync.Mutex 保护。
//   - 同一个 Span 句柄上的方法（SetAttr/AddEvent/SetError/End）不是线程安全的，
//     应由创建该 span 的 goroutine 串行调用（与 OTel Go 一致：span 不支持并发写）。
//   - 跨 goroutine 传递应通过 Context 传播 TraceID/SpanID 后另开新 span，而非共享句柄并发写。
//
// noop=true 表示此 span 因采样被丢弃（见 Tracer.SampleRate）：
// SetAttr/AddEvent/SetError 仍可安全调用（操作内部 *types.Span，不写入收集器），
// End 不会把该 span 写入 Tracer。
type Span struct {
	s      *types.Span
	tracer *Tracer
	ended  bool
	noop   bool
}

// IsNoop 报告此 span 是否为采样的丢弃 noop span（见 Tracer.SampleRate）。
func (s *Span) IsNoop() bool { return s.noop }

// SetAttr 设置 span 属性。
// 非线程安全：不要在多个 goroutine 并发调用同一 Span 的 SetAttr（见 Span 文档）。
func (s *Span) SetAttr(key, value string) *Span {
	s.s.Attributes[key] = value
	return s
}

// AddEvent 添加 span 事件。
// 非线程安全：不要在多个 goroutine 并发调用同一 Span 的 AddEvent（见 Span 文档）。
func (s *Span) AddEvent(name string) *Span {
	s.s.Events = append(s.s.Events, types.SpanEvent{
		Name: name, Timestamp: time.Now(),
	})
	return s
}

// SetError 标记 span 为错误状态。
// 非线程安全：不要在多个 goroutine 并发调用同一 Span 的 SetError（见 Span 文档）。
func (s *Span) SetError() *Span {
	s.s.Status = types.SpanError
	return s
}

// End 结束 span（注册到 Tracer）。
// noop span（被采样丢弃）不写入，但调用仍安全（幂等）。
func (s *Span) End() {
	if s.ended {
		return
	}
	s.ended = true
	if s.noop {
		return // 采样丢弃的 span 不进入收集器
	}
	s.s.EndTime = time.Now()
	s.tracer.mu.Lock()
	s.tracer.spans = append(s.tracer.spans, s.s)
	s.tracer.mu.Unlock()
}

// Inner 返回内部 *types.Span（只读用）。
func (s *Span) Inner() *types.Span { return s.s }

// formatID 把序号转成 16 进制字符串（简化 ID 生成，零依赖）。
func formatID(n uint64) string {
	if n == 0 {
		return "0"
	}
	const hex = "0123456789abcdef"
	var buf [16]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = hex[n&0xf]
		n >>= 4
	}
	return string(buf[i:])
}
