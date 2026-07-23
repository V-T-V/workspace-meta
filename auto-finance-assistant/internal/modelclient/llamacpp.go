package modelclient

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/QiuShichang/auto-finance-assistant/internal/config"
)

// LlamaCppClient 实现 ModelClient 接口，调用 llama-server 的 OpenAI 兼容 API。
type LlamaCppClient struct {
	baseURL string // 含 /v1 后缀，如 http://127.0.0.1:8081/v1
	rootURL string // 不含 /v1，如 http://127.0.0.1:8081（用于 /health）
	http    *http.Client
	cfg     config.OllamaConfig
	gen     config.GenerationConfig
}

// NewLlamaCpp 构造 llama.cpp 客户端。baseURL 自动补 /v1。
func NewLlamaCpp(cfg config.OllamaConfig, gen config.GenerationConfig) *LlamaCppClient {
	root := strings.TrimRight(cfg.BaseURL, "/")
	// 去掉已有的 /v1 后缀得到 rootURL
	root = strings.TrimSuffix(root, "/v1")
	root = strings.TrimSuffix(root, "/")
	base := root + "/v1"
	return &LlamaCppClient{
		baseURL: base,
		rootURL: root,
		http:    &http.Client{Timeout: cfg.RequestTimeout()},
		cfg:     cfg,
		gen:     gen,
	}
}

func (c *LlamaCppClient) BaseURL() string        { return c.baseURL }
func (c *LlamaCppClient) ChatModel() string      { return c.cfg.ChatModel }
func (c *LlamaCppClient) EmbeddingModel() string { return c.cfg.EmbeddingModel }

// openaiChatRequest 是 OpenAI /v1/chat/completions 请求体。
type openaiChatRequest struct {
	Model         string    `json:"model"`
	Messages      []Message `json:"messages"`
	Stream        bool      `json:"stream"`
	Temperature   float64   `json:"temperature,omitempty"`
	TopP          float64   `json:"top_p,omitempty"`
	TopK          int       `json:"top_k,omitempty"`
	MaxTokens     int       `json:"max_tokens,omitempty"`
	RepeatPenalty float64   `json:"repeat_penalty,omitempty"`
	// stream_options 让 llama-server 在流式响应的最后一帧附带 usage 统计
	StreamOptions *struct {
		IncludeUsage bool `json:"include_usage"`
	} `json:"stream_options,omitempty"`
}

// openaiChatDelta 是 SSE data 行的单帧。
type openaiChatDelta struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
		FinishReason *string `json:"finish_reason"`
	} `json:"choices"`
	Usage *struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
}

// Chat 流式对话（OpenAI SSE 格式）。
func (c *LlamaCppClient) Chat(ctx context.Context, model, systemPrompt string, history []Message) (<-chan ChatEvent, error) {
	var msgs []Message
	if systemPrompt != "" {
		msgs = append(msgs, Message{Role: "system", Content: systemPrompt})
	}
	msgs = append(msgs, history...)

	reqBody := openaiChatRequest{
		Model:         model,
		Messages:      msgs,
		Stream:        true,
		Temperature:   c.gen.Temperature,
		TopP:          c.gen.TopP,
		TopK:          c.gen.TopK,
		MaxTokens:     c.gen.MaxOutputTokens,
		RepeatPenalty: c.gen.RepeatPenalty,
		StreamOptions: &struct {
			IncludeUsage bool `json:"include_usage"`
		}{IncludeUsage: true}, // 让最后一帧带 token 统计
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("[llamacpp] 序列化失败: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")

	// 流式请求不用整体超时
	noTimeoutClient := &http.Client{Transport: c.http.Transport}
	resp, err := noTimeoutClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("[llamacpp] 请求失败（确认 llama-server 已运行）: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("[llamacpp] 状态 %d", resp.StatusCode)
	}

	events := make(chan ChatEvent, 32)
	go c.streamReadSSE(ctx, resp.Body, events)
	return events, nil
}

// streamReadSSE 解析 OpenAI SSE 格式：每行 data: {json}\n\n，结尾 data: [DONE]。
func (c *LlamaCppClient) streamReadSSE(ctx context.Context, body io.ReadCloser, events chan<- ChatEvent) {
	defer func() { _ = body.Close(); close(events) }()
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var promptTokens, completionTokens int

	for scanner.Scan() {
		if ctx.Err() != nil {
			events <- ChatEvent{Error: ctx.Err()}
			return
		}
		line := strings.TrimSpace(scanner.Text())
		if line == "" || !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			events <- ChatEvent{Done: true, PromptTokens: promptTokens, CompletionTokens: completionTokens}
			return
		}
		var delta openaiChatDelta
		if err := json.Unmarshal([]byte(data), &delta); err != nil {
			// 解析失败：可能是服务端错误对象，检查是否含 error 字段
			var errObj map[string]any
			if json.Unmarshal([]byte(data), &errObj) == nil {
				if errMsg, ok := errObj["error"]; ok {
					events <- ChatEvent{Error: fmt.Errorf("[llamacpp] 服务端错误: %v", errMsg)}
					return
				}
			}
			continue // 跳过其他无法解析的行
		}
		if delta.Usage != nil {
			promptTokens = delta.Usage.PromptTokens
			completionTokens = delta.Usage.CompletionTokens
		}
		if len(delta.Choices) > 0 {
			content := delta.Choices[0].Delta.Content
			if content != "" {
				events <- ChatEvent{Token: content}
			}
			if delta.Choices[0].FinishReason != nil {
				events <- ChatEvent{Done: true, PromptTokens: promptTokens, CompletionTokens: completionTokens}
				return
			}
		}
	}
	if err := scanner.Err(); err != nil && ctx.Err() == nil {
		events <- ChatEvent{Error: fmt.Errorf("[llamacpp] 读取流失败: %w", err)}
	}
}

