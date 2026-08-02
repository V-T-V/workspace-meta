package leader_elect

import (
	"strconv"

	"github.com/QiuShichang/consensus-atlas/internal/core"
)

// 消息 Kind 常量。Bully / Ring 共用。
const (
	KindElection    = "Election"    // 发起/转发选举
	KindAnswer      = "Answer"      // Bully 应答：我有更高 ID
	KindCoordinator = "Coordinator" // 新 Leader 公告
)

// ===== Bully =====

// BullyNode 是 Bully 选举算法的一个参与者。ID 用数值，比大小决定胜负。
//
// 单 goroutine 驱动（由 demo 的 Drain 调用），不内部并发，保证轨迹确定。
type BullyNode struct {
	ID     uint64
	Peers  []uint64 // 已知成员的 ID 集合（含自己）
	Online bool     // 是否在线（模拟节点崩溃：false 时丢弃所有消息）

	// knowsLeader 在收到 Coordinator 后被设为当选者 ID。
	// 0 表示尚未确定 Leader。
	knowsLeader uint64

	// electionStarted 标记本轮已发起过选举（避免重复发起）。
	electionStarted bool
	// waitingAnswer 标记发完 Election 后在等待更高 ID 的 Answer。
	// 若后续 Drain 中始终没收到 Answer，则自己当选。
	waitingAnswer bool

	transport core.Transport
}

// NewBullyNode 构造一个在线的 Bully 节点。
func NewBullyNode(id uint64, peers []uint64, tr core.Transport) *BullyNode {
	return &BullyNode{ID: id, Peers: peers, Online: true, transport: tr}
}

// Start 把节点注册到传输层。注意：transport 用 NodeID 标识节点，
// Bully 节点的传输层 ID 是其数值 ID 的字符串形式（见 BullyKey）。
func (n *BullyNode) Start() {
	n.transport.Install(n.bullyKey(), n.handle)
}

// bullyKey 把数值 ID 转成 core.NodeID，用于在传输层寻址。
func (n *BullyNode) bullyKey() core.NodeID { return BullyKey(n.ID) }

// BullyKey 把数值 ID 转成 core.NodeID（"b3" 形式，b 前缀避免和其它包冲突）。
func BullyKey(id uint64) core.NodeID {
	return core.NodeID("b" + strconv.FormatUint(id, 10))
}

// Leader 返回当前已知的新 Leader ID（0 表示未知）。
func (n *BullyNode) Leader() uint64 { return n.knowsLeader }

// StartElection 发起选举：向所有更高 ID 节点发 Election。
// 若没有任何更高 ID 节点在线（无 Answer 回来），自己当选并广播 Coordinator。
func (n *BullyNode) StartElection() {
	if !n.Online || n.electionStarted {
		return
	}
	n.electionStarted = true
	n.waitingAnswer = false

	hasHigher := false
	for _, pid := range n.Peers {
		if pid > n.ID {
			hasHigher = true
			n.transport.Send(core.Message{
				From: n.bullyKey(), To: BullyKey(pid),
				Kind:    KindElection,
				Payload: ElectionPayload{From: n.ID},
			})
		}
	}
	// 没有更高 ID 节点 → 直接当选。
	if !hasHigher {
		n.declareCoordinator()
	} else {
		// 标记等待 Answer；后续 Drain 中若收到 Answer 则 waitingAnswer 被清。
		// 若整个 Drain 结束仍无 Answer（更高节点都离线），由 FinishElection 兜底当选。
		n.waitingAnswer = true
	}
}

// FinishElection 在发起选举并 drain 完所有消息后由 demo 调用。
// 若仍在等待 Answer 却一个都没收到，说明更高 ID 节点都已离线，自己当选。
func (n *BullyNode) FinishElection() {
	if n.waitingAnswer {
		n.declareCoordinator()
	}
}

