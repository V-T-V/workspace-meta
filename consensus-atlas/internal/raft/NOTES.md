# Raft · 设计笔记

## 论文

- **In Search of an Understandable Consensus Algorithm**
- 作者：Diego Ongaro, John Ousterhout
- USENIX ATC 2014
- 论文：https://raft.github.io/raft.pdf · 主页：https://raft.github.io/ · 动画：https://raftscope.github.io/

## 核心循环

### Leader 选举（随机化超时）

```
Follower:
  electionTicks++
  if electionTicks >= ElectionTimeout:     # 隔离的 Follower 觉醒
      become Candidate
      CurrentTerm++, VotedFor = self
      send RequestVote(term, lastLogIndex, lastLogTerm) to peers

Candidate (收到 RequestVoteResponse):
  if VoteGranted: votes++
  if votes >= quorum: become Leader

Follower (收到 RequestVote):
  if msg.Term < CurrentTerm:               # 旧任期，拒绝
      reply VoteGranted=false
  elif VotedFor == nil or VotedFor == from:
      if candidate log up-to-date:
          VotedFor = from, reply granted
      else: reply denied
  else: reply denied

(any node 收到更高 Term):
  CurrentTerm = msg.Term, State = Follower, VotedFor = nil   # 强制降级
```

### 日志复制

```
Leader (Propose 或心跳):
  for each follower f:
      send AppendEntries(
          prevIndex = nextIndex[f]-1,
          prevTerm  = log[nextIndex[f]-1].Term,
          entries   = log[nextIndex[f] .. lastIndex],
          commit    = CommitIndex)

Follower (收到 AppendEntries):
  if prevIndex>0 and log[prevIndex].Term != prevTerm:   # 日志不匹配
      reply Success=false
  else:
      append entries (truncate on conflict)
      if LeaderCommit > CommitIndex:
          CommitIndex = min(LeaderCommit, lastIndex)
      reply Success=true, MatchIndex=lastIndex

Leader (收到 Success=false):
  nextIndex[f]--, resend                           # 递减回退
Leader (收到 Success=true):
  matchIndex[f] = MatchIndex
  if 某个 N 被多数 matchIndex 覆盖且 term==当前:
      CommitIndex = N                              # 提交
```

## 最小可识别特征（少了就不算 Raft）

1. **强 Leader**：所有客户端写必须经 Leader；Follower 收到写请求要转发或拒绝。
2. **任期（term）单调递增**：所有 RPC 带 term；收到更高 term 强制降级 Follower。
3. **随机化选举超时**：用随机超时打散候选人，避免多个节点同时觉醒形成活锁（论文 §5.4 的关键设计）。
   > **本教学库的工程取舍**：真随机化会让 demo 执行轨迹不确定。因此本包用**各节点错开的固定 `ElectionTimeout`（整数 tick）替代真随机化**——例如 demo 里 5 个节点取 5/7/9/11/13 tick，最低者必先觉醒。生产 Raft 用 `time.Duration` + `rand` 在一个区间内随机化（如 150–300ms）。这只影响"打散候选人"的确定性 vs 随机性，不影响算法本身的正确性，是确定性的工程取舍（见"本包简化"第 3 条）。
4. **日志前缀校验（prevIndex, prevTerm）**：AppendEntries 用前缀匹配保证日志一致性。
5. **Leader-Commit 限制**：Leader 只提交本任期的条目（间接提交旧条目），保证安全性（论文 §5.4.2）。

## 判定红线

- 有 Leader 但**写请求 Follower 自己能直接接受** → 不是 Raft（违反强 Leader）。
- 选举超时**所有节点相同**且无随机化 → 容易活锁，不是标准 Raft。
  > 本包**不违反**此红线：各节点取**错开的不同** `ElectionTimeout`（如 demo 的 5/7/9/11/13），打散效果与随机化等价（必有唯一最低者先觉醒）。只是用"固定错开"替代"运行时随机"，换取 demo 轨迹确定。生产实现请用真随机。
- 日志复制**不校验前缀**直接覆盖 → 可能丢失已提交数据，违反安全性。
- Leader 提交**跨任期**条目 → 违反 §5.4.2 leader-commit 安全约束。

## 与 Multi-Paxos 的区别（本仓库两算法对照）

| 维度 | Raft（本包） | Multi-Paxos（internal/paxos） |
|------|--------------|------------------------------|
| Leader 角色 | 强 Leader，写必经 | 弱协调者（Proposer），acceptor 可接受多 proposer |
| 顺序来源 | 显式 commitIndex + 日志 Index | 隐式：每个槽位独立 Paxos，编号最大者赢 |
| 心跳 | AppendEntries 空条目当心跳，Leader 在 `Tick` 里周期性广播（每 `heartbeatInterval` 个 tick 一次），`Propose` 时立即触发额外广播 | 通常需单独的 KeepAlive |
| 易理解性 | 论文核心目标，状态少 | 状态机更抽象，多 proposer 竞争复杂 |

## 本包简化（为教学清晰，非生产）

- 传输层为内存队列（显式 Drain 推进），非真实 RPC goroutine。
- 成员变更 / 日志压缩（snapshot）/ 持久化 state 未实现（论文 §6/7）。
- 选举超时用整数 tick 而非真实时间，由 demo 主循环驱动。
- **心跳机制已与实现一致**：Leader 在 `Tick` 里周期性广播空 `AppendEntries` 作心跳（间隔 = `ElectionTimeout/2`，向下取整至少 1，见 `heartbeatInterval`），并在 `Propose` 时立即广播一次带新条目的 `AppendEntries`。心跳重置 Follower 的选举时钟，防其觉醒挑战 Leader。
