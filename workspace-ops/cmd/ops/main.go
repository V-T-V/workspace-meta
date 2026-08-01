// Command ops 是 workspace-ops 的 CLI 入口。
//
// 用法：
//
//	workspace-ops scan [-config path]      # 扫描工作区，结果入库
//	workspace-ops report [-config path] [-format text|json|markdown]
//	workspace-ops serve [-config path] [-port 8765]
//	workspace-ops test  [-config path] [-slug xxx] [-timeout 2]  # 实跑测试采集
//	workspace-ops version
//
// 对齐 go-rmm/cmd/go-rmm 的 flag + 子命令分发风格。
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/QiuShichang/workspace-ops/internal/api"
	"github.com/QiuShichang/workspace-ops/internal/config"
	"github.com/QiuShichang/workspace-ops/internal/inspector"
	"github.com/QiuShichang/workspace-ops/internal/logging"
	"github.com/QiuShichang/workspace-ops/internal/report"
	"github.com/QiuShichang/workspace-ops/internal/storage"
	"github.com/QiuShichang/workspace-ops/internal/testrunner"
	"github.com/QiuShichang/workspace-ops/internal/web"
	"github.com/QiuShichang/workspace-ops/internal/workspace"
)

// version 由 -ldflags 注入（见 Makefile LDFLAGS）。
var version = "dev"

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	sub := os.Args[1]
	args := os.Args[2:]

	switch sub {
	case "scan":
		exit(runScan(args))
	case "report":
		exit(runReport(args))
	case "serve":
		exit(runServe(args))
	case "test":
		exit(runTest(args))
	case "diff":
		exit(runDiff(args))
	case "version", "-version", "--version":
		fmt.Println("workspace-ops", version)
	case "-h", "--help", "help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "未知子命令: %s\n\n", sub)
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, `workspace-ops - 工作区级项目状态管理工具

用法:
  workspace-ops scan   [-config config.dev.yaml]           扫描工作区，结果入库
  workspace-ops report [-config ...] [-format text|json|markdown]  输出报告
  workspace-ops serve  [-config ...] [-port 8765]          启动 Web 看板
  workspace-ops test   [-config ...] [-slug xxx]           实跑各项目测试采集成败
  workspace-ops version`)
}

func exit(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "错误:", err)
		os.Exit(1)
	}
}

// loadCfg 加载配置。configPath 不存在时用默认配置（让 scan/report/serve 在裸环境也能跑）。
// 返回配置 + configPath 所在目录（用于解析相对的 scan.root）。
func loadCfg(configPath string) (*config.Config, string, error) {
	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[warn] 加载配置 %s 失败（%v），使用默认配置\n", configPath, err)
		cfg = config.Default()
	}
	abs, _ := filepath.Abs(configPath)
	return cfg, filepath.Dir(abs), nil
}

// runScan: 扫描工作区，入库。
func runScan(args []string) error {
	fs := flag.NewFlagSet("scan", flag.ExitOnError)
	configPath := fs.String("config", "config.dev.yaml", "配置文件路径（YAML）")
	_ = fs.Parse(args)

	cfg, configDir, _ := loadCfg(*configPath)
	log := logging.New(cfg.Logging.Level, cfg.Logging.Format)

	root, err := cfg.ResolveRoot(configDir)
	if err != nil {
		return err
	}
	log.Info("[scan] 开始扫描", "root", root, "config", *configPath)

	dbPath, err := cfg.ResolveDBPath()
	if err != nil {
		return err
	}
	db, err := storage.Open(dbPath)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()

	projects, err := workspace.Discover(root, cfg.Scan.IgnoreDirs)
	if err != nil {
		return err
	}
	log.Info("[scan] 发现项目", "count", len(projects))

	insp := inspector.New(inspector.Checks{
		Stack:          cfg.Scan.Checks.Stack,
		Dependencies:   cfg.Scan.Checks.Dependencies,
		AgentsMD:       cfg.Scan.Checks.AgentsMD,
		GitStatus:      cfg.Scan.Checks.GitStatus,
		Tests:          cfg.Scan.Checks.Tests,
		BuildArtifacts: cfg.Scan.Checks.BuildArtifacts,
	}, "git")

	scanID, err := storage.StartScan(db)
	if err != nil {
		return err
	}
	facts := insp.InspectAll(projects)
	for _, f := range facts {
		log.Debug("[scan] " + f.Summary())
		if err := storage.SaveFacts(db, scanID, f.Slug, f.Path, f.KV); err != nil {
			return err
		}
	}
	if err := storage.FinishScan(db, scanID, len(projects), "done"); err != nil {
		return err
	}

	fmt.Printf("✅ 扫描完成: %d 个项目，已入库 %s\n", len(projects), dbPath)
	return nil
}

