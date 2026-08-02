package source

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QiuShichang/flow-pipe/internal/pipeline"
)

func TestHTTPSource_Array(t *testing.T) {
	// 测试服务器返回 JSON 数组
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		data := []map[string]any{{"id": 1, "name": "a"}, {"id": 2, "name": "b"}}
		json.NewEncoder(w).Encode(data)
	}))
	defer srv.Close()

	rows, err := (HTTPSource{}).Read(map[string]any{"url": srv.URL, "timeout": 5})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("应返回 2 行，实际 %d", len(rows))
	}
	if rows[0]["name"] != "a" {
		t.Errorf("name 错: %v", rows[0]["name"])
	}
	// JSON 数字解析为 float64，用类型断言比较
	if id, ok := rows[1]["id"].(float64); !ok || id != 2 {
		t.Errorf("id 错: %v (type %T)", rows[1]["id"], rows[1]["id"])
	}
}

func TestHTTPSource_Root(t *testing.T) {
	// 响应是对象，数组嵌套在 data.items
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		obj := map[string]any{
			"data": map[string]any{
				"items": []any{
					map[string]any{"k": "v1"},
					map[string]any{"k": "v2"},
				},
			},
		}
		json.NewEncoder(w).Encode(obj)
	}))
	defer srv.Close()

	rows, err := (HTTPSource{}).Read(map[string]any{"url": srv.URL, "root": "data.items"})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("root 路径应取到 2 行，实际 %d", len(rows))
	}
	if rows[0]["k"] != "v1" {
		t.Errorf("root 路径取值错: %v", rows[0])
	}
}

func TestHTTPSource_MissingURL(t *testing.T) {
	if _, err := (HTTPSource{}).Read(map[string]any{}); err == nil {
		t.Error("缺少 url 应报错")
	}
}

func TestHTTPSource_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	if _, err := (HTTPSource{}).Read(map[string]any{"url": srv.URL}); err == nil {
		t.Error("HTTP 500 应报错")
	}
}

func TestHTTPSource_Registered(t *testing.T) {
	if _, ok := pipeline.GetSource("http"); !ok {
		t.Fatal("http source 未注册")
	}
}
