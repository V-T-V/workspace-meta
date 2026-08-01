package types

import "testing"

// ===== RiskScore =====

// TestRiskScoreEmpty 空输入应返回 0。
func TestRiskScoreEmpty(t *testing.T) {
	if got := RiskScore(nil); got != 0 {
		t.Errorf("RiskScore(nil) = %d, want 0", got)
	}
	if got := RiskScore([]Detection{}); got != 0 {
		t.Errorf("RiskScore([]) = %d, want 0", got)
	}
}

// TestRiskScoreSingleMedium 单个 Medium 检测：base=50 + 加权 5 = 55。
func TestRiskScoreSingleMedium(t *testing.T) {
	got := RiskScore([]Detection{{Severity: SeverityMedium}})
	if got != 55 {
		t.Errorf("单 Medium: base 50 + 加权 5 = 55, got %d", got)
	}
}

// TestRiskScoreSingleCritical 单个 Critical 检测：base=95 + 加权 5 = 100。
func TestRiskScoreSingleCritical(t *testing.T) {
	got := RiskScore([]Detection{{Severity: SeverityCritical}})
	// 95 + 5 = 100（正好触顶）
	if got != 100 {
		t.Errorf("单 Critical: base 95 + 加权 5 = 100, got %d", got)
	}
}

// TestRiskScoreInfoLowNotWeighted Info/Low 不参与加权（< Medium），只取 base。
func TestRiskScoreInfoLowNotWeighted(t *testing.T) {
	// 单 Info：base 10，无加权
	if got := RiskScore([]Detection{{Severity: SeverityInfo}}); got != 10 {
		t.Errorf("单 Info: base 10, got %d", got)
	}
	// 单 Low：base 25，无加权
	if got := RiskScore([]Detection{{Severity: SeverityLow}}); got != 25 {
		t.Errorf("单 Low: base 25, got %d", got)
	}
	// 单 High：base 75 + 5 = 80
	if got := RiskScore([]Detection{{Severity: SeverityHigh}}); got != 80 {
		t.Errorf("单 High: base 75 + 加权 5 = 80, got %d", got)
	}
}

// TestRiskScoreMultipleCappedAt100 多个中/高/严重叠加，最终封顶 100。
func TestRiskScoreMultipleCappedAt100(t *testing.T) {
	dets := []Detection{
		{Severity: SeverityCritical}, // base 95
		{Severity: SeverityHigh},     // 不超 base
		{Severity: SeverityMedium},   // 不超 base
		{Severity: SeverityMedium},   // 不超 base
	}
	// base = 95，加权 = 4 个 >= Medium 各 +5 = 20 → 95 + 20 = 115 → 封顶 100
	got := RiskScore(dets)
	if got != 100 {
		t.Errorf("多检测叠加应封顶 100, got %d", got)
	}
}

// TestRiskScoreMaxBaseWins 最高严重度的 base 决定主分（加权只加不减）。
func TestRiskScoreMaxBaseWins(t *testing.T) {
	dets := []Detection{
		{Severity: SeverityLow},    // base 25，不参与加权
		{Severity: SeverityMedium}, // base 50，加权 +5
	}
	// max base = 50，加权只有 Medium 一个 +5 → 55
	if got := RiskScore(dets); got != 55 {
		t.Errorf("max base 50 + 加权 5 = 55, got %d", got)
	}
}

// TestRiskScoreExactly100Boundary 多 Medium 不应超过 100。
func TestRiskScoreExactly100Boundary(t *testing.T) {
	// 10 个 Medium：base 50 + 10×5 = 100（恰好，不超）
	dets := make([]Detection, 10)
	for i := range dets {
		dets[i] = Detection{Severity: SeverityMedium}
	}
	if got := RiskScore(dets); got != 100 {
		t.Errorf("10 个 Medium 应 = 100（50 + 50）, got %d", got)
	}
	// 11 个 Medium：50 + 55 = 105 → 封顶 100
	dets = append(dets, Detection{Severity: SeverityMedium})
	if got := RiskScore(dets); got != 100 {
		t.Errorf("11 个 Medium 应封顶 100, got %d", got)
	}
}

