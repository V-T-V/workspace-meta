// auto-finance-assistant 服务入口。
// 对应原计划第 4.1 节 + go-rmm 的 signal-shutdown 模式。
// M1：flag(-config) + 依赖装配 + HTTP 启动 + 优雅关闭。
// M8：install/start/stop/uninstall 子命令（kardianos/service）+ 启动检查。
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/QiuShichang/auto-finance-assistant/internal/api"
	"github.com/QiuShichang/auto-finance-assistant/internal/backup"
	"github.com/QiuShichang/auto-finance-assistant/internal/chat"
	"github.com/QiuShichang/auto-finance-assistant/internal/config"
	"github.com/QiuShichang/auto-finance-assistant/internal/knowledge"
	"github.com/QiuShichang/auto-finance-assistant/internal/logging"
	"github.com/QiuShichang/auto-finance-assistant/internal/modelclient"
	"github.com/QiuShichang/auto-finance-assistant/internal/parser"
	"github.com/QiuShichang/auto-finance-assistant/internal/queue"
	"github.com/QiuShichang/auto-finance-assistant/internal/rag"
	"github.com/QiuShichang/auto-finance-assistant/internal/storage"
)

// version 由 Makefile 通过 -ldflags "-X main.version=..." 注入。
var version = "dev"

func main() {
	configPath := flag.String("config", "config.yaml", "配置文件路径")
	flag.Parse()
	api.SetVersion(version)

	// M8：服务子命令分发（install/start/stop/uninstall/status/version/help）
	if handleServiceCommands(flag.Args(), *configPath) {
		return
	}

	// 前台运行（手动或被 SCM 调起）
	runForeground(*configPath, nil)
}

// runForeground 执行前台服务逻辑。stopCh 非 nil 时用于服务模式停止信号。
func runForeground(configPath string, stopCh chan struct{}) {
	// 1. 配置
	cfg, err := config.Load(configPath)
	if err != nil {
		os.Stderr.WriteString("[main] 配置加载失败: " + err.Error() + "\n")
		os.Exit(1)
	}

	// 2. 日志（写文件 + 轮转 + stderr）
	log, logWriter, _ := logging.NewWithFile(cfg.Logging.Directory, cfg.Logging.Level,
		cfg.Logging.MaxFileSizeMB, cfg.Logging.MaxFiles, cfg.Logging.RetainDays)
	if logWriter != nil {
		defer logWriter.Close()
	}
	log.Info("[main] auto-finance-assistant 启动", "version", version,
		"addr", cfg.Addr(), "model", cfg.Ollama.ChatModel,
		"logDir", cfg.Logging.Directory, "logMaxSize", cfg.Logging.MaxFileSizeMB)

	// 安全检查：非本地监听且无管理员密码时警告
	if cfg.Server.Host != "127.0.0.1" && cfg.Server.Host != "localhost" && cfg.Security.AdminPassword == "" {
		log.Error("[main] 安全风险：监听非本地地址且未设置 admin_password，拒绝启动。" +
			"请在 config.yaml 设置 security.admin_password，或仅监听 127.0.0.1")
		os.Exit(1)
	}

	// 3. 模型客户端（根据 config.backend 选择 ollama 或 llamacpp）
	oc := modelclient.New(cfg.Ollama, cfg.Generation)

	// 4. 启动检查清单（含模型服务探测，告警不阻断）
	if err := runStartupChecks(cfg, oc, log); err != nil {
		log.Error("[main] 启动检查失败", "err", err)
		os.Exit(1)
	}

	// 5. 数据目录就绪
	if err := ensureDirs(cfg); err != nil {
		log.Error("[main] 创建数据目录失败", "err", err)
		os.Exit(1)
	}

	// 6. 数据库 + 迁移
	db, err := storage.OpenDB(cfg.Storage.DatabasePath, log)
	if err != nil {
		log.Error("[main] 打开数据库失败", "err", err)
		os.Exit(1)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	if err := storage.Migrate(ctx, db, storage.AllActiveVersions(), log); err != nil {
		cancel()
		log.Error("[main] 数据库迁移失败", "err", err)
		os.Exit(1)
	}
	cancel()

	// 7. LLM 队列
	q := queue.New(cfg.Queue.GenerationConcurrency, cfg.Queue.MaximumWaiting,
		cfg.Queue.QueueTimeout(), log)

	// 8. 聊天服务 + FAQ 索引
	chatSvc := chat.New(oc, db, q, log)
	loadCtx, loadCancel := context.WithTimeout(context.Background(), 30*time.Second)
	if err := chatSvc.LoadFAQs(loadCtx); err != nil {
		log.Warn("[main] FAQ 索引加载失败（降级为纯模型回答）", "err", err)
	}
	loadCancel()

	// 9. RAG（FTS + 向量融合）
	fts := rag.NewFTSSearcher(db, cfg.RAG.FTSLimit)
	vecSearcher := rag.NewVectorSearcher(db, oc, cfg.RAG.VectorLimit, log)
	vecLoadCtx, vecLoadCancel := context.WithTimeout(context.Background(), 30*time.Second)
	if n, err := vecSearcher.LoadFromDB(vecLoadCtx); err != nil {
		log.Warn("[main] 向量索引加载失败（降级为纯 FTS）", "err", err)
	} else if n > 0 {
		log.Info("[main] 向量索引已就绪", "count", n)
	}
	vecLoadCancel()
	hybrid := rag.NewHybridRetriever(fts, vecSearcher, cfg.RAG.VectorWeight, cfg.RAG.KeywordWeight, log)
	ragSvc := rag.NewService(hybrid, cfg.RAG.MinimumConfidence, cfg.RAG.HighConfidence,
		cfg.RAG.FinalLimit, log)
	chatSvc.SetRAG(ragSvc)

	// 10. 文档导入器
	parserReg := parser.NewRegistry()
	importer := knowledge.NewImporter(db, parserReg, cfg.Documents,
		cfg.Storage.DocumentPath, cfg.Storage.TempPath, log)

	// 11. HTTP 路由
	mux := http.NewServeMux()
	srv := api.New(chatSvc, oc, q, db, importer)
	srv.SetVectorSearcher(vecSearcher)
	if cfg.Security.AdminPassword != "" {
		srv.SetAdminPassword(cfg.Security.AdminPassword)
	}
	// M9 备份管理器
	bm := backup.New(db, cfg.Storage.DatabasePath, cfg.Storage.BackupPath, cfg.Backup.RetainCount, log)
	srv.SetBackupManager(bm)
	srv.Register(mux)

	httpServer := &http.Server{
		Addr:              cfg.Addr(),
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       time.Duration(cfg.Server.ReadTimeoutSeconds) * time.Second,
		// WriteTimeout 故意不设：SSE 流式响应（/api/chat/stream）的写时间可能超过任何固定上限，
		// 设了会在生成中途断开连接。超时由 queue 层的 ctx 控制。
		// cfg.Server.WriteTimeoutSeconds 仅作文档参考，不应用到此处。
	}

	// 12. 启动 + 优雅关闭
	sigCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Info("[main] HTTP 监听", "addr", cfg.Addr())
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("[main] HTTP 监听失败", "err", err)
			os.Exit(1)
		}
	}()

	// 服务模式：监听 stopCh
	select {
	case <-sigCtx.Done():
		log.Info("[main] 收到退出信号，开始关闭...")
	case <-stopCh:
		log.Info("[main] 服务停止请求，开始关闭...")
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Error("[main] 关闭超时", "err", err)
	}
	log.Info("[main] 已关闭")
}

