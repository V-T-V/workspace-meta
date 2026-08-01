// Package paxos 实现经典 Multi-Paxos 共识算法的两阶段核心循环
// （Phase 1 Prepare/Promise + Phase 2 Accept/Accepted）。
//
// 三角色：
//   - Proposer（提议者，对应 Raft 的 Leader/Primary）：发起提案、收集多数派承诺、
//     选定最终值并广播 Accept。
//   - Acceptor（接受者）：对编号更高的提案做出"承诺"（不再接受更低编号），
//     并把已接受的最大编号提案回带，保证已选中值不会被覆盖。
//   - Learner（学习者）：收集多数派 Accepted，确定提案被选中。
//
// 与 Raft 的本质区别：
//   - 协调者"弱"：Multi-Paxos 的 Proposer 只是一个协调者，多个 Proposer 可并存竞争；
//     Acceptor 不绑定单一 Leader，谁先用更高编号拿到多数派承诺，谁就推进。
//   - 每个槽位独立提案：Multi-Paxos 把日志视作一串独立 Paxos 实例，每个实例独立
//     Prepare/Accept；为避免每条命令都走 Phase 1，稳定 Leader 可复用一轮 Prepare
//     （即"Multi-Paxos 优化"，本包演示单个实例的基础两阶段）。
//   - 编号承诺代替 commitIndex：Paxos 没有 Raft 的显式 commitIndex，安全性靠
//     "Acceptor 承诺后不再接受更低编号" + "Proposer 取 Promise 里最高编号已接受值"
//     这两条不变量保证，而非任期/索引的前缀校验。
//
// 论文：
//   - Lamport, "The Part-Time Parliament", ACM TOCS 1998
//     https://lamport.org/#qa-pubs
//   - Lamport, "Paxos Made Simple", ACM SIGACT News 2001
//     https://lamport.org/pubs/paxos-simple.pdf
package paxos