// ===== RiskLevel 阈值 =====

func TestRiskLevelThresholds(t *testing.T) {
	cases := []struct {
		score int
		want  string
	}{
		{0, "SAFE"},
		{19, "SAFE"}, // < 20 仍是 SAFE
		{20, "LOW"},  // 边界：20 进入 LOW
		{39, "LOW"},
		{40, "MEDIUM"}, // 边界：40 进入 MEDIUM
		{59, "MEDIUM"},
		{60, "HIGH"}, // 边界：60 进入 HIGH
		{79, "HIGH"},
		{80, "CRITICAL"}, // 边界：80 进入 CRITICAL
		{100, "CRITICAL"},
		{150, "CRITICAL"}, // 超出范围也应归 CRITICAL（>= 80）
	}
	for _, c := range cases {
		if got := RiskLevel(c.score); got != c.want {
			t.Errorf("RiskLevel(%d) = %q, want %q", c.score, got, c.want)
		}
	}
}

// TestRiskLevelBoundariesExplicit 明确测试每个阈值边界（防止 off-by-one）。
func TestRiskLevelBoundariesExplicit(t *testing.T) {
	// 19 vs 20
	if RiskLevel(19) == RiskLevel(20) {
		t.Error("19 和 20 应跨越 SAFE→LOW 边界")
	}
	// 39 vs 40
	if RiskLevel(39) == RiskLevel(40) {
		t.Error("39 和 40 应跨越 LOW→MEDIUM 边界")
	}
	// 59 vs 60
	if RiskLevel(59) == RiskLevel(60) {
		t.Error("59 和 60 应跨越 MEDIUM→HIGH 边界")
	}
	// 79 vs 80
	if RiskLevel(79) == RiskLevel(80) {
		t.Error("79 和 80 应跨越 HIGH→CRITICAL 边界")
	}
}

// ===== Severity.String() =====

func TestSeverityString(t *testing.T) {
	cases := []struct {
		s    Severity
		want string
	}{
		{SeverityInfo, "INFO"},
		{SeverityLow, "LOW"},
		{SeverityMedium, "MEDIUM"},
		{SeverityHigh, "HIGH"},
		{SeverityCritical, "CRITICAL"},
	}
	for _, c := range cases {
		if got := c.s.String(); got != c.want {
			t.Errorf("Severity(%d).String() = %q, want %q", c.s, got, c.want)
		}
	}
}

// TestSeverityUnknown 越界 severity 返回 "UNKNOWN"。
func TestSeverityUnknown(t *testing.T) {
	if got := Severity(999).String(); got != "UNKNOWN" {
		t.Errorf("越界 Severity 应返回 UNKNOWN, got %q", got)
	}
	if got := Severity(-1).String(); got != "UNKNOWN" {
		t.Errorf("负值 Severity 应返回 UNKNOWN, got %q", got)
	}
}

// ===== AttackType.String() =====

func TestAttackTypeString(t *testing.T) {
	cases := []struct {
		a    AttackType
		want string
	}{
		{AttackNone, "NONE"},
		{AttackPromptInjection, "PROMPT_INJECTION"},
		{AttackJailbreak, "JAILBREAK"},
		{AttackPIILeak, "PII_LEAK"},
		{AttackRoleOverride, "ROLE_OVERRIDE"},
		{AttackDataExfiltration, "DATA_EXFIL"},
		{AttackDan, "DAN"},
	}
	for _, c := range cases {
		if got := c.a.String(); got != c.want {
			t.Errorf("AttackType(%d).String() = %q, want %q", c.a, got, c.want)
		}
	}
}

// TestAttackTypeUnknown 越界 attack type 返回 "UNKNOWN"。
func TestAttackTypeUnknown(t *testing.T) {
	if got := AttackType(999).String(); got != "UNKNOWN" {
		t.Errorf("越界 AttackType 应返回 UNKNOWN, got %q", got)
	}
}
