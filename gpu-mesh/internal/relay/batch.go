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

// === Phase 4：批量离线推理 Map-Reduce 编排器 ===
//
// 提交一个 BatchSpec → 编排器：
//  1. Map：把全量 items 按 shardSize 切成多个分片
//  2. 每个分片经 GPU 感知调度分发到一个 IDLE/ACTIVE Agent（异步并行）
//  3. Reduce：收集所有分片结果，按顺序聚合（保持 items 对应关系）
//  4. 失败分片重试 N 次
//  5. 全部完成后状态转 completed，结果可查询/下载

// BatchOrchestrator 批量作业编排器。
type BatchOrchestrator struct {
	mu      sync.RWMutex
	jobs    map[string]*runningBatch // batchID → 运行态
	relay   *Relay
}

// runningBatch 一个批量作业的运行态。
type runningBatch struct {
	spec    proto.BatchSpec
	status  proto.BatchStatus
	shards  map[string]*shardState // shardID → 状态
	mu      sync.Mutex
}

// shardState 一个分片的运行态。
type shardState struct {
	id        string
	items     []string
	agentID   string
	attempt   int
	result    *proto.BatchShardResult
	failed    bool
}

// NewBatchOrchestrator 构造编排器。
func NewBatchOrchestrator(relay *Relay) *BatchOrchestrator {
	return &BatchOrchestrator{
		jobs:  make(map[string]*runningBatch),
		relay: relay,
	}
}

// Submit 提交批量作业：分片 + 异步分发 + 返回 batchID 供轮询。
func (o *BatchOrchestrator) Submit(spec proto.BatchSpec) (string, error) {
	if spec.Model == "" || len(spec.Items) == 0 {
		return "", fmt.Errorf("model 和 items 不能为空")
	}
	if spec.TaskType == "" {
		spec.TaskType = "chat"
	}
	if spec.ShardSize <= 0 {
		spec.ShardSize = 20
	}
	if spec.MinBudget <= 0 {
		// 批量任务默认只在 IDLE 节点跑（让位友好，不抢用户资源）
		spec.MinBudget = 100
	}
	if spec.BatchID == "" {
		spec.BatchID = "batch-" + uuid.NewString()[:8]
	}

	// 切片
	shards := shardItems(spec.Items, spec.ShardSize)
	rb := &runningBatch{
		spec:   spec,
		shards: make(map[string]*shardState, len(shards)),
		status: proto.BatchStatus{
			BatchID:    spec.BatchID,
			Total:      len(shards),
			Pending:    len(shards),
			TotalItems: len(spec.Items),
			Status:     "running",
			CreatedAt:  time.Now().UnixMilli(),
		},
	}
	for i, items := range shards {
		id := fmt.Sprintf("%s-shard-%d", spec.BatchID, i)
		rb.shards[id] = &shardState{id: id, items: items}
	}

	o.mu.Lock()
	o.jobs[spec.BatchID] = rb
	o.mu.Unlock()

	// 异步分发所有分片（并行 Map）
	go o.mapPhase(rb)
	incBatchJob()

	log.Printf("[batch] 提交 %s: %d 项 → %d 分片 (task=%s, shard=%d)",
		spec.BatchID, len(spec.Items), len(shards), spec.TaskType, spec.ShardSize)
	return spec.BatchID, nil
}

// mapPhase 并行分发所有分片到 Agent。
func (o *BatchOrchestrator) mapPhase(rb *runningBatch) {
	var wg sync.WaitGroup
	for _, sh := range rb.shards {
		wg.Add(1)
		go func(sh *shardState) {
			defer wg.Done()
			o.dispatchShard(rb, sh)
		}(sh)
	}
	wg.Wait()
	o.reducePhase(rb)
}

// dispatchShard 分发单个分片到选中的 Agent（带重试）。
func (o *BatchOrchestrator) dispatchShard(rb *runningBatch, sh *shardState) {
	const maxAttempts = 3
	for sh.attempt < maxAttempts {
		sh.attempt++
		// 选 Agent（批量任务 MinBudget=100，只在 IDLE 跑）
		agentID, err := o.relay.scheduler.Schedule(ScheduleRequest{
			Model:     rb.spec.Model,
			Engine:    rb.spec.Engine,
			MinBudget: rb.spec.MinBudget,
		}, o.relay.agents.SnapshotExcluding(nil))
		if err != nil {
			// 暂无 IDLE 节点，等待重试
			time.Sleep(5 * time.Second)
			continue
		}
		sh.agentID = agentID

		// 更新状态：processing
		rb.mu.Lock()
		rb.status.Pending--
		rb.status.Processing++
		rb.mu.Unlock()

		// 下发分片
		batchTask := proto.BatchTask{
			BatchID:    rb.spec.BatchID,
			ShardID:    sh.id,
			Engine:     rb.spec.Engine,
			Model:      rb.spec.Model,
			TaskType:   rb.spec.TaskType,
			Items:      sh.items,
			MaxTokens:  rb.spec.MaxTokens,
		}
		result, err := o.relay.dispatchAndWait(agentID, proto.TaskBatch, batchTask, 30*time.Minute)
		if err != nil {
			log.Printf("[batch] 分片 %s 失败 (attempt %d): %v", sh.id, sh.attempt, err)
			rb.mu.Lock()
			rb.status.Processing--
			rb.status.Pending++ // 回到待分发
			rb.mu.Unlock()
			continue
		}
		if !result.Success && result.Data == nil {
			log.Printf("[batch] 分片 %s 失败 (attempt %d): %s", sh.id, sh.attempt, result.Error)
			rb.mu.Lock()
			rb.status.Processing--
			rb.status.Pending++
			rb.mu.Unlock()
			continue
		}
		// 成功
		var sr proto.BatchShardResult
		if err := json.Unmarshal(result.Data, &sr); err == nil {
			sh.result = &sr
		}
		rb.mu.Lock()
		rb.status.Processing--
		rb.status.Completed++
		rb.status.DoneItems += len(sh.items)
		rb.mu.Unlock()
		return
	}
	// 重试用尽
	sh.failed = true
	rb.mu.Lock()
	rb.status.Processing--
	rb.status.Failed++
	rb.mu.Unlock()
	log.Printf("[batch] 分片 %s 重试 %d 次仍失败", sh.id, maxAttempts)
}

