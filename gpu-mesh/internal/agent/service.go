package agent

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/kardianos/service"

	"github.com/QiuShichang/gpu-mesh/internal/gpumon"
)

// ServiceConfig Windows 服务化配置（kardianos/service）。
//
// 安装为服务后，由 SCM 管理开机自启 + 崩溃自恢复。
// 也支持非 Windows 前台运行（开发调试用）。
type ServiceConfig struct {
	AgentID  string
	RelayURL string
	Token    string
}

// serviceOptions 构造 kardianos/service 的选项。
//
// 服务参数仅保留 "run"（无敏感信息）；RelayURL/AgentID/Token 通过环境变量注入，
// program.Start 时读取。这样 token 不会出现在服务可执行参数列表里。
func serviceOptions(cfg ServiceConfig) *service.Config {
	envs := map[string]string{}
	if cfg.RelayURL != "" {
		envs["GPU_MESH_RELAY"] = cfg.RelayURL
	}
	if cfg.AgentID != "" {
		envs["GPU_MESH_AGENT_ID"] = cfg.AgentID
	}
	if cfg.Token != "" {
		envs["GPU_MESH_TOKEN"] = cfg.Token
	}
	return &service.Config{
		Name:        "gpu-mesh-agent",
		DisplayName: "GPU Mesh Agent",
		Description: "gpu-mesh 分布式 GPU 算力调度平台受控端。采集 GPU 状态、上报让位信号、执行推理/批量/训练任务。",
		Arguments:  []string{"run"},
		EnvVars:    envs,
	}
}

// program 实现 service.Interface（Start/Stop）。
type program struct {
	cfg    Config
	ctx    context.Context
	cancel context.CancelFunc
	agent  *Agent
}

func (p *program) Start(s service.Service) error {
	p.ctx, p.cancel = context.WithCancel(context.Background())
	// ★ 服务模式日志：输出到文件（Windows 服务下 stdout 是虚空，否则运维看不到日志）
	// 路径：C:\ProgramData\gpu-mesh\logs\agent-YYYY-MM-DD.log，按天轮转
	if err := SetupServiceLogger(""); err != nil {
		// 日志初始化失败不阻断启动，降级到 stdout（至少 service 能起）
		log.Printf("[agent] 日志文件初始化失败（降级 stdout）: %v", err)
	}
	// 服务模式下配置来自环境变量（安装时注入）
	if p.cfg.RelayURL == "" {
		p.cfg = Config{
			AgentID:  os.Getenv("GPU_MESH_AGENT_ID"),
			RelayURL: os.Getenv("GPU_MESH_RELAY"),
			Token:    os.Getenv("GPU_MESH_TOKEN"),
		}
	}
	go p.run()
	return nil
}

func (p *program) run() {
	p.agent = New(p.cfg)
	p.agent.Run(p.ctx)
}

func (p *program) Stop(s service.Service) error {
	log.Printf("[agent] 收到服务停止指令，优雅退出...")
	if p.cancel != nil {
		p.cancel()
	}
	// 给在途任务一点时间回流结果
	time.Sleep(500 * time.Millisecond)
	CloseLogger()
	return nil
}

// RunMain Agent CLI 入口（由 cmd/agent/main.go 调用）。
//
// 子命令：
//   - 无参数 / run      前台运行（开发调试）
//   - install            安装为 Windows 服务
//   - uninstall          卸载服务
//   - start/stop         启停服务
func RunMain() {
	if len(os.Args) >= 2 {
		switch os.Args[1] {
		case "install", "uninstall", "start", "stop", "restart":
			handleServiceCmd(os.Args[1])
			return
		case "run":
			os.Args = append(os.Args[:1], os.Args[2:]...) // 移除 "run"，继续解析 flag
		}
	}
	runForeground()
}

