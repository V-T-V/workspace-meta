package api

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/QiuShichang/auto-finance-assistant/internal/chat"
	"github.com/QiuShichang/auto-finance-assistant/internal/modelclient"
	"github.com/QiuShichang/auto-finance-assistant/internal/queue"
	"github.com/QiuShichang/auto-finance-assistant/internal/storage"
)

// fakeModel 是测试用 ModelClient 实现，避免依赖真实 Ollama。
type fakeModel struct {
	chatModel  string
	embedModel string
	reachable  bool
	hasModel   bool
}

func (f *fakeModel) Chat(ctx context.Context, model, systemPrompt string, history []modelclient.Message) (<-chan modelclient.ChatEvent, error) {
	ch := make(chan modelclient.ChatEvent, 2)
	ch <- modelclient.ChatEvent{Token: "测试回答"}
	ch <- modelclient.ChatEvent{Done: true, PromptTokens: 10, CompletionTokens: 4}
	close(ch)
	return ch, nil
}
func (f *fakeModel) Embed(ctx context.Context, texts []string, batchSize int) ([][]float32, error) {
	out := make([][]float32, len(texts))
	for i := range texts {
		out[i] = []float32{0.1, 0.2, 0.3}
	}
	return out, nil
}
func (f *fakeModel) Health(ctx context.Context) modelclient.HealthStatus {
	return modelclient.HealthStatus{Reachable: f.reachable, HasModel: f.hasModel, Models: []string{f.chatModel}}
}
func (f *fakeModel) ChatModel() string       { return f.chatModel }
func (f *fakeModel) EmbeddingModel() string  { return f.embedModel }
func (f *fakeModel) BaseURL() string         { return "http://localhost:11434" }
func (f *fakeModel) Backend() string         { return "ollama" }

// newTestServer 构造一个完整装配的 Server（真实 SQLite + fake 模型 + 真实 chat.Service）。
// 返回 Server 与已迁移的 db（供断言）。
func newTestServer(t *testing.T) (*Server, *sql.DB) {
	t.Helper()
	dir := t.TempDir()
	dbPath := dir + "/test.db"
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	db, err := storage.OpenDB(dbPath, log)
	if err != nil {
		t.Fatalf("打开测试 DB 失败: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	// M1-M9 已完成：测试需应用全部 migration，否则 FAQ/文档/审计等表缺失。
	if err := storage.Migrate(context.Background(), db, storage.AllActiveVersions(), log); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	fm := &fakeModel{chatModel: "qwen3", embedModel: "bge", reachable: true, hasModel: true}
	q := queue.NewWithTimeouts(1, 2, time.Second, 5*time.Second, log)
	svc := chat.New(fm, db, q, log)
	srv := New(svc, fm, q, db, nil)
	srv.SetBackend("ollama")
	return srv, db
}

// doRequest 辅助：构造请求并执行，返回响应。
func doRequest(t *testing.T, mux *http.ServeMux, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var r io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body 失败: %v", err)
		}
		r = bytes.NewReader(b)
	}
	req := httptest.NewRequest(method, path, r)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

// decodeBody 解析 JSON 响应体。
func decodeBody(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil {
		t.Fatalf("解析响应失败: %v, body=%s", err, rec.Body.String())
	}
	return m
}

// ===========================================================================
// 健康检查端点
// ===========================================================================

func TestHealth_Ok(t *testing.T) {
	srv, _ := newTestServer(t)
	mux := http.NewServeMux()
	srv.Register(mux)
	rec := doRequest(t, mux, "GET", "/api/health", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("健康时应返回 200，实际 %d body=%s", rec.Code, rec.Body.String())
	}
	m := decodeBody(t, rec)
	if m["status"] != "ok" {
		t.Errorf("status 应为 ok，实际 %v", m["status"])
	}
	if m["database"] != "ok" {
		t.Errorf("database 应为 ok，实际 %v", m["database"])
	}
	if m["ollama"] != "ok" {
		t.Errorf("ollama 应为 ok，实际 %v", m["ollama"])
	}
	if m["model"] != "qwen3" {
		t.Errorf("model 应为 qwen3，实际 %v", m["model"])
	}
	if m["version"] == nil {
		t.Error("version 不应为 nil")
	}
}

func TestHealth_Degraded_WhenModelDown(t *testing.T) {
	dir := t.TempDir()
	dbPath := dir + "/test.db"
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	db, _ := storage.OpenDB(dbPath, log)
	t.Cleanup(func() { _ = db.Close() })
	fm := &fakeModel{chatModel: "qwen3", reachable: false, hasModel: false}
	q := queue.NewWithTimeouts(1, 2, time.Second, 5*time.Second, log)
	svc := chat.New(fm, db, q, log)
	srv := New(svc, fm, q, db, nil)

	mux := http.NewServeMux()
	srv.Register(mux)
	rec := doRequest(t, mux, "GET", "/api/health", nil)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("模型不可达应返回 503，实际 %d", rec.Code)
	}
	m := decodeBody(t, rec)
	if m["status"] != "degraded" {
		t.Errorf("status 应为 degraded，实际 %v", m["status"])
	}
	if m["ollama"] != "down" {
		t.Errorf("ollama 应为 down，实际 %v", m["ollama"])
	}
}

