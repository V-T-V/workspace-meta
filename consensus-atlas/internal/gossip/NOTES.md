# Gossip · 设计笔记

## 论文

- **Epidemic Algorithms for Replicated Database Maintenance**
- 作者：Alan Demers, Dan Greene, Carl Hauser, Wes Irish, John Larson,
  Scott Shenker, Howard Sturgis, Douglas Swinehart, Doug Terry
- ACM PODC 1987
- 论文：https://dl.acm.org/doi/10.1145/41840.41841

> 最早提出把"流行病传播"模型用于分布式数据库副本维护：每个节点周期性挑一个
> 随机邻居交换状态，一条更新在 O(log N) 轮内高概率扩散到全网。

## 核心循环

### Push-Pull 一轮（本包实现）

```
发起方 X（每个 Tick）:
  peer = pickNeighbor()                      # 论文：随机挑一个；本包：round-robin
  send GossipRequest(X.state) → peer          # Push：把自己的全量状态送过去

接收方 Y（收到 GossipRequest）:
  merge(Y.state, X.state)                     # 用"取更大值"规则吸收 X 的状态
  reply GossipResponse(Y.state) → X           # Pull：回送自己合并后的全量状态

发起方 X（收到 GossipResponse）:
  merge(X.state, Y.state)                     # 同样取更大值，吸收 Y 的状态

# 一次 Request/Response 往返完成 X、Y 双向同步。
# N 个节点各自周期性重复此循环，信息像传染病一样在全网扩散。

合并规则（保证收敛性）:
  for k, v in other:
      if k not in self or v > self[k]:        # 字符串序取更大（模拟版本号/时间戳）
          self[k] = v
  # 合并可交换、可结合 ⇒ 最终所有节点对同一 key 取到相同最大值 ⇒ 收敛。
```

## 最小可识别特征（少了就不算 Gossip）

1. **周期性挑随机邻居**：每个节点每个周期只联系 O(1) 个邻居，而非全量广播。
2. **Push-Pull 双向交换**：信息既往外推也往里拉（纯 Push 在"对方已感染"时浪费
   带宽，纯 Pull 在"自己已是最新的"时浪费；Push-Pull 收敛最快）。
3. **合并规则有界且确定**：用一个全序比较（版本号、时间戳、本包的字符串序）决定
   冲突值，使合并操作可交换、可结合。
4. **最终一致而非强一致**：没有任何 Leader / 多数派 / 两阶段提交；只承诺
   "停止更新后，经过足够轮次，所有节点状态一致"。

## 判定红线

- **不保证收敛轮数**：理论上 O(log N)，但具体几轮取决于拓扑与选邻居运气；
  不能假设"第 K 轮一定收敛"。本包 demo 也只设上限兜底，不承诺精确轮数。
- **不抗拜占庭故障**：合并规则假设对方传来的值是诚实的；恶意节点可以灌入
  任意"最大值"污染全网（参见 internal/byzantine 讨论的防拜占庭机制）。
- **重复传输 / 浪费带宽**：全量状态每次都发，已收敛后仍持续交换；生产实现需
  Merkle 摘要、版本向量或 TTL 来削减冗余流量。
- **不提供读一致性**：刚写入的值可能在几轮内对部分节点不可见——不能用作账本。

## 与共识算法对比（本仓库 Raft/Paxos vs Gossip）

| 维度 | 共识（Raft / Paxos） | Gossip（本包） |
|------|----------------------|----------------|
| 一致性强度 | 强一致（linearizability） | 最终一致（eventual consistency） |
| 写入路径 | 经 Leader / 多数派 prepare-accept | 任意节点本地写，靠扩散传播 |
| 每节点每周期联系数 | O(N)（Leader 广播全集群） | O(1)（只挑一个邻居） |
| 故障容忍 | 少数派故障可工作；拜占庭需 PBFT | 节点故障只是"暂时不传染"，恢复后会追上 |
| 收敛保证 | 任一时刻已提交数据不丢 | 不丢，但读到最新值需等几轮 |
| 适用场景 | 配置/账本/状态机复制 | 成员表、路由表、缓存失效、元数据同步 |

一句话：**共识追求"对"，Gossip 追求"够用且便宜"**。

## 本包简化（为教学清晰，非生产）

- **选邻居用 round-robin 而非随机**：`peers[tickCount % len(peers)]`。
  论文核心是"随机抽样"，但随机会引入 `math/rand` 让 demo 轨迹不确定；教学库
  优先保证确定性可复现，故用确定性轮询替代。语义上"周期性挑一个邻居"的特征不变。
- **传全量状态 map 而非 Merkle 摘要**：每次消息体积 O(状态大小)。生产实现
  （如 Cassandra、Riak）用 Merkle 树比较差异，只传不一致的 key。
- **无版本向量 / 无逻辑时钟**：冲突直接按字符串序取更大值，模拟"版本号比较"。
- **无故障/恢复/成员变更模型**：节点永不宕机；集群成员固定。
- **传输层为内存队列**（复用 core.MemTransport 的显式 Drain 推进），非真实 RPC goroutine。
