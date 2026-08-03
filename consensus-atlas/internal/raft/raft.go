package raft

import (
	"github.com/QiuShichang/consensus-atlas/internal/core"
)

// 消息 Kind 常量。
const (
	KindRequestVoteRequest    = "RequestVoteRequest"
	KindRequestVoteResponse   = "RequestVoteResponse"
	KindAppendEntriesRequest  = "AppendEntriesRequest"
	KindAppendEntriesResponse = "AppendEntriesResponse"
)

// RequestVoteRequest 是候选人请求投票的负载。
type RequestVoteRequest struct {
	LastLogIndex uint64 // 候选人最后一条日志的 Index
	LastLogTerm  uint64 // 候选人最后一条日志的 Term
}

// RequestVoteResponse 是节点对投票请求的回复。
type RequestVoteResponse struct {
	VoteGranted bool // 是否同意投票
}

// AppendEntriesRequest 是 Leader 复制日志/发送心跳的负载。
type AppendEntriesRequest struct {
	PrevLogIndex uint64          // 紧接新条目之前的日志 Index（0 表示空日志前）
	PrevLogTerm  uint64          // PrevLogIndex 处的 Term
	Entries      []core.LogEntry // 待复制的日志条目（心跳时为空）
	LeaderCommit uint64          // Leader 的 commitIndex
}

// AppendEntriesResponse 是 Follower 对 AppendEntries 的回复。
type AppendEntriesResponse struct {
	Success    bool   // 是否接受（prevIndex/prevTerm 匹配则 true）
	MatchIndex uint64 // 接受后 Follower 的最后一条日志 Index（便于 Leader 推进 matchIndex）
}

// Node 是一个 Raft 节点（Server）。单 goroutine 驱动（由 demo 的 tick 调用），
// 不内部并发，保证轨迹确定。
type Node struct {
	ID          core.NodeID
	Peers       []core.NodeID
	State       core.NodeState
	CurrentTerm uint64
	VotedFor    *core.NodeID // 本任期投给谁（nil 表示未投）
	Log         core.Log
	CommitIndex uint64
	LastApplied uint64

	// Leader 选举超时（以 tick 计）。收到 Leader 心跳会重置。
	// 不同节点设不同值，避免多个候选人同时觉醒（随机化选举超时，论文 §5.4）。
	ElectionTimeout int
	electionTicks   int // 自上次重置以来累计的 tick（Follower/Candidate 用）
	heartbeatTicks  int // Leader 自上次心跳以来累计的 tick

	// Leader 状态：每个 Follower 的下一个待复制 Index。
	nextIndex  map[core.NodeID]uint64
	matchIndex map[core.NodeID]uint64

	transport core.Transport
	votes     int // 本任期收到的票数
}

// NewNode 构造一个 Follower 节点。
func NewNode(id core.NodeID, peers []core.NodeID, electionTimeout int, tr core.Transport) *Node {
	n := &Node{
		ID:              id,
		Peers:           peers,
		State:           core.StateFollower,
		ElectionTimeout: electionTimeout,
		nextIndex:       make(map[core.NodeID]uint64),
		matchIndex:      make(map[core.NodeID]uint64),
		transport:       tr,
	}
	return n
}

// Start 把节点注册到传输层，开始接收消息。
func (n *Node) Start() {
	n.transport.Install(n.ID, n.handle)
}

// Tick 由外部驱动一个时间片。
//   - Follower/Candidate：累计选举超时，达 ElectionTimeout 则发起选举。
//   - Leader：每 heartbeatTicks 个 tick 广播一次 AppendEntries（空条目当心跳），
//     重置 Follower 的选举时钟，防止 Follower 因长期无消息而发起不必要的选举。
//
// 注：标准 Raft 用真实时间 + 随机化选举超时；本教学库用整数 tick + 各节点
// 错开的固定 ElectionTimeout 保证 demo 轨迹确定（见 NOTES.md 说明）。
func (n *Node) Tick() {
	if n.State == core.StateLeader {
		n.heartbeatTicks++
		// 心跳间隔 = 选举超时的一半（向下取整，至少 1）。
		if n.heartbeatTicks >= n.heartbeatInterval() {
			n.heartbeatTicks = 0
			n.broadcastAppendEntries()
		}
		return
	}
	n.electionTicks++
	if n.electionTicks >= n.ElectionTimeout {
		n.startElection()
	}
}

