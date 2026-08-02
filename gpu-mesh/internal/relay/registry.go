// Package relay 实现 gpu-mesh 的中继节点（Relay）。
//
// Relay 是部署在公网 VPS 上的单二进制，职责：
//   - 接受 Agent 的反向 WS 连接，穿透 NAT
//   - 维护在线 Agent 注册表（含最新 GPU 快照 + 让位状态）
//   - 提供 REST API + SSE 给 Web 控制台
//   - （Phase 2+）提供 OpenAI 兼容推理网关
//
// 依赖方向（单向）：relay → proto。
package relay

import (
	"context"
	"sync"
	"time"

	"github.com/coder/websocket"

	"github.com/QiuShichang/gpu-mesh/internal/proto"
)

// agentEntry 一个在线 Agent 的全部状态。
type agentEntry struct {
	info         proto.AgentRegister // 注册信息（含 GPU/引擎/让位）
	heartbeat    proto.AgentHeartbeat // 最新心跳（GPU 快照 + 让位状态）
	conn         *wsConn              // WS 连接
	lastBeat     time.Time            // 最近心跳时间
	registeredAt time.Time            // 注册时间
}

// Registry 维护在线 Agent 注册表（线程安全）。
type Registry struct {
	mu     sync.RWMutex
	agents map[string]*agentEntry
}

// NewRegistry 构造注册表。
func NewRegistry() *Registry {
	return &Registry{agents: make(map[string]*agentEntry)}
}

// Register 登记一个 Agent 上线。
func (r *Registry) Register(info proto.AgentRegister, conn *wsConn) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	_, wasOnline := r.agents[info.AgentID]
	r.agents[info.AgentID] = &agentEntry{
		info:         info,
		heartbeat:    proto.AgentHeartbeat{AgentID: info.AgentID, GPUs: info.GPUs, Yield: info.Yield},
		conn:         conn,
		lastBeat:     time.Now(),
		registeredAt: time.Now(),
	}
	return wasOnline
}

// UpdateHeartbeat 刷新心跳（GPU 快照 + 让位状态）。
// 返回 yieldChanged=true 表示让位档位发生变化（Relay 据此决定是否广播 yield_update）。
//
// 优化：避免 100 台机器 × 5s 心跳 = 每秒 20 次无变化广播。
// 只在 yield level 真正切换（IDLE→ACTIVE→BUSY）时才返回 true 触发广播。
// GPU 利用率/显存的细微变化由控制台 5s 轮询 /api/agents 兜底刷新。
func (r *Registry) UpdateHeartbeat(agentID string, hb proto.AgentHeartbeat) (ok bool, yieldChanged bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	e, exist := r.agents[agentID]
	if !exist {
		return false, false
	}
	oldLevel := e.heartbeat.Yield.Level
	e.heartbeat = hb
	e.lastBeat = time.Now()
	return true, oldLevel != hb.Yield.Level
}

// Unregister 移除 Agent（仅当 conn 匹配，防新连接被旧连接断开事件误删）。
func (r *Registry) Unregister(agentID string, conn *wsConn) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	e, ok := r.agents[agentID]
	if !ok || e.conn != conn {
		return false
	}
	delete(r.agents, agentID)
	return true
}

// Conn 取 Agent 连接。
func (r *Registry) Conn(agentID string) *wsConn {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if e, ok := r.agents[agentID]; ok {
		return e.conn
	}
	return nil
}

// Snapshot 返回所有在线 Agent 的完整状态（供控制台展示）。
func (r *Registry) Snapshot() []AgentView {
	return r.snapshotExcluding(nil)
}

// FindByTag 返回带指定标签 "key=value" 的在线 Agent ID 列表。
func (r *Registry) FindByTag(tag string) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []string
	for id, e := range r.agents {
		for k, v := range e.info.Tags {
			if k+"="+v == tag {
				out = append(out, id)
				break
			}
		}
	}
	return out
}

// SnapshotExcluding 返回在线 Agent 快照，排除 excludeIDs（重调度用，避免重复选同一节点）。
func (r *Registry) SnapshotExcluding(excludeIDs []string) []AgentView {
	return r.snapshotExcluding(excludeIDs)
}

// snapshotExcluding 内部实现，Snapshot/SnapshotExcluding 共用，消除重复代码。
func (r *Registry) snapshotExcluding(excludeIDs []string) []AgentView {
	excludeSet := make(map[string]bool, len(excludeIDs))
	for _, id := range excludeIDs {
		excludeSet[id] = true
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]AgentView, 0, len(r.agents))
	for _, e := range r.agents {
		if excludeSet[e.info.AgentID] {
			continue
		}
		out = append(out, AgentView{
			AgentID:     e.info.AgentID,
			Hostname:    e.info.Hostname,
			OS:          e.info.OS,
			Version:     e.info.Version,
			Engines:     e.info.Engines,
			Models:      e.info.Models,
			Tags:        e.info.Tags,
			GPUs:        e.heartbeat.GPUs,
			Yield:       e.heartbeat.Yield,
			Online:      true,
			LastBeatAgo: int(time.Since(e.lastBeat).Seconds()),
			Uptime:      int(time.Since(e.registeredAt).Seconds()),
		})
	}
	return out
}

// Sweep 清理超时未心跳的 Agent，返回被清理的 ID + 连接（调用方关连接）。
func (r *Registry) Sweep(timeout time.Duration) ([]string, []*wsConn) {
	r.mu.Lock()
	var removed []string
	var conns []*wsConn
	now := time.Now()
	for id, e := range r.agents {
		if now.Sub(e.lastBeat) > timeout {
			delete(r.agents, id)
			removed = append(removed, id)
			conns = append(conns, e.conn)
		}
	}
	r.mu.Unlock()
	return removed, conns
}

// AgentView 控制台展示用的 Agent 视图（注册信息 + 最新心跳合并）。
type AgentView struct {
	AgentID     string              `json:"agent_id"`
	Hostname    string              `json:"hostname"`
	OS          string              `json:"os"`
	Version     string              `json:"version"`
	Engines     []string            `json:"engines"`
	Models      []string            `json:"models"`
	Tags        map[string]string   `json:"tags,omitempty"`
	GPUs        []proto.GPUSnapshot `json:"gpus"`
	Yield       proto.YieldState    `json:"yield"`
	Online      bool                `json:"online"`
	LastBeatAgo int                 `json:"last_beat_ago_s"`
	Uptime      int                 `json:"uptime_s"`
}

// wsConn 封装连接（与 agent 包对称设计，串行化写）。
type wsConn struct {
	conn    *websocket.Conn
	writeMu sync.Mutex
}

func (w *wsConn) writeJSON(ctx context.Context, env proto.Envelope) error {
	data, err := marshalJSON(env)
	if err != nil {
		return err
	}
	w.writeMu.Lock()
	defer w.writeMu.Unlock()
	return w.conn.Write(ctx, websocket.MessageText, data)
}
