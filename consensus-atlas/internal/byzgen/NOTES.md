# Byzantine Generals · 设计笔记

## 论文

- **The Byzantine Generals Problem**
- 作者：Leslie Lamport, Robert Shostak, Marshall Pease
- ACM TOPLAS, July 1982
- 论文：https://lamport.org/pubs/byz.pdf

## 问题

一群将军包围一座城，必须就"进攻 (attack) / 撤退 (retreat)"达成一致。其中**有叛徒**
（Byzantine 故障，可任意作恶：发假消息、对不同人发不同值、装聋作哑）。忠诚将军要：

1. **Agreement（一致性）**：所有忠诚将军最终决定同一个值。
2. **Validity（有效性）**：若总司令忠诚，所有忠诚将军执行他的命令。

这是分布式容错的根本模型——节点不只崩溃，还可能作恶。容错强度远超 Raft/Paxos 的崩溃模型。

## 核心循环（OM 算法递归结构）

```
OM(m)  commander 发 order 给所有 lieutenant：
  for each lieutenant i:
      if commander 是叛徒:        # Byzantine：给每个人发不同值
          sent[i] = order + "-fake" + i
      else:
          sent[i] = order          # 忠诚：发同一个真值

  for each lieutenant i:
      decision[i] = OM(m-1)(充当 commander 的 i, 其他人, sent[i])

  return decision                  # 每个 lieutenant 的最终决定

OM(m-1)  对剩余 m-1 层递归，每层都重复"发→递归转发→majority"

OM(0):  # 递归基
  lieutenant 直接采用收到的值（无多数投票）

最终决定：
  每个 lieutenant 用 majority(所有收到的值) 把叛徒杂音过滤掉
```

## 3f+1 下界（口头消息模型）

定理：用**口头消息**（消息内容可被叛徒任意篡改/伪造、不可签名），容忍 f 个叛徒
**充要条件**是节点数 n ≥ 3f+1。

直觉：忠诚节点必须占**严格多数**（>2/3），才能让 majority 投票覆盖叛徒杂音——
因为叛徒可同时向两群忠诚节点发不同值，造成分裂；只有忠诚节点足够多（>叛徒的 2 倍），
majority 才稳定指向真值。

- f=1 → n≥4
- f=2 → n≥7
- f=3 → n≥10

> 用**签名消息**（叛徒无法伪造签名）可降到 n≥f+1 的总节点存活（即只要 1 个诚实节点），
> 但消息体积与验签开销显著增加。OM 用口头消息，故守 3f+1。

## 最小可识别特征（少了就不算 OM）

1. **递归结构**：OM(m) 调 OM(m-1)，深度 = f。没有递归就不是 OM（PBFT 用显式投票替代递归）。
2. **每层 commander 把值转发给其他 lieutenant**：lieutenant 充当新 commander 再传播。
3. **majority 决策**：每个 lieutenant 用 majority(收到的所有值) 过滤叛徒杂音。
4. **3f+1 下界**：容忍 f 叛徒必须 n ≥ 3f+1（口头消息模型的核心约束）。
5. **口头消息假设**：消息不可签名、可被任意篡改（叛徒可对不同人发不同值）。

## 判定红线

- 宣称容忍 f 叛徒但 **n < 3f+1**（口头消息）→ 违反下界，算法不可能正确。
- 递归深度 **m < f** → 容错能力不足，无法覆盖 f 个叛徒的扰动。
- 决策**不用 majority** 而用单值 → 无法过滤叛徒对人群发的分歧值。
- 用了**签名消息**仍守 3f+1 → 没用上签名的优势（签名可放宽到 n>f+1 存活）。

## 对比表（本仓库容错谱系）

| 维度 | OM（本包） | PBFT（byzantine） | Raft / Paxos |
|------|------------|-------------------|--------------|
| 故障模型 | 拜占庭（任意作恶） | 拜占庭 | 仅崩溃（crash-stop） |
| 容错下界 | n ≥ 3f+1 | n ≥ 3f+1 | n ≥ 2f+1 |
| 决策方式 | 递归 majority | 三阶段 quorum 投票 | 多数派日志/提案 |
| 消息复杂度 | O(n^(f+1)) 指数 | O(n^2) | O(n)（Raft）/ O(n)（Paxos） |
| 连续请求 | 单值一次性 | 支持（sequence 递增） | 支持（log 序号） |
| 用途 | 理论奠基 | 实用拜占庭系统 | 数据库/协调服务 |

## 与 PBFT 的关系（OM 的实用化）

PBFT (1999) 是 OM 的工程化继承：

- **OM 的 majority 思想保留**：PBFT 的 prepare 阶段本质上做的是 OM 的"间接互传 + majority"，
  即每个 replica 收集其他 replica 对同一提案的意见，凑齐 2f+1 票（quorum）即认为达成共识。
- **用显式投票替代递归**：OM 用 O(n^(f+1)) 条递归消息，PBFT 用 O(n^2) 条显式投票，
  把指数复杂度降到平方——这是"实用"的关键。
- **引入视图变更（view change）**：OM 不处理 primary 故障恢复；PBFT 加 StartViewChange/
  DoViewChange/StartView 三消息在 primary 失联时换主。
- **签名/认证**：PBFT 实际部署用 MAC 或数字签名，使叛徒无法伪造他人消息（接近签名消息模型）。

## 本包简化（为教学清晰，非生产）

- 纯函数式实现（无 transport / goroutine / 真实网络），用直接函数调用模拟同步递归口头消息。
- 叛徒行为用确定性的 `traitorOrder(base, idx)` 模拟（给每个接收者发不同值），非真实 Byzantine 攻击。
- 只实现单轮 OM（单值共识），不处理连续请求 / 视图变更 / 故障恢复（PBFT 的领域）。
- 只演示 f=1（n=4）；OM 对任意 f 正确，但消息数随 f 指数增长，demo 取最小有意义值。
- majority 平票时取字典序较小者（确定性，便于测试；真实系统应避免平票或用 tie-breaker）。
