package byzantine

import (
	"context"
	"testing"

	"github.com/QiuShichang/consensus-atlas/internal/core"
)

// TestDemoRuns 验证 demo 离线可跑，请求最终 committed 且 sequence=1。
func TestDemoRuns(t *testing.T) {
	res, err := Demo(context.Background())
	if err != nil {
		t.Fatalf("Demo 失败: %v", err)
	}
	if !res.Committed {
		t.Error("请求应已 committed")
	}
	if res.Sequence != 1 {
		t.Errorf("Sequence 应为 1，实际 %d", res.Sequence)
	}
	if res.Replicas != 4 {
		t.Errorf("集群应为 4 节点，实际 %d", res.Replicas)
	}
	if res.Commits != 4 {
		t.Errorf("4 个诚实 replica 都应进入 committed，实际 %d", res.Commits)
	}
}

// TestQuorum 验证拜占庭 quorum = 2f+1（仅在合法 n=3f+1 时有意义）。
// 4 节点 → f=1 → quorum=2*1+1=3，等价公式 (2*4+2)/3=3。
// 7 节点 → f=2 → quorum=5。
func TestQuorum(t *testing.T) {
	tr := core.NewMemTransport()
	r := NewReplica("a", []core.NodeID{"a", "b", "c", "d"}, true, tr)
	if q := r.quorum(); q != 3 {
		t.Errorf("4 节点 quorum 应为 3，实际 %d", q)
	}
	// 7 节点 → f=2 → quorum=5
	r7 := NewReplica("a", []core.NodeID{"a", "b", "c", "d", "e", "f", "g"}, true, tr)
	if q := r7.quorum(); q != 5 {
		t.Errorf("7 节点 quorum 应为 5，实际 %d", q)
	}
}

// TestValidateCluster 验证集群规模校验。
func TestValidateCluster(t *testing.T) {
	// 合法配置
	cases := []struct {
		n, wantF int
	}{
		{4, 1}, {7, 2}, {10, 3},
	}
	for _, c := range cases {
		f, err := ValidateCluster(c.n)
		if err != nil {
			t.Errorf("n=%d 应合法，得到 error: %v", c.n, err)
		}
		if f != c.wantF {
			t.Errorf("n=%d 容忍拜占庭数应为 %d，实际 %d", c.n, c.wantF, f)
		}
	}
	// 非法配置
	for _, n := range []int{1, 2, 3, 5, 6, 8} {
		if _, err := ValidateCluster(n); err == nil {
			t.Errorf("n=%d 不满足 3f+1，应返回 error", n)
		}
	}
}

// TestPrepareThreshold 验证：必须收齐 2f+1 个 prepare 才进入 prepared。
func TestPrepareThreshold(t *testing.T) {
	tr := core.NewMemTransport()
	ids := []core.NodeID{"a", "b", "c", "d"}
	rs := make(map[core.NodeID]*Replica, len(ids))
	for i, id := range ids {
		r := NewReplica(id, ids, i == 0, tr)
		r.Start()
		rs[id] = r
	}
	primary := rs["a"]
	// quorum=3：primary 自投 1 票，还差 2 票。
	primary.Propose(Request{Op: "x"})
	tr.Drain() // 投递 pre-prepare

	// 此时各 replica 应已记录 primary 的 prepare 自投票，但还不到 3 票。
	// （prepare 消息已广播但尚未投递。）
	if rs["b"].IsPrepared(1) {
		t.Error("收齐 prepare 前不应进入 prepared")
	}
	// 投递一轮 prepare。
	tr.Drain()
	// 投递后 b 收到 a+c+d 的 prepare（含 a 自投），应达 3 票进入 prepared。
	// 但不同 replica 进入时机不同，多 drain 几轮确保传播完成。
	for i := 0; i < 5; i++ {
		tr.Drain()
	}
	if !rs["b"].IsPrepared(1) {
		t.Error("收齐 2f+1 prepare 后应进入 prepared")
	}
	if !primary.IsPrepared(1) {
		t.Error("Primary 自身也应进入 prepared")
	}
}

