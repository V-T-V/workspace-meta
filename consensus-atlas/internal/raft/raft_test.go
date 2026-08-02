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

// TestSplitBrain 验证脑裂（网络分区导致双 Leader 候选）下的 Raft 安全性：
//
//	5 节点集群，先选出 Leader L1（低 term）。
//	人为分区：L1 + 1 follower（少数派，2 节点 < quorum=3）vs 3 followers（多数派）。
//	多数派侧收不到 L1 心跳 → 选举超时 → 选出新 Leader L2（更高 term）。
//
// 断言：
//  1. 分区期间，L1（少数派）侧 Propose 无法 commit（matchIndex 凑不齐 quorum）。
//  2. 分区期间，L2（多数派）侧 Propose 能 commit。
//  3. 恢复分区后，L2 因 term 更高成为唯一 Leader（L1 降级为 Follower）。
//
// 这正是 Raft 的核心安全保证（论文 §5.4.1, §6）：任一 term 内最多一个 Leader
// 能提交，且已提交的数据不会被"分区恢复"推翻。
func TestSplitBrain(t *testing.T) {
	tr := core.NewMemTransport()

	ids := []core.NodeID{"n1", "n2", "n3", "n4", "n5"}
	// 不同选举超时（5/7/9/11/13 ticks），n1 最先觉醒当 Leader（L1）。
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
	// tickPartition 只推进分区活跃侧的节点（被 BlockNode 隔离的不 Tick）。
	tickPartition := func() {
		for _, id := range ids {
			if !tr.IsBlocked(id) {
				nodes[id].Tick()
			}
		}
		tr.Drain()
	}

	// 第一阶段：选出初始 Leader L1（应为 n1，超时最小）。
	for i := 0; i < 30; i++ {
		tickAll()
	}
	var l1 *Node
	leaderCnt := 0
	for _, id := range ids {
		if nodes[id].State == core.StateLeader {
			l1 = nodes[id]
			leaderCnt++
		}
	}
	if leaderCnt != 1 || l1 == nil {
		t.Fatalf("应选出唯一 L1，实际 %d 个 Leader", leaderCnt)
	}
	if l1.ID != "n1" {
		t.Fatalf("L1 应为 n1（最小选举超时），实际 %s", l1.ID)
	}
	l1Term := l1.CurrentTerm
	t.Logf("L1 当选: %s, term=%d", l1.ID, l1Term)

	// 第二阶段：网络分区——把 L1 和一个 follower（n2）隔离成少数派。
	// BlockNode 让 From/To 含被隔离节点的消息全部丢失，等价于 n1↔{n3,n4,n5}
	// 和 n2↔{n3,n4,n5} 全断；只剩多数派 {n3,n4,n5} 内部互通。
	tr.BlockNode("n1")
	tr.BlockNode("n2")
	t.Logf("分区：少数派 {n1,n2}（含旧 Leader L1） vs 多数派 {n3,n4,n5}")

	// 第三阶段：多数派侧收不到 L1 心跳 → 选举超时 → 选出新 Leader L2。
	// n3 的选举超时（9 ticks）最小，故 n3 最先觉醒。给它足够 tick 让选举完成。
	var l2 *Node
	for i := 0; i < 40 && l2 == nil; i++ {
		tickPartition()
		for _, id := range ids {
			if id == "n1" || id == "n2" {
				continue // 少数派侧不算
			}
			if nodes[id].State == core.StateLeader {
				l2 = nodes[id]
			}
		}
	}
	if l2 == nil {
		t.Fatalf("多数派侧应选出新 Leader L2")
	}
	if l2.CurrentTerm <= l1Term {
		t.Fatalf("L2 term 应高于 L1（%d），实际 %d", l1Term, l2.CurrentTerm)
	}
	t.Logf("L2 当选: %s, term=%d", l2.ID, l2.CurrentTerm)

	// 第四阶段：L1（少数派）侧 Propose 应无法 commit（quorum=3 不够）。
	// L1 提议后即便多轮 tick，CommitIndex 不应前进。
	l1CommitBefore := l1.CommitIndex
	if !l1.Propose("l1-cmd") {
		t.Fatalf("L1 Propose 应被接受（Leader 身份仍在）")
	}
	for i := 0; i < 15; i++ {
		tickPartition() // L1 侧的 tick 让它继续尝试复制，但消息都被分区丢弃
	}
	if l1.CommitIndex != l1CommitBefore {
		t.Errorf("L1 少数派侧不应能 commit（quorum 不够），CommitIndex %d→%d",
			l1CommitBefore, l1.CommitIndex)
	}
	t.Logf("L1 侧 commit 卡在 %d（少数派无法提交，符合预期）", l1.CommitIndex)

	// 第五阶段：L2（多数派）侧 Propose 应能 commit（quorum=3 达到）。
	l2CommitBefore := l2.CommitIndex
	if !l2.Propose("l2-cmd") {
		t.Fatalf("L2 Propose 应被接受")
	}
	for i := 0; i < 15; i++ {
		tickPartition()
	}
	if l2.CommitIndex <= l2CommitBefore {
		t.Fatalf("L2 多数派侧应能 commit，CommitIndex 未前进（%d）", l2.CommitIndex)
	}
	t.Logf("L2 侧 commit 推进到 %d（多数派正常提交）", l2.CommitIndex)

	// 第六阶段：恢复分区。
	tr.UnblockNode("n1")
	tr.UnblockNode("n2")
	t.Logf("分区恢复，让 L2 的高 term 心跳覆盖 L1")

	// 多轮 tick+drain 让 L2 的 AppendEntries 到达 L1，迫使 L1 降级。
	for i := 0; i < 40; i++ {
		tickAll()
	}

	// 第七阶段：最终唯一 Leader 是 L2（term 更高），L1 降为 Follower。
	finalLeaders := []*Node{}
	for _, id := range ids {
		if nodes[id].State == core.StateLeader {
			finalLeaders = append(finalLeaders, nodes[id])
		}
	}
	if len(finalLeaders) != 1 {
		var ids []string
		for _, n := range finalLeaders {
			ids = append(ids, string(n.ID))
		}
		t.Fatalf("恢复后应唯一 Leader，实际 %d 个: %v", len(finalLeaders), ids)
	}
	onlyLeader := finalLeaders[0]
	if onlyLeader.ID != l2.ID {
		t.Errorf("唯一 Leader 应为 L2(%s)，实际 %s", l2.ID, onlyLeader.ID)
	}
	if onlyLeader.CurrentTerm < l2.CurrentTerm {
		t.Errorf("Leader term 不应低于 L2 的 %d，实际 %d", l2.CurrentTerm, onlyLeader.CurrentTerm)
	}
	if l1.State != core.StateFollower {
		t.Errorf("L1(%s) 应降级为 Follower，实际 %s", l1.ID, l1.State)
	}
	if l1.CurrentTerm < onlyLeader.CurrentTerm {
		t.Errorf("L1 term 应已被拉齐到 %d，实际 %d", onlyLeader.CurrentTerm, l1.CurrentTerm)
	}
	t.Logf("最终唯一 Leader: %s term=%d（L1 已降级为 %s）",
		onlyLeader.ID, onlyLeader.CurrentTerm, l1.State)
}

