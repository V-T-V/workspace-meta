package twopc

import (
	"fmt"

	"github.com/QiuShichang/consensus-atlas/internal/core"
)

// 消息 Kind 常量。两阶段提交的两个阶段由 Prepare→Vote 和 Commit/Abort→Ack 两轮
// 请求/响应组成：
//   - 阶段一（Prepare/Vote）：Coordinator 问"能否提交？"，Participant 回 Yes/No。
//   - 阶段二（Commit/Abort）：Coordinator 据投票结果决定 Commit 或 Abort，Participant 执行后回 Ack。
const (
	KindPrepare = "Prepare" // Coordinator → Participant：能否提交？
	KindVote    = "Vote"    // Participant → Coordinator：Yes / No
	KindCommit  = "Commit"  // Coordinator → Participant：落实提交
	KindAbort   = "Abort"   // Coordinator → Participant：放弃
	KindAck     = "Ack"     // Participant → Coordinator：已执行（committed/aborted）
)

// TxnID 标识一个分布式事务。一个 Coordinator 可同时推进多个独立事务。
type TxnID string

// ParticipantState 标记一个 Participant 在某事务中的状态（状态机）。
//
//	Init     → 收到 Prepare：
//	            能提交 → Prepared（锁定资源，承诺必能 Commit）
//	            不能  → Aborted（直接拒绝，事务夭折）
//	Prepared → 收到 Commit → Committed
//	          收到 Abort  → Aborted（罕见：Coordinator 在全 Yes 后仍决定放弃，如自身故障恢复）
//	Committed / Aborted：终态。
type ParticipantState int

const (
	StateInit ParticipantState = iota
	StatePrepared
	StateCommitted
	StateAborted
)

// String 返回状态的可读名称。
func (s ParticipantState) String() string {
	switch s {
	case StateInit:
		return "init"
	case StatePrepared:
		return "prepared"
	case StateCommitted:
		return "committed"
	case StateAborted:
		return "aborted"
	default:
		return "unknown"
	}
}

// PreparePayload 是 Coordinator 阶段一的负载：询问某事务能否提交。
type PreparePayload struct {
	TxnID TxnID
}

// VotePayload 是 Participant 对 Prepare 的回复。Yes=true 表示承诺可提交
// （此时 Participant 已锁定资源并持久化 Prepared 状态，必须能履行后续 Commit）。
type VotePayload struct {
	TxnID TxnID
	Yes   bool
}

// DecisionPayload 是 Coordinator 阶段二的负载：Commit 或 Abort 二选一。
// 两个字段共用一个结构体便于统一分发；Coordinator 发送时只填一个。
type DecisionPayload struct {
	TxnID  TxnID
	Commit bool // true=Commit，false=Abort
}

// AckPayload 是 Participant 对 Decision 的确认。Committed=true 表示已提交，
// 否则已放弃。
type AckPayload struct {
	TxnID     TxnID
	Committed bool
}

// txnRecord 是 Coordinator 跟踪一个进行中事务的状态。
type txnRecord struct {
	id      TxnID
	yes     int  // 已收到 Yes 票数
	no      int  // 已收到 No 票数（任一 No 即决定 Abort）
	decided bool // 是否已下发 Decision（避免重复决定）
	commit  bool // 决定结果：true=Commit
	replied map[core.NodeID]bool
}

// Coordinator 是两阶段提交的协调者。它持有 Participant 列表和若干进行中的事务，
// 通过 transport 与 Participant 通信。单 goroutine 由外部 Begin/Tick 驱动，
// 不内部并发，保证 demo 执行轨迹确定。
//
// 2PC 的 quorum 是**一致同意（unanimity）**：必须**全部** Participant 投 Yes 才能 Commit，
// 任一 No（或超时）即 Abort——这是 2PC 与 Raft/Paxos"多数派"的关键区别。
type Coordinator struct {
	ID           core.NodeID
	Participants []core.NodeID
	transport    core.Transport

	// pending 跟踪进行中事务。Begin 时插入，事务进入终态后保留以便观测。
	pending map[TxnID]*txnRecord

	// Outcome 记录每个已决事务的最终结果（commit=true/abort=false），便于 demo/测试断言。
	Outcome map[TxnID]bool
}

// NewCoordinator 构造一个协调者。participants 为参与方 ID 列表（不含协调者自身）。
func NewCoordinator(id core.NodeID, participants []core.NodeID, tr core.Transport) *Coordinator {
	return &Coordinator{
		ID:           id,
		Participants: participants,
		transport:    tr,
		pending:      make(map[TxnID]*txnRecord),
		Outcome:      make(map[TxnID]bool),
	}
}

