package source

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/QiuShichang/flow-pipe/internal/pipeline"
)

// JSONSource 从 JSON 数组文件读取数据。
// 文件格式：[{"a":1,"b":2}, ...]，解析成 []map[string]any → Rows。
type JSONSource struct{}

// Type 返回连接器类型标识。
func (JSONSource) Type() string { return "json" }

// Read 根据 config 读 JSON。config:
//
//	path  string  JSON 文件路径（必填）
//
// 示例: {path: "data.json"}
func (JSONSource) Read(config map[string]any) (pipeline.Rows, error) {
	path, ok := config["path"].(string)
	if !ok || path == "" {
		return nil, fmt.Errorf("json source 缺少 path 配置")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取 JSON 文件失败: %w", err)
	}

	// 先按数组解析（约定格式）；若失败再尝试单对象包成一行。
	var arr []map[string]any
	if err := json.Unmarshal(data, &arr); err == nil {
		rows := make(pipeline.Rows, 0, len(arr))
		for _, m := range arr {
			row := pipeline.Row{}
			for k, v := range m {
				row[k] = v
			}
			rows = append(rows, row)
		}
		return rows, nil
	}

	var single map[string]any
	if err := json.Unmarshal(data, &single); err == nil {
		row := pipeline.Row{}
		for k, v := range single {
			row[k] = v
		}
		return pipeline.Rows{row}, nil
	}
	return nil, fmt.Errorf("JSON 文件 %q 既不是对象数组也不是单对象", path)
}

func init() {
	pipeline.RegisterSource(&JSONSource{})
}
