package crdt

import (
	"context"
	"testing"

	"github.com/QiuShichang/consensus-atlas/internal/core"
)

// TestDemoRuns 验证 demo 离线可跑：3 节点并发增量后最终收敛到相同 Value。
func TestDemoRuns(t *testing.T) {
	res, err := Demo(context.Background())
	if err != nil {
		t.Fatalf("Demo 失败: %v", err)
	}
	if res.NodeCount != 3 {
		t.Errorf("应有 3 个节点，实际 %d", res.NodeCount)
	}
	if !res.Converged {
		t.Fatal("G-Counter 应最终收敛")
	}
	if res.Expected != 10 {
		t.Errorf("期望值应为 3+5+2=10，实际 %d", res.Expected)
	}
	for id, v := range res.FinalValue {
		if v != res.Expected {
			t.Errorf("节点 %s 的 Value 应为 %d，实际 %d（未收敛）", id, res.Expected, v)
		}
	}
}

// TestIncrementGrowOnly 验证 Increment 只改自己那一维（grow-only 约束）。
func TestIncrementGrowOnly(t *testing.T) {
	g := NewGCounter("a", []core.NodeID{"a", "b", "c"})
	if g.Local() != 0 {
		t.Error("初始本地计数应为 0")
	}
	g.Increment(5)
	g.Increment(3)
	if g.Local() != 8 {
		t.Errorf("两次 Increment(5+3) 后本地应为 8，实际 %d", g.Local())
	}
	// 别的节点维度应保持 0（只增自己那一维）。
	if g.Get("b") != 0 || g.Get("c") != 0 {
		t.Error("Increment 不应改动别的节点维度")
	}
	if g.Value() != 8 {
		t.Errorf("Value 应为 8，实际 %d", g.Value())
	}
}

// TestMergeIsMax 验证 Merge 是逐分量取 max（不是 sum，sum 会重复计数）。
func TestMergeIsMax(t *testing.T) {
	a := NewGCounter("a", []core.NodeID{"a", "b"})
	b := NewGCounter("b", []core.NodeID{"a", "b"})
	a.Increment(5) // a 的分量 = 5
	b.Increment(7) // b 的分量 = 7

	a.Merge(b)
	// max(5,0)=5 for a；max(0,7)=7 for b；sum = 12。
	if a.Get("a") != 5 || a.Get("b") != 7 {
		t.Errorf("Merge 后应 a=5,b=7（max），实际 a=%d b=%d", a.Get("a"), a.Get("b"))
	}
	if a.Value() != 12 {
		t.Errorf("Value 应为 12，实际 %d（若为 sum 重复计数会更大）", a.Value())
	}
}

// TestMergeIdempotent 验证 Merge 幂等：重复合并同一副本不改变结果。
// 幂等性是 CRDT 收敛性的根基（max 满足幂等律）。
func TestMergeIdempotent(t *testing.T) {
	a := NewGCounter("a", []core.NodeID{"a", "b"})
	b := NewGCounter("b", []core.NodeID{"a", "b"})
	a.Increment(5)
	b.Increment(7)

	a.Merge(b)
	v1 := a.Value()
	a.Merge(b) // 重复
	a.Merge(b) // 再重复
	if a.Value() != v1 {
		t.Errorf("Merge 应幂等，重复合并改变了 Value：%d → %d", v1, a.Value())
	}
}

// TestMergeCommutative 验证 Merge 可交换：merge(a,b) 与 merge(b,a) 结果一致。
func TestMergeCommutative(t *testing.T) {
	a := NewGCounter("a", []core.NodeID{"a", "b"})
	b := NewGCounter("b", []core.NodeID{"a", "b"})
	a.Increment(5)
	b.Increment(7)

	// 副本 a 合并 b，副本 b 合并 a：两边应得到相同 Value（收敛）。
	a.Merge(b)
	b.Merge(a)
	if a.Value() != b.Value() {
		t.Errorf("Merge 应可交换收敛，a=%d b=%d", a.Value(), b.Value())
	}
}

