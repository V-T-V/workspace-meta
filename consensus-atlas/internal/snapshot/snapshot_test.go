package snapshot

import (
	"context"
	"testing"
)

// TestDemoRuns 验证 demo 离线可跑：3 进程都记录了本地状态，快照一致。
func TestDemoRuns(t *testing.T) {
	res, err := Demo(context.Background())
	if err != nil {
		t.Fatalf("Demo 失败: %v", err)
	}
	if res.Processes != 3 {
		t.Errorf("应有 3 进程，实际 %d", res.Processes)
	}
	if res.Initiator != "P1" {
		t.Errorf("发起者应为 P1，实际 %s", res.Initiator)
	}
	// 每个进程都应记录了本地状态。
	for _, id := range []ProcessID{"P1", "P2", "P3"} {
		if _, ok := res.ProcessStates[id]; !ok {
			t.Errorf("进程 %s 未记录本地状态", id)
		}
	}
	if !res.Consistent {
		t.Error("FIFO 通道前提下快照应一致")
	}
}

// TestFIFOMarkerOrdering 验证 FIFO：marker 必须严格排在它之前发出的消息之后。
// P1 先发 m1 再发 marker；P2 应先收到 m1（消费），再收到 marker（触发记录）。
func TestFIFOMarkerOrdering(t *testing.T) {
	p1 := NewProcess("P1", "s1")
	p2 := NewProcess("P2", "s2")
	sys := NewSystem()
	sys.Add(p1)
	sys.Add(p2)
	c := NewChannel("P1", "P2")
	sys.AddChannel(c, p1, p2)

	c.Send("m1")      // 应用消息
	StartSnapshot(p1) // marker 排在 m1 之后

	// Step 1：投递 m1（在 P2 还未记录时正常消费）。
	n := sys.Step()
	if n != 1 {
		t.Errorf("第 1 步应投递 1 条消息，实际 %d", n)
	}
	if p2.State != "m1" {
		t.Errorf("P2 应消费 m1，实际 State=%s", p2.State)
	}
	if p2.recorded {
		t.Error("m1 不应触发 P2 记录（它不是 marker）")
	}

	// Step 2：投递 marker，触发 P2 记录。
	sys.Step()
	if !p2.recorded {
		t.Error("P2 收到 marker 应记录本地状态")
	}
}

// TestChannelStateRecordedAfterMarker 验证：marker 之后到达某入通道的应用消息
// 被记入该通道状态（而非被消费）。
func TestChannelStateRecordedAfterMarker(t *testing.T) {
	p1 := NewProcess("P1", "s1")
	p2 := NewProcess("P2", "s2")
	sys := NewSystem()
	sys.Add(p1)
	sys.Add(p2)
	c := NewChannel("P1", "P2")
	sys.AddChannel(c, p1, p2)

	// P1 发起快照（marker 入队）。
	StartSnapshot(p1)
	// marker 之后 P1 又发一条消息——这条应被记入 P2 的通道状态。
	c.Send("post-marker-msg")

	// 推进到完成。
	for i := 0; i < 10 && !sys.Complete(); i++ {
		sys.Step()
	}
	if !sys.Complete() {
		t.Fatal("快照未完成")
	}
	// 再 step 吸收 post-marker-msg 到通道状态。
	for i := 0; i < 5; i++ {
		sys.Step()
	}

	// P2 的 c1 通道状态应包含 post-marker-msg。
	msgs := p2.channelState["P1"]
	found := false
	for _, m := range msgs {
		if string(m) == "post-marker-msg" {
			found = true
		}
	}
	if !found {
		t.Errorf("marker 之后的消息应记入通道状态，实际 channelState=%v", msgs)
	}
}

// TestInitiatorRecordsFirst 发起者应在发出 marker 前先记录本地状态。
func TestInitiatorRecordsFirst(t *testing.T) {
	p1 := NewProcess("P1", "init")
	StartSnapshot(p1)
	st, ok := p1.LocalSnapshot()
	if !ok {
		t.Fatal("发起者应记录本地状态")
	}
	if string(st) != "init" {
		t.Errorf("发起者应记录发起时刻的本地状态，实际 %s", st)
	}
}

// TestCompleteAfterMarkerPropagation 所有进程都收到 marker 后 Complete 为真。
func TestCompleteAfterMarkerPropagation(t *testing.T) {
	p1 := NewProcess("P1", "a")
	p2 := NewProcess("P2", "b")
	p3 := NewProcess("P3", "c")
	sys := NewSystem()
	sys.Add(p1)
	sys.Add(p2)
	sys.Add(p3)
	c12 := NewChannel("P1", "P2")
	c23 := NewChannel("P2", "P3")
	c31 := NewChannel("P3", "P1")
	sys.AddChannel(c12, p1, p2)
	sys.AddChannel(c23, p2, p3)
	sys.AddChannel(c31, p3, p1)

	StartSnapshot(p1)
	for i := 0; i < 10 && !sys.Complete(); i++ {
		sys.Step()
	}
	if !sys.Complete() {
		t.Error("marker 环传一圈后应 Complete")
	}
}

// TestEmptyChannelSnapshot 通道在 marker 前无在途消息时，通道状态为空。
func TestEmptyChannelSnapshot(t *testing.T) {
	p1 := NewProcess("P1", "s1")
	p2 := NewProcess("P2", "s2")
	sys := NewSystem()
	sys.Add(p1)
	sys.Add(p2)
	c := NewChannel("P1", "P2")
	sys.AddChannel(c, p1, p2)

	StartSnapshot(p1)
	for i := 0; i < 10 && !sys.Complete(); i++ {
		sys.Step()
	}
	// c 在快照发起时无在途消息，且 marker 后无新消息 → P2 的通道状态为空。
	if msgs := p2.channelState["P1"]; len(msgs) != 0 {
		t.Errorf("空通道的通道状态应为空，实际 %v", msgs)
	}
}