func TestHealth_MissingModel(t *testing.T) {
	dir := t.TempDir()
	dbPath := dir + "/test.db"
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	db, _ := storage.OpenDB(dbPath, log)
	t.Cleanup(func() { _ = db.Close() })
	storage.Migrate(context.Background(), db, storage.AllActiveVersions(), log)
	fm := &fakeModel{chatModel: "qwen3", reachable: true, hasModel: false}
	q := queue.NewWithTimeouts(1, 2, time.Second, 5*time.Second, log)
	svc := chat.New(fm, db, q, log)
	srv := New(svc, fm, q, db, nil)
	mux := http.NewServeMux()
	srv.Register(mux)
	rec := doRequest(t, mux, "GET", "/api/health", nil)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("模型缺失应返回 503，实际 %d", rec.Code)
	}
	m := decodeBody(t, rec)
	if m["ollama"] != "missing_model" {
		t.Errorf("ollama 应为 missing_model，实际 %v", m["ollama"])
	}
}

// ===========================================================================
// 会话端点
// ===========================================================================

func TestCreateConversation_DefaultTitle(t *testing.T) {
	srv, _ := newTestServer(t)
	mux := http.NewServeMux()
	srv.Register(mux)
	// 空 body → 用默认标题
	rec := doRequest(t, mux, "POST", "/api/conversations", nil)
	if rec.Code != http.StatusCreated {
		t.Fatalf("应返回 201，实际 %d body=%s", rec.Code, rec.Body.String())
	}
	m := decodeBody(t, rec)
	if m["title"] != "新会话" {
		t.Errorf("默认标题应为 新会话，实际 %v", m["title"])
	}
	if m["id"] == nil || m["id"] == "" {
		t.Error("id 不应为空")
	}
}

