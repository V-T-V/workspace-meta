package engine

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"time"

	"github.com/QiuShichang/gpu-mesh/internal/proto"
)

// LlamaCppEngine 封装对本地 llama.cpp 的 llama-server 调用。
//
// llama-server 默认监听 127.0.0.1:8080，提供 OpenAI 兼容端点：
//   - GET  /health              健康检查
//   - GET  /v1/models           列出已加载模型
//   - POST /v1/chat/completions 对话（OpenAI 格式）
//   - POST /v1/embeddings       嵌入（OpenAI 格式）
//
// 注意：llama.cpp 无标准模型管理（不像 Ollama 能 pull），模型靠 llama-server 启动参数 -m 指定。
// 因此 Pull 操作对 llama.cpp 不支持（返回 ErrUnsupported），需通过部署脚本预设模型。
type LlamaCppEngine struct {
	BaseURL     string
	HTTP        *http.Client       // 推理用（长超时）
	probeClient *http.Client       // 探测用（短超时 2s）
}

// NewLlamaCpp 构造默认 llama.cpp 引擎。
func NewLlamaCpp() *LlamaCppEngine {
	return &LlamaCppEngine{
		BaseURL:     "http://127.0.0.1:8080",
		HTTP:        &http.Client{Timeout: 120 * time.Second},
		probeClient: &http.Client{Timeout: 2 * time.Second},
	}
}

func (e *LlamaCppEngine) Name() string { return "llamacpp" }

// Probe 探测：CLI 在 PATH 且 /health 可达（用 probeClient 短超时快速失败）。
func (e *LlamaCppEngine) Probe(ctx context.Context) bool {
	found := false
	for _, name := range []string{"llama-server", "llamacli", "main"} {
		if _, err := exec.LookPath(name); err == nil {
			found = true
			break
		}
	}
	if !found {
		return false
	}
	client := e.probeClient
	if client == nil {
		client = e.HTTP
	}
	req, err := http.NewRequestWithContext(ctx, "GET", e.BaseURL+"/health", nil)
	if err != nil {
		return false
	}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// llamacppModelsResp /v1/models 响应（OpenAI 格式）。
type llamacppModelsResp struct {
	Data []struct {
		ID string `json:"id"`
	} `json:"data"`
}

func (e *LlamaCppEngine) ListModels(ctx context.Context) ([]ModelInfo, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", e.BaseURL+"/v1/models", nil)
	if err != nil {
		return nil, err
	}
	client := e.probeClient
	if client == nil {
		client = e.HTTP
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("llamacpp /v1/models 返回 %d", resp.StatusCode)
	}
	var mr llamacppModelsResp
	if err := json.NewDecoder(resp.Body).Decode(&mr); err != nil {
		return nil, err
	}
	out := make([]ModelInfo, 0, len(mr.Data))
	for _, m := range mr.Data {
		out = append(out, ModelInfo{Name: m.ID})
	}
	return out, nil
}

// llamacppChatReq OpenAI 兼容 chat 请求体。
type llamacppChatReq struct {
	Model       string              `json:"model"`
	Messages    []proto.ChatMessage `json:"messages"`
	Stream      bool                `json:"stream"`
	Temperature float64             `json:"temperature,omitempty"`
	MaxTokens   int                 `json:"max_tokens,omitempty"`
}

// llamacppChatResp OpenAI 兼容 chat 响应体。
type llamacppChatResp struct {
	Model   string `json:"model"`
	Choices []struct {
		Message      proto.ChatMessage `json:"message"`
		FinishReason string            `json:"finish_reason"`
	} `json:"choices"`
	Usage *struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
}

// Chat 非流式对话（走 OpenAI 兼容端点）。
func (e *LlamaCppEngine) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	body := llamacppChatReq{
		Model:    req.Model,
		Messages: req.Messages,
		Stream:   false,
		MaxTokens: req.MaxTokens,
	}
	if req.Options != nil {
		body.Temperature = req.Options.Temperature
	}
	payload, _ := json.Marshal(body)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", e.BaseURL+"/v1/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := e.HTTP.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("llamacpp /v1/chat 返回 %d", resp.StatusCode)
	}
	var cr llamacppChatResp
	if err := json.NewDecoder(resp.Body).Decode(&cr); err != nil {
		return nil, err
	}
	out := &ChatResponse{Model: cr.Model}
	if len(cr.Choices) > 0 {
		out.Content = cr.Choices[0].Message.Content
		out.DoneReason = cr.Choices[0].FinishReason
	}
	if cr.Usage != nil {
		out.PromptTokens = cr.Usage.PromptTokens
		out.CompletionTokens = cr.Usage.CompletionTokens
	}
	return out, nil
}

// ChatStream 流式对话。
//
// llama.cpp 支持原生 SSE 流（stream=true），但解析较繁。
// 此处简化：用非流式拿到完整结果后整体回调一次（保证接口一致 + 网关 SSE 仍可工作）。
// 未来可改为读 SSE event 流逐 token 回调。
func (e *LlamaCppEngine) ChatStream(ctx context.Context, req ChatRequest, onDelta func(delta string)) (*ChatResponse, error) {
	resp, err := e.Chat(ctx, req)
	if err != nil {
		return nil, err
	}
	if resp.Content != "" {
		onDelta(resp.Content)
	}
	return resp, nil
}

// llamacppEmbedResp OpenAI 兼容 embeddings 响应。
type llamacppEmbedResp struct {
	Data []struct {
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
	Model string `json:"model"`
}

// Embed 批量嵌入（OpenAI 兼容端点）。
func (e *LlamaCppEngine) Embed(ctx context.Context, req EmbedRequest) (*EmbedResponse, error) {
	// OpenAI embeddings API 单条用 input 字符串，批量时用数组
	body := map[string]any{"model": req.Model, "input": req.Input}
	payload, _ := json.Marshal(body)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", e.BaseURL+"/v1/embeddings", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := e.HTTP.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("llamacpp /v1/embeddings 返回 %d", resp.StatusCode)
	}
	var er llamacppEmbedResp
	if err := json.NewDecoder(resp.Body).Decode(&er); err != nil {
		return nil, err
	}
	embeddings := make([][]float32, 0, len(er.Data))
	for _, d := range er.Data {
		embeddings = append(embeddings, d.Embedding)
	}
	return &EmbedResponse{Embeddings: embeddings, Model: er.Model}, nil
}

// Pull llama.cpp 不支持远程拉取模型（需本地 GGUF 文件）。
func (e *LlamaCppEngine) Pull(ctx context.Context, model string) error {
	return ErrUnsupported
}