// runReport: 从库读数据，输出报告。
func runReport(args []string) error {
	fs := flag.NewFlagSet("report", flag.ExitOnError)
	configPath := fs.String("config", "config.dev.yaml", "配置文件路径（YAML）")
	format := fs.String("format", "text", "输出格式: text|json|markdown")
	_ = fs.Parse(args)

	cfg, _, _ := loadCfg(*configPath)
	dbPath, err := cfg.ResolveDBPath()
	if err != nil {
		return err
	}
	db, err := storage.Open(dbPath)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()

	// 从库读 projects + facts，重建 Facts 切片给 report.Build。
	projects, err := storage.AllProjects(db)
	if err != nil {
		return err
	}
	facts := make([]inspector.Facts, 0, len(projects))
	for _, p := range projects {
		kv, err := storage.ProjectFacts(db, p.ID, 0)
		if err != nil {
			return err
		}
		facts = append(facts, inspector.Facts{Slug: p.Slug, Path: p.Path, KV: kv})
	}

	rep := report.Build(facts)
	rep.ScanAt = time.Now().UTC().Format(time.RFC3339)

	formatter, err := report.DefaultRegistry().Get(*format)
	if err != nil {
		return err
	}
	fmt.Println(formatter.Format(rep))
	return nil
}

// runServe: 启动 Web 看板。
func runServe(args []string) error {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	configPath := fs.String("config", "config.dev.yaml", "配置文件路径（YAML）")
	port := fs.Int("port", 0, "监听端口（覆盖配置）")
	_ = fs.Parse(args)

	cfg, configDir, _ := loadCfg(*configPath)
	if *port != 0 {
		cfg.Server.Port = *port
	}
	log := logging.New(cfg.Logging.Level, cfg.Logging.Format)

	dbPath, err := cfg.ResolveDBPath()
	if err != nil {
		return err
	}
	db, err := storage.Open(dbPath)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()

	// 若库为空（无 scan 记录），首次启动自动扫一次。
	if latest, _ := storage.LatestScan(db); latest == nil {
		log.Info("[serve] 库为空，首次启动自动扫描")
		srv := api.NewServer(db, log, cfg, configDir)
		_, _ = srv.Resolver.Scan(db, configDir)
	}

	srv := api.NewServer(db, log, cfg, configDir)
	mux := http.NewServeMux()
	// /api/ 走 API 路由
	mux.Handle("/api/", srv.Routes())
	// 其余走前端静态资源（或 fallback）
	mux.Handle("/", web.StaticHandler())

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
		log.Info("[serve] HTTP 监听", "addr", addr, "dist", web.HasDist())
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("[serve] HTTP 异常", "err", err)
			os.Exit(1)
		}
	}()

	fmt.Printf("✅ Web 看板已启动: http://%s\n", addr)
	<-ctx.Done()
	log.Info("[serve] 收到退出信号，关闭")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = httpSrv.Shutdown(shutdownCtx)
	return nil
}

