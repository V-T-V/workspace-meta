package viewstamped

import (
	"fmt"

	"github.com/QiuShichang/consensus-atlas/internal/core"
)

// 消息 Kind 常量。VR 分三阶段，本包实现"正常操作 + 视图变更"两阶段
// （恢复 state-transfer 阶段省略，见 NOTES 本包简化）。
//
// 正常操作（Normal Operation，§4.1）：
//   - Request：Client → Primary，发起请求。
//   - Prepare：Primary → Backups，把请求按序广播。
//   - PrepareOK：Backup → Primary，确认已就绪（达 quorum 后执行）。
//   - Reply：Primary → Client，返回执行结果。
//
// 视图变更（View Change，§4.2，Primary 失联时换主）：
//   - StartViewChange：Backup → 所有副本，声明本节点认为需要换 view。
//   - DoViewChange：Backup（凑齐票后）→ 新 Primary 候选，携带自己的日志。
//   - StartView：新 Primary → 所有副本，确立新 view。
const (
	KindRequest         = "Request"         // Client → Primary
	KindPrepare         = "Prepare"         // Primary → Backup
	KindPrepareOK       = "PrepareOK"       // Backup → Primary
	KindReply           = "Reply"           // Primary → Client
	KindStartViewChange = "StartViewChange" // Backup → all
	KindDoViewChange    = "DoViewChange"    // Backup → 新 Primary 候选
	KindStartView       = "StartView"       // 新 Primary → all
)

// ReplicaState 标记一个副本在 VR 协议中的状态。
type ReplicaState int

const (
	StateNormal     ReplicaState = iota // 正常操作（Primary 处理请求）
	StateViewChange                     // 视图变更中
	StateRecovering                     // 恢复中（本包不实现）
)

// String 返回状态的可读名称。
func (s ReplicaState) String() string {
	switch s {
	case StateNormal:
		return "normal"
	case StateViewChange:
		return "view-change"
	case StateRecovering:
		return "recovering"
	default:
		return "unknown"
	}
}

// RequestPayload 是客户端请求的负载（含操作与客户端标识，用于去重/线性一致）。
type RequestPayload struct {
	Op     string // 状态机操作（简化为字符串）
	Client core.NodeID
	ReqNum uint64 // 客户端请求序号（用于去重；本包简化，不强制唯一）
}

// PreparePayload 是 Primary 广播给 Backup 的请求负载。
// Primary 给每个请求分配一个全局递增的 opNumber（决定执行顺序）。
type PreparePayload struct {
	View      uint64 // 当前 view（即 Primary 任期）
	OpNumber  uint64 // 操作序号（全局递增，决定执行顺序）
	CommitNum uint64 // Primary 当前已提交序号（推动 Backup 执行）
	Request   RequestPayload
	// CommitOnly=true 表示这条 Prepare 只为推进 Backup 的 CommitNum（提交广播），
	// 不携带新 op（OpNumber 不变、不记日志、不回 PrepareOK）。避免幽灵日志条目。
	CommitOnly bool
}

// PrepareOKPayload 是 Backup 对 Prepare 的回复：表示已把该 op 记入日志，可执行。
type PrepareOKPayload struct {
	View     uint64
	OpNumber uint64
	From     core.NodeID // 投票的 Backup（便于 Primary 计数）
}

// ReplyPayload 是 Primary 给客户端的执行结果。
type ReplyPayload struct {
	View     uint64
	OpNumber uint64
	Result   string // 执行结果（本包用操作字符串占位）
}

// StartViewChangePayload：Backup 超时后声明要换 view。
type StartViewChangePayload struct {
	View uint64 // 本节点认为的下一个 view（= 当前 view + 1）
	From core.NodeID
}

// DoViewChangePayload：Backup 凑齐 quorum 后发给新 Primary 候选，携带自己的日志快照。
type DoViewChangePayload struct {
	View           uint64          // 候选新 view
	LastNormalView uint64          // 本副本最后处于 normal 的 view（用于选最新日志）
	LogEntries     []core.LogEntry // 本副本日志副本（用于新 Primary 合并）
	From           core.NodeID
}

