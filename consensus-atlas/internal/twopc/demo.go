package twopc

import (
	"context"
	"fmt"

	"github.com/QiuShichang/consensus-atlas/internal/core"
)

// DemoResult 是 2PC demo 的输出摘要。
type DemoResult struct {
	Participants int // 参与方数量
	// 场景一：全 Yes → Commit。
	CommitTxn   TxnID                  // 提交的事务 ID
	Committed   bool                   // 场景一最终是否 Commit
	FinalStates map[core.NodeID]string // 各参与方在 CommitTxn 上的终态
	// 场景二：一个 Participant 投 No → Abort。
	AbortTxn    TxnID                  // 放弃的事务 ID
	Aborted     bool                   // 场景二最终是否 Abort
	AbortStates map[core.NodeID]string // 各参与方在 AbortTxn 上的终态
	// 拒绝者 ID（场景二中投 No 的参与方）。
	Rejecter core.NodeID
}

// Demo 用 3 个 Participant 演示两阶段提交的两个典型场景：
//  1. 场景一（全 Yes → Commit）：Coordinator 发起事务 t1，3 个参与方都能提交，
//     收齐全票 Yes 后 Coordinator 下发 Commit，参与方落实为 Committed。
//  2. 场景二（任一 No → Abort）：Coordinator 发起事务 t2，其中一个参与方（p3）
//     拒绝（投 No），Coordinator 立即下发 Abort，参与方落实为 Aborted。
//
// 离线可跑（纯内存传输 + 显式 Drain，确定性轨迹，无 goroutine/time）。
func Demo(ctx context.Context) (*DemoResult, error) {
	_ = ctx
	tr := core.NewMemTransport()

	coordID := core.NodeID("coord")
	partIDs := []core.NodeID{"p1", "p2", "p3"}

	coord := NewCoordinator(coordID, partIDs, tr)
	coord.Start()

	parts := make(map[core.NodeID]*Participant, len(partIDs))
	for _, id := range partIDs {
		parts[id] = NewParticipant(id, tr)
		parts[id].Start()
	}

	res := &DemoResult{
		Participants: len(partIDs),
		FinalStates:  make(map[core.NodeID]string),
		AbortStates:  make(map[core.NodeID]string),
		Rejecter:     "p3",
	}

	// ---- 场景一：全 Yes → Commit ----
	t1 := TxnID("t1")
	if _, err := coord.Begin(t1); err != nil {
		return nil, fmt.Errorf("Begin t1: %w", err)
	}
	// 推进网络直到 Coordinator 做出决定并下发，参与方落实。
	// 轨迹：Prepare→Vote(Yes)→Commit→Ack，共需约 3 轮 Drain。
	for i := 0; i < 6; i++ {
		tr.Drain()
	}

	res.CommitTxn = t1
	committed, ok := coord.Outcome[t1]
	res.Committed = ok && committed
	for _, id := range partIDs {
		res.FinalStates[id] = parts[id].State(t1).String()
	}

	// ---- 场景二：p3 拒绝 → Abort ----
	// 让 p3 对 t2 投 No；p1/p2 保持总投 Yes。
	parts["p3"].SetCanCommit(func(id TxnID) bool { return id != "t2" })

	t2 := TxnID("t2")
	if _, err := coord.Begin(t2); err != nil {
		return nil, fmt.Errorf("Begin t2: %w", err)
	}
	for i := 0; i < 6; i++ {
		tr.Drain()
	}

	res.AbortTxn = t2
	aborted, ok := coord.Outcome[t2]
	// Outcome[t2]==false 表示决定为 Abort。
	res.Aborted = ok && !aborted
	for _, id := range partIDs {
		res.AbortStates[id] = parts[id].State(t2).String()
	}

	return res, nil
}
