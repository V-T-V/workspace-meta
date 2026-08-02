# Viewstamped Replication · 设计笔记

## 论文

- **Viewstamped Replication Revisited**（推荐重写版）
- 作者：Barbara Liskov, James Cowling
- 2012, MIT PMG. https://pmg.csail.mit.edu/papers/vr-revisited.pdf
- 原始版：Brian Oki & Barbara Liskov, PODC 1988

## 核心循环

### 正常操作（Normal Operation）

```
Client → Primary:  Request(op, client, reqNum)
Primary:
  OpNumber++
  log.append(view, opNumber, op)
  for each Backup b:
      send Prepare(view, opNumber, commitNum, request) to b
  pending[opNumber] = {ok: 1}  # 自己算一票

Backup (收到 Prepare):
  if view 匹配:
      log.append(view, opNumber, op)
      CommitNum = max(CommitNum, prepare.commitNum)
      reply PrepareOK(view, opNumber) to Primary

Primary (收到 PrepareOK):
  pending[opNumber].ok++
  if ok >= quorum and CommitNum < opNumber:
      CommitNum = opNumber
      execute op
      reply Reply(view, opNumber, result) to Client
      (顺便广播新 commitNum 推进 Backup 执行)
```

### 视图变更（View Change，Primary 失联）

```
Backup (electionTicks >= timeout 且没收到 Primary 消息):
  State = view-change
  View++
  viewChangeVotes = {self}
  broadcast StartViewChange(view) to all

Backup (收到 StartViewChange):
  if view 更新: 加入阵营，viewChangeVotes.add(from)
  if len(viewChangeVotes) >= quorum:
      send DoViewChange(view, log, lastNormalView) to 新 Primary 候选

新 Primary 候选 (收到 quorum DoViewChange):
  选最新日志作为基线
  become Primary, State = normal
  broadcast StartView(view, log, opNumber, commitNum) to all

Backup (收到 StartView):
  adopt 新 view + 日志基线
  State = normal, 认新 Primary
```

## 最小可识别特征（少了就不算 VR）

1. **View 号**：类似 Raft term，单调递增；Primary 由 view 决定。
2. **三段式正常操作**：Prepare（Primary→Backup）→ PrepareOK（Backup→Primary）→ Reply。
3. **opNumber 全局有序**：Primary 给请求分配递增 opNumber，决定执行顺序。
4. **quorum 多数派**：PrepareOK 需 ⌊n/2⌋+1 票才执行（含 Primary 自己）。
5. **显式 view change 协议**：StartViewChange / DoViewChange / StartView 三消息换主。

## 判定红线

- 客户端请求**直接发给 Backup 就执行** → 违反强 Primary 模型，不是 VR。
- PrepareOK **不凑 quorum 就执行** → 可能执行未复制操作，违反一致性。
- Primary 失联后 **没有 view change 协议** → 无法自动恢复，不是完整 VR。
- view **不递增**就换 Primary → 多个 Primary 同 view 并存（脑裂），违反 safety。

## 对比表（VR vs Raft vs Paxos —— 三者数学等价，哲学不同）

| 维度 | VR（本包） | Raft（internal/raft） | Paxos（internal/paxos） |
|------|------------|----------------------|------------------------|
| 出发点 | 复制状态机（1988，最早） | 可理解性（2014） | 抽象共识（1998） |
| 任期 | view | term | proposal/instance number |
| 主角色 | Primary | Leader | Proposer/Distinguished Proposer |
| 排序 | opNumber | log index | 槽位（每槽独立 Paxos） |
| 心跳 | Prepare/Commit 本身（无独立心跳） | AppendEntries 空条目心跳 | KeepAlive |
| 换主 | StartViewChange/DoViewChange/StartView | RequestVote 选举 | 视图变更（viewstamped 同源） |
| quorum | ⌊n/2⌋+1 | ⌊n/2⌋+1 | ⌊n/2⌋+1 |

> 三者**数学等价**（都是 majority quorum 共识，正确性证明可互推），区别在术语、
> 工程取向与可读性。VR 是其中最早的（1988），Raft 是最易教的（2014）。

## 本包简化（为教学清晰，非生产）

- 传输层为内存队列（显式 Drain 推进），非真实 RPC goroutine。
- 只实现"正常操作 + 视图变更"两阶段；**恢复（state transfer）阶段省略**——
  落后副本如何从新 Primary 拉取缺失日志未实现（重写版 §4.3）。
- view 的 Primary 选取简化：原版用 `primary = replicas[view % n]`，本包用"凑齐
  quorum 票者自荐当候选"，演示换主动态即可。
- 选举超时用整数 tick + 错开固定值（非真随机），保证 demo 轨迹确定（同 raft 包取舍）。
- 客户端去重（reqNum 表）省略；日志用 core.Log 复用，Term 字段存 view。
- 视图变更的日志合并简化为"取条目更多者"，未严格按 lastNormalView + opNumber 比较。
