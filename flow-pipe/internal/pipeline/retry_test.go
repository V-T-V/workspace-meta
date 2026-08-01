package pipeline

import (
	"fmt"
	"testing"
)

// init 显式注册本测试文件用到的连接器（不依赖其他 _test.go 的注册顺序）。
func init() {
	RegisterSource(&flakySource{failBefore: 2})
	RegisterSource(&alwaysFailSource{})
	RegisterSource(&mockSource{}) // 复用 pipeline_test.go 的 mockSource
	RegisterTransform(&alwaysFailTransform{})
	RegisterSink(&mockSink{}) // 复用 pipeline_test.go 的 mockSink
}

// TestRetrySourceSuccess 验证重试后成功（前 N 次失败，最后一次成功）。
func TestRetrySourceSuccess(t *testing.T) {
	flakyCounter = 0 // 重置计数器
	p := Pipeline{Name: "retry-test", Steps: []Step{
		{ID: "read", Kind: KindSource, Connector: "flakysource", Retry: 3},
		{ID: "out", Kind: KindSink, Connector: "mocksink", DependsOn: []string{"read"}},
	}}
	result := Run(p)
	if result.Err != nil {
		t.Fatalf("重试后应成功，实际失败: %v", result.Err)
	}
}

// TestRetrySourceExhausted 验证重试次数用尽后失败。
func TestRetrySourceExhausted(t *testing.T) {
	p := Pipeline{Name: "retry-exhausted", Steps: []Step{
		{ID: "read", Kind: KindSource, Connector: "alwaysfail", Retry: 2},
		{ID: "out", Kind: KindSink, Connector: "mocksink", DependsOn: []string{"read"}},
	}}
	result := Run(p)
	if result.Err == nil {
		t.Fatal("重试用尽应失败")
	}
}

// TestDeadLetterRescuesFailure 验证死信写入后管道不失败。
func TestDeadLetterRescuesFailure(t *testing.T) {
	p := Pipeline{Name: "deadletter-test", Steps: []Step{
		{ID: "read", Kind: KindSource, Connector: "mocksource"},
		{ID: "bad", Kind: KindTransform, Connector: "alwaysfailtransform",
			DependsOn:  []string{"read"},
			DeadLetter: &DeadLetterConfig{Connector: "mocksink", Config: map[string]any{}}},
		// bad 失败但死信兜底（无输出）；下游 sink 拿到空输入，管道仍完成。
		{ID: "out", Kind: KindSink, Connector: "mocksink", DependsOn: []string{"bad"}},
	}}
	result := Run(p)
	if result.Err != nil {
		t.Fatalf("有死信兜底时管道不应失败，实际: %v", result.Err)
	}
	// 找到 bad 步骤，应记录 DeadLettered > 0（mocksource 产出 3 行）
	var badStep *StepResult
	for i := range result.Steps {
		if result.Steps[i].StepID == "bad" {
			badStep = &result.Steps[i]
			break
		}
	}
	if badStep == nil {
		t.Fatal("未找到 bad 步骤")
	}
	if badStep.DeadLettered == 0 {
		t.Error("bad 步骤应记录死信行数 > 0")
	}
}

// ===== 测试用连接器 =====

type flakySource struct{ failBefore int }

var flakyCounter = 0

func (flakySource) Type() string { return "flakysource" }
func (f *flakySource) Read(config map[string]any) (Rows, error) {
	flakyCounter++
	if flakyCounter <= f.failBefore {
		return nil, fmt.Errorf("模拟失败第 %d 次", flakyCounter)
	}
	return Rows{{"ok": true}}, nil
}

type alwaysFailSource struct{}

func (alwaysFailSource) Type() string { return "alwaysfail" }
func (alwaysFailSource) Read(config map[string]any) (Rows, error) {
	return nil, fmt.Errorf("总是失败")
}

type alwaysFailTransform struct{}

func (alwaysFailTransform) Type() string { return "alwaysfailtransform" }
func (alwaysFailTransform) Transform(rows Rows, config map[string]any) (Rows, error) {
	return nil, fmt.Errorf("transform 总是失败")
}
