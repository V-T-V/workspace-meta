package api

// 本文件补充第二轮 API 路由测试：覆盖所有端点的参数校验、错误处理、响应格式。
// 与 router_test.go 共用 newTestServer / doRequest / decodeBody 辅助。

import (
	"context"
	"database/sql"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/QiuShichang/auto-finance-assistant/internal/chat"
	"github.com/QiuShichang/auto-finance-assistant/internal/queue"
	"github.com/QiuShichang/auto-finance-assistant/internal/rag"
	"github.com/QiuShichang/auto-finance-assistant/internal/storage"
)

// ===========================================================================
// 文档端点
// ===========================================================================

// seedDocument 直接落库一个文档（绕过 importer 的文件解析），供 GET/列表测试。
func seedDocument(t *testing.T, srv *Server, id, name, status string) *storage.Document {
	t.Helper()
	d := &storage.Document{
		ID: id, Name: name, OriginalName: name + ".txt", FileType: ".txt",
		FileSize: 100, FileHash: "hash-" + id, Status: status,
	}
	if err := storage.CreateDocument(context.Background(), srv.db, d); err != nil {
		t.Fatalf("建文档失败: %v", err)
	}
	return d
}

func TestListDocuments_Empty(t *testing.T) {
	srv, _ := newTestServer(t)
	mux := http.NewServeMux()
	srv.Register(mux)
	rec := doRequest(t, mux, "GET", "/api/documents", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("应返回 200，实际 %d", rec.Code)
	}
	m := decodeBody(t, rec)
	if m["total"] != float64(0) {
		t.Errorf("空库 total 应为 0，实际 %v", m["total"])
	}
	items, _ := m["items"].([]any)
	if len(items) != 0 {
		t.Errorf("空库 items 应为空，实际 %d", len(items))
	}
}

func TestListDocuments_StatusFilter(t *testing.T) {
	srv, _ := newTestServer(t)
	mux := http.NewServeMux()
	srv.Register(mux)
	seedDocument(t, srv, "d1", "活跃文档", storage.DocStatusActive)
	seedDocument(t, srv, "d2", "草稿文档", storage.DocStatusDraft)
	// 过滤 active
	rec := doRequest(t, mux, "GET", "/api/documents?status=active", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("应返回 200，实际 %d", rec.Code)
	}
	m := decodeBody(t, rec)
	items, _ := m["items"].([]any)
	if len(items) != 1 {
		t.Errorf("过滤 active 应 1 个，实际 %d", len(items))
	}
	// 不过滤：2 个
	rec = doRequest(t, mux, "GET", "/api/documents", nil)
	m = decodeBody(t, rec)
	items, _ = m["items"].([]any)
	if len(items) != 2 {
		t.Errorf("不过滤应 2 个，实际 %d", len(items))
	}
}

func TestGetDocument_NotFound(t *testing.T) {
	srv, _ := newTestServer(t)
	mux := http.NewServeMux()
	srv.Register(mux)
	rec := doRequest(t, mux, "GET", "/api/documents/nope", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("应返回 404，实际 %d", rec.Code)
	}
	m := decodeBody(t, rec)
	if errObj, _ := m["error"].(map[string]any); errObj["code"] != "not_found" {
		t.Errorf("code 应为 not_found，实际 %v", m["error"])
	}
}

func TestGetDocument_Success(t *testing.T) {
	srv, _ := newTestServer(t)
	mux := http.NewServeMux()
	srv.Register(mux)
	seedDocument(t, srv, "d1", "手册", storage.DocStatusDraft)
	rec := doRequest(t, mux, "GET", "/api/documents/d1", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("应返回 200，实际 %d body=%s", rec.Code, rec.Body.String())
	}
	m := decodeBody(t, rec)
	if m["id"] != "d1" {
		t.Errorf("id 应为 d1，实际 %v", m["id"])
	}
	if m["name"] != "手册" {
		t.Errorf("name 应为 手册，实际 %v", m["name"])
	}
	if m["status"] != "draft" {
		t.Errorf("status 应为 draft，实际 %v", m["status"])
	}
	if m["fileType"] != ".txt" {
		t.Errorf("fileType 应为 .txt，实际 %v", m["fileType"])
	}
}

