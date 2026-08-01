# examples

consensus-atlas 的示例与用法入口。

## 快速演示

```bash
# 在项目根目录运行（go run ./cmd/atlas -d <算法名>）
go run ./cmd/atlas -d raft        # 5 节点 Raft：选举 + 日志复制 + commit
go run ./cmd/atlas -d paxos       # Multi-Paxos 两阶段共识
go run ./cmd/atlas -d viewstamped # Viewstamped Replication：Primary-Backup + 视图变更
go run ./cmd/atlas -d zab         # ZAB：ZooKeeper 原子广播，zxid 顺序提交
go run ./cmd/atlas -d pbft        # PBFT 三阶段拜占庭容错
go run ./cmd/atlas -d byzgen      # Byzantine Generals：OM 口头消息算法（含叛徒场景）
go run ./cmd/atlas -d twopc       # 两阶段提交：原子提交协议
go run ./cmd/atlas -d crdt        # G-Counter CRDT：max 合并最终收敛
go run ./cmd/atlas -d gossip      # Gossip 反熵：5 节点状态最终收敛
go run ./cmd/atlas -d snapshot    # Chandy-Lamport 分布式快照
go run ./cmd/atlas -d bully       # Bully + Ring 选举
go run ./cmd/atlas -d clock       # Lamport + Vector Clock 因果序对比
go run ./cmd/atlas -d all         # 依次跑全部 12 个 demo
```

所有 demo **离线可跑**（纯内存网络 + 确定性轨迹，不需要任何外部依赖）。

## 作为库使用

```go
package main

import (
	"context"
	"fmt"

	"github.com/QiuShichang/consensus-atlas/internal/core"
	"github.com/QiuShichang/consensus-atlas/internal/raft"
)

func main() {
	// 自己组装一个 Raft 集群
	tr := core.NewMemTransport()
	peers := []core.NodeID{"a", "b", "c"}
	nodes := make(map[core.NodeID]*raft.Node, len(peers))
	for i, id := range peers {
		// 不同选举超时，保证 a 最先当选
		n := raft.NewNode(id, peers, 5+i*2, tr)
		n.Start()
		nodes[id] = n
	}

	// tick 推进选举
	for i := 0; i < 30; i++ {
		for _, id := range peers {
			nodes[id].Tick()
		}
		tr.Drain()
	}

	// 找到 Leader 并提交一条命令
	for _, n := range nodes {
		if n.State == core.StateLeader {
			n.Propose("set x=42")
			for i := 0; i < 10; i++ {
				tr.Drain()
			}
			fmt.Printf("Leader=%s commitIndex=%d\n", n.ID, n.CommitIndex)
		}
	}

	_ = context.Background()
}
```

## 各算法的学习路径

按难度递增：

1. **clock**（最易）—— 纯函数式，理解"分布式事件定序"
2. **crdt**（易）—— 纯函数式，理解"无冲突复制"与"最终一致收敛"
3. **gossip**（易）—— 理解"最终一致"与"传染病扩散"
4. **snapshot**（易中）—— 理解"不停机记录一致全局状态"的 marker 算法
5. **leader_elect**（中）—— 理解"显式选举"的两种经典做法
6. **twopc**（中）—— 理解"原子提交"与为何它是阻塞的
7. **byzgen**（中难）—— 理解"拜占庭将军"的理论下界与 OM 递归
8. **paxos**（中难）—— 理解"两阶段共识"的心智模型
9. **viewstamped**（难）—— 理解"Primary-Backup 复制"+ 视图变更
10. **zab**（难）—— 理解"zxid 全局有序"与主备原子广播
11. **raft**（难）—— 理解"强 Leader 共识"的工程化设计
12. **byzantine / PBFT**（最难）—— 理解"拜占庭容错"为何需要三阶段

每个算法包都有 `NOTES.md`（论文 + 核心循环 + 判定红线）和 `doc.go`（与相邻算法的区别），建议对照阅读。
