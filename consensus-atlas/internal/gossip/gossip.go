package gossip

import (
	"github.com/QiuShichang/consensus-atlas/internal/core"
)

// 消息 Kind 常量。
// 用"请求/响应"两阶段，对应 Push-Pull 一次往返：
//   - Request  由发起方发出，携带自己当前的全量状态（Push 阶段）。
//   - Response 由接收方在合并后回送，携带自己合并后的全量状态（Pull 阶段）。
const (
	KindGossipRequest  = "GossipRequest"
	KindGossipResponse = "GossipResponse"
)

// GossipPayload 是 Gossip 消息的负载：一个完整的键值状态表。
// 为教学简洁，这里直接传全量 map，而非论文里的 Merkle 摘要/版本向量——
// 代价是消息体积 O(状态大小)，但循环逻辑最直观。
type GossipPayload struct {
	Values map[string]string
}

// Node 是一个 Gossip 节点。单 goroutine 由外部 Tick 驱动（与 raft 包一致），
// 不内部并发，保证 demo 执行轨迹确定。
type Node struct {
	ID    core.NodeID
	Peers []core.NodeID
	State map[string]string // 本节点持有的键值状态

	transport core.Transport

	// tickCount 累计被 Tick 的次数；用 round-robin 选邻居而非随机：
	// peers[tickCount % len(peers)]，保证轨迹完全确定（无 rand 依赖）。
	tickCount int
	// lastPeer 记录本轮 Tick 选择的邻居，便于测试/观测。
	lastPeer core.NodeID
}

// NewNode 构造一个 Gossip 节点。peers 为集群成员列表（含自己或不含均可，
// 选邻居时会跳过自己）。初始状态为空，由调用方填充 n.State。
func NewNode(id core.NodeID, peers []core.NodeID, tr core.Transport) *Node {
	return &Node{
		ID:        id,
		Peers:     peers,
		State:     make(map[string]string),
		transport: tr,
	}
}

// Start 把节点注册到传输层，开始接收消息。
func (n *Node) Start() {
	n.transport.Install(n.ID, n.handle)
}

// LastPeer 返回最近一次 Tick 选择的邻居，便于测试断言"确实联系了邻居"。
// 未联系过任何邻居时返回空 NodeID。
func (n *Node) LastPeer() core.NodeID { return n.lastPeer }

// Set 让调用方写入一个键值（demo 用于初始化各节点持有的不同 key）。
func (n *Node) Set(k, v string) {
	n.State[k] = v
}

// Tick 由外部驱动一个时间片：用 round-robin 挑一个邻居，向其 Push 自己的全量状态。
// 没有邻居（单节点）则什么都不做。
//
// 选邻居用 peers[tickCount % len(peers)]——确定性轮询，而非论文里的随机抽样。
// 这样 demo 不依赖 math/rand，给定相同输入必然得到相同轨迹。
func (n *Node) Tick() {
	peer, ok := n.pickPeer()
	if !ok {
		return // 无邻居可联系（单节点集群）
	}
	n.lastPeer = peer
	n.transport.Send(core.Message{
		From: n.ID, To: peer,
		Kind:    KindGossipRequest,
		Payload: GossipPayload{Values: n.snapshot()},
	})
}

// pickPeer 用 round-robin 在 peers 中挑一个非自身的邻居。
// 返回 false 表示没有可选邻居（peers 只有自己 / 为空）。
func (n *Node) pickPeer() (core.NodeID, bool) {
	if len(n.Peers) == 0 {
		return "", false
	}
	// 在 peers 列表里最多扫一圈，找到第一个 != self 的。
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

// handle 是传输层回调：分发 GossipRequest / GossipResponse 两类消息。
//   - 收到 Request（对方 Push 来了）：合并对方状态到自己，再回 Response 带上自己
//     合并后的全量状态（Pull），完成一次双向同步。
//   - 收到 Response（对方 Pull 回了）：合并对方状态到自己，无需再回复。
func (n *Node) handle(msg core.Message) (core.Message, bool) {
	switch msg.Kind {
	case KindGossipRequest:
		req, ok := msg.Payload.(GossipPayload)
		if !ok {
			return core.Message{}, false
		}
		n.merge(req.Values)
		// 回送自己合并后的全量状态（Pull 阶段）。
		return core.Message{
			From: n.ID, To: msg.From,
			Kind:    KindGossipResponse,
			Payload: GossipPayload{Values: n.snapshot()},
		}, true
	case KindGossipResponse:
		resp, ok := msg.Payload.(GossipPayload)
		if !ok {
			return core.Message{}, false
		}
		n.merge(resp.Values)
		return core.Message{}, false
	}
	return core.Message{}, false
}

// merge 把 other 的状态合并进自己。合并规则（保证所有节点最终收敛到相同值）：
//   - 自己没有的 key → 加入。
//   - 双方都有的 key → 按字符串序取"更大"的值（模拟版本号/时间戳比较，
//     全序且确定，保证不同节点对同一对值做出相同抉择）。
//
// 这是一个"最大值合并函数"（last-writer-wins 的简化形态）：
// 合并可交换、可结合，因此无论交换顺序如何，最终结果一致——收敛性来源。
func (n *Node) merge(other map[string]string) {
	for k, v := range other {
		old, ok := n.State[k]
		if !ok || v > old {
			n.State[k] = v
		}
	}
}

// snapshot 返回当前状态的深拷贝，避免发出去的 map 被后续修改污染。
func (n *Node) snapshot() map[string]string {
	out := make(map[string]string, len(n.State))
	for k, v := range n.State {
		out[k] = v
	}
	return out
}

// Merge 是包级辅助函数，把 src 合并进 dst（同 Node.merge 的规则）。
// 暴露出来方便测试独立验证合并语义。
func Merge(dst, src map[string]string) {
	for k, v := range src {
		old, ok := dst[k]
		if !ok || v > old {
			dst[k] = v
		}
	}
}
