package api

// 第八轮：健康检查 + 监控端点测试。
// 覆盖 livez（存活探针）、readyz（就绪探针，DB+模型双就绪）、ping（延迟测量）。

import (
	"context"
	"net/http"
	"os"
	"testing"
	"time"

	"log/slog"

	"github.com/QiuShichang/auto-finance-assistant/internal/chat"
	"github.com/QiuShichang/auto-finance-assistant/internal/queue"
	"github.com/QiuShichang/auto-finance-assistant/internal/storage"
)

func TestLivez_Healthy(t *testing.T) {
	srv, _ := newTestServer(t)
	mux := http.NewServeMux()
	srv.Register(mux)
	rec := doRequest(t, mux, "GET", "/api/health/livez", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("DB 可 ping 应 200，实际 %d", rec.Code)
	}
	m := decodeBody(t, rec)
	if m["status"] != "alive" {
		t.Errorf("status 应 alive，实际 %v", m["status"])
	}
	db, _ := m["database"].(map[string]any)
	if db["ok"] != true {
		t.Errorf("database.ok 应 true，实际 %v", db["ok"])
	}
	if db["latencyMs"] == nil {
		t.Error("应含 latencyMs")
	}
	if m["uptimeSec"] == nil {
		t.Error("应含 uptimeSec")
	}
}

func TestLivez_IncludesVersion(t *testing.T) {
	srv, _ := newTestServer(t)
	mux := http.NewServeMux()
	srv.Register(mux)
	rec := doRequest(t, mux, "GET", "/api/health/livez", nil)
	m := decodeBody(t, rec)
	if m["version"] == nil {
		t.Error("应含 version")
	}
}

func TestReadyz_ReadyWhenModelAndDBUp(t *testing.T) {
	srv, _ := newTestServer(t)
	mux := http.NewServeMux()
	srv.Register(mux)
	rec := doRequest(t, mux, "GET", "/api/health/readyz", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("DB+模型就绪应 200，实际 %d", rec.Code)
	}
	m := decodeBody(t, rec)
	if m["status"] != "ready" {
		t.Errorf("status 应 ready，实际 %v", m["status"])
	}
	components, _ := m["components"].(map[string]any)
	db, _ := components["database"].(map[string]any)
	if db["ok"] != true {
		t.Errorf("database.ok 应 true，实际 %v", db["ok"])
	}
	model, _ := components["model"].(map[string]any)
	if model["reachable"] != true {
		t.Errorf("model.reachable 应 true，实际 %v", model["reachable"])
	}
}

// downServer 构造一个模型不可达的 Server（inline，复用 router_test 模式）。
func downServer(t *testing.T) *Server {
	t.Helper()
	dir := t.TempDir()
	dbPath := dir + "/test.db"
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	db, _ := storage.OpenDB(dbPath, log)
	t.Cleanup(func() { _ = db.Close() })
	storage.Migrate(context.Background(), db, storage.AllActiveVersions(), log)
	fm := &fakeModel{chatModel: "qwen3", reachable: false, hasModel: false}
	q := queue.NewWithTimeouts(1, 2, time.Second, 5*time.Second, log)
	svc := chat.New(fm, db, q, log)
	return New(svc, fm, q, db, nil)
}

func TestReadyz_NotReadyWhenModelDown(t *testing.T) {
	srv := downServer(t)
	mux := http.NewServeMux()
	srv.Register(mux)
	rec := doRequest(t, mux, "GET", "/api/health/readyz", nil)
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("模型不可达应 503，实际 %d", rec.Code)
	}
	m := decodeBody(t, rec)
	if m["status"] != "not_ready" {
		t.Errorf("status 应 not_ready，实际 %v", m["status"])
	}
}

func TestPing_ReturnsLatency(t *testing.T) {
	srv, _ := newTestServer(t)
	mux := http.NewServeMux()
	srv.Register(mux)
	rec := doRequest(t, mux, "GET", "/api/health/ping", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("应 200，实际 %d", rec.Code)
	}
	m := decodeBody(t, rec)
	db, _ := m["database"].(map[string]any)
	if db["latencyMs"] == nil {
		t.Error("database 应含 latencyMs")
	}
	if db["ok"] != true {
		t.Errorf("database.ok 应 true，实际 %v", db["ok"])
	}
	model, _ := m["model"].(map[string]any)
	if model["latencyMs"] == nil {
		t.Error("model 应含 latencyMs")
	}
	if m["timestamp"] == nil {
		t.Error("应含 timestamp")
	}
}

func TestPing_ModelDown_Still200(t *testing.T) {
	// ping 即使模型不可达也返回 200（探测结果包含 reachable=false）
	srv := downServer(t)
	mux := http.NewServeMux()
	srv.Register(mux)
	rec := doRequest(t, mux, "GET", "/api/health/ping", nil)
	if rec.Code != http.StatusOK {
		t.Errorf("ping 应始终 200（探测结果），实际 %d", rec.Code)
	}
	m := decodeBody(t, rec)
	model, _ := m["model"].(map[string]any)
	if model["reachable"] != false {
		t.Errorf("模型不可达时 reachable 应 false，实际 %v", model["reachable"])
	}
}

func TestRound2(t *testing.T) {
	cases := map[float64]float64{
		1.234:  1.23,
		1.235:  1.24,
		0.005:  0.01,
		10.999: 11,
		0:      0,
	}
	for in, want := range cases {
		if got := round2(in); got != want {
			t.Errorf("round2(%v) = %v, want %v", in, got, want)
		}
	}
}

func TestSetStartTime(t *testing.T) {
	// SetStartTime 应更新 startTime（烟雾测试）
	orig := startTime
	defer func() { startTime = orig }()
	custom := orig.Add(-100 * time.Second) // 100s 前
	SetStartTime(custom)
	if !startTime.Equal(custom) {
		t.Error("SetStartTime 应更新 startTime")
	}
}
