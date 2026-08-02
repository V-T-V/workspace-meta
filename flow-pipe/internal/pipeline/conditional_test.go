package pipeline

import (
	"strings"
	"testing"
)

// 本文件测试条件步骤（Step.When）。
//
// 核心能力：步骤声明 when: "{{stepID.rows_out}} > 0" 后，
// runner 在执行前对其求值——仅当上游步骤输出非空时才执行；否则 Skipped=true 跳过。
//
// 求值规则（evalWhen）：
//   - 仅支持 "{{stepID.rows_out}} OP number"，OP ∈ {>, >=, <, <=, ==, !=}
//   - 上游 RowsOut 取自已完成步骤的 StepResult；上游未执行/不存在视为 0
//   - 非法表达式导致步骤失败（暴露配置问题），不静默跳过

// conditionalPipeline 构造 read(source, 3 行) → maybe(transform) → write(sink)，
// maybe 是否执行由 when 控制。
func conditionalPipeline(when string) Pipeline {
	return Pipeline{Name: "cond-test", Steps: []Step{
		{ID: "read", Kind: KindSource, Connector: "mocksource"},
		{ID: "maybe", Kind: KindTransform, Connector: "passthrough", DependsOn: []string{"read"}, When: when},
		{ID: "write", Kind: KindSink, Connector: "countingsink", DependsOn: []string{"maybe"}},
	}}
}

// TestConditionalStepExecutes 条件满足（上游 read 输出 3 行 > 0）→ maybe 正常执行。
func TestConditionalStepExecutes(t *testing.T) {
	resetCounters()
	RegisterSource(mockSource{}) // pipeline_test.go 里定义
	RegisterSink(mockSink{})
	p := conditionalPipeline("{{read.rows_out}} > 0")

	result := Run(p)
	if result.Err != nil {
		t.Fatalf("Run 不应失败: %v", result.Err)
	}
	maybe := findStep(t, result, "maybe")
	if maybe.Skipped {
		t.Error("条件满足时 maybe 不应被跳过")
	}
	if maybe.RowsOut != 3 {
		t.Errorf("maybe 应透传 3 行，实际 %d", maybe.RowsOut)
	}
	// transform 确实被调用
	if passThrough.calls != 1 {
		t.Errorf("maybe 的 transform 应被调用 1 次，实际 %d", passThrough.calls)
	}
}

// TestConditionalStepSkipped 条件不满足（要求上游 > 5，实际只有 3）→ maybe Skipped=true 且不执行。
func TestConditionalStepSkipped(t *testing.T) {
	resetCounters()
	RegisterSource(mockSource{})
	RegisterSink(mockSink{})
	p := conditionalPipeline("{{read.rows_out}} > 5")

	result := Run(p)
	if result.Err != nil {
		t.Fatalf("Run 不应失败（条件不满足只是跳过）: %v", result.Err)
	}
	maybe := findStep(t, result, "maybe")
	if !maybe.Skipped {
		t.Error("条件不满足时 maybe 应 Skipped=true")
	}
	// transform 未被调用
	if passThrough.calls != 0 {
		t.Errorf("被跳过时 transform 不应执行，实际调用 %d 次", passThrough.calls)
	}
	// write 仍执行（拿到空输入，因 maybe 被跳过）
	write := findStep(t, result, "write")
	if write.Skipped {
		t.Error("write 无 When 条件，不应被跳过")
	}
}