// runForeground 前台运行（开发调试 / 容器内运行）。
func runForeground() {
	fs := flag.NewFlagSet("gpu-mesh-agent", flag.ExitOnError)
	relay := fs.String("relay", "", "Relay 接入地址，如 ws://relay.example.com:7780/agent 或裸 IP")
	agentID := fs.String("id", "", "Agent ID（默认用 hostname）")
	token := fs.String("token", "", "鉴权 token（Relay 未设则忽略）")
	_ = fs.Parse(os.Args[1:])

	if *relay == "" {
		*relay = os.Getenv("GPU_MESH_RELAY")
	}
	if *relay == "" {
		fmt.Fprintln(os.Stderr, "用法: gpu-mesh-agent run -relay ws://HOST:7780/agent")
		fmt.Fprintln(os.Stderr, "  或设环境变量 GPU_MESH_RELAY")
		os.Exit(2)
	}

	cfg := Config{
		AgentID:  firstNonEmpty(*agentID, os.Getenv("GPU_MESH_AGENT_ID")),
		RelayURL: *relay,
		Token:    firstNonEmpty(*token, os.Getenv("GPU_MESH_TOKEN")),
		Tags:     map[string]string{"gpu": detectGPUType()},
	}

	ctx, cancel := signalHandler()
	defer cancel()

	ag := New(cfg)
	log.Printf("[agent] 前台模式启动 id=%s relay=%s", cfg.AgentID, cfg.RelayURL)
	ag.Run(ctx)
}

// handleServiceCmd 处理服务管理子命令。
func handleServiceCmd(cmd string) {
	// 服务参数从 flag 解析
	fs := flag.NewFlagSet(cmd, flag.ExitOnError)
	relay := fs.String("relay", os.Getenv("GPU_MESH_RELAY"), "Relay 接入地址")
	agentID := fs.String("id", os.Getenv("GPU_MESH_AGENT_ID"), "Agent ID")
	token := fs.String("token", os.Getenv("GPU_MESH_TOKEN"), "鉴权 token")
	_ = fs.Parse(os.Args[2:])

	svcCfg := serviceOptions(ServiceConfig{
		AgentID: *agentID, RelayURL: *relay, Token: *token,
	})
	svc, err := service.New(&program{}, svcCfg)
	if err != nil {
		log.Fatalf("初始化服务失败: %v", err)
	}

	var err2 error
	switch cmd {
	case "install":
		err2 = svc.Install()
		if err2 == nil {
			fmt.Println("服务已安装。启动: gpu-mesh-agent start")
		}
	case "uninstall":
		err2 = svc.Uninstall()
		if err2 == nil {
			fmt.Println("服务已卸载")
		}
	case "start":
		err2 = svc.Start()
	case "stop":
		err2 = svc.Stop()
	case "restart":
		err2 = svc.Restart()
	}
	if err2 != nil {
		log.Fatalf("%s 失败: %v", cmd, err2)
	}
}

// detectGPUType 从 nvidia-smi 推断 GPU 型号用作标签（失败则返回 unknown）。
func detectGPUType() string {
	ctx := context.Background()
	snap, err := gpumon.SnapshotOnce(ctx)
	if err != nil || len(snap) == 0 {
		return "unknown"
	}
	// 简化：取第一张卡名，提取型号关键字
	name := snap[0].Name
	for _, k := range []string{"4060", "4070", "4080", "4090", "3060", "3070", "3080", "3090", "A100", "V100", "T4"} {
		if contains(name, k) {
			return k
		}
	}
	return "unknown"
}

// signalHandler 返回一个在收到 SIGINT/SIGTERM 时取消的 context（前台模式用）。
//
// 服务模式由 SCM 调 program.Stop 触发取消，不走信号。
// 前台模式（run 子命令）必须捕获信号，否则 Ctrl+C 无法优雅退出。
func signalHandler() (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigCh
		log.Printf("[agent] 收到信号 %v，优雅退出...", sig)
		cancel()
		// 再次收到信号强制退出（防止卡死）
		<-sigCh
		os.Exit(1)
	}()
	return ctx, cancel
}

// firstNonEmpty 返回第一个非空字符串。
func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
