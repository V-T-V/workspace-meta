package trace

import (
	"testing"
	"time"

	"github.com/QiuShichang/obs-lite/internal/types"
)

func TestSpanBasic(t *testing.T) {
	tr := NewTracer()
	_, span := tr.Start("root", nil)
	span.SetAttr("user", "alice")
	time.Sleep(1 * time.Millisecond)
	span.End()
	spans := tr.Collect()
	if len(spans) != 1 {
		t.Fatalf("应有 1 个 span，实际 %d", len(spans))
	}
	if spans[0].Name != "root" {
		t.Error("span 名应为 root")
	}
	if spans[0].Duration() <= 0 {
		t.Error("duration 应 > 0")
	}
	if spans[0].Attributes["user"] != "alice" {
		t.Error("attr 应含 user=alice")
	}
}

func TestSpanTree(t *testing.T) {
	tr := NewTracer()
	ctx, root := tr.Start("root", nil)
	root.End()
	_, child := tr.Start("child", ctx)
	child.End()
	spans := tr.Collect()
	if len(spans) != 2 {
		t.Fatalf("应有 2 个 span，实际 %d", len(spans))
	}
	// child 应继承 root 的 TraceID
	if spans[0].TraceID != spans[1].TraceID {
		t.Error("父子 span 应共享 TraceID")
	}
	// child 的 ParentID 应是 root 的 SpanID
	var rootSpan, childSpan *types.Span
	for _, s := range spans {
		if s.Name == "root" {
			rootSpan = s
		}
		if s.Name == "child" {
			childSpan = s
		}
	}
	if childSpan.ParentID != rootSpan.SpanID {
		t.Error("child.ParentID 应等于 root.SpanID")
	}
	if rootSpan.ParentID != "" {
		t.Error("root 应无 ParentID")
	}
}

func TestSpanStatus(t *testing.T) {
	tr := NewTracer()
	_, span := tr.Start("op", nil)
	span.SetError()
	span.End()
	spans := tr.Collect()
	if spans[0].Status != types.SpanError {
		t.Error("应为 ERROR 状态")
	}
}

func TestSpanEvents(t *testing.T) {
	tr := NewTracer()
	_, span := tr.Start("op", nil)
	span.AddEvent("log-something")
	span.End()
	spans := tr.Collect()
	if len(spans[0].Events) != 1 {
		t.Error("应有 1 个 event")
	}
	if spans[0].Events[0].Name != "log-something" {
		t.Error("event 名不对")
	}
}

func TestCollectClears(t *testing.T) {
	tr := NewTracer()
	_, span := tr.Start("op", nil)
	span.End()
	if len(tr.Collect()) != 1 {
		t.Error("首次 collect 应有 1 个")
	}
	if len(tr.Collect()) != 0 {
		t.Error("collect 后应清空")
	}
}

func TestMultipleTraces(t *testing.T) {
	tr := NewTracer()
	_, s1 := tr.Start("a", nil)
	s1.End()
	_, s2 := tr.Start("b", nil) // 新 trace
	s2.End()
	spans := tr.Collect()
	if spans[0].TraceID == spans[1].TraceID {
		t.Error("两个根 span 应有不同 TraceID")
	}
}

// TestSampleRateZero rate=0 时全部 span 被丢弃，Collect 应为空。
func TestSampleRateZero(t *testing.T) {
	tr := NewTracer()
	tr.SampleRate = 0
	for i := 0; i < 10; i++ {
		_, span := tr.Start("op", nil)
		span.End()
	}
	if got := tr.Collect(); len(got) != 0 {
		t.Errorf("rate=0 时不应有任何 span，实际 %d 个", len(got))
	}
}

// TestSampleRateFull rate=1 时全部 span 都被记录。
func TestSampleRateFull(t *testing.T) {
	tr := NewTracer()
	tr.SampleRate = 1.0
	const n = 10
	for i := 0; i < n; i++ {
		_, span := tr.Start("op", nil)
		span.End()
	}
	if got := tr.Collect(); len(got) != n {
		t.Errorf("rate=1 时应有 %d 个 span，实际 %d 个", n, len(got))
	}
}

