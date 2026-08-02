package chat

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/QiuShichang/auto-finance-assistant/internal/modelclient"
	"github.com/QiuShichang/auto-finance-assistant/internal/queue"
	"github.com/QiuShichang/auto-finance-assistant/internal/rag"
	"github.com/QiuShichang/auto-finance-assistant/internal/storage"
)

//go:embed prompts/system.txt prompts/grounded_answer.txt
var promptsFS embed.FS

// Source 表示答案来源。M1 声明，M4 RAG 填充。
type Source struct {
	DocumentID    string `json:"documentId,omitempty"`
	DocumentName  string `json:"documentName,omitempty"`
	Version       string `json:"version,omitempty"`
	Section       string `json:"section,omitempty"`
	PageNumber    int    `json:"pageNumber,omitempty"`
	EffectiveDate string `json:"effectiveDate,omitempty"`
}

// Action 表示答案后附操作按钮。M1 声明，M4+ 填充。
type Action struct {
	Type  string `json:"type"`  // human | calc | link
	Label string `json:"label"`
	URL   string `json:"url,omitempty"`
}

// ChatRequest 对应原计划 9.1。
type ChatRequest struct {
	ConversationID string `json:"conversationId"`
	Question       string `json:"question"`
	Stream         bool   `json:"stream"`
}

// ChatResponse 对应原计划 9.2。M1 子集（sources/confidence 留空）。
type ChatResponse struct {
	MessageID        string   `json:"messageId"`
	Answer           string   `json:"answer"`
	Intent           string   `json:"intent"`
	Confidence       string   `json:"confidence"` // high | medium | low（M4 填充）
	Score            float64  `json:"score"`
	Sources          []Source `json:"sources"`
	Actions          []Action `json:"actions"`
	RequiresHuman    bool     `json:"requiresHuman"`
	TraceID          string   `json:"traceId"`
	DurationMS       int64    `json:"durationMs"`
	PromptTokens     int      `json:"promptTokens"`
	CompletionTokens int      `json:"completionTokens"`
}

// Event 是 SSE 事件，由 Service.AnswerStream 推送。
type Event struct {
	Type    string // status | token | source | complete | error
	Payload string
	Extra   any // complete 时放 ChatResponse
	Err     error
}

// Service 编排问答流程。
// M1：脱敏 → 历史 → Ollama 生成 → 落库。
// M2：脱敏 → FAQ 匹配（高置信短路）→ 否则走 Ollama。
// M4：脱敏 → FAQ → RAG 检索（有证据则 grounded answer，低置信拒答）→ 否则纯模型。
type Service struct {
	model     modelclient.ModelClient
	db        *sql.DB
	queue     *queue.LLMQueue
	log       *slog.Logger
	system    string // 系统提示词
	grounded  string // RAG grounded answer 模板
	faqMu     sync.RWMutex
	faq       *FAQMatcher   // 内存 FAQ 索引
	rag       *rag.Service  // M4 RAG 检索服务
}

// New 构造 Service。
func New(o modelclient.ModelClient, db *sql.DB, q *queue.LLMQueue, log *slog.Logger) *Service {
	return &Service{
		model:     o,
		db:       db,
		queue:    q,
		log:      log,
		system:   loadPrompt("system.txt", "你是汽车金融业务客服助手，请基于知识库回答。"),
		grounded: loadPrompt("grounded_answer.txt", "请根据以下资料回答：\n{context}\n问题：{question}"),
		faq:      NewFAQMatcher(nil), // 启动后由 LoadFAQs 填充
	}
}

// SetRAG 注入 RAG 服务（M4 main.go 装配后调用）。
func (s *Service) SetRAG(r *rag.Service) { s.rag = r }

// LoadFAQs 从数据库加载启用 FAQ 到内存索引。启动时与 FAQ 变更后调用。
func (s *Service) LoadFAQs(ctx context.Context) error {
	items, err := storage.ListEnabledFAQsForMatch(ctx, s.db)
	if err != nil {
		return fmt.Errorf("[chat] 加载 FAQ 失败: %w", err)
	}
	s.faqMu.Lock()
	s.faq = NewFAQMatcher(items)
	s.faqMu.Unlock()
	s.log.Info("[chat] FAQ 索引已加载", "count", len(items))
	return nil
}

// FAQCount 返回当前内存 FAQ 数量（供健康/指标展示）。
func (s *Service) FAQCount() int {
	s.faqMu.RLock()
	defer s.faqMu.RUnlock()
	return s.faq.Size()
}

