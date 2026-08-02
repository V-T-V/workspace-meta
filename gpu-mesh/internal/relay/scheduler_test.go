package relay

import (
	"testing"

	"github.com/QiuShichang/gpu-mesh/internal/proto"
)

// makeAgent 构造测试用 Agent 视图。
func makeAgent(id string, yield string, models []string, freeMB int) AgentView {
	return AgentView{
		AgentID:  id,
		Engines:  []string{"ollama"},
		Models:   models,
		Online:   true,
		Yield:    proto.YieldState{Level: yield, Budget: proto.BudgetForLevel(yield)},
		GPUs:     []proto.GPUSnapshot{{MemTotalMB: 8192, MemUsedMB: 8192 - freeMB, UtilGPU: 0}},
	}
}

func TestSchedule_PreferIdleOverBusy(t *testing.T) {
	// 核心：让位感知——IDLE 节点应优先于 BUSY_HUMAN
	s := NewScheduler()
	agents := []AgentView{
		makeAgent("busy-01", proto.YieldBUSY, []string{"qwen2.5:7b"}, 4096),
		makeAgent("idle-01", proto.YieldIDLE, []string{"qwen2.5:7b"}, 4096),
		makeAgent("active-01", proto.YieldACTIVE, []string{"qwen2.5:7b"}, 4096),
	}
	req := ScheduleRequest{Model: "qwen2.5:7b", MinBudget: 10}
	got, err := s.Schedule(req, agents)
	if err != nil {
		t.Fatalf("Schedule 失败: %v", err)
	}
	if got != "idle-01" {
		t.Errorf("应优先选 IDLE 节点 idle-01，得到 %s", got)
	}
}

func TestSchedule_PreferActiveOverBusy(t *testing.T) {
	// 无 IDLE 时，ACTIVE 优于 BUSY
	s := NewScheduler()
	agents := []AgentView{
		makeAgent("busy-01", proto.YieldBUSY, []string{"m"}, 4096),
		makeAgent("active-01", proto.YieldACTIVE, []string{"m"}, 4096),
	}
	got, _ := s.Schedule(ScheduleRequest{Model: "m", MinBudget: 10}, agents)
	if got != "active-01" {
		t.Errorf("应选 ACTIVE 节点，得到 %s", got)
	}
}

func TestSchedule_MinBudgetFiltersBusy(t *testing.T) {
	// MinBudget=100 时，BUSY_HUMAN（budget=10）和 ACTIVE（50）都应被排除
	s := NewScheduler()
	agents := []AgentView{
		makeAgent("busy-01", proto.YieldBUSY, []string{"m"}, 4096),
		makeAgent("active-01", proto.YieldACTIVE, []string{"m"}, 4096),
		makeAgent("idle-01", proto.YieldIDLE, []string{"m"}, 4096),
	}
	// 批量任务设 MinBudget=100：只 IDLE 能跑
	got, err := s.Schedule(ScheduleRequest{Model: "m", MinBudget: 100}, agents)
	if err != nil {
		t.Fatalf("应返回 idle-01，却报错: %v", err)
	}
	if got != "idle-01" {
		t.Errorf("MinBudget=100 应只选 IDLE，得到 %s", got)
	}
}

func TestSchedule_MinBudgetNoCandidate(t *testing.T) {
	// 全是 BUSY 且 MinBudget=100 → 无候选
	s := NewScheduler()
	agents := []AgentView{
		makeAgent("busy-01", proto.YieldBUSY, []string{"m"}, 4096),
	}
	_, err := s.Schedule(ScheduleRequest{Model: "m", MinBudget: 100}, agents)
	if err == nil {
		t.Error("应报错无候选，却返回成功")
	}
}

func TestSchedule_VRAMFilter(t *testing.T) {
	// 显存不足的节点被排除
	s := NewScheduler()
	agents := []AgentView{
		makeAgent("low-mem", proto.YieldIDLE, []string{"m"}, 1024),  // 只剩 1GB
		makeAgent("ok-mem", proto.YieldIDLE, []string{"m"}, 6144),   // 剩 6GB
	}
	got, _ := s.Schedule(ScheduleRequest{Model: "m", MinBudget: 10, NeedMemMB: 4096}, agents)
	if got != "ok-mem" {
		t.Errorf("显存不足应被排除，应选 ok-mem，得到 %s", got)
	}
}

