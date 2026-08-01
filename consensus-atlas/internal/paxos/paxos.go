package paxos

import (
	"github.com/QiuShichang/consensus-atlas/internal/core"
)

// 消息 Kind 常量。对齐 raft 包风格：每对 RPC 两个 Kind。
const (
	KindPrepare  = "Prepare"  // Phase 1：Proposer → Acceptor
	KindPromise  = "Promise"  // Phase 1：Acceptor → Proposer
	KindAccept   = "Accept"   // Phase 2：Proposer → Acceptor
	KindAccepted = "Accepted" // Phase 2：Acceptor → Proposer/Learner
)

// PrepareRequest 是 Phase 1 的请求负载。
// Number 是提案编号（Paxos 的 "ballot number"，本包用单调递增 uint64）。
type PrepareRequest struct {
	Number uint64
}

// PrepareResponse 是 Acceptor 对 Prepare 的回复。
// Promised=true 表示承诺不再接受 < Number 的提案。
// 若 Acceptor 之前已接受过提案，把最高编号的已接受提案回带（AcceptedNumber>0 即有效）。
type PrepareResponse struct {
	Promised       bool
	AcceptedNumber uint64
	AcceptedValue  any
}

// AcceptRequest 是 Phase 2 的请求负载。
// Value 取 Promise 回复里最高编号的已接受值；若所有 Promise 都未带已接受值，则用自己的值。
type AcceptRequest struct {
	Number uint64
	Value  any
}

// AcceptResponse 是 Acceptor 对 Accept 的回复（回给 Proposer）。
// Accepted=true 表示接受（Number >= 已承诺编号）。
type AcceptResponse struct {
	Accepted bool
}

// Acceptor 是 Paxos 的接受者：对提案做出承诺、接受提案、并把已接受值通知 Learner。
// 单 goroutine 驱动（handle 作为 Transport Handler 被同步调用），无内部并发。
type Acceptor struct {
	ID              core.NodeID
	Peers           []core.NodeID // 含自己 + Proposer（用于 quorum 计算）
	HighestPromised uint64        // 已承诺的最高编号；不再接受 < 此值的 Prepare/Accept
	HighestAccepted uint64        // 已接受提案的最高编号
	AcceptedValue   any           // HighestAccepted 对应的值
	Learners        []core.NodeID // 接受提案后需通知的学习者列表
	transport       core.Transport
}

// NewAcceptor 构造一个 Acceptor 并持有传输层（暂未注册，需显式 Start）。
func NewAcceptor(id core.NodeID, peers []core.NodeID, tr core.Transport) *Acceptor {
	return &Acceptor{ID: id, Peers: peers, transport: tr}
}

// Start 把 Acceptor 注册到传输层，开始接收 Prepare/Accept。
func (a *Acceptor) Start() {
	a.transport.Install(a.ID, a.handle)
}

// handle 作为 Transport Handler 分发 Prepare / Accept。
func (a *Acceptor) handle(msg core.Message) (core.Message, bool) {
	switch msg.Kind {
	case KindPrepare:
		return a.handlePrepare(msg)
	case KindAccept:
		return a.handleAccept(msg)
	}
	return core.Message{}, false
}

// handlePrepare：若编号 >= 已承诺编号则承诺，并回带已接受的最大编号提案。
func (a *Acceptor) handlePrepare(msg core.Message) (core.Message, bool) {
	req, ok := msg.Payload.(PrepareRequest)
	if !ok {
		return core.Message{}, false
	}
	if req.Number < a.HighestPromised {
		// 已承诺更高编号：拒绝，回 Promise=false（不带已接受值，让 Proposer 自行重试更高编号）。
		return core.Message{
			From: a.ID, To: msg.From,
			Kind:    KindPromise,
			Payload: PrepareResponse{Promised: false},
		}, true
	}
	a.HighestPromised = req.Number
	return core.Message{
		From: a.ID, To: msg.From,
		Kind: KindPromise,
		Payload: PrepareResponse{
			Promised:       true,
			AcceptedNumber: a.HighestAccepted,
			AcceptedValue:  a.AcceptedValue,
		},
	}, true
}