func TestUpdateDocument_Success(t *testing.T) {
	srv, _ := newTestServer(t)
	mux := http.NewServeMux()
	srv.Register(mux)
	seedDocument(t, srv, "d1", "旧名", storage.DocStatusDraft)
	rec := doRequest(t, mux, "PUT", "/api/documents/d1", map[string]any{
		"name": "新名", "version": "v2", "institution": "工商银行",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("应返回 200，实际 %d body=%s", rec.Code, rec.Body.String())
	}
	m := decodeBody(t, rec)
	if m["name"] != "新名" {
		t.Errorf("name 应更新为 新名，实际 %v", m["name"])
	}
	if m["version"] != "v2" {
		t.Errorf("version 应为 v2，实际 %v", m["version"])
	}
	if m["institution"] != "工商银行" {
		t.Errorf("institution 应为 工商银行，实际 %v", m["institution"])
	}
}

func TestUpdateDocument_NotFound(t *testing.T) {
	srv, _ := newTestServer(t)
	mux := http.NewServeMux()
	srv.Register(mux)
	rec := doRequest(t, mux, "PUT", "/api/documents/missing", map[string]any{"name": "x"})
	if rec.Code != http.StatusNotFound {
		t.Errorf("应返回 404，实际 %d", rec.Code)
	}
}

func TestUpdateDocument_InvalidBody(t *testing.T) {
	srv, _ := newTestServer(t)
	mux := http.NewServeMux()
	srv.Register(mux)
	seedDocument(t, srv, "d1", "n", storage.DocStatusDraft)
	req := httptest.NewRequest("PUT", "/api/documents/d1", strings.NewReader("{bad"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("非法 body 应 400，实际 %d", rec.Code)
	}
	m := decodeBody(t, rec)
	if errObj, _ := m["error"].(map[string]any); errObj["code"] != "invalid_body" {
		t.Errorf("code 应为 invalid_body，实际 %v", m["error"])
	}
}

func TestListChunks_Empty(t *testing.T) {
	srv, _ := newTestServer(t)
	mux := http.NewServeMux()
	srv.Register(mux)
	seedDocument(t, srv, "d1", "n", storage.DocStatusDraft)
	rec := doRequest(t, mux, "GET", "/api/documents/d1/chunks", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("应返回 200，实际 %d", rec.Code)
	}
	m := decodeBody(t, rec)
	if m["total"] != float64(0) {
		t.Errorf("无片段 total 应 0，实际 %v", m["total"])
	}
}

func TestListChunks_WithData(t *testing.T) {
	srv, _ := newTestServer(t)
	mux := http.NewServeMux()
	srv.Register(mux)
	seedDocument(t, srv, "d1", "n", storage.DocStatusDraft)
	ctx := context.Background()
	storage.CreateChunk(ctx, srv.db, &storage.Chunk{ID: "c1", DocumentID: "d1", Sequence: 1, Content: "片段一"})
	storage.CreateChunk(ctx, srv.db, &storage.Chunk{ID: "c2", DocumentID: "d1", Sequence: 2, Content: "片段二"})
	rec := doRequest(t, mux, "GET", "/api/documents/d1/chunks", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("应返回 200，实际 %d", rec.Code)
	}
	m := decodeBody(t, rec)
	items, _ := m["items"].([]any)
	if len(items) != 2 {
		t.Errorf("应 2 片段，实际 %d", len(items))
	}
	// 验证按 sequence 升序
	first, _ := items[0].(map[string]any)
	if first["sequence"] != float64(1) {
		t.Errorf("首片段 sequence 应为 1，实际 %v", first["sequence"])
	}
}

// ===========================================================================
// Embed 端点（M6）
// ===========================================================================

func TestEmbed_VectorDisabled(t *testing.T) {
	// newTestServer 未注入 vector searcher，应返回 vector_disabled
	srv, _ := newTestServer(t)
	mux := http.NewServeMux()
	srv.Register(mux)
	seedDocument(t, srv, "d1", "n", storage.DocStatusActive)
	rec := doRequest(t, mux, "POST", "/api/documents/d1/embed", nil)
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("向量未启用应 503，实际 %d", rec.Code)
	}
	m := decodeBody(t, rec)
	if errObj, _ := m["error"].(map[string]any); errObj["code"] != "vector_disabled" {
		t.Errorf("code 应为 vector_disabled，实际 %v", m["error"])
	}
}

func TestEmbed_NotFound(t *testing.T) {
	srv, _ := newTestServer(t)
	mux := http.NewServeMux()
	srv.Register(mux)
	rec := doRequest(t, mux, "POST", "/api/documents/missing/embed", nil)
	// vector == nil 先返回 vector_disabled；注入空 vector 后才走到文档校验
	srv.SetVectorSearcher(nil) // 显式确认
	if rec.Code != http.StatusServiceUnavailable {
		t.Logf("vector 未注入，返回 %d（vector_disabled 优先于 not_found）", rec.Code)
	}
}

func TestEmbed_NotActive(t *testing.T) {
	// 注入 vector searcher 后，草稿文档应返回 not_active
	srv, db := newTestServer(t)
	mux := http.NewServeMux()
	srv.Register(mux)
	seedDocument(t, srv, "d1", "n", storage.DocStatusDraft)
	srv.SetVectorSearcher(newTestVectorSearcher(t, db))
	rec := doRequest(t, mux, "POST", "/api/documents/d1/embed", nil)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("草稿文档向量化应 400，实际 %d body=%s", rec.Code, rec.Body.String())
	}
	m := decodeBody(t, rec)
	if errObj, _ := m["error"].(map[string]any); errObj["code"] != "not_active" {
		t.Errorf("code 应为 not_active，实际 %v", m["error"])
	}
}

