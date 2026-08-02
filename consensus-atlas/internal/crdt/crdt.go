package crdt

import (
	"github.com/QiuShichang/consensus-atlas/internal/core"
)

// 消息 Kind 常量。与 gossip 包对齐，用"请求/响应"两阶段交换 G-Counter 的全量
// 向量，对应一次双向同步（Push-Pull）：
//   - StateRequest  由发起方发出，携带自己当前的向量（Push 自己的计数）。
//   - StateResponse 由接收方在合并后回送，携带自己合并后的向量（Pull）。
//
// 两阶段共用同一种负载（都是完整向量），合并规则对两边一致。
//
// 之所以区分请求/响应（而非都用 KindState）：响应不再触发回送，避免两个节点
// 互相回送形成无限 ping-pong。请求负责"双向同步"（合并 + 回送），响应只负责
// "单向吸收"（合并不回送）——和 gossip 包的 GossipRequest/Response 同构。
const (
	KindStateRequest  = "StateRequest"
	KindStateResponse = "StateResponse"
)

// StatePayload 是 G-Counter 消息的负载：一个完整的计数向量（按节点 ID 索引）。
// 为教学简洁直接传全量 map，不传增量/版本向量——代价是消息体积 O(节点数)。
type StatePayload struct {
	Values map[core.NodeID]uint64
}

// GCounter 是一个 Grow-only Counter（只增计数器）CRDT。
//
// 数据模型：每个节点维护一个向量（维度 = 节点数），只有"自己那一维"会递增，
// 其他维度来自合并别的节点。
//   - Increment(n)：自己的分量 += n（只增不减）。
//   - Merge(other)：每个分量取 max（max 满足交换/结合/幂等，故任意顺序合并都收敛）。
//   - Value()：所有分量求和（当前全局计数值）。
//
// 为什么 max 而不是 sum：sum 会把对方已经统计过的分量再加一遍（重复计数）；
// max 天然幂等，重复合并同一个状态不改变结果——这是 CRDT 收敛性的核心。
type GCounter struct {
	owner  core.NodeID            // 本计数器归属的节点（Increment 改的是这一维）
	nodes  []core.NodeID          // 集群成员列表（确定向量维度与遍历顺序）
	values map[core.NodeID]uint64 // 计数向量，按节点索引
}

// NewGCounter 构造一个属于 owner 的 G-Counter，集群成员为 nodes（含 owner）。
// owner 不在 nodes 中会被自动补齐，保证 Increment 有合法维度可写。
func NewGCounter(owner core.NodeID, nodes []core.NodeID) *GCounter {
	// 去重 + 补齐 owner，保持稳定顺序。
	seen := make(map[core.NodeID]bool, len(nodes)+1)
	list := make([]core.NodeID, 0, len(nodes)+1)
	add := func(id core.NodeID) {
		if !seen[id] {
			seen[id] = true
			list = append(list, id)
		}
	}
	for _, id := range nodes {
		add(id)
	}
	add(owner)

	values := make(map[core.NodeID]uint64, len(list))
	for _, id := range list {
		values[id] = 0
	}
	return &GCounter{owner: owner, nodes: list, values: values}
}

// Owner 返回本计数器归属的节点 ID（Increment 改这一维）。
func (g *GCounter) Owner() core.NodeID { return g.owner }

// Nodes 返回集群成员列表（确定顺序，便于遍历与测试）。
func (g *GCounter) Nodes() []core.NodeID {
	out := make([]core.NodeID, len(g.nodes))
	copy(out, g.nodes)
	return out
}

// Increment 把 n 加到自己的分量上。n 可以是任意非负整数；为 0 时是幂等的 no-op。
// 只改自己那一维（owner），不动别的节点维度——这是 grow-only 的关键约束。
func (g *GCounter) Increment(n uint64) {
	g.values[g.owner] += n
}

// Local 返回自己那一维的计数值（仅本节点贡献的计数）。
func (g *GCounter) Local() uint64 {
	return g.values[g.owner]
}

// Get 返回某节点的分量；未知节点返回 0。
func (g *GCounter) Get(id core.NodeID) uint64 {
	return g.values[id]
}

// Value 返回全局计数值：所有分量求和。
// 对收敛后的多个副本，调用 Value() 必然得到相同结果（CRDT 的收敛不变量）。
func (g *GCounter) Value() uint64 {
	var sum uint64
	for _, v := range g.values {
		sum += v
	}
	return sum
}

// Merge 把 other 合并进自己：每个分量取 max。
// 合并后双方再做一次对称 Merge，即完成一次双向同步（两个副本收敛到相同向量）。
// max 满足交换律、结合律、幂等律，因此无论合并顺序、是否重复，最终向量都一致。
func (g *GCounter) Merge(other *GCounter) {
	for id, v := range other.values {
		if cur, ok := g.values[id]; !ok || v > cur {
			g.values[id] = v
		}
	}
}

// mergeValues 是 map 级别的合并（供 Node.handle 直接合并网络上收到的向量，
// 无需构造完整 GCounter）。规则与 Merge 一致：逐分量取 max，未知维度补入。
func mergeValues(dst, src map[core.NodeID]uint64) {
	for id, v := range src {
		if cur, ok := dst[id]; !ok || v > cur {
			dst[id] = v
		}
	}
}

