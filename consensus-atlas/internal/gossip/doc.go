// Package gossip 实现经典 Gossip（流行病/epidemic）协议的 Push-Pull 反熵循环。
//
// Gossip 是什么：
//   - 每个节点周期性地挑一个（或少数几个）邻居，与之交换状态，就像传染病
//     在人群中口口相传。任意一条新信息在 O(log N) 轮内即可高概率扩散到全网。
//   - 本包用 Push-Pull 模式：发起方先 Push 自己的全量状态给对方，对方合并后
//     再 Pull 一份自己的全量状态回来（一次 Request/Response 完成双向同步）。
//
// 与共识算法（Raft / Paxos / PBFT）的本质区别：
//   - 共识追求**强一致**（linearizability），需要多数派、Leader、任期、两阶段；
//   - Gossip 只追求**最终一致**（eventual consistency），不保证读到最新值、
//     不保证收敛轮数，但每个节点只需联系 O(1) 个邻居，极具扩展性——
//     适合超大规模集群下"弱一致就够用"的状态同步（成员表、路由表、缓存失效等）。
//
// 论文：Demers et al. "Epidemic Algorithms for Replicated Database Maintenance",
// ACM PODC 1987. https://dl.acm.org/doi/10.1145/41840.41841
package gossip
