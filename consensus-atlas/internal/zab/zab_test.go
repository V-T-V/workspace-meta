package zab

import (
	"context"
	"testing"

	"github.com/QiuShichang/consensus-atlas/internal/core"
)

// TestDemoRuns 验证 demo 离线可跑：3 个请求全部 commit，zxid 递增，各 Follower 一致。
func TestDemoRuns(t *testing.T) {
	res, err := Demo(context.Background())
	if err != nil {
		t.Fatalf("Demo 失败: %v", err)
	}
	if res.Followers != 3 {
		t.Errorf("应有 3 个 Follower，实际 %d", res.Followers)
	}
	if len(res.ProposedZXIDs) != 3 {
		t.Errorf("应提议 3 个 zxid，实际 %d", len(res.ProposedZXIDs))
	}
	if len(res.CommittedZXIDs) != 3 {
		t.Errorf("应 commit 3 个 zxid，实际 %d", len(res.CommittedZXIDs))
	}
	if !res.Ordered {
		t.Error("zxid 应全局单调递增")
	}
	// 各 Follower 应都 commit 了全部 3 个，且顺序与 Leader 一致。
	for fid, fc := range res.FollowerCommitted {
		if len(fc) != 3 {
			t.Errorf("Follower %s 应 commit 3 个，实际 %d", fid, len(fc))
		}
		for i := range res.CommittedZXIDs {
			if fc[i] != res.CommittedZXIDs[i] {
				t.Errorf("Follower %s commit 顺序与 Leader 不一致: %v vs %v", fid, fc, res.CommittedZXIDs)
			}
		}
	}
}

// TestZXIDEncoding 验证 zxid 的 epoch/counter 编解码。
func TestZXIDEncoding(t *testing.T) {
	z := MakeZXID(1, 5)
	if z.Epoch() != 1 {
		t.Errorf("epoch 应为 1，实际 %d", z.Epoch())
	}
	if z.Counter() != 5 {
		t.Errorf("counter 应为 5，实际 %d", z.Counter())
	}
	// 跨 epoch：epoch 增大 → zxid 整体变大。
	z1 := MakeZXID(1, 100)
	z2 := MakeZXID(2, 1)
	if z2 <= z1 {
		t.Error("epoch 增大应使 zxid 整体变大")
	}
}

// TestZXIDString 验证 zxid 字符串形式。
func TestZXIDString(t *testing.T) {
	z := MakeZXID(3, 7)
	if s := z.String(); s != "3:7" {
		t.Errorf("zxid 字符串应为 3:7，实际 %s", s)
	}
}

// TestLeaderAssignsIncreasingZXID Leader 连续 Propose 应分配严格递增 zxid。
func TestLeaderAssignsIncreasingZXID(t *testing.T) {
	tr := core.NewMemTransport()
	l := NewLeader("L", []core.NodeID{"L", "a", "b"}, 1, tr)
	z1 := l.Propose("r1")
	z2 := l.Propose("r2")
	z3 := l.Propose("r3")
	if !(z1 < z2 && z2 < z3) {
		t.Errorf("zxid 应严格递增，实际 %s < %s < %s = %v", z1, z2, z3, z1 < z2 && z2 < z3)
	}
}

// TestQuorumCommit Leader 收 quorum Ack 后才 commit。
func TestQuorumCommit(t *testing.T) {
	tr := core.NewMemTransport()
	ids := []core.NodeID{"L", "a", "b"} // quorum = 2（含 L 自身 1 + 1 Follower）
	l := NewLeader("L", ids, 1, tr)
	l.Start()
	NewFollower("a", 1, tr).Start()
	NewFollower("b", 1, tr).Start()

	zxid := l.Propose("r1")
	for i := 0; i < 6; i++ {
		tr.Drain()
	}
	committed := l.CommittedZXIDs()
	if len(committed) != 1 || committed[0] != zxid {
		t.Errorf("Leader 应 commit %s，实际 %v", zxid, committed)
	}
}

// TestFollowerReceivesAll Follower 应收到全部 Proposal（记入日志）。
func TestFollowerReceivesAll(t *testing.T) {
	tr := core.NewMemTransport()
	ids := []core.NodeID{"L", "a", "b"}
	l := NewLeader("L", ids, 1, tr)
	l.Start()
	fa := NewFollower("a", 1, tr)
	fa.Start()
	NewFollower("b", 1, tr).Start()

	l.Propose("r1")
	l.Propose("r2")
	for i := 0; i < 10; i++ {
		tr.Drain()
	}
	if fa.LogLen() != 2 {
		t.Errorf("Follower a 应收到 2 个事务，实际 %d", fa.LogLen())
	}
}

// TestQuorumSize 验证 5 节点 quorum = 3。
func TestQuorumSize(t *testing.T) {
	tr := core.NewMemTransport()
	l := NewLeader("L", []core.NodeID{"L", "a", "b", "c", "d"}, 1, tr)
	if q := l.quorum(); q != 3 {
		t.Errorf("5 节点 quorum 应为 3，实际 %d", q)
	}
}
