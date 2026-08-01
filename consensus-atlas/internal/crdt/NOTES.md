# G-Counter CRDT · 设计笔记

## 论文

- **A comprehensive study of Convergent and Commutative Replicated Data Types**
- 作者：Marc Shapiro, Nuno Preguiça, Carlos Baquero, Marek Zawirski
- INRIA Research Report 7506, 2011（后扩展发表于 Theoretical Computer Science 2011）
- https://hal.inria.fr/inria-00555588/

> 这篇报告系统化了 CRDT 理论：把"最终一致"形式化为"半格（join-semilattice）合并"，
> 只要合并函数满足交换 + 结合 + 幂等，副本间任意顺序互相同步都能收敛。G-Counter
> 是其中最简单的 state-based CRDT（CvRDT）。

## 核心循环

### 本地写（无需协调）

```
节点 X（持有向量 Vx）:
  Vx[X] += n                              # 只改自己那一维，单调递增（grow-only）
  # 立即可读，不阻塞、不等任何节点
```

### 同步与合并（Push-Pull）

```
发起方 X（每个 Tick）:
  peer = pickNeighbor()                   # 随机挑邻居；本包 round-robin
  send StateRequest(Vx) → peer            # Push：把全量向量送过去

接收方 Y（收到 StateRequest）:
  for each node i:
      Vy[i] = max(Vy[i], Vx[i])           # 逐分量取 max（不是 sum！）
  reply StateResponse(Vy) → X             # Pull：回送合并后的向量

发起方 X（收到 StateResponse）:
  for each node i:
      Vx[i] = max(Vx[i], Vy[i])           # 同样取 max

# 一次 Request/Response 往复完成 X、Y 双向同步。
# N 个节点各自周期性重复，所有副本最终收敛到相同向量。

读全局值:
  value = sum(V[*])                       # 所有分量求和
```

### 为什么 max 而不是 sum

```
错误（sum）：X 持有 {a:3}，Y 持有 {a:3}。
  X.Merge(Y) 用 sum → {a:6}              # 把对方的 a 又加了一遍 → 重复计数！
正确（max）：X.Merge(Y) 用 max → {a:3}    # 幂等，重复合并不变 → 收敛。
```

max 满足**交换律**（max(a,b)=max(b,a)）、**结合律**（max(max(a,b),c)=max(a,max(b,c))）、
**幂等律**（max(a,a)=a）。这三条使合并顺序无关、重复无害——CRDT 收敛性的数学根基。

## 最小可识别特征（少了就不算 G-Counter）

1. **向量计数**：每个节点一维，全局值 = 各维求和（不是单一标量）。
2. **grow-only**：每个分量只增不减（Increment 只改自己那一维，永不回退）。
3. **Merge = max**（逐分量取上界），不是 sum、不是平均、不是 last-writer-wins。
4. **无需协调即可收敛**：本地写不等任何节点；同步靠合并而非共识。

## 判定红线

- **分量能减**（如支持 Decrement）→ 不是 G-Counter。带减法的计数器是 PN-Counter
  （用两个 G-Counter：一个计增 P、一个计减 N，value = sum(P) - sum(N)），见论文 §3.3。
- **Merge 用 sum** → 会重复计数，违反幂等，**不能收敛**。这是最常见的实现错误。
- **本地写需要等协调者 / 多数派** → 那是共识算法，不是 CRDT。CRDT 的写永不需要协调。
- **要求读到的值即时强一致** → CRDT 不保证；刚写的值可能要等几轮同步才对其他副本可见。
  要强一致请用 Raft/Paxos。

## 与共识算法对比（本仓库 Raft/Paxos/PBFT vs CRDT）

| 维度 | 共识（Raft / Paxos / PBFT） | CRDT（本包 G-Counter） |
|------|----------------------------|------------------------|
| 一致性强度 | 强一致（linearizability） | 最终一致（eventual consistency） |
| 写入路径 | 经 Leader / 多数派 prepare-accept | 任意节点本地写，立即可读 |
| 写延迟 | 高（需往返 quorum） | 极低（本地内存操作） |
| 冲突处理 | 避免冲突（一次只有一个写定序） | 数据结构自身吸收冲突（max 合并） |
| 故障容忍 | 少数派故障可工作；拜占庭需 PBFT | 节点故障只是"暂时不同步"，恢复后追上 |
| 收敛保证 | 已提交数据永不丢、强序 | 不丢（grow-only），但读到最新需等同步 |
| 数学根基 | 多数派 quorum + term/编号 | 半格（join-semilattice）+ 幂等合并 |
| 适用场景 | 账本、配置、状态机复制 | 计数器、集合、购物车、在线状态 |

一句话：**共识追求"对且实时"，CRDT 追求"可用且最终对"。**

## 与 Gossip 的关系（本仓库 gossip 包 vs 本包）

CRDT 与 Gossip 是**正交**的两件事，常被混为一谈：

| 维度 | Gossip（传输层） | CRDT（数据语义层） |
|------|------------------|--------------------|
| 回答的问题 | 状态如何**扩散**到全网 | 扩散来的状态如何**合并**才收敛 |
| 关注点 | 拓扑、选邻居、消息路由 | 合并函数的数学性质（交换/结合/幂等） |
| 是否依赖对方 | 独立存在 | 独立存在 |

本包用 Gossip 式的 Push-Pull 扩散 G-Counter 向量（复用 core.MemTransport），
G-Counter 的 max 合并保证收敛——这是经典的"Gossip + CRDT"组合。gossip 包本身
合并的是键值表（last-writer-wins，字符串序取更大），也是一种退化的 CRDT 合并；
本包把它做成显式的、带向量与因果比较的 G-Counter，更接近 Shapiro 论文的原始定义。

## 本包简化（为教学清晰，非生产）

- **选邻居用 round-robin 而非随机**：`peers[tickCount % len(peers)]`。论文里的
  Gossip 用随机抽样；本包优先保证 demo 轨迹确定（无 rand 依赖）。
- **传全量向量 map 而非增量**：每次消息体积 O(节点数)。生产实现可用 delta-CRDT
  （只传变化的分量）或版本向量削减流量。
- **无故障/恢复/成员变更模型**：节点永不宕机；集群成员固定。
- **只实现 G-Counter**：未实现 PN-Counter（带减法）、G-Set/PN-Set/OR-Set、LWW-Register
  等其他 CRDT。G-Counter 是最简形态，足以演示"max 合并收敛"的核心思想。
- **传输层为内存队列**（复用 core.MemTransport 的显式 Drain 推进），非真实 RPC goroutine。
- **Compare 的因果比较基于向量分量包含关系**（a ⊆ b 当 a 各分量 ≤ b），与向量时钟
  同构；本包未实现完整的向量时钟时间戳打标，仅用 G-Counter 向量本身做偏序判定。
