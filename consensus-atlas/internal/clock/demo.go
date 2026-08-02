package clock

import (
	"context"

	"github.com/QiuShichang/consensus-atlas/internal/core"
)

// DemoResult 是 clock demo 的输出摘要，用来直观对比 Lamport 与 Vector 两种时钟。
type DemoResult struct {
	// LamportFinal 是序列结束后 n2 节点的 Lamport 标量值（一个数字，看不出因果关系）。
	LamportFinal uint64
	// VectorFinalN2 是序列结束后 n2 节点的向量快照。
	VectorFinalN2 map[core.NodeID]uint64
	// VectorFinalN1 是 n1 发送那条消息（即被 n2 Observe）时的向量快照。
	VectorFinalN1 map[core.NodeID]uint64
	// VectorFinalN3 是 n3 独立并发事件后的向量快照。
	VectorFinalN3 map[core.NodeID]uint64
	// N2vsN3 比较 n2 与 n3 的最终向量，预期 Concurrent（二者无任何消息交互）。
	N2vsN3 Relation
	// SerializedN1vsN2 比较 n1 发送时刻 vs n2 接收后的向量，预期 HappensBefore。
	SerializedN1vsN2 Relation
}

// Demo 演示因果关系判定：在同一事件序列上同时跑 Lamport 与 Vector 时钟，
// 展示 Vector 能判并发而 Lamport 只能给一个递增数字。
//
// 事件序列（3 节点 n1/n2/n3，确定性、纯函数式、无 goroutine/time/rand）：
//  1. n1 本地事件
//  2. n1 发消息给 n2（携带当前向量/标量时间戳）
//  3. n2 收到（Observe）
//  4. n2 本地事件
//  5. n3 独立本地事件（与上述任何通信都无关 → 与 n2 并发）
//
// 离线可跑，轨迹完全确定。
func Demo(ctx context.Context) (*DemoResult, error) {
	_ = ctx

	ids := []core.NodeID{"n1", "n2", "n3"}

	// —— Vector Clock 侧 ——
	vc1 := NewVectorClock("n1", ids)
	vc2 := NewVectorClock("n2", ids)
	vc3 := NewVectorClock("n3", ids)

	// —— Lamport Clock 侧（同序列并行推进）——
	lc1 := &core.LamportClock{}
	lc2 := &core.LamportClock{}
	lc3 := &core.LamportClock{}

	// 1. n1 本地事件
	vc1.Tick()
	lc1.Tick()

	// 2. n1 发消息给 n2：把当前时间戳"打标"到消息上
	msgVector := vc1.Now()
	msgLamport := lc1.Now()

	// 3. n2 收到（Observe）
	vc2.Observe(msgVector)
	lc2.Observe(msgLamport)

	// 4. n2 本地事件
	vc2.Tick()
	lc2.Tick()

	// 5. n3 独立本地事件（从不与 n1/n2 通信）
	vc3.Tick()
	lc3.Tick()

	vectorFinalN1 := msgVector // n1 发送时刻的快照（已被 Observe 读取，快照本身不变）
	vectorFinalN2 := vc2.Now()
	vectorFinalN3 := vc3.Now()

	return &DemoResult{
		LamportFinal:     lc2.Now(),
		VectorFinalN2:    vectorFinalN2,
		VectorFinalN1:    vectorFinalN1,
		VectorFinalN3:    vectorFinalN3,
		N2vsN3:           Compare(vectorFinalN2, vectorFinalN3),
		SerializedN1vsN2: Compare(vectorFinalN1, vectorFinalN2),
	}, nil
}