func TestCreateConversation_InvalidBody(t *testing.T) {
	srv, _ := newTestServer(t)
	mux := http.NewServeMux()
	srv.Register(mux)
	// ContentLength>0 但 body 非法
	req := httptest.NewRequest("POST", "/api/conversations", strings.NewReader("{bad json"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("非法 body 应返回 400，实际 %d", rec.Code)
	}
	m := decodeBody(t, rec)
	if errMsg, ok := m["error"].(map[string]any); !ok || errMsg["code"] != "invalid_body" {
		t.Errorf("error.code 应为 invalid_body，实际 %v", m["error"])
	}
}

func TestListConversations(t *testing.T) {
	srv, _ := newTestServer(t)
	mux := http.NewServeMux()
	srv.Register(mux)
	// 先建 2 条
	doRequest(t, mux, "POST", "/api/conversations", map[string]string{"title": "A"})
	doRequest(t, mux, "POST", "/api/conversations", map[string]string{"title": "B"})
	rec := doRequest(t, mux, "GET", "/api/conversations", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("应返回 200，实际 %d", rec.Code)
	}
	m := decodeBody(t, rec)
	items, _ := m["items"].([]any)
	if len(items) != 2 {
		t.Errorf("应有 2 个会话，实际 %d", len(items))
	}
	if m["total"] != float64(2) {
		t.Errorf("total 应为 2，实际 %v", m["total"])
	}
}

func TestGetConversation_NotFound(t *testing.T) {
	srv, _ := newTestServer(t)
	mux := http.NewServeMux()
	srv.Register(mux)
	rec := doRequest(t, mux, "GET", "/api/conversations/nonexistent", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("不存在应返回 404，实际 %d", rec.Code)
	}
	m := decodeBody(t, rec)
	if errMsg, _ := m["error"].(map[string]any); errMsg["code"] != "not_found" {
		t.Errorf("code 应为 not_found，实际 %v", m["error"])
	}
}

func TestGetConversation_WithMessages(t *testing.T) {
	srv, _ := newTestServer(t)
	mux := http.NewServeMux()
	srv.Register(mux)
	// 建会话
	rec := doRequest(t, mux, "POST", "/api/conversations", map[string]string{"title": "T"})
	m := decodeBody(t, rec)
	id := m["id"].(string)
	// 直接落库消息（绕过 chat 流程）
	storage.AppendMessage(context.Background(), srv.db, &storage.Message{
		ID: "m1", ConversationID: id, Role: "user", Content: "问",
	})
	storage.AppendMessage(context.Background(), srv.db, &storage.Message{
		ID: "m2", ConversationID: id, Role: "assistant", Content: "答",
	})
	rec = doRequest(t, mux, "GET", "/api/conversations/"+id, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("应返回 200，实际 %d body=%s", rec.Code, rec.Body.String())
	}
	m = decodeBody(t, rec)
	msgs, _ := m["messages"].([]any)
	if len(msgs) != 2 {
		t.Errorf("应有 2 条消息，实际 %d", len(msgs))
	}
}

// ===========================================================================
// 金融计算端点（最独立，重点覆盖参数校验 + 错误处理 + 响应格式）
// ===========================================================================

func TestEqualPayment_Endpoint_Success(t *testing.T) {
	srv, _ := newTestServer(t)
	mux := http.NewServeMux()
	srv.Register(mux)
	rec := doRequest(t, mux, "POST", "/api/finance/equal-payment", map[string]any{
		"principal": 200000, "annualRate": 4.5, "months": 36,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("应返回 200，实际 %d body=%s", rec.Code, rec.Body.String())
	}
	m := decodeBody(t, rec)
	if m["type"] != "equal_payment" {
		t.Errorf("type 应为 equal_payment，实际 %v", m["type"])
	}
	if m["months"] != float64(36) {
		t.Errorf("months 应为 36，实际 %v", m["months"])
	}
	if m["annualRate"] != 4.5 {
		t.Errorf("annualRate 应为 4.5，实际 %v", m["annualRate"])
	}
	// monthlyPayment 应为字符串
	if _, ok := m["monthlyPayment"].(string); !ok {
		t.Errorf("monthlyPayment 应为字符串，实际 %T", m["monthlyPayment"])
	}
	// 必须含免责声明
	if disc, _ := m["disclaimer"].(string); !strings.Contains(disc, "试算") {
		t.Errorf("disclaimer 应含 试算，实际 %v", m["disclaimer"])
	}
}

func TestEqualPayment_Endpoint_InvalidBody(t *testing.T) {
	srv, _ := newTestServer(t)
	mux := http.NewServeMux()
	srv.Register(mux)
	req := httptest.NewRequest("POST", "/api/finance/equal-payment", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("非法 body 应返回 400，实际 %d", rec.Code)
	}
}

func TestEqualPayment_Endpoint_InvalidParams(t *testing.T) {
	srv, _ := newTestServer(t)
	mux := http.NewServeMux()
	srv.Register(mux)
	cases := []map[string]any{
		{"principal": 0, "annualRate": 4.5, "months": 36},    // 本金为 0
		{"principal": 10000, "annualRate": 4.5, "months": 0},  // 期数为 0
		{"principal": 10000, "annualRate": -1, "months": 12},  // 负利率
	}
	for _, c := range cases {
		rec := doRequest(t, mux, "POST", "/api/finance/equal-payment", c)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("非法参数应返回 400，实际 %d body=%s params=%v", rec.Code, rec.Body.String(), c)
		}
		m := decodeBody(t, rec)
		if errMsg, _ := m["error"].(map[string]any); errMsg["code"] != "calc_error" {
			t.Errorf("code 应为 calc_error，实际 %v", m["error"])
		}
	}
}

func TestEqualPrincipal_Endpoint_Success(t *testing.T) {
	srv, _ := newTestServer(t)
	mux := http.NewServeMux()
	srv.Register(mux)
	rec := doRequest(t, mux, "POST", "/api/finance/equal-principal", map[string]any{
		"principal": 200000, "annualRate": 4.5, "months": 36,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("应返回 200，实际 %d body=%s", rec.Code, rec.Body.String())
	}
	m := decodeBody(t, rec)
	if m["type"] != "equal_principal" {
		t.Errorf("type 应为 equal_principal，实际 %v", m["type"])
	}
	// firstPayment > lastPayment（字符串里数值递减，这里只校验字段存在）
	for _, k := range []string{"monthlyPrincipal", "firstPayment", "lastPayment", "totalPayment", "totalInterest"} {
		if _, ok := m[k].(string); !ok {
			t.Errorf("%s 应为字符串，实际 %T", k, m[k])
		}
	}
}

func TestEqualPrincipal_Endpoint_Invalid(t *testing.T) {
	srv, _ := newTestServer(t)
	mux := http.NewServeMux()
	srv.Register(mux)
	// 本金为 0
	rec := doRequest(t, mux, "POST", "/api/finance/equal-principal", map[string]any{
		"principal": 0, "annualRate": 4.5, "months": 36,
	})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("应返回 400，实际 %d", rec.Code)
	}
}

func TestDownPayment_Endpoint_Success(t *testing.T) {
	srv, _ := newTestServer(t)
	mux := http.NewServeMux()
	srv.Register(mux)
	rec := doRequest(t, mux, "POST", "/api/finance/down-payment", map[string]any{
		"vehiclePrice": 200000, "downPaymentPct": 0.2,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("应返回 200，实际 %d body=%s", rec.Code, rec.Body.String())
	}
	m := decodeBody(t, rec)
	if m["type"] != "down_payment" {
		t.Errorf("type 应为 down_payment，实际 %v", m["type"])
	}
	if m["downPayment"] != "40,000" {
		t.Errorf("首付应为 40,000，实际 %v", m["downPayment"])
	}
	if m["loanPrincipal"] != "160,000" {
		t.Errorf("贷款本金应为 160,000，实际 %v", m["loanPrincipal"])
	}
}

func TestDownPayment_Endpoint_InvalidPct(t *testing.T) {
	srv, _ := newTestServer(t)
	mux := http.NewServeMux()
	srv.Register(mux)
	// 比例 > 1
	rec := doRequest(t, mux, "POST", "/api/finance/down-payment", map[string]any{
		"vehiclePrice": 200000, "downPaymentPct": 1.5,
	})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("比例 150%% 应返回 400，实际 %d", rec.Code)
	}
	// 负比例
	rec = doRequest(t, mux, "POST", "/api/finance/down-payment", map[string]any{
		"vehiclePrice": 200000, "downPaymentPct": -0.1,
	})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("负比例应返回 400，实际 %d", rec.Code)
	}
}