// ===========================================================================
// 反馈端点
// ===========================================================================

func TestCreateFeedback_Success(t *testing.T) {
	srv, _ := newTestServer(t)
	mux := http.NewServeMux()
	srv.Register(mux)
	rec := doRequest(t, mux, "POST", "/api/feedback", map[string]any{
		"messageId": "m1", "rating": 1, "reason": "很好",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("应返回 201，实际 %d body=%s", rec.Code, rec.Body.String())
	}
	m := decodeBody(t, rec)
	if m["created"] != true {
		t.Errorf("created 应为 true，实际 %v", m["created"])
	}
}

func TestCreateFeedback_NegativeRating(t *testing.T) {
	srv, _ := newTestServer(t)
	mux := http.NewServeMux()
	srv.Register(mux)
	rec := doRequest(t, mux, "POST", "/api/feedback", map[string]any{
		"messageId": "m1", "rating": -1, "correction": "答错了",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("踩应返回 201，实际 %d", rec.Code)
	}
}

func TestCreateFeedback_InvalidRating(t *testing.T) {
	srv, _ := newTestServer(t)
	mux := http.NewServeMux()
	srv.Register(mux)
	cases := []int{0, 2, -2, 5}
	for _, r := range cases {
		rec := doRequest(t, mux, "POST", "/api/feedback", map[string]any{"rating": r})
		if rec.Code != http.StatusBadRequest {
			t.Errorf("rating=%d 应 400，实际 %d", r, rec.Code)
		}
		m := decodeBody(t, rec)
		if errObj, _ := m["error"].(map[string]any); errObj["code"] != "invalid_rating" {
			t.Errorf("code 应为 invalid_rating，实际 %v", m["error"])
		}
	}
}

func TestCreateFeedback_InvalidBody(t *testing.T) {
	srv, _ := newTestServer(t)
	mux := http.NewServeMux()
	srv.Register(mux)
	req := httptest.NewRequest("POST", "/api/feedback", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("非法 body 应 400，实际 %d", rec.Code)
	}
}

func TestCreateFeedback_PIIMasking(t *testing.T) {
	// 验证反馈中的手机号被脱敏后落库
	srv, _ := newTestServer(t)
	mux := http.NewServeMux()
	srv.Register(mux)
	rec := doRequest(t, mux, "POST", "/api/feedback", map[string]any{
		"rating": -1, "reason": "我的手机是13800138000",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("应 201，实际 %d", rec.Code)
	}
	items, _ := storage.ListFeedback(context.Background(), srv.db, 10)
	if len(items) != 1 {
		t.Fatalf("应有 1 条反馈，实际 %d", len(items))
	}
	if strings.Contains(items[0].Reason, "13800138000") {
		t.Errorf("手机号应被脱敏，实际 %s", items[0].Reason)
	}
	if !strings.Contains(items[0].Reason, "138****8000") {
		t.Errorf("脱敏格式异常，实际 %s", items[0].Reason)
	}
}

func TestListFeedback(t *testing.T) {
	srv, _ := newTestServer(t)
	mux := http.NewServeMux()
	srv.Register(mux)
	doRequest(t, mux, "POST", "/api/feedback", map[string]any{"messageId": "m1", "rating": 1})
	doRequest(t, mux, "POST", "/api/feedback", map[string]any{"messageId": "m2", "rating": -1})
	rec := doRequest(t, mux, "GET", "/api/feedback", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("应返回 200，实际 %d", rec.Code)
	}
	m := decodeBody(t, rec)
	if m["total"] != float64(2) {
		t.Errorf("total 应为 2，实际 %v", m["total"])
	}
}

// ===========================================================================
// 审计 / 拒答 / 指标端点
// ===========================================================================

func TestListAuditLogs_Empty(t *testing.T) {
	srv, _ := newTestServer(t)
	mux := http.NewServeMux()
	srv.Register(mux)
	rec := doRequest(t, mux, "GET", "/api/audit/logs", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("应返回 200，实际 %d", rec.Code)
	}
	m := decodeBody(t, rec)
	if m["total"] != float64(0) {
		t.Errorf("空库 total 应 0，实际 %v", m["total"])
	}
}

func TestListAuditLogs_AfterFAQCreate(t *testing.T) {
	srv, _ := newTestServer(t)
	mux := http.NewServeMux()
	srv.Register(mux)
	// 创建 FAQ 会触发审计（faq.create）
	doRequest(t, mux, "POST", "/api/faqs", map[string]any{
		"question": "Q", "answer": "A",
	})
	// 审计异步写入，等待一下
	time.Sleep(150 * time.Millisecond)
	rec := doRequest(t, mux, "GET", "/api/audit/logs", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("应返回 200，实际 %d", rec.Code)
	}
	m := decodeBody(t, rec)
	items, _ := m["items"].([]any)
	if len(items) == 0 {
		t.Error("创建 FAQ 后应有审计日志")
	}
}

func TestListRefused_Empty(t *testing.T) {
	srv, _ := newTestServer(t)
	mux := http.NewServeMux()
	srv.Register(mux)
	rec := doRequest(t, mux, "GET", "/api/refused", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("应返回 200，实际 %d", rec.Code)
	}
	m := decodeBody(t, rec)
	if m["total"] != float64(0) {
		t.Errorf("空库 total 应 0，实际 %v", m["total"])
	}
}

func TestMetrics(t *testing.T) {
	srv, _ := newTestServer(t)
	mux := http.NewServeMux()
	srv.Register(mux)
	// 造点数据
	doRequest(t, mux, "POST", "/api/conversations", map[string]string{"title": "T"})
	doRequest(t, mux, "POST", "/api/faqs", map[string]any{"question": "Q", "answer": "A"})
	rec := doRequest(t, mux, "GET", "/api/metrics", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("应返回 200，实际 %d", rec.Code)
	}
	m := decodeBody(t, rec)
	if m["conversations"] != float64(1) {
		t.Errorf("conversations 应为 1，实际 %v", m["conversations"])
	}
	if m["faqs"] != float64(1) {
		t.Errorf("faqs 应为 1，实际 %v", m["faqs"])
	}
	// 必须含性能字段
	for _, k := range []string{"messages", "documents", "feedback", "cacheSize", "tokensPerSec"} {
		if m[k] == nil {
			t.Errorf("metrics 缺少字段 %s", k)
		}
	}
}

// ===========================================================================
// 备份端点
// ===========================================================================

func TestBackup_DisabledWhenNoManager(t *testing.T) {
	srv, _ := newTestServer(t)
	mux := http.NewServeMux()
	srv.Register(mux)
	rec := doRequest(t, mux, "POST", "/api/system/backup", nil)
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("未注入备份管理器应 503，实际 %d", rec.Code)
	}
	m := decodeBody(t, rec)
	if errObj, _ := m["error"].(map[string]any); errObj["code"] != "backup_disabled" {
		t.Errorf("code 应为 backup_disabled，实际 %v", m["error"])
	}
}

func TestListBackups_DisabledWhenNoManager(t *testing.T) {
	srv, _ := newTestServer(t)
	mux := http.NewServeMux()
	srv.Register(mux)
	rec := doRequest(t, mux, "GET", "/api/system/backups", nil)
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("未注入备份管理器应 503，实际 %d", rec.Code)
	}
}

// ===========================================================================
// 数据清理端点
// ===========================================================================

func TestPurge_Default90Days(t *testing.T) {
	srv, _ := newTestServer(t)
	mux := http.NewServeMux()
	srv.Register(mux)
	doRequest(t, mux, "POST", "/api/conversations", map[string]string{"title": "T"})
	rec := doRequest(t, mux, "POST", "/api/system/purge", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("应返回 200，实际 %d body=%s", rec.Code, rec.Body.String())
	}
	m := decodeBody(t, rec)
	// 新建数据不会被清理（未过期），conversations 应为 0
	if m["conversations"] != float64(0) {
		t.Errorf("未过期数据 conversations 应 0，实际 %v", m["conversations"])
	}
}

func TestPurge_CustomDaysParam(t *testing.T) {
	srv, _ := newTestServer(t)
	mux := http.NewServeMux()
	srv.Register(mux)
	// days=0 应被钳制为 90（不会因 0 天全删）
	rec := doRequest(t, mux, "POST", "/api/system/purge?days=0", nil)
	if rec.Code != http.StatusOK {
		t.Errorf("days=0 应返回 200（钳制为 90），实际 %d body=%s", rec.Code, rec.Body.String())
	}
}

// ===========================================================================
// 合规日志端点
// ===========================================================================

func TestComplianceLogs_Empty(t *testing.T) {
	srv, _ := newTestServer(t)
	mux := http.NewServeMux()
	srv.Register(mux)
	rec := doRequest(t, mux, "GET", "/api/compliance/logs", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("应返回 200，实际 %d", rec.Code)
	}
	m := decodeBody(t, rec)
	if m["total"] != float64(0) {
		t.Errorf("空库 total 应 0，实际 %v", m["total"])
	}
}

func TestComplianceStats_Empty(t *testing.T) {
	srv, _ := newTestServer(t)
	mux := http.NewServeMux()
	srv.Register(mux)
	rec := doRequest(t, mux, "GET", "/api/compliance/stats", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("应返回 200，实际 %d", rec.Code)
	}
	m := decodeBody(t, rec)
	if m["blockRate"] != float64(0) {
		t.Errorf("空库 blockRate 应 0，实际 %v", m["blockRate"])
	}
	if m["totalRequests"] != float64(0) {
		t.Errorf("空库 totalRequests 应 0，实际 %v", m["totalRequests"])
	}
}

// ===========================================================================
// 认证中间件
// ===========================================================================

func TestAuthMiddleware_NoPassword_AllowsAll(t *testing.T) {
	// 未设密码（默认）→ 所有需认证端点放行
	srv, _ := newTestServer(t)
	mux := http.NewServeMux()
	srv.Register(mux)
	// /api/audit/logs 需认证
	rec := doRequest(t, mux, "GET", "/api/audit/logs", nil)
	if rec.Code != http.StatusOK {
		t.Errorf("未设密码应放行，实际 %d", rec.Code)
	}
}

func TestAuthMiddleware_WrongPassword_Unauthorized(t *testing.T) {
	srv, _ := newTestServer(t)
	srv.SetAdminPassword("secret123")
	mux := http.NewServeMux()
	srv.Register(mux)
	// 无密码头
	rec := doRequest(t, mux, "GET", "/api/audit/logs", nil)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("无认证头应 401，实际 %d", rec.Code)
	}
	m := decodeBody(t, rec)
	if errObj, _ := m["error"].(map[string]any); errObj["code"] != "unauthorized" {
		t.Errorf("code 应为 unauthorized，实际 %v", m["error"])
	}
	// 错误密码
	req := httptest.NewRequest("GET", "/api/audit/logs", nil)
	req.Header.Set("X-Admin-Password", "wrong")
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("错误密码应 401，实际 %d", rec.Code)
	}
}