// heartbeatInterval 返回 Leader 心跳间隔（tick 数）。
// 约定为 ElectionTimeout 的一半，保证心跳频率高于选举超时。
func (n *Node) heartbeatInterval() int {
	h := n.ElectionTimeout / 2
	if h < 1 {
		h = 1
	}
	return h
}

// startElection 发起一轮选举：变 Candidate、自增 term、投自己、向 peers 请求投票。
func (n *Node) startElection() {
	n.State = core.StateCandidate
	n.CurrentTerm++
	n.VotedFor = &n.ID
	n.votes = 1 // 自投
	n.electionTicks = 0

	req := RequestVoteRequest{
		LastLogIndex: n.Log.LastIndex(),
		LastLogTerm:  n.Log.LastTerm(),
	}
	for _, peer := range n.Peers {
		if peer == n.ID {
			continue
		}
		n.transport.Send(core.Message{
			From: n.ID, To: peer,
			Kind:    KindRequestVoteRequest,
			Term:    n.CurrentTerm,
			Payload: req,
		})
	}
	// 单节点集群立即当选。
	if n.votes >= n.quorum() {
		n.becomeLeader()
	}
}

// quorum 是当选/提交所需的多数派大小。
// Peers 列表包含自己（见 NewNode），所以集群总节点数 = len(Peers)。
// 多数派 = floor(total/2) + 1。
func (n *Node) quorum() int {
	return len(n.Peers)/2 + 1
}

// becomeLeader 转为 Leader：初始化 nextIndex/matchIndex，立即广播一次心跳。
func (n *Node) becomeLeader() {
	n.State = core.StateLeader
	last := n.Log.LastIndex()
	for _, peer := range n.Peers {
		if peer == n.ID {
			continue
		}
		n.nextIndex[peer] = last + 1
		n.matchIndex[peer] = 0
	}
	// 当选后立即心跳，确立权威（避免旧 term 的 Leader 干扰）。
	n.broadcastAppendEntries()
}

// Propose 由客户端调用：Leader 追加一条命令并立即广播一次 AppendEntries 开始复制。
// 非 Leader 返回 false。
//
// 关于心跳与广播时机：Leader 在 Tick 里周期性广播空 AppendEntries 当心跳
// （间隔 = heartbeatInterval 个 tick），重置 Follower 选举时钟防其挑战 Leader；
// Propose 时则立即触发一次带新条目的广播，不必等下一个心跳周期，使新命令尽快复制。
func (n *Node) Propose(command any) bool {
	if n.State != core.StateLeader {
		return false
	}
	n.Log.Append(n.CurrentTerm, command)
	n.broadcastAppendEntries()
	// 单节点集群（或刚追上 quorum 的 Leader）自洽提交：Append 后立即尝试推进 commit。
	// 多节点场景下 advanceCommit 因 matchIndex 未达 quorum 不会提前提交，行为不变；
	// 单节点场景下 quorum=1，Leader 自身 count=1 即达 quorum，故能立即提交本任期条目。
	n.advanceCommit()
	return true
}

