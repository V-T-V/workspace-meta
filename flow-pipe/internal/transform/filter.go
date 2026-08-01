// Package transform 实现 flow-pipe 的 transform 连接器（数据变换）。
// 每个连接器实现 pipeline.TransformConnector，通过 init() 注册到 pipeline.RegisterTransform。
//
// 设计：连接器各自一个文件，新增只需实现接口 + init 注册，零改框架。
package transform

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/QiuShichang/flow-pipe/internal/pipeline"
)

// FilterTransform 按条件过滤行。
// 支持简单 where 表达式：field op value（op ∈ >, >=, <, <=, ==, !=）。
type FilterTransform struct{}

// Type 返回连接器类型标识。
func (FilterTransform) Type() string { return "filter" }

// Transform 按 where 条件过滤。config:
//
//	where  string  条件表达式（如 "age > 18" 或 `name == \"alice\"`）
//
// 数值比较优先：两边都能转 float64 按数字比，否则字符串比。
func (FilterTransform) Transform(rows pipeline.Rows, config map[string]any) (pipeline.Rows, error) {
	where, _ := config["where"].(string)
	if where == "" {
		return rows, nil // 无条件 = 不过滤（透传）
	}
	expr, err := parseWhere(where)
	if err != nil {
		return nil, err
	}

	out := make(pipeline.Rows, 0, len(rows))
	for _, row := range rows {
		ok, err := expr.match(row)
		if err != nil {
			return nil, err
		}
		if ok {
			out = append(out, row)
		}
	}
	return out, nil
}

// whereExpr 解析后的 where 表达式。
type whereExpr struct {
	field string
	op    string
	value string // 原始值（去掉引号）
	raw   bool   // true 表示字符串字面量（带引号）
}

// operators 按长度降序排列，保证先匹配 ">=" 再匹配 ">"。
var operators = []string{">=", "<=", "==", "!=", ">", "<"}

// parseWhere 解析 "field op value" 三段式表达式。
// value 支持数字（18、3.14）和字符串字面量（"alice"、'bob'）。
func parseWhere(s string) (*whereExpr, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, fmt.Errorf("where 表达式为空")
	}
	for _, op := range operators {
		if idx := strings.Index(s, op); idx > 0 {
			field := strings.TrimSpace(s[:idx])
			rest := strings.TrimSpace(s[idx+len(op):])
			if field == "" || rest == "" {
				return nil, fmt.Errorf("where 表达式 %q 字段或值缺失", s)
			}
			e := &whereExpr{field: field, op: op, value: rest}
			// 字符串字面量：去引号
			if len(rest) >= 2 && (rest[0] == '"' && rest[len(rest)-1] == '"' ||
				rest[0] == '\'' && rest[len(rest)-1] == '\'') {
				e.value = rest[1 : len(rest)-1]
				e.raw = true
			}
			return e, nil
		}
	}
	return nil, fmt.Errorf("where 表达式 %q 缺少操作符（支持 >, >=, <, <=, ==, !=）", s)
}

// match 判断单行是否满足条件。
func (e *whereExpr) match(row pipeline.Row) (bool, error) {
	got, present := row[e.field]
	if !present {
		// 字段不存在：== 视为不匹配，!= 视为匹配（与 nil 比较）
		if e.op == "!=" {
			return true, nil
		}
		return false, nil
	}

	leftStr := toString(got)
	// 两边都能转 float64 → 数值比较；否则字符串比较
	lf, lerr := strconv.ParseFloat(leftStr, 64)
	rf, rerr := strconv.ParseFloat(e.value, 64)
	if e.raw {
		// 显式字符串字面量 → 强制字符串比较
		return compareStr(leftStr, e.op, e.value), nil
	}
	if lerr == nil && rerr == nil {
		return compareNum(lf, e.op, rf), nil
	}
	return compareStr(leftStr, e.op, e.value), nil
}

func compareNum(l float64, op string, r float64) bool {
	switch op {
	case ">":
		return l > r
	case ">=":
		return l >= r
	case "<":
		return l < r
	case "<=":
		return l <= r
	case "==":
		return l == r
	case "!=":
		return l != r
	}
	return false
}

func compareStr(l, op, r string) bool {
	cmp := strings.Compare(l, r)
	switch op {
	case ">":
		return cmp > 0
	case ">=":
		return cmp >= 0
	case "<":
		return cmp < 0
	case "<=":
		return cmp <= 0
	case "==":
		return cmp == 0
	case "!=":
		return cmp != 0
	}
	return false
}

// toString 把任意值转成可比较字符串（数字、bool 等）。
func toString(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case fmt.Stringer:
		return x.String()
	case nil:
		return ""
	default:
		return fmt.Sprintf("%v", x)
	}
}

func init() {
	pipeline.RegisterTransform(&FilterTransform{})
}
