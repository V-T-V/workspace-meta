package api

import (
	"context"
	"fmt"
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
		if !verifyPassword(provided, s.adminPassword) {
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

// handlePurge 手动触发数据清理。
func (s *Server) handlePurge(w http.ResponseWriter, r *http.Request) {
	days := 90
	if d := r.URL.Query().Get("days"); d != "" {
		fmt.Sscanf(d, "%d", &days)
	}
	if days < 1 {
		days = 90
	}
	result, err := storage.PurgeExpiredData(r.Context(), s.db, days)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "purge_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// handleComplianceLogs 查询合规日志。
func (s *Server) handleComplianceLogs(w http.ResponseWriter, r *http.Request) {
	eventType := r.URL.Query().Get("type")
	limit := 100
	if l := r.URL.Query().Get("limit"); l != "" {
		fmt.Sscanf(l, "%d", &limit)
	}
	items, err := storage.ListComplianceLogs(r.Context(), s.db, eventType, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", "查询合规日志失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "total": len(items)})
}

// handleComplianceStats 合规统计汇总。
func (s *Server) handleComplianceStats(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.QueryContext(r.Context(),
		`SELECT event_type, COALESCE(action_taken,''), count(*) FROM compliance_logs GROUP BY event_type, action_taken ORDER BY count(*) DESC`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", "查询合规统计失败")
		return
	}
	defer rows.Close()
	type stat struct {
		EventType   string `json:"eventType"`
		Count       int    `json:"count"`
		ActionTaken string `json:"actionTaken"`
	}
	var stats []stat
	totalBlocks, totalRequests := 0, 0
	for rows.Next() {
		var st stat
		if rows.Scan(&st.EventType, &st.ActionTaken, &st.Count) == nil {
			stats = append(stats, st)
			if st.ActionTaken == "block" || st.ActionTaken == "refuse" {
				totalBlocks += st.Count
			}
			if st.EventType == "model_invoke" || st.EventType == "guard_block" {
				totalRequests += st.Count
			}
		}
	}
	blockRate := 0.0
	if totalRequests > 0 {
		blockRate = float64(totalBlocks) / float64(totalRequests) * 100
	}
	writeJSON(w, http.StatusOK, map[string]any{"stats": stats, "totalBlocks": totalBlocks, "totalRequests": totalRequests, "blockRate": blockRate})
}

// handleListModels 列出可用模型。
func (s *Server) handleListModels(w http.ResponseWriter, r *http.Request) {
	var available []string
	if mc, ok := s.model.(interface {
		AvailableModels(context.Context) []string
	}); ok {
		available = mc.AvailableModels(r.Context())
	}
	current := s.model.ChatModel()
	type modelInfo struct {
		Name    string `json:"name"`
		Current bool   `json:"current"`
	}
	var out []modelInfo
	found := false
	for _, m := range available {
		out = append(out, modelInfo{Name: m, Current: m == current})
		if m == current {
			found = true
		}
	}
	if !found {
		out = append([]modelInfo{{Name: current, Current: true}}, out...)
	}
	writeJSON(w, http.StatusOK, map[string]any{"models": out, "current": current, "backend": s.backend})
}

// handleSwitchModel 动态切换模型。
func (s *Server) handleSwitchModel(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Model string `json:"model"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", "请求体格式错误")
		return
	}
	if body.Model == "" {
		writeError(w, http.StatusBadRequest, "empty_model", "model 不能为空")
		return
	}
	mc, ok := s.model.(interface{ SetChatModel(string) })
	if !ok {
		writeError(w, http.StatusNotImplemented, "not_supported", "当前后端不支持动态切换")
		return
	}
	mc.SetChatModel(body.Model)
	writeJSON(w, http.StatusOK, map[string]any{"switched": true, "model": body.Model})
}
