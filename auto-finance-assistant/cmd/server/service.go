//go:build !plan9

package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/kardianos/service"
)

// serviceConfig 返回 kardianos/service 的服务配置。
// configPath 自动转为绝对路径，防 SCM 以 System32 为 CWD 找不到配置。
func serviceConfig(displayName, description, configPath string) *service.Config {
	absConfig := configPath
	if !filepath.IsAbs(absConfig) {
		if exe, err := os.Executable(); err == nil {
			absConfig = filepath.Join(filepath.Dir(exe), configPath)
		} else if cwd, err := os.Getwd(); err == nil {
			absConfig = filepath.Join(cwd, configPath)
		}
	}
	return &service.Config{
		Name:        "AutoFinanceAssistant",
		DisplayName: displayName,
		Description: description,
		Arguments:   []string{"-config", absConfig, "run"},
	}
}

// program 实现 service.Interface，在 SCM 模式下被调用。
type program struct {
	configPath string
	logger     *slog.Logger
	stopCh     chan struct{}
}

func (p *program) Start(s service.Service) error {
	p.stopCh = make(chan struct{})
	go p.run()
	return nil
}

func (p *program) Stop(s service.Service) error {
	close(p.stopCh)
	return nil
}

func (p *program) run() {
	// SCM 调起时，复用前台运行逻辑
	runForeground(p.configPath, p.stopCh)
}

// handleServiceCommands 处理 install/start/stop/uninstall/status 子命令。
// 无子命令（或 "run"）时返回 false，走前台运行。
func handleServiceCommands(args []string, configPath string) bool {
	if len(args) == 0 || args[0] == "run" {
		return false
	}

	cfg := serviceConfig(
		"汽车金融本地智能客服",
		"本地化汽车金融客服问答服务（Go + SQLite + Ollama）",
		configPath,
	)

	cmd := args[0]
	svc, err := service.New(&program{configPath: configPath}, cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[service] 初始化失败: %v\n", err)
		os.Exit(1)
	}

	switch cmd {
	case "install":
		if err := svc.Install(); err != nil {
			fmt.Fprintf(os.Stderr, "[service] 安装失败: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("服务已安装：AutoFinanceAssistant")
		fmt.Println("启动：auto-finance-assistant.exe start")
	case "uninstall":
		if err := svc.Uninstall(); err != nil {
			fmt.Fprintf(os.Stderr, "[service] 卸载失败: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("服务已卸载")
	case "start":
		if err := svc.Start(); err != nil {
			fmt.Fprintf(os.Stderr, "[service] 启动失败: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("服务已启动")
	case "stop":
		if err := svc.Stop(); err != nil {
			fmt.Fprintf(os.Stderr, "[service] 停止失败: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("服务已停止")
	case "status":
		status, err := svc.Status()
		if err != nil {
			fmt.Fprintf(os.Stderr, "[service] 状态查询失败: %v\n", err)
			os.Exit(1)
		}
		names := map[service.Status]string{
			service.StatusUnknown: "未知",
			service.StatusRunning: "运行中",
			service.StatusStopped: "已停止",
		}
		fmt.Printf("服务状态：%s\n", names[status])
	case "version":
		fmt.Println(version)
	case "help", "-h", "--help":
		printHelp()
	default:
		fmt.Fprintf(os.Stderr, "未知子命令：%s\n", cmd)
		printHelp()
		os.Exit(1)
	}
	return true
}

func printHelp() {
	fmt.Println(`auto-finance-assistant - 汽车金融本地智能客服

用法：
  auto-finance-assistant.exe [选项] [子命令]

选项：
  -config <路径>   配置文件路径（默认 config.yaml）

子命令：
  (无)             前台运行（被 SCM 调起时自动进入服务模式）
  run              前台运行（显式）
  install          注册为系统服务（开机自启）
  uninstall        卸载系统服务
  start            启动系统服务
  stop             停止系统服务
  status           查询服务状态
  version          显示版本号

示例：
  auto-finance-assistant.exe -config config.dev.yaml run    # 前台 CPU 验证
  auto-finance-assistant.exe install                         # 安装服务
  auto-finance-assistant.exe start                           # 启动服务`)
}

// flagParseArgs 分离 flag 与子命令（子命令在 flag 之后）。
func flagParseArgs(fs *flag.FlagSet, args []string) []string {
	fs.Parse(args)
	return fs.Args()
}
