// Command server 是 flow-pipe 的服务入口。
//
// 用法：
//
//	flow-pipe -config config.dev.yaml                 # 启动 REST API 服务
//	flow-pipe -config config.dev.yaml -run pipe.yaml  # 跑一次管道后退出
//	flow-pipe -dot pipe.yaml > pipeline.dot           # 导出管道 DAG 为 Graphviz DOT
//	flow-pipe -version
//
// 匿名 import 连接器包以触发 init() 注册（source/transform/sink）。
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/QiuShichang/flow-pipe/internal/config"
	"github.com/QiuShichang/flow-pipe/internal/logging"
	"github.com/QiuShichang/flow-pipe/internal/pipeline"
	"github.com/QiuShichang/flow-pipe/internal/scheduler"
	_ "github.com/QiuShichang/flow-pipe/internal/sink"
	_ "github.com/QiuShichang/flow-pipe/internal/source"
	"github.com/QiuShichang/flow-pipe/internal/storage"
	_ "github.com/QiuShichang/flow-pipe/internal/transform"
)

// version 由 -ldflags 注入（见 Makefile LDFLAGS）。
var version = "dev"

func main() {
	configPath := flag.String("config", "config.dev.yaml", "配置文件路径（YAML）")
	runPath := flag.String("run", "", "跑一次指定管道文件后退出（不启服务）")
	recover := flag.Bool("recover", false, "配合 -run 使用：跳过最近一次已成功完成的步骤（状态恢复）")
	schedulePath := flag.String("schedule", "", "定时循环跑指定管道文件（按 -interval 间隔，Ctrl+C 退出）")
	intervalSec := flag.Int("interval", 60, "-schedule 的触发间隔（秒）")
	dotPath := flag.String("dot", "", "把指定管道文件导出为 Graphviz DOT 到 stdout（可重定向到 .dot 文件）")
	showVersion := flag.Bool("version", false, "打印版本号")
	flag.Parse()

	if *showVersion {
		fmt.Println("flow-pipe", version)
		return
	}

	// -dot 模式：读管道 YAML，把 DAG 导出为 Graphviz DOT 到 stdout。
	// 此模式不需要数据库/配置文件，所以放在加载配置之前。
	if *dotPath != "" {
		if err := exportDOT(*dotPath); err != nil {
			fmt.Fprintln(os.Stderr, "错误:", err)
			os.Exit(1)
		}
		return
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[warn] 加载配置 %s 失败（%v），用默认配置\n", *configPath, err)
		cfg = config.Default()
	}
	log := logging.New(cfg.Logging.Level, cfg.Logging.Format)

	dbPath, err := cfg.ResolveDBPath()
	if err != nil {
		exit(log, err)
	}
	db, err := storage.Open(dbPath)
	if err != nil {
		exit(log, err)
	}
	defer func() { _ = db.Close() }()

	// -run 模式：跑一次管道后退出。
	if *runPath != "" {
		if err := runOnce(db, *runPath, *recover, log); err != nil {
			exit(log, err)
		}
		return
	}

	// -schedule 模式：用 scheduler 定时循环跑管道（让 scheduler 包真正接线）。
	if *schedulePath != "" {
		if err := runScheduled(db, *schedulePath, *intervalSec, log); err != nil {
			exit(log, err)
		}
		return
	}

	// 否则启动 REST 服务。
	if err := runServer(db, cfg, log); err != nil {
		exit(log, err)
	}
}

func exit(log *slog.Logger, err error) {
	log.Error("[main] 失败", "err", err)
	fmt.Fprintln(os.Stderr, "错误:", err)
	os.Exit(1)
}

// exportDOT 加载管道文件并把其 DAG 导出为 Graphviz DOT 格式到 stdout。
// 用户可用 `dot -Tpng pipeline.dot -o pipeline.png` 渲染成图片。
// 此模式不需要数据库，仅解析 YAML。
func exportDOT(path string) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("解析路径失败 %s: %w", path, err)
	}
	p, err := pipeline.LoadFromFile(abs)
	if err != nil {
		return err
	}
	fmt.Print(p.ToDOT())
	return nil
}

// runOnce 加载管道文件、执行、存历史、打印摘要。
// recover=true 时，先查最近一次该管道成功完成的步骤，跳过它们（状态恢复）。
func runOnce(db *sql.DB, path string, recover bool, log *slog.Logger) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("解析路径失败 %s: %w", path, err)
	}
	p, err := pipeline.LoadFromFile(abs)
	if err != nil {
		return err
	}

	var opts []pipeline.RunOption
	if recover {
		skipIDs, serr := storage.LatestSuccessfulSteps(db, p.Name)
		if serr != nil {
			log.Warn("[recover] 查询历史步骤失败，改为全量重跑", "err", serr)
		} else if len(skipIDs) > 0 {
			log.Info("[recover] 跳过已成功步骤", "steps", skipIDs)
			fmt.Printf("♻️  状态恢复：跳过 %d 步 %v\n", len(skipIDs), skipIDs)
			opts = append(opts, pipeline.WithSkipSteps(skipIDs))
		} else {
			log.Info("[recover] 无历史成功步骤，全量重跑", "pipeline", p.Name)
		}
	}

	log.Info("[run] 执行管道", "name", p.Name, "file", abs, "recover", recover)
	result := pipeline.RunWithOptions(*p, opts...)
	_, _ = storage.SaveRun(db, result)
	fmt.Println(result.Summary())
	for _, sr := range result.Steps {
		status := "✓"
		if sr.Err != nil {
			status = "✗"
		}
		if sr.Skipped {
			status = "⤵" // 跳过（状态恢复）
		}
		fmt.Printf("  %s [%s] %s: %d → %d 行 (%s)\n",
			status, sr.Kind, sr.StepID, sr.RowsIn, sr.RowsOut, sr.Duration.Round(time.Millisecond))
	}
	if result.Err != nil {
		return result.Err
	}
	return nil
}

