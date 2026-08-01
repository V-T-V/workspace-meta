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
type Log struct {
	Entries []LogEntry
}

// Append 追加一条日志条目，返回它的 Index（从 1 开始）。
func (l *Log) Append(term uint64, command any) uint64 {
	idx := uint64(len(l.Entries)) + 1
	l.Entries = append(l.Entries, LogEntry{Term: term, Index: idx, Command: command})
	return idx
}

// LastIndex 返回最后一条日志的 Index；日志为空时返回 0。
func (l *Log) LastIndex() uint64 {
	if n := len(l.Entries); n > 0 {
		return uint64(n)
	}
	return 0
}

// LastTerm 返回最后一条日志的 Term；日志为空时返回 0。
func (l *Log) LastTerm() uint64 {
	if n := len(l.Entries); n > 0 {
		return l.Entries[n-1].Term
	}
	return 0
}

// At 返回指定 Index（从 1 开始）的日志条目；越界返回零值和 false。
func (l *Log) At(index uint64) (LogEntry, bool) {
	if index == 0 || index > uint64(len(l.Entries)) {
		return LogEntry{}, false
	}
	return l.Entries[index-1], true
}

// Truncate 从 index（含）开始截断后续所有条目，用于 Leader 强制覆盖 Follower 的冲突日志。
func (l *Log) Truncate(index uint64) {
	if index == 0 || index > uint64(len(l.Entries)) {
		return
	}
	l.Entries = l.Entries[:index-1]
}

// Slice 返回 [start, end]（含，Index 从 1 开始）范围内的条目副本。
func (l *Log) Slice(start, end uint64) []LogEntry {
	if start == 0 || start > uint64(len(l.Entries)) {
		return nil
	}
	if end > uint64(len(l.Entries)) {
		end = uint64(len(l.Entries))
	}
	if start > end {
		return nil
	}
	out := make([]LogEntry, 0, end-start+1)
	for i := start; i <= end; i++ {
		out = append(out, l.Entries[i-1])
	}
	return out
}
