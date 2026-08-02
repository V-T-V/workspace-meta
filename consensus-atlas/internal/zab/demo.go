package zab

import (
	"context"
	"fmt"

	"github.com/QiuShichang/consensus-atlas/internal/core"
)

// DemoResult 是 ZAB demo 的输出摘要。
type DemoResult struct {
	Followers         int                 // follower 数
	Epoch             uint32              // 当前纪元
	ProposedZXIDs     []string            // Leader 分配的 zxid（按请求顺序）
	CommittedZXIDs    []string            // Leader 已 commit 的 zxid（按提交顺序）
	FollowerCommitted map[string][]string // 各 Follower 已 commit 的 zxid（验证按序复制）
	Ordered           bool                // zxid 是否全局单调递增
}

// Demo 用 1 Leader + 3 Follower（共 4 节点）演示 ZAB 广播阶段：
//
//	Leader 连续处理 3 个客户端请求，每个请求：
//	  分配 zxid → 广播 Proposal → 收 quorum Ack → 广播 Commit。
//	验证：
//	 1. zxid 全局单调递增（同 epoch 内 counter 递增）。
//	 2. Leader 与各 Follower 的 commit 顺序一致（按 zxid 顺序）。
//	 3. 每个 Follower 都收到了全部 3 个事务。
//
// 离线可跑（MemTransport + 显式 Drain，确定性轨迹，无 goroutine/time/rand）。
func Demo(ctx context.Context) (*DemoResult, error) {
	_ = ctx
	tr := core.NewMemTransport()

	const epoch uint32 = 1
	leaderID := core.NodeID("leader")
	followerIDs := []core.NodeID{"f1", "f2", "f3"}
	allPeers := append([]core.NodeID{leaderID}, followerIDs...)

	leader := NewLeader(leaderID, allPeers, epoch, tr)
	leader.Start()

	followers := make(map[core.NodeID]*Follower, len(followerIDs))
	for _, id := range followerIDs {
		f := NewFollower(id, epoch, tr)
		f.Start()
		followers[id] = f
	}

	// 连续提交 3 个请求。
	requests := []string{"create /app", "set /app v1", "delete /old"}
	var proposed []ZXID
	for _, req := range requests {
		zxid := leader.Propose(req)
		proposed = append(proposed, zxid)
		// Drain 推进 Proposal → Ack → Commit（每个请求约 2 轮）。
		for i := 0; i < 6; i++ {
			tr.Drain()
		}
	}

	// 兜底：再 Drain 几轮确保最后的 Commit 全部投递。
	for i := 0; i < 6; i++ {
		tr.Drain()
	}

	// 验证 zxid 单调递增。
	ordered := true
	for i := 1; i < len(proposed); i++ {
		if proposed[i] <= proposed[i-1] {
			ordered = false
			break
		}
	}

	committed := leader.CommittedZXIDs()
	if len(committed) != len(requests) {
		return nil, fmt.Errorf("Leader 应 commit %d 个，实际 %d", len(requests), len(committed))
	}

	res := &DemoResult{
		Followers:         len(followerIDs),
		Epoch:             epoch,
		Ordered:           ordered,
		FollowerCommitted: make(map[string][]string),
	}
	for _, z := range proposed {
		res.ProposedZXIDs = append(res.ProposedZXIDs, z.String())
	}
	for _, z := range committed {
		res.CommittedZXIDs = append(res.CommittedZXIDs, z.String())
	}
	for _, id := range followerIDs {
		fc := followers[id].CommittedZXIDs()
		outs := make([]string, len(fc))
		for i, z := range fc {
			outs[i] = z.String()
		}
		res.FollowerCommitted[string(id)] = outs
	}

	return res, nil
}