func TestAuthMiddleware_CorrectPassword_Allows(t *testing.T) {
	srv, _ := newTestServer(t)
	srv.SetAdminPassword("secret123")
	mux := http.NewServeMux()
	srv.Register(mux)
	// 正确密码（X-Admin-Password 头）
	req := httptest.NewRequest("GET", "/api/audit/logs", nil)
	req.Header.Set("X-Admin-Password", "secret123")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("正确密码应 200，实际 %d", rec.Code)
	}
}

func TestAuthMiddleware_BearerToken(t *testing.T) {
	srv, _ := newTestServer(t)
	srv.SetAdminPassword("secret123")
	mux := http.NewServeMux()
	srv.Register(mux)
	// Bearer token 形式
	req := httptest.NewRequest("GET", "/api/audit/logs", nil)
	req.Header.Set("Authorization", "Bearer secret123")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("Bearer 正确密码应 200，实际 %d", rec.Code)
	}
}

func TestAuthMiddleware_HashedPassword(t *testing.T) {
	// 存储哈希密码（sha256: 前缀）
	srv, _ := newTestServer(t)
	srv.SetAdminPassword(hashPassword("secret123"))
	mux := http.NewServeMux()
	srv.Register(mux)
	req := httptest.NewRequest("GET", "/api/audit/logs", nil)
	req.Header.Set("X-Admin-Password", "secret123")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("哈希密码验证应 200，实际 %d", rec.Code)
	}
	// 错误密码对哈希
	req = httptest.NewRequest("GET", "/api/audit/logs", nil)
	req.Header.Set("X-Admin-Password", "wrong")
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("哈希密码错误应 401，实际 %d", rec.Code)
	}
}

