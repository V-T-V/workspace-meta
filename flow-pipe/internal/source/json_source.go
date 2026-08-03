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
//	root  string  响应 JSON 里数组的路径（如 "data.items"，默认空=整体是数组）
//
// 当 root 为空时：文件必须是 JSON 数组 [{...}] 或单个对象 {...}（自动包成一行）。
// 当 root 非空时：文件必须是 JSON 对象，按点分路径取出其中的数组（嵌套场景），
// 例如 {"data":{"items":[{...}]}} 配 root="data.items" 即取 data.items 数组。
//
// 示例:
//
//	{path: "data.json"}
//	{path: "data.json", root: "data.items"}
func (JSONSource) Read(config map[string]any) (pipeline.Rows, error) {
	path, ok := config["path"].(string)
	if !ok || path == "" {
		return nil, fmt.Errorf("json source 缺少 path 配置")
	}
	root, _ := config["root"].(string)

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取 JSON 文件失败: %w", err)
	}

	// root 非空：按点分路径从嵌套对象里取数组（与 http source 的 root 语义一致）。
	if root != "" {
		var obj map[string]any
		if err := json.Unmarshal(data, &obj); err != nil {
			return nil, fmt.Errorf("解析 JSON 失败（root 模式期望对象）: %w", err)
		}
		v, ok := extractPath(obj, root)
		if !ok {
			return nil, fmt.Errorf("root 路径 %q 不存在", root)
		}
		list, ok := v.([]any)
		if !ok {
			return nil, fmt.Errorf("root 路径 %q 不是数组", root)
		}
		arr := toRows(list)
		rows := make(pipeline.Rows, 0, len(arr))
		for _, m := range arr {
			rows = append(rows, pipeline.Row(m))
		}
		return rows, nil
	}

	// root 为空：先按数组解析（约定格式）；若失败再尝试单对象包成一行。
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
