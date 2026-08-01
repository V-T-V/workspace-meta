package pipeline

import (
	"fmt"
	"strings"
)

// dotColor 按 StepKind 返回 Graphviz 填充色。
// source=绿、transform=黄、sink=蓝；未知 kind 用白（兜底，避免空色）。
func dotColor(k StepKind) string {
	switch k {
	case KindSource:
		return "lightgreen"
	case KindTransform:
		return "lightyellow"
	case KindSink:
		return "lightblue"
	default:
		return "white"
	}
}

// dotQuote 把字符串包成 DOT 安全的双引号字面量。
// 仅转义双引号；反斜杠不转义，以便 label 里的 \n 被 Graphviz 当作换行处理
// （DOT 字符串里 \n = 换行）。
func dotQuote(s string) string {
	s = strings.ReplaceAll(s, "\"", "\\\"")
	return `"` + s + `"`
}

// ToDOT 把管道导出为 Graphviz DOT 格式。
// 步骤按 Kind 着色：source=绿、transform=黄、sink=蓝。
// 边表示 depends_on 关系（依赖在前，被依赖者指向依赖者）。
//
// 输出示例：
//
//	digraph "csv-to-sqlite" {
//	  rankdir=LR;
//	  node [shape=box, style=filled];
//
//	  read [label="read\n(csv)", fillcolor=lightgreen];
//	  filter [label="filter\n(filter)", fillcolor=lightyellow];
//	  write [label="write\n(sqlite)", fillcolor=lightblue];
//
//	  read -> filter;
//	  filter -> write;
//	}
func (p Pipeline) ToDOT() string {
	var b strings.Builder
	name := p.Name
	if name == "" {
		name = "pipeline"
	}
	fmt.Fprintf(&b, "digraph %s {\n", dotQuote(name))
	b.WriteString("  rankdir=LR;\n")
	b.WriteString("  node [shape=box, style=filled];\n")

	// 节点：每个步骤一行。节点 ID 用引号包裹（防止含 - 等特殊字符破坏 Graphviz 解析）。
	for _, s := range p.Steps {
		// label = "<id>\n(<connector>)"
		label := s.ID + `\n(` + s.Connector + `)`
		fmt.Fprintf(&b, "  %s [label=%s, fillcolor=%s];\n",
			dotQuote(s.ID), dotQuote(label), dotColor(s.Kind))
	}

	// 边：每个 depends_on 产生一条边 dep -> step。
	for _, s := range p.Steps {
		for _, dep := range s.DependsOn {
			fmt.Fprintf(&b, "  %s -> %s;\n", dotQuote(dep), dotQuote(s.ID))
		}
	}

	b.WriteString("}\n")
	return b.String()
}
