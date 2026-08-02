// Command gpu-mesh-relay 启动中继节点（含 Web 控制台）。
//
// 部署：公网 VPS 上运行，监听 :7780，接受 Agent 反向连接 + 提供控制台。
//
// 用法：
//
//	gpu-mesh-relay                      # 零配置启动（默认 :7780，无 token）
//	gpu-mesh-relay -addr :80            # 指定端口
//	gpu-mesh-relay -token s3cret        # 启用鉴权（Agent/Web 都需带 token）
//	gpu-mesh-relay -store               # 启用 bbolt 持久化
//	gpu-mesh-relay -audit audit.jsonl   # 启用审计日志
//	gpu-mesh-relay -mtls                # Phase 6 启用 mTLS（Agent 需证书）
package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/QiuShichang/gpu-mesh/internal/relay"
	"github.com/QiuShichang/gpu-mesh/internal/web"
)

func main() {
	addr := flag.String("addr", getEnvOrDefault("ADDR", ":7780"), "监听地址")
	token := flag.String("token", os.Getenv("GPU_MESH_TOKEN"), "鉴权 token（空则不校验）")
	useStore := flag.Bool("store", true, "启用 bbolt 任务持久化")
	storePath := flag.String("store-path", "gpu-mesh.db", "bbolt 文件路径")
	auditPath := flag.String("audit", "", "审计日志路径（空则不记审计）")
	_ = flag.Bool("mtls", false, "启用 mTLS 双向证书（Phase 6，需先初始化 CA）")
	flag.Parse()

	r, err := relay.New(relay.Config{
		Addr:      *addr,
		Token:     *token,
		Store:     *useStore,
		StorePath: *storePath,
		AuditPath: *auditPath,
	})
	if err != nil {
		log.Fatalf("初始化 Relay 失败: %v", err)
	}
	defer r.Close()

	mux := http.NewServeMux()
	r.Routes(mux)

	// 内嵌 Web 控制台（根路径）
	mux.Handle("GET /", http.FileServerFS(web.FS))

	// 心跳超时清理 + 任务 TTL 清理
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r.StartSweeper(ctx)
	r.StartTaskCleaner(ctx)

	// 优雅关停
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		log.Println("[relay] 收到关闭信号，退出…")
		cancel()
		os.Exit(0)
	}()

	log.Printf("[relay] GPU Mesh 中继节点启动 http://%s", *addr)
	log.Printf("[relay] 控制台: http://%s/  Agent 接入: ws://%s/agent", *addr, *addr)
	if *token == "" {
		log.Printf("[relay] ⚠️  未设 token（零配置模式）。生产环境请用 -token 指定。")
	}
	if *auditPath != "" {
		log.Printf("[relay] 审计日志: %s", *auditPath)
	}
	if err := http.ListenAndServe(*addr, mux); err != nil {
		log.Fatalf("ListenAndServe 失败: %v", err)
	}
}

func getEnvOrDefault(env, def string) string {
	if v := os.Getenv(env); v != "" {
		return v
	}
	return def
}
