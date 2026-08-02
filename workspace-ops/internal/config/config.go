// Package config 实现 workspace-ops 的配置加载，三层：
// 默认值 → YAML 文件 → 命令行 flag（后者覆盖前者）。
//
// 对齐 auto-finance-assistant / generic-admin 的 config 范式。
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config 是 workspace-ops 的完整配置。
type Config struct {
	Scan    ScanConfig    `yaml:"scan"`
	Storage StorageConfig `yaml:"storage"`
	Logging LoggingConfig `yaml:"logging"`
	Server  ServerConfig  `yaml:"server"`
}

// ScanConfig 控制工作区扫描行为。
type ScanConfig struct {
	Root       string       `yaml:"root"`        // 工作区根目录（相对 config 文件或绝对）
	IgnoreDirs []string     `yaml:"ignore_dirs"` // 不算项目的目录名
	Checks     ChecksConfig `yaml:"checks"`      // 各检查项开关
}

// ChecksConfig 开关各检查项。
type ChecksConfig struct {
	Stack          bool `yaml:"stack"`
	Dependencies   bool `yaml:"dependencies"`
	AgentsMD       bool `yaml:"agents_md"`
	GitStatus      bool `yaml:"git_status"`
	Tests          bool `yaml:"tests"`
	BuildArtifacts bool `yaml:"build_artifacts"`
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
	cfg.Overlay(loaded, filepath.Dir(abs))
	return cfg, nil
}

// Overlay 用 loaded 覆盖 cfg 的字段（非零值覆盖），并解析相对路径。
func (cfg *Config) Overlay(loaded Config, baseDir string) {
	// Scan
	if loaded.Scan.Root != "" {
		cfg.Scan.Root = loaded.Scan.Root
	}
	if len(loaded.Scan.IgnoreDirs) > 0 {
		cfg.Scan.IgnoreDirs = loaded.Scan.IgnoreDirs
	}
	// Checks 覆盖规则：
	// bool 零值是 false，YAML 解析无法区分"字段未出现"和"显式设为 false"。
	// 折中规则：只要 loaded.Checks 任一字段为 true，就认为用户显式配置了 checks，整体替换。
	// 这意味着：用户可以在 YAML 里开启/关闭单项检查，但前提是至少保留一个 true。
	// 若想全部关闭，请删除 checks 块后用默认配置（全开），或修改 Default()。
	// 这是 yaml.v3 + bool 的已知限制，M2 若需精确控制可改用 *bool 指针。
	c := loaded.Scan.Checks
	if c.Stack || c.Dependencies || c.AgentsMD || c.GitStatus || c.Tests || c.BuildArtifacts {
		cfg.Scan.Checks = c
	}

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
}

// ResolveRoot 把 Scan.Root 解析为绝对路径（相对 configDir 或 cwd）。
func (cfg *Config) ResolveRoot(configDir string) (string, error) {
	root := cfg.Scan.Root
	if root == "" {
		root = "."
	}
	if filepath.IsAbs(root) {
		return root, nil
	}
	// 相对 configDir 解析；若 configDir 为空则相对 cwd。
	base := configDir
	if base == "" {
		base, _ = os.Getwd()
	}
	return filepath.Abs(filepath.Join(base, root))
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
