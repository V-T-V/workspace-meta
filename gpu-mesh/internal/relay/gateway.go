package relay

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/QiuShichang/gpu-mesh/internal/proto"
)

// === Phase 2：OpenAI 兼容推理网关 + 轮询负载均衡 ===

// pendingResults 等待 Agent 回流的任务结果（taskID → chan）+ 流式 progress 回调。
//
// 网关同步等待语义：HTTP 请求阻塞等 Agent 把结果推回来。
// 流式推理时，progress（token delta）实时经 onProgress 回调推送。
//
// ★ 内存保护：每个 entry 带 TTL，注册时启动看门狗 goroutine，
// 到期未 deliver 则自动清理（防 Agent 崩溃后 entry 永驻内存）。
type pendingResults struct {
	mu sync.Mutex
	m  map[string]*pendingEntry
}

// pendingEntry 一个等待条目：结果 channel + 可选的 progress 回调 + 注册时间。
type pendingEntry struct {
	result     chan proto.TaskResult
	onProgress func(proto.TaskProgress) // 流式推理用，nil 则忽略
	registeredAt time.Time
}

func newPendingResults() *pendingResults {
	return &pendingResults{m: make(map[string]*pendingEntry)}
}

// register 注册一个等待，返回结果 channel。
// 同时启动看门狗：ttl 后若仍未 deliver，自动清理 entry（防内存泄漏）。
func (p *pendingResults) register(taskID string) <-chan proto.TaskResult {
	return p.registerTTL(taskID, nil, 5*time.Minute)
}

// registerWithProgress 注册带流式回调的等待。
func (p *pendingResults) registerWithProgress(taskID string, onProgress func(proto.TaskProgress)) <-chan proto.TaskResult {
	return p.registerTTL(taskID, onProgress, 5*time.Minute)
}

// registerTTL 内部注册实现，带 TTL 看门狗。
func (p *pendingResults) registerTTL(taskID string, onProgress func(proto.TaskProgress), ttl time.Duration) <-chan proto.TaskResult {
	ch := make(chan proto.TaskResult, 1)
	p.mu.Lock()
	p.m[taskID] = &pendingEntry{result: ch, onProgress: onProgress, registeredAt: time.Now()}
	p.mu.Unlock()
	// 看门狗：ttl 后清理（若仍存在）
	go func() {
		timer := time.NewTimer(ttl)
		defer timer.Stop()
		<-timer.C
		p.mu.Lock()
		_, ok := p.m[taskID]
		if ok {
			delete(p.m, taskID)
		}
		p.mu.Unlock()
		if ok {
			log.Printf("[pending] 任务 %s 等待超时未回流，清理 entry（防泄漏）", taskID)
		}
	}()
	return ch
}

// deliver 投递结果（来自 serveAgent 的 task_result 消息）。
func (p *pendingResults) deliver(taskID string, result proto.TaskResult) bool {
	p.mu.Lock()
	e, ok := p.m[taskID]
	if ok {
		delete(p.m, taskID)
	}
	p.mu.Unlock()
	if !ok {
		return false
	}
	e.result <- result
	return true
}

// deliverProgress 投递流式进度（来自 serveAgent 的 task_progress 消息）。
func (p *pendingResults) deliverProgress(taskID string, pg proto.TaskProgress) bool {
	p.mu.Lock()
	e, ok := p.m[taskID]
	p.mu.Unlock()
	if !ok || e.onProgress == nil {
		return false
	}
	e.onProgress(pg)
	return true
}

// （Phase 2 的轮询计数器已随 Phase 3 调度器升级移除，见 scheduler.go）

// dispatchAndWait 下发任务给 Agent 并等待结果（同步语义，供网关 HTTP handler 用）。
// 调用 scheduler 释放槽位（任务完成/失败/超时后）。
//
// 让位协作（★Phase 3）：若首个 Agent NACK（yield_budget_too_low），
// 自动重选其他 IDLE/ACTIVE 节点重投一次（最多 retry 次）。
func (r *Relay) dispatchAndWait(agentID, taskType string, payload any, timeout time.Duration) (proto.TaskResult, error) {
	return r.dispatchWithRetry(agentID, taskType, payload, timeout, 1, nil)
}

