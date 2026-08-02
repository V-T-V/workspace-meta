package gossip

import (
	"context"

	"github.com/QiuShichang/consensus-atlas/internal/core"
)

// DemoResult 是 Gossip demo 的输出摘要。
type DemoResult struct {
	NodeCount  int               // 参与的节点数
	Rounds     int               // 实际跑了多少轮 Gossip（每轮 = 所有节点 Tick 一次 + Drain）
	Converged  bool              // 所有节点状态是否已收敛到一致
	FinalState map[string]string // 收敛后的状态（任一节点的快照；未收敛时为最后一个节点的状态）
}

// 期望的最终状态：5 个节点各持有一个不同的键，收敛后每人都应拿到全 5 个键。
var wantState = map[string]string{
	"a": "1", "b": "2", "c": "3", "d": "4", "e": "5",
}

// Demo 用 5 节点集群演示 Push-Pull Gossip 的最终一致性：
//  1. 启动 5 个节点，每个初始持有一个不同的键值（n1:{a:1}, n2:{b:2}, ... n5:{e:5}）。
//  2. 反复跑 Gossip 轮次：每一轮所有节点 Tick 一次（各挑一个邻居 Push），再 Drain 推进网络。
//  3. 当所有节点状态都等于 wantState 时认为收敛，记录所用的轮数。
//
// 因为选邻居用 round-robin（而非 rand），整个轨迹是确定性的：同一份代码每次跑
// 收敛轮数相同。离线可跑（纯内存传输）。
func Demo(ctx context.Context) (*DemoResult, error) {
	_ = ctx
	tr := core.NewMemTransport()

	ids := []core.NodeID{"n1", "n2", "n3", "n4", "n5"}
	// 每个节点初始持有一个互不相同的键值。
	seed := map[core.NodeID]map[string]string{
		"n1": {"a": "1"},
		"n2": {"b": "2"},
		"n3": {"c": "3"},
		"n4": {"d": "4"},
		"n5": {"e": "5"},
	}

	nodes := make(map[core.NodeID]*Node, len(ids))
	for _, id := range ids {
		n := NewNode(id, ids, tr)
		for k, v := range seed[id] {
			n.Set(k, v)
		}
		n.Start()
		nodes[id] = n
	}

	const maxRounds = 50 // 兜底上限，5 节点 round-robin 远不到这里就该收敛
	converged := false
	rounds := 0
	for r := 1; r <= maxRounds; r++ {
		rounds = r
		// 每个节点 Tick 一次：各挑一个邻居 Push 自己全量状态。
		for _, id := range ids {
			nodes[id].Tick()
		}
		// Drain 推进网络：投递 Request、处理、把 Response 重新入队、再投递。
		// 这里多 drain 几次保证本轮的 Request + Response 往返都被处理完。
		for d := 0; d < 4; d++ {
			tr.Drain()
		}
		if allConverged(nodes, ids, wantState) {
			converged = true
			break
		}
	}

	final := nodes[ids[len(ids)-1]].snapshot()
	return &DemoResult{
		NodeCount:  len(ids),
		Rounds:     rounds,
		Converged:  converged,
		FinalState: final,
	}, nil
}

// allConverged 检查所有节点的状态是否都已等于 want。
func allConverged(nodes map[core.NodeID]*Node, ids []core.NodeID, want map[string]string) bool {
	for _, id := range ids {
		if !statesEqual(nodes[id].State, want) {
			return false
		}
	}
	return true
}

// statesEqual 判断 a 是否与 b 拥有完全相同的键值集合。
func statesEqual(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range b {
		if av, ok := a[k]; !ok || av != v {
			return false
		}
	}
	return true
}
