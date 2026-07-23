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

// OllamaClient 实现 ModelClient 接口，调用 Ollama REST API。
type OllamaClient struct {
	baseURL string
	http    *http.Client
	cfg     config.OllamaConfig
	gen     config.GenerationConfig
}

// NewOllama 构造 Ollama 客户端。
func NewOllama(cfg config.OllamaConfig, gen config.GenerationConfig) *OllamaClient {
	return &OllamaClient{
		baseURL: cfg.BaseURL,
		http:    &http.Client{Timeout: cfg.RequestTimeout()},
		cfg:     cfg,
		gen:     gen,
	}
}

func (c *OllamaClient) BaseURL() string        { return c.baseURL }
func (c *OllamaClient) ChatModel() string      { return c.cfg.ChatModel }
func (c *OllamaClient) EmbeddingModel() string { return c.cfg.EmbeddingModel }

// ollamaOptions 映射 Ollama 生成参数。
type ollamaOptions struct {
	NumGPU        int     `json:"num_gpu,omitempty"`
	NumThread     int     `json:"num_thread,omitempty"`
	NumCtx        int     `json:"num_ctx,omitempty"`
	Temperature   float64 `json:"temperature,omitempty"`
	TopP          float64 `json:"top_p,omitempty"`
	TopK          int     `json:"top_k,omitempty"`
	RepeatPenalty float64 `json:"repeat_penalty,omitempty"`
	NumPredict    int     `json:"num_predict,omitempty"`
}

type ollamaChatRequest struct {
	Model    string          `json:"model"`
	Messages []Message       `json:"messages"`
	Stream   bool            `json:"stream"`
	Options  ollamaOptions   `json:"options,omitempty"`
}

type ollamaChatResponse struct {
	Message struct {
		Content string `json:"content"`
	} `json:"message"`
	Done            bool  `json:"done"`
	PromptEvalCount int   `json:"prompt_eval_count,omitempty"`
	EvalCount       int   `json:"eval_count,omitempty"`
}

func (c *OllamaClient) buildOptions() ollamaOptions {
	o := ollamaOptions{
		NumCtx:        c.gen.ContextSize,
		Temperature:   c.gen.Temperature,
		TopP:          c.gen.TopP,
		TopK:          c.gen.TopK,
		RepeatPenalty: c.gen.RepeatPenalty,
		NumPredict:    c.gen.MaxOutputTokens,
	}
	if c.gen.NumGPU > 0 {
		o.NumGPU = c.gen.NumGPU
	}
	if c.gen.NumThread > 0 {
		o.NumThread = c.gen.NumThread
	}
	return o
}

// Chat 流式对话。
func (c *OllamaClient) Chat(ctx context.Context, model, systemPrompt string, history []Message) (<-chan ChatEvent, error) {
	var msgs []Message
	if systemPrompt != "" {
		msgs = append(msgs, Message{Role: "system", Content: systemPrompt})
	}
	msgs = append(msgs, history...)

	reqBody := ollamaChatRequest{
		Model: model, Messages: msgs, Stream: true, Options: c.buildOptions(),
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("[ollama] 序列化失败: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		joinURL(c.baseURL, "/api/chat"), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	noTimeoutClient := &http.Client{Transport: c.http.Transport}
	resp, err := noTimeoutClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("[ollama] 请求失败: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("[ollama] 状态 %d", resp.StatusCode)
	}

	events := make(chan ChatEvent, 32)
	go c.streamRead(ctx, resp.Body, events)
	return events, nil
}

func (c *OllamaClient) streamRead(ctx context.Context, body io.ReadCloser, events chan<- ChatEvent) {
	defer func() { _ = body.Close(); close(events) }()
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		if ctx.Err() != nil {
			events <- ChatEvent{Error: ctx.Err()}
			return
		}
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var frame ollamaChatResponse
		if err := json.Unmarshal([]byte(line), &frame); err != nil {
			events <- ChatEvent{Error: fmt.Errorf("[ollama] 解析失败: %w", err)}
			return
		}
		if frame.Done {
			events <- ChatEvent{Done: true, PromptTokens: frame.PromptEvalCount, CompletionTokens: frame.EvalCount}
			return
		}
		if frame.Message.Content != "" {
			events <- ChatEvent{Token: frame.Message.Content}
		}
	}
	if err := scanner.Err(); err != nil && ctx.Err() == nil {
		events <- ChatEvent{Error: fmt.Errorf("[ollama] 读取流失败: %w", err)}
	}
}

// Embed 批量生成向量。
func (c *OllamaClient) Embed(ctx context.Context, texts []string, batchSize int) ([][]float32, error) {
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
		body, _ := json.Marshal(map[string]any{"model": c.cfg.EmbeddingModel, "input": texts[start:end]})
		req, _ := http.NewRequestWithContext(ctx, http.MethodPost, joinURL(c.baseURL, "/api/embed"), bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		client := &http.Client{Transport: c.http.Transport}
		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("[ollama] embed 失败: %w", err)
		}
		if resp.StatusCode != http.StatusOK {
			_ = resp.Body.Close()
			return nil, fmt.Errorf("[ollama] embed 状态 %d", resp.StatusCode)
		}
		var result struct {
			Embeddings [][]float32 `json:"embeddings"`
		}
		if err := json.NewDecoder(io.LimitReader(resp.Body, 64<<20)).Decode(&result); err != nil {
			_ = resp.Body.Close()
			return nil, fmt.Errorf("[ollama] 解析 embed 失败: %w", err)
		}
		_ = resp.Body.Close()
		all = append(all, result.Embeddings...)
	}
	return all, nil
}

// Health 健康检查。
func (c *OllamaClient) Health(ctx context.Context) HealthStatus {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	version, ok := c.probeVersion(ctx)
	if !ok {
		return HealthStatus{Reachable: false}
	}
	models := c.listModels(ctx)
	hasModel := false
	for _, m := range models {
		if m == c.cfg.ChatModel {
			hasModel = true
			break
		}
	}
	return HealthStatus{Reachable: true, HasModel: hasModel, Models: models, Version: version}
}

func (c *OllamaClient) probeVersion(ctx context.Context) (string, bool) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, joinURL(c.baseURL, "/api/version"), nil)
	if err != nil {
		return "", false
	}
	client := &http.Client{Transport: c.http.Transport}
	resp, err := client.Do(req)
	if err != nil {
		return "", false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", false
	}
	var v struct{ Version string `json:"version"` }
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1024)).Decode(&v); err != nil {
		return "", false
	}
	return v.Version, true
}

func (c *OllamaClient) listModels(ctx context.Context) []string {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, joinURL(c.baseURL, "/api/tags"), nil)
	if err != nil {
		return nil
	}
	client := &http.Client{Transport: c.http.Transport}
	resp, err := client.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil
	}
	var body struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&body); err != nil {
		return nil
	}
	names := make([]string, 0, len(body.Models))
	for _, m := range body.Models {
		names = append(names, m.Name)
	}
	return names
}

func joinURL(base, path string) string {
	return strings.TrimRight(base, "/") + path
}
