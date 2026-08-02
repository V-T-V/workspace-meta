# 分布式系统核心算法 · 研究总结

> consensus-atlas 的算法家族全景 + 选型指南。12 类算法每类对应 `internal/<pkg>/` 一个包，含完整实现 + 离线 demo + 论文笔记（NOTES.md）。

## 一、算法家族全景

按"解决什么问题"分类。注意：很多算法跨多个家族（如 Raft 既做共识又做复制又含选举）。

### 1. 共识（Consensus）—— 多节点对一个值达成一致
| 算法 | 容错模型 | 消息复杂度 | 进度条件 | 论文 |
|------|----------|------------|----------|------|
| **Raft** | Crash fault (< 1/2) | O(n) | 多数派可达 | Ongaro & Ousterhout 2014 |
| **Multi-Paxos** | Crash fault (< 1/2) | O(n)（稳定 Leader 后） | 多数派可达 | Lamport 1998/2001 |
| **Viewstamped Replication** | Crash fault (< 1/2) | O(n) | 多数派可达 | Oki 1988 / Liskov 2012 |
| **ZAB** | Crash fault (< 1/2) | O(n)（broadcast 阶段） | 多数派可达 | Junqueira 2011 |
| **PBFT** | Byzantine fault (< 1/3) | O(n²) | 2f+1 可达 | Castro & Liskov 1999 |

注：**2PC**（两阶段提交）是"原子提交协议"而非严格共识——它要求**全部**参与方同意（unanimity）而非多数派，无法容忍任何参与方故障，且 Coordinator 崩溃会阻塞。**Byzantine Generals (OM)** 是 PBFT 的理论前身（Lamport 1982），证明 n≥3f+1 是口头消息模型下容忍 f 个拜占庭节点的下界。

**核心权衡**：容错的"恶意程度"↑ → 需要的 quorum ↑（1/2→1/3）→ 消息复杂度 ↑（O(n)→O(n²)）。Raft/Paxos/VR/ZAB 不能容忍哪怕一个拜占庭节点；PBFT 能，代价是每轮 O(n²) 消息。Raft/Paxos/VR/ZAB 四者数学等价（都是 majority quorum 共识），差异在工程取舍：Raft 重可理解性、Paxos 重通用性、VR 重状态机视角、ZAB 重 zxid 全局有序。

### 2. 状态传播与复制（Dissemination & Replication）—— 信息扩散到全网 / 数据收敛
| 算法 | 一致性 | 收敛轮数 | 适用规模 |
|------|--------|----------|----------|
| **Gossip**（反熵 Push-Pull） | 最终一致 | O(log N) | 极大（万级） |
| **CRDT (G-Counter)** | 最终一致 | 视同步频率 | 任意 |

共识算法（Raft/Paxos）也能广播，但它们为"强一致"付出 O(n) 同步代价，扩展性受限。Gossip 牺牲强一致换扩展性——像传染病一样扩散，O(log N) 轮覆盖全网，是大规模集群状态同步的首选。**CRDT** 解决的是"扩散后状态如何合并才收敛"——靠合并函数的可交换/可结合/幂等性质（G-Counter 用 max），与 Gossip 正交可组合（Gossip 传输 + CRDT 语义 = Riak/Dynamo 的数据类型）。

### 3. 选举（Leader Election）—— 选出一个协调者
| 算法 | 假设 | 消息复杂度 | 特点 |
|------|------|------------|------|
| **Bully** | 已知成员、可比 ID | O(n²) 最坏 | ID 最大者当选，简单粗暴 |
| **Ring** | 逻辑成环 | O(n²) | 消息沿环传递，最大 ID 当选 |
| **Raft 选举** | 随机超时 | O(n) | 隐式选举，无显式 Coordinator |

Bully/Ring 是"显式选举"（成员已知、发消息比 ID），Raft 是"隐式选举"（随机超时觉醒）。现代系统（如 etcd）多用 Raft 式隐式选举；Bully/Ring 在固定成员的小集群（如数据库主从切换）仍有应用。

### 4. 时序与因果（Time & Causality）—— 给分布式事件定序
| 算法 | 输出 | 能否判并发 |
|------|------|-----------|
| **Lamport Clock** | 单整数时间戳 | ❌ 只保证 a→b ⇒ C(a)<C(b)，不能反推 |
| **Vector Clock** | N 维向量 | ✅ 精确判断 a→b / b→a / 并发 |

逻辑时钟解决"没有全局物理时间如何定序"。Lamport 简单但信息量少；Vector Clock 用 N 维向量精确刻画因果，代价是存储 O(N)。实践中 Lamport 用于排序日志，Vector Clock 用于检测冲突（如 Dynamo、Riak）。

---

## 二、选型指南

### 按场景选

