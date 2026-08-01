package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefault(t *testing.T) {
	cfg := Default()
	if cfg.Storage.DatabasePath != "flow-pipe.db" {
		t.Errorf("默认 database_path = %q, 想要 flow-pipe.db", cfg.Storage.DatabasePath)
	}
	if cfg.Server.Port != 8767 {
		t.Errorf("默认 port = %d, 想要 8767", cfg.Server.Port)
	}
	if cfg.Logging.Level != "info" || cfg.Logging.Format != "json" {
		t.Errorf("默认 logging = %s/%s, 想要 info/json", cfg.Logging.Level, cfg.Logging.Format)
	}
	if cfg.Worker.Enabled {
		t.Errorf("默认 worker.enabled 应为 false")
	}
	if got := cfg.Server.Addr(); got != "127.0.0.1:8767" {
		t.Errorf("Addr() = %q, 想要 127.0.0.1:8767", got)
	}
}

func TestLoadEmptyReturnsDefault(t *testing.T) {
	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load(\"\") 出错: %v", err)
	}
	d := Default()
	if cfg.Server.Port != d.Server.Port {
		t.Errorf("空路径应返回默认配置: port=%d want %d", cfg.Server.Port, d.Server.Port)
	}
}

func TestLoadThenOverlay(t *testing.T) {
	// 写一个临时 YAML，覆盖部分字段（server 端口 + logging format），其余保持默认。
	dir := t.TempDir()
	yamlPath := filepath.Join(dir, "config.yaml")
	content := `storage:
  database_path: "test.db"
logging:
  format: "text"
server:
  port: 9999
worker:
  enabled: true
  id: "w-1"
`
	if err := os.WriteFile(yamlPath, []byte(content), 0o644); err != nil {
		t.Fatalf("写临时配置失败: %v", err)
	}

	cfg, err := Load(yamlPath)
	if err != nil {
		t.Fatalf("Load 失败: %v", err)
	}

	// 覆盖的
	if cfg.Storage.DatabasePath != "test.db" {
		t.Errorf("database_path = %q, 想要 test.db", cfg.Storage.DatabasePath)
	}
	if cfg.Logging.Format != "text" {
		t.Errorf("logging.format = %q, 想要 text", cfg.Logging.Format)
	}
	if cfg.Server.Port != 9999 {
		t.Errorf("server.port = %d, 想要 9999", cfg.Server.Port)
	}
	if cfg.Worker.Enabled != true {
		t.Errorf("worker.enabled = false, 想要 true")
	}
	if cfg.Worker.ID != "w-1" {
		t.Errorf("worker.id = %q, 想要 w-1", cfg.Worker.ID)
	}

	// 未覆盖的应保留默认（host 保持 127.0.0.1，level 保持 info）
	if cfg.Server.Host != "127.0.0.1" {
		t.Errorf("未覆盖的 host = %q, 想保持默认 127.0.0.1", cfg.Server.Host)
	}
	if cfg.Logging.Level != "info" {
		t.Errorf("未覆盖的 level = %q, 想保持默认 info", cfg.Logging.Level)
	}

	// Addr 用覆盖后的端口
	if got := cfg.Server.Addr(); got != "127.0.0.1:9999" {
		t.Errorf("Addr() = %q, 想要 127.0.0.1:9999", got)
	}
}

func TestOverlayWorkerExplicitFalse(t *testing.T) {
	// worker.enabled 显式 false 应被采用（默认也是 false，语义一致）。
	base := Default()
	base.Worker.Enabled = true
	loaded := Config{} // Enabled = false（零值）
	base.Overlay(loaded)
	if base.Worker.Enabled {
		t.Errorf("Overlay 显式 false 后 enabled 应为 false")
	}
}

func TestResolveDBPath(t *testing.T) {
	// 绝对路径原样返回（用平台合适的绝对路径，Windows 需要盘符）
	cfg := Default()
	absInput, _ := filepath.Abs(filepath.Join(os.TempDir(), "abs.db"))
	cfg.Storage.DatabasePath = absInput
	got, err := cfg.ResolveDBPath()
	if err != nil {
		t.Fatalf("ResolveDBPath 失败: %v", err)
	}
	if got != absInput {
		t.Errorf("绝对路径 ResolveDBPath = %q, 想要 %q", got, absInput)
	}

	// 相对路径解析为绝对（含 cwd）
	cfg.Storage.DatabasePath = "rel.db"
	got, _ = cfg.ResolveDBPath()
	if !filepath.IsAbs(got) {
		t.Errorf("相对路径 ResolveDBPath 应返回绝对路径, 得到 %q", got)
	}
	if filepath.Base(got) != "rel.db" {
		t.Errorf("ResolveDBPath base = %q, 想要 rel.db", filepath.Base(got))
	}
}
