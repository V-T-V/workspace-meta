package paxos

import (
	"context"
	"testing"

	"github.com/QiuShichang/consensus-atlas/internal/core"
)

// TestDemoRuns 验证 demo 离线可跑且 Learner 学到 "v1"。
func TestDemoRuns(t *testing.T) {
	res, err := Demo(context.Background())
	if err != nil {
		t.Fatalf("Demo 失败: %v", err)
	}
	if res.ChosenValue != "v1" {
		t.Errorf("ChosenValue 应为 v1，实际 %v", res.ChosenValue)
	}
	if res.Promises < 2 {
		t.Errorf("Proposer 至少应收 2 个 Promise，实际 %d", res.Promises)
	}
	if res.Accepts < 2 {
		t.Errorf("Proposer 至少应收 2 个 Accepted，实际 %d", res.Accepts)
	}
}

// TestQuorum 验证 3 个 Acceptor 的多数派 = 2。
func TestQuorum(t *testing.T) {
	tr := core.NewMemTransport()
	p := NewProposer("p", []core.NodeID{"a1", "a2", "a3"}, tr)
	if q := p.quorum(); q != 2 {
		t.Errorf("3 Acceptor quorum 应为 2，实际 %d", q)
	}
}

// TestHigherNumberWins 验证：Acceptor 先承诺 n=1 后，n=2 的 Prepare 应承诺；
// 但之后 n=1 的 Accept 应被拒（已承诺更高编号）。
func TestHigherNumberWins(t *testing.T) {
	tr := core.NewMemTransport()
	a := NewAcceptor("a1", []core.NodeID{"a1", "a2", "a3"}, tr)
	a.Start()

	// 1) n=1 的 Prepare：应承诺。
	rep, ok := a.handle(core.Message{
		From: "p", To: "a1", Kind: KindPrepare,
		Payload: PrepareRequest{Number: 1},
	})
	if !ok {
		t.Fatal("n=1 Prepare 应被处理")
	}
	pr := rep.Payload.(PrepareResponse)
	if !pr.Promised {
		t.Error("n=1 Prepare 应承诺")
	}
	if a.HighestPromised != 1 {
		t.Errorf("承诺后 HighestPromised 应为 1，实际 %d", a.HighestPromised)
	}

	// 2) n=2 的 Prepare：应再次承诺（更高编号胜出）。
	rep2, ok := a.handle(core.Message{
		From: "p", To: "a1", Kind: KindPrepare,
		Payload: PrepareRequest{Number: 2},
	})
	if !ok {
		t.Fatal("n=2 Prepare 应被处理")
	}
	pr2 := rep2.Payload.(PrepareResponse)
	if !pr2.Promised {
		t.Error("n=2 Prepare（更高编号）应承诺")
	}
	if a.HighestPromised != 2 {
		t.Errorf("承诺后 HighestPromised 应为 2，实际 %d", a.HighestPromised)
	}

	// 3) 之后 n=1 的 Accept：应被拒（已承诺 n=2，不再接受更低编号）。
	rep3, ok := a.handle(core.Message{
		From: "p", To: "a1", Kind: KindAccept,
		Payload: AcceptRequest{Number: 1, Value: "stale"},
	})
	if !ok {
		t.Fatal("n=1 Accept 应被处理（回复拒绝）")
	}
	ar := rep3.Payload.(AcceptResponse)
	if ar.Accepted {
		t.Error("n=1 Accept 在承诺 n=2 后应被拒")
	}
	if a.HighestAccepted != 0 {
		t.Errorf("被拒的 Accept 不应改写 HighestAccepted，实际 %d", a.HighestAccepted)
	}
}

// TestOldNumberRejected 验证：Acceptor 承诺 n=5 后，n=3 的 Prepare 被拒。
func TestOldNumberRejected(t *testing.T) {
	tr := core.NewMemTransport()
	a := NewAcceptor("a1", []core.NodeID{"a1", "a2", "a3"}, tr)
	a.Start()

	// 先承诺 n=5。
	_, ok := a.handle(core.Message{
		From: "p", To: "a1", Kind: KindPrepare,
		Payload: PrepareRequest{Number: 5},
	})
	if !ok {
		t.Fatal("n=5 Prepare 应被处理")
	}

	// 再来 n=3 的 Prepare：应拒绝（Promise=false）。
	rep, ok := a.handle(core.Message{
		From: "p", To: "a1", Kind: KindPrepare,
		Payload: PrepareRequest{Number: 3},
	})
	if !ok {
		t.Fatal("n=3 Prepare 应被处理（回复拒绝）")
	}
	pr := rep.Payload.(PrepareResponse)
	if pr.Promised {
		t.Error("n=3 Prepare 在承诺 n=5 后应被拒")
	}
	if a.HighestPromised != 5 {
		t.Errorf("被拒的 Prepare 不应改写 HighestPromised，实际 %d", a.HighestPromised)
	}
}

