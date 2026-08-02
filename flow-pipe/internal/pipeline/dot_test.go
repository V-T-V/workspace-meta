package pipeline

import (
	"strings"
	"testing"
)

// TestToDOTSimple 3 步线性管道：验证 DOT 含 digraph 头 + 3 节点 + 2 边。
func TestToDOTSimple(t *testing.T) {
	p := Pipeline{Name: "csv-to-sqlite", Steps: []Step{
		{ID: "read", Kind: KindSource, Connector: "csv"},
		{ID: "filter", Kind: KindTransform, Connector: "filter", DependsOn: []string{"read"}},
		{ID: "write", Kind: KindSink, Connector: "sqlite", DependsOn: []string{"filter"}},
	}}
	dot := p.ToDOT()

	// 头部：digraph 声明 + 基础属性。
	if !strings.HasPrefix(dot, `digraph "csv-to-sqlite" {`) {
		t.Errorf("缺少 digraph 头，实际开头: %q", truncHead(dot, 40))
	}
	if !strings.Contains(dot, "rankdir=LR;") {
		t.Error("缺少 rankdir=LR")
	}

	// 3 个节点：每个步骤 ID 应作为节点出现（引号包裹）。
	for _, id := range []string{`"read"`, `"filter"`, `"write"`} {
		if !strings.Contains(dot, "  "+id+" [label=") {
			t.Errorf("缺少节点 %s", id)
		}
	}

	// 2 条边（引号包裹的 ID）。
	if !strings.Contains(dot, `"read" -> "filter";`) {
		t.Error("缺少边 read -> filter")
	}
	if !strings.Contains(dot, `"filter" -> "write";`) {
		t.Error("缺少边 filter -> write")
	}

	// 尾部闭合。
	if !strings.HasSuffix(dot, "}\n") {
		t.Errorf("应以 }\\n 结尾，实际结尾: %q", truncTail(dot, 10))
	}

	// 边数精确（避免误判分支/多余边）：统计 " -> " 出现次数。
	if got := strings.Count(dot, " -> "); got != 2 {
		t.Errorf("应有 2 条边，实际 %d 条", got)
	}
}

// TestToDOTColor 验证 source/transform/sink 三种 kind 着色不同。
func TestToDOTColor(t *testing.T) {
	p := Pipeline{Name: "colors", Steps: []Step{
		{ID: "src", Kind: KindSource, Connector: "csv"},
		{ID: "tr", Kind: KindTransform, Connector: "filter", DependsOn: []string{"src"}},
		{ID: "sn", Kind: KindSink, Connector: "stdout", DependsOn: []string{"tr"}},
	}}
	dot := p.ToDOT()

	// source=lightgreen、transform=lightyellow、sink=lightblue。
	if !strings.Contains(dot, "fillcolor=lightgreen") {
		t.Error("source 应着色 lightgreen")
	}
	if !strings.Contains(dot, "fillcolor=lightyellow") {
		t.Error("transform 应着色 lightyellow")
	}
	if !strings.Contains(dot, "fillcolor=lightblue") {
		t.Error("sink 应着色 lightblue")
	}

	// 三种颜色各应只出现一次（即每种 kind 恰好一个节点）。
	for _, c := range []string{"lightgreen", "lightyellow", "lightblue"} {
		if got := strings.Count(dot, c); got != 1 {
			t.Errorf("颜色 %s 应出现 1 次，实际 %d 次", c, got)
		}
	}
}

// TestToDOTEmpty 空管道不应 panic，且仍生成合法 DOT 框架。
func TestToDOTEmpty(t *testing.T) {
	p := Pipeline{Name: "empty", Steps: nil}
	dot := p.ToDOT() // 不 panic 即通过

	if !strings.HasPrefix(dot, `digraph "empty" {`) {
		t.Errorf("空管道应仍有 digraph 头，实际开头: %q", truncHead(dot, 30))
	}
	if !strings.HasSuffix(dot, "}\n") {
		t.Error("空管道应正常闭合 }")
	}
	// 空管道不应有任何节点声明或边。
	if strings.Contains(dot, " -> ") {
		t.Error("空管道不应有边")
	}
	if strings.Contains(dot, "[label=") {
		t.Error("空管道不应有节点")
	}
}

// TestToDOTBranch 分支管道：一个 source → 两个 transform，应有 2 条从 source 出发的边。
func TestToDOTBranch(t *testing.T) {
	p := Pipeline{Name: "branch", Steps: []Step{
		{ID: "read", Kind: KindSource, Connector: "csv"},
		{ID: "t1", Kind: KindTransform, Connector: "filter", DependsOn: []string{"read"}},
		{ID: "t2", Kind: KindTransform, Connector: "field", DependsOn: []string{"read"}},
	}}
	dot := p.ToDOT()

	// read 应同时指向 t1 和 t2：两条边（节点 ID 用引号包裹）。
	if !strings.Contains(dot, `"read" -> "t1";`) {
		t.Error("缺少边 read -> t1")
	}
	if !strings.Contains(dot, `"read" -> "t2";`) {
		t.Error("缺少边 read -> t2")
	}
	// 总边数应为 2（两个 transform 都只依赖 read）。
	if got := strings.Count(dot, " -> "); got != 2 {
		t.Errorf("分支管道应有 2 条边，实际 %d 条", got)
	}
	// 3 个节点都应在（引号包裹的 ID）。
	for _, id := range []string{`"read"`, `"t1"`, `"t2"`} {
		if !strings.Contains(dot, "  "+id+" [label=") {
			t.Errorf("缺少节点 %s", id)
		}
	}
}

// truncHead/truncTail 安全截取字符串前/后 n 个字符（n 超长时返回全部）。
func truncHead(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
func truncTail(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[len(s)-n:]
}
