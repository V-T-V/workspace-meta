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