// runTest: 实跑各项目的测试命令，采集成败 + 耗时。
func runTest(args []string) error {
	fs := flag.NewFlagSet("test", flag.ExitOnError)
	configPath := fs.String("config", "config.dev.yaml", "配置文件路径（YAML）")
	slugFilter := fs.String("slug", "", "只跑指定 slug 的项目（空=全部）")
	timeoutMin := fs.Int("timeout", 2, "单项目测试超时（分钟）")
	_ = fs.Parse(args)

	cfg, configDir, _ := loadCfg(*configPath)
	log := logging.New(cfg.Logging.Level, cfg.Logging.Format)

	root, err := cfg.ResolveRoot(configDir)
	if err != nil {
		return err
	}
	projects, err := workspace.Discover(root, cfg.Scan.IgnoreDirs)
	if err != nil {
		return err
	}

	trCfg := testrunner.Config{Timeout: time.Duration(*timeoutMin) * time.Minute}

	// 打开 db，拿最新 scan_id 作为本次测试运行关联。若无 scan 记录则建一条。
	dbPath, err := cfg.ResolveDBPath()
	if err != nil {
		return err
	}
	db, err := storage.Open(dbPath)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()
	latest, _ := storage.LatestScan(db)
	var scanID int64
	if latest == nil {
		scanID, _ = storage.StartScan(db)
	} else {
		scanID = latest.ID
	}

	// 先做一次轻量 stack 识别（不跑完整 inspect，只看标志文件）
	pass, fail, skip, errs := 0, 0, 0, 0
	fmt.Printf("开始跑 %d 个项目的测试（超时 %dmin/项，入库 scan=%d）...\n\n", len(projects), *timeoutMin, scanID)
	for _, p := range projects {
		if *slugFilter != "" && p.Slug != *slugFilter {
			continue
		}
		stack := detectStackLite(p.Path)
		r := testrunner.Run(p.Slug, stack, p.Path, trCfg)
		fmt.Println(testrunner.FormatSummary(r))
		// 入库：按 slug 找 project_id（若项目已 scan 过则存在）
		if pid, err := storage.GetProjectIDBySlug(db, p.Slug); err == nil && pid > 0 {
			_, _ = storage.SaveTestRun(db, pid, scanID, r.Status, r.Command, r.Duration, r.OutputTail)
		}
		switch r.Status {
		case "pass":
			pass++
		case "fail":
			fail++
		case "skipped":
			skip++
		default:
			errs++
		}
		// 失败时打印输出末尾
		if r.Status == "fail" && r.OutputTail != "" {
			fmt.Printf("  └─ %s\n", strings.Split(r.OutputTail, "\n")[0])
		}
	}
	log.Info("[test] 完成", "pass", pass, "fail", fail, "skipped", skip, "error", errs)
	fmt.Printf("\n汇总: ✓ pass=%d  ✗ fail=%d  ⊘ skipped=%d  ? error=%d\n", pass, fail, skip, errs)
	return nil
}

// detectStackLite 轻量栈识别（只看标志文件，不读依赖内容），test 子命令用。
func detectStackLite(path string) string {
	if existsFile(path, "go.mod") {
		return "go"
	}
	if existsFile(path, "package.json") {
		return "node/ts"
	}
	if existsFile(path, "Cargo.toml") {
		return "rust"
	}
	if existsFile(path, "pyproject.toml") || existsFile(path, "requirements.txt") {
		return "python"
	}
	if existsFile(path, "project.godot") {
		return "godot"
	}
	if existsFile(path, "pubspec.yaml") {
		return "flutter"
	}
	return "unknown"
}

func existsFile(dir, name string) bool {
	_, err := os.Stat(filepath.Join(dir, name))
	return err == nil
}

// runDiff: 比较两次 scan 的差异（新增/删除/变更项目）。
func runDiff(args []string) error {
	fs := flag.NewFlagSet("diff", flag.ExitOnError)
	configPath := fs.String("config", "config.dev.yaml", "配置文件路径（YAML）")
	scanA := fs.Int64("a", 0, "旧 scan ID（默认：倒数第二近的 scan）")
	scanB := fs.Int64("b", 0, "新 scan ID（默认：最近的 scan）")
	_ = fs.Parse(args)

	cfg, _, _ := loadCfg(*configPath)
	dbPath, err := cfg.ResolveDBPath()
	if err != nil {
		return err
	}
	db, err := storage.Open(dbPath)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()

	// 默认取最近两次 scan
	scans, err := storage.AllScans(db)
	if err != nil {
		return err
	}
	if len(scans) < 2 {
		return fmt.Errorf("需要至少 2 次 scan 才能比较（当前 %d）", len(scans))
	}
	aID := *scanA
	bID := *scanB
	if aID == 0 {
		aID = scans[1].ID // 倒数第二（AllScans 按 id DESC 返回）
	}
	if bID == 0 {
		bID = scans[0].ID // 最近
	}

	diff, err := storage.DiffScans(db, aID, bID)
	if err != nil {
		return err
	}
	fmt.Print(storage.FormatDiff(diff))
	return nil
}
