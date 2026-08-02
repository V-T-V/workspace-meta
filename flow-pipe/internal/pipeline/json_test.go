package pipeline

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestToJSONSimple 3 步线性管道：验证 JSON 含 Name + 3 个 Steps，且字段为 snake_case。
func TestToJSONSimple(t *testing.T) {
	p := Pipeline{Name: "csv-to-sqlite", Steps: []Step{
		{ID: "read", Kind: KindSource, Connector: "csv"},
		{ID: "filter", Kind: KindTransform, Connector: "filter", DependsOn: []string{"read"}},
		{ID: "write", Kind: KindSink, Connector: "sqlite", DependsOn: []string{"filter"}},
	}}
	s, err := p.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON 出错: %v", err)
	}

	// 应是合法 JSON。
	var got map[string]any
	if err := json.Unmarshal([]byte(s), &got); err != nil {
		t.Fatalf("输出不是合法 JSON: %v\n%s", err, s)
	}

	// Name 字段。
	if got["Name"] != "csv-to-sqlite" {
		t.Errorf("Name 应为 csv-to-sqlite，实际 %v", got["Name"])
	}

	// 3 个 step。
	steps, ok := got["Steps"].([]any)
	if !ok {
		t.Fatalf("Steps 应为数组，实际 %T", got["Steps"])
	}
	if len(steps) != 3 {
		t.Fatalf("应有 3 个 step，实际 %d", len(steps))
	}

	// snake_case 字段名应出现（depends_on / connector / kind / id）。
	for _, key := range []string{`"id"`, `"kind"`, `"connector"`, `"depends_on"`} {
		if !strings.Contains(s, key) {
			t.Errorf("输出应含 snake_case 字段 %s，实际:\n%s", key, s)
		}
	}

	// 验证每个 step 的 id 和 connector。
	first, _ := steps[0].(map[string]any)
	if first["id"] != "read" || first["connector"] != "csv" || first["kind"] != "source" {
		t.Errorf("第一步字段不对: %v", first)
	}
}

// TestToJSONRoundTrip 序列化后再反序列化，字段应保持一致。
func TestToJSONRoundTrip(t *testing.T) {
	orig := Pipeline{Name: "etl", Steps: []Step{
		{
			ID: "src", Kind: KindSource, Connector: "csv",
			Config: map[string]any{"path": "in.csv"},
		},
		{
			ID: "tf", Kind: KindTransform, Connector: "filter",
			Config: map[string]any{"where": "x > 0"}, DependsOn: []string{"src"},
			Retry: 3,
		},
		{
			ID: "out", Kind: KindSink, Connector: "sqlite",
			Config: map[string]any{"path": "o.db"}, DependsOn: []string{"tf"},
			DeadLetter: &DeadLetterConfig{
				Connector: "csv",
				Config:    map[string]any{"path": "dead.csv"},
			},
		},
	}}
	s, err := orig.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON 出错: %v", err)
	}

	var back Pipeline
	if err := json.Unmarshal([]byte(s), &back); err != nil {
		t.Fatalf("反序列化失败: %v", err)
	}
	if back.Name != orig.Name {
		t.Errorf("Name 应 %q，实际 %q", orig.Name, back.Name)
	}
	if len(back.Steps) != len(orig.Steps) {
		t.Fatalf("Steps 数应 %d，实际 %d", len(orig.Steps), len(back.Steps))
	}
	// 校验每一步关键字段。
	for i, want := range orig.Steps {
		got := back.Steps[i]
		if got.ID != want.ID || got.Kind != want.Kind || got.Connector != want.Connector {
			t.Errorf("step %d: got=%+v want=%+v", i, got, want)
		}
		if got.Retry != want.Retry {
			t.Errorf("step %d retry: got=%d want=%d", i, got.Retry, want.Retry)
		}
		if len(got.DependsOn) != len(want.DependsOn) {
			t.Errorf("step %d depends_on 长度不符", i)
		}
	}
	// 第三步的死信配置应保留。
	dl := back.Steps[2].DeadLetter
	if dl == nil || dl.Connector != "csv" {
		t.Errorf("第三步死信配置应保留，实际 %v", dl)
	}
}

// TestToJSONEmpty 空管道：Steps 应是 [] 而非 null，且 Name 保留。
func TestToJSONEmpty(t *testing.T) {
	p := Pipeline{Name: "empty", Steps: nil}
	s, err := p.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON 出错: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(s), &got); err != nil {
		t.Fatalf("输出不是合法 JSON: %v", err)
	}
	steps, ok := got["Steps"].([]any)
	if !ok {
		t.Errorf("空管道 Steps 应为空数组 []，实际 %T（可能为 null）", got["Steps"])
	}
	if len(steps) != 0 {
		t.Errorf("空管道 Steps 应长度 0，实际 %d", len(steps))
	}
	// 不应出现 "null" 作为 Steps 的值。
	if strings.Contains(s, `"Steps": null`) {
		t.Errorf("空管道 Steps 不应为 null:\n%s", s)
	}
}

// TestToJSONOmitempty retry=0 且无 depends_on/config 时不应出现在输出里。
func TestToJSONOmitempty(t *testing.T) {
	p := Pipeline{Name: "min", Steps: []Step{
		{ID: "src", Kind: KindSource, Connector: "csv"},
	}}
	s, err := p.ToJSON()
	if err != nil {
		t.Fatalf("ToJSON 出错: %v", err)
	}
	// retry/depends_on/dead_letter 都是 omitempty，不应出现。
	for _, key := range []string{`"retry"`, `"depends_on"`, `"dead_letter"`, `"config"`} {
		if strings.Contains(s, key) {
			t.Errorf("零值字段 %s 不应出现（omitempty）:\n%s", key, s)
		}
	}
}
