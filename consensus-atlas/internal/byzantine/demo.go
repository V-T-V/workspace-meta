package byzantine

import (
	"context"

	"github.com/QiuShichang/consensus-atlas/internal/core"
)

// DemoResult 是 PBFT demo 的输出摘要。
type DemoResult struct {
	Committed bool   // 请求是否最终 committed
	Sequence  uint64 // 已提交的序列号
	View      uint64 // 当前视图
	Replicas  int    // 集群规模
	PrimaryID core.NodeID
	Prepared  int // 进入 prepared 阶段的 replica 数
	Commits   int // 进入 committed 阶段的 replica 数

	// 拜占庭场景（场景二）的输出：1 个叛徒 replica 静默故障时，诚实节点是否仍达成一致。
	ByzCommitted bool // 诚实节点是否最终 committed（尽管有 1 拜占庭）
	ByzHonestNum int  // 诚实 replica 数（应 = 集群 - 拜占庭数）
	ByzHonestOK  int  // 进入 committed 的诚实 replica 数（应 = ByzHonestNum）
	ByzTraitorID core.NodeID
	ByzTraitorOK bool // 叛徒自身是否 committed（应 false，PBFT 不保证拜占庭节点状态）
}

// Demo 用 4 节点 PBFT（1 Primary + 3 Replica）演示完整三阶段共识。
//
// 配置：n=4 → f=1 → quorum=2f+1=3，可容忍 1 个拜占庭节点。
//
// 演示两个场景：
//
//  1. 诚实集群（happy path）：4 个节点全部诚实，请求最终在全部 4 个 replica committed。
//  2. 拜占庭场景：把 r3 设为叛徒（IsTraitor=true），它在 prepare/commit 阶段静默故障
//     （不广播自己的票、不累计收到的票）。验证其余 3 个诚实 replica 仍能 committed——
//     这正是 PBFT 的核心保证（n=3f+1 容忍 f 个拜占庭，quorum=2f+1）。
//
// 流程（每场景）：
//  1. 启动 4 个 replica，r0 为 view 0 的 Primary。
//  2. Primary 对序列号 1 提议请求。
//  3. Drain 推进网络 tick：pre-prepare → prepare → commit 三阶段消息依次投递。
//  4. 收齐 3 个 prepare 进入 prepared，再收齐 3 个 commit 进入 committed。
//
// 离线可跑（纯内存传输，确定性轨迹，无 goroutine/time/rand）。
func Demo(ctx context.Context) (*DemoResult, error) {
	_ = ctx

	// 场景一：4 个诚实节点（happy path）。
	honestPrepared, honestCommits, _ := runScenario(ctx, false)

	// 场景二：1 个叛徒（r3 静默故障），验证诚实节点仍 committed。
	_, byzCommits, traitorCommitted := runScenario(ctx, true)

	return &DemoResult{
		// 场景一（诚实集群）。
		Committed: honestCommits > 0,
		Sequence:  1,
		View:      0,
		Replicas:  4,
		PrimaryID: "r0",
		Prepared:  honestPrepared,
		Commits:   honestCommits,
		// 场景二（拜占庭）：诚实 replica 数 = 4 - 1 = 3，全部应 committed。
		ByzCommitted: byzCommits == 3,
		ByzHonestNum: 3,
		ByzHonestOK:  byzCommits,
		ByzTraitorID: "r3",
		ByzTraitorOK: traitorCommitted,
	}, nil
}

// runScenario 跑一次 4 节点 PBFT 共识，返回进入 prepared/committed 的 replica 数。
// withTraitor=true 时把 r3 设为拜占庭叛徒（静默故障）。
// 返回：（prepared 数, committed 数, 叛徒 r3 是否 committed）。
func runScenario(_ context.Context, withTraitor bool) (prepared, commits int, traitorCommitted bool) {
	tr := core.NewMemTransport()
	ids := []core.NodeID{"r0", "r1", "r2", "r3"}
	replicas := make(map[core.NodeID]*Replica, len(ids))
	for i, id := range ids {
		r := NewReplica(id, ids, i == 0, tr)
		if withTraitor && id == "r3" {
			r.IsTraitor = true
		}
		r.Start()
		replicas[id] = r
	}

	primary := replicas["r0"]
	_ = primary.Propose(Request{Op: "op", Client: "c1"})

	// 推进网络：三阶段消息在多轮 Drain 中传播。多 drain 几轮兜底。
	for i := 0; i < 10; i++ {
		tr.Drain()
	}

	for _, id := range ids {
		if replicas[id].IsPrepared(1) {
			prepared++
		}
		if replicas[id].IsCommitted(1) {
			commits++
		}
	}
	return prepared, commits, replicas["r3"].IsCommitted(1)
}
