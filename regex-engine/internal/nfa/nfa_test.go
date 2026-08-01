package nfa

import (
	"testing"

	"github.com/QiuShichang/regex-engine/internal/ast"
)

// buildAndRun 用 matcher 逻辑验证 NFA（不引 matcher 包，直接模拟）。
// 这里只测 NFA 构造的正确性（状态数/边数合理），匹配由 matcher_test 覆盖。

func TestBuildLiteral(t *testing.T) {
	n := Build(ast.NewLiteral('a'))
	if n.Start == n.Accept {
		t.Error("Start 和 Accept 应不同")
	}
	// 字面量应有 1 条字符边
	edges := n.Transitions(n.Start)
	if len(edges) != 1 || edges[0].Epsilon {
		t.Error("字面量应有 1 条非 ε 边")
	}
	if !edges[0].Match('a') {
		t.Error("字面量边应匹配 'a'")
	}
	if edges[0].Match('b') {
		t.Error("字面量边不应匹配 'b'")
	}
}

func TestBuildWildcard(t *testing.T) {
	n := Build(ast.NewWildcard())
	edges := n.Transitions(n.Start)
	if len(edges) != 1 {
		t.Fatalf("应有 1 条边，实际 %d", len(edges))
	}
	if !edges[0].Match('x') {
		t.Error("wildcard 应匹配 'x'")
	}
	if edges[0].Match('\n') {
		t.Error("wildcard 不应匹配换行")
	}
}

func TestBuildStar(t *testing.T) {
	n := Build(ast.NewStar(ast.NewLiteral('a')))
	// Star 的 Start 应有 ε 边到子 NFA start 和 Accept（0 次跳转）
	edges := n.Transitions(n.Start)
	hasEpsilonToAccept := false
	hasEpsilonToChild := false
	for _, e := range edges {
		if e.Epsilon {
			if e.To == n.Accept {
				hasEpsilonToAccept = true
			} else {
				hasEpsilonToChild = true
			}
		}
	}
	if !hasEpsilonToAccept {
		t.Error("Star 应有 ε 边直通 Accept（0 次匹配）")
	}
	if !hasEpsilonToChild {
		t.Error("Star 应有 ε 边到子 NFA（≥1 次匹配）")
	}
}

func TestBuildAlternate(t *testing.T) {
	n := Build(ast.NewAlternate(ast.NewLiteral('a'), ast.NewLiteral('b')))
	// Alternate 的 Start 应有 2 条 ε 边（分别到 a 和 b）
	edges := n.Transitions(n.Start)
	epsilonCount := 0
	for _, e := range edges {
		if e.Epsilon {
			epsilonCount++
		}
	}
	if epsilonCount != 2 {
		t.Errorf("Alternate Start 应有 2 条 ε 边，实际 %d", epsilonCount)
	}
}

func TestBuildConcat(t *testing.T) {
	n := Build(ast.NewConcat(ast.NewLiteral('a'), ast.NewLiteral('b')))
	// Concat 的 Start 应是 a 的 start，a 的 accept 应有 ε 边到 b 的 start
	edges := n.Transitions(n.Start)
	if len(edges) != 1 || edges[0].Epsilon {
		t.Error("Concat Start 应有 1 条字符边（到 a）")
	}
}

func TestBuildPlus(t *testing.T) {
	n := Build(ast.NewPlus(ast.NewLiteral('a')))
	// Plus 的 Start 应有 ε 边到子 NFA（不像 Star 那样有直通 Accept）
	edges := n.Transitions(n.Start)
	hasDirectToAccept := false
	for _, e := range edges {
		if e.Epsilon && e.To == n.Accept {
			hasDirectToAccept = true
		}
	}
	if hasDirectToAccept {
		t.Error("Plus 不应有直通 Accept 的 ε 边（必须至少 1 次）")
	}
}

func TestBuildGroup(t *testing.T) {
	// Group 在内部 NFA 外包一层带捕获标记的 ε 边：
	// start --ε(CaptureStart=0)--> inner.start ... inner.accept --ε(CaptureEnd=0)--> accept
	n := Build(ast.NewGroup(ast.NewLiteral('x')))
	if n.GroupCount() != 1 {
		t.Errorf("单个 Group 的 GroupCount 应为 1，实际 %d", n.GroupCount())
	}
	// Start 应有一条 CaptureStart=0 的 ε 边
	edges := n.Transitions(n.Start)
	if len(edges) != 1 || !edges[0].Epsilon {
		t.Fatalf("Group Start 应有 1 条 ε 边，实际 %+v", edges)
	}
	if edges[0].CaptureStart != 0 || edges[0].CaptureEnd != -1 {
		t.Errorf("进入边应为 CaptureStart=0, CaptureEnd=-1，实际 start=%d end=%d",
			edges[0].CaptureStart, edges[0].CaptureEnd)
	}
}

func TestBuildNestedGroupNumbering(t *testing.T) {
	// ((ab)) 按左括号出现顺序编号：外层=0，内层=1（GroupCount 不含整体组 0）
	outer := ast.NewGroup(ast.NewGroup(ast.NewLiteral('a')))
	n := Build(outer)
	if n.GroupCount() != 2 {
		t.Errorf("((a)) 的 GroupCount 应为 2，实际 %d", n.GroupCount())
	}
}

func TestGroupCountNoGroup(t *testing.T) {
	// 无分组的正则 GroupCount=0
	n := Build(ast.NewLiteral('a'))
	if n.GroupCount() != 0 {
		t.Errorf("无分组 GroupCount 应为 0，实际 %d", n.GroupCount())
	}
}

func TestBuildCharClass(t *testing.T) {
	n := Build(ast.NewCharClass([]rune("abc"), false))
	edges := n.Transitions(n.Start)
	if !edges[0].Match('a') || !edges[0].Match('b') || !edges[0].Match('c') {
		t.Error("[abc] 应匹配 a/b/c")
	}
	if edges[0].Match('d') {
		t.Error("[abc] 不应匹配 d")
	}
}

func TestBuildNegatedCharClass(t *testing.T) {
	n := Build(ast.NewCharClass([]rune("abc"), true))
	edges := n.Transitions(n.Start)
	if edges[0].Match('a') {
		t.Error("[^abc] 不应匹配 a")
	}
	if !edges[0].Match('d') {
		t.Error("[^abc] 应匹配 d")
	}
}

func TestTransitionsEmpty(t *testing.T) {
	n := Build(ast.NewLiteral('a'))
	// 不存在的状态应返回 nil
	if n.Transitions(999) != nil {
		t.Error("不存在的状态应返回 nil")
	}
}

func TestEpsilonClosure(t *testing.T) {
	// 验证 ε 闭包逻辑（通过 matcher 间接，这里只确认 NFA 结构合理）
	n := Build(ast.NewStar(ast.NewLiteral('a')))
	// Star 有多条 ε 边，Start 的 ε 闭包应包含多个状态
	edges := n.Transitions(n.Start)
	if len(edges) < 2 {
		t.Error("Star Start 应有至少 2 条边（ε 到子 + ε 到 Accept）")
	}
}
