package modelclient

import (
	"log/slog"

	"github.com/QiuShichang/auto-finance-assistant/internal/config"
)

// New 根据 config 的 backend 字段返回对应的 ModelClient 实现。
func New(cfg config.OllamaConfig, gen config.GenerationConfig) ModelClient {
	switch cfg.Backend {
	case "llamacpp":
		return NewLlamaCpp(cfg, gen)
	case "", "ollama":
		return NewOllama(cfg, gen)
	default:
		slog.Warn("[modelclient] 未知 backend，回退到 ollama", "backend", cfg.Backend)
		return NewOllama(cfg, gen)
	}
}
