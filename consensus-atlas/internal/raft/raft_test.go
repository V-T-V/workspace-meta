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

// TestNetworkPartitionRecovery 验证 5 节点集群在网络分区下的提交与恢复：
//   - 选出 Leader 后提交 cmd1（全集群复制）
//   - 隔离 2 个 Follower（BlockNode），Leader 再提交 cmd2：quorum=3 仍够，故 cmd2 能 commit
//   - 隔离期间，被隔离的 2 个节点收不到 AppendEntries，日志落后于 cmd2
//   - 解除隔离（UnblockNode）并继续 tick+drain，Leader 心跳通过 AppendEntries 把它们同步到最新
//   - 最终 5 个节点日志完全一致
//
// 这是 Raft 最核心的容错性质（论文 §5.4.2 leader-commit + §6 对网络分区的容忍）。
func TestNetworkPartitionRecovery(t *testing.T) {
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

	// tickAll 推进一轮：所有节点 Tick 一次 + transport Drain 一次。
	tickAll := func() {
		for _, id := range ids {
			nodes[id].Tick()
		}
		tr.Drain()
	}

	// 第一阶段：选举，tick 到产生唯一 Leader。
	var leader *Node
	for i := 0; i < 30 && leader == nil; i++ {
		tickAll()
		leaderCnt := 0
		for _, id := range ids {
			if nodes[id].State == core.StateLeader {
				leader = nodes[id]
				leaderCnt++
			}
		}
		if leaderCnt > 1 {
			t.Fatalf("同一任期不应有多个 Leader，实际 %d 个", leaderCnt)
		}
	}
	if leader == nil {
		t.Fatalf("选举超时未产生 Leader")
	}
	t.Logf("Leader 当选: %s, term=%d", leader.ID, leader.CurrentTerm)

	// 第二阶段：Leader 提交 cmd1，多轮 drain 让全集群复制。
	if !leader.Propose("cmd1") {
		t.Fatalf("Leader %s Propose cmd1 失败", leader.ID)
	}
	for i := 0; i < 10; i++ {
		tickAll()
	}
	if leader.CommitIndex != 1 {
		t.Fatalf("cmd1 后 Leader CommitIndex 应为 1，实际 %d", leader.CommitIndex)
	}
	for _, id := range ids {
		if nodes[id].Log.LastIndex() != 1 {
			t.Fatalf("节点 %s 日志未复制 cmd1，LastIndex=%d", id, nodes[id].Log.LastIndex())
		}
	}

	// 第三阶段：网络分区——隔离 2 个非 Leader 的 Follower。
	var isolated []core.NodeID
	for _, id := range ids {
		if id != leader.ID && len(isolated) < 2 {
			isolated = append(isolated, id)
		}
	}
	if len(isolated) != 2 {
		t.Fatalf("应选出 2 个隔离 Follower，实际 %d", len(isolated))
	}
	for _, id := range isolated {
		tr.BlockNode(id)
	}
	t.Logf("隔离节点: %v（Leader=%s 仍在多数派侧）", isolated, leader.ID)

	// 第四阶段：分区中 Leader 提交 cmd2。只有 Leader + 另外 2 个未隔离 Follower
	// 能收到 AppendEntries，共 3 个节点，正好达 quorum=3，故能 commit。
	if !leader.Propose("cmd2") {
		t.Fatalf("Leader %s Propose cmd2 失败", leader.ID)
	}
	// 只 tick 未隔离的节点（隔离的不动，避免其选举时钟推进引发干扰）。
	for i := 0; i < 10; i++ {
		for _, id := range ids {
			if !tr.IsBlocked(id) {
				nodes[id].Tick()
			}
		}
		tr.Drain()
	}
	if leader.CommitIndex != 2 {
		t.Fatalf("分区中 cmd2 应能 commit（quorum=3），Leader CommitIndex 应为 2，实际 %d",
			leader.CommitIndex)
	}

	// 断言：被隔离的 2 个节点确实落后（只有 cmd1，没有 cmd2）。
	for _, id := range isolated {
		if li := nodes[id].Log.LastIndex(); li != 1 {
			t.Errorf("隔离节点 %s 应停留在 cmd1（LastIndex=1），实际 %d", id, li)
		}
		if nodes[id].CommitIndex != 1 {
			t.Errorf("隔离节点 %s CommitIndex 应仍为 1，实际 %d", id, nodes[id].CommitIndex)
		}
	}

	// 第五阶段：恢复分区。
	for _, id := range isolated {
		tr.UnblockNode(id)
	}
	t.Logf("恢复分区，重新同步 %v", isolated)

	// 第六阶段：继续 tick+drain，Leader 心跳通过 AppendEntries 把落后节点拉到最新。
	synced := false
	for i := 0; i < 40 && !synced; i++ {
		tickAll()
		synced = true
		for _, id := range ids {
			if nodes[id].Log.LastIndex() != 2 || nodes[id].CommitIndex != 2 {
				synced = false
				break
			}
		}
	}

	// 最终断言：5 个节点日志完全一致（长度 + 每条 Entry 的 Term/Command）。
	for _, id := range ids {
		if li := nodes[id].Log.LastIndex(); li != 2 {
			t.Errorf("恢复后节点 %s 应已同步到 cmd2（LastIndex=2），实际 %d", id, li)
		}
		if nodes[id].CommitIndex != 2 {
			t.Errorf("恢复后节点 %s CommitIndex 应为 2，实际 %d", id, nodes[id].CommitIndex)
		}
	}
	leaderLog := leader.Log.Entries
	for _, id := range ids {
		got := nodes[id].Log.Entries
		if len(got) != len(leaderLog) {
			t.Errorf("恢复后节点 %s 日志长度 %d != Leader %d", id, len(got), len(leaderLog))
			continue
		}
		for i := range got {
			if got[i].Term != leaderLog[i].Term || got[i].Command != leaderLog[i].Command {
				t.Errorf("恢复后节点 %s 日志 Entry[%d] 不一致: got %+v want %+v",
					id, i, got[i], leaderLog[i])
			}
		}
	}
	if !synced {
		t.Error("40 轮 tick+drain 后仍有节点未同步到 cmd2")
	}
}
