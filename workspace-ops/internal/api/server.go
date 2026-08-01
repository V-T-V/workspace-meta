// Package api 实现 workspace-ops 的 REST API（Go 1.22 方法路由）。
// 端点：GET /api/projects / GET /api/scans / GET /api/scans/{id} / POST /api/rescan。
package api

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/QiuShichang/workspace-ops/internal/config"
	"github.com/QiuShichang/workspace-ops/internal/inspector"
	"github.com/QiuShichang/workspace-ops/internal/report"
	"github.com/QiuShichang/workspace-ops/internal/storage"
	"github.com/QiuShichang/workspace-ops/internal/workspace"
)

// Server 持有 scan/report 所需的依赖。
type Server struct {
	DB       *sql.DB
	Log      *slog.Logger
	Cfg      *config.Config
	Resolver *Resolver // 扫描逻辑注入（便于测试）
	// rootDir 是配置文件所在目录，作为 ResolveRoot 的基准。
	// serve 首启与 handleRescan 复用同一 rootDir，避免扫描到不同的工作区根。
	rootDir string
}

// Resolver 封装"扫描工作区"的核心流程，便于 serve 子命令复用。
type Resolver struct {
	Cfg       *config.Config
	Inspector *inspector.Inspector
}

// Scan 扫描工作区，把结果入库，返回项目数。
// rootDir 若非空，作为 ResolveRoot 的 config 基准目录（便于命令行注入）。
func (rs *Resolver) Scan(db *sql.DB, rootDir string) (int, error) {
	root, err := rs.Cfg.ResolveRoot(rootDir)
	if err != nil {
		return 0, err
	}
	projects, err := workspace.Discover(root, rs.Cfg.Scan.IgnoreDirs)
	if err != nil {
		return 0, err
	}
	scanID, err := storage.StartScan(db)
	if err != nil {
		return 0, err
	}
	facts := rs.Inspector.InspectAll(projects)
	for _, f := range facts {
		if err := storage.SaveFacts(db, scanID, f.Slug, f.Path, f.KV); err != nil {
			return 0, err
		}
	}
	if err := storage.FinishScan(db, scanID, len(projects), "done"); err != nil {
		return 0, err
	}
	return len(projects), nil
}

// NewServer 构造 Server。rootDir 是配置文件所在目录，作为 ResolveRoot 的基准
// （与 Resolver.Scan 的 rootDir 含义一致）。handleRescan 会复用它，保证
// serve 首启与后续 rescan 扫描同一个工作区根。
func NewServer(db *sql.DB, log *slog.Logger, cfg *config.Config, rootDir string) *Server {
	return &Server{
		DB:      db,
		Log:     log,
		Cfg:     cfg,
		rootDir: rootDir,
		Resolver: &Resolver{
			Cfg: cfg,
			Inspector: inspector.New(inspector.Checks{
				Stack:          cfg.Scan.Checks.Stack,
				Dependencies:   cfg.Scan.Checks.Dependencies,
				AgentsMD:       cfg.Scan.Checks.AgentsMD,
				GitStatus:      cfg.Scan.Checks.GitStatus,
				Tests:          cfg.Scan.Checks.Tests,
				BuildArtifacts: cfg.Scan.Checks.BuildArtifacts,
			}, "git"),
		},
	}
}

// Routes 返回 http.Handler（Go 1.22 方法路由）。
func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/projects", s.handleProjects)
	mux.HandleFunc("GET /api/scans", s.handleScans)
	mux.HandleFunc("POST /api/rescan", s.handleRescan)
	mux.HandleFunc("GET /api/test-runs", s.handleTestRuns)
	mux.HandleFunc("GET /api/health", s.handleHealth)
	return mux
}

// handleTestRuns 返回测试运行历史。
func (s *Server) handleTestRuns(w http.ResponseWriter, r *http.Request) {
	runs, err := storage.AllTestRuns(s.DB, 200)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	// 统计 pass/fail/skipped/error 分布
	summary := map[string]int{"pass": 0, "fail": 0, "skipped": 0, "timeout": 0, "error": 0}
	for _, tr := range runs {
		summary[tr.Status]++
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"test_runs":      runs,
		"total":          len(runs),
		"status_summary": summary,
	})
}

// handleProjects 返回全部项目（带 facts）。
func (s *Server) handleProjects(w http.ResponseWriter, r *http.Request) {
	projects, err := storage.AllProjects(s.DB)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	views := make([]report.ProjectView, 0, len(projects))
	stackSummary := map[string]int{}
	for _, p := range projects {
		facts, err := storage.ProjectFacts(s.DB, p.ID, 0)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		v := report.ProjectView{
			Slug:         p.Slug,
			StackPrimary: p.StackPrimary,
			StackAll:     facts["stack_all"],
			HasAgentsMD:  p.HasAgentsMD,
			GitBranch:    p.GitBranch,
			GitDirty:     p.GitDirty,
			TestCount:    facts["test_count"],
		}
		// 附带最近一次实跑测试状态（若已跑过 ops test）
		if tr, _ := storage.LatestTestRunForProject(s.DB, p.ID); tr != nil {
			v.TestStatus = tr.Status
			v.TestDuration = fmt.Sprintf("%dms", tr.DurationMs)
		}
		views = append(views, v)
		stack := p.StackPrimary
		if stack == "" {
			stack = "unknown"
		}
		stackSummary[stack]++
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"projects":      views,
		"project_count": len(views),
		"stack_summary": stackSummary,
	})
}

// handleScans 返回扫描历史。
func (s *Server) handleScans(w http.ResponseWriter, r *http.Request) {
	scans, err := storage.AllScans(s.DB)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"scans": scans})
}

// handleRescan 触发一次新扫描。复用 Server 启动时确定的 rootDir，
// 与 serve 首启扫描保持同一个工作区根。
func (s *Server) handleRescan(w http.ResponseWriter, r *http.Request) {
	count, err := s.Resolver.Scan(s.DB, s.rootDir)
	if err != nil {
		s.Log.Error("[api] rescan 失败", "err", err)
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"scanned": count, "status": "done"})
}

// handleHealth 健康检查。
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ===== 辅助 =====

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}