// ===========================================================================
// FAQ 端点
// ===========================================================================

func TestFAQ_CRUD_Endpoint(t *testing.T) {
	srv, _ := newTestServer(t)
	mux := http.NewServeMux()
	srv.Register(mux)
	// 创建
	rec := doRequest(t, mux, "POST", "/api/faqs", map[string]any{
		"category": "贷款", "question": "利率是多少？", "answer": "4.5%", "keywords": "利率", "priority": 10,
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("创建应返回 201，实际 %d body=%s", rec.Code, rec.Body.String())
	}
	m := decodeBody(t, rec)
	id := m["id"].(string)
	if m["question"] != "利率是多少？" {
		t.Errorf("question 不匹配，实际 %v", m["question"])
	}
	if m["enabled"] != true {
		t.Errorf("未传 enabled 应默认 true，实际 %v", m["enabled"])
	}

	// 查询列表
	rec = doRequest(t, mux, "GET", "/api/faqs", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("列表应返回 200，实际 %d", rec.Code)
	}
	m = decodeBody(t, rec)
	if m["total"] != float64(1) {
		t.Errorf("total 应为 1，实际 %v", m["total"])
	}

	// 单查
	rec = doRequest(t, mux, "GET", "/api/faqs/"+id, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("单查应返回 200，实际 %d", rec.Code)
	}

	// 更新
	rec = doRequest(t, mux, "PUT", "/api/faqs/"+id, map[string]any{"answer": "5.0%"})
	if rec.Code != http.StatusOK {
		t.Fatalf("更新应返回 200，实际 %d", rec.Code)
	}
	m = decodeBody(t, rec)
	if m["answer"] != "5.0%" {
		t.Errorf("更新后 answer 应为 5.0%%，实际 %v", m["answer"])
	}

	// 删除
	rec = doRequest(t, mux, "DELETE", "/api/faqs/"+id, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("删除应返回 200，实际 %d", rec.Code)
	}
	m = decodeBody(t, rec)
	if m["deleted"] != true {
		t.Errorf("deleted 应为 true，实际 %v", m["deleted"])
	}

	// 删除后查应 404
	rec = doRequest(t, mux, "GET", "/api/faqs/"+id, nil)
	if rec.Code != http.StatusNotFound {
		t.Errorf("删除后查应 404，实际 %d", rec.Code)
	}
}

func TestCreateFAQ_Validation(t *testing.T) {
	srv, _ := newTestServer(t)
	mux := http.NewServeMux()
	srv.Register(mux)
	cases := []struct {
		name string
		body map[string]any
	}{
		{"question 空", map[string]any{"question": "", "answer": "x"}},
		{"answer 空", map[string]any{"question": "x", "answer": ""}},
		{"两者都空", map[string]any{"question": "   ", "answer": "   "}},
	}
	for _, c := range cases {
		rec := doRequest(t, mux, "POST", "/api/faqs", c.body)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%s: 应返回 400，实际 %d", c.name, rec.Code)
		}
		m := decodeBody(t, rec)
		if errMsg, _ := m["error"].(map[string]any); errMsg["code"] != "invalid_faq" {
			t.Errorf("%s: code 应为 invalid_faq，实际 %v", c.name, m["error"])
		}
	}
}

