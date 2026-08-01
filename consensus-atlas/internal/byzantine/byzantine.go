package byzantine

import (
	"fmt"

	"github.com/QiuShichang/consensus-atlas/internal/core"
)

// 消息 Kind 常量。三阶段对应三种消息。
const (
	KindPrePrepare = "PrePrepare"
	KindPrepare    = "Prepare"
	KindCommit     = "Commit"
)

// Request 是客户端发起的请求，由 Primary 排定 Sequence 后进入共识。
type Request struct {
	Op     string // 状态机操作（简化为字符串）
	Client string // 客户端标识
}

// PrePrepare 是 Primary 广播的提议负载。
type PrePrepare struct {
	View     uint64  // 当前视图（Primary 任期）
	Sequence uint64  // 请求的序列号（决定执行顺序）
	Request  Request // 客户端请求
}

// Vote 是 prepare / commit 阶段各 Replica 投出的票。
// Signature 用字符串占位（真实系统是 MAC 或数字签名），本包不校验其内容。
type Vote struct {
	Sequence  uint64      // 投票针对的序列号
	View      uint64      // 投票所在的视图
	Node      core.NodeID // 投票节点
	Signature string      // 占位签名
}

// Replica 是一个 PBFT 节点。PBFT 术语中"节点"叫 replica，
// Primary 是当前视图下负责排序的 replica（由 view = primary_id 决定）。
//
// 单 goroutine 驱动（由 demo 的 Drain 调用），不内部并发，保证轨迹确定。
type Replica struct {
	ID    core.NodeID
	Peers []core.NodeID // 含自己；集群总节点数 = len(Peers)

	IsPrimary bool   // 是否为当前视图的 Primary
	View      uint64 // 当前视图号
	Sequence  uint64 // 本节点已分配的最大序列号（Primary 自增用）

	// IsTraitor 标记本 replica 为拜占庭（叛徒）节点，仅用于演示/测试。
	// 叛徒行为：在 prepare/commit 阶段既不广播自己的票、也不累计收到的票
	// （即"静默/遗漏故障"——对诚实节点而言相当于收不到叛徒的任何投票）。
	// 诚实集群仍能在 f 个叛徒下达成 quorum=2f+1，这正是 PBFT 的核心保证。
	IsTraitor bool

	// prepared[seq] 记录序列号 seq 收到的 prepare 投票集合。
	// 进入 prepared 后保留，用于触发 commit 并防止重复进入。
	prepared map[uint64]map[core.NodeID]bool
	// committed[seq] 记录序列号 seq 收到的 commit 投票集合。
	committed map[uint64]map[core.NodeID]bool
	// isPrepared[seq] / isCommitted[seq] 标记该 seq 是否已通过对应阶段门槛。
	isPrepared  map[uint64]bool
	isCommitted map[uint64]bool
	// executed[seq] 标记该 seq 是否已执行（demo 用，避免重复执行）。
	executed map[uint64]bool
	// proposal[seq] 保存 PrePrepare 内容，便于 prepare/commit 阶段校验一致性。
	proposal map[uint64]PrePrepare
	// CommittedSeqs 按序记录已 committed 的序列号（demo 可观察）。
	CommittedSeqs []uint64

	transport core.Transport
}

// NewReplica 构造一个 Replica。isPrimary 决定它是否在 view 0 担任 Primary。
//
// 注意：peers 的长度 n 必须满足 n ≡ 1 (mod 3)（即 n = 3f+1 形）才是合法 PBFT 配置。
// 非法 n（如 n=2/3/5）不满足拜占庭容错的 quorum 语义。
// 调用方应在构造前用 ValidateCluster 检查；本函数不 panic 以保持 demo 灵活性。
func NewReplica(id core.NodeID, peers []core.NodeID, isPrimary bool, tr core.Transport) *Replica {
	return &Replica{
		ID:          id,
		Peers:       peers,
		IsPrimary:   isPrimary,
		View:        0,
		prepared:    make(map[uint64]map[core.NodeID]bool),
		committed:   make(map[uint64]map[core.NodeID]bool),
		isPrepared:  make(map[uint64]bool),
		isCommitted: make(map[uint64]bool),
		executed:    make(map[uint64]bool),
		proposal:    make(map[uint64]PrePrepare),
		transport:   tr,
	}
}

