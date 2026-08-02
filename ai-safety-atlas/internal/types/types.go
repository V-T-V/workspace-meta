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
