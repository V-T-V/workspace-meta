package api

import (
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/QiuShichang/auto-finance-assistant/internal/storage"
)

// faqRequestBody 是创建/更新 FAQ 的请求体。
type faqRequestBody struct {
	Category  string `json:"category"`
	Question  string `json:"question"`
	Answer    string `json:"answer"`
	Keywords  string `json:"keywords"`
	Enabled   *bool  `json:"enabled"`   // 指针：未传时默认 true
	Priority  int    `json:"priority"`
}

// faqResponse 是 FAQ 的对外结构。
type faqResponse struct {
	ID        string `json:"id"`
	Category  string `json:"category"`
	Question  string `json:"question"`
	Answer    string `json:"answer"`
	Keywords  string `json:"keywords"`
	Enabled   bool   `json:"enabled"`
	Priority  int    `json:"priority"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

func toFAQResponse(f *storage.FAQ) faqResponse {
	return faqResponse{
		ID:        f.ID,
		Category:  f.Category,
		Question:  f.Question,
		Answer:    f.Answer,
		Keywords:  f.Keywords,
		Enabled:   f.Enabled,
		Priority:  f.Priority,
		CreatedAt: f.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt: f.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}

func (s *Server) handleCreateFAQ(w http.ResponseWriter, r *http.Request) {
	var body faqRequestBody
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", "请求体格式错误")
		return
	}
	if strings.TrimSpace(body.Question) == "" || strings.TrimSpace(body.Answer) == "" {
		writeError(w, http.StatusBadRequest, "invalid_faq", "question 和 answer 不能为空")
		return
	}
	enabled := true
	if body.Enabled != nil {
		enabled = *body.Enabled
	}
	faq := &storage.FAQ{
		ID:       uuid.NewString(),
		Category: body.Category,
		Question: strings.TrimSpace(body.Question),
		Answer:   strings.TrimSpace(body.Answer),
		Keywords: body.Keywords,
		Enabled:  enabled,
		Priority: body.Priority,
	}
	if err := s.chat.CreateFAQ(r.Context(), faq); err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", err.Error())
		return
	}
	full, _ := s.chat.GetFAQ(r.Context(), faq.ID)
	writeJSON(w, http.StatusCreated, toFAQResponse(full))
}

func (s *Server) handleListFAQs(w http.ResponseWriter, r *http.Request) {
	enabledOnly := r.URL.Query().Get("enabled") == "true"
	items, err := s.chat.ListFAQs(r.Context(), enabledOnly, 200)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", "查询 FAQ 列表失败")
		return
	}
	out := make([]faqResponse, 0, len(items))
	for _, f := range items {
		out = append(out, toFAQResponse(f))
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": out, "total": len(out)})
}

func (s *Server) handleGetFAQ(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	f, err := s.chat.GetFAQ(r.Context(), id)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "FAQ 不存在")
			return
		}
		writeError(w, http.StatusInternalServerError, "db_error", "查询 FAQ 失败")
		return
	}
	writeJSON(w, http.StatusOK, toFAQResponse(f))
}

func (s *Server) handleUpdateFAQ(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var body faqRequestBody
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", "请求体格式错误")
		return
	}
	existing, err := s.chat.GetFAQ(r.Context(), id)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "FAQ 不存在")
			return
		}
		writeError(w, http.StatusInternalServerError, "db_error", "查询 FAQ 失败")
		return
	}
	if body.Question != "" {
		existing.Question = strings.TrimSpace(body.Question)
	}
	if body.Answer != "" {
		existing.Answer = strings.TrimSpace(body.Answer)
	}
	if body.Category != "" {
		existing.Category = body.Category
	}
	if body.Keywords != "" {
		existing.Keywords = body.Keywords
	}
	if body.Enabled != nil {
		existing.Enabled = *body.Enabled
	}
	if body.Priority != 0 {
		existing.Priority = body.Priority
	}
	if err := s.chat.UpdateFAQ(r.Context(), existing); err != nil {
		writeInternalError(w, "db_error")
		return
	}
	full, _ := s.chat.GetFAQ(r.Context(), id)
	writeJSON(w, http.StatusOK, toFAQResponse(full))
}

func (s *Server) handleDeleteFAQ(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.chat.DeleteFAQ(r.Context(), id); err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "FAQ 不存在")
			return
		}
		writeError(w, http.StatusInternalServerError, "db_error", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"deleted": true, "id": id})
}

// faqImportRequest 批量导入。
type faqImportRequest struct {
	Items []faqRequestBody `json:"items"`
}

// faqImportResult 导入结果（逐条成功/失败原因）。
type faqImportResult struct {
	Success int             `json:"success"`
	Failed  int             `json:"failed"`
	Errors  []importFailure `json:"errors,omitempty"`
}

type importFailure struct {
	Index   int    `json:"index"`
	Question string `json:"question"`
	Reason  string `json:"reason"`
}

func (s *Server) handleImportFAQs(w http.ResponseWriter, r *http.Request) {
	var body faqImportRequest
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", "请求体格式错误")
		return
	}
	if len(body.Items) == 0 {
		writeError(w, http.StatusBadRequest, "empty_import", "导入列表不能为空")
		return
	}

	result := faqImportResult{}
	for i, item := range body.Items {
		if strings.TrimSpace(item.Question) == "" || strings.TrimSpace(item.Answer) == "" {
			result.Failed++
			result.Errors = append(result.Errors, importFailure{
				Index: i, Question: item.Question, Reason: "question 或 answer 为空",
			})
			continue
		}
		enabled := true
		if item.Enabled != nil {
			enabled = *item.Enabled
		}
		faq := &storage.FAQ{
			ID:       uuid.NewString(),
			Category: item.Category,
			Question: strings.TrimSpace(item.Question),
			Answer:   strings.TrimSpace(item.Answer),
			Keywords: item.Keywords,
			Enabled:  enabled,
			Priority: item.Priority,
		}
		if err := s.chat.CreateFAQ(r.Context(), faq); err != nil {
			result.Failed++
			result.Errors = append(result.Errors, importFailure{
				Index: i, Question: item.Question, Reason: err.Error(),
			})
			continue
		}
		result.Success++
	}
	writeJSON(w, http.StatusOK, result)
}

// faqTestRequest 测试匹配。
type faqTestRequest struct {
	Question string `json:"question"`
}

func (s *Server) handleTestFAQMatch(w http.ResponseWriter, r *http.Request) {
	var body faqTestRequest
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", "请求体格式错误")
		return
	}
	if strings.TrimSpace(body.Question) == "" {
		writeError(w, http.StatusBadRequest, "empty_question", "问题不能为空")
		return
	}
	match := s.chat.TestFAQMatch(body.Question)
	resp := map[string]any{
		"strategy":  match.Strategy,
		"score":     match.Score,
		"hit":       match.IsHighConfidence(),
	}
	if match.FAQ != nil {
		resp["faq"] = toFAQResponse(match.FAQ)
	}
	writeJSON(w, http.StatusOK, resp)
}