// runStartupChecks 启动检查清单（原计划 21.5）。
// 关键检查失败时返回错误；ollama 不可达只告警不阻断（降级运行）。
func runStartupChecks(cfg *config.Config, oc modelclient.ModelClient, log *slog.Logger) error {
	if cfg.Storage.DatabasePath == "" {
		return fmt.Errorf("[startup] storage.database_path 不能为空")
	}
	if cfg.Ollama.BaseURL == "" {
		return fmt.Errorf("[startup] ollama.base_url 不能为空")
	}
	// 探测 ollama 可达性 + 模型是否存在（告警，不阻断）
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	hs := oc.Health(ctx)
	if !hs.Reachable {
		log.Warn("[startup] Ollama 不可达，请先运行 `ollama serve`。" +
			"服务将以降级模式启动（聊天功能不可用，知识库管理仍可用）。")
	} else if !hs.HasModel {
		log.Warn("[startup] Ollama 已运行但缺少对话模型，请运行:",
			"model", cfg.Ollama.ChatModel)
		fmt.Fprintf(os.Stderr, "\n  >>> ollama pull %s\n\n", cfg.Ollama.ChatModel)
	}
	// embedding 模型检查
	if hs.Reachable && cfg.Ollama.EmbeddingModel != "" {
		hasEmbed := false
		for _, m := range hs.Models {
			if m == cfg.Ollama.EmbeddingModel {
				hasEmbed = true
				break
			}
		}
		if !hasEmbed {
			log.Warn("[startup] 缺少向量模型，请运行（否则向量检索降级为纯 FTS）:",
				"model", cfg.Ollama.EmbeddingModel)
			fmt.Fprintf(os.Stderr, "\n  >>> ollama pull %s\n\n", cfg.Ollama.EmbeddingModel)
		}
	}
	return nil
}

// ensureDirs 确保运行时数据目录存在。
func ensureDirs(cfg *config.Config) error {
	for _, dir := range []string{
		"./data",
		cfg.Storage.DocumentPath,
		cfg.Storage.TempPath,
		cfg.Storage.BackupPath,
		cfg.Logging.Directory,
	} {
		if dir == "" {
			continue
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return nil
}