// ValidateCluster 校验集群规模 n 是否满足 PBFT 的拜占庭容错要求（n = 3f+1 形）。
// 合法：n=4(f=1) / n=7(f=2) / n=10(f=3)...；非法：n=1/2/3/5/6/8/9...
// 返回 f（可容忍的拜占庭节点数）和 error。
func ValidateCluster(n int) (f int, err error) {
	if n < 4 {
		return 0, fmt.Errorf("PBFT 至少需要 4 个节点（n=4 容忍 1 拜占庭），得到 n=%d", n)
	}
	if (n-1)%3 != 0 {
		return 0, fmt.Errorf("PBFT 要求 n ≡ 1 (mod 3)（n=3f+1），得到 n=%d 不满足", n)
	}
	return (n - 1) / 3, nil
}

// Start 把 Replica 注册到传输层，开始接收消息。
func (r *Replica) Start() {
	r.transport.Install(r.ID, r.handle)
}

// quorum 是 PBFT 各阶段的多数派门槛：2f+1。
// 拜占庭容错要求 n = 3f+1（n ≡ 1 mod 3），此时 quorum = 2f+1 = (2n+1)/3。
// 等价整数实现：(2n+2)/3 在 n=3f+1 时正好等于 2f+1：
//
//	n=4 → 3, n=7 → 5, n=10 → 7。
//
// 注意：n 必须 ≡ 1 (mod 3) 才是合法 PBFT 配置。
// n=3 时 (2*3+2)/3=2，但 n=3 不满足 3f+1（无拜占庭容错能力），
// 调用方应用 ValidateCluster(n) 校验后再构造 Replica。
func (r *Replica) quorum() int {
	return (2*len(r.Peers) + 2) / 3
}

// Propose 由 Primary 调用：为请求分配 Sequence，广播 PrePrepare 启动共识。
// 非 Primary 调用返回错误。
func (r *Replica) Propose(req Request) error {
	if !r.IsPrimary {
		return fmt.Errorf("只有 Primary 才能 Propose")
	}
	r.Sequence++
	seq := r.Sequence

	pp := PrePrepare{View: r.View, Sequence: seq, Request: req}
	r.proposal[seq] = pp
	// Primary 自己也进入 prepare 阶段（自投一票），并广播自己的 Prepare 票给全网。
	// （在 PBFT 中 Primary 的 prepare 票与 pre-prepare 一同发出，这样其余 replica
	// 才能把"Primary 的票"计入 quorum——拜占庭场景下若少这一票，诚实 replica 会差一票。）
	r.recordPrepare(seq, r.View, r.ID)
	r.maybeAdvance(seq)
	r.broadcastPrepare(seq, pp.View)

	for _, peer := range r.Peers {
		if peer == r.ID {
			continue
		}
		r.transport.Send(core.Message{
			From: r.ID, To: peer,
			Kind:    KindPrePrepare,
			Term:    r.View,
			Payload: pp,
		})
	}
	return nil
}

// handle 是传输层回调：分发 PrePrepare / Prepare / Commit 三类消息。
func (r *Replica) handle(msg core.Message) (core.Message, bool) {
	switch msg.Kind {
	case KindPrePrepare:
		r.handlePrePrepare(msg)
	case KindPrepare:
		r.handlePrepare(msg)
	case KindCommit:
		r.handleCommit(msg)
	}
	return core.Message{}, false
}

// handlePrePrepare：Replica 收到 Primary 的提议，校验后广播自己的 Prepare 票。
func (r *Replica) handlePrePrepare(msg core.Message) {
	pp, ok := msg.Payload.(PrePrepare)
	if !ok {
		return
	}
	// 叛徒节点：丢弃 PrePrepare，既不记录提议也不广播 Prepare（静默故障）。
	// 诚实节点收不到叛徒的 Prepare 票，但仍能靠其余诚实节点达 quorum=2f+1。
	if r.IsTraitor {
		return
	}
	// 忽略旧视图的提议。
	if pp.View < r.View {
		return
	}
	// 记录提议（用于后续阶段校验同一 seq/view 的一致性）。
	if _, exists := r.proposal[pp.Sequence]; !exists {
		r.proposal[pp.Sequence] = pp
	}
	// 自投一票并广播 Prepare 给所有节点。
	r.recordPrepare(pp.Sequence, pp.View, r.ID)
	r.maybeAdvance(pp.Sequence)
	r.broadcastPrepare(pp.Sequence, pp.View)
}