// Start 把协调者注册到传输层，开始接收 Participant 的 Vote/Ack 回复。
func (c *Coordinator) Start() {
	c.transport.Install(c.ID, c.handle)
}

// Begin 开启一个新事务的阶段一：向所有 Participant 广播 Prepare。
// 返回事务记录指针供调用方观测进度。重复 TxnID 会被拒绝（返回 error）。
func (c *Coordinator) Begin(id TxnID) (*txnRecord, error) {
	if _, exists := c.pending[id]; exists {
		return nil, fmt.Errorf("事务 %s 已存在", id)
	}
	rec := &txnRecord{
		id:      id,
		replied: make(map[core.NodeID]bool),
	}
	c.pending[id] = rec

	for _, p := range c.Participants {
		c.transport.Send(core.Message{
			From: c.ID, To: p,
			Kind:    KindPrepare,
			Payload: PreparePayload{TxnID: id},
		})
	}
	return rec, nil
}

// handle 是传输层回调：分发 Vote / Ack 两类回复。
func (c *Coordinator) handle(msg core.Message) (core.Message, bool) {
	switch msg.Kind {
	case KindVote:
		v, ok := msg.Payload.(VotePayload)
		if !ok {
			return core.Message{}, false
		}
		c.handleVote(msg.From, v)
		return core.Message{}, false
	case KindAck:
		a, ok := msg.Payload.(AckPayload)
		if !ok {
			return core.Message{}, false
		}
		c.handleAck(msg.From, a)
		return core.Message{}, false
	}
	return core.Message{}, false
}

// handleVote 处理一张投票。全 Yes → 决定 Commit；任一 No → 决定 Abort。
// 决定只下发一次（rec.decided 守卫）。
func (c *Coordinator) handleVote(from core.NodeID, v VotePayload) {
	rec, ok := c.pending[v.TxnID]
	if !ok || rec.decided {
		return // 未知事务或已决定，忽略
	}
	if rec.replied[from] {
		return // 每个 Participant 只计一票
	}
	rec.replied[from] = true
	if v.Yes {
		rec.yes++
	} else {
		rec.no++
	}

	// 任一 No 立即决定 Abort（无需等其他票）。
	if rec.no > 0 {
		c.decide(rec, false)
		return
	}
	// 全部 Yes 才 Commit（unanimity：需收齐所有 Participant 的 Yes）。
	if rec.yes == len(c.Participants) {
		c.decide(rec, true)
	}
}

// decide 下发阶段二的 Decision（Commit 或 Abort）给所有 Participant，并记录结果。
func (c *Coordinator) decide(rec *txnRecord, commit bool) {
	rec.decided = true
	rec.commit = commit
	c.Outcome[rec.id] = commit

	kind := KindCommit
	if !commit {
		kind = KindAbort
	}
	for _, p := range c.Participants {
		c.transport.Send(core.Message{
			From: c.ID, To: p,
			Kind:    kind,
			Payload: DecisionPayload{TxnID: rec.id, Commit: commit},
		})
	}
}

// handleAck 记录 Participant 的最终确认。仅用于观测/测试；不影响协议推进。
func (c *Coordinator) handleAck(from core.NodeID, a AckPayload) {
	// 记录到 Outcome 已在 decide 完成，这里仅做存在性校验（保持处理函数存在，
	// 避免 Ack 被当作未知消息丢弃）。
	if _, ok := c.pending[a.TxnID]; !ok {
		return
	}
}

// txnState 记录 Participant 对每个事务的状态（按 TxnID 索引）。
// 谓词成员嵌入结构体便于 Participant.handle 内联判断。

// canPrepare 控制 Participant 是否承诺提交（投 Yes）。
// demo 用它模拟"能否提交"的本地条件（资源可用、约束满足等）。
// 返回 true=可提交。默认实现总是返回 true（总能提交）；
// 调用方可在 Begin 前注入自定义谓词来制造 No 投票场景。
type canCommitFn func(id TxnID) bool

// Participant 是两阶段提交的参与方。它对 Prepare 回 Yes/No，对 Commit/Abort 落实
// 后回 Ack。单 goroutine 由 transport 回调驱动，不内部并发。
//
// 关键约束（2PC 的正确性根基）：投 Yes 后 Participant 进入 Prepared 态并锁定资源，
// 必须**能履行**后续 Commit（即使故障恢复也要能补提交）。这是 2PC 的 durability 假设。
type Participant struct {
	ID        core.NodeID
	transport core.Transport

	// state 按事务 ID 记录本参与方的状态机。
	state map[TxnID]ParticipantState

	// canCommit 决定对某事务投 Yes 还是 No。nil 表示总投 Yes。
	canCommit canCommitFn
}

