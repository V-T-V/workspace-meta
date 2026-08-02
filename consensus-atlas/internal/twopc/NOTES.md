# Two-Phase Commit (2PC) · 设计笔记

## 论文

- **Notes on Data Base Operating Systems**
- 作者：Jim Gray
- Operating Systems: An Advanced Course, Lecture Notes in Computer Science 60, Springer 1978
- 2PC 的经典出处：https://www.microsoft.com/en-us/research/people/gray/
  （Gray 在此笔记里系统阐述了用于分布式事务原子提交的两阶段协议）

- **Concurrency Control in Distributed Database Systems**（综述）
- 作者：Philip A. Bernstein, Nathan Goodman
- ACM Computing Surveys, Vol. 13, No. 2, 1981
- https://dl.acm.org/doi/10.1145/356842.356846

- 经典教材：Bernstein, Hadzilacos & Goodman, *Concurrency Control and Recovery
  in Database Systems*, Addison-Wesley 1987（第 7 章详述 2PC 与 3PC）

## 核心循环

### 两阶段（Prepare → Commit/Abort）

```
Coordinator (Begin 事务 T):
  for each participant p:
      send Prepare(T) → p                  # 阶段一：能否提交？

Participant (收到 Prepare):
  if 能提交（资源可锁、约束满足）:
      state = Prepared                      # 锁资源 + 持久化（durability）
      reply Vote(Yes)
  else:
      state = Aborted
      reply Vote(No)                        # 任一 No 即夭折

Coordinator (收齐 Vote):
  if 任一 No 或 超时:
      decide = Abort
  elif 全部 Yes:                            # unanimity：要全票，不是多数派
      decide = Commit
  for each participant p:
      send decide → p                       # 阶段二：下发决定

Participant (收到 Commit/Abort):
  if Commit and state==Prepared:
      state = Committed                     # 履行承诺
  elif Abort and state!=Committed:
      state = Aborted
  reply Ack                                 # 确认已落实
```

## 最小可识别特征（少了就不算 2PC）

1. **两个明确阶段**：先 Prepare/Vote（征询），后 Commit/Abort（落实）。一阶段直接
   提交不是 2PC；三阶段（加一轮 Pre-Commit）是 3PC。
2. **一致同意（unanimity）**：阶段一必须**全部** Participant 投 Yes 才 Commit。
   用多数派（⌊N/2⌋+1）通过的是共识算法，不是 2PC。
3. **Coordinator 主导 + Participant 承诺**：Coordinator 单点决定事务命运；
   Participant 投 Yes 后进入 Prepared，承诺能履行 Commit（即使故障恢复也要补提交）。
4. **Prepare 后资源被锁**：Yes 票=已锁定资源。在 Coordinator 下发决定前，Participant
   不能单方面释放（否则违反 atomicity）。

## 判定红线

- **只有一轮（无 Prepare）直接 Commit** → 不是 2PC，是一阶段提交（脆弱、不原子）。
- **用多数派代替一致同意**（如 2/3 Yes 即 Commit）→ 不是 2PC，那是共识算法的语义。
  2PC 的 quorum 永远是 **N（全部）**，不是 ⌊N/2⌋+1。
- **Coordinator 故障后 Participant 不阻塞** → 违反 2PC 的阻塞性质。真实 2PC 中，
  Coordinator 在两阶段间崩溃会让已 Prepared 的 Participant 无限等待（blocking），
  这是 2PC 的著名缺陷，3PC 试图解决。本教学库为确定性不演示此故障场景，但须知
  这是 2PC 的固有缺陷而非实现疏漏。
- **Participant 投 Yes 后不锁资源 / 不持久化** → 违反 durability 假设，崩溃恢复后
  无法履行 Commit，破坏 atomicity。本包用状态机转移（Prepared→Committed）模拟此约束。

## 与共识算法对比（本仓库 Raft/Paxos vs 2PC）

| 维度 | 2PC（本包） | 共识（Raft / Paxos / PBFT） |
|------|-------------|----------------------------|
| 目标 | 分布式事务**原子提交**（全做或全不做） | 多节点对**单个值/日志**达成一致 |
| quorum | 一致同意（unanimity，全部 N） | 多数派（Raft/Paxos）或 2f+1（PBFT 抗拜占庭） |
| 容忍故障 | 不容忍参与方故障（任一故障即阻塞/Abort） | 容忍少数派故障仍可工作 |
| 阻塞性 | 阻塞（Coordinator 崩溃则 Participant 卡死） | 非阻塞（Leader 挂了可重选） |
| 角色模型 | Coordinator + Participant（不对称） | 对等节点（Raft 临时 Leader / Paxos Proposer-Acceptor） |
| 典型场景 | 跨分片事务、XA、分布式数据库提交点 | 复制状态机、配置中心、元数据 |

一句话：**2PC 保证事务"要么全做要么全不做"，但代价是任一节点故障就卡住；
共识保证"多数同意即生效"，能用牺牲强一致门槛换取可用性。**

## 本包简化（为教学清晰，非生产）

- **传输层为内存队列**（复用 core.MemTransport 的显式 Drain 推进），非真实 RPC goroutine。
- **不演示 Coordinator 阻塞故障**：真实 2PC 的 Coordinator 崩溃会让 Participant 无限
  等待；本包 Coordinator 永不宕机，专注演示正常路径下的两阶段握手。要观察阻塞需引入
  故障注入，超出教学 demo 范围。
- **无持久化/恢复日志**：Participant 投 Yes 后用内存状态机（Prepared）模拟"已持久化承诺"，
  不写预写日志（WAL）。生产实现（如 XA）必须把 Prepared 状态落盘，崩溃恢复才能补提交。
- **无超时机制**：真实 2PC 的 Participant 在等 Coordinator 决定时会超时，进入"终结协议"
  询问其他 Participant 或协调者。本包由 demo 主循环显式 Drain 推进，无超时。
- **Coordinator 串行处理单事务为主**：虽支持多 TxnID 并发（按 ID 索引 pending 表），
  但 demo 一次跑一个事务以求轨迹清晰。