// StartViewPayload：新 Primary 确立后广播，告诉所有人新 view + 新日志基线。
type StartViewPayload struct {
	View       uint64
	OpNumber   uint64          // 新 Primary 的下一个 opNumber
	LogEntries []core.LogEntry // 新 view 的日志基线
	CommitNum  uint64
}

// opRecord 跟踪一个 opNumber 的 PrepareOK 计数（Primary 用）。
type opRecord struct {
	opNumber uint64
	okCount  int                  // 收到的 PrepareOK 数（含 Primary 自身算 1）
	replied  map[core.NodeID]bool // 哪些 Backup 已就绪
}

// Replica 是一个 VR 副本。VR 中"Primary"是当前 view 下负责排序的副本
// （由 primary = replicas[view % n] 决定，简化为固定顺序第一个）。
//
// 单 goroutine 由外部 Tick / transport 回调驱动，不内部并发，保证 demo 轨迹确定
// （与本仓库 raft/paxos 包一致的工程取向）。
type Replica struct {
	ID    core.NodeID
	Peers []core.NodeID // 含自己；集群总节点数 = len(Peers)
	View  uint64        // 当前 view（Primary 任期）

	State ReplicaState // normal / view-change
	Log   core.Log     // 操作日志（复用 core.Log）

	OpNumber  uint64 // 本副本已分配的最大 opNumber（Primary 自增用）
	CommitNum uint64 // 已执行到的 opNumber

	IsPrimary bool // 是否当前 view 的 Primary

	// Primary 跟踪每个 op 的 PrepareOK 计数。
	pending map[uint64]*opRecord

	// view change 计数：StartViewChange 票数 / DoViewChange 票数。
	viewChangeVotes map[core.NodeID]bool // 已发 StartViewChange 的副本

	transport core.Transport

	// tick 驱动视图变更超时（Backup 检测 Primary 失联）。
	// 与 raft 类似用整数 tick + 错开超时保证 demo 轨迹确定。
	electionTicks   int
	electionTimeout int // Backup 多久没收到 Primary 消息就发起 view change
}

// NewReplica 构造一个 normal 态副本。
// view 初值为 0；调用方应在集群启动后用 StartView 或显式指定首个 Primary。
func NewReplica(id core.NodeID, peers []core.NodeID, view uint64, electionTimeout int, tr core.Transport) *Replica {
	r := &Replica{
		ID:              id,
		Peers:           peers,
		View:            view,
		State:           StateNormal,
		transport:       tr,
		pending:         make(map[uint64]*opRecord),
		viewChangeVotes: make(map[core.NodeID]bool),
		electionTimeout: electionTimeout,
	}
	return r
}

// Start 把副本注册到传输层。
func (r *Replica) Start() {
	r.transport.Install(r.ID, r.handle)
}

// SetPrimary 标记本副本是否当前 view 的 Primary（demo 启动时显式设定首个 Primary）。
func (r *Replica) SetPrimary(b bool) { r.IsPrimary = b }

// 以下 getter 仅供 demo / 测试观测内部状态，不影响协议。
func (r *Replica) GetView() uint64        { return r.View }
func (r *Replica) GetState() ReplicaState { return r.State }
func (r *Replica) GetIsPrimary() bool     { return r.IsPrimary }

// quorum 是执行/视图变更所需的多数派大小（含 Primary 自己）。
func (r *Replica) quorum() int { return len(r.Peers)/2 + 1 }

