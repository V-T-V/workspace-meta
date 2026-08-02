package config

// Default 返回合理的默认配置（不读任何文件）。
// 默认值对齐 config.example.yaml：db=flow-pipe.db, port=8767, log info/json, worker 关闭。
func Default() *Config {
	return &Config{
		Storage: StorageConfig{
			DatabasePath: "flow-pipe.db",
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "json",
		},
		Server: ServerConfig{
			Host:                "127.0.0.1",
			Port:                8767,
			ReadTimeoutSeconds:  30,
			WriteTimeoutSeconds: 30,
		},
		Worker: WorkerConfig{
			Enabled:   false,
			ServerURL: "",
			ID:        "",
		},
	}
}
