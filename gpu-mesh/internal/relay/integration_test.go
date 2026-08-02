package relay

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/QiuShichang/gpu-mesh/internal/proto"
)

// === 集成测试：多 Agent 协作链路 ===

// TestIntegration_MultiAgentOnline 多 Agent 上线后 /api/agents 能查到所有，调度选 IDLE。
func TestIntegration_MultiAgentOnline(t *testing.T) {
	c := newTestCluster(t)
	defer c.teardown()

	a1 := c.newMockAgent("idle-01", proto.YieldState{Level: proto.YieldIDLE, Budget: 100})
	a2 := c.newMockAgent("busy-01", proto.YieldState{Level: proto.YieldBUSY, Budget: 10})
	a3 := c.newMockAgent("active-01", proto.YieldState{Level: proto.YieldACTIVE, Budget: 50})

	waitFor(t, "3 个 Agent 上线", 2*time.Second, func() bool {
		return len(c.listAgents()) == 3
	})

	agents := c.listAgents()
	if len(agents) != 3 {
		t.Fatalf("期望 3 个 Agent，得到 %d", len(agents))
	}
	// 验证每个 Agent 的 yield 状态正确同步
	yieldMap := map[string]string{}
	for _, a := range agents {
		yieldMap[a.AgentID] = a.Yield.Level
	}
	if yieldMap["idle-01"] != proto.YieldIDLE {
		t.Errorf("idle-01 应为 idle，得到 %s", yieldMap["idle-01"])
	}
	if yieldMap["busy-01"] != proto.YieldBUSY {
		t.Errorf("busy-01 应为 busy_human，得到 %s", yieldMap["busy-01"])
	}
	_ = a1; _ = a2; _ = a3
}

// TestIntegration_SchedulePrefersIdle 推理请求应路由到 IDLE 节点。
func TestIntegration_SchedulePrefersIdle(t *testing.T) {
	c := newTestCluster(t)
	defer c.teardown()

	// busy 节点也设 MinBudget 兼容（budget=10 可接推理）
	busyAgent := c.newMockAgent("busy-node", proto.YieldState{Level: proto.YieldBUSY, Budget: 10})
	idleAgent := c.newMockAgent("idle-node", proto.YieldState{Level: proto.YieldIDLE, Budget: 100})
	// 让两个节点都能处理该模型
	busyAgent.models = []string{"m"}
	idleAgent.models = []string{"m"}
	// 重新注册更新 models
	busyAgent.engines = []string{"ollama"}
	idleAgent.engines = []string{"ollama"}
	busyAgent.sendRegister()
	idleAgent.sendRegister()
	time.Sleep(100 * time.Millisecond)

	// 发推理请求，应路由到 idle-node
	resp, err := c.chat("m", "test")
	if err != nil {
		t.Fatalf("推理失败: %v", err)
	}
	if resp.ID == "" || len(resp.Choices) == 0 {
		t.Fatalf("推理未成功: %+v", resp)
	}
	// idle-node 应收到任务，busy-node 不应收到
	waitFor(t, "idle-node 收到任务", 1*time.Second, func() bool {
		return idleAgent.taskCount() == 1
	})
	if busyAgent.taskCount() != 0 {
		t.Errorf("busy-node 不应收到任务，收到 %d 个", busyAgent.taskCount())
	}
	if idleAgent.taskCount() != 1 {
		t.Errorf("idle-node 应收到 1 个任务，收到 %d 个", idleAgent.taskCount())
	}
}

// TestIntegration_TaskRoundTrip 任务下发→执行→结果回流完整闭环。
func TestIntegration_TaskRoundTrip(t *testing.T) {
	c := newTestCluster(t)
	defer c.teardown()

	agent := c.newMockAgent("roundtrip-node", proto.YieldState{Level: proto.YieldIDLE, Budget: 100})
	// 节点模型设为 "test-model"（与默认一致），请求也用 "test-model"
	agent.setTaskHandler(func(task proto.TaskRequest) proto.TaskResult {
		return proto.TaskResult{
			TaskID:  task.TaskID,
			Success: true,
			Data:    proto.MarshalData(proto.InferenceResult{Content: "hello from agent", Model: "test-model"}),
		}
	})

	resp, err := c.chat("test-model", "hi")
	if err != nil {
		t.Fatalf("推理失败: %v", err)
	}
	if len(resp.Choices) == 0 {
		t.Fatalf("无 choices: %+v", resp)
	}
	if resp.Choices[0].Message.Content != "hello from agent" {
		t.Errorf("期望 'hello from agent'，得到 %q", resp.Choices[0].Message.Content)
	}
}

