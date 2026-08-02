package core

import "sync"

// Message 是节点间在网络上传送的消息。Kind 由各协议自定义（如 "RequestVote" / "AppendEntries"）。
type Message struct {
	From    NodeID
	To      NodeID
	Kind    string
	Term    uint64 // 任期/轮次（Raft/Paxos 通用）
	Payload any    // 协议特定负载（投票请求、日志条目等）
}

// Handler 处理收到的消息，返回是否需要回复以及回复消息。
// handled=false 表示接收方忽略此消息（如旧任期消息）。
type Handler func(msg Message) (reply Message, handled bool)

// Transport 是节点间的传输抽象。内存实现用于 demo（确定性、离线），
// 真实网络实现可作为后续扩展（参考 go-rmm 的 relay/proto）。
type Transport interface {
	// Send 投递一条消息到目标节点的收件箱（非阻塞，返回前不入队则视为丢弃）。
	Send(msg Message)
	// Install 注册一个节点的消息处理器。
	Install(id NodeID, h Handler)
	// Drain 取出并清空所有节点收件箱里待处理的消息，按投递顺序返回。
	// 用于 demo 单线程驱动网络"一个 tick"，保证执行轨迹确定。
	Drain() []Message
}

// MemTransport 是进程内的内存网络实现。所有 Send 的消息先进待投递队列，
// 由调用方显式 Drain 推进，避免 goroutine 调度带来的非确定性。
type MemTransport struct {
	mu       sync.Mutex
	handlers map[NodeID]Handler
	pending  []Message       // 待投递队列（FIFO）
	blocked  map[NodeID]bool // 被网络分区隔离的节点（收/发都被丢弃）
}

// NewMemTransport 创建一个空的内存传输层。
func NewMemTransport() *MemTransport {
	return &MemTransport{handlers: make(map[NodeID]Handler), blocked: make(map[NodeID]bool)}
}

// Install 注册/覆盖一个节点的处理器。
func (t *MemTransport) Install(id NodeID, h Handler) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.handlers[id] = h
}

// Send 把消息追加到待投递队列。
func (t *MemTransport) Send(msg Message) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.pending = append(t.pending, msg)
}

// BlockNode 把 id 标记为"网络分区隔离"：此后 Drain 时，From 或 To 是 id 的
// 消息一律被丢弃（模拟该节点断连，既收不到任何消息也发不出任何消息）。
// 用于网络分区恢复测试：在 Leader 提交后隔离部分 Follower，再 UnblockNode 恢复。
func (t *MemTransport) BlockNode(id NodeID) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.blocked[id] = true
}

// UnblockNode 解除 id 的分区隔离，恢复正常收发。
func (t *MemTransport) UnblockNode(id NodeID) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.blocked, id)
}

// IsBlocked 报告 id 当前是否被分区隔离。
func (t *MemTransport) IsBlocked(id NodeID) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.blocked[id]
}

// Drain 推进一个网络 tick：取出所有待投递消息，逐条调用目标节点的 Handler。
// 若 Handler 返回 reply（handled=true 且 reply.From 非空），把回复重新入队，
// 下一轮 Drain 再投递。返回本轮实际被处理（非丢弃）的消息数。
//
// 这种"显式推进"模型让 demo 的执行轨迹完全确定：调用方控制 tick 次数。
//
// 分区语义：若 From 或 To 是被 BlockNode 隔离的节点，该消息视为在网络中丢失，
// 直接跳过（不调用 Handler、不计入返回切片），模拟断连。
func (t *MemTransport) Drain() []Message {
	t.mu.Lock()
	if len(t.pending) == 0 {
		t.mu.Unlock()
		return nil
	}
	batch := t.pending
	t.pending = nil
	handlers := make(map[NodeID]Handler, len(t.handlers))
	for k, v := range t.handlers {
		handlers[k] = v
	}
	blocked := make(map[NodeID]bool, len(t.blocked))
	for k, v := range t.blocked {
		blocked[k] = v
	}
	t.mu.Unlock()

	var handled []Message
	for _, msg := range batch {
		// 网络分区：From/To 任一被隔离则消息丢失。
		if blocked[msg.From] || blocked[msg.To] {
			continue
		}
		h, ok := handlers[msg.To]
		if !ok {
			continue
		}
		reply, ok := h(msg)
		if !ok {
			continue
		}
		handled = append(handled, msg)
		if reply.From != "" || reply.To != "" {
			// 回复若来自/去往被隔离节点，同样丢失。
			if blocked[reply.From] || blocked[reply.To] {
				continue
			}
			t.Send(reply)
		}
	}
	return handled
}
