// Package crdt 实现无冲突复制数据类型（Conflict-free Replicated Data Type）中
// 最基础的一种：G-Counter（Grow-only Counter，只增计数器）。
//
// CRDT 是什么：
//   - 让多个节点并发更新同一份数据而不需要相互协调，只要最终互相同步，所有副本
//     就会收敛（converge）到相同状态——即最终一致性（eventual consistency）。
//   - 收敛性来自合并函数的数学性质：可交换（commutative）、可结合（associative）、
//     幂等（idempotent）。满足这三条，无论合并的顺序、次数、是否重复，结果都一致。
//
// G-Counter 模型：
//   - 每个节点维护一个向量（维度 = 节点数），只有自己那一维会递增（grow-only）。
//   - Increment(n)：自己的分量 += n。
//   - Merge(other)：每个分量取 max（不是 sum——sum 会把对方已统计的分量再加一遍）。
//   - Value()：所有分量求和，得到全局计数值。
//
// 与共识算法（Raft / Paxos / PBFT）和与 Gossip 的区别：
//   - 共识追求**强一致**：写要经 Leader / 多数派，任一时刻读到的是已提交值；
//     代价是高延迟、不容忍多数派故障。
//   - CRDT 只追求**最终一致**：任意节点本地写立即可读（可能领先/落后其他副本），
//     互相同步后才收敛；但写永远不阻塞、不需协调，可用性极高。
//   - 与本仓库 gossip 包的区别：Gossip 是"传输协议"（如何扩散状态），CRDT 是
//     "数据语义"（状态如何合并才收敛）。本包用 Gossip 式的 Push-Pull 扩散 G-Counter
//     向量，G-Counter 的 max 合并保证收敛——两者正交、可组合（Gossip + CRDT 是
//     分布式计数器/集合的经典组合，如 Riak 数据类型）。
//
// 论文：Shapiro, Preguiça, Baquero, Zawirski, "A comprehensive study of
// Convergent and Commutative Replicated Data Types", INRIA Research Report
// 7506, 2011. https://hal.inria.fr/inria-00555588/
package crdt
