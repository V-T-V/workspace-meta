package core

// LogEntry 是复制状态机日志的一条条目。
// Raft / Multi-Paxos / PBFT 都用此结构承载"已达成共识的命令序列"。
type LogEntry struct {
	Term    uint64 // 写入该条目时 Leader/Primary 的任期
	Index   uint64 // 日志位置（从 1 开始；0 表示哨兵）
	Command any    // 状态机命令（协议无关）
}

// Log 是一个节点的复制日志。各协议按需使用 Index/Term 字段。
// 设计为值类型切片，调用方持有副本；并发安全由协议层（单 goroutine 驱动）保证。
//
// 索引模型：LogEntry.Index 是"逻辑索引"，从 1 开始连续递增。
// 物理切片 Entries 的第 0 个元素对应逻辑索引 BaseIndex+1。
//   - 未压缩时 BaseIndex=0，Entries[0] 的逻辑 Index 就是 1（与历史行为一致）。
//   - Compact(keepLast) 后，丢弃最旧若干条，BaseIndex 前移；逻辑索引空间不变，
//     仍连续，所以 raft.go 里所有按 Index 做的匹配/比较都自动保持正确。
type Log struct {
	Entries   []LogEntry
	BaseIndex uint64 // 已被压缩（snapshot 丢弃）的条目数；Entries[0] 的逻辑 Index = BaseIndex+1
	BaseTerm  uint64 // 逻辑 Index == BaseIndex 那一条（最后一条被压缩进 snapshot 的）的 Term；无压缩时为 0
}

// Append 追加一条日志条目，返回它的 Index（连续递增，受 BaseIndex 影响）。
func (l *Log) Append(term uint64, command any) uint64 {
	idx := l.BaseIndex + uint64(len(l.Entries)) + 1
	l.Entries = append(l.Entries, LogEntry{Term: term, Index: idx, Command: command})
	return idx
}

// LastIndex 返回最后一条日志的 Index；日志为空时返回 BaseIndex（哨兵：上一条已压缩条目的 Index）。
// 注意：返回 BaseIndex 而非 0，是为了让压缩后的"空日志"仍能正确表达
// "最后一条已知（已压缩进 snapshot）的 Index"——这正是 Raft InstallSnapshot
// 后 Leader 与 Follower 协商 nextIndex 时需要的信息。
func (l *Log) LastIndex() uint64 {
	if n := len(l.Entries); n > 0 {
		return l.BaseIndex + uint64(n)
	}
	return l.BaseIndex
}

// LastTerm 返回最后一条日志的 Term；日志为空时返回 0。
func (l *Log) LastTerm() uint64 {
	if n := len(l.Entries); n > 0 {
		return l.Entries[n-1].Term
	}
	return 0
}

// At 返回指定逻辑 Index（从 1 开始）的日志条目；越界返回零值和 false。
func (l *Log) At(index uint64) (LogEntry, bool) {
	if index <= l.BaseIndex || index > l.LastIndex() {
		return LogEntry{}, false
	}
	return l.Entries[index-l.BaseIndex-1], true
}

// Truncate 从 index（含）开始截断后续所有条目，用于 Leader 强制覆盖 Follower 的冲突日志。
// index == 0 是哨兵（无操作，与历史行为兼容）。
// index <= BaseIndex（已压缩到该处及之前）视为清空全部未压缩条目。
func (l *Log) Truncate(index uint64) {
	if index == 0 {
		return
	}
	if index <= l.BaseIndex {
		l.Entries = l.Entries[:0]
		return
	}
	pos := index - l.BaseIndex - 1 // 转成切片下标（含）
	if pos >= uint64(len(l.Entries)) {
		return
	}
	l.Entries = l.Entries[:pos]
}

// Slice 返回 [start, end]（含，逻辑 Index 从 1 开始）范围内的条目副本。
func (l *Log) Slice(start, end uint64) []LogEntry {
	last := l.LastIndex()
	if start <= l.BaseIndex || start > last {
		return nil
	}
	if end > last {
		end = last
	}
	if start > end {
		return nil
	}
	out := make([]LogEntry, 0, end-start+1)
	for i := start; i <= end; i++ {
		out = append(out, l.Entries[i-l.BaseIndex-1])
	}
	return out
}

// Compact 执行日志压缩（snapshot 的简化版）：保留最后 keepLast 条未压缩条目，
// 丢弃更早的条目，并把丢弃的数量累加进 BaseIndex。
//
// 逻辑 Index 空间不变（保留条目的 Index 不被改写），所以 raft.go 里所有按
// Index 做的一致性检查（PrevLogIndex 匹配、advanceCommit、nextIndex 回退等）
// 都自动保持正确——这正是把 BaseIndex 内建进 Log 的好处。
//
// 同时记录 BaseTerm：被丢弃的最后一条（逻辑 Index == 新 BaseIndex）的 Term，
// 供 PrevLogIndex 恰好落在压缩边界时（Leader 给 Follower 补发保留段的第一条）
// 计算 PrevLogTerm 用——这是简化版（未实现完整 InstallSnapshot）让压缩后的
// Leader 仍能向"已拥有 snapshot 等价状态"的 Follower 续传日志的关键。
//
// 边界：
//   - keepLast <= 0：丢弃全部未压缩条目（BaseIndex += len(Entries)，Entries 清空）。
//   - keepLast >= len(Entries)：无操作（没有可丢弃的旧条目）。
//   - keepLast 之间：保留尾部 keepLast 条，前面丢弃。
//
// 返回值是被丢弃的条目数（便于测试/日志统计）。
func (l *Log) Compact(keepLast int) int {
	n := len(l.Entries)
	if keepLast < 0 {
		keepLast = 0
	}
	if keepLast >= n {
		return 0
	}
	drop := n - keepLast
	// 记下被丢弃的最后一条的 Term（它将成为新的压缩边界 BaseIndex 处的 Term）。
	l.BaseTerm = l.Entries[drop-1].Term
	l.BaseIndex += uint64(drop)
	// 用 append 到新切片的方式释放底层数组对被丢弃元素的引用，避免内存泄漏。
	kept := append([]LogEntry(nil), l.Entries[drop:]...)
	l.Entries = kept
	return drop
}

// TermAt 返回指定逻辑 Index 处的 Term。
//   - index == BaseIndex（>0）：返回压缩时记录的 BaseTerm（最后一条被压缩条目的 Term）。
//   - BaseIndex < index <= LastIndex：返回对应未压缩条目的 Term。
//   - 其它（0、已压缩区间内部、越界）：返回 0（无法提供，调用方应据此回退/发 snapshot）。
//
// 专为 raft.go 的 broadcastAppendEntries 计算 PrevLogTerm 设计：当 PrevLogIndex
// 落在压缩边界时仍能给出正确 Term，使 Leader 能向已恢复 snapshot 的 Follower 续传。
func (l *Log) TermAt(index uint64) uint64 {
	if index == l.BaseIndex && index > 0 {
		return l.BaseTerm
	}
	if e, ok := l.At(index); ok {
		return e.Term
	}
	return 0
}

// Length 返回当前未压缩（仍保留在 Entries 里）的条目数。
// 压缩后此值会减小，但逻辑 LastIndex 不变（被压缩的条目仍计入 BaseIndex）。
func (l *Log) Length() int {
	return len(l.Entries)
}
