# 算法等价与对比关系

> 跨算法横向对比，回答"什么时候它们是一样的，什么时候不一样"。

## 一、共识算法三件套：Raft / Multi-Paxos / PBFT

这三种都解决"多节点对一个值/命令序列达成一致"的核心问题，但容错假设不同。

### 1.1 容错模型对比

| 维度 | Raft | Multi-Paxos | PBFT |
|------|------|------------|------|
| 故障模型 | Crash fault | Crash fault | **Byzantine fault** |
| 容忍比例 | < n/2 crash | < n/2 crash | **< n/3 恶意** |
| quorum | ⌊n/2⌋+1（majority） | ⌊n/2⌋+1（majority） | **2f+1**（f=n-1/3） |
| 4 节点 quorum | 3 | 3 | 3（容忍 1 拜占庭） |
| 7 节点 quorum | 4 | 4 | 5（容忍 2 拜占庭） |
| 消息复杂度 | O(n) | O(n) | **O(n²)** |

**洞察**：n=4 时三者 quorum 都=3，但含义不同：
- Raft/Paxos：容忍 1 个**crash**（4-3=1）
- PBFT：容忍 1 个**任意恶意**（4=3f+1, f=1），安全性来自 2f+1=3 个诚实签名覆盖

### 1.2 Leader 模型对比

| | Raft | Multi-Paxos | PBFT |
|---|------|------------|------|
| Leader 强度 | **强 Leader**（写必经） | 弱协调者 | Primary（轮换） |
| 心跳 | AppendEntries 空条目 | 需独立 KeepAlive | 空 pre-prepare / view-change |
| Leader 变更 | 随机超时选举 | view-change（复杂） | view-change（复杂） |

**洞察**：Raft 的核心设计哲学就是"强 Leader 简化一切"——所有流量经 Leader，所以状态少。Paxos/PBFT 允许多 Proposer 并发，状态机更复杂。

### 1.3 阶段数对比

| | 阶段 | 说明 |
|---|------|------|
| Raft | 1（隐式） | Leader 写本地日志 + 复制（算 1 轮 RPC） |
| Multi-Paxos | 2 | Phase 1 Prepare/Promise + Phase 2 Accept/Accepted（稳定 Leader 后 Phase 1 可省） |
| PBFT | 3 | pre-prepare + prepare + commit（多一层 commit 防 Leader 作恶） |

**洞察**：阶段数 ↑ = 安全性 ↑（容忍的恶意程度↑）但延迟 ↑。PBFT 三阶段的原因：crash 模型下两阶段够（prepare 后即可执行），拜占庭模型下 Leader 可能在 prepare 后撒谎，需要 commit 阶段让 2f+1 节点确认"prepare 已被确认"。

---

## 二、选举算法：Bully / Ring / Raft

| | Bully | Ring | Raft 选举 |
|---|------|------|-----------|
| 触发 | 显式（发现 Leader 失联） | 显式 | 隐式（随机超时） |
| 假设 | 成员已知、ID 可比 | 成员已知、组成逻辑环 | 成员已知 |
| 消息复杂度 | O(n²) 最坏 | O(n²) | O(n) |
| 当选者 | ID 最大者（在线） | ID 最大者 | 第一个超时的（非确定） |
| 活锁风险 | 低 | 低 | 用随机化避免 |

**洞察**：Bully/Ring 是"确定性强主"——明确选出最高 ID；Raft 是"概率性弱主"——谁先超时谁当，靠随机化避免冲突。现代分布式系统多用 Raft 式（因为不需要严格的"最优 Leader"）。

---

## 三、时钟：Lamport / Vector

| | Lamport | Vector |
|---|---------|--------|
| 表示 | 单整数 C | N 维向量 V |
| 空间 | O(1) | O(N) |
| 本地事件 | C++ | V[self]++ |
| 收消息 | C = max(C, msg.C)+1 | V[i] = max(V[i], msg.V[i]) ∀i; V[self]++ |
| 能判并发？ | ❌ | ✅ |
| 关系 | a→b ⇒ C(a)<C(b) | a→b ⇔ V(a)<V(b)（充要） |