// CompactLog 执行日志压缩（snapshot 的简化版）：保留最后 keepLast 条日志，
// 丢弃更早的条目（它们已被"提交并应用到状态机"，可安全从内存日志移除）。
//
// 这是 Raft 快照/日志压缩的核心思想（论文 §7）：日志无限增长会耗尽存储并拖慢
// 重启回放；定期把已应用的状态打成 snapshot，然后裁剪日志前缀。本教学实现
// 只做"裁剪前缀"这一步（不真的序列化状态机状态），用 BaseIndex 偏移保留逻辑
// Index 空间的连续性，使 raft.go 的所有 Index 一致性检查自动保持正确。
//
// 关键不变量（本方法必须保证，且 TestLogCompaction 会校验）：
//   - Log.LastIndex() 不变（被压缩的条目仍计入 BaseIndex）。
//   - CommitIndex / LastApplied 不变（已提交的进度不能因压缩丢失）。
//   - Log.Length()（物理条目数）减少到 keepLast。
//   - 压缩后仍能给新加入的 Follower 同步：Leader 对该 Follower 的 nextIndex
//     若落在已压缩区间（<= BaseIndex），后续广播会以 BaseIndex 为 PrevLogIndex
//     重置（真实 Raft 这里改用 InstallSnapshot；本简化版用"从 BaseIndex+1 重发"）。
//
// 返回实际被丢弃的条目数。
func (n *Node) CompactLog(keepLast int) int {
	dropped := n.Log.Compact(keepLast)
	// Leader 还需修正对各 Follower 的 nextIndex/matchIndex：
	//   - nextIndex 不能落在已被丢弃的区间（<= BaseIndex），否则 broadcastAppendEntries
	//     计算 PrevLogIndex=nextIndex-1 时会取到已不存在的条目（At 返回 false），
	//     导致 PrevLogTerm=0 误判匹配。把过低的 nextIndex 抬到 BaseIndex+1。
	//   - matchIndex 若高于 BaseIndex 则保持（该 Follower 已有部分未压缩条目），
	//     不做下调（matchIndex 反映"已知对方拥有的最高 Index"，与是否压缩无关）。
	base := n.Log.BaseIndex
	if n.State == core.StateLeader {
		for _, peer := range n.Peers {
			if peer == n.ID {
				continue
			}
			if ni := n.nextIndex[peer]; ni <= base {
				n.nextIndex[peer] = base + 1
			}
		}
	}
	return dropped
}

// LogBaseIndex 返回日志的 BaseIndex（已被压缩/丢弃的条目数）。
// 暴露给测试与运维观察用：LogBaseIndex()>0 表示该节点已做过压缩。
func (n *Node) LogBaseIndex() uint64 {
	return n.Log.BaseIndex
}

// broadcastAppendEntries 向所有 Follower 发送 AppendEntries（含日志或心跳）。
func (n *Node) broadcastAppendEntries() {
	for _, peer := range n.Peers {
		if peer == n.ID {
			continue
		}
		ni := n.nextIndex[peer]
		prevIndex := ni - 1
		// 用 TermAt 而非 At().Term：当 prevIndex 恰好落在压缩边界（== BaseIndex）时，
		// At 会返回 false（条目已被压缩），但 TermAt 仍能给出压缩时记录的 BaseTerm，
		// 使 Leader 能向"已恢复 snapshot 等价状态"的 Follower 续传保留段日志。
		prevTerm := n.Log.TermAt(prevIndex)
		var entries []core.LogEntry
		if last := n.Log.LastIndex(); ni <= last {
			entries = n.Log.Slice(ni, last)
		}
		n.transport.Send(core.Message{
			From: n.ID, To: peer,
			Kind: KindAppendEntriesRequest,
			Term: n.CurrentTerm,
			Payload: AppendEntriesRequest{
				PrevLogIndex: prevIndex,
				PrevLogTerm:  prevTerm,
				Entries:      entries,
				LeaderCommit: n.CommitIndex,
			},
		})
	}
}

// handle 是传输层回调：分发 RequestVote / AppendEntries 两类消息。
func (n *Node) handle(msg core.Message) (core.Message, bool) {
	// 旧任期消息一律忽略（论文 §5.1：所有 server 都拒绝旧 term 的请求）。
	if msg.Term < n.CurrentTerm {
		// 但对旧任期的 RequestVote 要回一个拒绝，让候选人停止。
		if msg.Kind == KindRequestVoteRequest {
			return n.voteResponse(msg.From, false), true
		}
		return core.Message{}, false
	}
	// 收到更高任期的消息：自身降级为 Follower 并更新 term（论文 §5.1）。
	if msg.Term > n.CurrentTerm {
		n.CurrentTerm = msg.Term
		n.State = core.StateFollower
		n.VotedFor = nil
		n.electionTicks = 0
	}

	switch msg.Kind {
	case KindRequestVoteRequest:
		return n.handleRequestVote(msg)
	case KindAppendEntriesRequest:
		return n.handleAppendEntries(msg)
	case KindRequestVoteResponse:
		n.handleVoteResponse(msg)
		return core.Message{}, false
	case KindAppendEntriesResponse:
		n.handleAppendEntriesResponse(msg)
		return core.Message{}, false
	}
	return core.Message{}, false
}

