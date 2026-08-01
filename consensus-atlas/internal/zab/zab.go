package zab

import (
	"fmt"

	"github.com/QiuShichang/consensus-atlas/internal/core"
)

// 消息 Kind 常量。ZAB 的广播阶段（broadcast）三消息：
//   - Proposal：Leader → Follower，发提案（带全局递增 zxid）。
//   - Ack：Follower → Leader，确认已把提案记入事务日志。
//   - Commit/Forward（本包用 Commit）：Leader → Follower，告诉 Follower 可以提交执行。
//
// 注：ZAB 还有 discovery（发现阶段，选 leader + 发现最新已提交 zxid）和
// synchronization（同步阶段，把 leader 的事务历史同步给 follower）两个阶段。
// 本包只实现 broadcast 阶段（见 NOTES 本包简化）。
const (
	KindProposal = "Proposal"
	KindAck      = "Ack"
	KindCommit   = "Commit"
)

// ZXID 是 ZooKeeper 的事务 ID（64 位）：
//
//	高 32 位 = epoch（纪元，每次 leader 切换 +1）
//	低 32 位 = counter（本 epoch 内递增的事务计数）
//
// 这个设计保证全局有序：同 epoch 内 counter 递增；跨 epoch 高位变大。
// Leader 分配 zxid，所有副本按 zxid 顺序提交——这是 ZAB"主备原子广播"的核心。
type ZXID uint64

// Epoch 返回 zxid 的高 32 位（纪元）。
func (z ZXID) Epoch() uint32 { return uint32(z >> 32) }

// Counter 返回 zxid 的低 32 位（本纪元内计数）。
func (z ZXID) Counter() uint32 { return uint32(z & 0xffffffff) }

// String 返回 "epoch:counter" 形式。
func (z ZXID) String() string {
	return fmt.Sprintf("%d:%d", z.Epoch(), z.Counter())
}

// MakeZXID 由 epoch 和 counter 组装一个 ZXID。
func MakeZXID(epoch uint32, counter uint32) ZXID {
	return ZXID(uint64(epoch)<<32 | uint64(counter))
}

// ProposalPayload 是 Leader 广播的提案负载。
type ProposalPayload struct {
	ZXID    ZXID
	Request string // 状态机操作（简化为字符串）
}

// AckPayload 是 Follower 对 Proposal 的确认。
type AckPayload struct {
	ZXID ZXID
	From core.NodeID
}

// CommitPayload 是 Leader 通知 Follower 提交的负载。
type CommitPayload struct {
	ZXID ZXID
}

// Proposal 记录一个提案的状态（Leader 用）：是否凑齐 quorum Ack、是否已 Commit。
type proposal struct {
	zxid      ZXID
	acks      int                  // 已收 Ack 数（含 Leader 自身算 1）
	ackFrom   map[core.NodeID]bool // 哪些副本已 Ack
	committed bool
}

// txn 是一个已记入日志但可能未提交的事务（Follower 用）。
type txn struct {
	zxid    ZXID
	request string
}

// Leader 是 ZAB 的领导者（主）。它负责：
//   - 给每个客户端请求分配全局递增的 zxid；
//   - 广播 Proposal 给所有 Follower；
//   - 收 quorum Ack 后广播 Commit；
//   - 维护一个 FIFO 的 commit 队列（保证按 zxid 顺序提交）。
//
// 单 goroutine 由外部 transport 回调驱动，不内部并发，保证 demo 轨迹确定。
type Leader struct {
	ID      core.NodeID
	Peers   []core.NodeID // 含自己；集群总节点数 = len(Peers)
	Epoch   uint32        // 当前纪元（每次 leader 切换 +1；本包简化为常量）
	counter uint32        // 本纪元内已分配的最大 counter

	// pending[zxid] 跟踪进行中的提案。Commit 后保留以便观测。
	pending map[ZXID]*proposal

	// committedZXIDs 按提交顺序记录所有已 commit 的 zxid（用于 demo/测试断言顺序）。
	committedZXIDs []ZXID

	transport core.Transport
}

// NewLeader 构造一个 Leader，epoch 为初始纪元。
func NewLeader(id core.NodeID, peers []core.NodeID, epoch uint32, tr core.Transport) *Leader {
	return &Leader{
		ID:        id,
		Peers:     peers,
		Epoch:     epoch,
		pending:   make(map[ZXID]*proposal),
		transport: tr,
	}
}

// Start 把 Leader 注册到传输层。
func (l *Leader) Start() {
	l.transport.Install(l.ID, l.handle)
}

// quorum 是提交所需的多数派大小（含 Leader 自己）。
func (l *Leader) quorum() int { return len(l.Peers)/2 + 1 }

// nextZXID 分配下一个 zxid：counter++，组装成 (epoch, counter)。
func (l *Leader) nextZXID() ZXID {
	l.counter++
	return MakeZXID(l.Epoch, l.counter)
}

// Propose 处理一个客户端请求：分配 zxid、广播 Proposal、记入 pending（自身算 1 Ack）。
// ZAB 保证 zxid 全局单调递增，故所有提案按 Propose 调用顺序获得严格递增的 zxid。
func (l *Leader) Propose(request string) ZXID {
	zxid := l.nextZXID()
	l.pending[zxid] = &proposal{
		zxid:    zxid,
		acks:    1, // Leader 自己已"记入日志"
		ackFrom: map[core.NodeID]bool{l.ID: true},
	}
	for _, peer := range l.Peers {
		if peer == l.ID {
			continue
		}
		l.transport.Send(core.Message{
			From: l.ID, To: peer,
			Kind:    KindProposal,
			Payload: ProposalPayload{ZXID: zxid, Request: request},
		})
	}
	return zxid
}

