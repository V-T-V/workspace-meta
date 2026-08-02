package api

import (
	"context"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/QiuShichang/auto-finance-assistant/internal/storage"
)

// writeAudit 异步写入审计日志（不阻塞请求）。
// action 标识操作类型（faq.create / doc.upload / compliance.refuse 等）。
// targetType/targetID 标识操作对象。
func (s *Server) writeAudit(r *http.Request, action, targetType, targetID, detail string) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		ip := clientIP(r)
		_ = storage.CreateAuditLog(ctx, s.db, &storage.AuditLog{
			ID:         uuid.NewString(),
			Action:     action,
			TargetType: targetType,
			TargetID:   targetID,
			Detail:     detail,
			IPAddress:  ip,
		})
	}()
}

// AuditMiddleware 包装写操作，请求完成后异步记录审计日志。
// action 为审计事件标识（如 "faq.create"）。
func (s *Server) AuditMiddleware(action string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 用 wrappedWriter 捕获状态码
		sw := &statusWriter{ResponseWriter: w, status: 200}
		next(sw, r)

		// 只审计成功的写操作（2xx）
		if r.Method != "GET" && sw.status >= 200 && sw.status < 300 {
			s.writeAudit(r, action, r.URL.Path, "", r.Method+" "+r.URL.Path)
		}
	}
}

// AuditAuthMiddleware 认证失败时记录审计日志。
func (s *Server) AuditAuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.adminPassword == "" {
			next(w, r)
			return
		}
		sw := &statusWriter{ResponseWriter: w, status: 200}
		next(sw, r)
		if sw.status == http.StatusUnauthorized {
			s.writeAudit(r, "auth.failed", "", "", "认证失败")
		}
	}
}

// clientIP 提取客户端 IP（优先 X-Forwarded-For，回退 RemoteAddr）。
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return xff
	}
	return r.RemoteAddr
}

// statusWriter 包装 http.ResponseWriter 以捕获状态码。
type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}
