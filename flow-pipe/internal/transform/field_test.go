package transform

import (
	"reflect"
	"testing"

	"github.com/QiuShichang/flow-pipe/internal/pipeline"
)

func TestFieldTransform_Add(t *testing.T) {
	rows := pipeline.Rows{{"id": 1}, {"id": 2}}
	out, err := (FieldTransform{}).Transform(rows, map[string]any{
		"add": map[string]any{"type": "user"},
	})
	if err != nil {
		t.Fatalf("Transform 失败: %v", err)
	}
	for _, r := range out {
		if r["type"] != "user" {
			t.Fatalf("add 字段未生效: %#v", r)
		}
		if r["id"] == nil {
			t.Fatalf("原字段丢失: %#v", r)
		}
	}
}

func TestFieldTransform_Rename(t *testing.T) {
	rows := pipeline.Rows{{"id": 1, "name": "x"}}
	out, err := (FieldTransform{}).Transform(rows, map[string]any{
		"rename": map[string]any{"id": "user_id"},
	})
	if err != nil {
		t.Fatalf("Transform 失败: %v", err)
	}
	r := out[0]
	if _, exists := r["id"]; exists {
		t.Fatalf("旧字段 id 应已不存在: %#v", r)
	}
	if r["user_id"] != 1 {
		t.Fatalf("新字段 user_id 应为 1: %#v", r)
	}
	if r["name"] != "x" {
		t.Fatalf("未改名的字段应保留: %#v", r)
	}
}

func TestFieldTransform_Drop(t *testing.T) {
	rows := pipeline.Rows{{"id": 1, "tmp": "x", "name": "n"}}
	out, err := (FieldTransform{}).Transform(rows, map[string]any{
		"drop": []any{"tmp"},
	})
	if err != nil {
		t.Fatalf("Transform 失败: %v", err)
	}
	r := out[0]
	if _, exists := r["tmp"]; exists {
		t.Fatalf("drop 字段未删除: %#v", r)
	}
	if r["id"] != 1 || r["name"] != "n" {
		t.Fatalf("其它字段应保留: %#v", r)
	}
}

func TestFieldTransform_Combined(t *testing.T) {
	// 顺序：先 add/rename 再 drop
	rows := pipeline.Rows{{"id": 1, "tmp": "x"}}
	out, err := (FieldTransform{}).Transform(rows, map[string]any{
		"add":    map[string]any{"type": "user"},
		"rename": map[string]any{"id": "user_id"},
		"drop":   []any{"tmp"},
	})
	if err != nil {
		t.Fatalf("Transform 失败: %v", err)
	}
	want := pipeline.Row{"user_id": 1, "type": "user"}
	if !reflect.DeepEqual(out[0], want) {
		t.Fatalf("组合操作结果错误: 得到 %#v，期望 %#v", out[0], want)
	}
}

func TestFieldTransform_RenameMissing_NoOp(t *testing.T) {
	rows := pipeline.Rows{{"id": 1}}
	out, err := (FieldTransform{}).Transform(rows, map[string]any{
		"rename": map[string]any{"nonexistent": "whatever"},
	})
	if err != nil {
		t.Fatalf("Transform 失败: %v", err)
	}
	if out[0]["id"] != 1 {
		t.Fatalf("rename 不存在的字段应无副作用: %#v", out[0])
	}
}

func TestFieldTransform_Registered(t *testing.T) {
	if _, ok := pipeline.GetTransform("field"); !ok {
		t.Fatal("field transform 未注册")
	}
}