// broadcastPrepare 构造本节点的 Prepare 票并广播给所有 peer（不含自己——
// 自己那票由调用方在 recordPrepare 中记录）。
func (r *Replica) broadcastPrepare(seq, view uint64) {
	vote := Vote{
		Sequence:  seq,
		View:      view,
		Node:      r.ID,
		Signature: "sig-" + string(r.ID), // 占位签名
	}
	for _, peer := range r.Peers {
		if peer == r.ID {
			continue
		}
		r.transport.Send(core.Message{
			From: r.ID, To: peer,
			Kind:    KindPrepare,
			Term:    view,
			Payload: vote,
		})
	}
}

// handlePrepare：累计某 seq 的 prepare 票数，达 2f+1 则进入 prepared 并广播 Commit。
func (r *Replica) handlePrepare(msg core.Message) {
	vote, ok := msg.Payload.(Vote)
	if !ok {
		return
	}
	if vote.View < r.View {
		return
	}
	// 叛徒节点：无视收到的 Prepare，不累计票数（自身永远不进入 prepared，
	// 也不广播 Commit），模拟"投假票/丢票"的拜占庭行为。
	if r.IsTraitor {
		return
	}
	r.recordPrepare(vote.Sequence, vote.View, vote.Node)
	r.maybeAdvance(vote.Sequence)
}

// handleCommit：累计某 seq 的 commit 票数，达 2f+1 则进入 committed。
func (r *Replica) handleCommit(msg core.Message) {
	vote, ok := msg.Payload.(Vote)
	if !ok {
		return
	}
	if vote.View < r.View {
		return
	}
	// 叛徒节点：无视收到的 Commit，不累计、也不进入 committed。
	if r.IsTraitor {
		return
	}
	r.recordCommit(vote.Sequence, vote.View, vote.Node)
	r.maybeCommit(vote.Sequence)
}

// recordPrepare 记录一张 prepare 票。
func (r *Replica) recordPrepare(seq, view uint64, node core.NodeID) {
	if _, ok := r.prepared[seq]; !ok {
		r.prepared[seq] = make(map[core.NodeID]bool)
	}
	r.prepared[seq][node] = true
}

// recordCommit 记录一张 commit 票。
func (r *Replica) recordCommit(seq, view uint64, node core.NodeID) {
	if _, ok := r.committed[seq]; !ok {
		r.committed[seq] = make(map[core.NodeID]bool)
	}
	r.committed[seq][node] = true
}

// maybeAdvance 检查 seq 是否收齐 2f+1 个 prepare（含自己）。
// 满足则进入 prepared，并广播 Commit 启动第三阶段。
func (r *Replica) maybeAdvance(seq uint64) {
	if r.isPrepared[seq] {
		return
	}
	if len(r.prepared[seq]) >= r.quorum() {
		r.isPrepared[seq] = true
		// 广播 Commit。
		vote := Vote{
			Sequence:  seq,
			View:      r.View,
			Node:      r.ID,
			Signature: "sig-" + string(r.ID),
		}
		r.recordCommit(seq, r.View, r.ID)
		r.maybeCommit(seq)

		for _, peer := range r.Peers {
			if peer == r.ID {
				continue
			}
			r.transport.Send(core.Message{
				From: r.ID, To: peer,
				Kind:    KindCommit,
				Term:    r.View,
				Payload: vote,
			})
		}
	}
}

// maybeCommit 检查 seq 是否收齐 2f+1 个 commit（含自己），满足则进入 committed。
func (r *Replica) maybeCommit(seq uint64) {
	if r.isCommitted[seq] {
		return
	}
	if len(r.committed[seq]) >= r.quorum() {
		r.isCommitted[seq] = true
		r.CommittedSeqs = append(r.CommittedSeqs, seq)
		// 真实系统在此 apply 到状态机；demo 仅记录 committed 事实。
	}
}

// IsCommitted 报告给定 seq 是否已通过 commit 门槛。
func (r *Replica) IsCommitted(seq uint64) bool {
	return r.isCommitted[seq]
}

// IsPrepared 报告给定 seq 是否已通过 prepare 门槛。
func (r *Replica) IsPrepared(seq uint64) bool {
	return r.isPrepared[seq]
}
