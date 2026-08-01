package source

import (
	"testing"

	"github.com/QiuShichang/flow-pipe/internal/pipeline"
)

func TestJSONSource_ReadArray(t *testing.T) {
	p := writeTemp(t, "data.json", `[{"a":1,"b":"x"},{"a":2,"b":"y"}]`)
	src := JSONSource{}
	if src.Type() != "json" {
		t.Fatalf("Type 应为 json，得到 %q", src.Type())
	}
	rows, err := src.Read(map[string]any{"path": p})
	if err != nil {
		t.Fatalf("Read 失败: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("应为 2 行，得到 %d", len(rows))
	}
	// JSON 数字解析成 float64
	if v, ok := rows[0]["a"].(float64); !ok || v != 1 {
		t.Fatalf("首行 a 应为 float64(1)，得到 %#v", rows[0]["a"])
	}
	if rows[0]["b"] != "x" {
		t.Fatalf("首行 b 错误: %#v", rows[0]["b"])
	}
}

func TestJSONSource_SingleObject(t *testing.T) {
	p := writeTemp(t, "single.json", `{"k":"v","n":5}`)
	rows, err := JSONSource{}.Read(map[string]any{"path": p})
	if err != nil {
		t.Fatalf("Read 单对象失败: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("单对象应返回 1 行，得到 %d", len(rows))
	}
	if rows[0]["k"] != "v" {
		t.Fatalf("k 错误: %#v", rows[0]["k"])
	}
}

func TestJSONSource_MissingPath(t *testing.T) {
	if _, err := (JSONSource{}).Read(map[string]any{}); err == nil {
		t.Fatal("缺少 path 应报错")
	}
}

func TestJSONSource_BadFormat(t *testing.T) {
	p := writeTemp(t, "bad.json", `not json`)
	if _, err := (JSONSource{}).Read(map[string]any{"path": p}); err == nil {
		t.Fatal("非法 JSON 应报错")
	}
}

func TestJSONSource_Registered(t *testing.T) {
	if _, ok := pipeline.GetSource("json"); !ok {
		t.Fatal("json source 未注册")
	}
}