// Compare 判断两个 G-Counter 的因果关系：a ⊆ b 当且仅当 a 的每个分量都 ≤ b。
//   - a ⊆ b 且 b ⊈ a → a happens-before b（a 是 b 的祖先）。
//   - a ⊆ b 且 b ⊆ a → 相等（同一状态）。
//   - 两者互不包含 → 并发（concurrent），需 Merge 收敛。
//
// 这是基于向量时钟的偏序判定，CRDT 用它区分"已被对方覆盖"与"并发需合并"。
func Compare(a, b *GCounter) (aSubsetB, bSubsetA bool) {
	aSubsetB = true
	bSubsetA = true
	// 遍历并集（任一集合出现过的 key）。
	for _, id := range unionKeys(a.values, b.values) {
		av := a.values[id] // map 缺失返回 0，无需 ok 检查
		bv := b.values[id]
		if av > bv {
			aSubsetB = false
		}
		if bv > av {
			bSubsetA = false
		}
	}
	return aSubsetB, bSubsetA
}

// unionKeys 返回两个 map 的键并集（顺序不确定，仅用于遍历）。
func unionKeys(a, b map[core.NodeID]uint64) []core.NodeID {
	seen := make(map[core.NodeID]bool, len(a)+len(b))
	out := make([]core.NodeID, 0, len(a)+len(b))
	for k := range a {
		if !seen[k] {
			seen[k] = true
			out = append(out, k)
		}
	}
	for k := range b {
		if !seen[k] {
			seen[k] = true
			out = append(out, k)
		}
	}
	return out
}

// Snapshot 返回当前向量的深拷贝，避免发出去的 map 被后续修改污染。
func (g *GCounter) Snapshot() map[core.NodeID]uint64 {
	out := make(map[core.NodeID]uint64, len(g.values))
	for k, v := range g.values {
		out[k] = v
	}
	return out
}

// Node 是一个承载 G-Counter 的网络节点。单 goroutine 由外部 Tick 驱动（与
// raft/gossip 包一致），不内部并发，保证 demo 执行轨迹确定。
//
// 与 GCounter 值类型的关系：Node 持有一个 GCounter 副本，并通过 transport 与
// 邻居交换向量，对端 Merge 后回送——这样多个 Node 的 GCounter 最终收敛到相同 Value。
type Node struct {
	ID        core.NodeID
	Peers     []core.NodeID
	Counter   *GCounter
	transport core.Transport

	// tickCount 用于 round-robin 选邻居（确定性，无 rand）。
	tickCount int
	lastPeer  core.NodeID
}

// NewNode 构造一个承载 G-Counter 的节点。peers 为集群成员（含自己或不含均可，
// 选邻居时跳过自己）。
func NewNode(id core.NodeID, peers []core.NodeID, tr core.Transport) *Node {
	return &Node{
		ID:        id,
		Peers:     peers,
		Counter:   NewGCounter(id, peers),
		transport: tr,
	}
}

// Start 把节点注册到传输层，开始接收邻居的 State 消息。
func (n *Node) Start() {
	n.transport.Install(n.ID, n.handle)
}

// LastPeer 返回最近一次 Tick 选择的邻居，便于测试断言"确实联系了邻居"。
func (n *Node) LastPeer() core.NodeID { return n.lastPeer }

// Tick 由外部驱动一个时间片：用 round-robin 挑一个邻居，向其 Push 自己的向量。
// 没有邻居（单节点）则什么都不做。选邻居用 peers[tickCount % len(peers)]——确定性轮询。
func (n *Node) Tick() {
	peer, ok := n.pickPeer()
	if !ok {
		return
	}
	n.lastPeer = peer
	n.transport.Send(core.Message{
		From: n.ID, To: peer,
		Kind:    KindStateRequest,
		Payload: StatePayload{Values: n.Counter.Snapshot()},
	})
}

// pickPeer 用 round-robin 在 peers 中挑一个非自身的邻居。
func (n *Node) pickPeer() (core.NodeID, bool) {
	if len(n.Peers) == 0 {
		return "", false
	}
	for i := 0; i < len(n.Peers); i++ {
		idx := (n.tickCount + i) % len(n.Peers)
		peer := n.Peers[idx]
		if peer != n.ID {
			n.tickCount = idx + 1
			return peer, true
		}
	}
	n.tickCount++
	return "", false
}

// handle 是传输层回调：收到邻居的向量后 Merge 进自己。
//   - 收到 StateRequest（对方 Push 来了）：合并对方向量，再回 StateResponse 带上
//     自己合并后的向量（Pull），完成一次双向同步。
//   - 收到 StateResponse（对方 Pull 回了）：合并对方向量，不再回送（避免 ping-pong）。
func (n *Node) handle(msg core.Message) (core.Message, bool) {
	switch msg.Kind {
	case KindStateRequest:
		req, ok := msg.Payload.(StatePayload)
		if !ok {
			return core.Message{}, false
		}
		// 合并对方向量（max）。
		mergeValues(n.Counter.values, req.Values)
		// 回送自己合并后的向量（Pull 阶段）。
		return core.Message{
			From: n.ID, To: msg.From,
			Kind:    KindStateResponse,
			Payload: StatePayload{Values: n.Counter.Snapshot()},
		}, true
	case KindStateResponse:
		resp, ok := msg.Payload.(StatePayload)
		if !ok {
			return core.Message{}, false
		}
		mergeValues(n.Counter.values, resp.Values)
		return core.Message{}, false
	}
	return core.Message{}, false
}