// dispatchWithRetry 带重试的下发。tried 记录已试过的 Agent（避免重复选到同一个）。
func (r *Relay) dispatchWithRetry(agentID, taskType string, payload any, timeout time.Duration, retriesLeft int, tried []string) (proto.TaskResult, error) {
	conn := r.agents.Conn(agentID)
	if conn == nil {
		r.scheduler.Release(agentID)
		return proto.TaskResult{}, fmt.Errorf("agent %s 不在线", agentID)
	}
	taskID := "gw-" + uuid.NewString()[:8]
	payloadBytes, _ := json.Marshal(payload)
	task := proto.TaskRequest{
		TaskID:  taskID,
		Type:    taskType,
		AgentID: agentID,
		Payload: payloadBytes,
		Timeout: int(timeout.Seconds()),
	}
	ch := r.pending.register(taskID)

	// 下发
	env, _ := proto.NewEnvelope(proto.TypeTaskRequest, "relay", agentID, task)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	if err := conn.writeJSON(ctx, env); err != nil {
		cancel()
		r.scheduler.Release(agentID)
		r.pending.mu.Lock()
		delete(r.pending.m, taskID)
		r.pending.mu.Unlock()
		return proto.TaskResult{}, fmt.Errorf("下发失败: %w", err)
	}
	cancel()
	log.Printf("[gateway] 下发 %s task=%s → agent=%s (retriesLeft=%d)", taskType, taskID, agentID, retriesLeft)

	// 等结果
	var result proto.TaskResult
	var err error
	select {
	case result = <-ch:
		err = nil
	case <-time.After(timeout):
		err = fmt.Errorf("任务超时 (%v)", timeout)
	}

	// 清理槽位
	r.scheduler.Release(agentID)
	r.pending.mu.Lock()
	delete(r.pending.m, taskID)
	r.pending.mu.Unlock()

	if err != nil {
		return proto.TaskResult{}, err
	}

	// ★ 让位重调度：NACK（预算不足）则换其他 IDLE 节点重试
	if !result.Success && retriesLeft > 0 && isYieldNack(result.Error) {
		incYieldNack()
		tried = append(tried, agentID)
		log.Printf("[gateway] Agent %s 让位，重调度 task=%s（已试 %v）", agentID, taskID, tried)
		nextID, schedErr := r.scheduler.Schedule(ScheduleRequest{
			Model:     extractModel(payloadBytes),
			MinBudget: 10,
		}, r.agents.SnapshotExcluding(tried))
		if schedErr != nil {
			return result, nil // 没有其他候选，返回原失败
		}
		return r.dispatchWithRetry(nextID, taskType, payload, timeout, retriesLeft-1, tried)
	}
	return result, nil
}

// isYieldNack 判断错误是否为让位 NACK（可重调度）。
func isYieldNack(errMsg string) bool {
	return strings.Contains(errMsg, "yield_budget") || strings.Contains(errMsg, "拒绝执行")
}

// extractModel 从任务 payload 提取 model 字段（用于重调度时选候选）。
func extractModel(payload []byte) string {
	var p struct{ Model string `json:"model"` }
	json.Unmarshal(payload, &p)
	return p.Model
}

// extractAPIKey 从请求提取 API Key（Authorization: Bearer sk-xxx 或 ?api_key=）。
func extractAPIKey(req *http.Request) string {
	auth := req.Header.Get("Authorization")
	if len(auth) > 7 && auth[:7] == "Bearer " {
		return auth[7:]
	}
	if k := req.URL.Query().Get("api_key"); k != "" {
		return k
	}
	return ""
}