// loadSystemPrompt 从 embed 读取默认系统提示词（第 12 节）。
func loadPrompt(name, fallback string) string {
	raw, err := promptsFS.ReadFile(name)
	if err != nil {
		return fallback
	}
	return strings.TrimSpace(string(raw))
}

// SystemPrompt 返回当前系统提示词（供 /api/system/info 展示）。
func (s *Service) SystemPrompt() string { return s.system }

// --- FAQ 管理（CRUD 后自动刷新内存索引） ---

// CreateFAQ 创建 FAQ 并刷新索引。
func (s *Service) CreateFAQ(ctx context.Context, f *storage.FAQ) error {
	if f.ID == "" {
		f.ID = uuid.NewString()
	}
	if f.NormalizedQuestion == "" {
		f.NormalizedQuestion = Normalize(f.Question)
	}
	if err := storage.CreateFAQ(ctx, s.db, f); err != nil {
		return err
	}
	return s.LoadFAQs(ctx)
}

// UpdateFAQ 更新 FAQ 并刷新索引。
func (s *Service) UpdateFAQ(ctx context.Context, f *storage.FAQ) error {
	if f.NormalizedQuestion == "" {
		f.NormalizedQuestion = Normalize(f.Question)
	}
	if err := storage.UpdateFAQ(ctx, s.db, f); err != nil {
		return err
	}
	return s.LoadFAQs(ctx)
}

// DeleteFAQ 删除 FAQ 并刷新索引。
func (s *Service) DeleteFAQ(ctx context.Context, id string) error {
	if err := storage.DeleteFAQ(ctx, s.db, id); err != nil {
		return err
	}
	return s.LoadFAQs(ctx)
}

// GetFAQ 按 ID 查询 FAQ。
func (s *Service) GetFAQ(ctx context.Context, id string) (*storage.FAQ, error) {
	return storage.GetFAQ(ctx, s.db, id)
}

// ListFAQs 返回 FAQ 列表。
func (s *Service) ListFAQs(ctx context.Context, enabledOnly bool, limit int) ([]*storage.FAQ, error) {
	return storage.ListFAQs(ctx, s.db, enabledOnly, limit)
}

// TestFAQMatch 测试一段文本的 FAQ 匹配效果（供管理后台"测试匹配"）。
func (s *Service) TestFAQMatch(question string) FAQMatch {
	return s.matchFAQ(question)
}

// CreateConversation 创建新会话。
func (s *Service) CreateConversation(ctx context.Context, userID, title string) (*storage.Conversation, error) {
	id := uuid.NewString()
	return storage.CreateConversation(ctx, s.db, id, userID, title)
}

// ListConversations 返回会话列表。
func (s *Service) ListConversations(ctx context.Context, limit int) ([]*storage.Conversation, error) {
	return storage.ListConversations(ctx, s.db, limit)
}

// GetConversationWithMessages 返回会话及其消息。
func (s *Service) GetConversationWithMessages(ctx context.Context, id string) (*storage.Conversation, []*storage.Message, error) {
	conv, err := storage.GetConversation(ctx, s.db, id)
	if err != nil {
		return nil, nil, err
	}
	msgs, err := storage.ListMessages(ctx, s.db, id, 100)
	if err != nil {
		return nil, nil, err
	}
	return conv, msgs, nil
}

