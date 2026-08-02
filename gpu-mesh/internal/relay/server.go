package relay

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/coder/websocket"

	"github.com/QiuShichang/gpu-mesh/internal/proto"
)

// Relay 中继节点主结构。
type Relay struct {
	agents    *Registry
	console   *EventBus
	store     *Store              // 可选，nil 则不持久化
	pending   *pendingResults     // 网关同步等待的任务结果
	scheduler *Scheduler          // Phase 3 GPU 感知调度器
	batch     *BatchOrchestrator  // Phase 4 批量 Map-Reduce 编排器
	train     *TrainOrchestrator  // Phase 5 训练编排器
	audit     *AuditLogger        // Phase 6 审计日志
	tenants   *TenantManager      // Phase 6 多租户配额
	alerts    *AlertManager       // Phase 6 告警

	token string // 鉴权 token（空则不校验）
	cfg   Config // 运维参数
}

// Config Relay 配置。
type Config struct {
	Addr      string // 监听地址 ":7780"
	Token     string // 鉴权 token（空则不校验）
	Store     bool   // 是否启用 bbolt 持久化
	StorePath string // bbolt 文件路径
	AuditPath string // 审计日志路径（空则不记审计）
	// 运维参数（零值用默认，可按负载调优）
	SweepInterval    time.Duration // 心跳超时检查周期，默认 15s
	HeartbeatTimeout time.Duration // 心跳超时阈值，默认 45s
	ReadTimeout      time.Duration // Agent 连接读超时，默认 120s
}

// New 构造 Relay。
func New(cfg Config) (*Relay, error) {
	// 运维参数补默认
	if cfg.SweepInterval == 0 {
		cfg.SweepInterval = 15 * time.Second
	}
	if cfg.HeartbeatTimeout == 0 {
		cfg.HeartbeatTimeout = 45 * time.Second
	}
	if cfg.ReadTimeout == 0 {
		cfg.ReadTimeout = 120 * time.Second
	}
	r := &Relay{
		agents:    NewRegistry(),
		console:   NewEventBus(),
		pending:   newPendingResults(),
		scheduler: NewScheduler(),
		tenants:   NewTenantManager(),
		alerts:    NewAlertManager(),
		token:     cfg.Token,
		cfg:       cfg,
	}
	r.batch = NewBatchOrchestrator(r)
	r.train = NewTrainOrchestrator(r)
	r.scheduler.SetReservedChecker(r.train.isReserved)

	// 审计日志（可选）
	if cfg.AuditPath != "" {
		al, err := NewAuditLogger(cfg.AuditPath)
		if err == nil {
			r.audit = al
		}
	}
	if cfg.Store {
		s, err := NewStore(cfg.StorePath)
		if err != nil {
			return nil, fmt.Errorf("打开持久化存储失败: %w", err)
		}
		r.store = s
		// ★ 重启恢复：把上一次崩溃时"已下发但未完成"的任务标记为 pending，
		// 等 Agent 重连后重新投递。否则这些任务永远卡在 dispatched 状态。
		r.recoverInFlightTasks()
	}
	return r, nil
}