// handleChatCompletions POST /v1/chat/completions —— OpenAI 兼容推理网关。
func (r *Relay) handleChatCompletions(w http.ResponseWriter, req *http.Request) {
	if !r.authorize(req) {
		writeJSON(w, http.StatusUnauthorized, errMap("unauthorized"))
		return
	}
	// Phase 6 多租户配额检查（API Key 在 Authorization: Bearer）
	apiKey := extractAPIKey(req)
	if blocked, _ := r.checkTenantQuota(apiKey); blocked {
		writeJSON(w, http.StatusTooManyRequests, openAIError("quota_exceeded", "超过配额限制"))
		return
	}
	// 限制请求体大小（防超大 body 攻击，10MB）
	req.Body = http.MaxBytesReader(w, req.Body, 10<<20)
	var oaiReq proto.OpenAIChatRequest
	if err := json.NewDecoder(req.Body).Decode(&oaiReq); err != nil {
		writeJSON(w, http.StatusBadRequest, openAIError("invalid_request_error", "请求体解析失败: "+err.Error()))
		return
	}
	// 流式分支：SSE 透传 token
	if oaiReq.Stream {
		r.handleChatStream(w, req, oaiReq, apiKey)
		return
	}
	if oaiReq.Model == "" || len(oaiReq.Messages) == 0 {
		writeJSON(w, http.StatusBadRequest, openAIError("invalid_request_error", "model 和 messages 不能为空"))
		return
	}

	// 选 Agent（Phase 3 GPU 感知调度）
	agentID, err := r.scheduler.Schedule(ScheduleRequest{
		Model:     oaiReq.Model,
		Engine:    oaiReq.Engine,
		AgentID:   oaiReq.AgentID,
		MinBudget: 10, // 推理任务：IDLE/ACTIVE/BUSY 都可，但优先 IDLE
	}, r.agents.Snapshot())
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, openAIError("no_agent", err.Error()))
		return
	}

	// 构造 inference 任务
	infTask := proto.InferenceTask{
		Engine:    oaiReq.Engine,
		Model:     oaiReq.Model,
		Messages:  oaiReq.Messages,
		Stream:    false,
		MaxTokens: oaiReq.MaxTokens,
	}
	if oaiReq.Temperature > 0 {
		infTask.Options = &proto.GenOptions{Temperature: oaiReq.Temperature}
	}

	// 下发并等待（推理给足超时）
	infStart := time.Now()
	result, err := r.dispatchAndWait(agentID, proto.TaskInference, infTask, 180*time.Second)
	incInference(err == nil && result.Success, time.Since(infStart).Milliseconds())
	if err != nil {
		writeJSON(w, http.StatusGatewayTimeout, openAIError("upstream_error", err.Error()))
		return
	}
	if !result.Success {
		writeJSON(w, http.StatusBadGateway, openAIError("upstream_error", result.Error))
		return
	}

	// 解析 Agent 返回的推理结果，转 OpenAI 格式
	var inf proto.InferenceResult
	if err := json.Unmarshal(result.Data, &inf); err != nil {
		writeJSON(w, http.StatusBadGateway, openAIError("upstream_error", "解析推理结果失败"))
		return
	}
	resp := proto.OpenAIChatResponse{
		ID:      "chatcmpl-" + uuid.NewString()[:8],
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   inf.Model,
		Choices: []proto.OpenAIChoice{{
			Index:        0,
			Message:      proto.ChatMessage{Role: "assistant", Content: inf.Content},
			FinishReason: fallbackStr(inf.DoneReason, "stop"),
		}},
	}
	if inf.PromptTokens > 0 || inf.CompletionTokens > 0 {
		resp.Usage = &proto.OpenAIUsage{
			PromptTokens:     inf.PromptTokens,
			CompletionTokens: inf.CompletionTokens,
			TotalTokens:      inf.PromptTokens + inf.CompletionTokens,
		}
	}
	// 扣减租户 token 配额
	if apiKey != "" {
		if t := r.tenants.Get(apiKey); t != nil {
			t.ConsumeTokens(resp.Usage.TotalTokens)
		}
	}
	writeJSON(w, http.StatusOK, resp)
}

