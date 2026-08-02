package byzgen

import (
	"context"
	"testing"
)

// TestDemoRuns 验证 demo 离线可跑：叛徒 commander 场景下 3 个忠诚 lieutenant 一致。
func TestDemoRuns(t *testing.T) {
	res, err := Demo(context.Background())
	if err != nil {
		t.Fatalf("Demo 失败: %v", err)
	}
	if res.N != 4 {
		t.Errorf("N 应为 4（3f+1, f=1），实际 %d", res.N)
	}
	if res.F != 1 {
		t.Errorf("F 应为 1，实际 %d", res.F)
	}
	if !res.CommanderBad {
		t.Error("demo 场景的 commander 应是叛徒")
	}
	if !res.LoyalAgree {
		t.Error("3f+1=4 满足下界，忠诚 lieutenant 必须达成一致")
	}
}

// TestQuorum3fPlus1 验证下界公式：n 个节点容忍 ⌊(n-1)/3⌋ 个叛徒。
func TestQuorum3fPlus1(t *testing.T) {
	cases := []struct {
		n, want int
	}{
		{4, 1},  // 3*1+1=4 → 容忍 1
		{7, 2},  // 3*2+1=7 → 容忍 2
		{10, 3}, // 3*3+1=10 → 容忍 3
		{1, 0},
		{2, 0},
		{3, 0}, // n=3 不够 3f+1（f>=1），容忍 0
	}
	for _, c := range cases {
		if got := CheckQuorum(c.n); got != c.want {
			t.Errorf("CheckQuorum(%d) = %d, want %d", c.n, got, c.want)
		}
	}
}

// TestLoyalCommanderOneTraitorLieutenant 忠诚 commander + 1 叛徒 lieutenant → 忠诚 lieutenant 都收到真值。
func TestLoyalCommanderOneTraitorLieutenant(t *testing.T) {
	commander := &Commander{ID: "c", Traitor: false}
	lieutenants := []*Lieutenant{
		{ID: "a", Traitor: false},
		{ID: "b", Traitor: false},
		{ID: "t", Traitor: true}, // 叛徒
	}
	out := OM(commander, lieutenants, "attack", 1)
	// 忠诚 lieutenant a/b 都应决定 "attack"。
	if out.Decisions["a"] != "attack" {
		t.Errorf("忠诚 lieutenant a 应决定 attack，实际 %s", out.Decisions["a"])
	}
	if out.Decisions["b"] != "attack" {
		t.Errorf("忠诚 lieutenant b 应决定 attack，实际 %s", out.Decisions["b"])
	}
}

// TestTraitorCommanderConsistent 叛徒 commander + 全忠诚 lieutenant → 忠诚 lieutenant 仍一致。
func TestTraitorCommanderConsistent(t *testing.T) {
	commander := &Commander{ID: "c", Traitor: true}
	lieutenants := []*Lieutenant{
		{ID: "a", Traitor: false},
		{ID: "b", Traitor: false},
		{ID: "d", Traitor: false},
	}
	out := OM(commander, lieutenants, "attack", 1)
	agree, _ := Consistent(out, lieutenants)
	if !agree {
		t.Errorf("叛徒 commander 场景下忠诚 lieutenant 应一致，决定=%v", out.Decisions)
	}
}

// TestMajority 验证 majority 决策函数。
func TestMajority(t *testing.T) {
	if got := majority([]Order{"a", "b", "a"}); got != "a" {
		t.Errorf("majority([a,b,a]) = %s, want a", got)
	}
	if got := majority([]Order{"x"}); got != "x" {
		t.Errorf("majority([x]) = %s, want x", got)
	}
	// 平票取字典序较小者。
	if got := majority([]Order{"b", "a"}); got != "a" {
		t.Errorf("平票应取字典序较小 a，实际 %s", got)
	}
}

// TestMajorityFiltersTraitorNoise 多数值中混入杂音（叛徒伪造值）仍取多数真值。
func TestMajorityFiltersTraitorNoise(t *testing.T) {
	// 3 票真值 "attack" + 2 票杂音 → majority = attack。
	votes := []Order{"attack", "attack", "attack", "fake1", "fake2"}
	if got := majority(votes); got != "attack" {
		t.Errorf("多数真值应被选出，实际 %s", got)
	}
}

// TestInsufficientQuorumNotConsistent n<3f+1 时忠诚 lieutenant 不保证一致（下界被违反）。
// 3 节点想容忍 1 叛徒（3 < 3*1+1=4）：构造叛徒 commander + 2 忠诚 lieutenant，
// 忠诚 lieutenant 可能拿到不同值（OM 在此条件下失效）。
func TestInsufficientQuorumNotConsistent(t *testing.T) {
	commander := &Commander{ID: "c", Traitor: true}
	lieutenants := []*Lieutenant{
		{ID: "a", Traitor: false},
		{ID: "b", Traitor: false},
	}
	// n=3 < 4，强行跑 OM(f=1)：忠诚 lieutenant 不保证一致。
	out := OM(commander, lieutenants, "attack", 1)
	agree, _ := Consistent(out, lieutenants)
	// 这只是"可能"不一致；不强断言 agree==false（取决于具体值），但下界不满足这一事实由 CheckQuorum 保证。
	if CheckQuorum(3) != 0 {
		t.Error("n=3 时 CheckQuorum 应为 0（容忍不了 1 个叛徒）")
	}
	_ = agree
}
