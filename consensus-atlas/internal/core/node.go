// Package core 提供 consensus-atlas 各算法包共享的底座类型：
// 节点 ID、内存网络传输、复制日志、逻辑时钟。
//
// 设计原则（对齐 go-agent-research/internal/core）：
//   - 纯标准库，零外部依赖
//   - 各算法包只依赖本包，彼此互不 import
//   - 传输层用内存实现，保证 demo 离线可跑且轨迹确定
package core

// NodeID 是集群中节点的唯一标识。
type NodeID string

// NodeState 标记一个节点在共识协议中的角色/状态。
type NodeState int

const (
	StateFollower NodeState = iota
	StateCandidate
	StateLeader
	StatePrimary // Paxos / PBFT 用的"主"角色
	StateReplica // PBFT 的副本角色
)

// String 返回节点状态的可读名称。
func (s NodeState) String() string {
	switch s {
	case StateFollower:
		return "follower"
	case StateCandidate:
		return "candidate"
	case StateLeader:
		return "leader"
	case StatePrimary:
		return "primary"
	case StateReplica:
		return "replica"
	default:
		return "unknown"
	}
}