// handleChatStream 流式推理：下发 stream 任务，把 Agent 回传的 token delta
// 实时转成 OpenAI SSE `data:` 帧推给客户端。
//
// ★ 并发安全：progress 回调在 serveAgent 的 Read goroutine 里触发，
// 同时本 handler goroutine 也会在结果回来后写 done 帧。
// 两者都写同一个 http.ResponseWriter，必须用 sseMu 串行化，否则 data race + 帧字节交错。
func (r *Relay) handleChatStream(w http.ResponseWriter, req *http.Request, oaiReq proto.OpenAIChatRequest, apiKey string) {
	// 选 Agent
	agentID, err := r.scheduler.Schedule(ScheduleRequest{
		Model: oaiReq.Model, Engine: oaiReq.Engine, AgentID: oaiReq.AgentID, MinBudget: 10,
	}, r.agents.Snapshot())
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, openAIError("no_agent", err.Error()))
		return
	}

	// SSE 头
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeJSON(w, http.StatusInternalServerError, openAIError("server_error", "streaming unsupported"))
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	completionID := "chatcmpl-" + uuid.NewString()[:8]
	created := time.Now().Unix()

	// ★ SSE 写锁：保护 w.Write/Flush 不被 progress 回调与 done 帧并发写
	var sseMu sync.Mutex
	writeSSE := func(data []byte) {
		sseMu.Lock()
		defer sseMu.Unlock()
		w.Write(data)
		flusher.Flush()
	}

	// 下发流式任务 + 注册 progress 回调
	infTask := proto.InferenceTask{
		Engine: oaiReq.Engine, Model: oaiReq.Model, Messages: oaiReq.Messages,
		Stream: true, MaxTokens: oaiReq.MaxTokens,
	}
	if oaiReq.Temperature > 0 {
		infTask.Options = &proto.GenOptions{Temperature: oaiReq.Temperature}
	}
	taskID := "gws-" + uuid.NewString()[:8]
	payloadBytes, _ := jsonMarshal(infTask)
	task := proto.TaskRequest{TaskID: taskID, Type: proto.TaskInference, AgentID: agentID, Payload: payloadBytes, Timeout: 180}

	// progress 回调：转 SSE delta 帧（经 sseMu 串行化）
	ch := r.pending.registerWithProgress(taskID, func(pg proto.TaskProgress) {
		if pg.Step == "delta" && pg.Message != "" {
			chunk := map[string]any{
				"id": completionID, "object": "chat.completion.chunk", "created": created, "model": oaiReq.Model,
				"choices": []map[string]any{{
					"index": 0, "finish_reason": nil,
					"delta": map[string]any{"content": pg.Message},
				}},
			}
			data, _ := jsonMarshal(chunk)
			writeSSE([]byte("data: " + string(data) + "\n\n"))
		}
	})
	defer func() {
		r.scheduler.Release(agentID)
		r.pending.mu.Lock()
		delete(r.pending.m, taskID)
		r.pending.mu.Unlock()
	}()

	// 下发
	conn := r.agents.Conn(agentID)
	if conn == nil {
		writeSSE([]byte("data: [DONE]\n\n"))
		return
	}
	env, _ := proto.NewEnvelope(proto.TypeTaskRequest, "relay", agentID, task)
	ctx, cancel := context.WithTimeout(req.Context(), 10*time.Second)
	if err := conn.writeJSON(ctx, env); err != nil {
		cancel()
		writeSSE([]byte("data: [DONE]\n\n"))
		return
	}
	cancel()

	// 等终态结果
	select {
	case result := <-ch:
		if result.Success {
			// 解析最终结果发 done 帧
			var inf proto.InferenceResult
			jsonUnmarshalData(result.Data, &inf)
			doneChunk := map[string]any{
				"id": completionID, "object": "chat.completion.chunk", "created": created, "model": inf.Model,
				"choices": []map[string]any{{"index": 0, "finish_reason": fallbackStr(inf.DoneReason, "stop"), "delta": map[string]any{}}},
			}
			data, _ := jsonMarshal(doneChunk)
			writeSSE([]byte("data: " + string(data) + "\n\n"))
			// 扣减租户 token 配额
			if apiKey != "" {
				if t := r.tenants.Get(apiKey); t != nil {
					t.ConsumeTokens(inf.PromptTokens + inf.CompletionTokens)
				}
			}
		}
	case <-time.After(180 * time.Second):
	}
	writeSSE([]byte("data: [DONE]\n\n"))
}

// jsonUnmarshalData 辅助（json.Unmarshal 包装）。
func jsonUnmarshalData(data []byte, v any) { _ = json.Unmarshal(data, v) }

// jsonMarshal 辅助。
func jsonMarshal(v any) ([]byte, error) { return json.Marshal(v) }

