// Package config 实现 flow-pipe 的配置加载，三层：
// 默认值 → YAML 文件 → 命令行 flag（后者覆盖前者）。
//
// 配置四块：storage（数据库）/ logging（日志）/ server（HTTP）/ worker（分布式，M3 才启用）。
// 对齐 workspace-ops / generic-admin 的 config 范式。
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config 是 flow-pipe 的完整配置。
type Config struct {
	Storage StorageConfig `yaml:"storage"`
	Logging LoggingConfig `yaml:"logging"`
	Server  ServerConfig  `yaml:"server"`
	Worker  WorkerConfig  `yaml:"worker"`
}

// StorageConfig 数据库配置。
type StorageConfig struct {
	DatabasePath string `yaml:"database_path"`
}

// LoggingConfig 日志配置。
type LoggingConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
}

// ServerConfig Web 服务配置。
type ServerConfig struct {
	Host                string `yaml:"host"`
	Port                int    `yaml:"port"`
	ReadTimeoutSeconds  int    `yaml:"read_timeout_seconds"`
	WriteTimeoutSeconds int    `yaml:"write_timeout_seconds"`
}

// Addr 返回 Server 的监听地址 "host:port"。
func (s ServerConfig) Addr() string {
	return fmt.Sprintf("%s:%d", s.Host, s.Port)
}

// WorkerConfig 分布式 worker 配置（M3 才启用，M1 单机模式忽略）。
type WorkerConfig struct {
	Enabled   bool   `yaml:"enabled"`    // true 时启动 worker 模式（cmd/worker）
	ServerURL string `yaml:"server_url"` // 中心调度器地址（ws://host:port）
	ID        string `yaml:"id"`         // worker 唯一 ID（空则自动生成）
}

// Load 从 YAML 文件加载配置，未设置的字段用默认值填充。
// configPath 为空时只返回默认配置。
func Load(configPath string) (*Config, error) {
	cfg := Default()

	if configPath == "" {
		return cfg, nil
	}

	abs, err := filepath.Abs(configPath)
	if err != nil {
		return nil, fmt.Errorf("解析配置路径失败 %s: %w", configPath, err)
	}
	raw, err := os.ReadFile(abs)
	if err != nil {
		return nil, fmt.Errorf("读取配置失败 %s: %w", configPath, err)
	}

	// 先解码到一个临时变量，再选择性覆盖（保留默认值的字段语义）。
	var loaded Config
	if err := yaml.Unmarshal(raw, &loaded); err != nil {
		return nil, fmt.Errorf("解析 YAML 失败 %s: %w", configPath, err)
	}
	cfg.Overlay(loaded)
	return cfg, nil
}

// Overlay 用 loaded 覆盖 cfg 的字段（非零值覆盖）。
//
// 注意 bool 字段（Worker.Enabled）的零值是 false，无法区分"未设"和"设为 false"，
// 这里沿用 workspace-ops 的简化策略：YAML 里显式 false 也直接采用（因为 worker 默认就是 false，
// 显式写 false 等价于不变；显式写 true 时覆盖默认 false，语义正确）。
func (cfg *Config) Overlay(loaded Config) {
	// Storage
	if loaded.Storage.DatabasePath != "" {
		cfg.Storage.DatabasePath = loaded.Storage.DatabasePath
	}
	// Logging
	if loaded.Logging.Level != "" {
		cfg.Logging.Level = loaded.Logging.Level
	}
	if loaded.Logging.Format != "" {
		cfg.Logging.Format = loaded.Logging.Format
	}
	// Server
	if loaded.Server.Host != "" {
		cfg.Server.Host = loaded.Server.Host
	}
	if loaded.Server.Port != 0 {
		cfg.Server.Port = loaded.Server.Port
	}
	if loaded.Server.ReadTimeoutSeconds != 0 {
		cfg.Server.ReadTimeoutSeconds = loaded.Server.ReadTimeoutSeconds
	}
	if loaded.Server.WriteTimeoutSeconds != 0 {
		cfg.Server.WriteTimeoutSeconds = loaded.Server.WriteTimeoutSeconds
	}
	// Worker
	cfg.Worker.Enabled = loaded.Worker.Enabled // 显式写就用（含 false）
	if loaded.Worker.ServerURL != "" {
		cfg.Worker.ServerURL = loaded.Worker.ServerURL
	}
	if loaded.Worker.ID != "" {
		cfg.Worker.ID = loaded.Worker.ID
	}
}

// ResolveDBPath 把数据库路径解析为绝对路径（相对 cwd）。
func (cfg *Config) ResolveDBPath() (string, error) {
	p := cfg.Storage.DatabasePath
	if filepath.IsAbs(p) {
		return p, nil
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", err
	}
	return abs, nil
}