// handleAccept：若编号 >= 已承诺编号则接受，记录为最高已接受值，并通知 Learner。
// 回给 Proposer 的是 AcceptResponse（不含值）；发给 Learner 的是 AcceptRequest（含值，便于按值计数）。
func (a *Acceptor) handleAccept(msg core.Message) (core.Message, bool) {
	req, ok := msg.Payload.(AcceptRequest)
	if !ok {
		return core.Message{}, false
	}
	if req.Number < a.HighestPromised {
		// 已承诺更高编号：拒绝接受。
		return core.Message{
			From: a.ID, To: msg.From,
			Kind:    KindAccepted,
			Payload: AcceptResponse{Accepted: false},
		}, true
	}
	// 接受：更新已接受的最大编号提案。
	a.HighestPromised = req.Number
	a.HighestAccepted = req.Number
	a.AcceptedValue = req.Value

	// 通知所有 Learner（带值，用 AcceptRequest 当 payload）。
	for _, learner := range a.Learners {
		a.transport.Send(core.Message{
			From: a.ID, To: learner,
			Kind:    KindAccepted,
			Payload: AcceptRequest{Number: req.Number, Value: req.Value},
		})
	}
	// 回 Proposer：用 AcceptResponse。
	return core.Message{
		From: a.ID, To: msg.From,
		Kind:    KindAccepted,
		Payload: AcceptResponse{Accepted: true},
	}, true
}

// Proposer 是 Paxos 的提议者（对应 Raft 的 Leader/Primary）。
// 单 goroutine 驱动：propose() 触发 Phase 1，handle 收 Promise 触发 Phase 2，
// 收 Accepted 触发完成统计。
type Proposer struct {
	ID             core.NodeID
	Peers          []core.NodeID // 含自己 + 所有 Acceptor（用于 quorum 计算）
	ProposalNumber uint64        // 当前提案编号（单调递增）
	Value          any           // 想提议的值

	// Phase 1 累计状态。
	promises       int    // 收到的 Promise 数
	highestSeenNum uint64 // Promise 里带回来的最高已接受编号
	highestSeenVal any    // 对应的值

	// Phase 2 累计状态。
	accepted   int  // 收到的 Accepted=true 数
	phase2Sent bool // 是否已发出 Accept（避免重复）

	Chosen      bool // 是否已确认 chosen
	ChosenValue any  // 最终 chosen 的值

	transport core.Transport
}

// NewProposer 构造一个 Proposer。
func NewProposer(id core.NodeID, peers []core.NodeID, tr core.Transport) *Proposer {
	return &Proposer{ID: id, Peers: peers, transport: tr}
}

// Start 把 Proposer 注册到传输层。
func (p *Proposer) Start() {
	p.transport.Install(p.ID, p.handle)
}

// quorum 是达成共识所需的多数派大小。Peers 含自己（见 raft 包约定），
// 多数派 = floor(len/2) + 1。
func (p *Proposer) quorum() int {
	return len(p.Peers)/2 + 1
}

// propose 发起一轮 Paxos：向所有 Acceptor 发 Prepare。
// 调用前应已设好 ProposalNumber 和 Value。
func (p *Proposer) propose() {
	// 重置本轮累计状态。
	p.promises = 0
	p.highestSeenNum = 0
	p.highestSeenVal = nil
	p.accepted = 0
	p.phase2Sent = false
	p.Chosen = false
	p.ChosenValue = nil

	for _, peer := range p.Peers {
		if peer == p.ID {
			continue
		}
		p.transport.Send(core.Message{
			From: p.ID, To: peer,
			Kind:    KindPrepare,
			Payload: PrepareRequest{Number: p.ProposalNumber},
		})
	}
}

