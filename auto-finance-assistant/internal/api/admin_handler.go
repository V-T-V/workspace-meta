package api

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/QiuShichang/auto-finance-assistant/internal/chat"
	"github.com/QiuShichang/auto-finance-assistant/internal/storage"
)

// AuthMiddleware 单管理员密码认证（配置为空时不校验）。
// 仅保护管理类接口（feedback/audit/metrics/backup）。聊天接口不强制。
func (s *Server) AuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.adminPassword == "" {
			next(w, r)
			return
		}
		// Bearer token 或 X-Admin-Password 头
		provided := r.Header.Get("X-Admin-Password")
		if provided == "" {
			if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
				provided = strings.TrimPrefix(auth, "Bearer ")
			}
		}
		if subtle.ConstantTimeCompare([]byte(provided), []byte(s.adminPassword)) != 1 {
			writeError(w, http.StatusUnauthorized, "unauthorized", "管理员认证失败")
			return
		}
		next(w, r)
	}
}

// --- 反馈 ---

type feedbackRequest struct {
	MessageID  string `json:"messageId"`
	Rating     int    `json:"rating"`    // 1=赞 -1=踩
	Reason     string `json:"reason"`
	Correction string `json:"correction"`
}

func (s *Server) handleCreateFeedback(w http.ResponseWriter, r *http.Request) {
	var body feedbackRequest
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", "请求体格式错误")
		return
	}
	if body.Rating != 1 && body.Rating != -1 {
		writeError(w, http.StatusBadRequest, "invalid_rating", "rating 必须为 1（赞）或 -1（踩）")
		return
	}
	// 限制长度 + 脱敏
	reason := truncateStr(body.Reason, 500)
	correction := truncateStr(body.Correction, 1000)
	if err := storage.CreateFeedback(r.Context(), s.importer.DB(), &storage.Feedback{
		ID: uuid.NewString(), MessageID: body.MessageID, Rating: body.Rating,
		Reason: chat.MaskPII(reason), Correction: chat.MaskPII(correction),
	}); err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"created": true})
}

func (s *Server) handleListFeedback(w http.ResponseWriter, r *http.Request) {
	items, err := storage.ListFeedback(r.Context(), s.importer.DB(), 100)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", "查询反馈失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "total": len(items)})
}

// --- 审计 ---

func (s *Server) handleListAuditLogs(w http.ResponseWriter, r *http.Request) {
	action := r.URL.Query().Get("action")
	items, err := storage.ListAuditLogs(r.Context(), s.importer.DB(), action, 200)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", "查询审计日志失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "total": len(items)})
}

// --- 无答案问题（拒答列表）---

func (s *Server) handleListRefused(w http.ResponseWriter, r *http.Request) {
	items, err := storage.ListRefusedMessages(r.Context(), s.importer.DB(), 100)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", "查询拒答列表失败")
		return
	}
	out := make([]map[string]any, 0, len(items))
	for _, m := range items {
		out = append(out, map[string]any{
			"messageId":    m.ID,
			"content":      m.Content,
			"confidence":   m.Confidence,
			"createdAt":    m.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": out, "total": len(out)})
}

// --- 指标 ---

func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	db := s.importer.DB()
	var convCount, msgCount, faqCount, docCount, feedbackCount, refuseCount int
	_ = db.QueryRowContext(ctx, `SELECT count(*) FROM conversations`).Scan(&convCount)
	_ = db.QueryRowContext(ctx, `SELECT count(*) FROM messages`).Scan(&msgCount)
	_ = db.QueryRowContext(ctx, `SELECT count(*) FROM faqs`).Scan(&faqCount)
	_ = db.QueryRowContext(ctx, `SELECT count(*) FROM documents`).Scan(&docCount)
	_ = db.QueryRowContext(ctx, `SELECT count(*) FROM feedback`).Scan(&feedbackCount)
	_ = db.QueryRowContext(ctx, `SELECT count(*) FROM messages WHERE intent='refuse'`).Scan(&refuseCount)

	vecCount := 0
	if s.vector != nil {
		vecCount = s.vector.Index().Size()
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"conversations":  convCount,
		"messages":       msgCount,
		"faqs":           faqCount,
		"documents":      docCount,
		"feedback":       feedbackCount,
		"refusedAnswers": refuseCount,
		"vectorIndex":    vecCount,
		"queueActive":    s.queue.Active(),
		"queueWaiting":   s.queue.Waiting(),
	})
}

// SetAdminPassword 注入管理员密码（main 装配时调用）。
func (s *Server) SetAdminPassword(p string) { s.adminPassword = p }

// truncateStr 限制字符串长度（rune 计）。
func truncateStr(s string, maxRunes int) string {
	r := []rune(s)
	if len(r) <= maxRunes {
		return s
	}
	return string(r[:maxRunes])
}