| 场景 | 推荐 | 理由 |
|------|------|------|
| 强一致的小集群（< 10 节点）状态机复制 | **Raft** | 易理解、强 Leader、O(n) 消息；etcd/Consul 在用 |
| 需要多 Proposer 竞争的共识 | **Multi-Paxos** | 比 Raft 灵活，但更复杂 |
| ZooKeeper 风格的顺序读写 | **ZAB** | zxid 全局单调有序，主备原子广播 |
| 状态机视角的复制 + 显式视图变更 | **Viewstamped Replication** | Primary-Backup 模型，view/opNumber 语义清晰 |
| 联盟链/需容忍恶意节点 | **PBFT** | 三阶段 + 2f+1 抗拜占庭；Hyperledger Fabric 在用 |
| 研究拜占庭容错的理论下界 | **Byzantine Generals (OM)** | Lamport 1982 原始算法，证明 n≥3f+1 |
| 跨节点事务的原子提交（数据库） | **2PC** | unanimity 保证原子性，但阻塞 |
| 大规模计数器/集合的高可用复制 | **CRDT** | 写永不阻塞，最终收敛；Riak/Dynamo 在用 |
| 大规模状态同步（> 100 节点） | **Gossip** | 扩展性最好，Cassandra/Serf 在用 |
| 记录一致的全局状态（死锁检测/checkpoint） | **Chandy-Lamport 快照** | marker 算法，不停机记录 CUT |
| 固定成员集群选主 | **Bully**（简单）/ **Ring**（无中心） | 比 Raft 选举更直接 |
| 给事件定序/检测因果冲突 | **Vector Clock**（精确）/ **Lamport**（轻量） | 视是否需判并发而定 |

### 按容错需求选

```
节点会做什么？
├─ 只会 crash（宕机/断网）→ Crash fault
│   ├─ 集群 < 1/2 可能 crash → Raft / Multi-Paxos（quorum = majority）
│   └─ 集群多数可达 → 进度保证
└─ 可能恶意（拜占庭）→ Byzantine fault
    ├─ 集群 < 1/3 恶意 → PBFT（quorum = 2f+1）
    └─ 需要 O(n²) 消息换安全性
```

---

## 三、算法间的等价与包含关系

```
           强一致共识
          ┌──────────┐
          │  Raft    │ ◀── Raft ⊂ Paxos（Ongaro 论文明确：Raft 是 Paxos 的改进版）
          │  Paxos   │
          │  PBFT    │ ◀── PBFT 是 Paxos 的拜占庭版（把 quorum 从 majority 提到 2f+1）
          └────┬─────┘
               │ 含 Leader 选举子问题
               ▼
          ┌──────────┐
          │  Bully   │ ◀── Bully/Ring 是"显式选举"
          │  Ring    │     Raft 的选举是"隐式（随机超时）"——不在此类
          └──────────┘

     最终一致传播
          ┌──────────┐
          │  Gossip  │ ◀── 与共识正交，不保证顺序，只保证最终扩散
          └──────────┘

     时序基础设施
          ┌──────────┐
          │ Lamport  │ ◀── Vector Clock ⊋ Lamport（信息更多，能判并发）
          │ Vector   │
          └──────────┘
```

**关键洞察**：
- **Raft ⊂ Paxos**：Raft 论文的贡献就是把 Multi-Paxos 重新设计得更易理解，二者等价（都解决 crash fault 共识）。
- **PBFT = Paxos + 拜占庭**：把"多数派"换成"2f+1"，把"信任消息内容"换成"验证签名"。
- **Gossip 与共识正交**：共识解决"对所有节点一致的值"，Gossip 解决"值扩散到所有节点"，可叠加使用（如用 Gossip 传播 Raft 的 Leader 变更）。
- **Vector ⊋ Lamport**：Vector Clock 多出的信息（各分量）正好用来判断并发，Lamport 做不到。

---

## 四、M1 实现清单（12 算法）

| 包 | 算法 | 家族 | 状态 |
|----|------|------|------|
| `internal/raft` | Raft（Leader 选举 + 日志复制） | 共识（Crash） | ✅ M1 |
| `internal/paxos` | Multi-Paxos（两阶段 Prepare/Accept） | 共识（Crash） | ✅ M1 |
| `internal/viewstamped` | Viewstamped Replication（Primary-Backup + 视图变更） | 共识（Crash） | ✅ M1 |
| `internal/zab` | ZAB（zxid=epoch\|counter 顺序广播，broadcast 阶段） | 共识（Crash） | ✅ M1 |
| `internal/byzantine` | PBFT（三阶段 + 2f+1，含拜占庭场景测试） | 共识（Byzantine） | ✅ M1 |
| `internal/byzgen` | Byzantine Generals OM（口头消息递归，n≥3f+1） | 共识（Byzantine） | ✅ M1 |
| `internal/twopc` | 2PC（两阶段提交，unanimity 阻塞） | 分布式事务 | ✅ M1 |
| `internal/crdt` | CRDT G-Counter（max 合并最终收敛） | 无冲突复制 | ✅ M1 |
| `internal/gossip` | Gossip Push-Pull 反熵 | 状态传播 | ✅ M1 |
| `internal/snapshot` | Chandy-Lamport 快照（marker + FIFO CUT） | 全局状态 | ✅ M1 |
| `internal/leader_elect` | Bully + Ring 选举 | 选举 | ✅ M1 |
| `internal/clock` | Lamport + Vector Clock | 时序 | ✅ M1 |

## 五、M2 候选（未来扩展）

- 3PC（三阶段提交，非阻塞原子提交，解决 2PC 的 Coordinator 单点阻塞）
- CRDT 其他变种（OR-Set / PN-Counter / LWW-Register）
- PBFT view-change / 区块链共识（PoW/PoS 工程化变种）
- 成员变更 / 动态重配置（Raft joint consensus 等）
- 真实网络传输层（参考 go-rmm 的 relay/proto，替换 core.MemTransport）