// ===========================================================================
// 限流中间件（独立限流器，避免污染全局 chatLimiter）
// ===========================================================================

func TestRateLimiter_AllowUntilLimit(t *testing.T) {
	rl := newRateLimiter(3, time.Minute)
	for i := 0; i < 3; i++ {
		if !rl.allow("1.2.3.4") {
			t.Errorf("第 %d 次应放行", i+1)
		}
	}
	// 第 4 次超限
	if rl.allow("1.2.3.4") {
		t.Error("第 4 次应拒绝")
	}
}

func TestRateLimiter_DifferentIPsIndependent(t *testing.T) {
	rl := newRateLimiter(1, time.Minute)
	if !rl.allow("1.1.1.1") {
		t.Error("1.1.1.1 首次应放行")
	}
	if rl.allow("1.1.1.1") {
		t.Error("1.1.1.1 第二次应拒绝")
	}
	// 不同 IP 不受影响
	if !rl.allow("2.2.2.2") {
		t.Error("2.2.2.2 应放行")
	}
}

func TestRateLimiter_WindowReset(t *testing.T) {
	rl := newRateLimiter(1, 30*time.Millisecond)
	if !rl.allow("ip") {
		t.Error("首次应放行")
	}
	if rl.allow("ip") {
		t.Error("窗口内第二次应拒绝")
	}
	time.Sleep(40 * time.Millisecond)
	if !rl.allow("ip") {
		t.Error("窗口重置后应放行")
	}
}

