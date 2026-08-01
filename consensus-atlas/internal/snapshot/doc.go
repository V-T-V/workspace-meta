// Package snapshot 实现 Chandy-Lamport 分布式快照算法（1985）：在不停止系统运行的
// 前提下，记录一个**一致的全局状态**（consistent global state）。
//
// 算法是什么：
//   - N 个进程通过**单向 FIFO 通道**互连发消息，通道可靠且先进先出。
//   - 要记录全局状态，任一进程可发起快照：先记录本地状态，然后给所有出通道发一个
//     特殊的 **marker** 消息。
//   - 进程首次收到 marker 时：记录本地状态，给所有出通道发 marker，并开始记录此后
//     到达该入通道的应用消息作为该通道的状态。
//   - 已记录后再收到 marker 的通道：该通道状态为空（marker 前的在途消息已先于 marker
//     到达并被消费，FIFO 保证）。
//   - 所有进程都收到 marker 后，汇总即得一致的全局快照。
//
// 为什么一致（CUT 性质）：
//   - marker 把消息历史切成"快照前/快照后"。每条消息要么属于"被记录前消费"
//     （在本地状态里），要么属于"在途被记入通道状态"——两者不重不漏。
//   - FIFO 保证 marker 严格在其前驱消息之后到达，使切割点（CUT）良定义。
//
// 与相邻算法的区别：
//   - 与 raft/paxos/twopc（共识/原子提交）不同：快照不"达成一致"，只"如实记录"
//     一个可达的全局状态——可能从未真实同时存在过，但满足一致性约束（CUT）。
//   - 与 byzantine（拜占庭容错）不同：本算法假设通道可靠 FIFO、进程诚实（crash 模型），
//     不处理作恶节点。
//   - 与 core 包的关系：本包不依赖 core（无 NodeID/Transport 需求），用纯 Go 的显式
//     通道队列实现——取向与 crdt 包（纯函数式）类似，区别于 raft/paxos 的 transport 驱动。
//
// 应用：死锁检测（用快照构造等待图）、全局 checkpoint、分布式调试/重现、
// 终止检测（stable property detection）。
//
// 论文：K. Mani Chandy, Leslie Lamport, "Distributed Snapshots: Determining
// Global States of Distributed Systems", ACM Transactions on Computer Systems,
// Vol. 3, No. 1, Feb. 1985. https://lamport.org/pubs/chandy-lamport.pdf
package snapshot