// TestProposalConflict 验证 Multi-Paxos 的核心冲突不变量：
// 两个 Proposer 同时用不同编号提案时，最终只有一个值被 chosen，
// 且较高编号的 Proposer 赢（其提议的值被选定）。
//
// 拓扑：p_high(编号 5, 值 "high") + p_low(编号 1, 值 "low") + 3 个 Acceptor + 1 Learner。
//
// 关键断言：
//  1. 只有一个值被 chosen（Paxos safety：绝不会同时选定两个不同值）
//  2. chosen 的值是 "high"（高编号胜出——p_high 的 Prepare 把 Acceptor 的
//     HighestPromised 抬到 5，使 p_low 的 Prepare(1) 全被拒，p_low 永远进不了 Phase 2）
//  3. Learner 学到的值与 Proposer 们自报的 ChosenValue 一致（无歧义）
//
// 用 MemTransport 的 FIFO 队列保证执行轨迹确定：先 propose p_high，再 propose p_low，
// p_high 的 Prepare 全部排在 p_low 前面，第一轮 Drain 就把承诺权交给 p_high。
// 注意：本测试用 p_high 先发起，是为了让"高编号胜出"在确定轨迹下成立；
// 若反过来先发 p_low，它可能在自己被覆盖前先 chosen "low"——那时高编号 Proposer
// 走 Prepare 会回收 "low" 并被迫提议 "low"（Paxos 不会覆盖已 chosen 的值），
// 这同样是合法的 safety，但不是本测试要验证的"高编号赢"竞争场景。
func TestProposalConflict(t *testing.T) {
	tr := core.NewMemTransport()

	acceptorIDs := []core.NodeID{"a1", "a2", "a3"}
	learnerID := core.NodeID("l1")

	// 3 个 Acceptor，每个都通知同一个 Learner。保留句柄用于一致性断言。
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

	// 两个 Proposer 用不同编号提案不同值。
	pHigh := NewProposer("p_high", acceptorIDs, tr)
	pHigh.ProposalNumber = 5
	pHigh.Value = "high"
	pHigh.Start()

	pLow := NewProposer("p_low", acceptorIDs, tr)
	pLow.ProposalNumber = 1
	pLow.Value = "low"
	pLow.Start()

	// 两个 Proposer "同时"发起提案：先 p_high 后 p_low，
	// MemTransport 的 FIFO 队列让 p_high 的 Prepare 全部排在 p_low 前面。
	pHigh.propose()
	pLow.propose()

	// 显式 Drain 推进网络，直到 Learner 学到值（最多 10 轮兜底）。
	for i := 0; i < 10 && !learner.Chosen; i++ {
		tr.Drain()
	}

	// ===== 不变量 1：最终必须有且仅有一个值被 chosen =====
	if !learner.Chosen {
		t.Fatal("Paxos 应在冲突提案下最终 chosen 一个值，但 Learner 未学到")
	}

	// 统计两个 Proposer 各自的 Chosen 状态。
	// 注意：Paxos safety 保证不会选定两个不同值；这里通过校验
	// "所有已 chosen 的值都相同"来等价表达"只有一个值被 chosen"。
	// 在本测试的确定轨迹下，p_low 进不了 Phase 2（Prepare 全被拒），
	// 故 pLow.Chosen 应为 false。
	chosenValues := make(map[any]int)
	if learner.Chosen {
		chosenValues[learner.Value]++
	}
	if pHigh.Chosen {
		chosenValues[pHigh.ChosenValue]++
	}
	if pLow.Chosen {
		chosenValues[pLow.ChosenValue]++
	}
	if len(chosenValues) > 1 {
		t.Errorf("Paxos safety 破裂：多个不同值被 chosen %v", chosenValues)
	}

	// ===== 不变量 2：较高编号的 Proposer 赢（值 = "high"）=====
	if learner.Value != "high" {
		t.Errorf("高编号 Proposer(5) 应赢，chosen 值应为 \"high\"，实际 %v", learner.Value)
	}
	if !pHigh.Chosen || pHigh.ChosenValue != "high" {
		t.Errorf("p_high 应确认 chosen=\"high\"，实际 Chosen=%v ChosenValue=%v",
			pHigh.Chosen, pHigh.ChosenValue)
	}

	// ===== 不变量 3：低编号 Proposer 未 chosen（其 Prepare 被高编号覆盖）=====
	if pLow.Chosen {
		t.Errorf("p_low(编号 1) 不应 chosen（被 p_high 编号 5 覆盖），但 ChosenValue=%v",
			pLow.ChosenValue)
	}

	// ===== 不变量 4：所有 Acceptor 最终接受同一值（跨节点一致性）=====
	// 高编号胜出后，3 个 Acceptor 都应承诺并接受编号 5 的 "high"，
	// 没有任何 Acceptor 残留 "low"（p_low 从未进 Phase 2）。
	for _, id := range acceptorIDs {
		a := acceptors[id]
		if a.HighestAccepted != 5 {
			t.Errorf("Acceptor %s 的 HighestAccepted 应为 5，实际 %d", id, a.HighestAccepted)
		}
		if a.AcceptedValue != "high" {
			t.Errorf("Acceptor %s 应接受 \"high\"，实际 %v", id, a.AcceptedValue)
		}
	}
}
