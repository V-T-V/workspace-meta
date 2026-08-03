package clock

import (
	"context"
	"testing"

	"github.com/QiuShichang/consensus-atlas/internal/core"
)

// TestLamportTick 验证 core.LamportClock 连续两次 Tick 返回 1, 2。
func TestLamportTick(t *testing.T) {
	lc := &core.LamportClock{}
	if c := lc.Tick(); c != 1 {
		t.Errorf("第一次 Tick 应为 1，实际 %d", c)
	}
	if c := lc.Tick(); c != 2 {
		t.Errorf("第二次 Tick 应为 2，实际 %d", c)
	}
}

// TestLamportObserve 验证 max+1 规则：本地=5，收到 8，observe 后应为 9。
func TestLamportObserve(t *testing.T) {
	lc := &core.LamportClock{}
	// 先把本地时钟推到 5。
	for i := 0; i < 5; i++ {
		lc.Tick()
	}
	if lc.Now() != 5 {
		t.Fatalf("前置：本地时钟应为 5，实际 %d", lc.Now())
	}
	// 收到时间戳 8，应取 max(5,8)+1 = 9。
	if c := lc.Observe(8); c != 9 {
		t.Errorf("Observe(8) 后应为 9，实际 %d", c)
	}
}

// TestVectorTick 验证 3 节点 vector，owner n2 的 Tick 只递增自己的分量。
func TestVectorTick(t *testing.T) {
	ids := []core.NodeID{"n1", "n2", "n3"}
	vc := NewVectorClock("n2", ids)

	v1 := vc.Tick()
	if v1["n2"] != 1 {
		t.Errorf("第一次 Tick 后 n2 分量应为 1，实际 %d", v1["n2"])
	}
	if v1["n1"] != 0 || v1["n3"] != 0 {
		t.Errorf("非 owner 分量应保持 0，实际 n1=%d n3=%d", v1["n1"], v1["n3"])
	}

	v2 := vc.Tick()
	if v2["n2"] != 2 {
		t.Errorf("第二次 Tick 后 n2 分量应为 2，实际 %d", v2["n2"])
	}
	if v2["n1"] != 0 || v2["n3"] != 0 {
		t.Errorf("非 owner 分量应保持 0，实际 n1=%d n3=%d", v2["n1"], v2["n3"])
	}
}

// TestVectorCompare 验证 Compare 的四种关系判定。
func TestVectorCompare(t *testing.T) {
	// (1,0,0) vs (2,1,0)：前者严格 ≤ 后者 → HappensBefore。
	a := map[core.NodeID]uint64{"n1": 1, "n2": 0, "n3": 0}
	b := map[core.NodeID]uint64{"n1": 2, "n2": 1, "n3": 0}
	if r := Compare(a, b); r != HappensBefore {
		t.Errorf("(1,0,0) vs (2,1,0) 应为 HappensBefore，实际 %s", r)
	}

	// (1,1,0) vs (0,0,1)：n1 上 a>b，n2 上 a>b，n3 上 a<b → Concurrent。
	c := map[core.NodeID]uint64{"n1": 1, "n2": 1, "n3": 0}
	d := map[core.NodeID]uint64{"n1": 0, "n2": 0, "n3": 1}
	if r := Compare(c, d); r != Concurrent {
		t.Errorf("(1,1,0) vs (0,0,1) 应为 Concurrent，实际 %s", r)
	}

	// 相等向量 → Equal。
	e := map[core.NodeID]uint64{"n1": 2, "n2": 3, "n3": 1}
	f := map[core.NodeID]uint64{"n1": 2, "n2": 3, "n3": 1}
	if r := Compare(e, f); r != Equal {
		t.Errorf("相等向量应为 Equal，实际 %s", r)
	}

	// 反向 HappensBefore 应判为 HappensAfter。
	if r := Compare(b, a); r != HappensAfter {
		t.Errorf("(2,1,0) vs (1,0,0) 应为 HappensAfter，实际 %s", r)
	}
}