// HandleRequest 由 Primary 处理客户端请求：分配 opNumber、追加日志、广播 Prepare。
// 非 Primary 返回 false（demo 中客户端应只发给 Primary）。
func (r *Replica) HandleRequest(req RequestPayload) bool {
	if !r.IsPrimary || r.State != StateNormal {
		return false
	}
	r.OpNumber++
	r.Log.Append(r.View, req) // 复用 core.Log，Term 字段存 view

	// 初始化该 op 的 PrepareOK 计数（Primary 自身算 1）。
	rec := &opRecord{opNumber: r.OpNumber, okCount: 1, replied: map[core.NodeID]bool{r.ID: true}}
	r.pending[r.OpNumber] = rec

	// 广播 Prepare 给所有 Backup。
	for _, peer := range r.Peers {
		if peer == r.ID {
			continue
		}
		r.transport.Send(core.Message{
			From: r.ID, To: peer,
			Kind: KindPrepare,
			Term: r.View,
			Payload: PreparePayload{
				View:      r.View,
				OpNumber:  r.OpNumber,
				CommitNum: r.CommitNum,
				Request:   req,
			},
		})
	}
	return true
}

// Tick 由外部驱动一个时间片。
//   - Primary：无周期性心跳逻辑（VR 原版 Primary 不主动发心跳；Backup 靠请求活跃度计时）。
//   - Backup（normal 态）：累计 electionTicks，超时且未收到 Primary 消息则发起 view change。
//   - Backup（view-change 态）：继续累计 electionTicks，超时则**重发** StartViewChange
//     （针对同一目标 view，不递增 view）——保证 view-change 消息持续传播直到凑齐 quorum。
//
// 与 raft 不同：VR 的"心跳"是 Primary 发的 Prepare/Commit 消息本身；本包 Backup
// 在 Tick 里累计超时，demo 通过"显式停止给 Primary 投递消息"模拟 Primary 失联。
func (r *Replica) Tick() {
	if r.IsPrimary {
		return
	}
	r.electionTicks++
	if r.electionTicks < r.electionTimeout {
		return
	}
	// 超时：normal 态发起 view change（view++）；view-change 态重发 StartViewChange（不++）。
	r.startViewChange()
}

// resetElectionTimer 收到 Primary 消息后重置计时（任何来自当前 view Primary 的消息）。
func (r *Replica) resetElectionTimer() {
	r.electionTicks = 0
}

// primaryFor 返回某 view 下"应当担任 Primary"的副本 ID。
// 约定：primary = Peers[(view-1) % len(Peers)]（view 从 1 开始）。
// 这让 view change 的 Primary 候选对所有人确定且一致，避免多副本各自自荐造成脑裂。
func (r *Replica) primaryFor(view uint64) core.NodeID {
	if len(r.Peers) == 0 {
		return r.ID
	}
	return r.Peers[(int(view)-1)%len(r.Peers)]
}

// startViewChange：Backup 发起 view change。
//
// 本包采用**确定性的 view-based Primary 候选**：primaryFor(view) 给出唯一候选，
// 所有副本对同一目标 view 投票；只有候选自己凑齐 quorum 票才发 DoViewChange 当主。
// 这避免了"多个副本各自自荐当主"的脑裂，保证 demo 轨迹确定。
func (r *Replica) startViewChange() {
	// normal 态首次超时：推进到下一个 view；view-change 态重发：保持同一 view。
	if r.State != StateViewChange {
		r.View++
	}
	r.State = StateViewChange
	r.electionTicks = 0
	r.viewChangeVotes[r.ID] = true // 自投（重发场景保留已累计的他人票）

	for _, peer := range r.Peers {
		if peer == r.ID {
			continue
		}
		r.transport.Send(core.Message{
			From: r.ID, To: peer,
			Kind:    KindStartViewChange,
			Term:    r.View,
			Payload: StartViewChangePayload{View: r.View, From: r.ID},
		})
	}
	// 若本副本是该 view 的候选且凑齐 quorum，发 DoViewChange 当主。
	r.maybeBecomeCandidate()
}

// maybeBecomeCandidate：若本副本是当前 view 的 Primary 候选，且已收齐 quorum 票，
// 则向自己发 DoViewChange（携带日志）确立为新 Primary。
func (r *Replica) maybeBecomeCandidate() {
	if r.State != StateViewChange {
		return
	}
	if r.ID != r.primaryFor(r.View) {
		return // 不是候选，只投票
	}
	if len(r.viewChangeVotes) < r.quorum() {
		return
	}
	// 是候选且凑齐票：发 DoViewChange 给自己（携带日志）。
	logCopy := make([]core.LogEntry, len(r.Log.Entries))
	copy(logCopy, r.Log.Entries)
	r.transport.Send(core.Message{
		From: r.ID, To: r.ID,
		Kind: KindDoViewChange,
		Term: r.View,
		Payload: DoViewChangePayload{
			View:           r.View,
			LastNormalView: r.View,
			LogEntries:     logCopy,
			From:           r.ID,
		},
	})
}