// TestConvergesAcrossNodes 验证多节点充分两两 Merge 后最终 Value 相同
// （无论合并顺序如何——CRDT 的核心保证）。
//
// 注意：CRDT 的收敛要求"信息充分传播"——每个节点的每个分量都要被传播到所有副本。
// 单条因果链（a←b←c←a）只让链尾节点拿到全量，前端节点缺失后续分量。因此本测试
// 让每个节点都与所有其他节点各做一次 Merge（等价于一轮全连接同步），保证充分传播。
func TestConvergesAcrossNodes(t *testing.T) {
	ids := []core.NodeID{"a", "b", "c"}
	counters := map[core.NodeID]*GCounter{}
	for _, id := range ids {
		counters[id] = NewGCounter(id, ids)
	}
	counters["a"].Increment(1)
	counters["b"].Increment(2)
	counters["c"].Increment(4)

	// 充分传播：每个节点吸收其他所有节点（乱序，验证交换/结合性）。
	// 一轮全连接后，每个副本都应包含全部三个分量。
	for _, dst := range ids {
		for _, src := range ids {
			if dst == src {
				continue
			}
			counters[dst].Merge(counters[src])
		}
	}

	// 全部应等于 1+2+4=7。
	for _, id := range ids {
		if v := counters[id].Value(); v != 7 {
			t.Errorf("节点 %s 收敛后 Value 应为 7，实际 %d", id, v)
		}
	}
}

// TestCompareCausality 验证基于向量分量的因果关系判定。
func TestCompareCausality(t *testing.T) {
	ids := []core.NodeID{"a", "b"}
	base := NewGCounter("a", ids)
	base.Increment(1)

	newer := NewGCounter("a", ids)
	newer.Increment(1)
	newer.Increment(2) // newer 在 a 维度更大 → base ⊆ newer

	aSubB, bSubA := Compare(base, newer)
	if !aSubB {
		t.Error("base ⊆ newer 应为 true（base 是 newer 的祖先）")
	}
	if bSubA {
		t.Error("newer ⊆ base 应为 false")
	}

	// 相等状态：互为子集。
	eq1 := NewGCounter("a", ids)
	eq2 := NewGCounter("a", ids)
	eq1.Increment(3)
	eq2.Increment(3)
	aSubB, bSubA = Compare(eq1, eq2)
	if !aSubB || !bSubA {
		t.Error("相等状态应互为子集")
	}

	// 并发：a 维度更大 vs b 维度更大。
	c1 := NewGCounter("a", ids)
	c1.Increment(5) // a=5,b=0
	c2 := NewGCounter("b", ids)
	c2.Increment(5) // a=0,b=5
	aSubB, bSubA = Compare(c1, c2)
	if aSubB || bSubA {
		t.Error("互不包含应判为并发（两者都不是对方的子集）")
	}
}

// TestNodeExchange 验证通过 transport 交换向量后两个 Node 的 Value 收敛。
func TestNodeExchange(t *testing.T) {
	tr := core.NewMemTransport()
	ids := []core.NodeID{"x", "y"}
	nx := NewNode("x", ids, tr)
	ny := NewNode("y", ids, tr)
	nx.Start()
	ny.Start()

	nx.Counter.Increment(10)
	ny.Counter.Increment(20)

	// x 主动 Tick 一次（Push 给 y），y Merge 后 Pull 回来。
	nx.Tick()
	for i := 0; i < 4; i++ {
		tr.Drain()
	}
	// 一次 Push-Pull 往返应让双方都收敛到 30。
	if v := nx.Counter.Value(); v != 30 {
		t.Errorf("x 收敛后 Value 应为 30，实际 %d", v)
	}
	if v := ny.Counter.Value(); v != 30 {
		t.Errorf("y 收敛后 Value 应为 30，实际 %d", v)
	}
}