// handleRequestVote 决定是否给候选人投票。
func (n *Node) handleRequestVote(msg core.Message) (core.Message, bool) {
	req, ok := msg.Payload.(RequestVoteRequest)
	if !ok {
		return core.Message{}, false
	}
	grant := false
	// 论文 §5.4.1：每个 server 每个任期最多投一票。
	canVote := n.VotedFor == nil || *n.VotedFor == msg.From
	if canVote {
		// 候选人日志必须至少与自己一样新（up-to-date）。
		if n.logUpToDate(req.LastLogTerm, req.LastLogIndex) {
			grant = true
			v := msg.From
			n.VotedFor = &v
			n.electionTicks = 0 // 投票也算一次"互动"，重置选举超时
		}
	}
	return n.voteResponse(msg.From, grant), true
}

func (n *Node) voteResponse(to core.NodeID, grant bool) core.Message {
	return core.Message{
		From: n.ID, To: to,
		Kind:    KindRequestVoteResponse,
		Term:    n.CurrentTerm,
		Payload: RequestVoteResponse{VoteGranted: grant},
	}
}

// logUpToDate 判断候选人日志是否至少与自己一样新。
// 规则（论文 §5.4.1）：比 lastTerm，term 相同则比 lastIndex。
func (n *Node) logUpToDate(candLastTerm, candLastIndex uint64) bool {
	myTerm := n.Log.LastTerm()
	if candLastTerm != myTerm {
		return candLastTerm > myTerm
	}
	return candLastIndex >= n.Log.LastIndex()
}

// handleVoteResponse 累计票数，达 quorum 当选。
func (n *Node) handleVoteResponse(msg core.Message) {
	if n.State != core.StateCandidate {
		return
	}
	resp, ok := msg.Payload.(RequestVoteResponse)
	if !ok {
		return
	}
	if resp.VoteGranted {
		n.votes++
		if n.votes >= n.quorum() {
			n.becomeLeader()
		}
	}
}

// handleAppendEntries 处理 Leader 的日志复制/心跳请求。
func (n *Node) handleAppendEntries(msg core.Message) (core.Message, bool) {
	req, ok := msg.Payload.(AppendEntriesRequest)
	if !ok {
		return core.Message{}, false
	}
	// 识别合法 Leader：收到当前任期的 AppendEntries 即认此 Leader，重置选举超时。
	n.State = core.StateFollower
	n.electionTicks = 0

	// 日志匹配检查：PrevLogIndex 处的 Term 必须等于 PrevLogTerm（论文 §5.3）。
	if req.PrevLogIndex > 0 {
		e, ok := n.Log.At(req.PrevLogIndex)
		if !ok || e.Term != req.PrevLogTerm {
			return n.appendResponse(msg.From, false), true
		}
	}
	// 复制新条目，处理冲突（同 Index 不同 Term 则截断后重写）。
	for i, entry := range req.Entries {
		existing, ok := n.Log.At(entry.Index)
		if ok {
			if existing.Term != entry.Term {
				// 冲突：截断此处及之后，再追加。
				n.Log.Truncate(entry.Index)
				for _, e := range req.Entries[i:] {
					n.Log.Append(e.Term, e.Command)
				}
				break
			}
			// 已存在且一致，跳过。
			continue
		}
		n.Log.Append(entry.Term, entry.Command)
	}
	// 推进 commitIndex（论文 §5.3.2）。
	if req.LeaderCommit > n.CommitIndex {
		lastNew := req.PrevLogIndex + uint64(len(req.Entries))
		if req.LeaderCommit < lastNew {
			n.CommitIndex = req.LeaderCommit
		} else {
			n.CommitIndex = lastNew
		}
		n.LastApplied = n.CommitIndex
	}
	return n.appendResponse(msg.From, true), true
}

