package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/QiuShichang/auto-finance-assistant/internal/chat"
	"github.com/QiuShichang/auto-finance-assistant/internal/queue"
	"github.com/QiuShichang/auto-finance-assistant/internal/rag"
)

type chatRequestBody struct {
	ConversationID string `json:"conversationId"`
	Question       string `json:"question"`
}

// handleChat 非流式问答：聚合完整回答后一次返回。
func (s *Server) handleChat(w http.ResponseWriter, r *http.Request) {
	var body chatRequestBody
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", "请求体格式错误")
		return
	}
	if body.Question == "" {
		writeError(w, http.StatusBadRequest, "empty_question", "问题不能为空")
		return
	}

	// 先确认 Ollama 可达，给出明确错误而非让队列里的生成失败
	if hs := s.model.Health(r.Context()); !hs.Reachable || !hs.HasModel {
		writeError(w, http.StatusServiceUnavailable, "ollama_unavailable",
			hs.MissingHint(s.model.BaseURL(), s.model.ChatModel()))
		return
	}

	events, err := s.chat.AnswerStream(r.Context(), chat.ChatRequest{
		ConversationID: body.ConversationID,
		Question:       body.Question,
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, "chat_init_failed", err.Error())
		return
	}

	var fullAnswer string
	var final chat.ChatResponse
	var sources []chat.Source
	for ev := range events {
		switch ev.Type {
		case "token":
			fullAnswer += ev.Payload
		case "source":
			if r, ok := ev.Extra.(rag.SearchResult); ok {
				sources = append(sources, chat.Source{
					DocumentName:  r.DocumentName,
					Section:       r.Section,
					Version:       r.Version,
					EffectiveDate: r.EffectiveDate,
				})
			}
		case "complete":
			if resp, ok := ev.Extra.(chat.ChatResponse); ok {
				final = resp
			}
		case "error":
			if errors.Is(ev.Err, queue.ErrBusy) {
				writeError(w, http.StatusServiceUnavailable, "system_busy",
					"系统繁忙，请稍后再试")
				return
			}
			writeError(w, http.StatusInternalServerError, "generation_failed",
				ev.Err.Error())
			return
		}
	}

	// Ollama 路径用聚合的 fullAnswer；FAQ 短路路径无 token，保留 complete 里的 Answer
	if fullAnswer != "" {
		final.Answer = fullAnswer
	}
	if len(sources) > 0 {
		final.Sources = sources
	}
	writeJSON(w, http.StatusOK, final)
}

// handleChatStream SSE 流式问答。对应原计划 10.2 SSE 事件。
func (s *Server) handleChatStream(w http.ResponseWriter, r *http.Request) {
	var body chatRequestBody
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", "请求体格式错误")
		return
	}
	if body.Question == "" {
		writeError(w, http.StatusBadRequest, "empty_question", "问题不能为空")
		return
	}

	if hs := s.model.Health(r.Context()); !hs.Reachable || !hs.HasModel {
		writeError(w, http.StatusServiceUnavailable, "ollama_unavailable",
			hs.MissingHint(s.model.BaseURL(), s.model.ChatModel()))
		return
	}

	events, err := s.chat.AnswerStream(r.Context(), chat.ChatRequest{
		ConversationID: body.ConversationID,
		Question:       body.Question,
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, "chat_init_failed", err.Error())
		return
	}

	// 检测客户端断开
	notify := r.Context()
	sseHeaders(w)

	for ev := range events {
		if notify.Err() != nil {
			return // 客户端断开，停止
		}
		switch ev.Type {
		case "status":
			payload, _ := json.Marshal(map[string]string{
				"status":  ev.Payload,
				"traceId": getTraceID(ev.Extra),
			})
			sseWrite(w, "status", string(payload))
		case "token":
			payload, _ := json.Marshal(map[string]string{"token": ev.Payload})
			sseWrite(w, "token", string(payload))
		case "source":
			// M4：推送检索到的知识来源
			if r, ok := ev.Extra.(rag.SearchResult); ok {
				payload, _ := json.Marshal(map[string]any{
					"documentName":  r.DocumentName,
					"section":       r.Section,
					"version":       r.Version,
					"effectiveDate": r.EffectiveDate,
					"score":         r.Score,
				})
				sseWrite(w, "source", string(payload))
			}
		case "complete":
			resp, _ := ev.Extra.(chat.ChatResponse)
			payload, _ := json.Marshal(resp)
			sseWrite(w, "complete", string(payload))
			return
		case "error":
			code := "generation_failed"
			if errors.Is(ev.Err, queue.ErrBusy) {
				code = "system_busy"
			}
			payload, _ := json.Marshal(map[string]string{
				"code":    code,
				"message": ev.Err.Error(),
			})
			sseWrite(w, "error", string(payload))
			return
		}
	}
}

// getTraceID 从 status 事件的 Extra（string）提取 traceId。
func getTraceID(extra any) string {
	if id, ok := extra.(string); ok {
		return id
	}
	return ""
}
