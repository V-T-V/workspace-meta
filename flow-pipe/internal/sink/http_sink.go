package sink

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/QiuShichang/flow-pipe/internal/pipeline"
)

// HTTPSink 把行 POST 到 HTTP endpoint。
// 行批量或逐条发送（按 batch_size 配置），endpoint 收到 JSON 数组或单对象。
type HTTPSink struct{}

// Type 返回连接器类型标识。
func (HTTPSink) Type() string { return "http" }

// Write 把 rows 发到 HTTP endpoint。config:
//
//	url       string  目标地址（必填）
//	method    string  HTTP 方法（默认 POST）
//	timeout   int     超时秒数（默认 30）
//	batch     bool    是否批量发送（true=一次 POST 全部 rows 的 JSON 数组；false=逐条 POST，默认 true）
//
// 示例: {url: "https://hook.example.com/ingest", batch: true}
func (HTTPSink) Write(rows pipeline.Rows, config map[string]any) error {
	url, ok := config["url"].(string)
	if !ok || url == "" {
		return fmt.Errorf("http sink 缺少 url 配置")
	}
	method, _ := config["method"].(string)
	if method == "" {
		method = http.MethodPost
	}
	timeoutSec := 30
	if t, ok := toInt(config["timeout"]); ok && t > 0 {
		timeoutSec = t
	}
	batch := true
	if b, ok := config["batch"].(bool); ok {
		batch = b
	}

	client := &http.Client{Timeout: time.Duration(timeoutSec) * time.Second}

	if batch {
		return httpPost(client, url, method, rows)
	}
	// 逐条发送
	for _, r := range rows {
		if err := httpPost(client, url, method, pipeline.Rows{r}); err != nil {
			return err
		}
	}
	return nil
}

// httpPost 发一次请求，body 是 rows 的 JSON。
func httpPost(client *http.Client, url, method string, rows pipeline.Rows) error {
	body, err := json.Marshal(rows)
	if err != nil {
		return fmt.Errorf("JSON 编码失败: %w", err)
	}
	req, err := http.NewRequest(method, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("构造请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP 请求失败 %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, url)
	}
	return nil
}

// toInt 把 any 转 int（YAML 解析数字可能是 int/int64/float64）。
// 注意：sink 包没有现成的 toInt，这里局部定义。
func toInt(v any) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case int64:
		return int(n), true
	case float64:
		return int(n), true
	}
	return 0, false
}

func init() {
	pipeline.RegisterSink(&HTTPSink{})
}
