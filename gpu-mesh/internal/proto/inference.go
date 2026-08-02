package proto

import "encoding/json"

// === Phase 2：推理 / 嵌入 / 模型管理载荷 ===

// ChatMessage OpenAI 兼容的对话消息。
type ChatMessage struct {
	Role    string `json:"role"`             // system / user / assistant
	Content string `json:"content"`
}

// InferenceTask 推理任务载荷（Agent 侧执行器消费）。
type InferenceTask struct {
	Engine   string        `json:"engine,omitempty"`  // 指定引擎 "ollama"/"llamacpp"，空则用首个可用
	Model    string        `json:"model"`             // 模型名，如 "qwen2.5:7b"
	Messages []ChatMessage `json:"messages"`          // 对话上下文
	Stream   bool          `json:"stream,omitempty"`  // 是否流式（Phase 2 先非流式）
	Options  *GenOptions   `json:"options,omitempty"` // 采样参数
	// MaxTokens 限制（兼容 OpenAI max_tokens 与 Ollama num_predict）。
	MaxTokens int `json:"max_tokens,omitempty"`
}

// GenOptions 采样参数（兼容 Ollama options / llama.cpp）。
type GenOptions struct {
	Temperature float64 `json:"temperature,omitempty"`
	TopP        float64 `json:"top_p,omitempty"`
	TopK        int     `json:"top_k,omitempty"`
	NumPredict  int     `json:"num_predict,omitempty"` // Ollama: 最大生成 token 数
	Seed        int     `json:"seed,omitempty"`
}

// InferenceResult 推理任务结果（放在 TaskResult.Data 里）。
type InferenceResult struct {
	Content      string `json:"content"`             // 生成文本
	Model        string `json:"model"`                // 实际使用的模型
	DoneReason   string `json:"done_reason,omitempty"` // stop / length
	PromptTokens int    `json:"prompt_tokens,omitempty"`
	CompletionTokens int `json:"completion_tokens,omitempty"`
}

// EmbedTask 嵌入任务载荷。
type EmbedTask struct {
	Engine string   `json:"engine,omitempty"`
	Model  string   `json:"model"` // 如 "nomic-embed-text"
	Input  []string `json:"input"` // 待向量化文本（支持批量）
}

// EmbedResult 嵌入结果。
type EmbedResult struct {
	Embeddings [][]float32 `json:"embeddings"` // 每条文本一个向量
	Model      string      `json:"model"`
}

// PullTask 拉取/加载模型任务（模型管理）。
type PullTask struct {
	Engine string `json:"engine"` // "ollama" / "llamacpp"
	Model  string `json:"model"`  // 模型名
	// llama.cpp 场景：模型文件路径（GGUF），需 Agent 启动 llama-server 加载
	ModelPath string `json:"model_path,omitempty"`
}

// PullProgress 拉模型进度。
type PullProgress struct {
	Model   string `json:"model"`
	Percent int    `json:"percent"`
	Status  string `json:"status"` // pulling / extracting / success / error
}

// --- OpenAI 兼容请求/响应 schema（Relay 网关对外用）---

// OpenAIChatRequest POST /v1/chat/completions 请求体（OpenAI 兼容）。
type OpenAIChatRequest struct {
	Model    string        `json:"model"`
	Messages []ChatMessage `json:"messages"`
	Stream   bool          `json:"stream,omitempty"`
	Temperature float64    `json:"temperature,omitempty"`
	MaxTokens  int         `json:"max_tokens,omitempty"`
	// 可选：强制指定引擎与目标 Agent（调试用）
	Engine  string `json:"x_engine,omitempty"`
	AgentID string `json:"x_agent_id,omitempty"`
}

// OpenAIChoice OpenAI 响应里的一个选项。
type OpenAIChoice struct {
	Index   int         `json:"index"`
	Message ChatMessage `json:"message"`
	FinishReason string  `json:"finish_reason"`
}

// OpenAIChatResponse POST /v1/chat/completions 响应体（OpenAI 兼容）。
type OpenAIChatResponse struct {
	ID      string         `json:"id"`
	Object  string         `json:"object"` // "chat.completion"
	Created int64          `json:"created"`
	Model   string         `json:"model"`
	Choices []OpenAIChoice `json:"choices"`
	Usage   *OpenAIUsage   `json:"usage,omitempty"`
}

// OpenAIUsage token 用量。
type OpenAIUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// OpenAIEmbedRequest POST /v1/embeddings 请求体。
type OpenAIEmbedRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
	Engine string  `json:"x_engine,omitempty"`
	AgentID string `json:"x_agent_id,omitempty"`
}

// OpenAIEmbedResponse POST /v1/embeddings 响应体。
type OpenAIEmbedResponse struct {
	Object string            `json:"object"` // "list"
	Data   []OpenAIEmbedItem `json:"data"`
	Model  string            `json:"model"`
}

// OpenAIEmbedItem 单条嵌入结果。
type OpenAIEmbedItem struct {
	Object    string    `json:"object"` // "embedding"
	Index     int       `json:"index"`
	Embedding []float32 `json:"embedding"`
}

// MarshalData 把任意结构序列化为 TaskResult.Data（json.RawMessage）。
func MarshalData(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}
