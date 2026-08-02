// Package source 实现 flow-pipe 的 source 连接器（数据源）。
// 每个连接器实现 pipeline.SourceConnector，通过 init() 注册到 pipeline.RegisterSource。
//
// 设计：连接器各自一个文件，新增只需实现接口 + init 注册，零改框架。
package source

import (
	"github.com/QiuShichang/flow-pipe/internal/pipeline"
)

// init 注册本包所有 source 连接器。
func init() {
	pipeline.RegisterSource(&GenerateSource{})
}

// ===== generate：造测试数据 =====

// GenerateSource 按配置生成测试数据行（无需外部文件，便于 demo）。
type GenerateSource struct{}

// Type 返回连接器类型标识。
func (GenerateSource) Type() string { return "generate" }

// Read 根据 config 生成行。config:
//
//	count   int       生成行数（默认 5）
//	fields  map       字段定义（key → 固定值 或 "seq" 表示序号）
//
// 示例 config: {count: 3, fields: {id: "seq", name: "alice", amount: 100}}
func (GenerateSource) Read(config map[string]any) (pipeline.Rows, error) {
	count := 5
	if c, ok := config["count"]; ok {
		if n, ok := toInt(c); ok {
			count = n
		}
	}
	fields, _ := config["fields"].(map[string]any)

	rows := make(pipeline.Rows, 0, count)
	for i := 0; i < count; i++ {
		row := pipeline.Row{}
		for k, v := range fields {
			if s, ok := v.(string); ok && s == "seq" {
				row[k] = i + 1
				continue
			}
			row[k] = v
		}
		rows = append(rows, row)
	}
	return rows, nil
}

// toInt 把 any 转 int（YAML 解析数字可能是 int 或 int64）。
func toInt(v any) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case int64:
		return int(n), true
	case float64:
		return int(n), true
	}
	return 0, false
}
