package agent

import (
	"context"
	"log"
	"time"

	"github.com/QiuShichang/gpu-mesh/internal/proto"
)

// sendRegister 连接建立后发送注册消息，登记身份/GPU/引擎/让位状态。
func (a *Agent) sendRegister(ctx context.Context, conn *wsConn) error {
	engines, models := a.buildRegister()
	reg := proto.AgentRegister{
		AgentID:  a.cfg.AgentID,
		Hostname: hostname(),
		OS:       osName(),
		Version:  Version,
		GPUs:     a.gpu.Snapshot(),
		Engines:  engines,
		Models:   models,
		Yield:    a.yield.State(),
		Tags:     a.cfg.Tags,
	}
	env, err := proto.NewEnvelope(proto.TypeRegister, a.cfg.AgentID, "relay", reg)
	if err != nil {
		return err
	}
	return conn.writeJSON(ctx, env)
}

// heartbeatLoop 周期心跳：携带最新 GPU 快照 + 让位状态。
func (a *Agent) heartbeatLoop(ctx context.Context, conn *wsConn) {
	ticker := time.NewTicker(a.cfg.HeartbeatInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			hb := proto.AgentHeartbeat{
				AgentID: a.cfg.AgentID,
				GPUs:    a.gpu.Snapshot(),
				Yield:   a.yield.State(),
				Seq:     a.nextSeq(),
			}
			env, err := proto.NewEnvelope(proto.TypeHeartbeat, a.cfg.AgentID, "relay", hb)
			if err != nil {
				continue
			}
			writeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			if err := conn.writeJSON(writeCtx, env); err != nil {
				cancel()
				log.Printf("[agent] 心跳发送失败: %v", err)
				return
			}
			cancel()
		}
	}
}
