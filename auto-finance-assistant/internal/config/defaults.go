package config

// Defaults 返回带合理默认值的配置。
// 开发机（CPU）与生产（GPU）的差异通过 config.yaml 覆盖，代码层一致。
func Defaults() *Config {
	return &Config{
		Server: ServerConfig{
			Host:               "127.0.0.1",
			Port:               8080,
			ReadTimeoutSeconds: 30,
			WriteTimeoutSeconds: 120,
		},
		Ollama: OllamaConfig{
			BaseURL:              "http://127.0.0.1:11434",
			ChatModel:            "qwen2.5:1.5b",
			EmbeddingModel:       "nomic-embed-text",
			RequestTimeoutSeconds: 90,
			KeepAlive:            "30m",
		},
		Generation: GenerationConfig{
			NumThread:       8, // CPU 默认；生产覆盖为 NumGPU
			ContextSize:     4096,
			MaxOutputTokens: 500,
			Temperature:     0.3,
			TopP:            0.9,
			TopK:            40,
			RepeatPenalty:   1.1,
		},
		RAG: RAGConfig{
			FTSLimit:          20,
			VectorLimit:       20,
			FinalLimit:        5,
			MinimumConfidence: 0.40,
			HighConfidence:    0.70,
			VectorWeight:      0.65,
			KeywordWeight:     0.35,
		},
		Queue: QueueConfig{
			GenerationConcurrency: 1,
			MaximumWaiting:        10,
			RequestTimeoutSeconds: 120,
		},
		Storage: StorageConfig{
			DatabasePath: "./data/assistant.db",
			DocumentPath: "./data/documents",
			TempPath:     "./data/temp",
			BackupPath:   "./data/backups",
		},
		Documents: DocumentsConfig{
			MaxFileSizeMB:     50,
			ChunkMinChars:     300,
			ChunkMaxChars:     800,
			ChunkOverlapChars: 80,
			AllowedExtensions: []string{".txt", ".md", ".html", ".docx", ".xlsx", ".pdf"},
		},
		Logging: LoggingConfig{
			Level:         "info",
			Directory:     "./data/logs",
			MaxFileSizeMB: 20,
			MaxFiles:      7,
			RetainDays:    30,
		},
		Backup: BackupConfig{
			Enabled:     false,
			Schedule:    "02:00",
			RetainCount: 7,
		},
		Security: SecurityConfig{
			AdminPassword: "",
		},
	}
}
