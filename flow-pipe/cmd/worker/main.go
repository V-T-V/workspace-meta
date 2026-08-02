// Command worker 是 flow-pipe 的远程 worker 入口（M3 待实现）。
//
// 匿名 Import 连接器包以保证编译时它们被链接（worker 未来需要本地执行步骤）。
//
// M1 状态：管道在 server 进程内单机执行（pipeline.Run），不需要 worker。
// M3 将通过 internal/proto 的反向 WS 协议实现远程 worker：
//
//	worker 启动 → 反向 WS 连接 server → 注册（MsgRegister）→ 接收 task_assign → 执行 → 回报 task_result
//
// 协议结构已在 internal/proto 定义，传输层（transport.go）+ 真连接逻辑留待 M3。
package main

import (
	"flag"
	"fmt"

	_ "github.com/QiuShichang/flow-pipe/internal/sink"
	_ "github.com/QiuShichang/flow-pipe/internal/source"
	_ "github.com/QiuShichang/flow-pipe/internal/transform"
)

// version 由 -ldflags 注入。
var version = "dev"

func main() {
	showVersion := flag.Bool("version", false, "打印版本号")
	flag.Parse()

	if *showVersion {
		fmt.Println("flow-pipe-worker", version)
		return
	}

	fmt.Println(`flow-pipe worker（M3 待实现）

当前 M1 为单机模式：管道在 server 进程内执行（pipeline.Run），无需 worker。

M3 路线（参考 go-rmm 的 relay/agent 反向 WS 模式）：
  1. worker 启动 → 反向 WebSocket 连接中心 server
  2. 发送 MsgRegister（携带 ID + 支持的连接器列表）
  3. 接收 MsgTaskAssign（一个管道步骤）
  4. 本地执行（source.Read / transform.Transform / sink.Write）
  5. 回报 MsgTaskResult

协议结构已在 internal/proto 定义：
  - Envelope（消息信封）
  - TaskPayload / TaskResultPayload
  - WorkerInfo

M1 不实装真分布式的原因：避免过度设计。单机模式已满足 ETL 学习/原型需求，
分布式引入的连接管理/重试/故障转移/分片策略是独立工程量。`)
}
