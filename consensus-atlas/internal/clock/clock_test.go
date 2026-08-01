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