// AnswerStream 执行流式问答。通过 events channel 推送 SSE 事件。
// 即使流式，也会把完整回答落库（含 token 统计）。
// 调用方必须消费 channel 至收到 complete/error。
func (s *Service) AnswerStream(ctx context.Context, req ChatRequest) (<-chan Event, error) {
	traceID := uuid.NewString()
	events := make(chan Event, 64)

	// 1. 输入校验
	if strings.TrimSpace(req.Question) == "" {
		close(events)
		return events, fmt.Errorf("问题不能为空")
	}
	if len(req.Question) > 2000 {
		close(events)
		return events, fmt.Errorf("问题长度不能超过 2000 字")
	}
	if req.ConversationID != "" {
		// 校验会话存在
		if _, err := storage.GetConversation(ctx, s.db, req.ConversationID); err != nil {
			return nil, fmt.Errorf("会话 %s 不存在: %w", req.ConversationID, err)
		}
	}

	// 脱敏后的问题（用于日志 + 落库记录 user 消息 + 会话标题）
	maskedQuestion := MaskPII(req.Question)

	// 自动建会话用脱敏后的问题作标题
	if req.ConversationID == "" {
		title := truncate(strings.TrimSpace(maskedQuestion), 30)
		conv, err := s.CreateConversation(ctx, "", title)
		if err != nil {
			return nil, err
		}
		req.ConversationID = conv.ID
	}

	// 落库 user 消息
	userMsg := &storage.Message{
		ID:             uuid.NewString(),
		ConversationID: req.ConversationID,
		Role:           "user",
		Content:        maskedQuestion,
	}
	if err := storage.AppendMessage(ctx, s.db, userMsg); err != nil {
		return nil, err
	}

	// 1b. 合规预检：触碰承诺审批/放款等红线时直接拒答，不依赖知识库
	if hit, refuseMsg := CheckCompliance(req.Question); hit {
		go s.runComplianceRefuse(ctx, req, traceID, refuseMsg, events)
		return events, nil
	}

	// 1c. 输入预检：脏话/无关话题/注入/闲聊
	guard := CheckInput(req.Question)
	if guard.Action == GuardShortcut {
		go s.runGuardReply(ctx, req, traceID, guard.Reply, "guard_shortcut", events)
		return events, nil
	}
	if guard.Action == GuardReject {
		go s.runGuardReply(ctx, req, traceID, guard.Reply, "guard_reject:"+guard.Reason, events)
		return events, nil
	}

	// 1d. 超短问题（<4 字符）无法提取有效 FTS 关键词，标记跳过 RAG
	isShortChitchat := len([]rune(strings.TrimSpace(maskedQuestion))) < 4

	// 2. FAQ 匹配短路：高置信命中时直接返回标准答案，不调用模型（<500ms）
	if match := s.matchFAQ(req.Question); match.IsHighConfidence() {
		go s.runFAQAnswer(ctx, req, maskedQuestion, traceID, match, events)
		return events, nil
	}

	go s.runGeneration(ctx, req, maskedQuestion, traceID, events, isShortChitchat)
	return events, nil
}

// matchFAQ 在内存 FAQ 索引上做匹配（读锁保护）。
func (s *Service) matchFAQ(question string) FAQMatch {
	s.faqMu.RLock()
	matcher := s.faq
	s.faqMu.RUnlock()
	return matcher.Match(question)
}

// runFAQAnswer FAQ 命中时的快速应答路径：不进队列、不调模型。
// 把标准答案作为单次 complete 事件推送并落库。
func (s *Service) runFAQAnswer(ctx context.Context, req ChatRequest, maskedQ, traceID string, match FAQMatch, events chan<- Event) {
	defer close(events)
	start := time.Now()

	if !sendEvent(ctx, events, Event{Type: "status", Payload: "FAQ 命中", Extra: traceID}) {
		return
	}

	answer := strings.TrimSpace(match.FAQ.Answer)
	assistantMsgID := uuid.NewString()
	resp := ChatResponse{
		MessageID:     assistantMsgID,
		Answer:        answer,
		Intent:        "faq",
		Confidence:    "high",
		Score:         match.Score,
		TraceID:       traceID,
		DurationMS:    time.Since(start).Milliseconds(),
	}

	// 落库 assistant 消息（标记来源为 FAQ）
	if err := storage.AppendMessage(ctx, s.db, &storage.Message{
		ID:               assistantMsgID,
		ConversationID:   req.ConversationID,
		Role:             "assistant",
		Content:          answer,
		Intent:           "faq",
		Confidence:       match.Score,
		DurationMS:       resp.DurationMS,
	}); err != nil {
		s.log.Error("[chat] FAQ 回答落库失败", "traceId", traceID, "err", err)
	}

	s.log.Info("[chat] FAQ 命中", "traceId", traceID, "faqId", match.FAQ.ID,
		"strategy", match.Strategy, "score", match.Score, "durationMs", resp.DurationMS)

	sendEvent(ctx, events, Event{Type: "complete", Extra: resp})
}

// runComplianceRefuse 合规拒答：触碰红线时不调模型，直接返回拒答并转人工。
func (s *Service) runComplianceRefuse(ctx context.Context, req ChatRequest, traceID, refuseMsg string, events chan<- Event) {
	defer close(events)
	start := time.Now()
	if !sendEvent(ctx, events, Event{Type: "status", Payload: "合规检查", Extra: traceID}) {
		return
	}
	assistantMsgID := uuid.NewString()
	resp := ChatResponse{
		MessageID:     assistantMsgID,
		Answer:        refuseMsg,
		Intent:        "compliance_refuse",
		Confidence:    "high",
		Score:         1.0,
		RequiresHuman: true,
		TraceID:       traceID,
		DurationMS:    time.Since(start).Milliseconds(),
	}
	_ = storage.AppendMessage(ctx, s.db, &storage.Message{
		ID: uuid.NewString(), ConversationID: req.ConversationID,
		Role: "assistant", Content: refuseMsg, Intent: "compliance_refuse",
		Confidence: 1.0, DurationMS: resp.DurationMS,
	})
	s.log.Info("[chat] 合规拒答", "traceId", traceID)
	sendEvent(ctx, events, Event{Type: "complete", Extra: resp})
}