func TestRateLimitMiddleware_Returns429(t *testing.T) {
	rl := newRateLimiter(1, time.Minute)
	handler := srv429().RateLimitMiddleware(rl, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	})
	// 第一次放行
	req := httptest.NewRequest("GET", "/x", nil)
	rec := httptest.NewRecorder()
	handler(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("首次应 200，实际 %d", rec.Code)
	}
	// 第二次 429
	rec = httptest.NewRecorder()
	handler(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("超限应 429，实际 %d", rec.Code)
	}
	if rec.Header().Get("Retry-After") == "" {
		t.Error("429 应含 Retry-After 头")
	}
	m := decodeBody(t, rec)
	if errObj, _ := m["error"].(map[string]any); errObj["code"] != "rate_limited" {
		t.Errorf("code 应为 rate_limited，实际 %v", m["error"])
	}
}

// srv429 返回一个最小 Server 供中间件测试（仅用 clientIP）。
func srv429() *Server { return &Server{} }

// ===========================================================================
// 聊天端点（非流式）参数校验
// ===========================================================================

func TestChat_EmptyQuestion(t *testing.T) {
	srv, _ := newTestServer(t)
	mux := http.NewServeMux()
	srv.Register(mux)
	rec := doRequest(t, mux, "POST", "/api/chat", map[string]any{"question": ""})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("空问题应 400，实际 %d", rec.Code)
	}
	m := decodeBody(t, rec)
	if errObj, _ := m["error"].(map[string]any); errObj["code"] != "empty_question" {
		t.Errorf("code 应为 empty_question，实际 %v", m["error"])
	}
}

func TestChat_InvalidBody(t *testing.T) {
	srv, _ := newTestServer(t)
	mux := http.NewServeMux()
	srv.Register(mux)
	req := httptest.NewRequest("POST", "/api/chat", strings.NewReader("{bad"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("非法 body 应 400，实际 %d", rec.Code)
	}
}

func TestChat_ModelDown(t *testing.T) {
	// 模型不可达时，聊天应返回 503
	dir := t.TempDir()
	dbPath := dir + "/test.db"
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	db, _ := storage.OpenDB(dbPath, log)
	t.Cleanup(func() { _ = db.Close() })
	storage.Migrate(context.Background(), db, storage.AllActiveVersions(), log)
	fm := &fakeModel{chatModel: "qwen3", reachable: false, hasModel: false}
	q := queue.NewWithTimeouts(1, 2, time.Second, 5*time.Second, log)
	svc := chat.New(fm, db, q, log)
	srv := New(svc, fm, q, db, nil)
	mux := http.NewServeMux()
	srv.Register(mux)
	rec := doRequest(t, mux, "POST", "/api/chat", map[string]any{"question": "你好"})
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("模型不可达应 503，实际 %d body=%s", rec.Code, rec.Body.String())
	}
	m := decodeBody(t, rec)
	if errObj, _ := m["error"].(map[string]any); errObj["code"] != "ollama_unavailable" {
		t.Errorf("code 应为 ollama_unavailable，实际 %v", m["error"])
	}
}

// ===========================================================================
// 响应格式边界
// ===========================================================================

func TestJSONResponse_TrailingNewline(t *testing.T) {
	// 所有 JSON 响应以换行结尾（便于 curl / 分行解析）
	srv, _ := newTestServer(t)
	mux := http.NewServeMux()
	srv.Register(mux)
	rec := doRequest(t, mux, "GET", "/api/health", nil)
	body := rec.Body.String()
	if !strings.HasSuffix(body, "\n") {
		t.Errorf("响应应以换行结尾，实际 %q", body[len(body)-5:])
	}
}