func TestFAQTestMatch_Endpoint(t *testing.T) {
	srv, _ := newTestServer(t)
	mux := http.NewServeMux()
	srv.Register(mux)
	// 先建 FAQ
	doRequest(t, mux, "POST", "/api/faqs", map[string]any{
		"question": "利率是多少", "answer": "4.5%", "keywords": "利率", "priority": 10,
	})
	// 测试精确匹配
	rec := doRequest(t, mux, "POST", "/api/faqs/test", map[string]any{"question": "利率是多少"})
	if rec.Code != http.StatusOK {
		t.Fatalf("应返回 200，实际 %d body=%s", rec.Code, rec.Body.String())
	}
	m := decodeBody(t, rec)
	if m["strategy"] != "exact" {
		t.Errorf("strategy 应为 exact，实际 %v", m["strategy"])
	}
	if m["hit"] != true {
		t.Errorf("精确匹配 hit 应为 true，实际 %v", m["hit"])
	}
	if m["score"] != 1.0 {
		t.Errorf("精确匹配 score 应为 1.0，实际 %v", m["score"])
	}
}

func TestFAQTestMatch_EmptyQuestion(t *testing.T) {
	srv, _ := newTestServer(t)
	mux := http.NewServeMux()
	srv.Register(mux)
	rec := doRequest(t, mux, "POST", "/api/faqs/test", map[string]any{"question": "   "})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("空问题应返回 400，实际 %d", rec.Code)
	}
}

