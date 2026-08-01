package source

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/QiuShichang/flow-pipe/internal/pipeline"
)

// HTTPSource 从 HTTP GET 请求读 JSON 数据，解析成 Rows。
// 响应体必须是 JSON 数组 [{...}, {...}] 或单个对象 {...}（自动包成一行）。
type HTTPSource struct{}

// Type 返回连接器类型标识。
func (HTTPSource) Type() string { return "http" }

// Read 根据 config 发 HTTP GET，解析 JSON 数组成 Rows。config:
//
//	url      string  请求地址（必填）
//	timeout  int     超时秒数（默认 30）
//	root     string  响应 JSON 里数组的路径（如 "data.items"，默认空=整体是数组）
//
// 示例: {url: "https://api.example.com/users", timeout: 10}
func (HTTPSource) Read(config map[string]any) (pipeline.Rows, error) {
	url, ok := config["url"].(string)
	if !ok || url == "" {
		return nil, fmt.Errorf("http source 缺少 url 配置")
	}
	timeoutSec := 30
	if t, ok := toInt(config["timeout"]); ok && t > 0 {
		timeoutSec = t
	}
	root, _ := config["root"].(string)

	client := &http.Client{Timeout: time.Duration(timeoutSec) * time.Second}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("构造 HTTP 请求失败: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP 请求失败 %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, url)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20)) // 上限 10MB，防 OOM
	if err != nil {
		return nil, fmt.Errorf("读取响应体失败: %w", err)
	}

	// 解析 JSON。支持两种：数组（直接用）或对象（按 root 路径取数组）。
	var arr []map[string]any
	if root != "" {
		// 按点分路径取嵌套数组，如 "data.items"
		var obj map[string]any
		if err := json.Unmarshal(body, &obj); err != nil {
			return nil, fmt.Errorf("解析响应 JSON 失败（期望对象）: %w", err)
		}
		v, ok := extractPath(obj, root)
		if !ok {
			return nil, fmt.Errorf("root 路径 %q 不存在", root)
		}
		list, ok := v.([]any)
		if !ok {
			return nil, fmt.Errorf("root 路径 %q 不是数组", root)
		}
		arr = toRows(list)
	} else {
		// 尝试数组；若失败尝试单对象包成一行
		if err := json.Unmarshal(body, &arr); err != nil {
			var single map[string]any
			if err2 := json.Unmarshal(body, &single); err2 == nil {
				return pipeline.Rows{single}, nil
			}
			return nil, fmt.Errorf("解析响应 JSON 失败（期望数组或对象）: %w", err)
		}
	}

	rows := make(pipeline.Rows, 0, len(arr))
	for _, m := range arr {
		rows = append(rows, pipeline.Row(m))
	}
	return rows, nil
}

// extractPath 按点分路径从 obj 取值，如 obj["data"]["items"]。
func extractPath(obj map[string]any, path string) (any, bool) {
	var cur any = obj
	first := true
	for _, seg := range splitDot(path) {
		if seg == "" {
			continue
		}
		m, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}
		cur, ok = m[seg]
		if !ok {
			return nil, false
		}
		_ = first
	}
	return cur, true
}

func splitDot(s string) []string {
	var out []string
	cur := ""
	for _, r := range s {
		if r == '.' {
			out = append(out, cur)
			cur = ""
			continue
		}
		cur += string(r)
	}
	if cur != "" {
		out = append(out, cur)
	}
	return out
}

// toRows 把 []any（JSON 数组解析结果）转成 []map[string]any。
func toRows(list []any) []map[string]any {
	out := make([]map[string]any, 0, len(list))
	for _, item := range list {
		if m, ok := item.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}

func init() {
	pipeline.RegisterSource(&HTTPSource{})
}