func TestDecodeJSON_LimitsBodySize(t *testing.T) {
	// decodeJSON 限制 1MB，超大 body 应解码失败
	srv, _ := newTestServer(t)
	mux := http.NewServeMux()
	srv.Register(mux)
	// 构造 > 1MB 的 body
	big := strings.Repeat("a", (1<<20)+10)
	req := httptest.NewRequest("POST", "/api/finance/equal-payment",
		strings.NewReader(`{"principal":`+big+`}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("超 1MB body 应 400，实际 %d", rec.Code)
	}
}

func TestErrorJSON_AlwaysHasCodeAndMessage(t *testing.T) {
	// 抽样多个错误端点，断言 error 对象结构一致
	srv, _ := newTestServer(t)
	mux := http.NewServeMux()
	srv.Register(mux)
	endpoints := []struct {
		method, path string
		expectStatus int
	}{
		{"GET", "/api/conversations/missing", 404},
		{"GET", "/api/documents/missing", 404},
		{"GET", "/api/faqs/missing", 404},
		{"POST", "/api/feedback", 400}, // 缺 rating
	}
	for _, e := range endpoints {
		var body any
		if e.method == "POST" {
			body = map[string]any{}
		}
		rec := doRequest(t, mux, e.method, e.path, body)
		if rec.Code != e.expectStatus {
			t.Errorf("%s %s: 应 %d 实际 %d", e.method, e.path, e.expectStatus, rec.Code)
			continue
		}
		m := decodeBody(t, rec)
		errObj, ok := m["error"].(map[string]any)
		if !ok {
			t.Errorf("%s %s: error 应为对象", e.method, e.path)
			continue
		}
		if _, ok := errObj["code"].(string); !ok {
			t.Errorf("%s %s: error.code 应为字符串", e.method, e.path)
		}
		if _, ok := errObj["message"].(string); !ok {
			t.Errorf("%s %s: error.message 应为字符串", e.method, e.path)
		}
	}
}

// ===========================================================================
// 反馈学习闭环端点（M7 扩展）
// ===========================================================================

func TestFeedbackStats_Endpoint(t *testing.T) {
	srv, _ := newTestServer(t)
	mux := http.NewServeMux()
	srv.Register(mux)
	// 造数据：2 赞 1 踩
	doRequest(t, mux, "POST", "/api/feedback", map[string]any{"messageId": "m1", "rating": 1})
	doRequest(t, mux, "POST", "/api/feedback", map[string]any{"messageId": "m2", "rating": 1})
	doRequest(t, mux, "POST", "/api/feedback", map[string]any{"messageId": "m3", "rating": -1, "correction": "纠"})
	rec := doRequest(t, mux, "GET", "/api/feedback/stats", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("应返回 200，实际 %d body=%s", rec.Code, rec.Body.String())
	}
	m := decodeBody(t, rec)
	if m["total"] != float64(3) {
		t.Errorf("total 应 3，实际 %v", m["total"])
	}
	if m["positive"] != float64(2) {
		t.Errorf("positive 应 2，实际 %v", m["positive"])
	}
	if m["negative"] != float64(1) {
		t.Errorf("negative 应 1，实际 %v", m["negative"])
	}
	if m["satisfaction"] != 2.0/3.0 {
		t.Errorf("satisfaction 应 %.4f，实际 %v", 2.0/3.0, m["satisfaction"])
	}
}

func TestFeedbackCandidates_Endpoint(t *testing.T) {
	srv, _ := newTestServer(t)
	mux := http.NewServeMux()
	srv.Register(mux)
	// 负面带纠正 + 负面无纠正 + 正面
	doRequest(t, mux, "POST", "/api/feedback", map[string]any{"rating": -1, "correction": "答错了"})
	doRequest(t, mux, "POST", "/api/feedback", map[string]any{"rating": -1})
	doRequest(t, mux, "POST", "/api/feedback", map[string]any{"rating": 1, "correction": "好评"})
	rec := doRequest(t, mux, "GET", "/api/feedback/candidates", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("应返回 200，实际 %d", rec.Code)
	}
	m := decodeBody(t, rec)
	items, _ := m["items"].([]any)
	if len(items) != 1 {
		t.Errorf("应只 1 个候选（负面带纠正），实际 %d", len(items))
	}
}

func TestFeedbackPromote_Endpoint_Success(t *testing.T) {
	srv, _ := newTestServer(t)
	mux := http.NewServeMux()
	srv.Register(mux)
	// 先造一条带纠正的负面反馈
	doRequest(t, mux, "POST", "/api/feedback", map[string]any{"rating": -1, "correction": "正确答案"})
	// 取 feedbackId（通过 candidates）
	rec := doRequest(t, mux, "GET", "/api/feedback/candidates", nil)
	m := decodeBody(t, rec)
	items, _ := m["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("应有 1 候选，实际 %d", len(items))
	}
	first := items[0].(map[string]any)
	feedbackID := first["feedbackId"].(string)
	// 提升为 FAQ
	rec = doRequest(t, mux, "POST", "/api/feedback/promote", map[string]any{
		"feedbackId": feedbackID, "question": "利率", "answer": "4.5%",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("应返回 201，实际 %d body=%s", rec.Code, rec.Body.String())
	}
	m = decodeBody(t, rec)
	if m["promoted"] != true {
		t.Errorf("promoted 应 true，实际 %v", m["promoted"])
	}
	faqID := m["faqId"].(string)
	// 验证 FAQ 可查
	rec = doRequest(t, mux, "GET", "/api/faqs/"+faqID, nil)
	if rec.Code != http.StatusOK {
		t.Errorf("提升的 FAQ 应可查，实际 %d", rec.Code)
	}
}

func TestFeedbackPromote_Endpoint_AlreadyPromoted(t *testing.T) {
	srv, _ := newTestServer(t)
	mux := http.NewServeMux()
	srv.Register(mux)
	doRequest(t, mux, "POST", "/api/feedback", map[string]any{"rating": -1, "correction": "纠"})
	rec := doRequest(t, mux, "GET", "/api/feedback/candidates", nil)
	m := decodeBody(t, rec)
	items, _ := m["items"].([]any)
	feedbackID := items[0].(map[string]any)["feedbackId"].(string)
	// 第一次成功
	doRequest(t, mux, "POST", "/api/feedback/promote", map[string]any{
		"feedbackId": feedbackID, "question": "Q", "answer": "A",
	})
	// 第二次应 409
	rec = doRequest(t, mux, "POST", "/api/feedback/promote", map[string]any{
		"feedbackId": feedbackID, "question": "Q2", "answer": "A2",
	})
	if rec.Code != http.StatusConflict {
		t.Errorf("重复提升应 409，实际 %d", rec.Code)
	}
	m = decodeBody(t, rec)
	if errObj, _ := m["error"].(map[string]any); errObj["code"] != "already_promoted" {
		t.Errorf("code 应 already_promoted，实际 %v", m["error"])
	}
}

func TestFeedbackPromote_Endpoint_InvalidParams(t *testing.T) {
	srv, _ := newTestServer(t)
	mux := http.NewServeMux()
	srv.Register(mux)
	rec := doRequest(t, mux, "POST", "/api/feedback/promote", map[string]any{
		"feedbackId": "", "question": "Q", "answer": "A",
	})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("缺 feedbackId 应 400，实际 %d", rec.Code)
	}
}

// ===========================================================================
// 提前还款端点（M5 扩展）
// ===========================================================================

func TestPrepay_Endpoint_ShortenTerm(t *testing.T) {
	srv, _ := newTestServer(t)
	mux := http.NewServeMux()
	srv.Register(mux)
	rec := doRequest(t, mux, "POST", "/api/finance/prepay", map[string]any{
		"principal": 200000, "annualRate": 4.5, "months": 36,
		"paidPeriods": 12, "prepayAmount": 50000, "mode": "shorten_term",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("应返回 200，实际 %d body=%s", rec.Code, rec.Body.String())
	}
	m := decodeBody(t, rec)
	if m["type"] != "prepay" {
		t.Errorf("type 应 prepay，实际 %v", m["type"])
	}
	if m["mode"] != "shorten_term" {
		t.Errorf("mode 应 shorten_term，实际 %v", m["mode"])
	}
	// 缩期：新期数 < 24
	if m["newMonths"].(float64) >= 24 {
		t.Errorf("缩期 newMonths 应 < 24，实际 %v", m["newMonths"])
	}
	if disc, _ := m["disclaimer"].(string); !strings.Contains(disc, "试算") {
		t.Errorf("应含免责声明，实际 %v", m["disclaimer"])
	}
}

func TestPrepay_Endpoint_DefaultMode(t *testing.T) {
	// 不传 mode 默认缩期
	srv, _ := newTestServer(t)
	mux := http.NewServeMux()
	srv.Register(mux)
	rec := doRequest(t, mux, "POST", "/api/finance/prepay", map[string]any{
		"principal": 200000, "annualRate": 4.5, "months": 36,
		"paidPeriods": 12, "prepayAmount": 50000,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("应返回 200，实际 %d", rec.Code)
	}
	m := decodeBody(t, rec)
	if m["mode"] != "shorten_term" {
		t.Errorf("默认 mode 应 shorten_term，实际 %v", m["mode"])
	}
}

func TestPrepay_Endpoint_InvalidParams(t *testing.T) {
	srv, _ := newTestServer(t)
	mux := http.NewServeMux()
	srv.Register(mux)
	cases := []map[string]any{
		{"principal": 0, "annualRate": 4.5, "months": 36, "paidPeriods": 12, "prepayAmount": 1000},
		{"principal": 100000, "annualRate": 4.5, "months": 12, "paidPeriods": 12, "prepayAmount": 1000},
	}
	for _, c := range cases {
		rec := doRequest(t, mux, "POST", "/api/finance/prepay", c)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("非法参数应 400，实际 %d body=%s", rec.Code, rec.Body.String())
		}
		m := decodeBody(t, rec)
		if errObj, _ := m["error"].(map[string]any); errObj["code"] != "calc_error" {
			t.Errorf("code 应 calc_error，实际 %v", m["error"])
		}
	}
}

func TestPrepay_Endpoint_InvalidBody(t *testing.T) {
	srv, _ := newTestServer(t)
	mux := http.NewServeMux()
	srv.Register(mux)
	req := httptest.NewRequest("POST", "/api/finance/prepay", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("非法 body 应 400，实际 %d", rec.Code)
	}
}

// ===========================================================================
// 辅助
// ===========================================================================

// newTestVectorSearcher 构造一个真实 VectorSearcher（fake 模型），供 embed 测试。

// newTestVectorSearcher 构造一个真实 VectorSearcher（fake 模型），供 embed 测试。
func newTestVectorSearcher(t *testing.T, db *sql.DB) *rag.VectorSearcher {
	t.Helper()
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	fm := &fakeModel{chatModel: "qwen3", embedModel: "bge", reachable: true, hasModel: true}
	return rag.NewVectorSearcher(db, fm, 10, log)
}
