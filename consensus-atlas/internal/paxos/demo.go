package paxos

import (
	"context"
	"fmt"

	"github.com/QiuShichang/consensus-atlas/internal/core"
)

// DemoResult 是 Paxos demo 的输出摘要。
type DemoResult struct {
	ChosenValue any // Learner 学到的最终值
	Rounds      int // 走完的 Paxos 轮数（1 轮 = 一次成功的两阶段）
	Promises    int // Proposer 收到的 Promise 数
	Accepts     int // Proposer 收到的 Accepted=true 数
}

// Demo 用 1 Proposer + 3 Acceptor + 1 Learner 演示一次完整的 Multi-Paxos：
//  1. Proposer 用编号 1 发 Prepare（Phase 1）
//  2. Acceptor 承诺并回 Promise，Proposer 收齐多数派后选值发 Accept（Phase 2）
//  3. Acceptor 接受并通知 Learner，Learner 学到 "v1"
//
// 离线可跑（纯内存传输 + 显式 Drain，确定性轨迹，不用 goroutine/time）。
func Demo(ctx context.Context) (*DemoResult, error) {
	_ = ctx
	tr := core.NewMemTransport()

	// 角色身份。
	proposerID := core.NodeID("p1")
	acceptorIDs := []core.NodeID{"a1", "a2", "a3"}
	learnerID := core.NodeID("l1")

	// Proposer 的 Peers = 3 个 Acceptor（quorum = 3/2+1 = 2，正确 Paxos 多数派）。
	proposer := NewProposer(proposerID, acceptorIDs, tr)
	proposer.ProposalNumber = 1
	proposer.Value = "v1"
	proposer.Start()

	// 3 个 Acceptor，每个都知道完整 Acceptor 集群（Peers）+ 通知 Learner。
	acceptors := make(map[core.NodeID]*Acceptor, len(acceptorIDs))
	for _, id := range acceptorIDs {
		a := NewAcceptor(id, acceptorIDs, tr)
		a.Learners = []core.NodeID{learnerID}
		a.Start()
		acceptors[id] = a
	}

	// Learner 的 Peers = 3 个 Acceptor（quorum = 2）。
	learner := NewLearner(learnerID, acceptorIDs, tr)
	learner.Start()

	// 触发 Phase 1：Proposer 向 Acceptor 广播 Prepare。
	proposer.propose()

	// 显式 Drain 推进网络 tick，直到 Learner 学到值（最多 8 轮兜底）。
	rounds := 0
	for i := 0; i < 8 && !learner.Chosen; i++ {
		tr.Drain()
		rounds++
	}

	if !learner.Chosen {
		return nil, fmt.Errorf("Paxos 未能在 %d 轮内 chosen", rounds)
	}

	return &DemoResult{
		ChosenValue: learner.Value,
		Rounds:      rounds,
		Promises:    proposer.promises,
		Accepts:     proposer.accepted,
	}, nil
}
