package clock

import "github.com/QiuShichang/consensus-atlas/internal/core"

// Relation 描述两个向量时钟之间的因果关系。
type Relation int

const (
	// HappensBefore 表示 a 因果先于 b（a → b）：a 的所有分量 ≤ b，且至少一个严格 <。
	HappensBefore Relation = iota
	// HappensAfter 表示 a 因果后于 b（b → a）：a 的所有分量 ≥ b，且至少一个严格 >。
	HappensAfter
	// Concurrent 表示 a 与 b 互不因果可达（无 max 关系）：既非 a≤b 也非 b≤a。
	Concurrent
	// Equal 表示两个向量完全相等。
	Equal
)

// String 返回关系的可读名称。
func (r Relation) String() string {
	switch r {
	case HappensBefore:
		return "HappensBefore"
	case HappensAfter:
		return "HappensAfter"
	case Concurrent:
		return "Concurrent"
	case Equal:
		return "Equal"
	default:
		return "Unknown"
	}
}

// VectorClock 是向量逻辑时钟的实现。
//
// 维度 N = len(Nodes)，每个分量 Values[node] 记录该节点已观察到的本地事件计数。
// ownerID 标记"自己的位置"——本地事件只递增自己的分量；收消息时每个分量取 max，
// 自己的分量再额外 +1（对应"接收"本身也是一个事件）。
//
// 与 core.LamportClock（标量）对偶：Lamport 是 N=1 的特例退化，丢掉了"哪个节点"
// 的信息，因此无法判断并发；Vector Clock 保留全部维度，能精确刻画因果关系。
type VectorClock struct {
	Nodes   []core.NodeID          // 所有节点（定下向量维度，顺序无关，仅用于遍历）
	Values  map[core.NodeID]uint64 // 当前向量，按节点 ID 索引
	ownerID core.NodeID            // 自己的节点 ID（Tick 时递增这个分量）
}

// NewVectorClock 构造一个全 0 的向量时钟。owner 必须出现在 allNodes 中。
func NewVectorClock(owner core.NodeID, allNodes []core.NodeID) *VectorClock {
	v := &VectorClock{
		Nodes:   make([]core.NodeID, len(allNodes)),
		Values:  make(map[core.NodeID]uint64, len(allNodes)),
		ownerID: owner,
	}
	for i, n := range allNodes {
		v.Nodes[i] = n
		v.Values[n] = 0
	}
	return v
}

// copyVector 返回一个独立拷贝（避免外部修改污染内部状态）。
func copyVector(src map[core.NodeID]uint64) map[core.NodeID]uint64 {
	dst := make(map[core.NodeID]uint64, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

// Tick 推进一个本地事件：owner 的分量 +1，返回当前向量快照。
func (vc *VectorClock) Tick() map[core.NodeID]uint64 {
	vc.Values[vc.ownerID]++
	return copyVector(vc.Values)
}

// Observe 处理收到一条带向量时间戳的消息：
// 每个分量 V[j] = max(V[j], other[j])，然后自己的分量再 +1。
// 返回更新后的向量快照。other 中若出现本时钟未知的节点，按 0 处理（教学实现，
// 不动态扩维——成员变更不在本包范围内）。
func (vc *VectorClock) Observe(other map[core.NodeID]uint64) map[core.NodeID]uint64 {
	for _, n := range vc.Nodes {
		if o, ok := other[n]; ok && o > vc.Values[n] {
			vc.Values[n] = o
		}
	}
	vc.Values[vc.ownerID]++
	return copyVector(vc.Values)
}

// Now 返回当前向量快照（不推进）。返回的是独立拷贝，调用方可自由修改。
func (vc *VectorClock) Now() map[core.NodeID]uint64 {
	return copyVector(vc.Values)
}

// Compare 比较两个向量时间戳的因果关系。
//
// 规则（Mattern 1989）：
//   - a ≤ b 当且仅当对所有分量 i：a[i] ≤ b[i]。
//   - a < b 当且仅当 a ≤ b 且至少一个分量严格小于。
//   - a = b 当且仅当所有分量相等。
//   - 若既非 a ≤ b 也非 b ≤ a，则二者 Concurrent（并发，无因果关系）。
//
// 函数只看传入的快照，与任何 VectorClock 实例的状态无关，便于离线比较历史快照。
func Compare(a, b map[core.NodeID]uint64) Relation {
	// 合并所有出现过的节点 ID，保证维度对齐（缺失分量按 0 计）。
	keys := make(map[core.NodeID]struct{}, len(a)+len(b))
	for k := range a {
		keys[k] = struct{}{}
	}
	for k := range b {
		keys[k] = struct{}{}
	}

	aLEb := true // a 的所有分量 ≤ b
	bLEa := true // b 的所有分量 ≤ a
	aStrict := false
	bStrict := false
	for k := range keys {
		av := a[k] // 缺失视为 0
		bv := b[k]
		if av > bv {
			aLEb = false
			bStrict = true
		}
		if bv > av {
			bLEa = false
			aStrict = true
		}
	}

	switch {
	case aLEb && bLEa: // 所有分量相等
		return Equal
	case aLEb && aStrict:
		return HappensBefore
	case bLEa && bStrict:
		return HappensAfter
	default:
		return Concurrent
	}
}
