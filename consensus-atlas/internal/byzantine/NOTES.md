# PBFT · 设计笔记

## 论文

- **Practical Byzantine Fault Tolerance**
- 作者：Miguel Castro, Barbara Liskov
- OSDI 1999
- 论文：https://pmg.csail.mit.edu/papers/osdi99.pdf

## 核心循环（三阶段）

```
[Client] --request--> [Primary p]

Primary p:
  seq++ ; proposal[seq] = (view, seq, req)
  recordPrepare(seq, self)               # primary 自投 prepare
  broadcast PrePrepare(view, seq, req)   # ---- 阶段 1 ----

Replica (收到 PrePrepare):
  proposal[seq] = pp
  recordPrepare(seq, self)
  broadcast Prepare(seq, view, node=self, sig)

(any Replica 收到 Prepare):
  recordPrepare(seq, from)
  if len(prepared[seq]) >= 2f+1 && !isPrepared[seq]:   # 达门槛
      isPrepared[seq] = true                           # ---- 进入 prepared ----
      recordCommit(seq, self)
      broadcast Commit(seq, view, node=self, sig)      # ---- 阶段 2 ----

(any Replica 收到 Commit):
  recordCommit(seq, from)
  if len(committed[seq]) >= 2f+1 && !isCommitted[seq]:  # 达门槛
      isCommitted[seq] = true                           # ---- 进入 committed ----
      CommittedSeqs += seq                              # ---- 阶段 3：执行 ----
      reply to Client
```

## 集群规模约束（n 必须 ≡ 1 mod 3）

PBFT 的安全性建立在 quorum 交集代数上：任意两个 2f+1 的 quorum 必在诚实节点
上有非空交集（交集至少 n − 2·(2f+1) = n − 4f − 2 ≥ 2f−1 ≥ 1 当 n=3f+1）。
这要求集群规模严格满足：

> **n ≡ 1 (mod 3)，即 n = 3f + 1**（f 为可容忍的拜占庭节点数）。

合法配置：n=4 (f=1) / n=7 (f=2) / n=10 (f=3) / n=13 (f=4) ...
非法配置：n=1 / 2 / 3 / 5 / 6 / 8 / 9 ...

`ValidateCluster(n)` 函数强制校验此约束：合法返回 `(f, nil)`，非法返回
`(0, error)`。**生产部署必须先过此校验，否则无拜占庭容错能力。**

### 关于非法 n 的兜底行为

本包的 `quorum()` 用整数公式 `(2n+2)/3` 对**任意** n 都给出一个数值（这是
为了让 demo 在边界值上不 panic，保持运行灵活）。但这**不意味着**该配置具
备拜占庭容错：

- **n=3** 时公式给出 `quorum=2`，看似"只需 2 票"。但 n=3 对应 f=（3−1)/3
  取整为 0，即**容不下任何拜占庭节点**。2 票里完全可能混入 1 票拜占庭 +
  1 票诚实，两个互相矛盾的 2 票 quorum 可同时成立 → 可被拜占庭节点分叉，
  **无安全性可言**。
- **n=5/6/8/9 等**非 3f+1 形同理：公式给兜底 quorum 值，但交集代数不成立，
  两个 quorum 可能在诚实节点上无交集。

因此 n=3 等非法配置虽然在代码里能跑、公式也给出 quorum 数值，但**不构成
拜占庭容错**，只适合教学演示"happy path"，绝不能用于真实拜占庭场景。

## 最小可识别特征（少了就不算 PBFT）

1. **三阶段** pre-prepare / prepare / commit，缺一不可（少 prepare 无序保证，少 commit 无法证明"足够多人已 prepared"）。
2. **quorum = 2f+1**，且 n = 3f+1（容忍 f 个拜占庭节点；详见上文"集群规模约束"）。
3. **容忍 f < n/3 的拜占庭故障**（任意恶意行为，不只是 crash）。
4. **两轮投票的代数本质**：prepare 阶段保证"序被足够多人认可"；
   commit 阶段证明"足够多人已认可这个序"——后者让任何已 committed 的请求
   必然在诚实节点的 prepared 集合中（safety）。

## 判定红线

- **只有两阶段**（pre-prepare + prepare）→ 不是完整 PBFT。真实系统里
  两阶段只能保证"本地 prepared"，无法向新 view 证明该序已被认可，
  view-change 时可能丢失已执行请求（违反 safety）。
- **quorum 用 > n/2 多数派而非 2f+1** → 不耐拜占庭。f 个拜占庭节点可
  伪造"假多数"导致分叉。必须 2f+1 才能让任意两个 quorum 的诚实节点必有交集。
- **Primary 单方面决定 committed 而无第二阶段投票** → 退化为 Paxos/Raft
  的 crash-fault 模型，无法抵御 Primary 撒谎。
- **不签名/MAC** → 真实 PBFT 必须签名或共享密钥 MAC，否则拜占庭节点可
  伪造他人投票。本包用占位 Signature，教学简化。
- **集群规模不满足 n ≡ 1 (mod 3)**（如 n=3/5/6）→ quorum 交集代数不成立，
  两个 quorum 可能在诚实节点上无交集，拜占庭节点可制造分叉，**无拜占庭
  容错能力**。详见上文"集群规模约束"与 `ValidateCluster`。即便 `quorum()`
  公式对非法 n 也给出兜底值，那只保证 demo 不 panic，不保证安全。

## 与 Raft 对比

| 维度 | PBFT（本包） | Raft（internal/raft） |
|------|--------------|----------------------|
| 故障模型 | 拜占庭（任意恶意） | crash fault（崩溃/失联） |
| 容错上限 | f < n/3 | f < n/2 |
| quorum | 2f+1（> 2/3） | floor(n/2)+1（> 1/2） |
| 消息复杂度 | O(n²)（全网两两投票） | O(n)（Leader→Follower 复制） |
| Leader 角色 | Primary（按 view 轮换） | 强 Leader（按 term） |
| 阶段数 | 3（pre-prepare/prepare/commit） | 2 隐式（投票当选 + 复制提交） |
| 典型场景 | 联盟链/许可链 | 一般分布式存储（etcd 等） |

## 本包简化（为教学清晰，非生产）

- **无真签名/MAC**：`Vote.Signature` 是占位字符串，不校验内容。
  真实系统用 RSA/Ed25519 签名或基于共享密钥的 HMAC。
- **无 view-change**：Primary 故障后无法切换 view 恢复。完整 PBFT 的
  view-change 协议相当复杂（论文 §4.4），是 PBFT 工程实现最难的部分。
- **无 checkpoint**：日志无界增长，没有 stable checkpoint 机制压缩。
- **无客户端请求去重 / 跨视图重传**：诚实节点 happy path 足够展示三阶段。
