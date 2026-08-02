package relay

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/QiuShichang/gpu-mesh/internal/proto"
)

// === Phase 5：训练/微调编排器 ===
//
// 训练与推理/批量的关键区别：
//   - 资源独占：训练期间占满整卡，该 Agent 标记 RESERVED，不接其他任务
//   - 长时运行：可能数小时，需 checkpoint 管理
//   - 让位续训：遇 BUSY 暂停存档，回 IDLE 后断点续训
//
// 8GB 显存约束：只支持 LoRA/QLoRA 小模型（Qwen2.5-7B Q4 + LoRA）。

// TrainOrchestrator 训练编排器。
type TrainOrchestrator struct {
	mu     sync.RWMutex
	jobs   map[string]*runningTrain // jobID → 运行态
	relay  *Relay
	reserved map[string]bool // agentID → 是否被训练独占
}

// runningTrain 一个训练作业的运行态。
type runningTrain struct {
	task    proto.TrainTask
	status  proto.TrainStatus
	mu      sync.Mutex
}

// NewTrainOrchestrator 构造训练编排器。
func NewTrainOrchestrator(relay *Relay) *TrainOrchestrator {
	return &TrainOrchestrator{
		jobs:     make(map[string]*runningTrain),
		reserved: make(map[string]bool),
		relay:    relay,
	}
}

// isReserved 某 Agent 是否被训练独占。
func (o *TrainOrchestrator) isReserved(agentID string) bool {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.reserved[agentID]
}

// Submit 提交训练作业：资源独占调度 + 异步执行 + 断点续训循环。
func (o *TrainOrchestrator) Submit(task proto.TrainTask) (string, error) {
	if task.BaseModel == "" || task.Dataset == "" {
		return "", fmt.Errorf("base_model 和 dataset 不能为空")
	}
	if task.JobID == "" {
		task.JobID = "train-" + uuid.NewString()[:8]
	}
	rt := &runningTrain{
		task: task,
		status: proto.TrainStatus{
			JobID: task.JobID, Status: "queued", CreatedAt: time.Now().UnixMilli(),
		},
	}
	o.mu.Lock()
	o.jobs[task.JobID] = rt
	o.mu.Unlock()

	// 异步执行（含让位续训循环）
	go o.runWithResume(rt)
	log.Printf("[train] 提交 %s: base=%s framework=%s", task.JobID, task.BaseModel, task.Framework)
	return task.JobID, nil
}

// runWithResume 执行训练，遇让位暂停后自动续训。
func (o *TrainOrchestrator) runWithResume(rt *runningTrain) {
	const maxResumeAttempts = 50 // 防无限续训
	for attempt := 0; attempt < maxResumeAttempts; attempt++ {
		// 选 Agent（训练需 IDLE 整卡独占）
		agentID, err := o.relay.scheduler.Schedule(ScheduleRequest{
			MinBudget: 100, // 训练只在 IDLE 跑
		}, o.relay.agents.SnapshotExcluding(nil))
		if err != nil {
			rt.mu.Lock()
			rt.status.Status = "queued"
			rt.status.Message = "等待 IDLE 节点: " + err.Error()
			rt.status.UpdatedAt = time.Now().UnixMilli()
			rt.mu.Unlock()
			time.Sleep(30 * time.Second)
			continue
		}

		// 标记独占（本轮迭代结束统一释放，不用 defer——defer 在 for 里会延迟到函数返回）
		o.mu.Lock()
		o.reserved[agentID] = true
		o.mu.Unlock()
		releaseReserve := func() {
			o.mu.Lock()
			delete(o.reserved, agentID)
			o.mu.Unlock()
		}

		rt.mu.Lock()
		rt.status.Status = "running"
		rt.status.AgentID = agentID
		rt.status.UpdatedAt = time.Now().UnixMilli()
		rt.mu.Unlock()

		// 下发训练任务（长超时）
		result, err := o.relay.dispatchAndWait(agentID, proto.TaskTrain, rt.task, 24*time.Hour)
		if err != nil {
			releaseReserve()
			log.Printf("[train] %s 执行失败: %v", rt.task.JobID, err)
			rt.mu.Lock()
			rt.status.Status = "failed"
			rt.status.Message = err.Error()
			rt.mu.Unlock()
			o.scheduleCleanup(rt.task.JobID)
			return
		}
		// 解析结果
		var tr proto.TrainResult
		if result.Data != nil {
			json.Unmarshal(result.Data, &tr)
		}
		if tr.Paused {
			// 让位暂停 → 更新 resume_from → 续训
			releaseReserve()
			log.Printf("[train] %s 在 %s 暂停，准备续训 (attempt %d)", rt.task.JobID, agentID, attempt+1)
			rt.task.ResumeFrom = tr.CheckpointDir
			rt.mu.Lock()
			rt.status.Status = "paused"
			rt.status.Message = "让位暂停，等待 IDLE 续训"
			rt.mu.Unlock()
			time.Sleep(60 * time.Second) // 等机器空闲
			continue
		}
		// 训练完成
		releaseReserve()
		rt.mu.Lock()
		rt.status.Status = "completed"
		rt.status.Step = tr.Steps
		rt.status.Loss = tr.FinalLoss
		rt.status.UpdatedAt = time.Now().UnixMilli()
		rt.mu.Unlock()
		log.Printf("[train] %s 完成: steps=%d loss=%.4f", rt.task.JobID, tr.Steps, tr.FinalLoss)
		o.scheduleCleanup(rt.task.JobID)
		return
	}
	// 续训次数用尽
	rt.mu.Lock()
	rt.status.Status = "failed"
	rt.status.Message = "续训次数超限"
	rt.mu.Unlock()
	o.scheduleCleanup(rt.task.JobID)
}

// scheduleCleanup 延迟 1 小时清理已终态的训练作业（防 jobs map 无限增长）。
func (o *TrainOrchestrator) scheduleCleanup(jobID string) {
	time.AfterFunc(time.Hour, func() {
		o.mu.Lock()
		delete(o.jobs, jobID)
		o.mu.Unlock()
	})
}

// Status 查询训练进度。
func (o *TrainOrchestrator) Status(jobID string) (*proto.TrainStatus, bool) {
	o.mu.RLock()
	rt, ok := o.jobs[jobID]
	o.mu.RUnlock()
	if !ok {
		return nil, false
	}
	rt.mu.Lock()
	defer rt.mu.Unlock()
	s := rt.status
	return &s, true
}

// handleTrainSubmit POST /api/train —— 提交训练作业。
func (r *Relay) handleTrainSubmit(w http.ResponseWriter, req *http.Request) {
	if !r.authorize(req) {
		writeJSON(w, http.StatusUnauthorized, errMap("unauthorized"))
		return
	}
	var task proto.TrainTask
	if err := json.NewDecoder(req.Body).Decode(&task); err != nil {
		writeJSON(w, http.StatusBadRequest, errMap(err.Error()))
		return
	}
	jobID, err := r.train.Submit(task)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errMap(err.Error()))
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"job_id": jobID, "status": "queued"})
}

// handleTrainStatus GET /api/train/{id} —— 查询训练进度。
func (r *Relay) handleTrainStatus(w http.ResponseWriter, req *http.Request) {
	if !r.authorize(req) {
		writeJSON(w, http.StatusUnauthorized, errMap("unauthorized"))
		return
	}
	jobID := req.URL.Path[len("/api/train/"):]
	status, ok := r.train.Status(jobID)
	if !ok {
		writeJSON(w, http.StatusNotFound, errMap("训练作业不存在"))
		return
	}
	writeJSON(w, http.StatusOK, status)
}
