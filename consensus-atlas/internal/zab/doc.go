// Package zab 实现 ZAB（ZooKeeper Atomic Broadcast）——ZooKeeper 的核心共识协议。
//
// ZAB 是什么：
//   - 一种为**主备原子广播（primary-backup atomic broadcast）**优化的共识协议。
//     一个 Leader 顺序广播所有提案（Proposal），Follower 按序确认（Ack），
//     Leader 收 quorum Ack 后提交（Commit）——保证所有副本按相同顺序应用事务。
//   - 三个阶段（phase）：
//     1. discovery（发现）：发现集群中大多数、并发现最新已提交的 zxid。
//     2. synchronization（同步）：把 Leader 的事务历史同步给 Follower，使各副本一致。
//     3. broadcast（广播）：Leader 处理新请求、广播 Proposal、收 Ack、发 Commit。
//   - 本包只实现 **broadcast 阶段**（最核心、最常用）；discovery/synchronization 省略
//     （见 NOTES 本包简化）。
//
// zxid 设计（核心）：
//   - 64 位 = 高 32 位 epoch（纪元）| 低 32 位 counter（纪元内计数）。
//   - 同 epoch 内 counter 递增；跨 epoch（leader 切换）epoch+1。
//   - 这保证全局单调有序，所有副本按 zxid 顺序提交——ZAB"强 leader 顺序"的基石。
//
// 与相邻算法的区别：
//   - 与 Paxos（internal/paxos）：ZAB 也是 majority quorum 共识，但为"主备广播"优化——
//     Leader 串行分配 zxid、顺序广播，正常路径只需一轮（Proposal→Ack→Commit），
//     比Multi-Paxos 的"每槽独立 prepare/accept"更高效。ZAB 的 phase 化
//     （discovery/sync/broadcast）与 Multi-Paxos 的 view change 类似但语义不同。
//   - 与 Raft（internal/raft）：Raft 用 term+log index 排序，ZAB 用 epoch+counter（zxid）。
//     两者都是"强 leader 顺序广播"，本质相似；ZAB 更强调 zxid 的全局单调与
//     "primary-backup"语义，Raft 更强调可理解性。
//   - 与 Viewstamped（internal/viewstamped）：VR 的 view 对应 ZAB 的 epoch，opNumber
//     对应 counter；都是 leader-based 顺序广播，差异在换主协议细节。
//
// 论文：Flavio P. Junqueira, Benjamin Reed, Marco Serafini, "Zab: High-performance
// broadcast for primary-backup systems", DSN 2011.
// https://zookeeper.apache.org/doc/r3.4.13/zab.pdf
// 早期技术报告：Junqueira, Reed, Serafini 2008.
package zab
