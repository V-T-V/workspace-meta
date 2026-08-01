package byzgen

import (
	"fmt"
	"sort"

	"github.com/QiuShichang/consensus-atlas/internal/core"
)

// 命令值用一个字符串表示（"attack" / "retreat" 等）。OM 算法对值域无要求，
// 只要可比较、可投票取多数即可。本包用 string 作 Order。
type Order string

// 消息 Kind 常量。OM 算法是同步递归的，本包用"直接函数调用"模拟而非 transport
// 异步投递（见 demo.go 说明）。这里保留 Kind 常量仅用于给消息打标，便于观测/测试。
const (
	KindOM = "OM" // 一轮 OM(m) 转发的消息：commander 把 order 发给 lieutenant
)

// OMMessage 是一轮 OM 转发的消息负载（观测/文档用，本包实际用直接函数调用）。
//
// OM 算法是"口头消息"（oral messages）模型：消息内容可被叛徒任意篡改/伪造，
// 不可签名、不可加密。因此每条消息只携带一个 order 值，叛徒可以给不同人发不同值。
//
// 字段：
//   - Order：本轮 commander 发出的命令（叛徒可对不同接收方发不同值）。
//   - Path：消息经过的节点序列（commander → ... → 当前接收者），用于 OM 递归的
//     "谁是 commander"判定与 IC 约束演示。路径长度 = 当前递归深度 + 1。
type OMMessage struct {
	Order Order
	Path  []core.NodeID // 消息传播路径（首元素是原始 commander）
}

// Commander 是发起命令的将军。在 OM 算法里 commander 在每一层递归都出现一次：
// 最外层是原 commander，之后每个 lieutenant 充当新 commander 跑 OM(m-1)。
//
// 叛徒 commander 可以对每个 lieutenant 发不同的 order（Byzantine 行为）。
type Commander struct {
	ID      core.NodeID
	Traitor bool // 是否叛徒
}

// Lieutenant 是接收命令并参与递归 OM 协议的将军。
// 每个 lieutenant 最终用 majority(votes) 决定一个一致值。
type Lieutenant struct {
	ID      core.NodeID
	Traitor bool // 是否叛徒
}

// Outcome 是一次完整 OM(m) 运行后，每个 lieutenant 的最终决定值。
// 忠诚 lieutenant 的决定应当一致（OM 的正确性保证）。
type Outcome struct {
	// Decisions[lieutenantID] = 该 lieutenant 经过 OM 协议后决定的 order。
	Decisions map[core.NodeID]Order
}

// OM 执行一次完整的 OM(m) 算法（m=f）：commander 向 lieutenants 发命令，
// 容忍最多 f 个叛徒，返回每个 lieutenant 的最终决定。
//
// 前置条件：n = 1(commander) + len(lieutenants)，要求 n >= 3f+1 才能容忍 f 个叛徒。
// 若不满足，忠诚 lieutenant 仍会运行但**不保证**一致（下界条件被违反）。
// 调用方可在外层用 CheckQuorum 校验。
//
// commander.Traitor / lieutenant.Traitor 决定各自是否为叛徒：
//   - 忠诚 commander：对所有 lieutenant 发同一个 order。
//   - 叛徒 commander：对每个 lieutenant 发不同的 order（模拟 Byzantine 篡改）。
//   - 忠诚 lieutenant：忠实转发收到的值，最后用 majority 决定。
//   - 叛徒 lieutenant：转发时可能篡改值。
//
// 该函数是纯函数（无 transport、无 goroutine），确定性。OM 算法天然同步递归，
// 用直接函数调用模拟即可，无需 MemTransport。
func OM(commander *Commander, lieutenants []*Lieutenant, order Order, f int) *Outcome {
	m := f
	decisions := make(map[core.NodeID]Order, len(lieutenants))

	// 把所有将军（commander + lieutenants）装进一个 id→node 索引，递归内按 ID 取节点。
	all := make(map[core.NodeID]*Lieutenant, len(lieutenants)+1)
	all[commander.ID] = &Lieutenant{ID: commander.ID, Traitor: commander.Traitor}
	for _, lt := range lieutenants {
		all[lt.ID] = lt
	}
	// lieutenant ID 列表（固定顺序，递归内遍历用）。
	ltIDs := make([]core.NodeID, 0, len(lieutenants))
	for _, lt := range lieutenants {
		ltIDs = append(ltIDs, lt.ID)
	}

	// 每个 lieutenant 的决定 = 从他视角跑的 omStep（其余 lieutenant 视作同伴，原 commander 视作 sender）。
	for _, lt := range lieutenants {
		// 其他 lieutenant（不含自己）。
		others := idsExcept(ltIDs, lt.ID)
		decisions[lt.ID] = omStep(commander.ID, lt.ID, others, order, m, all)
	}

	return &Outcome{Decisions: decisions}
}