// TestConditionalStepEqualOp 验证 == / != / >= / <= 等操作符。
func TestConditionalStepEqualOp(t *testing.T) {
	cases := []struct {
		when string
		exec bool
	}{
		{"{{read.rows_out}} == 3", true}, // 上游正好 3 行
		{"{{read.rows_out}} != 0", true},
		{"{{read.rows_out}} == 0", false},
		{"{{read.rows_out}} >= 3", true},
		{"{{read.rows_out}} <= 2", false},
	}
	for _, c := range cases {
		resetCounters()
		RegisterSource(mockSource{})
		RegisterSink(mockSink{})
		p := conditionalPipeline(c.when)
		result := Run(p)
		if result.Err != nil {
			t.Errorf("when=%q 不应失败: %v", c.when, result.Err)
			continue
		}
		maybe := findStep(t, result, "maybe")
		if maybe.Skipped == c.exec {
			t.Errorf("when=%q 期望 executed=%v，实际 Skipped=%v", c.when, c.exec, maybe.Skipped)
		}
	}
}

// TestConditionalStepUpstreamMissing 上游步骤 ID 在占位符里但不存在/未执行 → 视为 0。
// 要求 > 0 而 read（真实存在但本管道里没引用）……这里直接用一个不存在的 stepID。
func TestConditionalStepUpstreamMissing(t *testing.T) {
	resetCounters()
	RegisterSource(mockSource{})
	RegisterSink(mockSink{})
	// "ghost" 在管道里不存在 → 视为 RowsOut=0 → 0 > 0 为 false → 跳过
	p := Pipeline{Name: "cond-missing", Steps: []Step{
		{ID: "read", Kind: KindSource, Connector: "mocksource"},
		{ID: "maybe", Kind: KindTransform, Connector: "passthrough", DependsOn: []string{"read"},
			When: "{{ghost.rows_out}} > 0"},
		{ID: "write", Kind: KindSink, Connector: "countingsink", DependsOn: []string{"maybe"}},
	}}
	result := Run(p)
	if result.Err != nil {
		t.Fatalf("Run 不应失败: %v", result.Err)
	}
	if !findStep(t, result, "maybe").Skipped {
		t.Error("上游不存在视为 RowsOut=0，条件 0>0 不满足，应跳过 maybe")
	}
}

// TestConditionalStepInvalidExpr 非法 When 表达式应导致步骤失败（暴露配置问题）。
func TestConditionalStepInvalidExpr(t *testing.T) {
	resetCounters()
	RegisterSource(mockSource{})
	RegisterSink(mockSink{})
	p := conditionalPipeline("not a valid expression")
	result := Run(p)
	if result.Err == nil {
		t.Fatal("非法 when 表达式应导致 Run 失败")
	}
	if !strings.Contains(result.Err.Error(), "when") {
		t.Errorf("错误信息应提及 when, 实际: %v", result.Err)
	}
}

// TestConditionalStepNoWhen When 为空 → 总是执行（对照组）。
func TestConditionalStepNoWhen(t *testing.T) {
	resetCounters()
	RegisterSource(mockSource{})
	RegisterSink(mockSink{})
	p := conditionalPipeline("")
	result := Run(p)
	if result.Err != nil {
		t.Fatalf("Run 不应失败: %v", result.Err)
	}
	if findStep(t, result, "maybe").Skipped {
		t.Error("When 为空时 maybe 不应被跳过")
	}
}

// TestParseConditionYAML 验证 loader 从 YAML 解析 when 字段到 Step.When。
func TestParseConditionYAML(t *testing.T) {
	yaml := []byte(`
name: cond-yaml
steps:
  - id: read
    type: source
    connector: csv
    config: { path: "in.csv" }
  - id: maybe
    type: transform
    connector: filter
    config: { where: "x > 0" }
    depends_on: [read]
    when: "{{read.rows_out}} > 0"
  - id: write
    type: sink
    connector: stdout
    depends_on: [maybe]
`)
	p, err := Parse(yaml)
	if err != nil {
		t.Fatalf("Parse 失败: %v", err)
	}
	var whenVal string
	for _, s := range p.Steps {
		if s.ID == "maybe" {
			whenVal = s.When
		}
	}
	if whenVal != "{{read.rows_out}} > 0" {
		t.Errorf("maybe.When 应被解析, 实际 %q", whenVal)
	}
}