// reducePhase 聚合所有分片结果（按分片顺序保持 items 对齐）。
func (o *BatchOrchestrator) reducePhase(rb *runningBatch) {
	rb.mu.Lock()

	// 按分片顺序（创建顺序）聚合
	if rb.spec.TaskType == "chat" {
		results := []string{}
		for i := 0; i < rb.status.Total; i++ {
			id := fmt.Sprintf("%s-shard-%d", rb.spec.BatchID, i)
			sh := rb.shards[id]
			if sh != nil && sh.result != nil {
				results = append(results, sh.result.Results...)
			}
		}
		rb.status.Results = results
	} else { // embed
		embeddings := [][]float32{}
		for i := 0; i < rb.status.Total; i++ {
			id := fmt.Sprintf("%s-shard-%d", rb.spec.BatchID, i)
			sh := rb.shards[id]
			if sh != nil && sh.result != nil {
				embeddings = append(embeddings, sh.result.Embeddings...)
			}
		}
		rb.status.Embeddings = embeddings
	}

	rb.status.Status = "completed"
	if rb.status.Failed > 0 && rb.status.Completed == 0 {
		rb.status.Status = "failed"
	} else if rb.status.Failed > 0 {
		rb.status.Status = "partial"
	}
	rb.status.FinishedAt = time.Now().UnixMilli()
	batchID := rb.spec.BatchID
	log.Printf("[batch] %s 完成: %d 分片成功 / %d 失败 (%.1fs)",
		batchID, rb.status.Completed, rb.status.Failed,
		float64(rb.status.FinishedAt-rb.status.CreatedAt)/1000)
	rb.mu.Unlock()

	// ★ 内存保护：完成后延迟 1 小时清理 jobs（给客户端留查询窗口，防 map 无限增长）
	time.AfterFunc(time.Hour, func() {
		o.mu.Lock()
		delete(o.jobs, batchID)
		o.mu.Unlock()
	})
}

// Status 查询批量作业进度。
func (o *BatchOrchestrator) Status(batchID string) (*proto.BatchStatus, bool) {
	o.mu.RLock()
	rb, ok := o.jobs[batchID]
	o.mu.RUnlock()
	if !ok {
		return nil, false
	}
	rb.mu.Lock()
	defer rb.mu.Unlock()
	status := rb.status // 值拷贝
	return &status, true
}

// shardItems 把 items 按 size 切成多个分片。
func shardItems(items []string, size int) [][]string {
	if size <= 0 {
		size = 20
	}
	out := [][]string{}
	for i := 0; i < len(items); i += size {
		end := i + size
		if end > len(items) {
			end = len(items)
		}
		out = append(out, items[i:end])
	}
	return out
}

// handleBatchSubmit POST /api/batches —— 提交批量作业。
func (r *Relay) handleBatchSubmit(w http.ResponseWriter, req *http.Request) {
	if !r.authorize(req) {
		writeJSON(w, http.StatusUnauthorized, errMap("unauthorized"))
		return
	}
	var spec proto.BatchSpec
	if err := json.NewDecoder(req.Body).Decode(&spec); err != nil {
		writeJSON(w, http.StatusBadRequest, errMap(err.Error()))
		return
	}
	batchID, err := r.batch.Submit(spec)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errMap(err.Error()))
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{
		"batch_id": batchID, "status": "submitted",
	})
}

// handleBatchStatus GET /api/batches/{id} —— 查询批量进度。
func (r *Relay) handleBatchStatus(w http.ResponseWriter, req *http.Request) {
	if !r.authorize(req) {
		writeJSON(w, http.StatusUnauthorized, errMap("unauthorized"))
		return
	}
	batchID := req.URL.Path[len("/api/batches/"):]
	if batchID == "" {
		writeJSON(w, http.StatusBadRequest, errMap("缺少 batch_id"))
		return
	}
	status, ok := r.batch.Status(batchID)
	if !ok {
		writeJSON(w, http.StatusNotFound, errMap("批量作业不存在"))
		return
	}
	writeJSON(w, http.StatusOK, status)
}