// handle 作为 Transport Handler 分发 Promise / Accepted。
func (p *Proposer) handle(msg core.Message) (core.Message, bool) {
	switch msg.Kind {
	case KindPromise:
		p.handlePromise(msg)
		return core.Message{}, false
	case KindAccepted:
		p.handleAccepted(msg)
		return core.Message{}, false
	}
	return core.Message{}, false
}

// handlePromise 累计 Promise；达 quorum 后发 Accept，值取已接受的最大编号值。
func (p *Proposer) handlePromise(msg core.Message) {
	if p.phase2Sent {
		return // 已进入 Phase 2，忽略迟到的 Promise
	}
	resp, ok := msg.Payload.(PrepareResponse)
	if !ok {
		return
	}
	if !resp.Promised {
		return // 被拒（编号过低），本包不重试更高编号，demo 用保证够大的编号
	}
	p.promises++
	// 跟踪 Promise 里带回来的最高编号已接受提案（保证不会覆盖已 chosen 的值）。
	if resp.AcceptedNumber > p.highestSeenNum {
		p.highestSeenNum = resp.AcceptedNumber
		p.highestSeenVal = resp.AcceptedValue
	}
	if p.promises >= p.quorum() {
		// 选定 Phase 2 的值：若 Promise 里有已接受值，用它；否则用自己的值。
		value := p.Value
		if p.highestSeenNum > 0 {
			value = p.highestSeenVal
		}
		p.phase2Sent = true
		for _, peer := range p.Peers {
			if peer == p.ID {
				continue
			}
			p.transport.Send(core.Message{
				From: p.ID, To: peer,
				Kind:    KindAccept,
				Payload: AcceptRequest{Number: p.ProposalNumber, Value: value},
			})
		}
	}
}

// handleAccepted 累计 Accepted；达 quorum 后标记 chosen。
func (p *Proposer) handleAccepted(msg core.Message) {
	resp, ok := msg.Payload.(AcceptResponse)
	if !ok {
		return
	}
	if !resp.Accepted {
		return
	}
	p.accepted++
	if !p.Chosen && p.accepted >= p.quorum() {
		// 值的优先级与发 Accept 时一致：已接受最大值 > 自己的值。
		if p.highestSeenNum > 0 {
			p.ChosenValue = p.highestSeenVal
		} else {
			p.ChosenValue = p.Value
		}
		p.Chosen = true
	}
}

// Learner 是 Paxos 的学习者：收集多数派 Accepted 确定 chosen 值。
// 单 goroutine 驱动。
type Learner struct {
	ID     core.NodeID
	Peers  []core.NodeID // 含自己 + Acceptor（用于 quorum 计算）
	counts map[any]int   // 值 → 收到的 Accepted 数
	Chosen bool
	Value  any

	transport core.Transport
}

// NewLearner 构造一个 Learner。
func NewLearner(id core.NodeID, peers []core.NodeID, tr core.Transport) *Learner {
	return &Learner{
		ID:        id,
		Peers:     peers,
		counts:    make(map[any]int),
		transport: tr,
	}
}

// Start 把 Learner 注册到传输层。
func (l *Learner) Start() {
	l.transport.Install(l.ID, l.handle)
}

// quorum 同 Proposer。
func (l *Learner) quorum() int {
	return len(l.Peers)/2 + 1
}

// handle 收 Accepted（payload 为 AcceptRequest，含值），按值累计；达 quorum 标记 chosen。
func (l *Learner) handle(msg core.Message) (core.Message, bool) {
	if msg.Kind != KindAccepted {
		return core.Message{}, false
	}
	req, ok := msg.Payload.(AcceptRequest)
	if !ok {
		return core.Message{}, false
	}
	l.counts[req.Value]++
	if !l.Chosen && l.counts[req.Value] >= l.quorum() {
		l.Chosen = true
		l.Value = req.Value
	}
	return core.Message{}, false
}
