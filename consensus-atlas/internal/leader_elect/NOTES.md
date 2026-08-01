# Leader 选举（Bully + Ring）· 设计笔记

## 论文

- **Elections in a Distributed Computing System**
- 作者：Hector Garcia-Molina
- IEEE Transactions on Computers, vol. C-31, no. 1, Jan 1982

## 核心循环

### Bully（比 ID 大小）

```
节点 p 发现 Leader 失联，p.StartElection():
  for each peer q where q.ID > p.ID:
      send Election(from=p) to q
  if 没有更高 ID 节点:
      p 当选, broadcast Coordinator(leader=p)
  else:
      p 进入"等待 Answer"状态

节点 q 收到 Election(from=p):
  if q.ID > p.ID:
      send Answer(from=q) to p          # 告诉 p "我比你大，退出"
      q.StartElection()                 # 自己也去竞争

节点 p 收到 Answer(from=q 且 q.ID > p.ID):
  清除"等待 Answer" → p 退出竞争，等 Coordinator

p 的等待兜底（drain 完仍无 Answer）:
  → 所有更高节点都离线 → p 当选, broadcast Coordinator(leader=p)

任意节点收到 Coordinator(leader=L):
  knowsLeader = L
```

### Ring（沿环收集 ID）

```
环：n1 -> n2 -> n3 -> ... -> nk -> n1   (单向)

发起者 s.StartElection():
  send Election(IDs=[s.ID]) to s.Next

节点 x 收到 Election(IDs):
  if x.ID ∈ IDs:                        # 消息绕一圈回来了
      L = max(IDs)
      knowsLeader = L
      for each id ∈ IDs:
          send Coordinator(leader=L) to ring[id]
  else:
      send Election(IDs ∪ {x.ID}) to x.Next
```

## 最小可识别特征

### Bully
1. **比 ID 数值大小**：ID 最大（且在线）者当选。
2. **显式 Coordinator 公告**：当选者必须向全网广播 Coordinator，否则其它节点无从得知结果。

### Ring
1. **逻辑环拓扑**：每个节点只知道后继 Next，不存在"更高 ID"概念。
2. **消息单向绕环**：Election 沿环逐跳收集所有 ID，绕回发起者后取最大。

## 判定红线

- Bully 选完**不广播 Coordinator** → 不是 Bully（其它节点不会知道结果）。
- Bully 节点收到更高 ID 的 Election **不回 Answer** → 发起者会误判自己当选，导致双 Leader。
- Ring 节点**不把自身 ID 加入集合就转发** → 最终 max 不完整，可能选出非最大者。
- Ring 消息**绕一圈回到发起者却不判定结束** → 死循环。
- 用**随机超时**当触发条件 → 那是 Raft，不是 Bully/Ring（二者都是显式消息驱动）。

## 与 Raft 随机超时选举对比

| 维度 | Bully（本包） | Ring（本包） | Raft（internal/raft） |
|------|--------------|--------------|----------------------|
| 触发方式 | 显式消息（发现失联即发） | 显式消息 | 随机化选举超时隐式触发 |
| 决定胜负 | ID 数值大小 | 环上收集的最大 ID | 任期 + 日志新旧 + 多数票 |
| 消息复杂度 | O(n²)（每个发起者向更高者发） | O(n²)（消息绕环 + Coordinator 广播） | O(n)（候选人向 peers 请求投票） |
| 是否需多数派 | 否（ID 比较） | 否（环遍历） | 是（quorum 投票） |
| 成员假设 | 已知全部成员 ID | 已知后继 + 全环成员 | 已知 peers 配置 |
| 容忍故障 | 仅 crash（离线节点不响应） | 仅 crash | crash，且 Leader 失联可重选 |

## 本包简化（为教学清晰，非生产）

- **无真实超时**：Bully 的"等待 Answer 超时"用 demo 主循环的显式 `FinishElection()`
  兜底模拟（drain 完仍无 Answer 即当选），不依赖 wall-clock 定时器，保证确定性。
- **无故障检测**：节点是否在线由 `Online` 字段显式设置，未实现心跳/lease 检测。
- **Ring 假设环完整**：不处理环上 Next 节点离线的修复（真实系统需维护可用环）。
- **Coordinator 广播范围**：Bully 用已知 Peers，Ring 用消息中收集的 IDs 集合，
  二者都不假设有独立的"组成员服务"。