// TestIntegration_YieldNackReschedule ★核心：让位 NACK 触发重调度到其他节点。
//
// 场景：任务先派到 busy-agent（MinBudget=10 可接），但 busy-agent 的 handler 返回 NACK，
// Relay 应自动重选 idle-agent 重投。
func TestIntegration_YieldNackReschedule(t *testing.T) {
	c := newTestCluster(t)
	defer c.teardown()

	// 第一个 agent：收到任务就 NACK（模拟让位）
	nackAgent := c.newMockAgent("nack-node", proto.YieldState{Level: proto.YieldIDLE, Budget: 100})
	nackAgent.setTaskHandler(func(task proto.TaskRequest) proto.TaskResult {
		// 不在这里 NACK——NACK 是 Agent 主动发的，这里通过 sendNack 模拟
		// 实际 Agent 在 handleTaskRequest 里发现 budget<MinBudget 才 NACK
		// 这里直接返回失败带 yield_budget 标记，触发 gateway 的 isYieldNack 重调度
		return proto.TaskResult{TaskID: task.TaskID, Success: false, Error: "agent 拒绝执行: yield_budget_too_low"}
	})

	// 第二个 agent：正常处理
	okAgent := c.newMockAgent("ok-node", proto.YieldState{Level: proto.YieldIDLE, Budget: 100})
	okAgent.setTaskHandler(func(task proto.TaskRequest) proto.TaskResult {
		return proto.TaskResult{
			TaskID:  task.TaskID, Success: true,
			Data: proto.MarshalData(proto.InferenceResult{Content: "from ok-node", Model: "m"}),
		}
	})

	// 两个节点都能处理该模型
	nackAgent.models = []string{"m"}
	okAgent.models = []string{"m"}
	nackAgent.sendRegister()
	okAgent.sendRegister()
	time.Sleep(100 * time.Millisecond)

	// 发推理，nack-node 先收到返回失败 → 重调度到 ok-node
	resp, err := c.chat("m", "test")
	if err != nil {
		t.Fatalf("推理失败: %v", err)
	}
	// 应最终成功（重调度后 ok-node 处理）
	waitFor(t, "重调度到 ok-node", 2*time.Second, func() bool {
		return okAgent.taskCount() > 0
	})

	if okAgent.taskCount() == 0 {
		t.Error("让位重调度失败：ok-node 未收到任务")
	}
	// 最终响应应来自 ok-node
	if len(resp.Choices) > 0 && resp.Choices[0].Message.Content == "from ok-node" {
		// 成功
	} else if len(resp.Choices) == 0 {
		t.Error("让位重调度后推理仍失败")
	}
}

// TestIntegration_BatchMapReduce 批量作业分片并行分发到多节点。
func TestIntegration_BatchMapReduce(t *testing.T) {
	c := newTestCluster(t)
	defer c.teardown()

	// 2 个 IDLE 节点
	agent1 := c.newMockAgent("batch-node-1", proto.YieldState{Level: proto.YieldIDLE, Budget: 100})
	agent2 := c.newMockAgent("batch-node-2", proto.YieldState{Level: proto.YieldIDLE, Budget: 100})
	agent1.models = []string{"m"}
	agent2.models = []string{"m"}
	agent1.setTaskHandler(func(task proto.TaskRequest) proto.TaskResult {
		return batchOKResult(task)
	})
	agent2.setTaskHandler(func(task proto.TaskRequest) proto.TaskResult {
		return batchOKResult(task)
	})
	agent1.sendRegister()
	agent2.sendRegister()
	time.Sleep(100 * time.Millisecond)

	// 提交 6 项，分片大小 2 → 3 个分片
	data, err := c.apiPostJSON("/api/batches", proto.BatchSpec{
		Model: "m", TaskType: "chat", ShardSize: 2, MinBudget: 100,
		Items: []string{"q1", "q2", "q3", "q4", "q5", "q6"},
	})
	if err != nil {
		t.Fatalf("提交批量失败: %v", err)
	}
	var submitResp struct{ BatchID string `json:"batch_id"` }
	json.Unmarshal(data, &submitResp)
	if submitResp.BatchID == "" {
		t.Fatal("未返回 batch_id")
	}

	// 轮询进度，等待完成
	waitFor(t, "批量完成", 15*time.Second, func() bool {
		status := c.batchStatus(submitResp.BatchID)
		return status != nil && (status.Status == "completed" || status.Status == "partial")
	})

	status := c.batchStatus(submitResp.BatchID)
	if status == nil {
		t.Fatal("查询批量状态失败")
	}
	if status.Completed != 3 {
		t.Errorf("期望 3 分片完成，得到 %d", status.Completed)
	}
	if len(status.Results) != 6 {
		t.Errorf("期望 6 条结果，得到 %d", len(status.Results))
	}
	// 验证两节点都有参与（并行分发）
	if agent1.taskCount() == 0 || agent2.taskCount() == 0 {
		t.Error("批量应分发到多个节点，但有节点未参与")
	}
}