// sendEvent 向 events channel 发送事件，可被 parent ctx 取消（防 goroutine 泄漏）。
// 返回 false 表示 ctx 已取消，调用方应立即返回。
func sendEvent(ctx context.Context, events chan<- Event, ev Event) bool {
	select {
	case events <- ev:
		return true
	case <-ctx.Done():
		return false
	}
}

// runGeneration 在队列保护下调用 Ollama 并推送事件。
// M4：若 RAG 可用，先检索；有证据用 grounded prompt，低置信拒答。
// isShortChitchat=true 时跳过 RAG（短问候无法提取有效关键词）。
func (s *Service) runGeneration(parent context.Context, req ChatRequest, maskedQ, traceID string, events chan<- Event, isShortChitchat bool) {
	defer close(events)

	start := time.Now()
	if !sendEvent(parent, events, Event{Type: "status", Payload: "排队中", Extra: traceID}) {
		return
	}

	// M4：RAG 检索（队列外执行，不占模型并发）
	// 多轮上下文增强：如果是简短追问，拼接上一轮用户问题补充检索上下文
	ragQuery := maskedQ
	if msgs, err := s.recentHistory(parent, req.ConversationID, 4); err == nil && len(msgs) >= 2 {
		// 取上一条 user 消息
		var lastUserQ string
		for i := len(msgs) - 1; i >= 0; i-- {
			if msgs[i].Role == "user" {
				lastUserQ = msgs[i].Content
				break
			}
		}
		// 当前问题短（<8字）且上一轮有内容 → 拼接
		if lastUserQ != "" && len([]rune(maskedQ)) < 8 {
			ragQuery = lastUserQ + " " + maskedQ
		}
	}

	var ragResp *rag.RetrieveResponse
	if s.rag != nil && !isShortChitchat {
		if !sendEvent(parent, events, Event{Type: "status", Payload: "检索知识库"}) {
			return
		}
		rr, err := s.rag.Retrieve(parent, rag.SearchQuery{Text: ragQuery})
		if err != nil {
			s.log.Warn("[chat] RAG 检索失败，降级为纯模型", "traceId", traceID, "err", err)
		} else if len(rr.Results) > 0 {
			// 有检索结果：先判定置信度，通过才推送来源 + grounded answer
			if s.rag.ShouldAnswer(rr.Level) {
				ragResp = rr
				// 只在确认要回答时才推送来源（避免拒答后还显示来源造成困惑）
				for _, r := range rr.Results {
					if !sendEvent(parent, events, Event{Type: "source", Payload: r.DocumentName, Extra: r}) {
						return
					}
				}
			} else {
				// 有结果但低置信：拒答（来源不推送）
				duration := time.Since(start).Milliseconds()
				refuse := "根据现有知识库，我无法确认该问题的答案。建议联系人工客服或提供更具体的信息。"
				resp := ChatResponse{
					MessageID:     uuid.NewString(),
					Answer:        refuse,
					Intent:        "refuse",
					Confidence:    string(rr.Level),
					Score:         rr.Confidence,
					RequiresHuman: true,
					TraceID:       traceID,
					DurationMS:    duration,
				}
				if err := storage.AppendMessage(parent, s.db, &storage.Message{
					ID: uuid.NewString(), ConversationID: req.ConversationID,
					Role: "assistant", Content: refuse, Intent: "refuse",
					Confidence: rr.Confidence, DurationMS: duration,
				}); err != nil {
					s.log.Error("[chat] 拒答落库失败", "traceId", traceID, "err", err)
				}
				s.log.Info("[chat] 低置信拒答", "traceId", traceID, "score", rr.Confidence)
				sendEvent(parent, events, Event{Type: "complete", Extra: resp})
				return
			}
		}
		// 无检索结果（len==0）或 ShouldAnswer 通过：继续走 grounded answer / 纯模型
	}

	var fullAnswer strings.Builder
	var promptTokens, completionTokens int

	// 构造用户消息：RAG 有证据时用 grounded prompt 包裹上下文
	userContent := maskedQ
	if ragResp != nil && ragResp.Context != "" {
		userContent = strings.ReplaceAll(s.grounded, "{context}", ragResp.Context)
		userContent = strings.ReplaceAll(userContent, "{question}", maskedQ)
	}

	err := s.queue.Run(parent, func(ctx context.Context) error {
		if !sendEvent(ctx, events, Event{Type: "status", Payload: "生成中"}) {
			return ctx.Err()
		}

		// 取最近历史（脱敏后）
		history, err := s.recentHistory(ctx, req.ConversationID, 10)
		if err != nil {
			return err
		}
		history = append(history, modelclient.Message{Role: "user", Content: userContent})

		stream, err := s.model.Chat(ctx, s.model.ChatModel(), s.system, history)
		if err != nil {
			return err
		}

		for ev := range stream {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if ev.Error != nil {
				return ev.Error
			}
			if ev.Token != "" {
				fullAnswer.WriteString(ev.Token)
				if !sendEvent(ctx, events, Event{Type: "token", Payload: ev.Token}) {
					return ctx.Err()
				}
			}
			if ev.Done {
				promptTokens = ev.PromptTokens
				completionTokens = ev.CompletionTokens
			}
		}
		return nil
	})

	duration := time.Since(start).Milliseconds()

	if err != nil {
		s.log.Error("[chat] 生成失败", "traceId", traceID, "err", err, "durationMs", duration)
		sendEvent(parent, events, Event{Type: "error", Err: err})
		// 仍记录失败痕迹到消息表（role=assistant, 内容为错误提示）
		_ = storage.AppendMessage(parent, s.db, &storage.Message{
			ID:               uuid.NewString(),
			ConversationID:   req.ConversationID,
			Role:             "assistant",
			Content:          fmt.Sprintf("（生成失败：%s）", err.Error()),
			DurationMS:       duration,
		})
		return
	}

	answer := strings.TrimSpace(fullAnswer.String())
	assistantMsgID := uuid.NewString()
	resp := ChatResponse{
		MessageID:        assistantMsgID,
		Answer:           answer,
		TraceID:          traceID,
		DurationMS:       duration,
		PromptTokens:     promptTokens,
		CompletionTokens: completionTokens,
	}

	// 落库 assistant 消息
	if err := storage.AppendMessage(parent, s.db, &storage.Message{
		ID:               assistantMsgID,
		ConversationID:   req.ConversationID,
		Role:             "assistant",
		Content:          answer,
		DurationMS:       duration,
		PromptTokens:     promptTokens,
		CompletionTokens: completionTokens,
	}); err != nil {
		s.log.Error("[chat] 落库回答失败", "traceId", traceID, "err", err)
	}

	s.log.Info("[chat] 回答完成", "traceId", traceID, "durationMs", duration,
		"promptTokens", promptTokens, "completionTokens", completionTokens, "chars", len(answer))

	sendEvent(parent, events, Event{Type: "complete", Extra: resp})
}

