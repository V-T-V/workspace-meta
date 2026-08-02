package viewstamped

import (
	"context"
	"testing"

	"github.com/QiuShichang/consensus-atlas/internal/core"
)

// TestDemoRuns 验证 demo 离线可跑：正常操作提交 + 视图变更换主。
func TestDemoRuns(t *testing.T) {
	res, err := Demo(context.Background())
	if err != nil {
		t.Fatalf("Demo 失败: %v", err)
	}
	if res.Replicas != 5 {
		t.Errorf("应有 5 副本，实际 %d", res.Replicas)
	}
	if res.InitialPrimary != "r1" {
		t.Errorf("初始 Primary 应为 r1，实际 %s", res.InitialPrimary)
	}
	if res.OpsCommitted != 2 {
		t.Errorf("应提交 2 个操作，实际 %d", res.OpsCommitted)
	}
	if !res.ViewChanged {
		t.Fatal("视图变更应成功")
	}
	if res.FinalView <= res.InitialView {
		t.Errorf("新 view 应 > 初始 view，实际 %d <= %d", res.FinalView, res.InitialView)
	}
	if res.FinalPrimary == "r1" || res.FinalPrimary == "" {
		t.Errorf("新 Primary 不应是下线的 r1，实际 %s", res.FinalPrimary)
	}
}

// TestNormalOperationQuorum Primary 收 quorum PrepareOK 后提交。
func TestNormalOperationQuorum(t *testing.T) {
	tr := core.NewMemTransport()
	ids := []core.NodeID{"a", "b", "c"}
	reps := map[core.NodeID]*Replica{}
	for _, id := range ids {
		r := NewReplica(id, ids, 1, 5, tr)
		r.Start()
		reps[id] = r
	}
	reps["a"].SetPrimary(true)

	ok := reps["a"].HandleRequest(RequestPayload{Op: "x", Client: "c", ReqNum: 1})
	if !ok {
		t.Fatal("Primary 应能处理请求")
	}
	for i := 0; i < 8; i++ {
		tr.Drain()
	}
	if reps["a"].CommitNum != 1 {
		t.Errorf("Primary 收 quorum 后应 CommitNum=1，实际 %d", reps["a"].CommitNum)
	}
}

// TestNonPrimaryRejectsRequest 非 Primary 不应处理请求。
func TestNonPrimaryRejectsRequest(t *testing.T) {
	tr := core.NewMemTransport()
	r := NewReplica("b", []core.NodeID{"a", "b"}, 1, 5, tr)
	r.Start()
	if r.HandleRequest(RequestPayload{Op: "x"}) {
		t.Error("非 Primary 不应能 HandleRequest")
	}
}

// TestBackupReceivesPrepare Backup 收到 Prepare 应记入日志并回 PrepareOK。
func TestBackupReceivesPrepare(t *testing.T) {
	tr := core.NewMemTransport()
	ids := []core.NodeID{"a", "b"}
	reps := map[core.NodeID]*Replica{}
	for _, id := range ids {
		r := NewReplica(id, ids, 1, 5, tr)
		r.Start()
		reps[id] = r
	}
	reps["a"].SetPrimary(true)
	reps["a"].HandleRequest(RequestPayload{Op: "x", Client: "c", ReqNum: 1})
	tr.Drain() // Prepare 投递给 b
	if reps["b"].OpNumber != 1 {
		t.Errorf("Backup 应记录 opNumber=1，实际 %d", reps["b"].OpNumber)
	}
	if reps["b"].Log.LastIndex() != 1 {
		t.Errorf("Backup 日志应有 1 条，实际 %d", reps["b"].Log.LastIndex())
	}
}

// TestViewChangeTimeout Backup 超时（无 Primary 消息）应发起 view change。
func TestViewChangeTimeout(t *testing.T) {
	tr := core.NewMemTransport()
	ids := []core.NodeID{"a", "b", "c"}
	reps := map[core.NodeID]*Replica{}
	for _, id := range ids {
		r := NewReplica(id, ids, 1, 3, tr)
		r.Start()
		reps[id] = r
	}
	reps["a"].SetPrimary(true)
	// 模拟 a 下线：卸载其 handler。
	tr.Install("a", func(core.Message) (core.Message, bool) { return core.Message{}, false })

	// b/c tick 直到发起 view change。
	for i := 0; i < 10; i++ {
		reps["b"].Tick()
		reps["c"].Tick()
		tr.Drain()
	}
	// 至少一个 Backup 应进入 view-change 或已换主。
	changed := reps["b"].View > 1 || reps["c"].View > 1
	if !changed {
		t.Error("Backup 超时后应推进 view（发起 view change）")
	}
}

// TestQuorum 验证 5 节点 quorum = 3。
func TestQuorum(t *testing.T) {
	tr := core.NewMemTransport()
	r := NewReplica("a", []core.NodeID{"a", "b", "c", "d", "e"}, 1, 5, tr)
	if q := r.quorum(); q != 3 {
		t.Errorf("5 副本 quorum 应为 3，实际 %d", q)
	}
}
