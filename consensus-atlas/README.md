# consensus-atlas

> 分布式系统核心算法的可运行教学库 —— 用 Go 纯标准库实现 12 类经典算法（Raft / Multi-Paxos / Viewstamped / ZAB / PBFT / Byzantine Generals / 2PC / CRDT / Gossip / Chandy-Lamport 快照 / Bully+Ring 选举 / Lamport+Vector Clock），每种配离线 demo + 确定性单测 + 论文笔记。

![Go](https://img.shields.io/badge/Go-1.25-00ADD8?logo=go)
![零依赖](https://img.shields.io/badge/dependencies-0-success)
![license](https://img.shields.io/badge/license-MIT-blue)
![algorithms](https://img.shields.io/badge/algorithms-12-orange)

## 为什么有这个项目

工作区里 `go-agent-research` 把 Agent 范式做成了可运行教学库，但**分布式系统基础设施**（共识、复制、选举、时序）一直是空白。这些算法是 etcd / Consul / Cassandra / Hyperledger 这些系统的骨架，光看论文很难建立直觉——`consensus-atlas` 用最小可运行的实现补上这一环，每个算法都能 `go run` 起来看它怎么运转。

## 算法家族

| 包 | 算法 | 解决什么 | 容错/一致性 | 论文 |
|----|------|----------|-------------|------|
| `internal/raft` | **Raft** | 强一致共识（状态机复制） | Crash fault < 1/2 | [Ongaro 2014](https://raft.github.io/raft.pdf) |
| `internal/paxos` | **Multi-Paxos** | 强一致共识（多 Proposer） | Crash fault < 1/2 | [Lamport 1998](https://lamport.org/pubs/lamport-paxos.pdf) |
| `internal/viewstamped` | **Viewstamped Replication** | 强一致共识（Primary-Backup，第三种主流共识） | Crash fault < 1/2 | [Oki 1988 / Liskov 2012](https://pmg.csail.mit.edu/papers/vr-revisited.pdf) |
| `internal/zab` | **ZAB** | ZooKeeper 原子广播（主备顺序广播） | Crash fault < 1/2 | [Junqueira 2011](https://zookeeper.apache.org/doc/r3.4.13/zab.pdf) |
| `internal/byzantine` | **PBFT** | 拜占庭容错共识（三阶段 + 2f+1） | Byzantine fault < 1/3 | [Castro 1999](https://pmg.csail.mit.edu/papers/osdi99.pdf) |
| `internal/byzgen` | **Byzantine Generals (OM)** | 拜占庭将军问题（口头消息递归算法） | Byzantine fault < 1/3 | [Lamport 1982](https://lamport.org/pubs/byz.pdf) |
| `internal/twopc` | **2PC** | 两阶段提交（原子提交协议） | 无（一致同意，阻塞） | [Gray 1978](https://www.microsoft.com/en-us/research/people/gray/) |
| `internal/crdt` | **CRDT (G-Counter)** | 无冲突复制数据类型（max 合并收敛） | 最终一致 | [Shapiro 2011](https://hal.inria.fr/inria-00555588/) |
| `internal/gossip` | **Gossip** | 状态扩散到全网（Push-Pull 反熵） | 最终一致 | [Demers 1987](https://dl.acm.org/doi/10.1145/41840.41841) |
| `internal/snapshot` | **Chandy-Lamport 快照** | 一致的全局状态记录（marker 算法） | Crash fault（FIFO 通道） | [Chandy 1985](https://lamport.org/pubs/chandy-lamport.pdf) |
| `internal/leader_elect` | **Bully + Ring** | 选出协调者 | 显式选举 | [Garcia-Molina 1982](https://dl.acm.org/doi/10.1109/TC.1982.1675965) |
| `internal/clock` | **Lamport + Vector Clock** | 给分布式事件定序 | 因果序 | [Lamport 1978](https://lamport.org/pubs/time-clocks.pdf) |

## 快速开始

```bash
git clone <repo>
cd consensus-atlas

# 跑单个 demo（默认 raft）
go run ./cmd/atlas -d raft
go run ./cmd/atlas -d paxos
go run ./cmd/atlas -d viewstamped
go run ./cmd/atlas -d zab
go run ./cmd/atlas -d pbft
go run ./cmd/atlas -d byzgen
go run ./cmd/atlas -d twopc
go run ./cmd/atlas -d crdt
go run ./cmd/atlas -d gossip
go run ./cmd/atlas -d snapshot
go run ./cmd/atlas -d bully
go run ./cmd/atlas -d clock

# 跑全部 12 个 demo
go run ./cmd/atlas -d all

# 全部测试
make test   # 或 go test ./...
```

所有 demo **离线可跑**，零外部依赖（纯标准库），不需要任何部署。

## 核心设计

```
                     ┌─────────────────┐
   客户端命令 ───────▶│  共识算法        │
                     │  Raft/Paxos/PBFT │
                     └────────┬────────┘
                              │ 已提交的值
              ┌───────────────┼───────────────┐
              ▼               ▼               ▼
        ┌──────────┐   ┌──────────┐   ┌─────────────┐
        │ 选举      │   │ 传播      │   │ 时序         │
        │ Bully/Ring│   │ Gossip   │   │ Lamport/Vec │
        └──────────┘   └──────────┘   └─────────────┘
                              ▲
                              │ 共享底座
                     ┌────────┴────────┐
                     │ internal/core   │
                     │  Node/Transport │  ← 内存网络，demo 离线可跑
                     │  Log/Clock      │
                     └─────────────────┘
```

### 单个算法的文件结构（5 件套）

```
internal/raft/
├── raft.go          # 实现（Node struct + 两个 RPC 的处理）
├── raft_test.go     # 确定性单测（表驱动 + 明确断言）
├── demo.go          # Demo(ctx) 离线可跑，返回结果摘要
├── doc.go           # Go package doc（算法定义 + 与相邻算法区别）
└── NOTES.md         # 论文笔记（核心循环 ASCII + 判定红线 + 对比表）
```

## 设计原则

1. **零外部依赖是灵魂约束** —— `go.mod` 无 `require`，纯标准库实现。这让算法逻辑透明可读，不被框架/库的魔法遮蔽。
2. **确定性优先** —— 所有 demo 用内存传输（`core.MemTransport`）+ 显式 `Drain()` 推进网络 tick，不用 goroutine / time / 不可控 rand。同一份代码每次跑轨迹相同。
3. **每个算法包互不 import** —— 只共享 `internal/core` 底座，差异在算法本身可见。
4. **教学清晰，非生产** —— 刻意省略生产特性（持久化、snapshot、成员变更、真签名、view-change 等），聚焦"算法的核心循环"。每个 NOTES.md 明确标注"本包简化"。
5. **判定红线** —— 每个算法的 NOTES.md 写明"少了什么就不算该算法"，避免实现跑偏成"形似神不似"。

## 目录结构

```
consensus-atlas/
├── go.mod / Makefile / LICENSE
├── README.md / AGENTS.md
├── cmd/atlas/main.go          # -d <name> 选 demo 的统一入口
├── internal/
│   ├── core/                  # 共享底座：Node/Transport/MemTransport/Log/Clock
│   ├── raft/                  # 5 件套（Raft）
│   ├── paxos/                 # 5 件套（Multi-Paxos）
│   ├── viewstamped/           # 5 件套（Viewstamped Replication）
│   ├── zab/                   # 5 件套（ZAB）
│   ├── byzantine/             # 5 件套（PBFT）
│   ├── byzgen/                # 5 件套（Byzantine Generals / OM）
│   ├── twopc/                 # 5 件套（两阶段提交）
│   ├── crdt/                  # 5 件套（G-Counter）
│   ├── gossip/                # 5 件套（Gossip）
│   ├── snapshot/              # 5 件套（Chandy-Lamport 快照）
│   ├── leader_elect/          # 5 件套（Bully + Ring）
│   └── clock/                 # 5 件套（Lamport + Vector）
├── docs/
│   ├── RESEARCH_SUMMARY.md    # 算法家族全景 + 选型指南
│   └── equivalence.md         # 等价/包含/对比关系
└── examples/README.md         # 用法示例 + 学习路径
```

## 测试与质量

```bash
make test        # go test ./... 全绿
make test-race   # 竞态检测（需 CGO）
make vet         # 静态检查
make fmt         # 格式化
make build       # 构建到 bin/
```

## 路线图

- **M1（已完成）**：12 个核心算法（Raft / Multi-Paxos / Viewstamped Replication / ZAB / PBFT / Byzantine Generals / 2PC / CRDT / Gossip / Chandy-Lamport 快照 / Bully+Ring / Lamport+Vector Clock）
- **M2（候选）**：3PC / CRDT 其他变种（OR-Set / PN-Counter）/ PBFT view-change / 成员变更 / Chandy-Lamport 之外的快照协议
- **不做的**：生产级可靠性（持久化/snapshot/成员变更）—— 那是 etcd/Consul 的领域，本库聚焦教学

## 相关项目

- [`go-agent-research`](../go-agent-research) —— 同范式的 Agent 范式教学库（73 范式），本库的目录结构/5 件套/NOTES.md 风格全部对齐它
- [`rust-agent-research`](../rust-agent-research) —— 上者的 Rust 移植