// runScheduled 用 scheduler 包定时循环跑管道文件。
// 每次触发时重新从文件加载（便于开发时改 YAML 即时生效）。
func runScheduled(db *sql.DB, path string, intervalSec int, log *slog.Logger) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("解析路径失败 %s: %w", path, err)
	}
	sched := scheduler.New(log)
	if err := sched.Add(scheduler.Job{
		Name:     filepath.Base(abs),
		Interval: time.Duration(intervalSec) * time.Second,
		RunFunc: func(ctx context.Context, _ string) error {
			// 每次重新读文件（开发时改 YAML 即时生效）
			p, err := pipeline.LoadFromFile(abs)
			if err != nil {
				return err
			}
			result := pipeline.Run(*p)
			_, _ = storage.SaveRun(db, result)
			fmt.Println(result.Summary())
			return nil
		},
	}); err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	fmt.Printf("✅ 定时调度启动: 每 %ds 跑 %s（Ctrl+C 退出）\n", intervalSec, abs)
	cancel := sched.Start(ctx)
	<-ctx.Done()
	cancel()
	return nil
}

// runServer 启动 REST API 服务，阻塞直到收到信号。
func runServer(db *sql.DB, cfg *config.Config, log *slog.Logger) error {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/run", func(w http.ResponseWriter, r *http.Request) {
		handleRunFromYAML(w, r, db, log)
	})
	mux.HandleFunc("POST /api/run-file", func(w http.ResponseWriter, r *http.Request) {
		handleRunFromFile(w, r, db, log)
	})
	mux.HandleFunc("GET /api/pipelines", func(w http.ResponseWriter, r *http.Request) {
		handleListPipelines(w, r, db)
	})
	mux.HandleFunc("POST /api/pipelines", func(w http.ResponseWriter, r *http.Request) {
		handleSavePipeline(w, r, db)
	})
	mux.HandleFunc("GET /api/runs", func(w http.ResponseWriter, r *http.Request) {
		handleListRuns(w, r, db)
	})
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "version": version})
	})

	addr := cfg.Server.Addr()
	httpSrv := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  time.Duration(cfg.Server.ReadTimeoutSeconds) * time.Second,
		WriteTimeout: time.Duration(cfg.Server.WriteTimeoutSeconds) * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Info("[server] HTTP 监听", "addr", addr)
		fmt.Printf("✅ flow-pipe server 启动: http://%s\n", addr)
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("[server] HTTP 异常", "err", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	log.Info("[server] 收到退出信号，关闭")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = httpSrv.Shutdown(shutdownCtx)
	return nil
}

// ===== handlers =====

// maxBodyBytes 限制 POST body 大小（防 OOM）。1MB 足够单个管道 YAML。
const maxBodyBytes = 1 << 20

func handleRunFromYAML(w http.ResponseWriter, r *http.Request, db *sql.DB, log *slog.Logger) {
	// 限制 body 大小，防止超大 YAML 把进程打 OOM。
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "读取 body 失败（可能超过 1MB 限制）: " + err.Error()})
		return
	}
	p, err := pipeline.Parse(body)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "解析 YAML 失败: " + err.Error()})
		return
	}
	log.Info("[api] 执行管道", "name", p.Name)
	result := pipeline.Run(*p)
	_, _ = storage.SaveRun(db, result)
	writeJSON(w, http.StatusOK, result)
}

// allowedPipelineDirs 是 run-file 允许加载管道文件的目录白名单（相对工作目录）。
// 防止路径遍历：任意客户端不应让 server 读取机器上任意路径的文件。
var allowedPipelineDirs = []string{"examples", "pipelines"}

// validatePipelinePath 校验 path 是相对路径、在白名单目录下、且不含 .. 遍历。
func validatePipelinePath(path string) error {
	if path == "" {
		return fmt.Errorf("缺少 path 参数")
	}
	if filepath.IsAbs(path) {
		return fmt.Errorf("path 必须是相对路径（禁止绝对路径）")
	}
	cleaned := filepath.Clean(path)
	// 禁止 .. 遍历
	if strings.HasPrefix(cleaned, "..") {
		return fmt.Errorf("path 禁止包含 .. 遍历")
	}
	// 必须在白名单目录下
	for _, dir := range allowedPipelineDirs {
		if strings.HasPrefix(cleaned, dir+string(filepath.Separator)) || cleaned == dir {
			return nil
		}
	}
	return fmt.Errorf("path 必须在以下目录下: %s", strings.Join(allowedPipelineDirs, ", "))
}

func handleRunFromFile(w http.ResponseWriter, r *http.Request, db *sql.DB, log *slog.Logger) {
	path := r.URL.Query().Get("path")
	if err := validatePipelinePath(path); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	p, err := pipeline.LoadFromFile(path)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	log.Info("[api] 执行管道（文件）", "name", p.Name, "file", path)
	result := pipeline.Run(*p)
	_, _ = storage.SaveRun(db, result)
	writeJSON(w, http.StatusOK, result)
}

func handleListPipelines(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	pipes, err := storage.AllPipelines(db)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"pipelines": pipes})
}

func handleSavePipeline(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	var req struct {
		Name string `json:"name"`
		YAML string `json:"yaml"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if req.Name == "" || req.YAML == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name 和 yaml 必填"})
		return
	}
	if _, err := storage.SavePipeline(db, req.Name, req.YAML); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "saved", "name": req.Name})
}

func handleListRuns(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	limit := 20
	runs, err := storage.AllRuns(db, limit)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"runs": runs, "limit": limit})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