// TestIntegration_AgentOfflineStatus Agent 离线后不再出现在列表。
func TestIntegration_AgentOfflineStatus(t *testing.T) {
	c := newTestCluster(t)
	defer c.teardown()

	agent := c.newMockAgent("ephemeral-node", proto.YieldState{Level: proto.YieldIDLE, Budget: 100})
	waitFor(t, "Agent 上线", 1*time.Second, func() bool { return len(c.listAgents()) == 1 })

	// 关闭 Agent 连接
	agent.close()
	// 等待 Relay 检测到断开（serveAgent 的 Read 返回错误 → Unregister）
	waitFor(t, "Agent 离线清理", 2*time.Second, func() bool { return len(c.listAgents()) == 0 })

	if len(c.listAgents()) != 0 {
		t.Errorf("Agent 离线后仍在线：%d 个", len(c.listAgents()))
	}
}

// TestIntegration_YieldChangeBroadcast 让位状态变化在 API 可见（心跳同步验证）。
func TestIntegration_YieldChangeBroadcast(t *testing.T) {
	c := newTestCluster(t)
	defer c.teardown()

	agent := c.newMockAgent("yield-node", proto.YieldState{Level: proto.YieldIDLE, Budget: 100})
	waitFor(t, "上线", 1*time.Second, func() bool { return len(c.listAgents()) == 1 })

	// 模拟用户开始使用 → yield 变 BUSY
	agent.setYield(proto.YieldBUSY, 10)

	// API 应反映新状态
	waitFor(t, "yield 变化可见", 1*time.Second, func() bool {
		for _, a := range c.listAgents() {
			if a.AgentID == "yield-node" && a.Yield.Level == proto.YieldBUSY {
				return true
			}
		}
		return false
	})
}

// batchStatus 查询批量状态。
func (c *testCluster) batchStatus(batchID string) *proto.BatchStatus {
	data, err := c.apiGet("/api/batches/" + batchID)
	if err != nil {
		return nil
	}
	var status proto.BatchStatus
	json.Unmarshal(data, &status)
	if status.BatchID == "" {
		return nil
	}
	return &status
}

// batchOKResult mock 批量分片的成功结果。
func batchOKResult(task proto.TaskRequest) proto.TaskResult {
	var bt proto.BatchTask
	json.Unmarshal(task.Payload, &bt)
	results := make([]string, len(bt.Items))
	for i, item := range bt.Items {
		results[i] = "answer:" + item
	}
	sr := proto.BatchShardResult{
		BatchID: bt.BatchID, ShardID: bt.ShardID,
		Results: results, Succeeded: len(results),
	}
	return proto.TaskResult{TaskID: task.TaskID, Success: true, Data: proto.MarshalData(sr)}
}

// 确保 strings 被使用（避免未用 import）
var _ = strings.Contains

// TestIntegration_RecoverInFlightTasks ★ 重启恢复：dispatched 任务重启后转 pending，
// Agent 重连后补投。
//
// 验证 at-least-once 语义：Relay 崩溃重启后，未完成任务不丢失。
func TestIntegration_RecoverInFlightTasks(t *testing.T) {
	dir := t.TempDir()
	dbPath := dir + "/test.db"

	// 第一次：建 Relay，存一个 dispatched 任务，模拟崩溃
	r1, err := New(Config{Store: true, StorePath: dbPath})
	if err != nil {
		t.Fatalf("第一次 New 失败: %v", err)
	}
	r1.store.SaveTask(StoredTask{
		Request:   proto.TaskRequest{TaskID: "recover-test-1", AgentID: "rec-agent", Type: proto.TaskDiag, Payload: proto.MarshalData(proto.DiagTask{Command: "echo ok"})},
		Status:    StatusDispatched,
		Attempt:   1,
		CreatedAt: time.Now().UnixMilli(),
	})
	r1.Close()

	// 模拟"崩溃后重启"：重新打开同一个 db
	r2, err := New(Config{Store: true, StorePath: dbPath})
	if err != nil {
		t.Fatalf("重启 New 失败: %v", err)
	}
	defer r2.Close()

	// 验证任务已被转为 pending
	pending, _ := r2.store.ListTasksByStatus(StatusPending)
	found := false
	for _, p := range pending {
		if p.Request.TaskID == "recover-test-1" {
			found = true
			if p.Attempt != 2 {
				t.Errorf("Attempt 应递增到 2，得到 %d", p.Attempt)
			}
		}
	}
	if !found {
		t.Error("重启后 dispatched 任务应转为 pending，但未找到")
	}
}