// handleEmbeddings POST /v1/embeddings —— OpenAI 兼容嵌入网关。
func (r *Relay) handleEmbeddings(w http.ResponseWriter, req *http.Request) {
	if !r.authorize(req) {
		writeJSON(w, http.StatusUnauthorized, errMap("unauthorized"))
		return
	}
	var oaiReq proto.OpenAIEmbedRequest
	if err := json.NewDecoder(req.Body).Decode(&oaiReq); err != nil {
		writeJSON(w, http.StatusBadRequest, openAIError("invalid_request_error", err.Error()))
		return
	}
	if oaiReq.Model == "" || len(oaiReq.Input) == 0 {
		writeJSON(w, http.StatusBadRequest, openAIError("invalid_request_error", "model 和 input 不能为空"))
		return
	}

	// 选 Agent（Phase 3 GPU 感知调度）
	agentID, err := r.scheduler.Schedule(ScheduleRequest{
		Model:     oaiReq.Model,
		Engine:    oaiReq.Engine,
		AgentID:   oaiReq.AgentID,
		MinBudget: 10,
	}, r.agents.Snapshot())
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, openAIError("no_agent", err.Error()))
		return
	}

	embTask := proto.EmbedTask{Engine: oaiReq.Engine, Model: oaiReq.Model, Input: oaiReq.Input}
	result, err := r.dispatchAndWait(agentID, "embed", embTask, 120*time.Second)
	if err != nil {
		writeJSON(w, http.StatusGatewayTimeout, openAIError("upstream_error", err.Error()))
		return
	}
	if !result.Success {
		writeJSON(w, http.StatusBadGateway, openAIError("upstream_error", result.Error))
		return
	}
	var emb proto.EmbedResult
	if err := json.Unmarshal(result.Data, &emb); err != nil {
		writeJSON(w, http.StatusBadGateway, openAIError("upstream_error", "解析嵌入结果失败"))
		return
	}
	items := make([]proto.OpenAIEmbedItem, 0, len(emb.Embeddings))
	for i, vec := range emb.Embeddings {
		items = append(items, proto.OpenAIEmbedItem{Object: "embedding", Index: i, Embedding: vec})
	}
	writeJSON(w, http.StatusOK, proto.OpenAIEmbedResponse{Object: "list", Data: items, Model: emb.Model})
}

// --- 模型管理 API ---

// handleListModels GET /v1/models —— 列出集群所有可用模型（去重）。
func (r *Relay) handleListModels(w http.ResponseWriter, req *http.Request) {
	if !r.authorize(req) {
		writeJSON(w, http.StatusUnauthorized, errMap("unauthorized"))
		return
	}
	seen := make(map[string]string) // model → engine
	for _, a := range r.agents.Snapshot() {
		for _, m := range a.Models {
			if _, ok := seen[m]; !ok && len(a.Engines) > 0 {
				seen[m] = a.Engines[0]
			}
		}
	}
	data := make([]map[string]any, 0, len(seen))
	for m, eng := range seen {
		data = append(data, map[string]any{
			"id": m, "object": "model", "owned_by": "gpu-mesh:" + eng,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"object": "list", "data": data})
}

// handlePullModel POST /api/models/pull —— 向指定/全部 Agent 下发拉模型任务。
func (r *Relay) handlePullModel(w http.ResponseWriter, req *http.Request) {
	if !r.authorize(req) {
		writeJSON(w, http.StatusUnauthorized, errMap("unauthorized"))
		return
	}
	var body struct {
		Engine  string `json:"engine"`
		Model   string `json:"model"`
		AgentID string `json:"agent_id"` // 空 = 广播到所有在线 Agent
		Tag     string `json:"tag"`      // 或按标签批量
	}
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, errMap(err.Error()))
		return
	}
	if body.Model == "" || body.Engine == "" {
		writeJSON(w, http.StatusBadRequest, errMap("engine 和 model 不能为空"))
		return
	}

	// 选目标 Agent
	var targets []string
	if body.AgentID != "" {
		targets = []string{body.AgentID}
	} else if body.Tag != "" {
		targets = r.agents.FindByTag(body.Tag)
	} else {
		for _, a := range r.agents.Snapshot() {
			if containsStr(a.Engines, body.Engine) {
				targets = append(targets, a.AgentID)
			}
		}
	}
	if len(targets) == 0 {
		writeJSON(w, http.StatusNotFound, errMap("无匹配的在线 Agent"))
		return
	}

	// 异步下发（pull 耗时长，不阻塞 HTTP）
	pullTask := proto.PullTask{Engine: body.Engine, Model: body.Model}
	payloadBytes, _ := json.Marshal(pullTask)
	for _, aid := range targets {
		conn := r.agents.Conn(aid)
		if conn == nil {
			continue
		}
		task := proto.TaskRequest{
			TaskID: "pull-" + uuid.NewString()[:8],
			Type:   "pull", AgentID: aid, Payload: payloadBytes, Timeout: 1800,
		}
		env, _ := proto.NewEnvelope(proto.TypeTaskRequest, "relay", aid, task)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		_ = conn.writeJSON(ctx, env)
		cancel()
	}
	log.Printf("[gateway] 拉模型 %s/%s → %d agents", body.Engine, body.Model, len(targets))
	writeJSON(w, http.StatusAccepted, map[string]any{
		"model": body.Model, "engine": body.Engine, "dispatched_to": targets,
	})
}

