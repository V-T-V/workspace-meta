package raft

import (
	"context"
	"fmt"

	"github.com/QiuShichang/consensus-atlas/internal/core"
)

// DemoResult 是 Raft demo 的输出摘要。
type DemoResult struct {
	LeaderID    core.NodeID
	Term        uint64
	CommitIndex uint64
	LogLen      int
	Replicated  bool // 提交后多数节点日志是否一致
}

// Demo 用 5 节点集群演示完整的 Raft 流程：
//  1. 启动 5 个 Follower（不同选举超时）
//  2. tick 推进，最先超时的节点当选 Leader
//  3. Leader 提交一条命令，复制到多数派并 commit
//
// 离线可跑（纯内存传输，确定性轨迹）。
func Demo(ctx context.Context) (*DemoResult, error) {
	_ = ctx
	tr := core.NewMemTransport()

	ids := []core.NodeID{"n1", "n2", "n3", "n4", "n5"}
	// 不同选举超时（5/7/9/11/13 ticks），保证 n1 最先觉醒当 Leader。
	timeouts := map[core.NodeID]int{"n1": 5, "n2": 7, "n3": 9, "n4": 11, "n5": 13}
	nodes := make(map[core.NodeID]*Node, len(ids))
	for _, id := range ids {
		n := NewNode(id, ids, timeouts[id], tr)
		n.Start()
		nodes[id] = n
	}

	// 第一阶段：选举。tick 直到产生 Leader（最多 30 个 tick 兜底）。
	var leader *Node
	for i := 0; i < 30 && leader == nil; i++ {
		for _, id := range ids {
			nodes[id].Tick()
		}
		tr.Drain()
		for _, id := range ids {
			if nodes[id].State == core.StateLeader {
				leader = nodes[id]
				break
			}
		}
	}
	if leader == nil {
		return nil, fmt.Errorf("选举超时未产生 Leader")
	}

	// 第二阶段：Leader 提交一条命令。
	leader.Propose("set x=1")
	// 多轮 drain 推进日志复制 + commit。
	for i := 0; i < 10; i++ {
		tr.Drain()
	}

	// 统计：有多少节点日志长度 ≥ 1（即收到了该条目）。
	replicated := 0
	for _, id := range ids {
		if nodes[id].Log.LastIndex() >= 1 {
			replicated++
		}
	}

	return &DemoResult{
		LeaderID:    leader.ID,
		Term:        leader.CurrentTerm,
		CommitIndex: leader.CommitIndex,
		LogLen:      len(leader.Log.Entries),
		Replicated:  replicated >= 3, // 多数派
	}, nil
}