// sendDoViewChange 保留为兼容入口（实际逻辑移入 maybeBecomeCandidate）。
func (r *Replica) sendDoViewChange() {
	r.maybeBecomeCandidate()
}

// handle 是传输层回调：分发各类 VR 消息。
func (r *Replica) handle(msg core.Message) (core.Message, bool) {
	switch msg.Kind {
	case KindPrepare:
		return r.handlePrepare(msg), true
	case KindPrepareOK:
		r.handlePrepareOK(msg)
		return core.Message{}, false
	case KindStartViewChange:
		r.handleStartViewChange(msg)
		return core.Message{}, false
	case KindDoViewChange:
		r.handleDoViewChange(msg)
		return core.Message{}, false
	case KindStartView:
		r.handleStartView(msg)
		return core.Message{}, false
	}
	return core.Message{}, false
}

// handlePrepare：Backup 收到 Primary 的 Prepare——记入日志、回 PrepareOK、推进 commit。
func (r *Replica) handlePrepare(msg core.Message) core.Message {
	p, ok := msg.Payload.(PreparePayload)
	if !ok {
		return core.Message{}
	}
	// 只接受当前 view 的 Prepare（旧 view 忽略）。
	if p.View < r.View {
		return core.Message{}
	}
	// 收到新 view 的 Prepare：认此 Primary，更新 view，重置计时。
	if p.View > r.View {
		r.View = p.View
	}
	r.IsPrimary = false
	r.State = StateNormal
	r.resetElectionTimer()

	// CommitOnly 的提交广播：仅推进 Backup 的 CommitNum，不记日志、不回 PrepareOK。
	if p.CommitOnly {
		if p.CommitNum > r.CommitNum {
			r.CommitNum = p.CommitNum
		}
		return core.Message{}
	}

	// 真正的 Prepare：记入日志（去重，仅当 opNumber 是新的）。
	if p.OpNumber > r.Log.LastIndex() {
		r.Log.Append(p.View, p.Request)
	}
	if p.OpNumber > r.OpNumber {
		r.OpNumber = p.OpNumber
	}

	// 推进 commit（Primary 的 CommitNum 推动 Backup 执行）。
	if p.CommitNum > r.CommitNum {
		r.CommitNum = p.CommitNum
	}

	// 回 PrepareOK。
	return core.Message{
		From: r.ID, To: msg.From,
		Kind:    KindPrepareOK,
		Term:    r.View,
		Payload: PrepareOKPayload{View: r.View, OpNumber: p.OpNumber, From: r.ID},
	}
}

// handlePrepareOK：Primary 收到 Backup 的 PrepareOK，达 quorum 即执行 + 回 Reply。
func (r *Replica) handlePrepareOK(msg core.Message) {
	p, ok := msg.Payload.(PrepareOKPayload)
	if !ok {
		return
	}
	if !r.IsPrimary || r.State != StateNormal {
		return
	}
	rec, exists := r.pending[p.OpNumber]
	if !exists {
		return
	}
	if rec.replied[p.From] {
		return
	}
	rec.replied[p.From] = true
	rec.okCount++

	// 达 quorum（含 Primary 自己）→ 提交并广播 Reply（含推进 commit）。
	if rec.okCount >= r.quorum() && r.CommitNum < p.OpNumber {
		r.CommitNum = p.OpNumber
		// 给所有 Backup 发一条 CommitOnly 的 Prepare 推进它们的 commit。
		// （不携带新 op，Backup 不会记日志、不会回 PrepareOK——避免幽灵条目）。
		for _, peer := range r.Peers {
			if peer == r.ID {
				continue
			}
			r.transport.Send(core.Message{
				From: r.ID, To: peer,
				Kind: KindPrepare,
				Term: r.View,
				Payload: PreparePayload{
					View:       r.View,
					OpNumber:   r.OpNumber,
					CommitNum:  r.CommitNum,
					CommitOnly: true,
				},
			})
		}
		// 给"客户端"（demo 用 Primary 自己代收）回 Reply。
		r.transport.Send(core.Message{
			From: r.ID, To: r.ID,
			Kind:    KindReply,
			Term:    r.View,
			Payload: ReplyPayload{View: r.View, OpNumber: p.OpNumber, Result: fmt.Sprintf("done:%d", p.OpNumber)},
		})
	}
}

