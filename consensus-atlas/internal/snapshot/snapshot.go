package snapshot

// Chandy-Lamport 分布式快照算法（1985）——在不停止系统运行的前提下，记录一个
// **一致的全局状态**（consistent global state）。
//
// 模型：N 个进程通过**单向 FIFO 通道**互连发消息。通道可靠、FIFO（先进先出）。
// 快照发起者给所有出通道发 marker；进程收到 marker 时按规则记录本地/通道状态。
//
// 本包用纯 Go（无 goroutine / time / rand）实现：通道是显式的消息队列（FIFO），
// 由调用方显式 Step 推进，保证 demo 轨迹确定（与本仓库 raft/paxos 的"显式 Drain"思路一致）。

// ProcessID 标识一个进程。本算法不依赖 core 包（无 NodeID / Transport 需求，
// 纯函数式 + 显式通道），保持自包含便于教学——与 crdt 包"另一种范式"取向一致。
type ProcessID string

// MsgVal 是通道里传输的应用消息值（简化为字符串）。marker 不走此类型，单独标识。
type MsgVal string

// 通道里的一项：要么是应用消息，要么是 marker。
type channelItem struct {
	isMarker bool
	val      MsgVal // 仅当 isMarker=false 时有意义
}

// Channel 是一条单向 FIFO 通道：从 From 流向 To。
// pending 是已发送但未被 To 取走的消息队列（FIFO，先进先出）。
type Channel struct {
	From     ProcessID
	To       ProcessID
	pending  []channelItem // 待投递（To 未取走）的消息，队首在 [0]
	recorded []MsgVal      // 快照期间记录的通道状态（marker 之后到达的应用消息）
}

// NewChannel 构造一条 From→To 的空 FIFO 通道。
func NewChannel(from, to ProcessID) *Channel {
	return &Channel{From: from, To: to}
}

// Send 把一条应用消息追加到通道尾部（FIFO）。
func (c *Channel) Send(v MsgVal) {
	c.pending = append(c.pending, channelItem{isMarker: false, val: v})
}

// sendMarker 把一条 marker 追加到通道尾部。marker 之后的应用消息会被通道接收方
// 在快照期间记录为该通道的状态。
func (c *Channel) sendMarker() {
	c.pending = append(c.pending, channelItem{isMarker: true})
}

// pop 取出队首；返回 (isMarker, val, ok)，ok=false 表示通道空。
func (c *Channel) pop() (isMarker bool, val MsgVal, ok bool) {
	if len(c.pending) == 0 {
		return false, "", false
	}
	item := c.pending[0]
	c.pending = c.pending[1:]
	return item.isMarker, item.val, true
}

// Process 是一个分布式进程。它持有本地状态（应用变量）、出入通道列表、
// 以及快照记录状态。
//
// 快照记录状态机（论文规则）：
//
//	未记录本地状态（!recorded）：
//	  收到 marker（首次）：
//	    1. 记录本地状态。
//	    2. 标记"已在此通道收到 marker"，开始把后续到达该通道的应用消息记入通道状态。
//	    3. 给所有**出通道**发 marker。
//	  收到 marker（已记录，但本通道未标记）：把本通道标记为"已收到 marker"（通道状态为空，
//	    因为 marker 之前没有应用消息在路上——这是 FIFO 的保证）。
//	  收到应用消息：
//	    若本进程已记录本地状态且此入通道已收到过 marker → 消息属于"快照后"，记入该通道状态。
//	    否则 → 正常处理（更新本地状态）。
type Process struct {
	ID      ProcessID
	State   MsgVal // 本地应用状态（简化为单个可变值）
	Inputs  []*Channel
	Outputs []*Channel

	// 快照状态
	recorded      bool                   // 是否已记录本地状态
	recordedState MsgVal                 // 记录的本地状态快照
	markerSeen    map[ProcessID]bool     // 哪些入通道已收到 marker（按 From 索引）
	channelState  map[ProcessID][]MsgVal // 记录的各入通道状态（marker 后到达的应用消息）
}

// NewProcess 构造一个进程，本地状态初值为 initState。
func NewProcess(id ProcessID, initState MsgVal) *Process {
	return &Process{
		ID:           id,
		State:        initState,
		markerSeen:   make(map[ProcessID]bool),
		channelState: make(map[ProcessID][]MsgVal),
	}
}

// AddInput 注册一条入通道（来自 from）。
func (p *Process) AddInput(c *Channel) {
	p.Inputs = append(p.Inputs, c)
}

// AddOutput 注册一条出通道（发往 to）。
func (p *Process) AddOutput(c *Channel) {
	p.Outputs = append(p.Outputs, c)
}

// RecordLocal 记录本地状态，并给所有出通道发 marker。
// 这是进程**首次**收到 marker（或自己作为发起者）时的动作。
func (p *Process) recordLocalAndMark() {
	if p.recorded {
		return // 已记录，幂等
	}
	p.recorded = true
	p.recordedState = p.State
	// 给所有出通道发 marker（FIFO：marker 排在已发送消息之后）。
	for _, c := range p.Outputs {
		c.sendMarker()
	}
}

// markChannelSeen 标记某入通道已收到 marker。
// 若此前该通道已记录过消息，保留；否则通道状态保持为空（marker 前无在途消息）。
func (p *Process) markChannelSeen(from ProcessID) {
	p.markerSeen[from] = true
}

