package transform

import (
	"fmt"

	"github.com/QiuShichang/flow-pipe/internal/pipeline"
)

// MapTransform 对字段值做映射转换（查表替换 + 类型转换 + 表达式求值的轻量子集）。
//
// 三种模式（按 config 选一种）：
//   - lookup：按 map 把字段值替换为新值（如 status: {"0":"pending","1":"done"}）
//   - cast：  转字段类型（如 amount: "float"，把字符串数字转 float64）
//   - template：用其他字段拼出新字段值（如 full_name: "{first} {last}"）
type MapTransform struct{}

// Type 返回连接器类型标识。
func (MapTransform) Type() string { return "map" }

// Transform 按配置转换字段。config:
//
//	lookup   map[string]map[string]any  字段值查表替换（字段名→{旧值:新值}）
//	cast     map[string]string          字段类型转换（字段名→"float"/"int"/"string"）
//	template map[string]string          模板拼新字段（新字段名→"{a} {b}" 模板）
//
// 示例: {lookup: {status: {0: "pending"}}, cast: {amount: "float"}, template: {name: "{first} {last}"}}
func (MapTransform) Transform(rows pipeline.Rows, config map[string]any) (pipeline.Rows, error) {
	lookup, _ := config["lookup"].(map[string]any) // map[field]map[old]new
	castCfg, _ := config["cast"].(map[string]any)  // map[field]string
	template, _ := config["template"].(map[string]any)

	out := make(pipeline.Rows, 0, len(rows))
	for _, r := range rows {
		row := pipeline.Row{}
		for k, v := range r {
			row[k] = v
		}

		// 1. lookup：字段值查表替换
		for field, tableAny := range lookup {
			table, ok := tableAny.(map[string]any)
			if !ok {
				continue
			}
			cur, exists := row[field]
			if !exists {
				continue
			}
			key := fmt.Sprintf("%v", cur)
			if newVal, ok := table[key]; ok {
				row[field] = newVal
			}
		}

		// 2. cast：类型转换
		for field, typeAny := range castCfg {
			typeName, ok := typeAny.(string)
			if !ok {
				continue
			}
			val, exists := row[field]
			if !exists {
				continue
			}
			converted, err := castValue(val, typeName)
			if err != nil {
				return nil, fmt.Errorf("cast 字段 %s 为 %s 失败: %w", field, typeName, err)
			}
			row[field] = converted
		}

		// 3. template：模板拼新字段
		for newField, tmplAny := range template {
			tmpl, ok := tmplAny.(string)
			if !ok {
				continue
			}
			row[newField] = expandTemplate(tmpl, row)
		}

		out = append(out, row)
	}
	return out, nil
}

// castValue 把 val 转成指定类型。
func castValue(val any, typeName string) (any, error) {
	switch typeName {
	case "string":
		return fmt.Sprintf("%v", val), nil
	case "int":
		return toInt64(val)
	case "float":
		return toFloat64(val)
	default:
		return nil, fmt.Errorf("未知类型 %q（支持 string/int/float）", typeName)
	}
}

// toFloat64 把 any 转 float64。
func toFloat64(v any) (float64, error) {
	switch x := v.(type) {
	case float64:
		return x, nil
	case float32:
		return float64(x), nil
	case int:
		return float64(x), nil
	case int64:
		return float64(x), nil
	case string:
		var f float64
		_, err := fmt.Sscanf(x, "%f", &f)
		if err != nil {
			return 0, fmt.Errorf("无法解析 %q 为 float", x)
		}
		return f, nil
	}
	return 0, fmt.Errorf("无法转换 %T 为 float", v)
}

// toInt64 把 any 转 int64。
func toInt64(v any) (int64, error) {
	switch x := v.(type) {
	case int:
		return int64(x), nil
	case int64:
		return x, nil
	case float64:
		return int64(x), nil
	case string:
		var i int64
		_, err := fmt.Sscanf(x, "%d", &i)
		if err != nil {
			return 0, fmt.Errorf("无法解析 %q 为 int", x)
		}
		return i, nil
	}
	return 0, fmt.Errorf("无法转换 %T 为 int", v)
}

// expandTemplate 把 "{first} {last}" 这种模板用 row 的字段值填充。
func expandTemplate(tmpl string, row pipeline.Row) string {
	out := ""
	i := 0
	for i < len(tmpl) {
		if i+1 < len(tmpl) && tmpl[i] == '{' {
			end := indexByte(tmpl[i+1:], '}')
			if end >= 0 {
				key := tmpl[i+1 : i+1+end]
				if v, ok := row[key]; ok {
					out += fmt.Sprintf("%v", v)
				}
				i = i + 1 + end + 1
				continue
			}
		}
		out += string(tmpl[i])
		i++
	}
	return out
}

func indexByte(s string, b byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == b {
			return i
		}
	}
	return -1
}

func init() {
	pipeline.RegisterTransform(&MapTransform{})
}
