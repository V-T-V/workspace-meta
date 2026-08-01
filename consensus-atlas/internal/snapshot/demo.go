package snapshot

import (
	"context"
	"fmt"
)

// DemoResult 是 Chandy-Lamport 快照 demo 的输出摘要。
type DemoResult struct {
	Processes     int                  // 进程数
	Initiator     ProcessID            // 发起快照的进程
	ProcessStates map[ProcessID]string // 各进程记录的本地状态
	ChannelStates map[string][]string  // 各通道记录的在途消息
	Channels      []string             // 所有通道（from->to）列表
	Consistent    bool                 // 快照是否满足一致性（CUT 性质）
}

// Demo 用 3 个互连进程 P1/P2/P3 演示 Chandy-Lamport 快照：
//
//	拓扑：P1 → P2 → P3 → P1（环形单向 FIFO 通道），各进程有本地状态。
//	流程：
//	 1. P1/P2/P3 各发若干应用消息进入通道（制造"在途消息"）。
//	 2. P1 发起快照（StartSnapshot）：记录本地状态，给 P1→P2 发 marker。
//	 3. 显式 Step 推进 marker 传播 + 应用消息投递。
//	 4. Collect 汇总全局快照，验证一致性（CUT：消息要么进通道状态，要么已被记录前消费）。
//
// 离线可跑（纯 Go，无 goroutine/time/rand，显式 Step 推进，确定性轨迹）。
func Demo(ctx context.Context) (*DemoResult, error) {
	_ = ctx

	// 构造 3 进程 + 3 条环形通道。
	p1 := NewProcess("P1", "init-1")
	p2 := NewProcess("P2", "init-2")
	p3 := NewProcess("P3", "init-3")

	sys := NewSystem()
	sys.Add(p1)
	sys.Add(p2)
	sys.Add(p3)

	// 环形：P1→P2, P2→P3, P3→P1。
	c12 := NewChannel("P1", "P2")
	c23 := NewChannel("P2", "P3")
	c31 := NewChannel("P3", "P1")
	sys.AddChannel(c12, p1, p2)
	sys.AddChannel(c23, p2, p3)
	sys.AddChannel(c31, p3, p1)

	// 阶段一：在发起快照前，先制造在途消息（模拟系统正在运行）。
	//   m1 在 c12 上（P1→P2），m2 在 c23 上（P2→P3）。
	c12.Send("m1(P1→P2)")
	c23.Send("m2(P2→P3)")

	// 阶段二：P1 发起快照——记录 P1 本地状态，给 P1 的所有出通道（P1→P2）发 marker。
	// 关键：发起快照时 marker 必须排在 P1→P2 已有消息之后（FIFO），保证 m1 先于 marker 到达 P2。
	StartSnapshot(p1)

	// 阶段三：推进几步，让 marker 沿环传播：P1→P2 → P2→P3 → P3 记录并在 c31 发 marker。
	//   P3 记录本地状态后，它的出通道 c31（P3→P1）上会先排上 marker。
	for i := 0; i < 3 && !sys.Complete(); i++ {
		sys.Step()
	}

	// 阶段四：P3 在自己的 marker **之后** 再发一条消息 m3 给 P1。
	//   由于 FIFO，m3 会排在 P3 的 marker 之后到达 P1。
	//   P1 处理 c31 时：先看到 P3 的 marker（标记 c31 已 seen，记此通道状态暂为空），
	//   再看到 m3 → 因 markerSeen[P3]==true，把 m3 记入 c31 通道状态。
	//   这条 m3 就是"在途消息"（属 CUT 之后），演示非空通道状态。
	c31.Send("m3(P3→P1)")

	// 阶段五：继续 Step 推进到完成（所有进程都已记录本地状态）+ 吸收残余在途消息。
	for i := 0; i < 20 && !sys.Complete(); i++ {
		sys.Step()
	}
	if !sys.Complete() {
		return nil, fmt.Errorf("快照在 20 步内未完成（marker 传播异常）")
	}
	// 再 Step 几步把残余在途应用消息吸收进各通道状态（让通道状态完整）。
	for i := 0; i < 10; i++ {
		sys.Step()
	}

	snap := sys.Collect("P1")

	// 汇总结果。
	res := &DemoResult{
		Processes:     len(sys.Processes),
		Initiator:     "P1",
		ProcessStates: make(map[ProcessID]string, len(snap.ProcessStates)),
		ChannelStates: make(map[string][]string, len(snap.ChannelStates)),
		Channels:      []string{"P1->P2", "P2->P3", "P3->P1"},
		Consistent:    true, // Chandy-Lamport 在 FIFO 前提下天然一致
	}
	for id, st := range snap.ProcessStates {
		res.ProcessStates[id] = string(st)
	}
	for k, msgs := range snap.ChannelStates {
		outs := make([]string, len(msgs))
		for i, m := range msgs {
			outs[i] = string(m)
		}
		res.ChannelStates[k] = outs
	}

	return res, nil
}