// recentHistory 返回最近 N 轮历史（user/assistant 交替），转为 modelclient.Message。
func (s *Service) recentHistory(ctx context.Context, convID string, limit int) ([]modelclient.Message, error) {
	msgs, err := storage.ListMessages(ctx, s.db, convID, limit)
	if err != nil {
		return nil, err
	}
	// 去掉刚才插入的 user 消息（已在 caller 追加），避免重复。
	out := make([]modelclient.Message, 0, len(msgs))
	for _, m := range msgs {
		if m.Role == "system" {
			continue
		}
		out = append(out, modelclient.Message{Role: m.Role, Content: m.Content})
	}
	// 移除最后一条（本次 user 消息），由 caller 单独追加
	if n := len(out); n > 0 && out[n-1].Role == "user" {
		out = out[:n-1]
	}
	return out, nil
}

func truncate(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n]) + "…"
}

// runGuardReply Guard 层预设回复（闲聊/拒绝），不调模型。
func (s *Service) runGuardReply(ctx context.Context, req ChatRequest, traceID, reply, intent string, events chan<- Event) {
	defer close(events)
	start := time.Now()
	if !sendEvent(ctx, events, Event{Type: "status", Payload: "处理中", Extra: traceID}) {
		return
	}
	msgID := uuid.NewString()
	resp := ChatResponse{
		MessageID:  msgID,
		Answer:     reply,
		Intent:     intent,
		TraceID:    traceID,
		DurationMS: time.Since(start).Milliseconds(),
	}
	if err := storage.AppendMessage(ctx, s.db, &storage.Message{
		ID: msgID, ConversationID: req.ConversationID,
		Role: "assistant", Content: reply, Intent: intent,
	}); err != nil {
		s.log.Error("[chat] Guard 回答落库失败", "traceId", traceID, "err", err)
	}
	s.log.Info("[chat] Guard 拦截", "traceId", traceID, "intent", intent, "dur", resp.DurationMS)
	sendEvent(ctx, events, Event{Type: "complete", Extra: resp})
}
