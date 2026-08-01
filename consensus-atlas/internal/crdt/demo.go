package crdt

import (
	"context"

	"github.com/QiuShichang/consensus-atlas/internal/core"
)

// DemoResult 是 G-Counter CRDT demo 的输出摘要。
type DemoResult struct {
	NodeCount int // 参与的节点数
	// 每个节点各自的本地增量（演示并发、互不协调的写入）。
	LocalInc map[core.NodeID]uint64
	// 收敛后每个节点的 Value()——CRDT 的不变量：所有副本 Value 相同。
	FinalValue map[core.NodeID]uint64
	// 期望收敛值 = 所有节点本地增量之和。
	Expected uint64
	// 是否收敛（所有节点 Value 相同且等于 Expected）。
	Converged bool
	// 实际跑了多少轮交换。
	Rounds int
}

// Demo 用 3 个节点演示 G-Counter CRDT 的最终收敛：
//  1. 3 个节点各自 Increment 不同值（n1:+3, n2:+5, n3:+2），互不协调。
//  2. 反复跑交换轮次：每一轮所有节点 Tick 一次（各 Push 向量给一个邻居），
//     再 Drain 推进网络，对端 Merge 后 Pull 回来。
//  3. 当所有节点的 Value() 都相同（且等于 3+5+2=10）时认为收敛。
//
// 确定性：选邻居用 round-robin（无 rand），max 合并幂等——同一份代码每次跑
// 收敛值与轮数都相同。离线可跑（纯内存传输）。
func Demo(ctx context.Context) (*DemoResult, error) {
	_ = ctx
	tr := core.NewMemTransport()

	ids := []core.NodeID{"n1", "n2", "n3"}
	// 各节点并发的、互不协调的本地增量。
	inc := map[core.NodeID]uint64{"n1": 3, "n2": 5, "n3": 2}

	nodes := make(map[core.NodeID]*Node, len(ids))
	for _, id := range ids {
		n := NewNode(id, ids, tr)
		n.Start()
		nodes[id] = n
	}
	// 各自 Increment（并发写入，无协调）。
	for _, id := range ids {
		nodes[id].Counter.Increment(inc[id])
	}

	// 期望收敛值 = 所有本地增量之和。
	var expected uint64
	for _, v := range inc {
		expected += v
	}

	const maxRounds = 30 // 兜底；3 节点 round-robin 远不到这里就该收敛
	converged := false
	rounds := 0
	for r := 1; r <= maxRounds; r++ {
		rounds = r
		for _, id := range ids {
			nodes[id].Tick()
		}
		// 多 Drain 几次保证本轮 Request + Response 往返都被处理。
		for d := 0; d < 4; d++ {
			tr.Drain()
		}
		if allValuesEqual(nodes, ids, expected) {
			converged = true
			break
		}
	}

	final := make(map[core.NodeID]uint64, len(ids))
	for _, id := range ids {
		final[id] = nodes[id].Counter.Value()
	}

	return &DemoResult{
		NodeCount:  len(ids),
		LocalInc:   inc,
		FinalValue: final,
		Expected:   expected,
		Converged:  converged,
		Rounds:     rounds,
	}, nil
}

// allValuesEqual 检查所有节点的 Value() 是否都等于 want（收敛不变量）。
func allValuesEqual(nodes map[core.NodeID]*Node, ids []core.NodeID, want uint64) bool {
	for _, id := range ids {
		if nodes[id].Counter.Value() != want {
			return false
		}
	}
	return true
}
