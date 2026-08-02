// Package byzgen 实现拜占庭将军问题（Byzantine Generals Problem）的经典解法：
// Lamport / Shostak / Pease 1982 的**口头消息算法（Oral Messages algorithm, OM）**。
//
// 拜占庭将军问题是什么：
//   - 一群将军包围一座城，必须就"进攻还是撤退"达成一致协议；但其中有叛徒
//     （Byzantine 故障）会任意扰乱——发假消息、对不同人说不同的话、装聋作哑。
//   - 忠诚将军需要在有叛徒的情况下仍达成一致行动（agreement），且若总司令忠诚，
//     则所有忠诚将军都执行总司令的命令（validity）。
//   - 这是分布式容错的根本难题：节点可能不只"崩溃"，还可能"作恶"——
//     这正是 PBFT / 区块链共识要解决的世界（容错模型比 Raft/Paxos 的崩溃模型更强）。
//
// OM 算法（本包实现）：
//   - 司令官（commander）向 n-1 个中将（lieutenant）发命令，最多 f 个叛徒。
//   - OM(0)：commander 直接发值，lieutenant 直接采用。
//   - OM(m)：commander 发值给所有 lieutenant；每个 lieutenant 用 OM(m-1) 把他收到的值
//     转发给其他人；最后每个 lieutenant 用 majority(收到的所有值) 决定。
//   - 容错下界（口头消息模型）：需 n >= 3f+1 才能容忍 f 个叛徒。
//
// 与相邻算法的区别：
//   - 与 internal/byzantine（PBFT）的关系：PBFT 是 Castro-Liskov 1999 提出的**实用化**
//     拜占庭容错，用三阶段（pre-prepare/prepare/commit）+ 签名 + 视图变更，把 OM 的
//     O(n^(m+1)) 指数消息复杂度降到 O(n^2)，并支持连续请求。OM 是 PBFT 的理论前身：
//     PBFT 的 prepare 阶段本质上是在做 OM 的"间接互传 + majority"，只是改用显式投票计数
//     和 quorum 证书替代 OM 的递归。
//   - 与 Raft/Paxos 的区别：Raft/Paxos 只容忍**崩溃故障**（节点要么正常要么完全失联），
//     多数派即可（n>=2f+1）；拜占庭模型容忍**任意故障/作恶**，需 3f+1 节点 + 多数派。
//   - 本包是"纯函数式"实现（无 transport/goroutine），因为 OM 是同步递归的口头消息算法，
//     直接用函数调用模拟最清晰（与 internal/crdt 的纯函数取向类似，区别于 raft/paxos 的
//     transport 驱动）。
//
// 论文：Leslie Lamport, Robert Shostak, Marshall Pease, "The Byzantine Generals
// Problem", ACM Transactions on Programming Languages and Systems (TOPLAS),
// July 1982. https://lamport.org/pubs/byz.pdf
package byzgen