// Embed 批量生成向量（OpenAI /v1/embeddings 格式）。
func (c *LlamaCppClient) Embed(ctx context.Context, texts []string, batchSize int) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}
	if batchSize <= 0 {
		batchSize = 8
	}
	ctx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	var all [][]float32
	for start := 0; start < len(texts); start += batchSize {
		end := start + batchSize
		if end > len(texts) {
			end = len(texts)
		}
		// llama-server embeddings 支持单条 input（字符串）或批量
		reqBody := map[string]any{
			"model": c.cfg.EmbeddingModel,
			"input": texts[start:end],
		}
		body, _ := json.Marshal(reqBody)
		req, _ := http.NewRequestWithContext(ctx, http.MethodPost,
			c.baseURL+"/embeddings", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		client := &http.Client{Transport: c.http.Transport}
		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("[llamacpp] embed 失败: %w", err)
		}
		if resp.StatusCode != http.StatusOK {
			_ = resp.Body.Close()
			return nil, fmt.Errorf("[llamacpp] embed 状态 %d", resp.StatusCode)
		}
		// OpenAI 格式：{data: [{embedding: [...]}]}
		var result struct {
			Data []struct {
				Embedding []float32 `json:"embedding"`
			} `json:"data"`
		}
		if err := json.NewDecoder(io.LimitReader(resp.Body, 64<<20)).Decode(&result); err != nil {
			_ = resp.Body.Close()
			return nil, fmt.Errorf("[llamacpp] 解析 embed 失败: %w", err)
		}
		_ = resp.Body.Close()
		for _, d := range result.Data {
			all = append(all, d.Embedding)
		}
	}
	return all, nil
}

// Health 健康检查。llama-server 的 /health 返回 status，/v1/models 返回已加载模型。
func (c *LlamaCppClient) Health(ctx context.Context) HealthStatus {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	// 1. 探活 /health（在 rootURL，不含 /v1）
	healthReq, _ := http.NewRequestWithContext(ctx, http.MethodGet, c.rootURL+"/health", nil)
	client := &http.Client{Transport: c.http.Transport}
	healthResp, err := client.Do(healthReq)
	if err != nil {
		return HealthStatus{Reachable: false}
	}
	_ = healthResp.Body.Close()
	if healthResp.StatusCode != http.StatusOK {
		return HealthStatus{Reachable: false}
	}

	// 2. 查询已加载模型（/v1/models）
	modelsReq, _ := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/models", nil)
	modelsResp, err := client.Do(modelsReq)
	if err != nil {
		return HealthStatus{Reachable: true, Version: "llama-server"}
	}
	defer modelsResp.Body.Close()
	var modelsBody struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	models := []string{}
	if json.NewDecoder(io.LimitReader(modelsResp.Body, 1<<20)).Decode(&modelsBody) == nil {
		for _, m := range modelsBody.Data {
			models = append(models, m.ID)
		}
	}

	// llama-server 一次只加载一个模型，通过 /v1/models 非空确认模型已加载
	hasModel := len(models) > 0

	return HealthStatus{
		Reachable: true,
		HasModel:  hasModel,
		Models:    models,
		Version:   "llama-server",
	}
}
