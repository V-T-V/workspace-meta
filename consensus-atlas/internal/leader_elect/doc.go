// Package leader_elect 实现两种经典的显式 Leader 选举算法：Bully 与 Ring。
//
// 与 Raft 的随机化选举不同，Bully / Ring 不依赖超时竞争，而是用显式的
// 消息传递在已知成员集合中确定一个唯一的协调者（coordinator）。
// 这类算法假设集群成员已知、彼此可寻址，适合强假设的固定集群。
//
// # Bully
//
// 节点用数值 ID 标识，ID 最大者当选。任一节点发现 Leader 失联即发起选举：
// 向所有更高 ID 节点发 Election 消息；若在超时内收到任一 Answer
// （"我有更高 ID"），自己退出竞选并等待 Coordinator 公告；若超时无应答，
// 则认为所有更高节点都已离线，自己当选并向全网广播 Coordinator。
// 收到 Election 且自己 ID 更大者，回 Answer 并也发起自己的选举。
//
// # Ring
//
// 节点在逻辑上排成单向环，每个节点知道自己的后继（Next）。发起者把
// Election 消息（携带自己的 ID）发给 Next，消息沿环逐跳传递；每经过一个
// 节点就把自己的 ID 加入消息的 ID 集合。消息绕一圈回到发起者后，
// 取集合中的最大 ID 当选，新 Leader 向全网广播 Coordinator。
//
// # 与 Raft 选举的区别
//
//   - Raft：随机化超时隐式选举，候选人请求投票，多数派确认；容忍节点失联。
//   - Bully/Ring：显式消息传递，无需"多数派"概念，靠 ID 大小或环遍历
//     直接决定唯一获胜者；成员必须已知。
//
// 论文：Garcia-Molina, "Elections in a Distributed Computing System",
// IEEE Transactions on Computers, C-31(1), 1982.
package leader_elect
