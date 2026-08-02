package core

// Clock 是逻辑时钟接口。Lamport Clock 与 Vector Clock 都实现此接口，
// 让上层（如因果排序、消息打标）可以统一调用。
type Clock interface {
	// Tick 本地事件：推进时钟，返回新时间戳。
	Tick() uint64
	// Observe 收到对方时间戳时更新本地时钟（Lamport 取 max+1）。
	Observe(other uint64) uint64
	// Now 返回当前时间戳（不推进）。
	Now() uint64
}

// LamportClock 是 Lamport 逻辑时钟的最小实现。
// 规则：本地事件 C = C + 1；收到消息 C = max(C, msg.C) + 1。
type LamportClock struct {
	c uint64
}

// Tick 推进一个本地事件并返回新时间戳。
func (lc *LamportClock) Tick() uint64 {
	lc.c++
	return lc.c
}

// Observe 用对方时间戳更新本地时钟，返回新时间戳。
func (lc *LamportClock) Observe(other uint64) uint64 {
	if other > lc.c {
		lc.c = other
	}
	lc.c++
	return lc.c
}

// Now 返回当前时间戳。
func (lc *LamportClock) Now() uint64 {
	return lc.c
}
