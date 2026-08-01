# ZAB · 设计笔记

## 论文

- **Zab: High-performance broadcast for primary-backup systems**
- 作者：Flavio P. Junqueira, Benjamin Reed, Marco Serafini
- IEEE DSN 2011
- 论文：https://zookeeper.apache.org/doc/r3.4.13/zab.pdf

## 核心循环（broadcast 阶段）

```
Client → Leader:  request
Leader:
  zxid = (epoch << 32) | (++counter)        # 全局单调递增
  log.append(zxid, request)
  pending[zxid] = {acks: 1}                 # 自己算 1
  for each Follower f:
      send Proposal(zxid, request) to f

Follower (收到 Proposal):
  log.append(zxid, request)                 # FIFO：按 zxid 顺序
  reply Ack(zxid) to Leader

Leader (收到 Ack):
  pending[zxid].acks++
  if acks >= quorum and not committed:
      committed = true
      for each Follower f:
          send Commit(zxid) to f
      (Leader 本地也提交执行)

Follower (收到 Commit):
  按 zxid 顺序提交执行（跳过未连续的 zxid 直到补齐）
```

## zxid 设计（全局单调有序的核心）

```
  高 32 位                低 32 位
┌──────────────┬───────────────────┐
│    epoch     │     counter       │
└──────────────┴───────────────────┘
  64 位 ZXID
```

- **epoch（纪元）**：每次 leader 切换 +1。跨 epoch 的 zxid 高位变大，整体更大。
- **counter（计数）**：本 epoch 内 Leader 分配的递增事务号。
- 比较规则：epoch 高者大；epoch 相同比 counter。保证**全局单调递增**。
- 所有副本按 zxid 顺序提交 → 状态机一致（primary-backup 语义）。

## 三个阶段（phase）

```
1. discovery（发现）    ：发现集群 quorum + 最新已提交 zxid（选 leader 的依据）
2. synchronization（同步）：Leader 把事务历史同步给 Follower，各副本对齐
3. broadcast（广播）    ：Leader 处理新请求、广播 Proposal、收 Ack、发 Commit
                          ↑ 本包只实现这一阶段
```

## 最小可识别特征（少了就不算 ZAB）

1. **zxid 全局单调**：epoch<<32 | counter，所有副本按 zxid 顺序提交。
2. **强 Leader 顺序广播**：Leader 串行分配 zxid，Follower FIFO 接收。
3. **三消息广播**：Proposal → Ack → Commit（一轮正常路径）。
4. **quorum Ack 才 Commit**：多数派确认才提交（与 Paxos/Raft 同）。
5. **phase 化**：discovery / synchronization / broadcast 三阶段（本包简化只做 broadcast）。

## 判定红线

- **无 zxid** 或 zxid 不单调 → 无法保证全局有序，不是 ZAB。
- Leader **不按顺序** 广播 Proposal → Follower 日志乱序，违反 primary-backup。
- **不凑 quorum** 就 Commit → 可能提交未复制事务，违反一致性。
- 把 epoch 和 counter 混用（如只用单调计数）→ 跨 leader 切换无法区分新旧，违反 safety。

## 对比表（leader-based 顺序广播谱系）

| 维度 | ZAB（本包） | Raft（internal/raft） | Multi-Paxos（internal/paxos） |
|------|-------------|----------------------|-------------------------------|
| 排序标识 | zxid (epoch\|counter) | term + log index | 槽位 + proposal number |
| 强 leader | 是（primary-backup） | 是 | 弱（distinguished proposer） |
| 正常路径消息 | Proposal→Ack→Commit (1 轮) | AppendEntries→Response (1 轮) | Prepare→Accept→Accepted (可压缩) |
| 换主 | discovery+sync 阶段 | RequestVote 选举 | view change（同源） |
| 优化目标 | 主备原子广播性能 | 可理解性 | 抽象通用共识 |
| 典型系统 | ZooKeeper | etcd, Consul | Chubby, Spanner |

## 本包简化（为教学清晰，非生产）

- 只实现 **broadcast 阶段**；discovery（选 leader + 发现 zxid）与 synchronization
  （事务历史同步）两阶段省略——这两个阶段对应 Raft 的选举 + 日志修复。
- Leader 在 demo 中固定指定（不做 leader 选举）；epoch 为常量。
- Follower 假设 Proposal 按序到达（FIFO），直接追加日志，不做 zxid 乱序重排。
- Commit 顺序简化：达 quorum 即 commit，未严格实现"zxid 连续无空洞才推进"的
  顺序保证（demo 通过串行 Propose + FIFO 保证顺序）。
- 传输层为内存队列（显式 Drain 推进），非真实网络 goroutine。
- 不处理 Leader 故障切换（生产 ZAB 在 Leader 宕机时进入 discovery 重选 + sync）。
