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
