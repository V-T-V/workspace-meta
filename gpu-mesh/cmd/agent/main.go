// Command gpu-mesh-agent 启动受控端 Agent（Windows 服务 / 前台运行）。
//
// 前台运行（开发调试）：
//
//	gpu-mesh-agent.exe run -relay ws://RELAY:7780 -token TOKEN
//
// 安装为 Windows 服务（生产部署）：
//
//	gpu-mesh-agent.exe install -relay ws://RELAY:7780 -token TOKEN
//	gpu-mesh-agent.exe start
//
// Agent 会注册为开机自启的 Windows 服务，经反向 WS 连接 Relay 穿透 NAT，
// 周期上报 GPU 快照 + 让位状态，接收并执行任务。
package main

import (
	"github.com/QiuShichang/gpu-mesh/internal/agent"
)

func main() {
	// 必须在任何 HTTP 调用前禁用系统代理：Agent 直连 Relay，不走代理软件。
	// 原因：net/http 的 envProxyOnce（sync.Once）在首次 HTTP 请求时永久缓存代理配置。
	agent.EarlyInit()
	agent.RunMain()
}
