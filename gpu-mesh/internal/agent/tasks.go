package agent

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/QiuShichang/gpu-mesh/internal/proto"
)

// runningTasks 跟踪正在执行的任务，支持取消。
type runningTasks struct {
	mu   sync.Mutex
	jobs map[string]context.CancelFunc // taskID → cancel
}

func newRunningTasks() *runningTasks {
	return &runningTasks{jobs: make(map[string]context.CancelFunc)}
}

func (r *runningTasks) add(taskID string, cancel context.CancelFunc) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.jobs[taskID] = cancel
}

func (r *runningTasks) done(taskID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.jobs, taskID)
}

func (r *runningTasks) cancel(taskID string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if c, ok := r.jobs[taskID]; ok {
		c()
		return true
	}
	return false
}

// handleTaskRequest 处理 Relay 下发的任务。
//
// 让位协作（★Phase 3 核心机制）：本机算力配额不足以满足任务要求时，
// NACK 让 Relay 重调度到其他节点。
func (a *Agent) handleTaskRequest(ctx context.Context, conn *wsConn, env proto.Envelope) {
	var task proto.TaskRequest
	if err := env.Decode(&task); err != nil {
		log.Printf("[agent] 解析任务失败: %v", err)
		return
	}

	// ★ 让位协作：预算不足直接 NACK
	if a.yield.State().Budget < task.MinBudget {
		a.sendNack(ctx, conn, task.TaskID, "yield_budget_too_low")
		log.Printf("[agent] 让位 NACK task=%s budget=%d < min=%d",
			task.TaskID, a.yield.State().Budget, task.MinBudget)
		return
	}

	log.Printf("[agent] 收到任务 %s type=%s", task.TaskID, task.Type)

	// 异步执行（不阻塞读循环）
	go a.executeTask(ctx, conn, task)
}

// executeTask 执行任务并回流结果。
func (a *Agent) executeTask(parentCtx context.Context, conn *wsConn, task proto.TaskRequest) {
	timeout := time.Duration(task.Timeout) * time.Second
	if timeout == 0 {
		timeout = time.Duration(proto.DefaultTaskTimeout) * time.Second
	}
	ctx, cancel := context.WithTimeout(parentCtx, timeout)
	defer cancel()

	a.running.add(task.TaskID, cancel)
	defer a.running.done(task.TaskID)

	start := time.Now()
	var result proto.TaskResult
	switch task.Type {
	case proto.TaskDiag:
		result = a.execDiag(ctx, task)
	case proto.TaskInference:
		// 流式推理识别：Stream 字段为 true 时走流式执行器（能逐 token 发 progress）
		if isInferenceStream(task.Payload) {
			result = a.execInferenceStream(ctx, task, conn)
		} else {
			result = a.execInference(ctx, task)
		}
	case proto.TaskBatch:
		result = a.execBatch(ctx, task, conn)
	case proto.TaskTrain:
		result = a.execTrain(ctx, task, conn)
	case "pull":
		result = a.execPull(ctx, task)
	case "embed":
		result = a.execEmbed(ctx, task)
	default:
		result = proto.TaskResult{Success: false, Error: "未知任务类型: " + task.Type}
	}
	result.TaskID = task.TaskID
	result.Duration = int(time.Since(start).Milliseconds())

	// 回流结果
	respEnv, err := proto.NewEnvelope(proto.TypeTaskResult, a.cfg.AgentID, "relay", result)
	if err != nil {
		return
	}
	writeCtx, cancel2 := context.WithTimeout(parentCtx, 10*time.Second)
	defer cancel2()
	if err := conn.writeJSON(writeCtx, respEnv); err != nil {
		log.Printf("[agent] 回流结果失败 task=%s: %v", task.TaskID, err)
	}
}

// sendNack 发送任务拒绝（让位降级/资源不足触发，Relay 据此重调度）。
func (a *Agent) sendNack(ctx context.Context, conn *wsConn, taskID, reason string) {
	nack := proto.TaskNack{TaskID: taskID, Reason: reason}
	env, _ := proto.NewEnvelope(proto.TypeTaskNack, a.cfg.AgentID, "relay", nack)
	writeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	_ = conn.writeJSON(writeCtx, env)
}

// handleTaskCancel 取消正在执行的任务。
func (a *Agent) handleTaskCancel(ctx context.Context, conn *wsConn, env proto.Envelope) {
	var cancel proto.TaskCancel
	if err := env.Decode(&cancel); err != nil {
		return
	}
	if a.running.cancel(cancel.TaskID) {
		log.Printf("[agent] 已取消任务 %s", cancel.TaskID)
	}
}

// isInferenceStream 检查 inference 载荷是否要求流式。
func isInferenceStream(payload []byte) bool {
	var p struct{ Stream bool `json:"stream"` }
	if err := jsonUnmarshal(payload, &p); err != nil {
		return false
	}
	return p.Stream
}