**洞察**：Lamport 的不等号是"半"的（只保证必要不保证充分），所以无法反推并发。Vector Clock 补全了信息——每个分量对应一个节点的进展，两个向量互不 ≤ 才判并发。代价是 O(N) 存储。

---

## 四、Gossip 与共识的正交性

Gossip **不是**共识算法，它解决的是不同的"agreement"：

| | 共识（Raft/Paxos/PBFT） | Gossip |
|---|------------------------|--------|
| 保证 | 所有正确节点**最终决定相同值**，且值不再变 | 所有节点**最终收到**某信息 |
| 顺序 | 保证全序（状态机复制） | 不保证顺序 |
| 收敛 | 一轮 RPC | O(log N) 轮 |
| 冲突 | 不允许（Leader 决定） | 允许（合并规则决定） |

**洞察**：Gossip 可与共识叠加——例如用 Gossip 传播"Leader 变更通知"，用 Raft 维护"已提交日志"。Serf/Cassandra 用纯 Gossip 做成员发现；etcd 用 Raft 做强一致存储。

---

## 五、一张图看全

```
        强一致共识                          最终一致传播
   ┌──────────────────┐                ┌────────────────┐
   │  Crash fault     │                │                │
   │  ┌─────┐ ┌─────┐ │                │    Gossip      │
   │  │Raft │ │Paxos│ │  ◀── 正交 ──▶  │  (反熵/Push-Pull)│
   │  └─────┘ └─────┘ │                │                │
   │  ┌─────┐ ┌─────┐ │                │  ┌──────────┐  │
   │  │ VR  │ │ ZAB │ │                │  │  CRDT    │  │
   │  └─────┘ └─────┘ │                │  │ (G-Counter)│ │
   └──────────────────┘                └────────────────┘
   │ Byzantine fault                    │
   ▼                                    │ 时序基础                        │
   ┌──────────────────┐                ▼                                │
   │      PBFT        │           ┌─────────┐ ┌─────────┐               │
   │  (3 阶段, 2f+1)  │           │ Lamport │ │ Vector  │               │
   │  + Byz Gen (OM)  │           │  Clock  │ │  Clock  │               │
   └──────────────────┘           └─────────┘ └─────────┘               │
       ▲                                                       │       │
       │ 含子问题：选举                                        │       │
       │                                                       │       │
   ┌───┴────────────┐          ┌──────────────┐                │       │
   │ Bully │ Ring   │          │   2PC        │  ◀── Vector ⊋ Lamport ──┘
   │ (显式强主)      │          │ (原子提交)   │
   │ vs Raft(隐式)   │          └──────────────┘
   └────────────────┘                ▲
                                     │ 记录全局状态
                              ┌──────────────┐
                              │  Snapshot    │
                              │ (Chandy-Lam) │
                              └──────────────┘
```

---

## 六、新六算法与已有算法的关系

> M1 扩展阶段新增的 6 个算法（twopc / crdt / byzgen / snapshot / viewstamped / zab）与前面 6 个的关系。

### 6.1 Viewstamped Replication / ZAB —— 与 Raft/Paxos 数学等价

VR（1988，比 Raft 早 26 年）、ZAB（ZooKeeper）、Raft、Multi-Paxos 四者**数学等价**——都是 majority quorum 的强一致共识。差异在工程取舍：

| 维度 | Raft | Multi-Paxos | Viewstamped | ZAB |
|------|------|------------|-------------|-----|
| 排序单位 | term + log index | 槽位（每槽独立） | view + opNumber | epoch + counter（zxid） |
| Leader 称呼 | Leader | Proposer/Distinguished | Primary | Leader |
| 设计起点 | 可理解性 | 通用性 | 复制状态机 | 主备原子广播 |
| 换主 | 随机超时选举 | view-change | view-change（StartView）| phase 化（discovery/sync）|
| 正常路径轮数 | 1 | 2（稳定后 1） | 1（PrepareOK quorum） | 1（Proposal→Ack→Commit）|