// NewParticipant 构造一个总投 Yes 的参与方。
func NewParticipant(id core.NodeID, tr core.Transport) *Participant {
	return &Participant{
		ID:        id,
		transport: tr,
		state:     make(map[TxnID]ParticipantState),
		canCommit: func(TxnID) bool { return true },
	}
}

// SetCanCommit 注入一个谓词，控制对每个事务投 Yes/No。
// 用于 demo 制造"某个 Participant 拒绝"的场景。
func (p *Participant) SetCanCommit(fn func(id TxnID) bool) {
	if fn == nil {
		p.canCommit = func(TxnID) bool { return true }
		return
	}
	p.canCommit = fn
}

// Start 把参与方注册到传输层，开始接收 Coordinator 的 Prepare/Commit/Abort。
func (p *Participant) Start() {
	p.transport.Install(p.ID, p.handle)
}

// State 返回本参与方在某事务上的状态；未知事务返回 StateInit。
func (p *Participant) State(id TxnID) ParticipantState {
	return p.state[id]
}

// handle 是传输层回调：分发 Prepare / Commit / Abort 三类消息。
func (p *Participant) handle(msg core.Message) (core.Message, bool) {
	switch msg.Kind {
	case KindPrepare:
		pr, ok := msg.Payload.(PreparePayload)
		if !ok {
			return core.Message{}, false
		}
		return p.handlePrepare(msg.From, pr), true
	case KindCommit:
		d, ok := msg.Payload.(DecisionPayload)
		if !ok {
			return core.Message{}, false
		}
		return p.handleDecision(msg.From, d, true), true
	case KindAbort:
		d, ok := msg.Payload.(DecisionPayload)
		if !ok {
			return core.Message{}, false
		}
		return p.handleDecision(msg.From, d, false), true
	}
	return core.Message{}, false
}

// handlePrepare 处理 Prepare：能提交则进 Prepared 并回 Yes，否则进 Aborted 并回 No。
//
// 注意：状态机约束——已是 Prepared/Committed/Aborted 的事务收到重复 Prepare 时
// 保持原决定（幂等），不重复锁定资源。这是处理消息重传的稳健做法。
func (p *Participant) handlePrepare(from core.NodeID, pr PreparePayload) core.Message {
	switch p.state[pr.TxnID] {
	case StatePrepared:
		// 已承诺，重复 Prepare 直接再回一次 Yes（幂等）。
		return p.vote(from, pr.TxnID, true)
	case StateCommitted, StateAborted:
		// 已终态，忽略（不重复回票，避免干扰 Coordinator 计数）。
		return core.Message{}
	}

	if p.canCommit(pr.TxnID) {
		// 锁定资源 + 持久化 Prepared 状态（本教学库不真持久化，仅状态转移）。
		p.state[pr.TxnID] = StatePrepared
		return p.vote(from, pr.TxnID, true)
	}
	// 不能提交：直接夭折，回 No。Coordinator 收到任一 No 即 Abort。
	p.state[pr.TxnID] = StateAborted
	return p.vote(from, pr.TxnID, false)
}

// handleDecision 处理阶段二的 Commit/Abort：落实状态并回 Ack。
//
// Commit 仅在 Prepared 态合法（履行之前的承诺）；Abort 在任何非 Committed 态合法。
func (p *Participant) handleDecision(from core.NodeID, d DecisionPayload, commit bool) core.Message {
	if commit {
		// 只有 Prepared 态可推进到 Committed；Init/Aborted 收到 Commit 是协议异常，
		// 保持原状（Aborted 不应被 Commit 覆盖——这是 2PC 安全性所在）。
		if p.state[d.TxnID] == StatePrepared || p.state[d.TxnID] == StateCommitted {
			p.state[d.TxnID] = StateCommitted
		}
		return p.ack(from, d.TxnID, true)
	}
	// Abort：非 Committed 态都可转入 Aborted；已 Committed 不回退（不可撤销）。
	if p.state[d.TxnID] != StateCommitted {
		p.state[d.TxnID] = StateAborted
	}
	return p.ack(from, d.TxnID, false)
}

func (p *Participant) vote(to core.NodeID, id TxnID, yes bool) core.Message {
	return core.Message{
		From: p.ID, To: to,
		Kind:    KindVote,
		Payload: VotePayload{TxnID: id, Yes: yes},
	}
}

func (p *Participant) ack(to core.NodeID, id TxnID, committed bool) core.Message {
	return core.Message{
		From: p.ID, To: to,
		Kind:    KindAck,
		Payload: AckPayload{TxnID: id, Committed: committed},
	}
}