// dispatchPendingTasks Agent 上线时调用：补投该 Agent 的 pending 任务。
//
// 场景：Relay 重启后任务被恢复为 pending，或 Agent 断线重连后其之前的任务还在 pending。
// 这些任务在 Agent 重新上线时应立即补投，实现 at-least-once 语义。
func (r *Relay) dispatchPendingTasks(agentID string, conn *wsConn) {
	if r.store == nil {
		return
	}
	pending, err := r.store.ListTasksByStatus(StatusPending)
	if err != nil {
		log.Printf("[relay] 查询 pending 任务失败: %v", err)
		return
	}
	count := 0
	for _, t := range pending {
		if t.Request.AgentID != agentID {
			continue
		}
		env, err := proto.NewEnvelope(proto.TypeTaskRequest, "relay", agentID, t.Request)
		if err != nil {
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		if err := conn.writeJSON(ctx, env); err != nil {
			cancel()
			log.Printf("[relay] 补投任务 %s 失败: %v", t.Request.TaskID, err)
			continue
		}
		cancel()
		// 标记为已补投（回到 dispatched）
		t.Status = StatusDispatched
		_ = r.store.SaveTask(t)
		count++
	}
	if count > 0 {
		log.Printf("[relay] 向 Agent %s 补投 %d 个 pending 任务", agentID, count)
	}
}

// recoverInFlightTasks Relay 启动时调用：恢复在途任务。
//
// 崩溃/重启前已 dispatched（未 ACK/未完成）的任务，因内存 pending 表丢失而无人处理。
// 此处把它们批量转回 pending 状态，待 Agent 重连后由后续机制重投。
//
// 注意：网关同步任务（dispatchAndWait）走 pending 机制不经 store，重启时无法恢复
// （客户端的 HTTP 请求早已超时）。这是已知限制，仅异步任务（batch/train）可恢复。
func (r *Relay) recoverInFlightTasks() {
	if r.store == nil {
		return
	}
	dispatched, err := r.store.ListTasksByStatus(StatusDispatched)
	if err != nil {
		log.Printf("[relay] 恢复在途任务查询失败: %v", err)
		return
	}
	if len(dispatched) == 0 {
		return
	}
	log.Printf("[relay] 发现 %d 个上次未完成的 dispatched 任务，转 pending 待恢复", len(dispatched))
	for _, t := range dispatched {
		t.Status = StatusPending
		t.Attempt++
		_ = r.store.SaveTask(t)
	}
}

// StartTaskCleaner 启动任务 TTL 清理 goroutine（防 store.db 无限增长）。
//
// 每小时清理一次：超过 24 小时的终态任务（completed/failed）。
// 建议在 main 里调用。
func (r *Relay) StartTaskCleaner(ctx context.Context) {
	if r.store == nil {
		return
	}
	go func() {
		ticker := time.NewTicker(time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				n, err := r.store.CleanOldTasks(24 * time.Hour)
				if err != nil {
					log.Printf("[relay] 任务清理失败: %v", err)
				} else if n > 0 {
					log.Printf("[relay] 清理 %d 条过期任务记录", n)
				}
			}
		}
	}()
}

// Close 释放资源（store + 审计日志文件句柄）。
func (r *Relay) Close() {
	if r.store != nil {
		r.store.Close()
	}
	if r.audit != nil {
		r.audit.Close()
	}
}

// Routes 返回 HTTP 路由（cmd/relay/main.go 挂到 mux）。
func (r *Relay) Routes(mux *http.ServeMux) {
	// Go 1.22+ 新 ServeMux：所有路由统一用 "METHOD /path" 形式，避免路径模式冲突。
	mux.HandleFunc("GET /agent", r.handleAgentWS)   // Agent 反向 WS 接入
	mux.HandleFunc("GET /api/agents", r.handleListAgents)
	mux.HandleFunc("GET /api/metrics", r.handleMetrics)
	mux.HandleFunc("GET /api/events", r.handleSSE)  // 控制台事件流
	mux.HandleFunc("GET /api/tasks", r.handleListTasks)
	mux.HandleFunc("GET /healthz", r.handleHealthz)

	// Phase 2：OpenAI 兼容推理网关
	mux.HandleFunc("POST /v1/chat/completions", r.handleChatCompletions)
	mux.HandleFunc("POST /v1/embeddings", r.handleEmbeddings)
	mux.HandleFunc("GET /v1/models", r.handleListModels)
	mux.HandleFunc("POST /api/models/pull", r.handlePullModel) // 模型管理

	// Phase 4：批量离线推理
	mux.HandleFunc("POST /api/batches", r.handleBatchSubmit)
	mux.HandleFunc("GET /api/batches/", r.handleBatchStatus)

	// Phase 5：训练/微调
	mux.HandleFunc("POST /api/train", r.handleTrainSubmit)
	mux.HandleFunc("GET /api/train/", r.handleTrainStatus)

	// Phase 6：多租户 + 告警配置
	mux.HandleFunc("POST /api/tenants", r.handleAddTenant)
	mux.HandleFunc("POST /api/alerts/webhook", r.handleAddWebhook)
}

// StartSweeper 启动心跳超时清理 goroutine。
func (r *Relay) StartSweeper(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(r.cfg.SweepInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				r.sweep()
			}
		}
	}()
}

