// Package raft 实现经典 Raft 共识算法的Leader选举 + 日志复制核心循环。
//
// Raft 与 Multi-Paxos 的本质区别：
//   - Raft：强 Leader 模型。所有写流量经 Leader，Leader 用"任期(term)" +
//     "已提交索引(commitIndex)"两个显式量简化了 Paxos 的"编号/承诺"心智。
//     Leader 选举用随机化超时避免活锁，日志匹配用 (prevIndex, prevTerm) 前缀校验。
//   - Multi-Paxos：Leader 角色（Proposer）弱化为"协调者"，acceptor 接受任何
//     比已承诺编号更高的提案，日志顺序由"已接受的最大编号"隐式决定，无显式 commitIndex。
//
// 本包实现 Raft 的两个核心子问题：
//   - Leader 选举（RequestVote RPC，随机化选举超时）
//   - 日志复制（AppendEntries RPC，prevIndex/prevTerm 前缀校验 + commitIndex 推进）
//
// 论文：In Search of an Understandable Consensus Algorithm (Ongaro & Ousterhout, USENIX ATC 2014)
// https://raft.github.io/raft.pdf
package raft