// handle 是传输层回调：分发 Ack。
func (l *Leader) handle(msg core.Message) (core.Message, bool) {
	switch msg.Kind {
	case KindAck:
		a, ok := msg.Payload.(AckPayload)
		if !ok {
			return core.Message{}, false
		}
		l.handleAck(a)
		return core.Message{}, false
	}
	return core.Message{}, false
}

// handleAck 累计 Ack；达 quorum 即 Commit（广播 Commit + 推进本地提交队列）。
//
// ZAB 的提交顺序保证：Leader 按提案顺序（zxid 递增）提交，不乱序。
// 本包用"提案到达 quorum 即可 commit，但只 commit zxid 连续无空洞的最小者"保证顺序——
// 由于 Leader 串行 Propose 且 Follower FIFO 处理，ack 通常按序到达，简化为
// "达 quorum 立即 commit"即可保持顺序（demo 验证）。
func (l *Leader) handleAck(a AckPayload) {
	p, ok := l.pending[a.ZXID]
	if !ok {
		return
	}
	if p.ackFrom[a.From] {
		return
	}
	p.ackFrom[a.From] = true
	p.acks++

	if p.acks >= l.quorum() && !p.committed {
		l.commit(a.ZXID)
	}
}

// commit 把某 zxid 标记为已提交，广播 Commit，并记入提交顺序。
func (l *Leader) commit(zxid ZXID) {
	p, ok := l.pending[zxid]
	if !ok {
		return
	}
	p.committed = true
	l.committedZXIDs = append(l.committedZXIDs, zxid)

	for _, peer := range l.Peers {
		if peer == l.ID {
			continue
		}
		l.transport.Send(core.Message{
			From: l.ID, To: peer,
			Kind:    KindCommit,
			Payload: CommitPayload{ZXID: zxid},
		})
	}
}

// CommittedZXIDs 返回已提交 zxid 的有序列表（demo/测试用）。
func (l *Leader) CommittedZXIDs() []ZXID {
	out := make([]ZXID, len(l.committedZXIDs))
	copy(out, l.committedZXIDs)
	return out
}

// Follower 是 ZAB 的从节点。它接收 Leader 的 Proposal（记入事务日志）、回 Ack，
// 收到 Commit 时按 zxid 顺序提交执行。
type Follower struct {
	ID    core.NodeID
	Epoch uint32

	// log 按 zxid 排序记录所有已收到的事务（含未提交）。
	log []txn
	// committed 标记某 zxid 是否已提交。
	committed map[ZXID]bool
	// committedZXIDs 按提交到达顺序记录（验证按序）。
	committedZXIDs []ZXID

	transport core.Transport
}

// NewFollower 构造一个 Follower。
func NewFollower(id core.NodeID, epoch uint32, tr core.Transport) *Follower {
	return &Follower{
		ID:        id,
		Epoch:     epoch,
		committed: make(map[ZXID]bool),
		transport: tr,
	}
}

// Start 把 Follower 注册到传输层。
func (f *Follower) Start() {
	f.transport.Install(f.ID, f.handle)
}

// handle 是传输层回调：分发 Proposal / Commit。
func (f *Follower) handle(msg core.Message) (core.Message, bool) {
	switch msg.Kind {
	case KindProposal:
		p, ok := msg.Payload.(ProposalPayload)
		if !ok {
			return core.Message{}, false
		}
		return f.handleProposal(msg.From, p), true
	case KindCommit:
		c, ok := msg.Payload.(CommitPayload)
		if !ok {
			return core.Message{}, false
		}
		f.handleCommit(c)
		return core.Message{}, false
	}
	return core.Message{}, false
}

// handleProposal 把事务记入日志（按 zxid 顺序），回 Ack。
// ZAB 要求 Follower 按 zxid 顺序处理 Proposal（FIFO）；本包假设 Proposal 按序到达，
// 直接追加。
func (f *Follower) handleProposal(from core.NodeID, p ProposalPayload) core.Message {
	f.log = append(f.log, txn{zxid: p.ZXID, request: p.Request})
	return core.Message{
		From: f.ID, To: from,
		Kind:    KindAck,
		Payload: AckPayload{ZXID: p.ZXID, From: f.ID},
	}
}

// handleCommit 标记某 zxid 为已提交，按 zxid 顺序推进执行。
// 简化：直接记入 committed 集合 + committedZXIDs 列表（demo 验证 zxid 递增）。
func (f *Follower) handleCommit(c CommitPayload) {
	if f.committed[c.ZXID] {
		return
	}
	f.committed[c.ZXID] = true
	f.committedZXIDs = append(f.committedZXIDs, c.ZXID)
}

// CommittedZXIDs 返回本 Follower 已提交 zxid 的（按到达顺序）列表。
func (f *Follower) CommittedZXIDs() []ZXID {
	out := make([]ZXID, len(f.committedZXIDs))
	copy(out, f.committedZXIDs)
	return out
}

// LogLen 返回已收到的事务数（含未提交）。
func (f *Follower) LogLen() int { return len(f.log) }
