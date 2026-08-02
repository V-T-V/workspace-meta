// Package viewstamped 实现 Viewstamped Replication（VR）——与 Paxos / Raft 同时代的
// 第三种主流共识算法。
//
// VR 是什么：
//   - 一组副本（replica）复制一个状态机，提供强一致的读写；当前 view 下有一个
//     **Primary**（类似 Raft 的 Leader）负责排序客户端请求，其余为 Backup。
//   - 正常操作（Normal Operation）：Client 把请求发给 Primary；Primary 分配 opNumber、
//     广播 Prepare；Backup 记日志后回 PrepareOK；Primary 收 quorum PrepareOK 即执行
//     并回 Reply。
//   - 视图变更（View Change）：Backup 检测到 Primary 失联（超时无消息），推进 view 号，
//     通过 StartViewChange / DoViewChange / StartView 三消息选出新 Primary。
//
// 与相邻算法的区别：
//   - 与 Raft（本仓库 internal/raft）：VR 与 Raft 几乎等价——view 对应 term，Primary 对应
//     Leader，opNumber 对应 log index，PrepareOK quorum 对应 AppendEntries 复制 + commit。
//     主要差异在"设计哲学"：VR 从"复制状态机"出发（Oki 1988，比 Raft 早 26 年），
//     Raft 从"可理解性"出发重新设计。两者都需多数派 quorum（n≥2f+1）。
//   - 与 Paxos（internal/paxos）：VR/Paxos/Raft 三者数学等价（都是 majority quorum 共识），
//     但 Paxos 以"值"为单位（每个槽位独立共识），VR/Raft 以"有序日志"为单位（Primary 顺序）。
//   - 与 ZAB（internal/zab）：两者都是"主备原子广播"思路（一个 leader 顺序广播），
//     ZAB 更强调 zxid 的全局有序与 phase 化（discovery/sync/broadcast）。
//
// 论文：
//   - 原始版：Brian Oki & Barbara Liskov, "Viewstamped Replication: A New Primary Copy
//     Method to Support Highly-Available Distributed Systems", PODC 1988.
//   - 重写版（推荐）：Barbara Liskov & James Cowling, "Viewstamped Replication Revisited",
//     2012. https://pmg.csail.mit.edu/papers/vr-revisited.pdf
package viewstamped
