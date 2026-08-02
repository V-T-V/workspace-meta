package api

import (
	"errors"
	"net/http"

	"github.com/QiuShichang/auto-finance-assistant/internal/storage"
)

type createConversationRequest struct {
	Title string `json:"title"`
}

type conversationSummary struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

func (s *Server) handleCreateConversation(w http.ResponseWriter, r *http.Request) {
	var req createConversationRequest
	// 空 body 合法（用默认标题）
	if r.ContentLength > 0 {
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_body", "请求体格式错误")
			return
		}
	}
	title := req.Title
	if title == "" {
		title = "新会话"
	}
	conv, err := s.chat.CreateConversation(r.Context(), "", title)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", "创建会话失败")
		return
	}
	writeJSON(w, http.StatusCreated, conversationSummary{
		ID:        conv.ID,
		Title:     conv.Title,
		CreatedAt: conv.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt: conv.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	})
}

func (s *Server) handleListConversations(w http.ResponseWriter, r *http.Request) {
	convs, err := s.chat.ListConversations(r.Context(), 50)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", "查询会话列表失败")
		return
	}
	out := make([]conversationSummary, 0, len(convs))
	for _, c := range convs {
		out = append(out, conversationSummary{
			ID:        c.ID,
			Title:     c.Title,
			CreatedAt: c.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			UpdatedAt: c.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": out, "total": len(out)})
}

type conversationDetail struct {
	conversationSummary
	Messages []messageItem `json:"messages"`
}

type messageItem struct {
	ID        string `json:"id"`
	Role      string `json:"role"`
	Content   string `json:"content"`
	CreatedAt string `json:"createdAt"`
}

func (s *Server) handleGetConversation(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	conv, msgs, err := s.chat.GetConversationWithMessages(r.Context(), id)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "会话不存在")
			return
		}
		writeError(w, http.StatusInternalServerError, "db_error", "查询会话失败")
		return
	}
	items := make([]messageItem, 0, len(msgs))
	for _, m := range msgs {
		items = append(items, messageItem{
			ID:        m.ID,
			Role:      m.Role,
			Content:   m.Content,
			CreatedAt: m.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		})
	}
	writeJSON(w, http.StatusOK, conversationDetail{
		conversationSummary: conversationSummary{
			ID:        conv.ID,
			Title:     conv.Title,
			CreatedAt: conv.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			UpdatedAt: conv.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
		},
		Messages: items,
	})
}

// handleDeleteConversation PIPL 被遗忘权：删除会话及所有消息。
func (s *Server) handleDeleteConversation(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "invalid_id", "会话 ID 不能为空")
		return
	}
	if err := storage.DeleteConversation(r.Context(), s.db, id); err != nil {
		writeError(w, http.StatusInternalServerError, "db_error", "删除会话失败")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"deleted": true, "id": id})
}
