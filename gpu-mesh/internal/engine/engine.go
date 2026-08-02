// Package engine 抽象本地 LLM 推理引擎，使 Ollama 与 llama.cpp 可互换。
//
// 设计目标：
//   - Agent 启动时探测本机可用引擎，上报给 Relay（Phase 1）
//   - Relay 据此做模型路由（Phase 2/3）
//   - Agent 执行器调 Engine.Chat/Embed/Pull 执行实际推理（Phase 2+）
package engine

import (
	"context"
	"errors"

	"github.com/QiuShichang/gpu-mesh/internal/proto"
)

// ErrUnsupported 操作不被当前引擎支持（如 llama.cpp 无法 pull）。
var ErrUnsupported = errors.New("operation unsupported by engine")

// Engine 本地推理引擎抽象。
//
// 实现方：OllamaEngine（调 :11434）、LlamaCppEngine（调 llama-server HTTP）。
// 两者都暴露 OpenAI 兼容的 /v1/chat/completions，但管理 API 不同：
//   - Ollama: /api/tags 列模型、/api/pull 拉模型、/api/generate 生成
//   - llama.cpp: 无标准模型管理，靠启动参数指定模型
type Engine interface {
	// Name 引擎名 "ollama" / "llamacpp"。
	Name() string
	// Probe 探测本机引擎是否可用（检测进程/端口/CLI）。
	Probe(ctx context.Context) bool
	// ListModels 列出本机已下载/已加载的模型。
	ListModels(ctx context.Context) ([]ModelInfo, error)
	// Chat 非流式对话推理。
	Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error)
	// ChatStream 流式对话推理，每个 token 增量回调 onDelta。
	ChatStream(ctx context.Context, req ChatRequest, onDelta func(delta string)) (*ChatResponse, error)
	// Embed 批量文本嵌入。
	Embed(ctx context.Context, req EmbedRequest) (*EmbedResponse, error)
	// Pull 拉取/加载模型（Ollama 从仓库拉；llama.cpp 从本地 GGUF 加载）。
	Pull(ctx context.Context, model string) error
}

// ModelInfo 模型元信息。
type ModelInfo struct {
	Name string `json:"name"`       // 模型标识
	Size int64  `json:"size_bytes"` // 模型文件大小（字节），0 表示未知
}

// ChatRequest 对话推理请求（Agent 执行器构造）。
type ChatRequest struct {
	Model    string
	Messages []proto.ChatMessage
	Options  *proto.GenOptions
	MaxTokens int
}

// ChatResponse 对话推理响应。
type ChatResponse struct {
	Content          string
	Model            string
	DoneReason       string
	PromptTokens     int
	CompletionTokens int
}

// EmbedRequest 嵌入请求。
type EmbedRequest struct {
	Model string
	Input []string
}

// EmbedResponse 嵌入响应。
type EmbedResponse struct {
	Embeddings [][]float32
	Model      string
}

// ProbeAll 探测所有已知引擎，返回当前可用的引擎实例与各自模型。
//
// 返回引擎实例列表（而非仅名字），供 Agent 执行器按名查找复用。
func ProbeAll(ctx context.Context) (engines []Engine, allModels []ModelInfo) {
	candidates := []Engine{
		NewOllama(),
		NewLlamaCpp(),
	}
	type result struct {
		e      Engine
		ok     bool
		models []ModelInfo
	}
	ch := make(chan result, len(candidates))
	for _, e := range candidates {
		go func(e Engine) {
			ok := e.Probe(ctx)
			var models []ModelInfo
			if ok {
				if m, err := e.ListModels(ctx); err == nil {
					models = m
				}
			}
			ch <- result{e, ok, models}
		}(e)
	}
	for range candidates {
		r := <-ch
		if r.ok {
			engines = append(engines, r.e)
			allModels = append(allModels, r.models...)
		}
	}
	return engines, allModels
}

// EngineNames 从引擎列表提取名字。
func EngineNames(engines []Engine) []string {
	out := make([]string, 0, len(engines))
	for _, e := range engines {
		out = append(out, e.Name())
	}
	return out
}

// ModelNames 从 ModelInfo 列表提取名字。
func ModelNames(ms []ModelInfo) []string {
	out := make([]string, 0, len(ms))
	for _, m := range ms {
		out = append(out, m.Name)
	}
	return out
}

// Find 按名查找引擎。
func Find(engines []Engine, name string) Engine {
	for _, e := range engines {
		if e.Name() == name {
			return e
		}
	}
	return nil
}