// --- Phase 6 多租户配额 ---

// handleAddTenant POST /api/tenants —— 注册租户（生成 API Key）。
func (r *Relay) handleAddTenant(w http.ResponseWriter, req *http.Request) {
	if !r.authorize(req) {
		writeJSON(w, http.StatusUnauthorized, errMap("unauthorized"))
		return
	}
	var body struct {
		Name        string `json:"name"`
		RPM         int    `json:"rpm"`
		DailyTokens int    `json:"daily_tokens"`
	}
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, errMap(err.Error()))
		return
	}
	if body.Name == "" {
		writeJSON(w, http.StatusBadRequest, errMap("name 不能为空"))
		return
	}
	// 生成 API Key
	apiKey := "sk-gpumesh-" + uuid.NewString()[:24]
	r.tenants.AddTenant(&Tenant{
		APIKey:      apiKey,
		Name:        body.Name,
		RPM:         body.RPM,
		DailyTokens: body.DailyTokens,
	})
	if r.audit != nil {
		r.audit.Log(AuditEntry{Event: "tenant_added", APIKey: apiKey[:16], Detail: body.Name})
	}
	log.Printf("[tenant] 新增租户 %s (rpm=%d)", body.Name, body.RPM)
	writeJSON(w, http.StatusCreated, map[string]any{"api_key": apiKey, "name": body.Name})
}

// handleAddWebhook POST /api/alerts/webhook —— 注册告警 webhook。
func (r *Relay) handleAddWebhook(w http.ResponseWriter, req *http.Request) {
	if !r.authorize(req) {
		writeJSON(w, http.StatusUnauthorized, errMap("unauthorized"))
		return
	}
	var body struct {
		Name string `json:"name"`
		URL  string `json:"url"`
	}
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil || body.URL == "" {
		writeJSON(w, http.StatusBadRequest, errMap("name 和 url 不能为空"))
		return
	}
	r.alerts.AddWebhook(body.Name, body.URL)
	writeJSON(w, http.StatusOK, map[string]any{"name": body.Name, "added": true})
}

// checkTenantQuota 检查请求的 API Key 配额。apiKey 为空时跳过（用全局 token）。
//
// 在推理/批量/训练网关入口调用。超限返回 true（拒绝）。
func (r *Relay) checkTenantQuota(apiKey string) (blocked bool, tenant *Tenant) {
	if apiKey == "" {
		return false, nil
	}
	t := r.tenants.Get(apiKey)
	if t == nil {
		return true, nil // 未知 API Key，拒绝
	}
	if err := t.CheckQuota(); err != nil {
		incQuotaBlocked()
		if r.audit != nil {
			r.audit.Log(AuditEntry{Event: "quota_blocked", APIKey: apiKey[:16], Detail: err.Error()})
		}
		return true, t
	}
	return false, t
}


func openAIError(typ, message string) map[string]any {
	return map[string]any{
		"error": map[string]any{"type": typ, "message": message},
	}
}

func containsStr(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func fallbackStr(s, def string) string {
	if strings.TrimSpace(s) == "" {
		return def
	}
	return s
}