func TestFAQImport_Endpoint(t *testing.T) {
	srv, _ := newTestServer(t)
	mux := http.NewServeMux()
	srv.Register(mux)
	rec := doRequest(t, mux, "POST", "/api/faqs/import", map[string]any{
		"items": []map[string]any{
			{"question": "Q1", "answer": "A1"},
			{"question": "", "answer": "X"}, // 失败
			{"question": "Q2", "answer": "A2"},
		},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("应返回 200，实际 %d", rec.Code)
	}
	m := decodeBody(t, rec)
	if m["success"] != float64(2) {
		t.Errorf("success 应为 2，实际 %v", m["success"])
	}
	if m["failed"] != float64(1) {
		t.Errorf("failed 应为 1，实际 %v", m["failed"])
	}
}

func TestFAQImport_Empty(t *testing.T) {
	srv, _ := newTestServer(t)
	mux := http.NewServeMux()
	srv.Register(mux)
	rec := doRequest(t, mux, "POST", "/api/faqs/import", map[string]any{"items": []any{}})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("空导入应返回 400，实际 %d", rec.Code)
	}
	m := decodeBody(t, rec)
	if errMsg, _ := m["error"].(map[string]any); errMsg["code"] != "empty_import" {
		t.Errorf("code 应为 empty_import，实际 %v", m["error"])
	}
}

// ===========================================================================
// 响应格式统一性
// ===========================================================================

func TestContentType_UTF8(t *testing.T) {
	srv, _ := newTestServer(t)
	mux := http.NewServeMux()
	srv.Register(mux)
	rec := doRequest(t, mux, "GET", "/api/health", nil)
	ct := rec.Header().Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Errorf("Content-Type 应含 application/json，实际 %s", ct)
	}
	if !strings.Contains(ct, "charset=utf-8") {
		t.Errorf("Content-Type 应含 charset=utf-8，实际 %s", ct)
	}
}

func TestErrorFormat_Unified(t *testing.T) {
	srv, _ := newTestServer(t)
	mux := http.NewServeMux()
	srv.Register(mux)
	// 触发一个 404
	rec := doRequest(t, mux, "GET", "/api/conversations/nope", nil)
	m := decodeBody(t, rec)
	errObj, ok := m["error"].(map[string]any)
	if !ok {
		t.Fatalf("error 应为对象，实际 %T", m["error"])
	}
	// 统一格式：必须含 code 和 message
	if errObj["code"] == nil {
		t.Error("error.code 不应为 nil")
	}
	if errObj["message"] == nil {
		t.Error("error.message 不应为 nil")
	}
}

// ===========================================================================
// System Info / Model 端点
// ===========================================================================

func TestSystemModel_Endpoint(t *testing.T) {
	srv, _ := newTestServer(t)
	mux := http.NewServeMux()
	srv.Register(mux)
	rec := doRequest(t, mux, "GET", "/api/system/model", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("应返回 200，实际 %d", rec.Code)
	}
	m := decodeBody(t, rec)
	if m["chatModel"] != "qwen3" {
		t.Errorf("chatModel 应为 qwen3，实际 %v", m["chatModel"])
	}
	if m["embeddingModel"] != "bge" {
		t.Errorf("embeddingModel 应为 bge，实际 %v", m["embeddingModel"])
	}
	if m["hasChatModel"] != true {
		t.Errorf("hasChatModel 应为 true，实际 %v", m["hasChatModel"])
	}
}