// omStep 实现 OM(m) 的单步：sender 向 receiver 发送 order（经 m 层递归转发），
// 返回 receiver 最终决定的值。
//
// 对应论文 OM(m) 定义：
//
//	OM(0):
//	  receiver 直接采用收到的值（叛徒 sender 可能给了篡改值）。
//	OM(m), m>0:
//	  (1) sender 把 order 发给所有 companions（receiver 视角下的其他 lieutenant）。
//	  (2) 每个 companion i 收到值 vi（sender 若叛徒则 vi 可能不同），i 充当新 sender
//	      对其余人跑 OM(m-1)。
//	  (3) receiver 用 majority(自己收到的原始值 + 各 companion 转发来的值) 决定。
//
// 参数：
//   - senderID：本轮发值的将军（递归各层会变）。
//   - receiverID：本轮收值并要做决定的将军。
//   - companions：除 receiver 外的其他 lieutenant（这一轮要被 sender 发到的对象）。
//   - order：sender 要发的值（若 sender 是叛徒，发给每个 companion 的值可不同）。
//   - m：剩余递归深度。
//   - all：全部将军的 id→节点表（查 Traitor 用）。
func omStep(senderID, receiverID core.NodeID, companions []core.NodeID, order Order, m int, all map[core.NodeID]*Lieutenant) Order {
	// OM(0)：receiver 直接采用收到的值。
	// sender 若是叛徒，"发给 receiver 的值"由调用方在 order 参数里已确定（每接收方一份）。
	if m == 0 {
		return order
	}

	// (1) sender 把 order 发给每个 companion；叛徒 sender 给每个 companion 发不同值。
	sent := make(map[core.NodeID]Order, len(companions))
	sender := all[senderID]
	for i, compID := range companions {
		if sender != nil && sender.Traitor {
			sent[compID] = traitorOrder(order, i)
		} else {
			sent[compID] = order
		}
	}

	// (2) 每个 companion i 收到 sent[i]，i 充当新 sender，向"除自己外的其他 companions + receiver"
	//     跑 OM(m-1)，把 i 的视角结果收集给 receiver。
	//     receiver 从每个 companion j 收到的"j 转发的值"= omStep(j 作为 sender, receiver 视角)。
	//     注意：receiver 要收集的是"当 j 当 commander 跑 OM(m-1) 时，receiver 看到的值"。
	receivedFromCompanions := make(map[core.NodeID]Order, len(companions))
	for _, compID := range companions {
		vi := sent[compID]
		// j 充当 sender 跑 OM(m-1)，发给"除 j 自己外的其他 companions + receiver"。
		// 但 receiver 的视角：只关心 j 最终告诉他什么——
		// 即 j 作为 sender，receiver 作为接收方，剩余同伴 = companions 去掉 j（保留其他同伴做递归转发）。
		subCompanions := idsExcept(companions, compID)
		// receiver 从 j 的 OM(m-1) 中收到的值。
		receivedFromCompanions[compID] = omStep(compID, receiverID, subCompanions, vi, m-1, all)
	}

	// (3) receiver 用 majority：自己从原 sender 收到的值 + 各 companion 转发来的值。
	votes := make([]Order, 0, len(receivedFromCompanions)+1)
	votes = append(votes, order) // receiver 自己收到的原始值
	for _, compID := range companions {
		votes = append(votes, receivedFromCompanions[compID])
	}
	return majority(votes)
}

// majority 返回 votes 中的多数值（出现次数最多的值）。
// 平票时按字典序取较小者（确定性，便于测试）。
//
// 这是 OM 算法的"决策函数"：每个 lieutenant 收到一堆值（含叛徒伪造的杂音），
// 用 majority 把叛徒的杂音过滤掉——只要忠诚节点占多数（n>=3f+1），多数值必然是
// 忠诚节点发的同一个真值。
//
// 叛徒 commander 场景下所有值可能都不同（平票），此时 majority 取字典序最小者，
// 因为决策函数对所有忠诚 lieutenant 完全一致（输入集合相同），故结果仍一致——
// 满足 Agreement（虽不满足 Validity，叛徒 commander 无义务发真值）。
func majority(votes []Order) Order {
	if len(votes) == 0 {
		return ""
	}
	counts := make(map[Order]int, len(votes))
	for _, v := range votes {
		counts[v]++
	}
	// 找出现次数最多者；平票取字典序较小者（确定）。
	var best Order
	bestCount := -1
	keys := make([]Order, 0, len(counts))
	for k := range counts {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	for _, k := range keys {
		if counts[k] > bestCount {
			bestCount = counts[k]
			best = k
		}
	}
	return best
}

// traitorOrder 模拟叛徒篡改：基于原值和索引生成一个"看起来不同"的值。
// 用 idx 确保不同接收者拿到不同值（制造最大混乱）。保留原值前缀便于人眼辨识。
func traitorOrder(base Order, idx int) Order {
	return Order(fmt.Sprintf("%s-fake%d", base, idx))
}

// idsExcept 返回 ids 中排除 drop 后的新切片（不改原切片）。
func idsExcept(ids []core.NodeID, drop core.NodeID) []core.NodeID {
	out := make([]core.NodeID, 0, len(ids))
	for _, id := range ids {
		if id != drop {
			out = append(out, id)
		}
	}
	return out
}

// CheckQuorum 校验 OM 容错下界：n 个节点最多容忍 (n-1)/3 个叛徒。
// 返回最多可容忍的叛徒数 f（即 ⌊(n-1)/3⌋）。
// 论文定理：用口头消息，3f+1 是容忍 f 个叛徒的充要条件（下界）。
func CheckQuorum(n int) int {
	if n < 1 {
		return 0
	}
	return (n - 1) / 3
}

// Consistent 检查 outcome 中所有**忠诚** lieutenant 的决定是否一致。
// 叛徒 lieutenant 的决定不计（叛徒可输出任意值）。
// 返回 (consistent, loyalDecisions)：loyalDecisions 是忠诚 lieutenant 的决定值→计数。
func Consistent(out *Outcome, lieutenants []*Lieutenant) (bool, map[Order]int) {
	loyal := make(map[Order]int)
	for _, lt := range lieutenants {
		if !lt.Traitor {
			loyal[out.Decisions[lt.ID]]++
		}
	}
	return len(loyal) <= 1, loyal
}
