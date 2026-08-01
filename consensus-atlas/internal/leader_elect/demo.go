package leader_elect

import (
	"context"
	"fmt"

	"github.com/QiuShichang/consensus-atlas/internal/core"
)

// DemoResult 是 Leader 选举 demo 的输出摘要。
type DemoResult struct {
	Algorithm   string // "bully+ring"
	BullyLeader uint64 // Bully 选举最终当选者 ID
	RingLeader  uint64 // Ring 选举最终当选者 ID
}

// Demo 演示 Bully 选举：5 节点 ID 1-5，节点 5（当前 Leader）离线，
// 节点 3 发现并发起选举，最终节点 4（次高，因为 5 已离线）当选。
//
// 随后另起一个 Ring 拓扑演示 Ring 选举：5 节点成环，选举后最大 ID 当选。
//
// 离线可跑（纯内存传输，确定性轨迹，无 goroutine/time/rand）。
func Demo(ctx context.Context) (*DemoResult, error) {
	_ = ctx

	bullyLeader, err := demoBully()
	if err != nil {
		return nil, err
	}
	ringLeader, err := demoRing()
	if err != nil {
		return nil, err
	}

	return &DemoResult{
		Algorithm:   "bully+ring",
		BullyLeader: bullyLeader,
		RingLeader:  ringLeader,
	}, nil
}

// drainUntilStable 反复 Drain 直到网络收敛（连续两轮无新消息被处理）。
// 这模拟"消息已充分传播"的稳态，便于在此基础上判定 Bully 的超时兜底。
func drainUntilStable(tr core.Transport) {
	for i := 0; i < 64; i++ { // 上限兜底，防异常死循环
		handled := tr.Drain()
		if len(handled) == 0 {
			return
		}
	}
}

// demoBully 演示 Bully：5 节点 ID 1-5，节点 5 离线，节点 3 发起选举 → 4 当选。
func demoBully() (uint64, error) {
	tr := core.NewMemTransport()
	ids := []uint64{1, 2, 3, 4, 5}
	nodes := make(map[uint64]*BullyNode, len(ids))
	for _, id := range ids {
		n := NewBullyNode(id, ids, tr)
		n.Start()
		nodes[id] = n
	}
	// 节点 5（最高 ID，原本的 Leader）离线。
	nodes[5].Online = false

	// 节点 3 发现 Leader 失联，发起选举。
	nodes[3].StartElection()

	RunBullyToCompletion(tr, nodes, ids)

	leader := nodes[4].Leader()
	if leader == 0 {
		return 0, fmt.Errorf("Bully 选举未产生 Leader")
	}
	return leader, nil
}

// demoRing 演示 Ring：5 节点成环 1→2→3→4→5→1，节点 1 发起，最大 ID（5）当选。
func demoRing() (uint64, error) {
	tr := core.NewMemTransport()
	// 环：1→2→3→4→5→1
	ring := []uint64{1, 2, 3, 4, 5}
	nodes := make(map[uint64]*RingNode, len(ring))
	for i, id := range ring {
		next := ring[(i+1)%len(ring)]
		n := NewRingNode(id, next, tr)
		n.Start()
		nodes[id] = n
	}

	// 节点 1 发起选举。
	nodes[1].StartElection()
	drainUntilStable(tr)

	leader := nodes[1].Leader()
	if leader == 0 {
		return 0, fmt.Errorf("Ring 选举未产生 Leader")
	}
	return leader, nil
}
