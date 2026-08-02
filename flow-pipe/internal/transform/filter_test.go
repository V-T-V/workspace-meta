package transform

import (
	"testing"

	"github.com/QiuShichang/flow-pipe/internal/pipeline"
)

func sampleRows() pipeline.Rows {
	return pipeline.Rows{
		{"id": 1, "name": "alice", "age": 30},
		{"id": 2, "name": "bob", "age": 20},
		{"id": 3, "name": "carol", "age": 40},
	}
}

func TestFilterTransform_NumericGT(t *testing.T) {
	out, err := (FilterTransform{}).Transform(sampleRows(), map[string]any{"where": "age > 20"})
	if err != nil {
		t.Fatalf("Transform 失败: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("age>20 应剩 2 行，得到 %d", len(out))
	}
	for _, r := range out {
		if a, _ := r["age"].(int); a <= 20 {
			t.Fatalf("过滤结果含 age<=20: %#v", r)
		}
	}
}

func TestFilterTransform_NumericGE_LE(t *testing.T) {
	rows := sampleRows()
	ge, _ := (FilterTransform{}).Transform(rows, map[string]any{"where": "age >= 30"})
	if len(ge) != 2 {
		t.Fatalf("age>=30 应剩 2 行，得到 %d", len(ge))
	}
	le, _ := (FilterTransform{}).Transform(rows, map[string]any{"where": "age <= 30"})
	if len(le) != 2 {
		t.Fatalf("age<=30 应剩 2 行，得到 %d", len(le))
	}
}

func TestFilterTransform_EqAndNeq(t *testing.T) {
	rows := sampleRows()
	eq, _ := (FilterTransform{}).Transform(rows, map[string]any{"where": "age == 20"})
	if len(eq) != 1 || eq[0]["name"] != "bob" {
		t.Fatalf("age==20 应只剩 bob，得到 %#v", eq)
	}
	neq, _ := (FilterTransform{}).Transform(rows, map[string]any{"where": "age != 20"})
	if len(neq) != 2 {
		t.Fatalf("age!=20 应剩 2 行，得到 %d", len(neq))
	}
}

func TestFilterTransform_StringLiteral(t *testing.T) {
	out, err := (FilterTransform{}).Transform(sampleRows(), map[string]any{"where": `name == "bob"`})
	if err != nil {
		t.Fatalf("Transform 失败: %v", err)
	}
	if len(out) != 1 || out[0]["name"] != "bob" {
		t.Fatalf("字符串字面量匹配错误: %#v", out)
	}
}

func TestFilterTransform_StringComparison(t *testing.T) {
	// 字符串比较：name > "b" → alice(否? "alice">"b" 为 false) / bob(false) / carol(true)
	// 注意："carol" > "b" → true，"bob" > "b" → true（'o'>'b'），"alice">"b" → false（'a'<'b'）
	out, _ := (FilterTransform{}).Transform(sampleRows(), map[string]any{"where": "name > b"})
	names := []string{}
	for _, r := range out {
		names = append(names, r["name"].(string))
	}
	want := map[string]bool{"bob": true, "carol": true}
	if len(out) != 2 {
		t.Fatalf("name>b 应剩 2 行(bob,carol)，得到 %v", names)
	}
	for _, n := range names {
		if !want[n] {
			t.Fatalf("意外结果 %q", n)
		}
	}
}

func TestFilterTransform_NoWhere_Passthrough(t *testing.T) {
	out, err := (FilterTransform{}).Transform(sampleRows(), map[string]any{})
	if err != nil {
		t.Fatalf("无 where 不应报错: %v", err)
	}
	if len(out) != len(sampleRows()) {
		t.Fatalf("无 where 应透传，得到 %d", len(out))
	}
}

func TestFilterTransform_BadExpr(t *testing.T) {
	if _, err := (FilterTransform{}).Transform(sampleRows(), map[string]any{"where": "nonsense"}); err == nil {
		t.Fatal("非法 where 应报错")
	}
}

func TestFilterTransform_FieldMissing(t *testing.T) {
	// 字段不存在：== 不匹配，!= 匹配
	eq, _ := (FilterTransform{}).Transform(sampleRows(), map[string]any{"where": "missing == 1"})
	if len(eq) != 0 {
		t.Fatalf("缺失字段 == 应 0 行，得到 %d", len(eq))
	}
	neq, _ := (FilterTransform{}).Transform(sampleRows(), map[string]any{"where": "missing != 1"})
	if len(neq) != 3 {
		t.Fatalf("缺失字段 != 应全匹配(3 行)，得到 %d", len(neq))
	}
}

func TestFilterTransform_Registered(t *testing.T) {
	if _, ok := pipeline.GetTransform("filter"); !ok {
		t.Fatal("filter transform 未注册")
	}
}
