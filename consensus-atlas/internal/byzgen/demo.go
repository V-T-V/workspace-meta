package byzgen

import (
	"context"
	"fmt"
	"sort"

	"github.com/QiuShichang/consensus-atlas/internal/core"
)

// DemoResult 是 Byzantine Generals (OM) demo 的输出摘要。
type DemoResult struct {
	N            int                   // 节点总数（commander + lieutenants）
	F            int                   // 容忍的叛徒数
	CommanderID  core.NodeID           // commander ID
	CommanderBad bool                  // commander 是否叛徒
	Order        Order                 // 忠诚 commander 想发的命令（叛徒场景忽略）
	Decisions    map[core.NodeID]Order // 每个 lieutenant 的最终决定
	LoyalAgree   bool                  // 忠诚 lieutenant 是否达成一致
	LoyalValue   Order                 // 忠诚 lieutenant 共同决定的值（一致时才有意义）
	Traitors     []core.NodeID         // 叛徒列表（commander + lieutenants）
}

// Demo 用 3f+1 = 4 节点（1 commander + 3 lieutenant）演示 OM(m=1) 算法：
//
//  1. **忠诚 commander 场景**：commander 发 "attack"，3 个 lieutenant 中有 1 个叛徒。
//     验证 2 个忠诚 lieutenant 都决定 "attack"（OM(1) 容忍 1 叛徒）。
//  2. **叛徒 commander 场景**：commander 是叛徒，给 3 个 lieutenant 发不同值，
//     验证 3 个忠诚 lieutenant 用 majority 达成**同一个**值（一致即可，不要求是 attack）。
//
// 离线可跑（纯函数，无 transport/goroutine/time/rand，确定性轨迹）。
func Demo(ctx context.Context) (*DemoResult, error) {
	_ = ctx

	// 场景二更经典（叛徒 commander 是 Byzantine Generals 的核心难题），本 demo 主跑它。
	// n=4, f=1（4 节点容忍 1 叛徒，满足 3f+1=4）。
	const order Order = "attack"
	commander := &Commander{ID: "c1", Traitor: true} // 叛徒 commander
	lieutenants := []*Lieutenant{
		{ID: "l1", Traitor: false},
		{ID: "l2", Traitor: false},
		{ID: "l3", Traitor: false},
	}
	f := 1

	out := OM(commander, lieutenants, order, f)

	// 收集忠诚 lieutenant 的决定，验证一致性。
	agree, loyalTally := Consistent(out, lieutenants)
	var loyalValue Order
	if agree {
		// 取忠诚 lieutenant 唯一的决定值。
		for v := range loyalTally {
			loyalValue = v
			break
		}
	}

	res := &DemoResult{
		N:            1 + len(lieutenants),
		F:            f,
		CommanderID:  commander.ID,
		CommanderBad: commander.Traitor,
		Order:        order,
		Decisions:    out.Decisions,
		LoyalAgree:   agree,
		LoyalValue:   loyalValue,
		Traitors:     []core.NodeID{commander.ID},
	}

	// 兜底校验：叛徒 commander 场景下忠诚 lieutenant 必须一致（3f+1 满足）。
	if !agree {
		return res, fmt.Errorf("3f+1=%d 满足但忠诚 lieutenant 未达成一致（算法实现错误）", 1+len(lieutenants))
	}

	// 按 ID 排序决定，使输出稳定。
	sort.Slice(lieutenants, func(i, j int) bool { return lieutenants[i].ID < lieutenants[j].ID })
	return res, nil
}