// handleStartViewChange：收到其他副本的 StartViewChange，自己也加入换 view 阵营。
func (r *Replica) handleStartViewChange(msg core.Message) {
	p, ok := msg.Payload.(StartViewChangePayload)
	if !ok {
		return
	}
	// 已是更高或同 view 的 Primary：忽略（不降级，避免脑裂）。
	if r.IsPrimary && r.State == StateNormal && p.View <= r.View {
		return
	}
	// 只响应更新的 view 变更；同 view 的重复投票也接受（幂等，set 去重）。
	if p.View < r.View {
		return
	}
	if p.View > r.View {
		r.View = p.View
		r.State = StateViewChange
		r.IsPrimary = false
	}
	r.viewChangeVotes[p.From] = true

	// 若本副本是该 view 的候选且凑齐 quorum，发 DoViewChange 当主。
	r.maybeBecomeCandidate()
}

// handleDoViewChange：新 Primary 候选收到 DoViewChange（含自身自荐），确立新 view。
func (r *Replica) handleDoViewChange(msg core.Message) {
	p, ok := msg.Payload.(DoViewChangePayload)
	if !ok {
		return
	}
	if p.View < r.View {
		return
	}
	// 接受并成为新 view 的 Primary：用对方（或自己）的日志作为基线。
	if p.View > r.View || len(p.LogEntries) > len(r.Log.Entries) {
		// 用较新的日志覆盖（简化：取条目更多者）。
		if len(p.LogEntries) > len(r.Log.Entries) {
			r.Log.Entries = append([]core.LogEntry(nil), p.LogEntries...)
		}
	}
	r.View = p.View
	r.IsPrimary = true
	r.State = StateNormal
	r.electionTicks = 0
	r.viewChangeVotes = map[core.NodeID]bool{}
	if r.OpNumber < uint64(len(r.Log.Entries)) {
		r.OpNumber = uint64(len(r.Log.Entries))
	}

	// 广播 StartView 确立新 view。
	logCopy := make([]core.LogEntry, len(r.Log.Entries))
	copy(logCopy, r.Log.Entries)
	for _, peer := range r.Peers {
		if peer == r.ID {
			continue
		}
		r.transport.Send(core.Message{
			From: r.ID, To: peer,
			Kind: KindStartView,
			Term: r.View,
			Payload: StartViewPayload{
				View:       r.View,
				OpNumber:   r.OpNumber,
				LogEntries: logCopy,
				CommitNum:  r.CommitNum,
			},
		})
	}
}

// handleStartView：Backup 收到新 Primary 的 StartView，认主并同步日志。
func (r *Replica) handleStartView(msg core.Message) {
	p, ok := msg.Payload.(StartViewPayload)
	if !ok {
		return
	}
	if p.View < r.View {
		return
	}
	r.View = p.View
	r.IsPrimary = false
	r.State = StateNormal
	r.electionTicks = 0
	r.viewChangeVotes = map[core.NodeID]bool{}
	// 用新 Primary 的日志覆盖（简化：直接替换，假设新 Primary 是权威）。
	if len(p.LogEntries) >= len(r.Log.Entries) {
		r.Log.Entries = append([]core.LogEntry(nil), p.LogEntries...)
	}
	r.OpNumber = p.OpNumber
	r.CommitNum = p.CommitNum
}