// TestNoPrimaryNoProgress 验证：没有 Primary 时集群无法推进共识。
func TestNoPrimaryNoProgress(t *testing.T) {
	tr := core.NewMemTransport()
	ids := []core.NodeID{"a", "b", "c", "d"}
	// 全部 Replica，没有 Primary。
	rs := make(map[core.NodeID]*Replica, len(ids))
	for _, id := range ids {
		r := NewReplica(id, ids, false, tr)
		r.Start()
		rs[id] = r
	}
	// 任一非 Primary 节点尝试 Propose 应失败。
	if err := rs["a"].Propose(Request{Op: "x"}); err == nil {
		t.Error("非 Primary 节点 Propose 应返回错误")
	}
	// 没有任何消息在网络上，drain 后无 committed。
	for i := 0; i < 5; i++ {
		tr.Drain()
	}
	for _, id := range ids {
		if rs[id].IsCommitted(1) {
			t.Errorf("无 Primary 时节点 %s 不应 committed", id)
		}
	}
}

// TestByzantineFaultTolerance 验证 PBFT 的核心保证：n=4(f=1) 集群中
// 存在 1 个拜占庭（叛徒）replica 时，其余 3 个诚实 replica 仍能达成共识（committed）。
//
// 叛徒模拟：把 r3 标为 IsTraitor=true。它在 prepare/commit 阶段静默故障——
// 既不广播自己的 Prepare/Commit 票，也不累计收到的票（对诚实节点相当于收不到它的票）。
//
// 关键断言：诚实节点 r0/r1/r2 最终 isCommitted=true，尽管有 1 个叛徒 r3。
// 数学保证：n=3f+1=4, quorum=(2*4+2)/3=3，诚实节点恰好 3 个 → 必达 quorum。
func TestByzantineFaultTolerance(t *testing.T) {
	tr := core.NewMemTransport()
	ids := []core.NodeID{"r0", "r1", "r2", "r3"} // n=4 → f=1, quorum=3

	f, err := ValidateCluster(len(ids))
	if err != nil {
		t.Fatalf("集群配置非法: %v", err)
	}
	if f != 1 {
		t.Fatalf("4 节点应容忍 1 拜占庭，得到 f=%d", f)
	}

	// 构造 4 节点：r0 为诚实 Primary，r1/r2 为诚实 replica，r3 为叛徒 replica。
	replicas := make(map[core.NodeID]*Replica, len(ids))
	for i, id := range ids {
		r := NewReplica(id, ids, i == 0, tr)
		// r3 是叛徒：静默丢弃所有 prepare/commit，既不投票也不累计。
		if id == "r3" {
			r.IsTraitor = true
		}
		r.Start()
		replicas[id] = r
	}

	// 诚实 Primary 提议一个请求。
	if err := replicas["r0"].Propose(Request{Op: "transfer 10 coins", Client: "c1"}); err != nil {
		t.Fatalf("Propose 失败: %v", err)
	}

	// 推进网络：三阶段消息在多轮 Drain 中传播。
	// 叛徒 r3 不贡献任何票，但 r0(Primary) + r1 + r2 三个诚实节点互投，
	// 每个诚实节点收到 3 张票（自己 + 另两个诚实）= quorum=3，达 prepared/committed。
	for i := 0; i < 10; i++ {
		tr.Drain()
	}

	// 关键断言：3 个诚实 replica 全部 committed。
	for _, id := range []core.NodeID{"r0", "r1", "r2"} {
		if !replicas[id].IsPrepared(1) {
			t.Errorf("拜占庭场景下诚实节点 %s 应进入 prepared（尽管叛徒 r3 静默故障）", id)
		}
		if !replicas[id].IsCommitted(1) {
			t.Errorf("拜占庭场景下诚实节点 %s 应进入 committed（PBFT 容忍 f=1 拜占庭）", id)
		}
	}

	// 叛徒自身不进入 committed（它丢弃了所有票）——这与 PBFT 语义一致：
	// PBFT 只保证"诚实节点达成一致"，不保证拜占庭节点的状态。
	if replicas["r3"].IsCommitted(1) {
		t.Error("叛徒 r3 不应 committed（它丢弃了所有 prepare/commit）")
	}

	// 验证 quorum 数学：(2*4+2)/3 = 3。
	if q := replicas["r0"].quorum(); q != 3 {
		t.Errorf("4 节点 quorum 应为 3，实际 %d", q)
	}
}
