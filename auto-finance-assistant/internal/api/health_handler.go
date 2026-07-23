package api

import (
	"net/http"
)

// healthResponse 是 /api/health 返回结构。对应原计划 10.1。
type healthResponse struct {
	Status   string `json:"status"`   // ok | degraded
	Database string `json:"database"` // ok | down
	Ollama   string `json:"ollama"`   // ok | down | missing_model
	Model    string `json:"model"`
	Version  string `json:"version"`
	Detail   string `json:"detail,omitempty"` // 故障时的可读提示
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	resp := healthResponse{
		Database: "ok",
		Model:    s.model.ChatModel(),
		Version:  version,
	}

	hs := s.model.Health(r.Context())
	switch {
	case !hs.Reachable:
		resp.Ollama = "down"
		resp.Status = "degraded"
		resp.Detail = hs.MissingHint(s.model.BaseURL(), s.model.ChatModel())
	case !hs.HasModel:
		resp.Ollama = "missing_model"
		resp.Status = "degraded"
		resp.Detail = hs.MissingHint(s.model.BaseURL(), s.model.ChatModel())
	default:
		resp.Ollama = "ok"
		resp.Status = "ok"
	}

	status := http.StatusOK
	if resp.Status != "ok" {
		status = http.StatusServiceUnavailable
	}
	writeJSON(w, status, resp)
}

type systemInfoResponse struct {
	Version         string `json:"version"`
	OllamaBaseURL   string `json:"ollamaBaseUrl"`
	OllamaReachable bool   `json:"ollamaReachable"`
	Models          []string `json:"models"`
	Concurrency     int    `json:"concurrency"`
	SystemPromptLen int    `json:"systemPromptLen"`
}

func (s *Server) handleSystemInfo(w http.ResponseWriter, r *http.Request) {
	hs := s.model.Health(r.Context())
	writeJSON(w, http.StatusOK, systemInfoResponse{
		Version:         version,
		OllamaBaseURL:   s.model.BaseURL(),
		OllamaReachable: hs.Reachable,
		Models:          hs.Models,
		Concurrency:     s.queue.Concurrency(),
		SystemPromptLen: len(s.chat.SystemPrompt()),
	})
}

type systemModelResponse struct {
	ChatModel      string `json:"chatModel"`
	EmbeddingModel string `json:"embeddingModel"`
	HasChatModel   bool   `json:"hasChatModel"`
}

func (s *Server) handleSystemModel(w http.ResponseWriter, r *http.Request) {
	hs := s.model.Health(r.Context())
	writeJSON(w, http.StatusOK, systemModelResponse{
		ChatModel:      s.model.ChatModel(),
		EmbeddingModel: s.model.EmbeddingModel(),
		HasChatModel:   hs.HasModel,
	})
}