// TestSampleRateDefault NewTracer 默认全采样（rate=1.0），兼容旧调用方。
func TestSampleRateDefault(t *testing.T) {
	tr := NewTracer()
	_, span := tr.Start("op", nil)
	span.End()
	if got := tr.Collect(); len(got) != 1 {
		t.Errorf("默认 SampleRate 应为 1.0 全采样，实际收集 %d 个", len(got))
	}
}

// TestSampleRateNoopSpan 不采样的 span 是 noop：IsNoop 返回 true，End 不写入。
// 但仍可安全调用 SetAttr（不 panic）。
func TestSampleRateNoopSpan(t *testing.T) {
	tr := NewTracer()
	tr.SampleRate = 0
	_, span := tr.Start("op", nil)
	if !span.IsNoop() {
		t.Error("rate=0 时 span 应为 noop")
	}
	// noop span 上调 SetAttr 应安全（不 panic）
	span.SetAttr("k", "v")
	span.End()
	if got := tr.Collect(); len(got) != 0 {
		t.Errorf("noop span 的 End 不应写入收集器，实际 %d 个", len(got))
	}
}

// TestSampleRateBoundary rate<0 视为 0、rate>1 视为 1（边界归一化）。
func TestSampleRateBoundary(t *testing.T) {
	// 负数 → 不采样
	tr := NewTracer()
	tr.SampleRate = -0.5
	_, span := tr.Start("op", nil)
	span.End()
	if len(tr.Collect()) != 0 {
		t.Error("rate=-0.5 应视为 0，无 span")
	}

	// 超过 1 → 全采样
	tr2 := NewTracer()
	tr2.SampleRate = 5
	_, span = tr2.Start("op", nil)
	span.End()
	if len(tr2.Collect()) != 1 {
		t.Error("rate=5 应视为 1，应有 1 个 span")
	}
}

// TestSampleRatePartial 0<rate<1 的采样应确定性：用同一 Tracer 多次决策，
// 同样调用序列在固定种子下产生稳定结果（核心是确定性，不依赖具体命中数）。
func TestSampleRatePartial(t *testing.T) {
	// 两个独立 Tracer，相同 SampleRate + 相同 Start 次数，结果应一致（固定种子）。
	makeTracer := func() *Tracer {
		tr := NewTracer()
		tr.SampleRate = 0.5
		return tr
	}
	tr1, tr2 := makeTracer(), makeTracer()
	const n = 20
	for i := 0; i < n; i++ {
		_, s1 := tr1.Start("op", nil)
		s1.End()
		_, s2 := tr2.Start("op", nil)
		s2.End()
	}
	c1 := tr1.Collect()
	c2 := tr2.Collect()
	if len(c1) != len(c2) {
		t.Errorf("固定种子下两次运行的采样数应一致: %d vs %d", len(c1), len(c2))
	}
	// rate=0.5 时不可能全采或全丢（避免退化，验证 rng 真的参与决策）。
	if len(c1) == 0 || len(c1) == n {
		t.Errorf("rate=0.5 采样数应在 (0,%d)，实际 %d（rng 似乎未生效）", n, len(c1))
	}
}

// TestSampleRateContextPropagation 即使 span 被采样丢弃，
// 返回的 Context 仍应携带正常 TraceID/SpanID，供下游传播不断链。
func TestSampleRateContextPropagation(t *testing.T) {
	tr := NewTracer()
	tr.SampleRate = 0
	ctx, parent := tr.Start("parent", nil)
	parent.End()

	// 用 parent 的 ctx 起子 span（即使父被丢弃，子应继承同一 TraceID）。
	tr.SampleRate = 1.0 // 让子 span 真实记录
	_, child := tr.Start("child", ctx)
	child.End()

	spans := tr.Collect()
	if len(spans) != 1 {
		t.Fatalf("应有 1 个记录的 span，实际 %d", len(spans))
	}
	if spans[0].TraceID != ctx.TraceID {
		t.Error("子 span 应继承父（被丢弃）的 TraceID，保持链路不断")
	}
	if spans[0].ParentID != ctx.SpanID {
		t.Error("子 span 的 ParentID 应是父（被丢弃）的 SpanID")
	}
}
