package viewstamped

import (
	"context"
	"fmt"

	"github.com/QiuShichang/consensus-atlas/internal/core"
)

// DemoResult 是 Viewstamped Replication demo 的输出摘要。
type DemoResult struct {
	Replicas       int         // 副本数
	InitialView    uint64      // 初始 view
	InitialPrimary core.NodeID // 初始 Primary
	// 阶段一：正常操作。
	OpsCommitted    int                    // Primary 处理并提交的请求数
	LastCommit      uint64                 // 阶段一 Primary（初始）最终 CommitNum
	BackupCommitted map[core.NodeID]uint64 // 各 Backup 阶段一终态 CommitNum（验证复制）
	// 阶段二：视图变更。
	ViewChanged      bool        // 是否成功换 view
	FinalView        uint64      // 换主后的 view
	FinalPrimary     core.NodeID // 换主后的新 Primary
	DownPrimary      core.NodeID // 被模拟下线的原 Primary
	NewPrimaryCommit uint64      // 新 Primary 换主后继续处理的请求提交后的 CommitNum
}

// Demo 用 5 副本演示 Viewstamped Replication 的两阶段流程：
//
//  1. **正常操作**：指定 r1 为初始 Primary，向它发 2 个请求。Primary 广播 Prepare，
//     收 quorum PrepareOK 后执行（推进 CommitNum），并同步给 Backup。
//  2. **视图变更**：把原 Primary "下线"（停止给它投递消息），Backup 超时后发起
//     StartViewChange → DoViewChange → StartView，选出新 Primary，新 Primary 能继续处理请求。
//
// 离线可跑（MemTransport + 显式 Drain，确定性轨迹，无 goroutine/time/rand）。
func Demo(ctx context.Context) (*DemoResult, error) {
	_ = ctx
	tr := core.NewMemTransport()

	ids := []core.NodeID{"r1", "r2", "r3", "r4", "r5"}
	// Backup 的 view-change 超时（tick 计），错开保证确定性（最先超时者发起换主）。
	timeouts := map[core.NodeID]int{"r1": 999, "r2": 5, "r3": 7, "r4": 9, "r5": 11}
	// r1 是初始 Primary，给它一个很大的超时（它不会主动换主）。

	reps := make(map[core.NodeID]*Replica, len(ids))
	for _, id := range ids {
		r := NewReplica(id, ids, 1, timeouts[id], tr)
		r.Start()
		reps[id] = r
	}
	// 指定 r1 为初始 Primary。
	reps["r1"].SetPrimary(true)

	res := &DemoResult{
		Replicas:        len(ids),
		InitialView:     1,
		InitialPrimary:  "r1",
		BackupCommitted: make(map[core.NodeID]uint64),
		DownPrimary:     "r1",
	}

	// ---- 阶段一：正常操作。向 Primary r1 发 2 个请求。 ----
	for i, op := range []string{"set x=1", "set y=2"} {
		_ = i
		ok := reps["r1"].HandleRequest(RequestPayload{Op: op, Client: "client", ReqNum: uint64(i + 1)})
		if !ok {
			return res, fmt.Errorf("Primary 应能处理请求 %s", op)
		}
		// Drain 推进 Prepare → PrepareOK → commit。
		for d := 0; d < 8; d++ {
			tr.Drain()
		}
	}
	res.OpsCommitted = 2
	res.LastCommit = reps["r1"].CommitNum
	for _, id := range ids {
		if id != "r1" {
			res.BackupCommitted[id] = reps[id].CommitNum
		}
	}

	// ---- 阶段二：视图变更。模拟 r1 下线：从 transport "卸载" r1 的 handler。 ----
	// 用一个 noop handler 覆盖 r1，使其不再处理任何消息（等同 crash）。
	noop := func(core.Message) (core.Message, bool) { return core.Message{}, false }
	tr.Install("r1", noop)
	// 注意：r1 仍持有 IsPrimary=true 的状态，但它收不到消息、发不出消息。

	// Backup tick 直到最先超时者发起 StartViewChange（最多 30 tick 兜底）。
	var changed bool
	for i := 0; i < 30 && !changed; i++ {
		for _, id := range ids {
			if id == "r1" {
				continue // r1 下线，不 tick
			}
			reps[id].Tick()
		}
		tr.Drain()
		// 检查是否有非 r1 副本成了新 Primary。
		for _, id := range ids {
			if id == "r1" {
				continue
			}
			if reps[id].IsPrimary && reps[id].State == StateNormal && reps[id].View > 1 {
				res.FinalPrimary = id
				res.FinalView = reps[id].View
				changed = true
				break
			}
		}
	}
	res.ViewChanged = changed
	if !changed {
		return res, fmt.Errorf("视图变更未在 30 tick 内完成")
	}

	// 验证：新 Primary 能继续处理一个新请求（恢复服务）。
	newP := reps[res.FinalPrimary]
	ok := newP.HandleRequest(RequestPayload{Op: "set z=3", Client: "client", ReqNum: 99})
	if !ok {
		return res, fmt.Errorf("新 Primary %s 应能处理请求", res.FinalPrimary)
	}
	for d := 0; d < 8; d++ {
		tr.Drain()
	}
	// 新 Primary 的 CommitNum 应增加。
	if newP.CommitNum <= res.LastCommit {
		return res, fmt.Errorf("新 Primary 应能推进 commit，实际 %d <= 旧 %d", newP.CommitNum, res.LastCommit)
	}
	res.NewPrimaryCommit = newP.CommitNum

	return res, nil
}
