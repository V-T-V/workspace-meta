package twopc

import (
	"context"
	"testing"

	"github.com/QiuShichang/consensus-atlas/internal/core"
)

// TestDemoRuns 验证 demo 离线可跑：全 Yes 场景提交，任一 No 场景放弃。
func TestDemoRuns(t *testing.T) {
	res, err := Demo(context.Background())
	if err != nil {
		t.Fatalf("Demo 失败: %v", err)
	}
	if res.Participants != 3 {
		t.Errorf("应有 3 个参与方，实际 %d", res.Participants)
	}
	// 场景一：全 Yes → Commit。
	if !res.Committed {
		t.Error("全 Yes 场景应 Commit")
	}
	for _, id := range []core.NodeID{"p1", "p2", "p3"} {
		if s := res.FinalStates[id]; s != "committed" {
			t.Errorf("参与方 %s 在 t1 上应 committed，实际 %s", id, s)
		}
	}
	// 场景二：p3 拒绝 → Abort。
	if !res.Aborted {
		t.Error("任一 No 场景应 Abort")
	}
	if s := res.AbortStates["p3"]; s != "aborted" {
		t.Errorf("拒绝者 p3 在 t2 上应 aborted，实际 %s", s)
	}
}

// coordID 是测试用的协调者 ID，故意与参与方 a/b/c 不同名，避免两者在
// MemTransport 上注册时互相覆盖 handler（coordinator 与 participant 共享一个传输层）。
const coordID core.NodeID = "coord"

// TestAllYesCommit 表驱动：所有参与方都能提交 → 决定 Commit。
func TestAllYesCommit(t *testing.T) {
	tr := core.NewMemTransport()
	coord := NewCoordinator(coordID, []core.NodeID{"a", "b", "c"}, tr)
	coord.Start()
	parts := map[core.NodeID]*Participant{}
	for _, id := range []core.NodeID{"a", "b", "c"} {
		p := NewParticipant(id, tr)
		p.Start()
		parts[id] = p
	}
	if _, err := coord.Begin("tx"); err != nil {
		t.Fatalf("Begin: %v", err)
	}
	for i := 0; i < 6; i++ {
		tr.Drain()
	}
	if got, ok := coord.Outcome["tx"]; !ok || !got {
		t.Errorf("全 Yes 应决定 Commit，got=%v ok=%v", got, ok)
	}
	for _, id := range []core.NodeID{"a", "b", "c"} {
		if s := parts[id].State("tx"); s != StateCommitted {
			t.Errorf("参与方 %s 应 Committed，实际 %s", id, s)
		}
	}
}

// TestOneNoAbort 任一参与方拒绝 → 决定 Abort，且其他参与方应进入 Aborted。
func TestOneNoAbort(t *testing.T) {
	tr := core.NewMemTransport()
	ids := []core.NodeID{"a", "b", "c"}
	coord := NewCoordinator(coordID, ids, tr)
	coord.Start()
	parts := map[core.NodeID]*Participant{}
	for _, id := range ids {
		p := NewParticipant(id, tr)
		p.Start()
		parts[id] = p
	}
	// 让 b 拒绝。
	parts["b"].SetCanCommit(func(TxnID) bool { return false })

	if _, err := coord.Begin("tx"); err != nil {
		t.Fatalf("Begin: %v", err)
	}
	for i := 0; i < 6; i++ {
		tr.Drain()
	}
	if got, ok := coord.Outcome["tx"]; !ok || got {
		t.Errorf("任一 No 应决定 Abort，got=%v ok=%v", got, ok)
	}
}

// TestUnanimityQuorum 验证 2PC 的一致同意：3 票中 2 票 Yes + 1 票 No 仍 Abort。
// 与 Raft 的"多数派"(2/3 即可) 形成对比。
func TestUnanimityQuorum(t *testing.T) {
	tr := core.NewMemTransport()
	ids := []core.NodeID{"a", "b", "c"}
	coord := NewCoordinator(coordID, ids, tr)
	coord.Start()
	parts := map[core.NodeID]*Participant{}
	for _, id := range ids {
		p := NewParticipant(id, tr)
		p.Start()
		parts[id] = p
	}
	// a/b 投 Yes，c 投 No：2/3 多数同意但仍应 Abort（unanimity 要求全票）。
	parts["c"].SetCanCommit(func(TxnID) bool { return false })

	coord.Begin("tx")
	for i := 0; i < 6; i++ {
		tr.Drain()
	}
	if got, _ := coord.Outcome["tx"]; got {
		t.Error("2PC 要求一致同意(unanimity)：2 Yes + 1 No 仍应 Abort，不是多数派通过")
	}
}

// TestPreparedDurability 投 Yes 后进 Prepared 态；后续 Commit 必须能落实（durability）。
func TestPreparedDurability(t *testing.T) {
	tr := core.NewMemTransport()
	coord := NewCoordinator(coordID, []core.NodeID{"a"}, tr)
	coord.Start()
	p := NewParticipant("a", tr)
	p.Start()

	coord.Begin("tx")
	// 第一轮 Drain：Prepare 投递，a 投 Yes。
	tr.Drain()
	if s := p.State("tx"); s != StatePrepared {
		t.Fatalf("投 Yes 后应为 Prepared，实际 %s", s)
	}
	// 后续轮次：Coordinator 收齐 Yes → Commit → a 落实 Committed。
	for i := 0; i < 4; i++ {
		tr.Drain()
	}
	if s := p.State("tx"); s != StateCommitted {
		t.Errorf("Prepared 收到 Commit 应落实 Committed（durability），实际 %s", s)
	}
}

// TestAbortNotOverridden 已 Aborted 的参与方收到 Commit 不应被覆盖为 Committed。
func TestAbortNotOverridden(t *testing.T) {
	tr := core.NewMemTransport()
	coord := NewCoordinator(coordID, []core.NodeID{"a", "b"}, tr)
	coord.Start()
	pa := NewParticipant("a", tr)
	pa.Start()
	pb := NewParticipant("b", tr)
	pb.Start()
	// a 拒绝 → Aborted；b 投 Yes → Prepared。
	pa.SetCanCommit(func(TxnID) bool { return false })

	coord.Begin("tx")
	for i := 0; i < 6; i++ {
		tr.Drain()
	}
	// a 已 Aborted，Coordinator 决定 Abort，a 收到 Abort 保持 Aborted。
	if s := pa.State("tx"); s != StateAborted {
		t.Errorf("拒绝者应保持 Aborted，实际 %s", s)
	}
}

// TestDuplicateBegin 重复 Begin 同一 TxnID 应报错。
func TestDuplicateBegin(t *testing.T) {
	tr := core.NewMemTransport()
	coord := NewCoordinator(coordID, []core.NodeID{"a"}, tr)
	coord.Start()
	if _, err := coord.Begin("tx"); err != nil {
		t.Fatalf("第一次 Begin: %v", err)
	}
	if _, err := coord.Begin("tx"); err == nil {
		t.Error("重复 Begin 同一 TxnID 应报错")
	}
}