// TestMemberChange 验证"新节点动态加入集群后能通过 AppendEntries 追上 Leader 日志"。
//
// 这是成员变更的最简形态（不涉及 Raft joint consensus / C_old-new 两阶段协议）：
//   - 5 节点集群正常选出 Leader 并提交若干命令
//   - 动态构造第 6 个节点 n6：注册到 transport（Install）、加入 Leader 的 Peers、
//     初始化 Leader 对 n6 的 nextIndex=1（让 Leader 从头补发完整日志）
//   - 继续推进 tick + drain，Leader 的心跳/AppendEntries 把已有日志同步给 n6
//   - 断言 n6 最终日志与 Leader 完全一致，且 CommitIndex 也被拉齐
//
// 注：这里只改 Leader 视角的成员视图，老节点（n2-n5）的 Peers 仍是 5 节点，
// 但它们只对 Leader 的 AppendEntries 做被动响应，不影响 n6 的追赶；
// n6 设极大选举超时，保证它在追上之前不会自行发起选举干扰。
func TestMemberChange(t *testing.T) {
	tr := core.NewMemTransport()

	ids := []core.NodeID{"n1", "n2", "n3", "n4", "n5"}
	// 不同选举超时（5/7/9/11/13 ticks），n1 最先觉醒当 Leader。
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

	// 第一阶段：选出 Leader。
	for i := 0; i < 30; i++ {
		tickAll()
	}
	var leader *Node
	leaderCnt := 0
	for _, id := range ids {
		if nodes[id].State == core.StateLeader {
			leader = nodes[id]
			leaderCnt++
		}
	}
	if leaderCnt != 1 || leader == nil {
		t.Fatalf("应选出唯一 Leader，实际 %d 个", leaderCnt)
	}
	t.Logf("Leader 当选: %s, term=%d", leader.ID, leader.CurrentTerm)

	// 第二阶段：Leader 提交 cmd1、cmd2，并让全集群复制、commit。
	if !leader.Propose("cmd1") {
		t.Fatalf("Propose cmd1 失败")
	}
	for i := 0; i < 10; i++ {
		tickAll()
	}
	if !leader.Propose("cmd2") {
		t.Fatalf("Propose cmd2 失败")
	}
	for i := 0; i < 10; i++ {
		tickAll()
	}
	if leader.CommitIndex != 2 {
		t.Fatalf("Leader CommitIndex 应为 2，实际 %d", leader.CommitIndex)
	}
	// 记下 Leader 日志快照，供最后逐条比对。
	leaderSnapshot := append([]core.LogEntry(nil), leader.Log.Entries...)
	t.Logf("加入 n6 前 Leader 日志长度=%d, CommitIndex=%d", len(leaderSnapshot), leader.CommitIndex)

	// 第三阶段：构造第 6 个节点 n6 并加入集群。
	//   - NewNode 时 Peers 给全 6 节点（反映真实成员视图）
	//   - 极大选举超时，避免 n6 在追上前自行发起选举
	//   - Install 到 transport，使其能收发消息
	//   - 把 n6 加进 Leader 的 Peers，并初始化 nextIndex=1（从头补发完整日志）
	const newID core.NodeID = "n6"
	n6 := NewNode(newID, append(append([]core.NodeID{}, ids...), newID), 10_000, tr)
	n6.Start()
	nodes[newID] = n6

	leader.Peers = append(leader.Peers, newID)
	leader.nextIndex[newID] = 1  // 让 Leader 从头补发（n6 当前日志为空）
	leader.matchIndex[newID] = 0 // 尚未确认任何条目
	t.Logf("n6 已加入集群（Leader.Peers=%v，nextIndex[n6]=1）", leader.Peers)

	// tickAll6 推进所有 6 个节点。
	allIDs := append(append([]core.NodeID{}, ids...), newID)
	tickAll6 := func() {
		for _, id := range allIDs {
			nodes[id].Tick()
		}
		tr.Drain()
	}

	// 第四阶段：Leader 广播 AppendEntries（含完整日志）给 n6，n6 追加并回 MatchIndex。
	// 给足轮次让补发 + 回复往返完成。
	//
	// 注意网络时序：n6 在收到 AppendEntries 当轮就追加日志（LastIndex 立即对齐），
	// 但它回复的 AppendEntriesResponse 要到"下一轮 Drain"才送回 Leader——
	// 所以 Leader.matchIndex[n6] 比 n6 自身日志晚一轮推进。
	// 同步条件必须等 Leader 也确认（matchIndex 拉齐），否则会误判过早结束。
	synced := false
	for i := 0; i < 40 && !synced; i++ {
		tickAll6()
		// 同时满足：n6 日志对齐 + Leader 已收到 n6 的确认（matchIndex 拉齐）。
		if n6.Log.LastIndex() == leader.Log.LastIndex() &&
			leader.matchIndex[newID] == leader.Log.LastIndex() {
			synced = true
		}
	}

	// 第五阶段：断言 n6 日志与 Leader 完全一致（长度 + 每条 Term/Command）。
	if n6.Log.LastIndex() != leader.Log.LastIndex() {
		t.Fatalf("n6 未追上：LastIndex n6=%d Leader=%d", n6.Log.LastIndex(), leader.Log.LastIndex())
	}
	if len(n6.Log.Entries) != len(leader.Log.Entries) {
		t.Fatalf("n6 日志长度 %d != Leader %d", len(n6.Log.Entries), len(leader.Log.Entries))
	}
	for i := range n6.Log.Entries {
		got := n6.Log.Entries[i]
		want := leader.Log.Entries[i]
		if got.Term != want.Term || got.Command != want.Command || got.Index != want.Index {
			t.Errorf("n6 日志 Entry[%d] 不一致: got %+v want %+v", i, got, want)
		}
	}
	// Leader 维护的 matchIndex[n6] 也应已推进到最后一条。
	if leader.matchIndex[newID] != leader.Log.LastIndex() {
		t.Errorf("Leader.matchIndex[n6] 应为 %d，实际 %d",
			leader.Log.LastIndex(), leader.matchIndex[newID])
	}
	// n6 的 CommitIndex 应被 Leader 的 LeaderCommit 拉齐（<= Leader，因 n6 追上了全部已提交条目）。
	if n6.CommitIndex < leader.CommitIndex {
		t.Errorf("n6 CommitIndex 应 >= Leader 的 %d，实际 %d", leader.CommitIndex, n6.CommitIndex)
	}
	if !synced {
		t.Error("n6 在 40 轮 tick+drain 后仍未追上 Leader 日志")
	}
	t.Logf("n6 已追上 Leader：日志长度=%d, CommitIndex=%d", n6.Log.LastIndex(), n6.CommitIndex)

	// 第六阶段：加入 n6 后，Leader 再提一条新命令 cmd3，n6 也应被同步。
	if !leader.Propose("cmd3") {
		t.Fatalf("Propose cmd3 失败")
	}
	synced3 := false
	for i := 0; i < 40 && !synced3; i++ {
		tickAll6()
		if n6.Log.LastIndex() == leader.Log.LastIndex() {
			synced3 = true
		}
	}
	if n6.Log.LastIndex() != 3 {
		t.Errorf("cmd3 后 n6 LastIndex 应为 3，实际 %d", n6.Log.LastIndex())
	}
	last, ok := n6.Log.At(3)
	if !ok || last.Command != "cmd3" {
		t.Errorf("n6 日志第 3 条应为 cmd3，实际 %+v", last)
	}
	if !synced3 {
		t.Error("n6 未同步到加入后新增的 cmd3")
	}
	t.Logf("新命令 cmd3 也已复制到 n6：LastIndex=%d", n6.Log.LastIndex())
}
