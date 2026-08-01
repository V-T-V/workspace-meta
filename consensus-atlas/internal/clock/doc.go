// Package clock 实现分布式系统中的两种经典逻辑时钟：Lamport Clock 与 Vector Clock。
//
// 逻辑时钟解决的问题是"在没有全局物理时钟同步的前提下，如何给分布式事件定序"。
// 物理时钟（NTP）有漂移、不可信，无法保证两个事件之间的精确先后；逻辑时钟
// 放弃"真实时间"的执念，只跟踪"事件之间的因果关系"，从而推出一个一致的偏序。
//
// 两种实现：
//
//  1. Lamport Clock（core.LamportClock）：单个递增整数 C。
//     规则：本地事件 C = C + 1；收到消息 C = max(C, msg.C) + 1。
//     保证：若事件 a 因果先于 b（a → b），则 C(a) < C(b)。
//     局限：这是充分不必要条件——C(a) < C(b) 不意味着 a → b，a 与 b 也可能并发。
//     因此 Lamport Clock 无法判断两个事件是否并发。
//
//  2. Vector Clock（本包实现）：N 维向量（N = 节点数），每个分量对应一个节点。
//     规则：节点 i 的本地事件 V[i]++；节点 i 收到消息 V[i] = max(V[i], msg.V[i]) + 1，
//     其余分量 V[j] = max(V[j], msg.V[j])。
//     保证：能精确刻画因果关系——对任意两个事件 a, b，必居其一：
//     a → b（a 因果先于 b）/ b → a / 二者并发（无因果关系）。
//
// 本包在 core.LamportClock 之上提供 Vector Clock 实现，并用一个离线 demo 直观
// 对比两种时钟在因果判定能力上的差异（Lamport 给数字、Vector 给关系）。
//
// 参考文献：
//   - Lamport, "Time, Clocks, and the Ordering of Events in a Distributed System",
//     Communications of the ACM, 1978. https://lamport.org/pubs/time-clocks.pdf
//   - Mattern, "Virtual Time and Global States of Distributed Systems",
//     Proceedings of the International Workshop on Parallel and Distributed
//     Algorithms, 1989.（首次系统化 Vector Clock 并提出 happened-before 的向量判定）
package clock
