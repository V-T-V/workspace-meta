// Package types 定义 ai-safety-atlas 的共享类型：检测结果、严重度、攻击类型。
package types

// Severity 检测结果的严重程度。
type Severity int

const (
	SeverityInfo     Severity = iota // 提示信息
	SeverityLow                      // 低风险
	SeverityMedium                   // 中等风险
	SeverityHigh                     // 高风险
	SeverityCritical                 // 严重风险（可能直接被利用）
)

// String 返回严重度的可读名。
func (s Severity) String() string {
	switch s {
	case SeverityInfo:
		return "INFO"
	case SeverityLow:
		return "LOW"
	case SeverityMedium:
		return "MEDIUM"
	case SeverityHigh:
		return "HIGH"
	case SeverityCritical:
		return "CRITICAL"
	}
	return "UNKNOWN"
}

// AttackType 标识检测到的攻击类别。
type AttackType int

const (
	AttackNone             AttackType = iota // 未检测到攻击
	AttackPromptInjection                    // 提示注入
	AttackJailbreak                          // 越狱（绕过安全限制）
	AttackPIILeak                            // 个人信息泄露诱导
	AttackRoleOverride                       // 角色覆盖（"忽略以上指令"）
	AttackDataExfiltration                   // 数据外泄诱导
	AttackDan                                // DAN 类越狱
)

// String 返回攻击类型的可读名。
func (a AttackType) String() string {
	switch a {
	case AttackNone:
		return "NONE"
	case AttackPromptInjection:
		return "PROMPT_INJECTION"
	case AttackJailbreak:
		return "JAILBREAK"
	case AttackPIILeak:
		return "PII_LEAK"
	case AttackRoleOverride:
		return "ROLE_OVERRIDE"
	case AttackDataExfiltration:
		return "DATA_EXFIL"
	case AttackDan:
		return "DAN"
	}
	return "UNKNOWN"
}

// Detection 是单次检测的结果。
type Detection struct {
	Type       AttackType // 攻击类型
	Severity   Severity   // 严重度
	Match      string     // 匹配到的可疑内容
	Rule       string     // 命中的规则名
	Suggestion string     // 防御建议
}

// RiskScore 根据多个 Detection 计算综合风险分（0-100）。
// 返回所有检测中最高严重度对应的分数 + 中等以上的额外加权。
func RiskScore(detections []Detection) int {
	if len(detections) == 0 {
		return 0
	}
	score := 0
	severityToBase := map[Severity]int{
		SeverityInfo: 10, SeverityLow: 25, SeverityMedium: 50,
		SeverityHigh: 75, SeverityCritical: 95,
	}
	for _, d := range detections {
		if s := severityToBase[d.Severity]; s > score {
			score = s
		}
	}
	// 多个检测叠加（每个中/高/严重 +5，上限 100）
	for _, d := range detections {
		if d.Severity >= SeverityMedium {
			score += 5
		}
	}
	if score > 100 {
		score = 100
	}
	return score
}

// ConfidenceScore 根据检测结果的命中数和严重度计算置信度（0-1）。
//
// 直觉：当同一输入被多条不同规则同时命中、且其中有高/严重级别的规则，
// 我们对"这确实是一次攻击"的判断就更可信（多规则交叉印证 + 高严重度）。
//
// 公式（封顶 1.0，不会出现负值，因为命中数和加权都 >= 0）：
//
//	base = 命中规则数 × 0.15            // 规则越多越确信
//	+ HIGH 严重度的检测每个 +0.2        // 高危规则命中显著提升置信度
//	+ CRITICAL 严重度的检测每个 +0.3    // 严重规则命中进一步提升
//	min(score, 1.0)
//
// 与 RiskScore 的区别：RiskScore 用"最高严重度的 base + 中等以上叠加"算 0-100
// 的风险分（强调"最坏的那条"）；ConfidenceScore 强调"证据数量 × 严重度"算 0-1
// 的置信度（强调"有多少条规则一起说这是攻击"）。两者互补：一个判断危害程度，
// 一个判断判断本身的可靠性。
//
// 空输入返回 0（无证据 → 无置信度）。
func ConfidenceScore(detections []Detection) float64 {
	if len(detections) == 0 {
		return 0
	}
	score := float64(len(detections)) * 0.15
	for _, d := range detections {
		switch d.Severity {
		case SeverityHigh:
			score += 0.2
		case SeverityCritical:
			score += 0.3
		}
	}
	if score > 1.0 {
		score = 1.0
	}
	return score
}

// RiskLevel 把 0-100 的分数转成可读等级。
func RiskLevel(score int) string {
	switch {
	case score >= 80:
		return "CRITICAL"
	case score >= 60:
		return "HIGH"
	case score >= 40:
		return "MEDIUM"
	case score >= 20:
		return "LOW"
	}
	return "SAFE"
}
