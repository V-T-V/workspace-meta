package sink

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QiuShichang/flow-pipe/internal/pipeline"
)

func TestHTTPSink_Batch(t *testing.T) {
	var gotBody []map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotBody)
		if r.Header.Get("Content-Type") != "application/json" {
			t.Error("Content-Type 应为 application/json")
		}
	}))
	defer srv.Close()

	rows := pipeline.Rows{{"a": 1}, {"a": 2}}
	if err := (HTTPSink{}).Write(rows, map[string]any{"url": srv.URL, "timeout": 5}); err != nil {
		t.Fatal(err)
	}
	if len(gotBody) != 2 {
		t.Errorf("应收到 2 行，实际 %d", len(gotBody))
	}
}

func TestHTTPSink_SingleByOne(t *testing.T) {
	count := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count++
	}))
	defer srv.Close()

	rows := pipeline.Rows{{"a": 1}, {"a": 2}, {"a": 3}}
	if err := (HTTPSink{}).Write(rows, map[string]any{"url": srv.URL, "batch": false}); err != nil {
		t.Fatal(err)
	}
	if count != 3 {
		t.Errorf("逐条发送应请求 3 次，实际 %d", count)
	}
}

func TestHTTPSink_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer srv.Close()
	err := (HTTPSink{}).Write(pipeline.Rows{{"a": 1}}, map[string]any{"url": srv.URL})
	if err == nil {
		t.Error("HTTP 400 应报错")
	}
}

func TestHTTPSink_MissingURL(t *testing.T) {
	if err := (HTTPSink{}).Write(pipeline.Rows{{"a": 1}}, map[string]any{}); err == nil {
		t.Error("缺少 url 应报错")
	}
}

func TestHTTPSink_Registered(t *testing.T) {
	if _, ok := pipeline.GetSink("http"); !ok {
		t.Fatal("http sink 未注册")
	}
}