// ReceiveMessage 处理从入通道收到的一条消息（应用消息或 marker）。
//   - 收到 marker：触发本地记录（首次）或标记通道；并把 marker 之前在该通道排队的
//     应用消息已经全部被取出（FIFO），故此时该通道状态 = 后续到达的应用消息。
//   - 收到应用消息：若本进程已记录且此入通道已 seen marker → 记入通道状态；
//     否则正常更新本地状态。
//
// 注：本算法的 FIFO 保证意味着"收到 marker 时，该 marker 之前发出的应用消息
// 都已先于 marker 到达"——所以记录通道状态只需记 marker 之后到达的。
func (p *Process) ReceiveMessage(in *Channel, isMarker bool, val MsgVal) {
	if isMarker {
		// 首次收到 marker（任何通道）→ 记录本地状态 + 给所有出通道发 marker。
		if !p.recorded {
			p.recordLocalAndMark()
		}
		// 标记此入通道已收到 marker（此后到达此通道的应用消息记入通道状态）。
		p.markChannelSeen(in.From)
		return
	}
	// 应用消息：若本进程已开始记录且此入通道已 seen marker → 属于"快照后"消息，记入通道状态。
	if p.recorded && p.markerSeen[in.From] {
		p.channelState[in.From] = append(p.channelState[in.From], val)
		return
	}
	// 正常处理：更新本地状态（本教学包用"最后一条消息覆盖"语义模拟应用演进）。
	p.State = val
}

// Snapshot 是一次全局快照的结果：各进程的本地状态 + 各通道的在途消息状态。
type Snapshot struct {
	// ProcessStates[id] = 进程 id 记录的本地状态。
	ProcessStates map[ProcessID]MsgVal
	// ChannelStates[key] = 通道 (from→to) 记录的在途消息（marker 之后到达的）。
	// key 用 "from->to" 字符串。
	ChannelStates map[string][]MsgVal
	// Initiator 记录发起快照的进程。
	Initiator ProcessID
}

// StartedBy 标记一个进程是否已开始记录（用于判断快照是否完成）。
func (p *Process) StartedBy() bool { return p.recorded }

// LocalSnapshot 返回本进程已记录的本地状态（未记录时为零值）。
func (p *Process) LocalSnapshot() (MsgVal, bool) {
	return p.recordedState, p.recorded
}

// System 是一组互连进程的集合，用于驱动快照算法。
type System struct {
	Processes []*Process
	// 所有通道（按引用持有，便于遍历投递）。
	Channels []*Channel
}

// NewSystem 构造一个空系统。
func NewSystem() *System {
	return &System{}
}

// Add 把一个进程加入系统。
func (s *System) Add(p *Process) {
	s.Processes = append(s.Processes, p)
}

// AddChannel 把一条通道加入系统（同时会注册到两端进程的 Inputs/Outputs）。
func (s *System) AddChannel(c *Channel, fromProc, toProc *Process) {
	s.Channels = append(s.Channels, c)
	fromProc.AddOutput(c)
	toProc.AddInput(c)
}

// StartSnapshot 由 initiator 发起快照：记录其本地状态，给所有出通道发 marker。
// 返回前不推进消息投递；调用方需反复 Step 直到完成（所有进程都记录了本地状态）。
func StartSnapshot(initiator *Process) {
	initiator.recordLocalAndMark()
}

// Step 推进一个投递回合：遍历所有通道，把每条通道队首的一条消息投递给接收方。
// 返回本轮实际投递的消息数（含 marker 与应用消息）。
//
// 调用方反复 Step 直到返回 0（所有通道空）即表示快照的消息传播完成。
// 这种类比 raft MemTransport.Drain 的"显式推进"模型保证 demo 轨迹确定。
func (s *System) Step() int {
	delivered := 0
	// 建立进程索引以便按通道 To 找接收方。
	procByID := make(map[ProcessID]*Process, len(s.Processes))
	for _, p := range s.Processes {
		procByID[p.ID] = p
	}
	// 每条通道投递队首一条（FIFO，保证 marker 严格先于其后的应用消息到达）。
	for _, c := range s.Channels {
		isMarker, val, ok := c.pop()
		if !ok {
			continue
		}
		recv := procByID[c.To]
		if recv == nil {
			continue
		}
		recv.ReceiveMessage(c, isMarker, val)
		delivered++
	}
	return delivered
}

// Complete 判断快照是否完成：所有进程都已记录本地状态。
// （通道里的应用消息可能还有，但只要所有进程都已开始记录，marker 已传播完毕，
// 剩余在途消息会被各自通道的 channelState 正确吸收。）
func (s *System) Complete() bool {
	for _, p := range s.Processes {
		if !p.recorded {
			return false
		}
	}
	return true
}

// Collect 在快照完成后，汇总各进程的本地状态与通道状态为一个 Snapshot。
func (s *System) Collect(initiator ProcessID) *Snapshot {
	snap := &Snapshot{
		ProcessStates: make(map[ProcessID]MsgVal, len(s.Processes)),
		ChannelStates: make(map[string][]MsgVal),
		Initiator:     initiator,
	}
	for _, p := range s.Processes {
		snap.ProcessStates[p.ID] = p.recordedState
		for from, msgs := range p.channelState {
			key := string(from) + "->" + string(p.ID)
			snap.ChannelStates[key] = append([]MsgVal(nil), msgs...)
		}
	}
	return snap
}
