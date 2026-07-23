// Package config 实现三层配置加载：默认值 → YAML 文件 → 环境变量/flag 覆盖。
// 对应原计划第八节"配置设计"。
package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config 是运行时配置根。字段命名与 config.yaml 结构一一对应。
type Config struct {
	Server     ServerConfig     `yaml:"server"`
	Ollama     OllamaConfig     `yaml:"ollama"`
	Generation GenerationConfig `yaml:"generation"`
	RAG        RAGConfig        `yaml:"rag"`
	Queue      QueueConfig      `yaml:"queue"`
	Storage    StorageConfig    `yaml:"storage"`
	Documents  DocumentsConfig  `yaml:"documents"`
	Logging    LoggingConfig    `yaml:"logging"`
	Backup     BackupConfig     `yaml:"backup"`
	Security   SecurityConfig   `yaml:"security"`
}

type ServerConfig struct {
	Host               string `yaml:"host"`
	Port               int    `yaml:"port"`
	ReadTimeoutSeconds  int    `yaml:"read_timeout_seconds"`
	WriteTimeoutSeconds int    `yaml:"write_timeout_seconds"`
}

type OllamaConfig struct {
	Backend              string `yaml:"backend"` // ollama | llamacpp（默认 ollama）
	BaseURL              string `yaml:"base_url"`
	ChatModel            string `yaml:"chat_model"`
	EmbeddingModel       string `yaml:"embedding_model"`
	RequestTimeoutSeconds int   `yaml:"request_timeout_seconds"`
	KeepAlive            string `yaml:"keep_alive"`
}

type GenerationConfig struct {
	NumGPU          int     `yaml:"num_gpu"`          // 生产：GPU offload 层数（RTX 4060 用）
	NumThread       int     `yaml:"num_thread"`       // CPU：线程数（开发机用）
	ContextSize     int     `yaml:"context_size"`
	MaxOutputTokens int     `yaml:"max_output_tokens"`
	Temperature     float64 `yaml:"temperature"`
	TopP            float64 `yaml:"top_p"`
	TopK            int     `yaml:"top_k"`
	RepeatPenalty   float64 `yaml:"repeat_penalty"`
}

type RAGConfig struct {
	FTSLimit          int     `yaml:"fts_limit"`
	VectorLimit       int     `yaml:"vector_limit"`
	FinalLimit        int     `yaml:"final_limit"`
	MinimumConfidence float64 `yaml:"minimum_confidence"`
	HighConfidence    float64 `yaml:"high_confidence"`
	VectorWeight      float64 `yaml:"vector_weight"`
	KeywordWeight     float64 `yaml:"keyword_weight"`
}

type QueueConfig struct {
	GenerationConcurrency int `yaml:"generation_concurrency"`
	MaximumWaiting        int `yaml:"maximum_waiting"`
	RequestTimeoutSeconds int `yaml:"request_timeout_seconds"`
}

type StorageConfig struct {
	DatabasePath string `yaml:"database_path"`
	DocumentPath string `yaml:"document_path"`
	TempPath     string `yaml:"temp_path"`
	BackupPath   string `yaml:"backup_path"`
}

type DocumentsConfig struct {
	MaxFileSizeMB     int      `yaml:"max_file_size_mb"`
	ChunkMinChars     int      `yaml:"chunk_min_chars"`
	ChunkMaxChars     int      `yaml:"chunk_max_chars"`
	ChunkOverlapChars int      `yaml:"chunk_overlap_chars"`
	AllowedExtensions []string `yaml:"allowed_extensions"`
}

type LoggingConfig struct {
	Level         string `yaml:"level"`
	Directory     string `yaml:"directory"`
	MaxFileSizeMB int    `yaml:"max_file_size_mb"`
	MaxFiles      int    `yaml:"max_files"`
	RetainDays    int    `yaml:"retain_days"`
}

type BackupConfig struct {
	Enabled     bool   `yaml:"enabled"`
	Schedule    string `yaml:"schedule"`
	RetainCount int    `yaml:"retain_count"`
}

type SecurityConfig struct {
	AdminPassword string `yaml:"admin_password"`
}

// RequestTimeout 返回 Ollama 请求超时。
func (c *OllamaConfig) RequestTimeout() time.Duration {
	return time.Duration(c.RequestTimeoutSeconds) * time.Second
}

// QueueTimeout 返回队列整体超时。
func (c *QueueConfig) QueueTimeout() time.Duration {
	return time.Duration(c.RequestTimeoutSeconds) * time.Second
}

// Load 从 path 读取 YAML 配置，叠加默认值。
// path 为空或文件不存在时返回纯默认配置（不崩溃，便于首次启动）。
func Load(path string) (*Config, error) {
	cfg := Defaults()

	if path != "" {
		raw, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				// 配置文件不存在：用默认配置，不崩溃（首次启动场景）
				return cfg, nil
			}
			return nil, fmt.Errorf("读取配置文件 %s 失败: %w", path, err)
		}
		if err := yaml.Unmarshal(raw, cfg); err != nil {
			return nil, fmt.Errorf("解析配置文件 %s 失败: %w", path, err)
		}
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// WriteDefaults 把默认配置写到 path（供 setup 脚本调用）。
func WriteDefaults(path string) error {
	cfg := Defaults()
	out, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, out, 0o644)
}

// Validate 校验关键字段合法性。失败时返回带模块名的错误，便于定位。
func (c *Config) Validate() error {
	if c.Server.Port < 1 || c.Server.Port > 65535 {
		return fmt.Errorf("[config] server.port 非法: %d", c.Server.Port)
	}
	if c.Ollama.BaseURL == "" {
		return fmt.Errorf("[config] ollama.base_url 不能为空")
	}
	if c.Ollama.ChatModel == "" {
		return fmt.Errorf("[config] ollama.chat_model 不能为空")
	}
	if c.Generation.MaxOutputTokens < 1 {
		return fmt.Errorf("[config] generation.max_output_tokens 必须 > 0")
	}
	if c.Queue.GenerationConcurrency < 1 {
		return fmt.Errorf("[config] queue.generation_concurrency 必须 >= 1（单 GPU/CPU 推荐 1）")
	}
	if c.Storage.DatabasePath == "" {
		return fmt.Errorf("[config] storage.database_path 不能为空")
	}
	if c.RAG.MinimumConfidence >= c.RAG.HighConfidence {
		return fmt.Errorf("[config] rag.minimum_confidence(%v) 必须 < rag.high_confidence(%v)",
			c.RAG.MinimumConfidence, c.RAG.HighConfidence)
	}
	return nil
}

// Addr 返回 HTTP 监听地址，如 "127.0.0.1:8080"。
func (c *Config) Addr() string {
	return fmt.Sprintf("%s:%d", c.Server.Host, c.Server.Port)
}