func (n *Node) appendResponse(to core.NodeID, success bool) core.Message {
	return core.Message{
		From: n.ID, To: to,
		Kind:    KindAppendEntriesResponse,
		Term:    n.CurrentTerm,
		Payload: AppendEntriesResponse{Success: success, MatchIndex: n.Log.LastIndex()},
	}
}

// handleAppendEntriesResponse Leader 根据 Follower 回复推进 nextIndex/matchIndex，
// 并在 quorum 复制后提交。
func (n *Node) handleAppendEntriesResponse(msg core.Message) {
	if n.State != core.StateLeader {
		return
	}
	resp, ok := msg.Payload.(AppendEntriesResponse)
	if !ok {
		return
	}
	if resp.Success {
		// 更新该 Follower 的 matchIndex/nextIndex。
		if resp.MatchIndex > n.matchIndex[msg.From] {
			n.matchIndex[msg.From] = resp.MatchIndex
			n.nextIndex[msg.From] = resp.MatchIndex + 1
		}
		n.advanceCommit()
		return
	}
	// 失败：日志不一致，回退 nextIndex 重试（论文 §5.3）。
	if ni := n.nextIndex[msg.From]; ni > 1 {
		n.nextIndex[msg.From] = ni - 1
	}
	// 立即用回退后的 nextIndex 再发一次。
	n.sendAppendTo(msg.From)
}

// sendAppendTo 向单个 Follower 发送 AppendEntries。
func (n *Node) sendAppendTo(peer core.NodeID) {
	ni := n.nextIndex[peer]
	prevIndex := ni - 1
	// 同 broadcastAppendEntries：用 TermAt 兼容压缩边界。
	prevTerm := n.Log.TermAt(prevIndex)
	var entries []core.LogEntry
	if last := n.Log.LastIndex(); ni <= last {
		entries = n.Log.Slice(ni, last)
	}
	n.transport.Send(core.Message{
		From: n.ID, To: peer,
		Kind: KindAppendEntriesRequest,
		Term: n.CurrentTerm,
		Payload: AppendEntriesRequest{
			PrevLogIndex: prevIndex, PrevLogTerm: prevTerm,
			Entries:      entries,
			LeaderCommit: n.CommitIndex,
		},
	})
}

// advanceCommit 统计：若某个 N 被多数 matchIndex 覆盖且 > CommitIndex，
// 且 N 处的 term 是当前 term，则提交到 N（论文 §5.4.2 leader-commit 限制）。
func (n *Node) advanceCommit() {
	for idx := n.Log.LastIndex(); idx > n.CommitIndex; idx-- {
		e, ok := n.Log.At(idx)
		if !ok || e.Term != n.CurrentTerm {
			continue // 只提交本任期的条目（安全约束）
		}
		count := 1 // Leader 自己
		for _, peer := range n.Peers {
			if peer == n.ID {
				continue
			}
			if n.matchIndex[peer] >= idx {
				count++
			}
		}
		if count >= n.quorum() {
			n.CommitIndex = idx
			n.LastApplied = idx
			return
		}
	}
}

// TermCompare 比较两个任期：返回 -1/0/1（a<b / a==b / a>b）。
func TermCompare(a, b uint64) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}

// QuorumSize 返回集群的多数派大小（n/2+1）。
func QuorumSize(clusterSize int) int {
	return clusterSize/2 + 1
}

// MaxTerm 返回两个任期中较大的。
func MaxTerm(a, b uint64) uint64 {
	if a > b {
		return a
	}
	return b
}

// IsLeader 报告节点是否为 Leader。
func (n *Node) IsLeader() bool {
	return n.State == core.StateLeader
}
