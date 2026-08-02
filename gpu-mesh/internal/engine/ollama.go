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

// OllamaEngine 封装对本地 Ollama 守护进程的调用。
//
// Ollama 默认监听 127.0.0.1:11434，提供：
//   - GET  /api/tags     列出已下载模型
//   - POST /api/chat     对话（非流式）
//   - POST /api/embed    嵌入
//   - POST /api/pull     拉取模型
type OllamaEngine struct {
	BaseURL     string
	HTTP        *http.Client       // 推理用（长超时）
	probeClient *http.Client       // 探测用（短超时 2s）
}

// NewOllama 构造默认 Ollama 引擎。
// probeClient 用于 Probe/ListModels（短超时 2s，快速失败）；
// HTTP 用于推理（长超时，推理可能慢）。两个 client 分离避免探测卡顿。
func NewOllama() *OllamaEngine {
	return &OllamaEngine{
		BaseURL:     "http://127.0.0.1:11434",
		HTTP:        &http.Client{Timeout: 120 * time.Second}, // 推理可能慢，给足超时
		probeClient: &http.Client{Timeout: 2 * time.Second},   // 探测快速失败
	}
}

func (e *OllamaEngine) Name() string { return "ollama" }

// Probe 探测：CLI 在 PATH 且 /api/tags 可达（用 probeClient 短超时快速失败）。
func (e *OllamaEngine) Probe(ctx context.Context) bool {
	if _, err := exec.LookPath("ollama"); err != nil {
		return false
	}
	client := e.probeClient
	if client == nil {
		client = e.HTTP
	}
	req, err := http.NewRequestWithContext(ctx, "GET", e.BaseURL+"/api/tags", nil)
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

// --- 模型管理 ---

type ollamaTagsResp struct {
	Models []struct {
		Name    string `json:"name"`
		Size    int64  `json:"size"`
		Details struct {
			ParameterSize    string `json:"parameter_size"`
			QuantLevel       string `json:"quantization_level"`
		} `json:"details"`
	} `json:"models"`
}

func (e *OllamaEngine) ListModels(ctx context.Context) ([]ModelInfo, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", e.BaseURL+"/api/tags", nil)
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
		return nil, fmt.Errorf("ollama /api/tags 返回 %d", resp.StatusCode)
	}
	var tags ollamaTagsResp
	if err := json.NewDecoder(resp.Body).Decode(&tags); err != nil {
		return nil, err
	}
	out := make([]ModelInfo, 0, len(tags.Models))
	for _, m := range tags.Models {
		out = append(out, ModelInfo{Name: m.Name, Size: m.Size})
	}
	return out, nil
}

// ollamaChatReq /api/chat 请求体。
type ollamaChatReq struct {
	Model    string              `json:"model"`
	Messages []proto.ChatMessage `json:"messages"`
	Stream   bool                `json:"stream"`
	Options  *proto.GenOptions   `json:"options,omitempty"`
}

// ollamaChatResp /api/chat 响应体。
type ollamaChatResp struct {
	Model     string `json:"model"`
	Message   proto.ChatMessage `json:"message"`
	Done      bool   `json:"done"`
	DoneReason string `json:"done_reason"`
	PromptEvalCount int `json:"prompt_eval_count"`
	EvalCount       int `json:"eval_count"`
}

// Chat 非流式对话。
func (e *OllamaEngine) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	body := ollamaChatReq{
		Model:    req.Model,
		Messages: req.Messages,
		Stream:   false,
		Options:  req.Options,
	}
	if req.Options == nil {
		body.Options = &proto.GenOptions{}
	}
	if req.MaxTokens > 0 {
		body.Options.NumPredict = req.MaxTokens
	}
	payload, _ := json.Marshal(body)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", e.BaseURL+"/api/chat", bytes.NewReader(payload))
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
		var errResp struct{ Error string `json:"error"` }
		json.NewDecoder(resp.Body).Decode(&errResp)
		return nil, fmt.Errorf("ollama /api/chat 返回 %d: %s", resp.StatusCode, errResp.Error)
	}
	var cr ollamaChatResp
	if err := json.NewDecoder(resp.Body).Decode(&cr); err != nil {
		return nil, err
	}
	return &ChatResponse{
		Content:          cr.Message.Content,
		Model:            cr.Model,
		DoneReason:       cr.DoneReason,
		PromptTokens:     cr.PromptEvalCount,
		CompletionTokens: cr.EvalCount,
	}, nil
}

