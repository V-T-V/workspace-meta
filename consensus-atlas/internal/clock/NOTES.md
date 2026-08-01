# Logical Clocks · 设计笔记

## 论文

- **Time, Clocks, and the Ordering of Events in a Distributed System**
- 作者：Leslie Lamport
- Communications of the ACM, 1978
- 论文：https://lamport.org/pubs/time-clocks.pdf
- 贡献：定义了 happens-before 关系（→）与 Lamport 逻辑时钟规则，奠定了
  "分布式系统不需要全局物理时间"这一整套思维方式。后续的 Paxos、Raft 等
  共识算法中的 term/epoch 本质上都是逻辑时钟的特化。

- **Virtual Time and Global States of Distributed Systems**
- 作者：Friedemann Mattern
- Proceedings of the International Workshop on Parallel and Distributed Algorithms, 1989
- 贡献：系统化了 Vector Clock，给出了"两个事件 a/b 之间 a→b / b→a / 并发"
  的精确向量判定规则（本包 `Compare` 实现的就是它）。

## 核心循环

### Lamport Clock 规则（标量）

```
本地事件 / 发送事件：
    C = C + 1
    (发送时把 C 打到消息上：msg.C = C)

接收事件：
    C = max(C, msg.C) + 1
```

性质：若 a → b（因果先于），则 C(a) < C(b)。
**注意这是充分不必要**：C(a) < C(b) 不一定 a → b，二者可能并发。
因此 Lamport Clock 能用来给事件"打全序标签"（如 Raft 的 term），但
**不能**用来判断两个事件是否并发。

### Vector Clock 规则（N 维，N = 节点数）

```
节点 i 的本地事件：
    V[i] = V[i] + 1
    (其余分量不变)

节点 i 接收来自节点 j 的消息：
    for k in allNodes:
        V[k] = max(V[k], msg.V[k])
    V[i] = V[i] + 1            # "接收"本身也是事件
    (发送方 j 在发送时已经 V[j]++ 并把 V 打到消息上)
```

判定（对任意两个事件 a, b，比较其向量快照 Va, Vb）：

```
a ≤ b  ⟺  ∀i: Va[i] ≤ Vb[i]
a < b  ⟺  a ≤ b 且 ∃i: Va[i] < Vb[i]
a = b  ⟺  ∀i: Va[i] = Vb[i]

若 a ≤ b:              a → b
若 b ≤ a:              b → a
若既非 a ≤ b 也非 b ≤ a: 并发（无因果关系）
```

## 最小可识别特征（少了就不算）

### Lamport Clock

1. **单整数标量**：整个时钟状态就一个 uint64。
2. **max+1 规则**：收消息 `C = max(C, msg.C) + 1`，本地 `C = C + 1`。
3. **只保证弱偏序**：能保证 a→b ⟹ C(a)<C(b)，但**不保证反向**。

### Vector Clock

1. **N 维向量**：维度 = 节点数，每个分量对应一个节点的本地计数。
2. **分量 max + owner +1**：收消息对所有分量取 max，owner 分量再 +1。
3. **能判并发**：这是 Vector 相对 Lamport 的**唯一**质变能力——能区分
   "a 先于 b" / "b 先于 a" / "并发"。

## 判定红线

- **Lamport Clock 拿来判并发** → 错。C(a) < C(b) 可能只是并发事件的巧合，
  Lamport 给不出并发判定。
- **Vector Clock 维度 ≠ 节点数** → 错。少一个维度就丢掉对应节点的因果信息，
  判定会出错。维度必须 = 当前成员数。
- **Vector Clock 比较时维度不对齐**（一个含 n3 一个不含）→ 错。本包 `Compare`
  把缺失分量按 0 处理，对静态成员是安全的；动态成员变更不在本包范围。
- **Vector Clock 当全序用** → 误用。向量只能给偏序（+ 并发），全序需要再
  叠加节点 ID tie-breaker（如 ` Raft 的 (term, index)`）。

## 两者对比

| 维度 | Lamport Clock（core 包） | Vector Clock（本包） |
|------|--------------------------|----------------------|
| 状态 | 单 uint64 | N 维 map[NodeID]uint64 |
| 空间 | O(1) | O(N) |
| 更新规则 | max(C, msg.C)+1 | 每分量 max，owner 再 +1 |
| 因果序 | 弱偏序（a→b ⟹ C(a)<C(b)） | 强偏序（精确刻画 a→b / b→a / 并发） |
| 能否判并发 | **否** | **是** |
| 典型用途 | 消息打标、任期递增、全序排序键 | 因果追踪、分布式快照、CRDT 因果元数据 |

## 与因果排序 / 分布式快照的关系

- **因果排序（Causal Broadcast）**：发消息时带上 Vector Clock，接收方用
  `Compare` 判断"这条消息依赖的所有前置消息我都收到了吗"，没收到就缓 deliver。
  Vector Clock 是其状态载体。
- **分布式快照（Chandy-Lamport / Mattern）**：用 Vector Clock 标记"记录时刻"，
  判断不同进程记录的本地状态是否属于同一条一致割集（consistent cut）——
  Mattern 1989 论文正是把 Vector Clock 与全局状态检测绑在一起讨论的。
- **CRDT**：无冲突复制数据类型用 Vector Clock（或其变体 version-vector）做
  合并判定——两个版本若 Concurrent 则需应用层合并规则（如 LWW / merge）。

## 本包简化（为教学清晰，非生产）

- 复用 core.LamportClock 做标量侧，本包只补 Vector Clock + 因果比较 + demo。
- 成员变更（动态加/删节点）未实现：向量维度在构造时固定。
- 不做因果广播协议本身，只暴露 `Compare` 让上层自行构建 deliver 判定。
- demo 纯函数式：无 goroutine、无 time、无 rand，轨迹完全确定。
