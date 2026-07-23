// Package api 实现 HTTP 路由与处理器。
// 对应原计划第十节。M1 子集：health/chat/conversation/system。
package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// decodeJSON 解析请求体 JSON，限制最大 1MB。
// 允许未知字段（前向兼容）。
func decodeJSON(r *http.Request, v any) error {
	dec := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	return dec.Decode(v)
}

// writeJSON 写 JSON 响应。先编码到缓冲区，避免写出半截 JSON。
func writeJSON(w http.ResponseWriter, status int, v any) {
	buf, err := json.Marshal(v)
	if err != nil {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":{"code":"encode_failed","message":"响应编码失败"}}`))
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(buf)
	_, _ = w.Write([]byte("\n"))
}

// writeError 写统一错误响应。code 是可定位的错误码，msg 面向用户。
func writeError(w http.ResponseWriter, status int, code, msg string) {
	writeJSON(w, status, map[string]any{
		"error": map[string]any{
			"code":    code,
			"message": msg,
		},
	})
}

// writeInternalError 写 500 错误，用通用消息避免泄露内部细节。
// 详细错误应通过 log/slog 在服务端记录。
func writeInternalError(w http.ResponseWriter, code string) {
	writeError(w, http.StatusInternalServerError, code, "服务器内部错误，请稍后重试或联系管理员")
}

// sseHeaders 设置 Server-Sent Events 必需头。
func sseHeaders(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	// 禁用反向代理缓冲，保证 token 实时推送
	w.Header().Set("X-Accel-Buffering", "no")
}

// sseWrite 写一行 SSE 事件。event 为空则只发 data。
func sseWrite(w http.ResponseWriter, event string, data string) {
	if event != "" {
		fmt.Fprintf(w, "event: %s\n", event)
	}
	fmt.Fprintf(w, "data: %s\n\n", data)
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}