// ChatStream 流式对话：逐 token 回调（delta 为增量文本）。
//
// Ollama /api/chat stream=true 返回 NDJSON，每行一个 chunk：
//   {"message":{"content":"你"},"done":false}
//   {"message":{"content":"好"},"done":false}
//   {"done":true,"done_reason":"stop",...}
func (e *OllamaEngine) ChatStream(ctx context.Context, req ChatRequest, onDelta func(delta string)) (*ChatResponse, error) {
	body := ollamaChatReq{
		Model: req.Model, Messages: req.Messages, Stream: true, Options: req.Options,
	}
	if body.Options == nil {
		body.Options = &proto.GenOptions{}
	}
	if req.MaxTokens > 0 {
		body.Options.NumPredict = req.MaxTokens
	}
	payload, _ := json.Marshal(body)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", e.BaseURL+"/api/chat", bytes.NewReader(payload))
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
		return nil, fmt.Errorf("ollama /api/chat stream 返回 %d", resp.StatusCode)
	}
	// 流式读 NDJSON
	decoder := json.NewDecoder(resp.Body)
	out := &ChatResponse{Model: req.Model}
	for decoder.More() {
		var chunk ollamaChatResp
		if err := decoder.Decode(&chunk); err != nil {
			break
		}
		if chunk.Message.Content != "" {
			onDelta(chunk.Message.Content)
			out.Content += chunk.Message.Content
		}
		if chunk.Done {
			out.DoneReason = chunk.DoneReason
			out.PromptTokens = chunk.PromptEvalCount
			out.CompletionTokens = chunk.EvalCount
			break
		}
	}
	return out, nil
}

// ollamaEmbedReq /api/embed 请求体。
type ollamaEmbedReq struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

// ollamaEmbedResp /api/embed 响应体。
type ollamaEmbedResp struct {
	Model      string      `json:"model"`
	Embeddings [][]float32 `json:"embeddings"`
}

// Embed 批量文本嵌入。
func (e *OllamaEngine) Embed(ctx context.Context, req EmbedRequest) (*EmbedResponse, error) {
	body := ollamaEmbedReq{Model: req.Model, Input: req.Input}
	payload, _ := json.Marshal(body)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", e.BaseURL+"/api/embed", bytes.NewReader(payload))
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
		return nil, fmt.Errorf("ollama /api/embed 返回 %d", resp.StatusCode)
	}
	var er ollamaEmbedResp
	if err := json.NewDecoder(resp.Body).Decode(&er); err != nil {
		return nil, err
	}
	return &EmbedResponse{Embeddings: er.Embeddings, Model: er.Model}, nil
}

// ollamaPullReq /api/pull 请求体。
type ollamaPullReq struct {
	Name   string `json:"name"`
	Stream bool   `json:"stream"`
}

// Pull 拉取模型（阻塞直到完成）。Ollama 的 /api/pull 是流式 NDJSON，这里读到最后一条。
func (e *OllamaEngine) Pull(ctx context.Context, model string) error {
	body := ollamaPullReq{Name: model, Stream: false}
	payload, _ := json.Marshal(body)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", e.BaseURL+"/api/pull", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := e.HTTP.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ollama /api/pull 返回 %d", resp.StatusCode)
	}
	// 非流式模式 Ollama 会在完成后返回最终 JSON，直接丢弃 body 即可
	return nil
}
