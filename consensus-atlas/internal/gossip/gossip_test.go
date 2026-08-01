package gossip

import (
	"context"
	"reflect"
	"testing"

	"github.com/QiuShichang/consensus-atlas/internal/core"
)

// TestDemoRuns 验证 5 节点 Gossip demo 离线可跑且最终收敛到全 5 个键。
func TestDemoRuns(t *testing.T) {
	res, err := Demo(context.Background())
	if err != nil {
		t.Fatalf("Demo 失败: %v", err)
	}
	if res.NodeCount != 5 {
		t.Errorf("节点数应为 5，实际 %d", res.NodeCount)
	}
	if !res.Converged {
		t.Error("demo 应在限定轮数内收敛")
	}
	if len(res.FinalState) != 5 {
		t.Errorf("收敛后应有 5 个键，实际 %d (%v)", len(res.FinalState), res.FinalState)
	}
	if res.Rounds <= 0 {
		t.Errorf("收敛所用轮数应为正数，实际 %d", res.Rounds)
	}
	// 收敛后状态必须正好等于 wantState。
	if !reflect.DeepEqual(res.FinalState, wantState) {
		t.Errorf("收敛状态不匹配：got %v want %v", res.FinalState, wantState)
	}
}

// TestMerge 验证合并语义：新 key 加入；冲突 key 取字符串序更大的值。
func TestMerge(t *testing.T) {
	cases := []struct {
		name string
		dst  map[string]string
		src  map[string]string
		want map[string]string
	}{
		{
			name: "新键加入",
			dst:  map[string]string{"a": "1"},
			src:  map[string]string{"b": "2"},
			want: map[string]string{"a": "1", "b": "2"},
		},
		{
			name: "冲突取更大值",
			dst:  map[string]string{"a": "1"},
			src:  map[string]string{"a": "9"},
			want: map[string]string{"a": "9"},
		},
		{
			name: "冲突保留更大值",
			dst:  map[string]string{"a": "5"},
			src:  map[string]string{"a": "2"},
			want: map[string]string{"a": "5"},
		},
		{
			name: "混合：部分新增部分覆盖",
			dst:  map[string]string{"a": "1", "b": "8"},
			src:  map[string]string{"b": "3", "c": "9", "a": "7"},
			want: map[string]string{"a": "7", "b": "8", "c": "9"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// 用副本避免污染。
			dst := make(map[string]string, len(tc.dst))
			for k, v := range tc.dst {
				dst[k] = v
			}
			Merge(dst, tc.src)
			if !reflect.DeepEqual(dst, tc.want) {
				t.Errorf("合并结果不匹配：got %v want %v", dst, tc.want)
			}
		})
	}
}

// TestSingleNode 验证单节点（无邻居）Gossip 时状态不变、不 panic、不报错。
func TestSingleNode(t *testing.T) {
	tr := core.NewMemTransport()
	n := NewNode("solo", []core.NodeID{"solo"}, tr)
	n.Set("x", "1")
	n.Start()

	before := n.snapshot()
	// 跑若干 tick，应无事发生：没有邻居可联系。
	for i := 0; i < 10; i++ {
		n.Tick()
		tr.Drain()
	}
	after := n.snapshot()
	if !reflect.DeepEqual(before, after) {
		t.Errorf("单节点状态不应变化：before %v after %v", before, after)
	}
	if n.LastPeer() != "" {
		t.Errorf("单节点不应联系任何邻居，实际 LastPeer=%s", n.LastPeer())
	}
}

// TestConvergence 验证 3 节点跑足够轮次后状态完全一致（最终一致性）。
func TestConvergence(t *testing.T) {
	tr := core.NewMemTransport()
	ids := []core.NodeID{"a", "b", "c"}
	seed := map[core.NodeID]map[string]string{
		"a": {"k1": "1"},
		"b": {"k2": "2"},
		"c": {"k3": "3"},
	}
	nodes := make(map[core.NodeID]*Node, len(ids))
	for _, id := range ids {
		n := NewNode(id, ids, tr)
		for k, v := range seed[id] {
			n.Set(k, v)
		}
		n.Start()
		nodes[id] = n
	}

	want := map[string]string{"k1": "1", "k2": "2", "k3": "3"}
	// 3 节点 round-robin，跑 20 轮足够收敛。
	for r := 0; r < 20; r++ {
		for _, id := range ids {
			nodes[id].Tick()
		}
		for d := 0; d < 4; d++ {
			tr.Drain()
		}
	}
	for _, id := range ids {
		got := nodes[id].snapshot()
		if !reflect.DeepEqual(got, want) {
			t.Errorf("节点 %s 未收敛：got %v want %v", id, got, want)
		}
	}
}

// TestPushPullRound 验证一次完整 Push-Pull 往返后，双方状态合并成并集。
// 这是对 Gossip 核心机制的最小单元测试（区别于 demo 的多轮收敛）。
func TestPushPullRound(t *testing.T) {
	tr := core.NewMemTransport()
	ids := []core.NodeID{"a", "b"}
	na := NewNode("a", ids, tr)
	na.Set("x", "1")
	nb := NewNode("b", ids, tr)
	nb.Set("y", "9")
	na.Start()
	nb.Start()

	// a 主动 tick：向 b Push 自己的状态。
	na.Tick()
	// drain 推进：b 收 Request→合并→回 Response；a 收 Response→合并。
	for d := 0; d < 4; d++ {
		tr.Drain()
	}

	wantBoth := map[string]string{"x": "1", "y": "9"}
	if got := na.snapshot(); !reflect.DeepEqual(got, wantBoth) {
		t.Errorf("a 一次往返后应有双方状态：got %v want %v", got, wantBoth)
	}
	if got := nb.snapshot(); !reflect.DeepEqual(got, wantBoth) {
		t.Errorf("b 一次往返后应有双方状态：got %v want %v", got, wantBoth)
	}
	if na.LastPeer() != "b" {
		t.Errorf("a 应已联系 b，LastPeer=%s", na.LastPeer())
	}
}

// TestPickPeerOrder 验证 round-robin 选邻居的顺序是确定性的。
func TestPickPeerOrder(t *testing.T) {
	tr := core.NewMemTransport()
	// peers 列表把自己放在中间，验证跳过自身。
	n := NewNode("self", []core.NodeID{"p1", "self", "p2"}, tr)
	want := []core.NodeID{"p1", "p2", "p1", "p2"}
	for i, w := range want {
		got, ok := n.pickPeer()
		if !ok {
			t.Fatalf("第 %d 次 pickPeer 应成功", i)
		}
		if got != w {
			t.Errorf("第 %d 次 pickPeer = %s, want %s", i, got, w)
		}
	}
}
