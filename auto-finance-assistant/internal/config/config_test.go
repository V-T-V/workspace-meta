package config

import (
	"os"
	"path/filepath"
	"testing"
)

// TestDefaults 验证默认配置可通过 Validate。
func TestDefaults(t *testing.T) {
	cfg := Defaults()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("默认配置校验失败: %v", err)
	}
	if cfg.Server.Port != 8080 {
		t.Errorf("默认端口应为 8080，实际 %d", cfg.Server.Port)
	}
	if cfg.Queue.GenerationConcurrency != 1 {
		t.Errorf("默认生成并发应为 1（单 GPU/CPU），实际 %d", cfg.Queue.GenerationConcurrency)
	}
}

// TestLoad_ValidFile 验证 YAML 加载与字段映射。
func TestLoad_ValidFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := `
server:
  host: 0.0.0.0
  port: 9090
ollama:
  base_url: http://localhost:11434
  chat_model: qwen3.5:4b
generation:
  max_output_tokens: 800
queue:
  generation_concurrency: 1
rag:
  minimum_confidence: 0.6
  high_confidence: 0.8
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("加载失败: %v", err)
	}
	if cfg.Server.Host != "0.0.0.0" {
		t.Errorf("host 应为 0.0.0.0，实际 %s", cfg.Server.Host)
	}
	if cfg.Server.Port != 9090 {
		t.Errorf("port 应为 9090，实际 %d", cfg.Server.Port)
	}
	if cfg.Ollama.ChatModel != "qwen3.5:4b" {
		t.Errorf("chat_model 应为 qwen3.5:4b，实际 %s", cfg.Ollama.ChatModel)
	}
	if cfg.Generation.MaxOutputTokens != 800 {
		t.Errorf("max_output_tokens 应为 800，实际 %d", cfg.Generation.MaxOutputTokens)
	}
}

// TestLoad_InvalidPort 验证非法端口被拒绝。
func TestLoad_InvalidPort(t *testing.T) {
	cfg := Defaults()
	cfg.Server.Port = 0
	if err := cfg.Validate(); err == nil {
		t.Fatal("非法端口 0 应校验失败")
	}
}

// TestLoad_MissingModel 验证空模型名被拒绝。
func TestLoad_MissingModel(t *testing.T) {
	cfg := Defaults()
	cfg.Ollama.ChatModel = ""
	if err := cfg.Validate(); err == nil {
		t.Fatal("空 chat_model 应校验失败")
	}
}

// TestLoad_ConfidenceOrder 验证置信度阈值顺序。
func TestLoad_ConfidenceOrder(t *testing.T) {
	cfg := Defaults()
	cfg.RAG.MinimumConfidence = 0.8
	cfg.RAG.HighConfidence = 0.58
	if err := cfg.Validate(); err == nil {
		t.Fatal("minimum >= high 应校验失败")
	}
}

// TestLoad_Nonexistent 验证不存在文件降级为默认配置（不崩溃）。
func TestLoad_Nonexistent(t *testing.T) {
	cfg, err := Load("/nonexistent/path/config.yaml")
	if err != nil {
		t.Fatalf("不存在的文件应降级为默认配置，不应报错: %v", err)
	}
	if cfg == nil || cfg.Server.Port != 8080 {
		t.Error("应返回有效的默认配置")
	}
}

// TestAddr 验证监听地址拼接。
func TestAddr(t *testing.T) {
	cfg := Defaults()
	cfg.Server.Host = "0.0.0.0"
	cfg.Server.Port = 3000
	if got := cfg.Addr(); got != "0.0.0.0:3000" {
		t.Errorf("Addr 应为 0.0.0.0:3000，实际 %s", got)
	}
}

// TestRequestTimeout 验证超时转换。
func TestRequestTimeout(t *testing.T) {
	cfg := Defaults()
	cfg.Ollama.RequestTimeoutSeconds = 90
	if d := cfg.Ollama.RequestTimeout(); d.Seconds() != 90 {
		t.Errorf("RequestTimeout 应为 90s，实际 %v", d)
	}
}