// RunBullyToCompletion 把 Bully 选举推进到收敛：
//  1. drain 到网络稳定（所有 Election/Answer 级联充分传播）；
//  2. 此刻仍处于 waitingAnswer 的节点，说明它发出去的 Election 没有任何更高 ID
//     的在线节点应答（更高节点都已离线），由它兜底当选并广播 Coordinator；
//  3. 再 drain 一次让 Coordinator 传播到位。
//
// 这种"先稳定再兜底"的两段式保证确定性：不会在 Answer 还在传输途中就误判当选。
func RunBullyToCompletion(tr core.Transport, nodes map[uint64]*BullyNode, ids []uint64) {
	for i := 0; i < 64; i++ {
		handled := tr.Drain()
		if len(handled) == 0 {
			break
		}
	}
	for _, id := range ids {
		nodes[id].FinishElection()
	}
	for i := 0; i < 64; i++ {
		handled := tr.Drain()
		if len(handled) == 0 {
			break
		}
	}
}

// declareCoordinator 自己当选：记录 Leader 并向所有节点广播 Coordinator。
func (n *BullyNode) declareCoordinator() {
	n.waitingAnswer = false
	n.knowsLeader = n.ID
	for _, pid := range n.Peers {
		if pid == n.ID {
			continue
		}
		n.transport.Send(core.Message{
			From: n.bullyKey(), To: BullyKey(pid),
			Kind:    KindCoordinator,
			Payload: CoordinatorPayload{Leader: n.ID},
		})
	}
}

// handle 分发 Election / Answer / Coordinator 三类消息。
// 返回 handled=true（即使无回复），让 Drain 的"已处理计数"能反映真实推进，
// 便于上层用"本轮是否处理过消息"做确定性收敛判定。回复由各 handler 自行 Send。
func (n *BullyNode) handle(msg core.Message) (core.Message, bool) {
	if !n.Online {
		return core.Message{}, false // 离线节点静默丢弃（不计入已处理）
	}
	switch msg.Kind {
	case KindElection:
		n.handleElection(msg)
	case KindAnswer:
		n.handleAnswer(msg)
	case KindCoordinator:
		n.handleCoordinator(msg)
	}
	return core.Message{}, true
}

// handleElection：收到 Election，若自己 ID 更大则回 Answer 并发起自己的选举。
func (n *BullyNode) handleElection(msg core.Message) {
	ep, ok := msg.Payload.(ElectionPayload)
	if !ok {
		return
	}
	if n.ID > ep.From {
		// 回 Answer：告诉发起者"我比你大，你退出"。
		n.transport.Send(core.Message{
			From: n.bullyKey(), To: BullyKey(ep.From),
			Kind:    KindAnswer,
			Payload: AnswerPayload{From: n.ID},
		})
		// 自己也发起选举（去竞争更高的）。
		n.StartElection()
	}
	// 自己 ID 更小：不回 Answer（发起者超时后会自己当选）。
}

// handleAnswer：收到更高 ID 节点的 Answer，停止等待，退出竞争。
func (n *BullyNode) handleAnswer(msg core.Message) {
	ap, ok := msg.Payload.(AnswerPayload)
	if !ok {
		return
	}
	if ap.From > n.ID {
		n.waitingAnswer = false // 有更高节点应答，我退出，等 Coordinator
	}
}

// handleCoordinator：记录新 Leader。
func (n *BullyNode) handleCoordinator(msg core.Message) {
	cp, ok := msg.Payload.(CoordinatorPayload)
	if !ok {
		return
	}
	n.knowsLeader = cp.Leader
	n.waitingAnswer = false
}

// ===== Ring =====

// RingNode 是 Ring 选举算法的一个参与者。节点逻辑成环，每个知道后继 Next。
//
// 单 goroutine 驱动（由 demo 的 Drain 调用）。
type RingNode struct {
	ID     uint64
	Next   uint64 // 环上后继的 ID
	Online bool

	// knowsLeader 在收到 Coordinator 后被设为当选者 ID。
	knowsLeader uint64
	// initiator 标记本节点是否是本轮选举的发起者（消息绕一圈回到自己时用）。
	initiator bool

	transport core.Transport
}

