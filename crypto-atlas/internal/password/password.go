// Package password 实现一个教学向的密码强度检查器。
//
// 它不是密码学原语（不像 AES/SHA-256 那样处理密文/摘要），而是
// 把"口令强度评估"这件事拆成可观察的规则：长度、字符种类（大小写/数字/特殊）、
// 常见弱口令黑名单。每命中一项给分，最终汇总成 0-100 的分数和 weak/medium/strong
// 三档级别。
//
// 适用场景：注册改密表单的前端反馈、教学演示"什么样的口令算弱"。
// 不适用场景：**不能**当作口令存储（存储请用 bcrypt/Argon2/scrypt），
// 也**不能**当作安全门槛的唯一依据——强度分数只是启发式估计，不抗针对性攻击。
package password

import (
	"context"
	"fmt"
	"strings"
	"unicode"
)

// Level 是强度级别（三档）。
type Level string

const (
	// LevelWeak 弱口令：分数 < 40。
	LevelWeak Level = "weak"
	// LevelMedium 中等：40 <= 分数 < 80。
	LevelMedium Level = "medium"
	// LevelStrong 强：分数 >= 80。
	LevelStrong Level = "strong"
)

// 常见弱口令黑名单（小写匹配）。
// 来源于历年泄露口令榜单（123456、password、qwerty 等）的典型样本，
// 用于演示"黑名单"这条规则；真实系统应使用更大的泄露口令库（如 Have I Been Pwned）。
var commonPasswords = map[string]bool{
	"123456":     true,
	"123456789":  true,
	"password":   true,
	"12345678":   true,
	"qwerty":     true,
	"111111":     true,
	"1234567":    true,
	"1234567890": true,
	"abc123":     true,
	"000000":     true,
	"iloveyou":   true,
	"1234":       true,
	"admin":      true,
	"letmein":    true,
	"welcome":    true,
	"monkey":     true,
	"dragon":     true,
	"password1":  true,
	"qwerty123":  true,
	"1q2w3e":     true,
}

// StrengthResult 是 CheckStrength 的返回值。
type StrengthResult struct {
	Password string // 被检查的口令（回显，便于 demo 展示）
	Score    int    // 0-100，越高越强
	Level    Level  // weak / medium / strong
	Length   int    // 口令长度（按 rune 计，对中文等非 ASCII 友好）

	// 命中的字符种类（用于解释为什么得这个分）。
	HasLower   bool // 至少 1 个小写字母
	HasUpper   bool // 至少 1 个大写字母
	HasDigit   bool // 至少 1 个数字
	HasSpecial bool // 至少 1 个特殊字符（非字母非数字）
	IsCommon   bool // 命中常见弱口令黑名单

	// Reasons 是评分依据的人类可读说明（每条对应一项加减分）。
	Reasons []string
}

// CheckStrength 评估口令强度，返回 0-100 的分数与 weak/medium/strong 级别。
//
// 评分规则（满分 100，常见弱口令直接判 0）：
//
//   - 长度分（最多 40）：每字符 +4，上限 40
//   - 字符种类（每种 +15，最多 60）：小写、大写、数字、特殊
//   - 常见弱口令黑名单命中：分数直接清零，级别强制 weak
//
// 阈值：Score < 40 → weak；40 <= Score < 80 → medium；Score >= 80 → strong。
func CheckStrength(password string) StrengthResult {
	r := StrengthResult{
		Password: password,
		Length:   len([]rune(password)),
	}

	for _, ch := range password {
		switch {
		case unicode.IsLower(ch):
			r.HasLower = true
		case unicode.IsUpper(ch):
			r.HasUpper = true
		case unicode.IsDigit(ch):
			r.HasDigit = true
		default:
			// 非字母非数字视为特殊字符（含空格、标点、符号等）
			r.HasSpecial = true
		}
	}

	// 长度分：每字符 +4，上限 40
	if r.Length > 0 {
		lenScore := r.Length * 4
		if lenScore > 40 {
			lenScore = 40
		}
		r.Score += lenScore
		r.Reasons = append(r.Reasons, fmt.Sprintf("长度 %d 字符 +%d 分", r.Length, lenScore))
	}

	// 字符种类分：每种 +15
	kinds := 0
	if r.HasLower {
		kinds++
		r.Score += 15
		r.Reasons = append(r.Reasons, "含小写字母 +15 分")
	}
	if r.HasUpper {
		kinds++
		r.Score += 15
		r.Reasons = append(r.Reasons, "含大写字母 +15 分")
	}
	if r.HasDigit {
		kinds++
		r.Score += 15
		r.Reasons = append(r.Reasons, "含数字 +15 分")
	}
	if r.HasSpecial {
		kinds++
		r.Score += 15
		r.Reasons = append(r.Reasons, "含特殊字符 +15 分")
	}
	_ = kinds

	// 常见弱口令：分数清零，级别强制 weak
	if commonPasswords[strings.ToLower(password)] {
		r.IsCommon = true
		r.Score = 0
		r.Level = LevelWeak
		r.Reasons = append([]string{"命中常见弱口令黑名单，分数清零"}, r.Reasons...)
		return r
	}

	// 封顶 100
	if r.Score > 100 {
		r.Score = 100
	}

	r.Level = levelFromScore(r.Score)
	return r
}

// levelFromScore 把分数映射成级别。
func levelFromScore(score int) Level {
	switch {
	case score < 40:
		return LevelWeak
	case score < 80:
		return LevelMedium
	default:
		return LevelStrong
	}
}

// DemoResult 是 demo 输出摘要。
type DemoResult struct {
	Items []DemoItem
}

// DemoItem 是单组口令的强度演示条目。
type DemoItem struct {
	Password string
	Score    int
	Level    Level
	IsCommon bool
}

// Demo 演示对一组典型口令的强度评估：弱口令、中等口令、强口令各一例。
func Demo(ctx context.Context) (*DemoResult, error) {
	_ = ctx
	inputs := []struct {
		pw   string
		note string
	}{
		{"123456", "经典弱口令（命中黑名单 → 0 分）"},
		{"abc123", "短且无大小写/特殊字符（命中黑名单 → 0 分）"},
		{"hello2024", "长度尚可但缺大写/特殊 → 中等"},
		{"Tr0ub4dour&3", "长度 + 四类齐全 → 强"},
		{"xK9#mQ$vL2@pN7!", "长 + 四类齐全 → 强"},
	}
	r := &DemoResult{}
	fmt.Println("=== 密码强度检查器 demo ===")
	fmt.Printf("%-18s %-6s %-8s %s\n", "口令", "分数", "级别", "说明")
	for _, in := range inputs {
		res := CheckStrength(in.pw)
		fmt.Printf("%-18q %-6d %-8s %s\n", in.pw, res.Score, res.Level, in.note)
		r.Items = append(r.Items, DemoItem{
			Password: in.pw,
			Score:    res.Score,
			Level:    res.Level,
			IsCommon: res.IsCommon,
		})
	}
	return r, nil
}
