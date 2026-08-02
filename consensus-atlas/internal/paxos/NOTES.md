# Multi-Paxos · 设计笔记

## 论文

- **The Part-Time Parliament** — Lamport, ACM TOCS 1998（用希腊岛屿 Paxos 的虚构议会隐喻算法）
  作者主页：https://lamport.org/ · 论文集：https://lamport.org/#qa-pubs · 原文：https://lamport.org/pubs/lamport-paxos.pdf
- **Paxos Made Simple** — Lamport, ACM SIGACT News 2001（把 1998 那篇晦涩的隐喻重写成直白陈述，是入门首选）
  https://lamport.org/pubs/paxos-simple.pdf
- **Paxos Made Live** — Chandra, Griesemer, Redstone, PODC 2007（Google Chubby 工程化经验，论工程上为何要 Multi-Paxos 优化）

## 核心循环

### Phase 1 — Prepare / Promise（选Leader/确定可提案编号）

```
Proposer（想用编号 n 提议）:
  for each acceptor:
      send Prepare(n)

Acceptor（收到 Prepare(n)）:
  if n < HighestPromised:                 # 已承诺更高编号
      reply Promise(promised=false)        # 拒绝
  else:
      HighestPromised = n
      reply Promise(promised=true,
                   acceptedNumber = HighestAccepted,   # 回带已接受的最大编号提案
                   acceptedValue  = AcceptedValue)

Proposer（累计 Promise）:
  promises++
  if acceptedNumber > highestSeenNum:
      highestSeenNum, highestSeenVal = acceptedNumber, acceptedValue
  if promises >= quorum:                   # 多数派承诺
      进入 Phase 2
```

### Phase 2 — Accept / Accepted（确定值并通知 Learner）

```
Proposer（达 quorum 后）:
  value = (highestSeenNum > 0) ? highestSeenVal : self.Value   # 关键：不覆盖已 chosen 的值
  for each acceptor:
      send Accept(n, value)

Acceptor（收到 Accept(n, value)）:
  if n < HighestPromised:
      reply Accepted(accepted=false)       # 已承诺更高，拒绝
  else:
      HighestPromised  = n
      HighestAccepted = n
      AcceptedValue   = value
      notify Learners(Accepted(value))
      reply Accepted(accepted=true)

Proposer（累计 Accepted=true）:
  accepted++
  if accepted >= quorum: ChosenValue = value, Chosen = true

Learner（累计 Accepted）:
  counts[value]++
  if counts[value] >= quorum: Value = value, Chosen = true
```

## 最小可识别特征（少了就不算 Paxos）

1. **两阶段 Prepare/Accept**：先 Prepare 拿多数派"承诺"，再 Accept 提交值。任何省掉 Phase 1 直接广播值的协议不是 Paxos。
2. **提案编号单调递增**：编号是 Paxos 推进秩序的核心；不同 Proposer 用不相交的编号空间（本包简化为单一 Proposer 的自增 uint64）。
3. **多数派 quorum**：Promise 和 Accepted 都需多数派，多数派两两相交保证至多一个值被选中（安全性基石）。
4. **Acceptor 承诺后不再接受更低编号**：`n < HighestPromised` 一律拒绝。这是"承诺"的语义，保证旧编号 Proposer 无法覆盖新进展。
5. **Proposer 取已接受最大值**：Phase 2 的值必须取 Promise 里最高编号的已接受值（若有），否则可能覆盖已 chosen 的值，破坏安全性。

## 判定红线

- Acceptor 收到更低编号 Prepare/Accept **仍接受** → 违反承诺语义，不是 Paxos。
- Proposer 在 Promise 已带已接受值时 **仍用自己的值** → 可能覆盖已 chosen 值，违反安全性。
- 只看 1 个 Acceptor 就认为 chosen → 没有 quorum，无法保证唯一性。
- 用 "first-write-wins" 或时间戳而非编号 → 不是 Paxos 的 ballot 模型。
- 把 Phase 1 和 Phase 2 合并为一步（"我提议你直接接受"） → 退化成两将军/简单多数写，失去 Paxos 的冲突调和能力。

## 与 Raft 的区别（本仓库两算法对照）

| 维度 | Multi-Paxos（本包） | Raft（internal/raft） |
|------|--------------------|-----------------------|
| Leader 角色 | 弱协调者（Proposer），多 Proposer 可并存竞争 | 强 Leader，写必经 Leader |
| 顺序来源 | 每个槽位独立两阶段，编号最大者赢 | 显式 commitIndex + 日志 Index 前缀校验 |
| 安全性不变量 | 编号承诺 + 取已接受最大值 | 任期单调 + (prevIndex, prevTerm) 前缀匹配 + Leader-Commit 限制 |
| 心跳/续约 | 稳定 Leader 可复用 Phase 1（Multi-Paxos 优化），否则每条命令都两阶段 | AppendEntries 空条目当心跳 |
| 失败恢复 | 新 Leader 用更高编号 Prepare 重新建立权威 | Follower 选举超时后升 Candidate |
| 易理解性 | 抽象、状态机式、多 proposer 竞争复杂 | 论文核心目标，状态少、显式 |

## 本包简化（为教学清晰，非生产）

- 传输层为内存队列（显式 Drain 推进），非真实 RPC goroutine。
- 只演示单个 Paxos 实例（单槽位/单值）；Multi-Paxos 的"Leader 复用 Phase 1 跨多个槽位"优化未实现。
- Proposer 被 Acceptor 拒绝时不自动重试更高编号（demo 用保证够大的编号规避活锁）。
- 持久化 / 成员变更 / Learner 拉取（catch-up） / 快照均未实现。
- 提案编号用单一自增 uint64，未实现多 Proposer 的不相交编号空间（如 `round * N + nodeID`）。