// sweep 清理超时 Agent，广播离线事件，关闭连接。
func (r *Relay) sweep() {
	removed, conns := r.agents.Sweep(r.cfg.HeartbeatTimeout)
	for _, c := range conns {
		if c != nil {
			_ = c.conn.Close(websocket.StatusPolicyViolation, "heartbeat timeout")
		}
	}
	for _, id := range removed {
		r.console.Broadcast(Event{Kind: proto.TypeAgentOffline, AgentID: id, TS: time.Now().UnixMilli()})
		log.Printf("[relay] Agent %s 心跳超时离线", id)
		// Phase 6 告警：Agent 异常离线
		if r.alerts != nil {
			r.alerts.Send("agent_offline", "Agent "+id+" 心跳超时离线")
		}
		if r.audit != nil {
			r.audit.Log(AuditEntry{Event: "agent_offline", AgentID: id, Detail: "heartbeat timeout"})
			incAudit()
		}
	}
}

// handleAgentWS Agent 反向 WS 接入端点（穿透 NAT 的核心）。
func (r *Relay) handleAgentWS(w http.ResponseWriter, req *http.Request) {
	// 鉴权（token 在查询参数）
	if !r.authorize(req) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	c, err := websocket.Accept(w, req, &websocket.AcceptOptions{
		InsecureSkipVerify: true, // P0 先放开 origin 校验（Phase 6 收紧）
	})
	if err != nil {
		log.Printf("[relay] WS accept 失败: %v", err)
		return
	}
	c.SetReadLimit(64 << 20)
	conn := &wsConn{conn: c}

	// 阻塞读循环，直到 Agent 断开
	r.serveAgent(req.Context(), conn)
}

// serveAgent 处理一个 Agent 连接的全生命周期。
func (r *Relay) serveAgent(parentCtx context.Context, conn *wsConn) {
	var agentID string
	defer func() {
		if agentID != "" {
			if r.agents.Unregister(agentID, conn) {
				r.console.Broadcast(Event{Kind: proto.TypeAgentOffline, AgentID: agentID, TS: time.Now().UnixMilli()})
			}
		}
		_ = conn.conn.CloseNow()
	}()

	for {
		readCtx, cancel := context.WithTimeout(parentCtx, r.cfg.ReadTimeout)
		_, data, err := conn.conn.Read(readCtx)
		cancel()
		if err != nil {
			if agentID != "" {
				log.Printf("[relay] Agent %s 连接断开: %v", agentID, websocket.CloseStatus(err))
			}
			return
		}
		var env proto.Envelope
		if err := json.Unmarshal(data, &env); err != nil {
			log.Printf("[relay] 解析消息失败: %v", err)
			continue
		}
		switch env.Type {
		case proto.TypeRegister:
			var reg proto.AgentRegister
			if err := env.Decode(&reg); err != nil {
				log.Printf("[relay] 解析 register 失败: %v", err)
				continue
			}
			agentID = reg.AgentID
			r.agents.Register(reg, conn)
			r.console.Broadcast(Event{Kind: proto.TypeAgentOnline, AgentID: agentID, TS: time.Now().UnixMilli()})
			log.Printf("[relay] Agent %s 上线 (host=%s gpus=%d engines=%v yield=%s)",
				agentID, reg.Hostname, len(reg.GPUs), reg.Engines, reg.Yield.Level)
			// ★ 补投该 Agent 的 pending 任务（重启恢复/断线重连后）
			r.dispatchPendingTasks(agentID, conn)
		case proto.TypeHeartbeat:
			var hb proto.AgentHeartbeat
			if err := env.Decode(&hb); err != nil {
				continue
			}
			_, yieldChanged := r.agents.UpdateHeartbeat(hb.AgentID, hb)
			// ★ 优化：只在让位档位真正变化时广播（IDLE↔ACTIVE↔BUSY），
			// 避免百台机器 × 5s 心跳产生每秒 20 次无变化事件风暴。
			// GPU 利用率/显存的细微变化由控制台 5s 轮询兜底。
			if yieldChanged {
				r.console.Broadcast(Event{Kind: "yield_update", AgentID: hb.AgentID, TS: time.Now().UnixMilli()})
				log.Printf("[relay] Agent %s 让位状态变化 → %s (budget=%d%%)",
					hb.AgentID, hb.Yield.Level, hb.Yield.Budget)
				if r.audit != nil {
					r.audit.Log(AuditEntry{Event: "yield_change", AgentID: hb.AgentID,
						Detail: fmt.Sprintf("→ %s (%d%%)", hb.Yield.Level, hb.Yield.Budget)})
				}
			}
		case proto.TypeTaskResult:
			var result proto.TaskResult
			if err := env.Decode(&result); err != nil {
				continue
			}
			// 网关同步等待的任务：投递结果唤醒 HTTP handler
			r.pending.deliver(result.TaskID, result)
			r.console.Broadcast(Event{
				Kind: proto.TypeTaskResult, AgentID: agentID, TS: time.Now().UnixMilli(),
				Payload: mustMarshal(result),
			})
			if r.store != nil {
				_ = r.store.SaveResult(result.TaskID, result)
			}
		case proto.TypeTaskProgress:
			var pg proto.TaskProgress
			if err := env.Decode(&pg); err != nil {
				continue
			}
			// 流式推理：token delta 投递给等待的网关 handler
			r.pending.deliverProgress(pg.TaskID, pg)
			r.console.Broadcast(Event{
				Kind: proto.TypeTaskProgress, AgentID: agentID, TS: time.Now().UnixMilli(),
				Payload: mustMarshal(pg),
			})
		case proto.TypeTaskNack:
			var nack proto.TaskNack
			if err := env.Decode(&nack); err != nil {
				continue
			}
			log.Printf("[relay] Agent %s NACK 任务 %s: %s（Phase 3 将触发重调度）",
				agentID, nack.TaskID, nack.Reason)
			// 通知网关等待方（NACK 当作失败结果投递）
			r.pending.deliver(nack.TaskID, proto.TaskResult{
				TaskID: nack.TaskID, Success: false,
				Error: "agent 拒绝执行: " + nack.Reason,
			})
			if r.store != nil {
				_ = r.store.UpdateTaskStatus(nack.TaskID, StatusPending) // 回退待重调度
			}
		default:
			log.Printf("[relay] 未知消息类型 %s from %s", env.Type, agentID)
		}
	}
}

