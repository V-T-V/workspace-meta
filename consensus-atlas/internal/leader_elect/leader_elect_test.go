package leader_elect

import (
	"context"
	"testing"

	"github.com/QiuShichang/consensus-atlas/internal/core"
)

// TestDemoRuns 验证 demo 离线可跑，Bully 选出 4（5 离线），Ring 选出 5。
func TestDemoRuns(t *testing.T) {
	res, err := Demo(context.Background())
	if err != nil {
		t.Fatalf("Demo 失败: %v", err)
	}
	if res.BullyLeader != 4 {
		t.Errorf("Bully 当选者应为 4（5 离线），实际 %d", res.BullyLeader)
	}
	if res.RingLeader != 5 {
		t.Errorf("Ring 当选者应为 5（最大 ID），实际 %d", res.RingLeader)
	}
}

// TestBullyElection 验证完整 Bully 流程：5 节点，5 离线，3 发起，最终 4 当选。
func TestBullyElection(t *testing.T) {
	tr := core.NewMemTransport()
	ids := []uint64{1, 2, 3, 4, 5}
	nodes := make(map[uint64]*BullyNode, len(ids))
	for _, id := range ids {
		n := NewBullyNode(id, ids, tr)
		n.Start()
		nodes[id] = n
	}
	nodes[5].Online = false // Leader 离线

	nodes[3].StartElection()
	RunBullyToCompletion(tr, nodes, ids)

	// 所有在线节点应公认 4 为新 Leader。
	for _, id := range ids {
		if id == 5 {
			continue // 离线节点不参与
		}
		if nodes[id].Leader() != 4 {
			t.Errorf("节点 %d 应认为 Leader=4，实际 %d", id, nodes[id].Leader())
		}
	}
}

// TestBullyHighestOnline 所有节点在线时，最高 ID 当选。
func TestBullyHighestOnline(t *testing.T) {
	tr := core.NewMemTransport()
	ids := []uint64{1, 2, 3, 4, 5}
	nodes := make(map[uint64]*BullyNode, len(ids))
	for _, id := range ids {
		n := NewBullyNode(id, ids, tr)
		n.Start()
		nodes[id] = n
	}
	// 节点 1 发起选举。
	nodes[1].StartElection()
	RunBullyToCompletion(tr, nodes, ids)

	// 全部在线，最高 ID = 5 当选。
	for _, id := range ids {
		if nodes[id].Leader() != 5 {
			t.Errorf("节点 %d 应认为 Leader=5，实际 %d", id, nodes[id].Leader())
		}
	}
}

// TestRingElection 环上 5 节点选举，最大 ID 当选。
func TestRingElection(t *testing.T) {
	tr := core.NewMemTransport()
	ring := []uint64{1, 2, 3, 4, 5}
	nodes := make(map[uint64]*RingNode, len(ring))
	for i, id := range ring {
		next := ring[(i+1)%len(ring)]
		n := NewRingNode(id, next, tr)
		n.Start()
		nodes[id] = n
	}
	// 任意节点发起都应选出 5。
	nodes[3].StartElection()
	for i := 0; i < 15; i++ {
		tr.Drain()
	}

	for _, id := range ring {
		if nodes[id].Leader() != 5 {
			t.Errorf("Ring 节点 %d 应认为 Leader=5，实际 %d", id, nodes[id].Leader())
		}
	}
}

// TestBullyCoordinator 收到 Coordinator 后 knowsLeader 应更新。
func TestBullyCoordinator(t *testing.T) {
	tr := core.NewMemTransport()
	n := NewBullyNode(2, []uint64{1, 2, 3}, tr)
	n.Start()
	if n.Leader() != 0 {
		t.Error("初始 Leader 应为 0（未知）")
	}
	// 模拟收到来自节点 3 的 Coordinator 公告。
	n.transport.Send(core.Message{
		From: BullyKey(3), To: BullyKey(2),
		Kind:    KindCoordinator,
		Payload: CoordinatorPayload{Leader: 3},
	})
	tr.Drain()
	if n.Leader() != 3 {
		t.Errorf("收到 Coordinator 后 Leader 应为 3，实际 %d", n.Leader())
	}
}

// TestBullyOfflineDropsMessages 离线节点不应回应 Election。
func TestBullyOfflineDropsMessages(t *testing.T) {
	tr := core.NewMemTransport()
	high := NewBullyNode(5, []uint64{1, 5}, tr)
	high.Start()
	high.Online = false

	low := NewBullyNode(1, []uint64{1, 5}, tr)
	low.Start()
	low.StartElection()
	RunBullyToCompletion(tr, map[uint64]*BullyNode{1: low, 5: high}, []uint64{1, 5})
	// 高节点离线，low 无 Answer，应自己当选。
	if low.Leader() != 1 {
		t.Errorf("更高节点离线时 low 应当选（Leader=1），实际 %d", low.Leader())
	}
	if high.Leader() != 0 {
		t.Errorf("离线节点不应更新 Leader，实际 %d", high.Leader())
	}
}
