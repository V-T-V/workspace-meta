package config

// Default 返回合理的默认配置（不读任何文件）。
// 根目录默认 ".."（相对配置文件即工作区根），实际启动时按 cwd 再解析。
func Default() *Config {
	return &Config{
		Scan: ScanConfig{
			Root: "..",
			IgnoreDirs: []string{
				"node_modules",
				"godot-src",
				"bevy-src",
				"relay",
				"proto",
				"export",
				"scripts",
				".portfolio",
				".git",
			},
			Checks: ChecksConfig{
				Stack:          true,
				Dependencies:   true,
				AgentsMD:       true,
				GitStatus:      true,
				Tests:          true,
				BuildArtifacts: true,
			},
		},
		Storage: StorageConfig{
			DatabasePath: "workspace-ops.db",
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "json",
		},
		Server: ServerConfig{
			Host:                "127.0.0.1",
			Port:                8765,
			ReadTimeoutSeconds:  15,
			WriteTimeoutSeconds: 15,
		},
	}
}