// NewRingNode 构造一个在线的 Ring 节点。
func NewRingNode(id, next uint64, tr core.Transport) *RingNode {
	return &RingNode{ID: id, Next: next, Online: true, transport: tr}
}

// Start 把节点注册到传输层。
func (n *RingNode) Start() {
	n.transport.Install(n.ringKey(), n.handle)
}

func (n *RingNode) ringKey() core.NodeID { return RingKey(n.ID) }

// RingKey 把数值 ID 转成 core.NodeID（"r3" 形式）。
func RingKey(id uint64) core.NodeID {
	return core.NodeID("r" + strconv.FormatUint(id, 10))
}

// Leader 返回当前已知的新 Leader ID（0 表示未知）。
func (n *RingNode) Leader() uint64 { return n.knowsLeader }

// StartElection 发起选举：向 Next 发 Election 消息，带上自己的 ID。
func (n *RingNode) StartElection() {
	if !n.Online || n.initiator {
		return
	}
	n.initiator = true
	n.transport.Send(core.Message{
		From: n.ringKey(), To: RingKey(n.Next),
		Kind:    KindElection,
		Payload: RingElectionPayload{IDs: []uint64{n.ID}},
	})
}

// handle 分发 Ring 的 Election / Coordinator 消息。
// 返回 handled=true 让 Drain 的"已处理计数"反映真实推进。
func (n *RingNode) handle(msg core.Message) (core.Message, bool) {
	if !n.Online {
		return core.Message{}, false
	}
	switch msg.Kind {
	case KindElection:
		n.handleRingElection(msg)
	case KindCoordinator:
		n.handleCoordinator(msg)
	}
	return core.Message{}, true
}

// handleRingElection：把本节点 ID 加入集合，转发给 Next。
// 若消息绕一圈回到发起者（本节点 ID 已在集合且来自 Next 路径完整），
// 取最大 ID 当选并广播 Coordinator。
func (n *RingNode) handleRingElection(msg core.Message) {
	ep, ok := msg.Payload.(RingElectionPayload)
	if !ok {
		return
	}
	// 若本节点 ID 已在集合中 → 消息绕完一圈回来了。
	containsSelf := false
	for _, id := range ep.IDs {
		if id == n.ID {
			containsSelf = true
			break
		}
	}
	if containsSelf {
		// 选最大 ID 当选。
		maxID := ep.IDs[0]
		for _, id := range ep.IDs {
			if id > maxID {
				maxID = id
			}
		}
		// 广播 Coordinator 给环上所有已知节点（即 ep.IDs 集合）。
		n.knowsLeader = maxID
		for _, id := range ep.IDs {
			if id == n.ID {
				continue
			}
			n.transport.Send(core.Message{
				From: n.ringKey(), To: RingKey(id),
				Kind:    KindCoordinator,
				Payload: CoordinatorPayload{Leader: maxID},
			})
		}
		return
	}
	// 否则加入自己的 ID，转发给 Next。
	ids := append(append([]uint64{}, ep.IDs...), n.ID)
	n.transport.Send(core.Message{
		From: n.ringKey(), To: RingKey(n.Next),
		Kind:    KindElection,
		Payload: RingElectionPayload{IDs: ids},
	})
}

// handleCoordinator：记录新 Leader。
func (n *RingNode) handleCoordinator(msg core.Message) {
	cp, ok := msg.Payload.(CoordinatorPayload)
	if !ok {
		return
	}
	n.knowsLeader = cp.Leader
}

// ===== Payloads =====

// ElectionPayload 是 Bully 的 Election 消息负载（携带发起者 ID）。
type ElectionPayload struct {
	From uint64
}

// AnswerPayload 是 Bully 的 Answer 消息负载。
type AnswerPayload struct {
	From uint64
}

// CoordinatorPayload 是新 Leader 的公告负载（Bully/Ring 共用）。
type CoordinatorPayload struct {
	Leader uint64
}

// RingElectionPayload 是 Ring 的 Election 消息负载：携带沿环收集到的所有 ID。
type RingElectionPayload struct {
	IDs []uint64
}
