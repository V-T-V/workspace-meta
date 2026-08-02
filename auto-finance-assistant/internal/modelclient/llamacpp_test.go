package modelclient

import (
	"testing"

	"github.com/QiuShichang/auto-finance-assistant/internal/config"
)

func TestNewLlamaCppUsesSeparateEmbeddingEndpoint(t *testing.T) {
	client := NewLlamaCpp(config.OllamaConfig{
		BaseURL:          "http://127.0.0.1:8081/v1",
		EmbeddingBaseURL: "http://127.0.0.1:8082/v1",
	}, config.GenerationConfig{})

	if got, want := client.baseURL, "http://127.0.0.1:8081/v1"; got != want {
		t.Fatalf("chat base URL = %q, want %q", got, want)
	}
	if got, want := client.embeddingBaseURL, "http://127.0.0.1:8082/v1"; got != want {
		t.Fatalf("embedding base URL = %q, want %q", got, want)
	}
}

func TestNewLlamaCppReusesChatEndpointWithoutEmbeddingURL(t *testing.T) {
	client := NewLlamaCpp(config.OllamaConfig{BaseURL: "http://127.0.0.1:8081"}, config.GenerationConfig{})

	if got, want := client.embeddingBaseURL, "http://127.0.0.1:8081/v1"; got != want {
		t.Fatalf("embedding base URL = %q, want %q", got, want)
	}
}