**洞察**：把这四个并列学，能看清"共识"这个抽象的不同具象——同一数学骨架（quorum + 复制 + commit），四种工程皮肤。

### 6.2 2PC —— 不是共识，是原子提交（更弱）

| 维度 | 共识（Raft/Paxos/VR/ZAB） | 2PC |
|------|---------------------------|-----|
| quorum | majority（< n/2 故障） | **unanimity**（全部同意） |
| 容忍故障 | 多数派可达即推进 | **零容忍**（任一参与方故障即阻塞） |
| 阻塞性 | 非阻塞（活节点能选主） | **阻塞**（Coordinator 崩溃则参与者锁定资源） |
| 目标 | 对一个值达成一致 | 一个事务要么全提交要么全放弃 |

**洞察**：2PC 比共识**更弱**（要求更苛刻的 unanimity，容错更差）。它解决的是数据库事务的原子性，不是"选出唯一值"。3PC 加一轮 Pre-Commit 试图解决阻塞问题（本库未实现，列在 M2 候选）。

### 6.3 CRDT —— 与 Gossip 正交可组合

| | Gossip | CRDT (G-Counter) |
|---|--------|------------------|
| 是什么 | 传输协议（怎么扩散） | 数据语义（怎么合并才收敛） |
| 保证 | 消息最终到达全网 | 状态最终一致（收敛） |
| 合并 | 任意（由应用定义） | max（数学保证收敛） |
| 依赖 | 需要连通的图 | 需要可交换/结合/幂等的合并函数 |

**洞察**：两者正交——Gossip 管"传"，CRDT 管"合并"。Gossip + CRDT 是分布式计数器/集合的经典组合（Riak/Dynamo 的数据类型）。CRDT 与共识（Raft/Paxos）是对立哲学：共识要强一致付出协调代价，CRDT 要高可用放弃强一致靠最终收敛。

### 6.4 Byzantine Generals (OM) —— PBFT 的理论前身

| | Byzantine Generals (OM) | PBFT |
|---|------------------------|------|
| 年代 | Lamport 1982 | Castro-Liskov 1999 |
| 算法 | 口头消息递归 OM(m) | 三阶段 pre-prepare/prepare/commit |
| 消息复杂度 | O(n^(m+1))（指数） | O(n²)（多项式） |
| 目标 | 证明 n≥3f+1 下界 + 给出可解算法 | 实用化（把指数降到多项式 + 支持连续请求） |
| 关系 | PBFT prepare 阶段 = OM 的"间接互传 + majority"的工程化 | — |

**洞察**：OM 是"理论极限"——证明能解、给出第一个算法，但指数消息量无法实用。PBFT 是"工程突破"——保留同样的容错下界（n≥3f+1），把消息量压到 O(n²)，使拜占庭容错首次能在真实系统（联盟链）部署。

### 6.5 Chandy-Lamport Snapshot —— 与共识/传播都不同的第三类

| | 共识 | Gossip/CRDT | Chandy-Lamport 快照 |
|---|------|-------------|---------------------|
| 目标 | 对一个值达成一致 | 把状态扩散/收敛到全网 | 记录一个一致的全局状态 |
| 是否"达成一致" | 是 | 否（只扩散） | 否（只记录） |
| 假设 | crash 或 byzantine | 连通图 | FIFO 可靠通道 + crash 模型 |
| 输出 | 已提交的值 | 各副本相同状态 | 一个 CUT（可能从未真实同时存在过） |

**洞察**：快照不改变系统状态，只观察。它的难点是"不停机地观察到一个一致切面"——marker 消息把消息历史切成快照前/快照后，FIFO 保证切割点良定义。这是死锁检测、全局 checkpoint、分布式调试的基础设施，与共识/传播正交。
