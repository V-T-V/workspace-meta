package raft

import (
	"context"
	"testing"

	"github.com/QiuShichang/consensus-atlas/internal/core"
)

// TestDemoRuns 验证 demo 离线可跑且产生合法 Leader。
func TestDemoRuns(t *testing.T) {
	res, err := Demo(context.Background())
	if err != nil {
		t.Fatalf("Demo 失败: %v", err)
	}
	if res.LeaderID == "" {
		t.Error("未选出 Leader")
	}
	if res.Term == 0 {
		t.Error("任期应为 >= 1")
	}
	if res.LogLen != 1 {
		t.Errorf("Leader 日志应有 1 条提交，实际 %d", res.LogLen)
	}
	if !res.Replicated {
		t.Error("命令应已复制到多数派")
	}
	if res.CommitIndex != 1 {
		t.Errorf("commitIndex 应为 1，实际 %d", res.CommitIndex)
	}
}

// TestLogUpToDate 验证"候选人日志至少与自己一样新"的判定规则。
func TestLogUpToDate(t *testing.T) {
	tr := core.NewMemTransport()
	n := NewNode("a", []core.NodeID{"a"}, 5, tr)
	// 空 vs 空：相同 term，index 相等 → up-to-date
	if !n.logUpToDate(0, 0) {
		t.Error("空日志 vs 空日志应 up-to-date")
	}
	// 候选人 term 更高 → up-to-date
	n.Log.Append(2, "x")
	if !n.logUpToDate(3, 0) {
		t.Error("更高 term 应判定 up-to-date")
	}
	// 候选人 term 更低 → not
	if n.logUpToDate(1, 5) {
		t.Error("更低 term 应判定 not up-to-date")
	}
	// 同 term，候选人 index 更小 → not
	if n.logUpToDate(2, 0) {
		t.Error("同 term 更短日志应判定 not up-to-date")
	}
}

// TestQuorum 验证 5 节点 quorum = 3。
func TestQuorum(t *testing.T) {
	tr := core.NewMemTransport()
	n := NewNode("a", []core.NodeID{"a", "b", "c", "d", "e"}, 5, tr)
	if q := n.quorum(); q != 3 {
		t.Errorf("5 节点 quorum 应为 3，实际 %d", q)
	}
}

// TestProposeNonLeader 非 Leader 提交应失败。
func TestProposeNonLeader(t *testing.T) {
	tr := core.NewMemTransport()
	n := NewNode("a", []core.NodeID{"a", "b"}, 5, tr)
	n.Start()
	if n.Propose("x") {
		t.Error("Follower 不应能 Propose")
	}
}

// TestSingleElection 单节点集群超时后应当选（自投 1 票即达 quorum=1）。
func TestSingleElection(t *testing.T) {
	tr := core.NewMemTransport()
	n := NewNode("solo", []core.NodeID{"solo"}, 3, tr)
	n.Start()
	// tick 到选举超时（timeout=3）。
	for i := 0; i < 3; i++ {
		n.Tick()
	}
	if n.State != core.StateLeader {
		t.Errorf("单节点超时后应当选 Leader，实际 %s", n.State)
	}
}
