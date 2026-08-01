package pipeline

import (
	"testing"
)

// TestTopoSortLinear 线性链：read → filter → write。
func TestTopoSortLinear(t *testing.T) {
	p := Pipeline{Name: "x", Steps: []Step{
		{ID: "read", Kind: KindSource, Connector: "csv"},
		{ID: "filter", Kind: KindTransform, Connector: "filter", DependsOn: []string{"read"}},
		{ID: "write", Kind: KindSink, Connector: "stdout", DependsOn: []string{"filter"}},
	}}
	order, err := p.TopoSort()
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"read", "filter", "write"}
	for i, id := range order {
		if id != want[i] {
			t.Errorf("位置 %d: 想要 %s 实际 %s", i, want[i], id)
		}
	}
}

// TestTopoSortBranch 分支：read → [t1, t2] → merge。
func TestTopoSortBranch(t *testing.T) {
	p := Pipeline{Name: "x", Steps: []Step{
		{ID: "read", Kind: KindSource, Connector: "csv"},
		{ID: "t1", Kind: KindTransform, Connector: "filter", DependsOn: []string{"read"}},
		{ID: "t2", Kind: KindTransform, Connector: "field", DependsOn: []string{"read"}},
		{ID: "merge", Kind: KindSink, Connector: "stdout", DependsOn: []string{"t1", "t2"}},
	}}
	order, err := p.TopoSort()
	if err != nil {
		t.Fatal(err)
	}
	// read 必须第一，merge 必须最后，t1/t2 中间（字典序 t1<t2）
	if order[0] != "read" || order[3] != "merge" {
		t.Errorf("顺序错: %v", order)
	}
	if order[1] != "t1" || order[2] != "t2" {
		t.Errorf("中间顺序应字典序 t1,t2: %v", order)
	}
}

// TestTopoSortCycle 循环依赖应报错。
func TestTopoSortCycle(t *testing.T) {
	p := Pipeline{Name: "x", Steps: []Step{
		{ID: "a", Kind: KindSource, Connector: "csv", DependsOn: []string{"c"}},
		{ID: "b", Kind: KindTransform, Connector: "f", DependsOn: []string{"a"}},
		{ID: "c", Kind: KindSink, Connector: "s", DependsOn: []string{"b"}},
	}}
	_, err := p.TopoSort()
	if err == nil {
		t.Error("循环依赖应报错")
	}
}

// TestTopoSortMissingDep 依赖不存在的步骤应报错。
func TestTopoSortMissingDep(t *testing.T) {
	p := Pipeline{Name: "x", Steps: []Step{
		{ID: "a", Kind: KindSource, Connector: "csv"},
		{ID: "b", Kind: KindSink, Connector: "s", DependsOn: []string{"ghost"}},
	}}
	_, err := p.TopoSort()
	if err == nil {
		t.Error("依赖不存在应报错")
	}
}

// TestValidateValid 合法管道应通过。
func TestValidateValid(t *testing.T) {
	p := Pipeline{Name: "x", Steps: []Step{
		{ID: "read", Kind: KindSource, Connector: "csv"},
		{ID: "write", Kind: KindSink, Connector: "stdout", DependsOn: []string{"read"}},
	}}
	if err := p.Validate(); err != nil {
		t.Errorf("合法管道应通过: %v", err)
	}
}

// TestValidateNoSource 缺 source 应报错。
func TestValidateNoSource(t *testing.T) {
	p := Pipeline{Name: "x", Steps: []Step{
		{ID: "t", Kind: KindTransform, Connector: "f", DependsOn: []string{}},
	}}
	// 加个 source 凑成有 source 但 transform 无依赖
	if err := p.Validate(); err == nil {
		t.Error("transform 无依赖应报错")
	}
}

// TestValidateSourceWithDep source 有依赖应报错。
func TestValidateSourceWithDep(t *testing.T) {
	p := Pipeline{Name: "x", Steps: []Step{
		{ID: "a", Kind: KindSource, Connector: "csv", DependsOn: []string{"b"}},
		{ID: "b", Kind: KindSource, Connector: "csv"},
	}}
	if err := p.Validate(); err == nil {
		t.Error("source 有依赖应报错")
	}
}

// TestRunWithGenerateToStdout 端到端：generate → stdout（用真实连接器）。
// 注意：需要 source.generate 和 sink.stdout 已注册。本测试在 pipeline 包内，
// 不能 import source/sink（会循环），所以改用 mock 连接器注册。
func TestRunWithMockConnectors(t *testing.T) {
	// 注册临时 mock 连接器（测试用，覆盖任何同名）
	RegisterSource(&mockSource{})
	RegisterSink(&mockSink{})

	p := Pipeline{Name: "test", Steps: []Step{
		{ID: "read", Kind: KindSource, Connector: "mocksource"},
		{ID: "write", Kind: KindSink, Connector: "mocksink", DependsOn: []string{"read"}},
	}}
	result := Run(p)
	if result.Err != nil {
		t.Fatalf("Run 失败: %v", result.Err)
	}
	if len(result.Steps) != 2 {
		t.Errorf("应有 2 步结果，实际 %d", len(result.Steps))
	}
	// source 应输出 3 行
	if result.Steps[0].RowsOut != 3 {
		t.Errorf("source 应输出 3 行，实际 %d", result.Steps[0].RowsOut)
	}
}

// mockSource 造 3 行测试数据。
type mockSource struct{}

func (mockSource) Type() string { return "mocksource" }
func (mockSource) Read(config map[string]any) (Rows, error) {
	return Rows{{"a": 1}, {"a": 2}, {"a": 3}}, nil
}

// mockSink 记录写入的行数。
type mockSink struct{}

func (mockSink) Type() string { return "mocksink" }
func (mockSink) Write(rows Rows, config map[string]any) error {
	_ = rows
	return nil
}

// TestParseYAML 验证 YAML 解析。
func TestParseYAML(t *testing.T) {
	yaml := []byte(`
name: test-pipe
steps:
  - id: read
    type: source
    connector: csv
    config:
      path: "in.csv"
  - id: write
    type: sink
    connector: stdout
    depends_on: [read]
`)
	p, err := Parse(yaml)
	if err != nil {
		t.Fatal(err)
	}
	if p.Name != "test-pipe" {
		t.Errorf("name 应 test-pipe，实际 %s", p.Name)
	}
	if len(p.Steps) != 2 {
		t.Fatalf("应有 2 步，实际 %d", len(p.Steps))
	}
	if p.Steps[0].Connector != "csv" {
		t.Errorf("step0 connector 应 csv，实际 %s", p.Steps[0].Connector)
	}
	if p.Steps[1].DependsOn[0] != "read" {
		t.Errorf("step1 依赖 read，实际 %v", p.Steps[1].DependsOn)
	}
}

// TestParseMissingName 缺 name 应报错。
func TestParseMissingName(t *testing.T) {
	_, err := Parse([]byte("steps: []"))
	if err == nil {
		t.Error("缺 name 应报错")
	}
}

// TestPipelineString 验证 String 输出。
func TestPipelineString(t *testing.T) {
	p := Pipeline{Name: "x", Steps: []Step{
		{ID: "read", Kind: KindSource, Connector: "csv"},
	}}
	s := p.String()
	if s == "" {
		t.Error("String 不应为空")
	}
}
