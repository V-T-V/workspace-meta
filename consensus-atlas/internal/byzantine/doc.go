// Package byzantine 实现 PBFT（Practical Byzantine Fault Tolerance）的
// 三阶段共识核心循环：pre-prepare / prepare / commit。
//
// PBFT 与 Raft / Multi-Paxos 的本质区别在于故障模型：
//   - Raft / Paxos 只容忍 crash fault（节点崩溃/失联），需要 > 1/2 多数派即可。
//   - PBFT 容忍 byzantine fault（节点任意恶意：撒谎、伪造、双发、串谋），
//     需要 > 2/3 多数派（quorum = 2f+1，其中 n = 3f+1）。
//
// 代价是消息复杂度：每个请求都要在全网 O(n²) 两两交换投票（prepare + commit
// 各一轮），而 Raft 只需 Leader→Follower 的 O(n) 复制。这是为了在"无法信任
// 任何人"的环境下，仅凭投票数量排除拜占庭节点的影响。
//
// 三阶段：
//   - pre-prepare：Primary 广播提议（Sequence, View, op），确立请求顺序。
//   - prepare：各 Replica 收到后向全网广播 Prepare 投票；收齐 2f 个 prepare
//     （含自己，共 2f+1）即进入 prepared——意味"足够多人认可这个顺序"。
//   - commit：进入 prepared 的节点再广播 Commit 投票；收齐 2f+1 个 commit
//     即进入 committed，安全执行 op。两轮投票保证即使 f 个节点撒谎，
//     任意两个 quorum 必有诚实节点交集（safety）。
//
// 论文：Castro & Liskov, "Practical Byzantine Fault Tolerance", OSDI 1999
// https://pmg.csail.mit.edu/papers/osdi99.pdf
//
// 本包为教学清晰做了简化：Signature 用占位字符串（不实现真签名/MAC）、
// 不实现 view-change 故障恢复、不实现 checkpoint 日志压缩。聚焦三阶段共识本体。
package byzantine