// TestVectorObserve 验证分量 max + owner +1 规则。
func TestVectorObserve(t *testing.T) {
	ids := []core.NodeID{"n1", "n2", "n3"}
	vc := NewVectorClock("n2", ids)
	// owner=n2，先本地推一次 → n2=1。
	vc.Tick()
	// 收到来自 n1 的消息：n1=5, n3=2。
	incoming := map[core.NodeID]uint64{"n1": 5, "n2": 0, "n3": 2}
	out := vc.Observe(incoming)
	// 每个分量取 max：n1=max(0,5)=5, n2=max(1,0)=1, n3=max(0,2)=2
	// 然后 owner n2 再 +1 → n2=2。
	if out["n1"] != 5 {
		t.Errorf("Observe 后 n1 应为 5，实际 %d", out["n1"])
	}
	if out["n2"] != 2 {
		t.Errorf("Observe 后 owner n2 应为 max(1,0)+1=2，实际 %d", out["n2"])
	}
	if out["n3"] != 2 {
		t.Errorf("Observe 后 n3 应为 2，实际 %d", out["n3"])
	}
}

// TestDemoRuns 验证 Demo 离线可跑，且因果判定符合预期。
func TestDemoRuns(t *testing.T) {
	res, err := Demo(context.Background())
	if err != nil {
		t.Fatalf("Demo 失败: %v", err)
	}
	if res.LamportFinal == 0 {
		t.Error("Lamport 终值不应为 0")
	}
	if res.N2vsN3 != Concurrent {
		t.Errorf("n2 与 n3 未通信，应为 Concurrent，实际 %s", res.N2vsN3)
	}
	if res.SerializedN1vsN2 != HappensBefore {
		t.Errorf("n1 发送 → n2 接收应为 HappensBefore，实际 %s", res.SerializedN1vsN2)
	}
}

// TestCausalOrder 验证基于 Lamport 时钟的因果序判定。
func TestCausalOrder(t *testing.T) {
	// 模拟因果链：n1 发消息给 n2，n2 发消息给 n3
	// n1 本地事件 e1 = tick (Lamport=1)
	// n1 发消息给 n2：n2 observe(1) → Lamport=2
	// n2 本地事件 e2 = tick → Lamport=3
	// n2 发消息给 n3：n3 observe(3) → Lamport=4
	// n3 本地事件 e3 = tick → Lamport=5
	// 同时 n1 独立事件 e0 = tick → Lamport=2（与 n3 的 e3 并发）

	lc1 := &core.LamportClock{}
	lc2 := &core.LamportClock{}
	lc3 := &core.LamportClock{}

	e1 := lc1.Tick()       // n1 本地事件 = 1
	t12 := lc1.Tick()      // n1 发消息前 tick = 2
	o2 := lc2.Observe(t12) // n2 收到 = 3
	e2 := lc2.Tick()       // n2 本地 = 4
	t23 := lc2.Tick()      // n2 发消息前 tick = 5
	o3 := lc3.Observe(t23) // n3 收到 = 6
	e3 := lc3.Tick()       // n3 本地 = 7

	e0 := lc1.Tick() // n1 独立事件 = 3（与 e3 并发，无消息交互）

	// 因果链：e1(1) → e2(4) → e3(7)
	if e1 >= e2 {
		t.Errorf("e1(%d) 应 < e2(%d)（因果序）", e1, e2)
	}
	if e2 >= e3 {
		t.Errorf("e2(%d) 应 < e3(%d)（因果序）", e2, e3)
	}
	// e0 与 e3 并发：e0(3) 和 e3(7) 无因果关系
	// Lamport 不能区分"因果序"和"并发"——但可验证 e0 < e3（数值上）
	// 真正的并发判定需要 Vector Clock（clock 包已有）
	_ = o2
	_ = o3
	_ = e0
}

func TestVectorClockConcurrency(t *testing.T) {
	n1 := NewVectorClock("n1", []core.NodeID{"n1", "n2", "n3"})
	n3 := NewVectorClock("n3", []core.NodeID{"n1", "n2", "n3"})
	// n1 和 n3 各自独立操作无消息交互
	v1 := n1.Tick()
	v3 := n3.Tick()
	rel := Compare(v1, v3)
	if rel != Concurrent {
		t.Errorf("独立操作应 Concurrent，实际 %s", rel)
	}
}

func TestVectorClockHappensBefore(t *testing.T) {
	n1 := NewVectorClock("n1", []core.NodeID{"n1", "n2"})
	n2 := NewVectorClock("n2", []core.NodeID{"n1", "n2"})
	// n1 → n2 消息
	v1 := n1.Tick()
	v2 := n2.Observe(v1)
	rel := Compare(v1, v2)
	if rel != HappensBefore {
		t.Errorf("n1→n2 应 HappensBefore，实际 %s", rel)
	}
}
