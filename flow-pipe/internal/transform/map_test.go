package transform

import (
	"testing"

	"github.com/QiuShichang/flow-pipe/internal/pipeline"
)

func TestMapTransform_Lookup(t *testing.T) {
	rows := pipeline.Rows{{"status": "0"}, {"status": "1"}, {"status": "2"}}
	out, err := (MapTransform{}).Transform(rows, map[string]any{
		"lookup": map[string]any{
			"status": map[string]any{
				"0": "pending",
				"1": "done",
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if out[0]["status"] != "pending" || out[1]["status"] != "done" {
		t.Errorf("lookup 替换错: %v %v", out[0]["status"], out[1]["status"])
	}
	// 未在表里的值保持不变
	if out[2]["status"] != "2" {
		t.Errorf("未映射的值应保持原样，实际 %v", out[2]["status"])
	}
}

func TestMapTransform_CastFloat(t *testing.T) {
	rows := pipeline.Rows{{"amount": "100.5"}}
	out, err := (MapTransform{}).Transform(rows, map[string]any{
		"cast": map[string]any{"amount": "float"},
	})
	if err != nil {
		t.Fatal(err)
	}
	f, ok := out[0]["amount"].(float64)
	if !ok || f != 100.5 {
		t.Errorf("cast float 错: %v (type %T)", out[0]["amount"], out[0]["amount"])
	}
}

func TestMapTransform_CastInt(t *testing.T) {
	rows := pipeline.Rows{{"count": "42"}}
	out, err := (MapTransform{}).Transform(rows, map[string]any{
		"cast": map[string]any{"count": "int"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if i, ok := out[0]["count"].(int64); !ok || i != 42 {
		t.Errorf("cast int 错: %v (type %T)", out[0]["count"], out[0]["count"])
	}
}

func TestMapTransform_Template(t *testing.T) {
	rows := pipeline.Rows{{"first": "Alice", "last": "Smith"}}
	out, err := (MapTransform{}).Transform(rows, map[string]any{
		"template": map[string]any{"full": "{first} {last}"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if out[0]["full"] != "Alice Smith" {
		t.Errorf("template 拼接错: %v", out[0]["full"])
	}
}

func TestMapTransform_Combined(t *testing.T) {
	// 同时 lookup + cast + template
	rows := pipeline.Rows{{"status": "0", "amount": "99", "first": "Bob", "last": "Jones"}}
	out, err := (MapTransform{}).Transform(rows, map[string]any{
		"lookup":   map[string]any{"status": map[string]any{"0": "pending"}},
		"cast":     map[string]any{"amount": "float"},
		"template": map[string]any{"name": "{first} {last}"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if out[0]["status"] != "pending" {
		t.Errorf("lookup 错: %v", out[0]["status"])
	}
	if f, ok := out[0]["amount"].(float64); !ok || f != 99 {
		t.Errorf("cast 错: %v", out[0]["amount"])
	}
	if out[0]["name"] != "Bob Jones" {
		t.Errorf("template 错: %v", out[0]["name"])
	}
}

func TestMapTransform_Registered(t *testing.T) {
	if _, ok := pipeline.GetTransform("map"); !ok {
		t.Fatal("map transform 未注册")
	}
}
