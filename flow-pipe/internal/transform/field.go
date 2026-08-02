package transform

import (
	"github.com/QiuShichang/flow-pipe/internal/pipeline"
)

// FieldTransform 对行做字段级增/改/删。
// 支持三种操作（可同时配置，执行顺序：先 add/rename 再 drop）。
type FieldTransform struct{}

// Type 返回连接器类型标识。
func (FieldTransform) Type() string { return "field" }

// Transform 按配置改字段。config:
//
//	add    map[string]any  新增字段（key→值），已存在的会被覆盖
//	rename map[string]any  字段改名（旧名→新名），旧名不存在则跳过
//	drop   []any           要删除的字段名列表
//
// 示例: {add: {type: "user"}, rename: {id: "user_id"}, drop: ["tmp"]}
func (FieldTransform) Transform(rows pipeline.Rows, config map[string]any) (pipeline.Rows, error) {
	addMap, _ := config["add"].(map[string]any)
	renameMap, _ := config["rename"].(map[string]any)
	dropList, _ := config["drop"].([]any)

	dropSet := map[string]bool{}
	for _, d := range dropList {
		if s, ok := d.(string); ok {
			dropSet[s] = true
		}
	}

	out := make(pipeline.Rows, 0, len(rows))
	for _, row := range rows {
		nr := pipeline.Row{}

		// 1) 复制现有字段，同时处理 rename（旧名→新名）
		newNames := map[string]string{} // old → new
		for old, v := range renameMap {
			if nw, ok := v.(string); ok {
				newNames[old] = nw
			}
		}
		for k, v := range row {
			if dropSet[k] {
				continue
			}
			if newName, ok := newNames[k]; ok {
				nr[newName] = v
				continue
			}
			nr[k] = v
		}

		// 2) add 字段（覆盖同名）
		for k, v := range addMap {
			if dropSet[k] {
				continue // add 进而又 drop 的，跳过
			}
			// add 可能撞上 rename 后的新名，直接覆盖即可
			nr[k] = v
		}

		out = append(out, nr)
	}
	return out, nil
}

func init() {
	pipeline.RegisterTransform(&FieldTransform{})
}
