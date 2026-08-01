# consensus-atlas · AGENTS.md

## 项目内容（What）

Go 1.25 纯标准库实现的**分布式系统核心算法教学库**——用最小可运行的代码复刻 12 类经典算法（Raft / Multi-Paxos / Viewstamped / ZAB / PBFT / Byzantine Generals / 2PC / CRDT / Gossip / Chandy-Lamport 快照 / Bully+Ring 选举 / Lamport+Vector Clock），每种配离线 demo + 确定性单测 + 论文笔记 + 横向对比。

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
                     │  Node/Transport │  ← MemTransport 内存网络
                     │  Log/Clock      │
                     └─────────────────┘
```

**做**：12 类算法的核心循环实现、共享 core 底座、离线可跑 demo（内存网络，确定性轨迹）、每种 NOTES.md 论文笔记 + 判定红线 + 对比表、横向等价/对比分析。
**不做**：生产级可靠性（持久化 / snapshot / 成员变更 / view-change / 真签名），那是 etcd/Consul 的领域；Web/服务化（见 go-rmm / auto-finance-assistant）；真实网络传输（M1 用内存实现）。

## 目标（Goal）

- **G1**：12 类算法每类都有"少了就不算该算法"的最小可识别实现（判定红线见各 NOTES.md）。
- **G2**：所有 demo 离线可跑（默认内存网络，确定性），`go run ./cmd/atlas -d <name>` 一键演示。
- **G3**：12 类算法建在同一 core 类型上，差异在循环结构里可见，可横向对比（见 docs/equivalence.md）。
- **G4**：每个算法包 5 件套（impl + test + demo + doc.go + NOTES.md）齐全。
- **成功标准**：`go test ./...` 全绿 + 全 demo 离线跑通 + 每算法 NOTES.md + docs/ 对比文档。

## 当前情况（Status）

- **完成度**：**M1 全量 12 算法完成**——骨架 + 底座 + 12 算法包 + demo + 测试 + 文档
- **底座**（`internal/core`，已完成）：
  - `Node`：NodeID / NodeState（Follower/Candidate/Leader/Primary/Replica）
  - `Transport`：内存网络抽象（Send/Install/Drain），`MemTransport` 显式 tick 推进保证确定性
  - `Log`：复制日志（Append/LastIndex/LastTerm/At/Truncate/Slice）
  - `Clock`：Lamport 逻辑时钟（Tick/Observe/Now）
- **12 算法**（均已完成，每个 5 件套）：
  - **共识（Crash fault）**：Raft（Leader 选举 + 日志复制 + commitIndex 推进）/ Multi-Paxos（两阶段 Prepare/Accept）/ Viewstamped Replication（Primary-Backup，view/opNumber，含视图变更）/ ZAB（ZooKeeper Atomic Broadcast，zxid=epoch|counter 顺序广播，broadcast 阶段）
  - **共识（Byzantine fault）**：PBFT（三阶段 pre-prepare/prepare/commit，2f+1 quorum，含拜占庭场景测试）/ Byzantine Generals OM（Lamport 1982 口头消息递归算法，n≥3f+1 容忍 f 叛徒）
  - **分布式事务**：2PC（两阶段提交，Coordinator/Participant，unanimity quorum，阻塞协议）
  - **无冲突复制**：CRDT G-Counter（Grow-only Counter，merge 取 max，最终收敛）
  - **状态传播**：Gossip Push-Pull 反熵（round-robin 选邻居保证确定性，最终收敛）
  - **全局状态**：Chandy-Lamport 分布式快照（marker 算法，FIFO 通道下记录一致 CUT）
  - **选举**：Bully（ID 最大者当选）+ Ring（环上消息传递收集 ID）
  - **时序**：Vector Clock（N 维向量，Compare 判 HappensBefore/After/Concurrent）+ 复用 core.LamportClock
- **文档**：每算法 NOTES.md + docs/RESEARCH_SUMMARY.md（家族全景 + 选型指南）+ docs/equivalence.md（等价/包含/对比关系）+ examples/README（用法 + 学习路径）

## 技术栈与架构

- **语言**：Go 1.25.6
- **依赖**：**零外部依赖**（module 只引标准库：context / fmt / sync）
- **设计参考**：go-agent-research（同范式的 Agent 教学库，本库目录结构/5 件套/NOTES.md 全部对齐它）
- **目录**：cmd + internal（参照 go-agent-research），12 算法各自独立包，只共享 internal/core

```
consensus-atlas/
├── cmd/atlas/main.go          # -d <name> 统一 demo 入口
├── internal/
│   ├── core/                  # Node/Transport/MemTransport/Log/Clock
│   ├── raft/                  # Raft: raft.go + raft_test.go + demo.go + doc.go + NOTES.md
│   ├── paxos/                 # Multi-Paxos: 同 5 件套
│   ├── viewstamped/           # Viewstamped Replication: 同 5 件套
│   ├── zab/                   # ZAB (ZooKeeper Atomic Broadcast): 同 5 件套
│   ├── byzantine/             # PBFT: 同 5 件套
│   ├── byzgen/                # Byzantine Generals (OM 口头消息): 同 5 件套
│   ├── twopc/                 # 两阶段提交: 同 5 件套
│   ├── crdt/                  # G-Counter: 同 5 件套
│   ├── gossip/                # Gossip: 同 5 件套
│   ├── snapshot/              # Chandy-Lamport 快照: 同 5 件套
│   ├── leader_elect/          # Bully + Ring: 同 5 件套
│   └── clock/                 # Vector Clock + 复用 Lamport: 同 5 件套
├── docs/{RESEARCH_SUMMARY.md, equivalence.md}
└── examples/README.md
```

## 如何运行

```bash
make test                           # 全部测试
make test-race                      # 竞态检测（需 CGO）
make vet                            # 静态检查
go run ./cmd/atlas -d raft          # Raft demo（5 节点选举 + 日志复制 + commit）
go run ./cmd/atlas -d paxos         # Multi-Paxos 两阶段共识
go run ./cmd/atlas -d viewstamped   # Viewstamped Replication（Primary-Backup + 视图变更）
go run ./cmd/atlas -d zab           # ZAB（ZooKeeper 原子广播，zxid 顺序提交）
go run ./cmd/atlas -d pbft          # PBFT 三阶段拜占庭容错
go run ./cmd/atlas -d byzgen        # Byzantine Generals（OM 口头消息算法）
go run ./cmd/atlas -d twopc         # 两阶段提交（原子提交协议）
go run ./cmd/atlas -d crdt          # G-Counter CRDT（max 合并最终收敛）
go run ./cmd/atlas -d gossip        # Gossip 反熵最终收敛
go run ./cmd/atlas -d snapshot      # Chandy-Lamport 分布式快照
go run ./cmd/atlas -d bully         # Bully + Ring 选举
go run ./cmd/atlas -d clock         # Lamport + Vector Clock 因果序对比
go run ./cmd/atlas -d all           # 跑全部 12 个 demo
make build                          # 构建到 bin/consensus-atlas
```

所有 demo **离线可跑**，零外部依赖。

## 关键约定

- **零外部依赖是灵魂约束**：go.mod 无 require，纯标准库实现。算法逻辑不被框架遮蔽。
- **确定性优先**：所有 demo 用 `core.MemTransport` + 显式 `Drain()` 推进网络 tick，不用 goroutine / time / 不可控 rand。同一份代码每次跑轨迹相同。
- **每算法包互不 import**：只共享 `internal/core`，差异在算法本身可见。
- **5 件套齐全**：每个算法包必须含 `impl.go` + `_test.go` + `demo.go` + `doc.go` + `NOTES.md`。
- **判定红线**：每个 NOTES.md 写明"少了什么就不算该算法"，避免实现跑偏。
- **教学清晰，非生产**：刻意省略生产特性（持久化/snapshot/成员变更/真签名/view-change），聚焦核心循环；NOTES.md 明确标注"本包简化"。

## 与其他项目的关系

- **与 [`go-agent-research`](../go-agent-research) 同范式**：两者都是"纯标准库 + 5 件套 + NOTES.md + demo 离线可跑"的教学库，本库的目录结构/文档风格全部对齐它。go-agent-research 复刻 Agent 范式，本库复刻分布式系统算法，互不依赖。
- **与 [`rust-agent-research`](../rust-agent-research)**：同上，是 Agent 教学库的 Rust 版。
- **与 [`go-rmm`](../go-rmm)**：go-rmm 的 relay/proto 用反向 WS 做真实远程传输；本库的 `core.MemTransport` 是教学用的内存网络，理念同源（消息 + 传输抽象）但实现极简。
- **工作区定位**：补全工作区在"分布式系统基础设施教学"象限的空白（工作区此前在 Agent/3D游戏/科学模拟/哲学计算很厚，但共识/复制/选举/时序这块是零覆盖）。