func TestSchedule_ReservedExcluded(t *testing.T) {
	// Phase 5：被训练独占的节点应排除
	s := NewScheduler()
	s.SetReservedChecker(func(id string) bool { return id == "training-01" })
	agents := []AgentView{
		makeAgent("training-01", proto.YieldIDLE, []string{"m"}, 8192),
		makeAgent("free-01", proto.YieldIDLE, []string{"m"}, 8192),
	}
	got, _ := s.Schedule(ScheduleRequest{Model: "m", MinBudget: 10}, agents)
	if got != "free-01" {
		t.Errorf("被独占的 training-01 应排除，选 free-01，得到 %s", got)
	}
}

func TestSchedule_ExplicitAgentID(t *testing.T) {
	// 显式指定 AgentID 时直接用（即使让位状态差）
	s := NewScheduler()
	agents := []AgentView{
		makeAgent("idle-01", proto.YieldIDLE, []string{"m"}, 8192),
		makeAgent("busy-01", proto.YieldBUSY, []string{"m"}, 8192),
	}
	got, _ := s.Schedule(ScheduleRequest{AgentID: "busy-01"}, agents)
	if got != "busy-01" {
		t.Errorf("显式指定应直接用，得到 %s", got)
	}
}

func TestSchedule_ExplicitAgentIDOffline(t *testing.T) {
	// 显式指定不在线的 Agent 报错
	s := NewScheduler()
	agents := []AgentView{makeAgent("a1", proto.YieldIDLE, []string{"m"}, 8192)}
	_, err := s.Schedule(ScheduleRequest{AgentID: "ghost"}, agents)
	if err == nil {
		t.Error("指定不在线 Agent 应报错")
	}
}

func TestSchedule_PreferModelLoaded(t *testing.T) {
	// 同为 IDLE，已加载目标模型的优先（省冷启动）
	s := NewScheduler()
	agents := []AgentView{
		makeAgent("cold-01", proto.YieldIDLE, []string{"other-model"}, 8192),
		makeAgent("hot-01", proto.YieldIDLE, []string{"target-model"}, 8192),
	}
	got, _ := s.Schedule(ScheduleRequest{Model: "target-model", MinBudget: 10}, agents)
	if got != "hot-01" {
		t.Errorf("应优先已加载模型的 hot-01，得到 %s", got)
	}
}

func TestSchedule_LeastConnections(t *testing.T) {
	// 同档同模型，活跃任务少的优先
	s := NewScheduler()
	s.AcquireSlot("loaded-01")
	s.AcquireSlot("loaded-01") // loaded-01 有 2 个在途
	agents := []AgentView{
		makeAgent("loaded-01", proto.YieldIDLE, []string{"m"}, 8192),
		makeAgent("free-01", proto.YieldIDLE, []string{"m"}, 8192),
	}
	got, _ := s.Schedule(ScheduleRequest{Model: "m", MinBudget: 10}, agents)
	if got != "free-01" {
		t.Errorf("最少连接应选 free-01，得到 %s", got)
	}
}

func TestAcquireReleaseSlot(t *testing.T) {
	s := NewScheduler()
	s.AcquireSlot("a1")
	s.AcquireSlot("a1")
	if c := s.ActiveCount("a1"); c != 2 {
		t.Errorf("ActiveCount 应为 2，得到 %d", c)
	}
	s.Release("a1")
	if c := s.ActiveCount("a1"); c != 1 {
		t.Errorf("Release 后应为 1，得到 %d", c)
	}
	s.Release("a1")
	if c := s.ActiveCount("a1"); c != 0 {
		t.Errorf("全释放后应为 0，得到 %d", c)
	}
	// 过度 Release 不应为负
	s.Release("a1")
	if c := s.ActiveCount("a1"); c != 0 {
		t.Errorf("过度 Release 应保持 0，得到 %d", c)
	}
}
