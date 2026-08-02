# Chandy-Lamport Snapshot · 设计笔记

## 论文

- **Distributed Snapshots: Determining Global States of Distributed Systems**
- 作者：K. Mani Chandy, Leslie Lamport
- ACM Transactions on Computer Systems, Vol. 3 No. 1, Feb. 1985
- 论文：https://lamport.org/pubs/chandy-lamport.pdf

## 核心循环（marker 传播）

```
发起者 P（任意进程）：
  record local state
  for each outgoing channel c:
      send marker on c            # marker 排在已有消息之后（FIFO）

任意进程收到 marker（首次，任何入通道）：
  if not yet recorded:
      record local state
      for each outgoing channel c:
          send marker on c
  mark "marker seen" on this incoming channel   # 之后到达此通道的应用消息 → 通道状态

任意进程收到 marker（已记录，但此通道未见）：
  mark "marker seen" on this channel            # 通道状态 = 空（marker 前在途消息已先到）

任意进程收到应用消息（已开始记录）：
  if 此入通道已 "marker seen":
      append to this channel's recorded state   # 在途消息，记入通道状态
  else:
      process normally (update local state)     # 快照前的消息，正常消费

完成：所有进程都已 record local state（marker 已传遍）
汇总：Σ 本地状态 + Σ 通道状态 = 一致全局快照
```

## FIFO 通道前提（关键）

算法正确性**完全依赖通道 FIFO**：

- marker 必须严格在其前驱应用消息之后到达接收方。
- 因此"收到 marker"的时刻意味着"此通道上 marker 之前发出的消息都已被我收到"。
- 若通道非 FIFO（消息可乱序/绕过 marker），切割点不确定，快照可能不一致（漏记/重记）。

> 这是本算法的**硬前提**：要么用 TCP 这种 FIFO 通道，要么在上层加序号重排保证 FIFO。

## 为什么一致（CUT 性质）

把每条消息按"是否在快照记录前被消费"分两类，marker 定义切割点（CUT）：

```
时间轴 ──────────────────────────────────────►
            │ CUT（由 marker 定义）
   本地状态  │  通道状态
   (recorded)│  (in-transit)
```

- 每条消息要么在 CUT 之前（被记录进本地状态，已消费），
  要么在 CUT 之后（作为在途消息记入通道状态）。
- 两者**不重不漏**——这就是一致性（consistent cut）。
- 结果快照可能从未真实"同时存在"过，但它是一个**可达的**全局状态
  （满足系统不变量），可用于检测稳定性质（如死锁、终止）。

## 最小可识别特征（少了就不算 Chandy-Lamport）

1. **marker 消息**：用特殊消息标记切割点（无 marker 就不是本算法）。
2. **首次收到 marker 即记录本地 + 广播 marker**：这是单次触发、自传播的核心规则。
3. **通道状态 = marker 之后到达的应用消息**：依赖 FIFO，marker 前的消息已先到。
4. **FIFO 通道假设**：算法正确性的硬前提。
5. **非阻塞**：系统继续运行，快照与正常消息流交织进行。

## 判定红线

- 通道**非 FIFO** 却用 marker 记通道状态 → 切割点不确定，快照可能不一致。
- 收到 marker **不广播** marker → 算法无法自传播，快照不完整。
- 发起者**先发 marker 后记录本地状态** → 本地状态可能已变，违反"记录发起时刻"。
- 把 marker 之前的消息也算进通道状态 → 重记，违反 CUT 一致性。

## 对比表（本仓库"状态观测"谱系）

| 维度 | Chandy-Lamport（本包） | Raft/Paxos | CRDT |
|------|------------------------|------------|------|
| 目的 | 记录一致全局状态 | 达成共识（单值/日志） | 最终一致收敛 |
| 是否阻塞 | 非阻塞（系统继续运行） | 写需多数派（半阻塞） | 写永不阻塞 |
| 一致性强度 | 一致 cut（可达状态） | 线性一致（强） | 最终一致（弱） |
| 通道假设 | 可靠 FIFO | 可靠（重传即可，非 FIFO 必须） | 任意（消息可丢/重/乱） |
| 容错 | 进程可 crash（不要求拜占庭） | 多数派存活 | 任意分区可恢复 |

## 本包简化（为教学清晰，非生产）

- 通道是内存切片（显式 Step 推进），非真实网络 goroutine。
- 本地状态用单个可变字符串值（`MsgVal`）模拟应用演进（"最后一条消息覆盖"）。
- 只演示单向环形拓扑（3 进程 3 通道）；算法对任意拓扑成立。
- 不实现 stable property 检测（死锁/终止检测的应用层）——只产出快照数据。
- 不处理进程 crash 恢复（论文假设 fail-stop；本包假设进程存活至快照完成）。
