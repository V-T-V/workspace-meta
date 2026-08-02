// Package twopc 实现经典两阶段提交（Two-Phase Commit, 2PC）原子提交协议的核心循环。
//
// 2PC 是什么：
//   - 一个 Coordinator 协调多个 Participant，让一个跨节点的分布式事务要么在所有
//     节点全部提交、要么全部放弃——保证原子性（atomicity）。
//   - 阶段一（Prepare/Vote）：Coordinator 问每个 Participant "能否提交？"；
//     每个 Participant 投 Yes（锁定资源、承诺必能 Commit）或 No（直接放弃）。
//   - 阶段二（Commit/Abort）：全 Yes 则 Coordinator 下发 Commit；任一 No（或超时）
//     则下发 Abort。Participant 落实后回 Ack。
//
// 与共识算法（Raft / Paxos / PBFT）的本质区别：
//   - 共识用**多数派**（majority）容忍少数派故障，N 个节点只要 ⌊N/2⌋+1 同意即可；
//   - 2PC 要求**一致同意**（unanimity）：必须**全部** Participant 同意才能 Commit，
//     任一拒绝即 Abort。这使 2PC 无法容忍任何参与方故障——它是原子提交协议，不是共识协议。
//   - 因此 2PC 是**阻塞的**（blocking）：Coordinator 在阶段一与阶段二之间崩溃，
//     已投 Yes 的 Participant 锁定资源却不知道最终决定，只能无限期等待——这是 2PC
//     的著名缺陷，3PC（三阶段提交）试图用额外一轮 Pre-Commit 解决它。
//
// 论文：Jim Gray, "Notes on Data Base Operating Systems", Operating Systems
// Lecture Notes 60, Springer 1978（2PC 的经典出处）。
// 综述：Bernstein & Goodman, "Concurrency Control in Distributed Database
// Systems", ACM Computing Surveys 1981.
// 经典教材：Bernstein, Hadzilacos & Goodman, "Concurrency Control and Recovery
// in Database Systems", Addison-Wesley 1987（第 7 章详述 2PC 与 3PC）。
package twopc
