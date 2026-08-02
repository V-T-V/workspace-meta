package agent

import (
	"os"
	"time"
)

// Config Agent 配置。
type Config struct {
	// AgentID 唯一标识。空则用 hostname。
	AgentID string
	// RelayURL Relay 的 WS 接入地址，如 ws://relay.example.com:7780/agent
	// 裸 IP 会自动补全为 ws://IP:7780/agent
	RelayURL string
	// Token 鉴权 token（可选，Relay 未设则忽略）。
	Token string
	// HeartbeatInterval 心跳周期，默认 5s。
	HeartbeatInterval time.Duration
	// GPUCollectInterval GPU 采集周期，默认 2s。
	GPUCollectInterval time.Duration
	// Tags 业务标签，上报给 Relay 供调度/分组。
	Tags map[string]string
}

func (c *Config) applyDefaults() {
	if c.AgentID == "" {
		hn, _ := os.Hostname()
		c.AgentID = hn
	}
	if c.HeartbeatInterval == 0 {
		c.HeartbeatInterval = 5 * time.Second
	}
	if c.GPUCollectInterval == 0 {
		c.GPUCollectInterval = 2 * time.Second
	}
}
