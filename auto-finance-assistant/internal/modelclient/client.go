// Package modelclient 抽象模型推理后端（Ollama / llama.cpp）。
// 通过 config 的 backend 字段选择实现，chat.Service 等只依赖 ModelClient 接口。
package modelclient

import (
	"context"
)

// Message 是 chat 消息（Ollama 和 OpenAI 格式通用）。
type Message struct {
	Role    string `json:"role"`    // system | user | assistant
	Content string `json:"content"`
}

// ChatEvent 是流式输出的单帧事件。
type ChatEvent struct {
	Token            string
	Done             bool
	PromptTokens     int
	CompletionTokens int
	TotalDuration    int64
	Error            error
}

// HealthStatus 是健康检查结果。
type HealthStatus struct {
	Reachable bool
	HasModel  bool
	Models    []string
	Version   string
}

// MissingHint 返回模型缺失/服务不可达的可读提示。
func (s HealthStatus) MissingHint(baseURL, wantModel string) string {
	if !s.Reachable {
		return "模型服务不可达（" + baseURL + "），请确认服务已启动"
	}
	if !s.HasModel {
		return "模型 " + wantModel + " 未加载（本地已有：" + joinModels(s.Models) + "）"
	}
	return ""
}

// ModelClient 是模型推理后端接口。Ollama 和 llama.cpp 各有实现。
type ModelClient interface {
	// Chat 流式对话。model 为模型名，systemPrompt 为系统提示，history 为历史消息。
	// 返回事件 channel，逐 token 推送。调用方负责消费至 Done/Error。
	Chat(ctx context.Context, model, systemPrompt string, history []Message) (<-chan ChatEvent, error)
	// Embed 批量生成向量。
	Embed(ctx context.Context, texts []string, batchSize int) ([][]float32, error)
	// Health 健康检查。
	Health(ctx context.Context) HealthStatus
	// ChatModel 返回配置的对话模型名。
	ChatModel() string
	// EmbeddingModel 返回配置的向量模型名。
	EmbeddingModel() string
	// BaseURL 返回服务地址。
	BaseURL() string
}

func joinModels(models []string) string {
	result := ""
	for i, m := range models {
		if i > 0 {
			result += ", "
		}
		result += m
	}
	return result
}
