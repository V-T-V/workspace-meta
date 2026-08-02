package redteam

import "testing"

// TestDefaultCount Default() 必须返回恰好 31 个用例（28 攻击 + 3 良性）。
func TestDefaultCount(t *testing.T) {
	cases := Default()
	if len(cases) != 31 {
		t.Errorf("Default() 应返回 31 个用例, got %d", len(cases))
	}
}

// TestDefaultReturnsFreshSlice Default() 每次调用应返回同一底层数据（验证不泄漏 + 稳定）。
// （注意：Default 返回的是 defaultCases 切片头，测试不可修改其元素以免污染后续测试。）
func TestDefaultReturnsStableData(t *testing.T) {
	a := Default()
	b := Default()
	if len(a) != len(b) {
		t.Fatalf("Default() 两次调用长度不一致: %d vs %d", len(a), len(b))
	}
	for i := range a {
		if a[i].ID != b[i].ID {
			t.Errorf("Default() 第 %d 个用例 ID 不稳定: %q vs %q", i, a[i].ID, b[i].ID)
		}
	}
}

// TestMaliciousOnlyExcludesBenign MaliciousOnly 中所有用例 Category != "benign"。
func TestMaliciousOnlyExcludesBenign(t *testing.T) {
	cases := MaliciousOnly()
	if len(cases) == 0 {
		t.Fatal("MaliciousOnly 不应返回空")
	}
	for _, c := range cases {
		if c.Category == "benign" {
			t.Errorf("MaliciousOnly 不应含 benign, 命中 %s (%s)", c.ID, c.Category)
		}
	}
	// 31 总 - 3 benign = 28 攻击
	if len(cases) != 28 {
		t.Errorf("MaliciousOnly 应有 28 个用例, got %d", len(cases))
	}
}

// TestBenignOnlyAllBenign BenignOnly 中所有用例 Category == "benign"。
func TestBenignOnlyAllBenign(t *testing.T) {
	cases := BenignOnly()
	if len(cases) == 0 {
		t.Fatal("BenignOnly 不应返回空")
	}
	for _, c := range cases {
		if c.Category != "benign" {
			t.Errorf("BenignOnly 应全是 benign, 命中非 benign: %s (%s)", c.ID, c.Category)
		}
	}
	// 恰好 3 个良性用例
	if len(cases) != 3 {
		t.Errorf("BenignOnly 应有 3 个用例, got %d", len(cases))
	}
}

// TestMaliciousPlusBenignEqualsDefault 恶意 + 良性 = 总数（无遗漏、无重叠）。
func TestMaliciousPlusBenignEqualsDefault(t *testing.T) {
	mal := MaliciousOnly()
	ben := BenignOnly()
	if len(mal)+len(ben) != len(Default()) {
		t.Errorf("恶意(%d) + 良性(%d) != 总数(%d)", len(mal), len(ben), len(Default()))
	}
}

// TestByCategory ByCategory 分组正确：每个用例归入其声明的 category。
func TestByCategory(t *testing.T) {
	groups := ByCategory()
	if len(groups) == 0 {
		t.Fatal("ByCategory 不应返回空 map")
	}

	// 重建一份"category -> count"与 Default 对比
	wantCounts := map[string]int{}
	for _, c := range Default() {
		wantCounts[c.Category]++
	}
	if len(groups) != len(wantCounts) {
		t.Errorf("分组数不一致: got %d want %d", len(groups), len(wantCounts))
	}

	// 校验每组内每个用例的 category 都匹配 key，且总数对齐
	totalSeen := 0
	for cat, cases := range groups {
		for _, c := range cases {
			if c.Category != cat {
				t.Errorf("用例 %s 的 category=%q 与分组 key %q 不符", c.ID, c.Category, cat)
			}
		}
		if len(cases) != wantCounts[cat] {
			t.Errorf("分组 %q 数量错: got %d want %d", cat, len(cases), wantCounts[cat])
		}
		totalSeen += len(cases)
	}
	if totalSeen != len(Default()) {
		t.Errorf("分组内用例总数 %d != Default 总数 %d（有遗漏）", totalSeen, len(Default()))
	}
}

// TestByCategoryContainsBenign 分组中应包含 benign 组。
func TestByCategoryContainsBenign(t *testing.T) {
	groups := ByCategory()
	if _, ok := groups["benign"]; !ok {
		t.Error("ByCategory 应包含 'benign' 分组")
	}
	// benign 组应有 3 个
	if len(groups["benign"]) != 3 {
		t.Errorf("benign 组应有 3 个, got %d", len(groups["benign"]))
	}
}

// TestAllCasesFieldsNonEmpty 每个用例的 ID/Category/Input/Description 必须非空。
func TestAllCasesFieldsNonEmpty(t *testing.T) {
	cases := Default()
	if len(cases) == 0 {
		t.Fatal("Default 返回空")
	}
	for _, c := range cases {
		if c.ID == "" {
			t.Error("存在 ID 为空的用例")
		}
		if c.Category == "" {
			t.Errorf("用例 %s 的 Category 为空", c.ID)
		}
		if c.Input == "" {
			t.Errorf("用例 %s 的 Input 为空", c.ID)
		}
		if c.Description == "" {
			t.Errorf("用例 %s 的 Description 为空", c.ID)
		}
	}
}

// TestCaseIDsUnique 所有用例 ID 必须唯一（防止重复定义）。
func TestCaseIDsUnique(t *testing.T) {
	cases := Default()
	seen := map[string]bool{}
	for _, c := range cases {
		if seen[c.ID] {
			t.Errorf("用例 ID %q 重复", c.ID)
		}
		seen[c.ID] = true
	}
}

// TestMaliciousCasesHaveAttackInputs 攻击用例的 Input 不应是良性查询。
// （启发式校验：不应包含 "weather"/"春天的诗" 等良性关键词。）
func TestMaliciousCasesHaveAttackInputs(t *testing.T) {
	for _, c := range MaliciousOnly() {
		switch {
		case contains(c.Input, "weather today"), contains(c.Input, "春天的诗"), contains(c.Input, "neural networks work"):
			t.Errorf("恶意用例 %s 的 Input 看起来是良性: %q", c.ID, c.Input)
		}
	}
}

// contains 简单子串包含（避免 import strings 也行，但显式更清晰）。
func contains(s, sub string) bool {
	if len(sub) == 0 {
		return true
	}
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