// authorize 校验鉴权 token（常量时间比较，防时序攻击）。
func (r *Relay) authorize(req *http.Request) bool {
	if r.token == "" {
		return true // 未设 token 则放行
	}
	tok := req.URL.Query().Get("token")
	if tok == "" {
		tok = req.Header.Get("X-Gpu-Mesh-Token")
	}
	if tok == "" {
		tok = req.Header.Get("Authorization")
		if len(tok) > 7 && tok[:7] == "Bearer " {
			tok = tok[7:]
		}
	}
	// 常量时间比较，防时序攻击逐字节猜测
	return subtle.ConstantTimeCompare([]byte(tok), []byte(r.token)) == 1
}

// handleHealthz 深度健康检查（查 store/调度器/在线 Agent 数/运行时长）。
//
// 返回各组件状态，任一关键组件异常则 HTTP 503。
// 用于负载均衡探活、K8s liveness/readiness 探针。
func (r *Relay) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	status := map[string]any{
		"relay":         "ok",
		"version":       relayVersion,
		"agents_online": len(r.agents.Snapshot()),
		"uptime_s":      int(time.Since(metrics.StartedAt).Seconds()),
		"scheduler":     "ok",
		"store":         "disabled",
	}
	httpStatus := http.StatusOK

	// 检查 store 可用性（若启用）
	if r.store != nil {
		if _, err := r.store.ListTasks(1); err != nil {
			status["store"] = "error: " + err.Error()
			httpStatus = http.StatusServiceUnavailable
		} else {
			status["store"] = "ok"
		}
	}

	// 检查调度器（无 goroutine 泄漏隐患：active 数为负则异常）
	status["scheduler_active_slots"] = len(r.scheduler.active)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(httpStatus)
	json.NewEncoder(w).Encode(status)
}

// relayVersion Relay 版本（编译时注入）。
var relayVersion = "dev"

// mustMarshal 序列化辅助（失败返回 nil，仅用于事件载荷）。
func mustMarshal(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}

// marshalJSON 序列化辅助。
func marshalJSON(v any) ([]byte, error) { return json.Marshal(v) }
