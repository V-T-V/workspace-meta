// Package chat 实现客服问答编排：脱敏 → 检索（M4）→ 生成 → 落库。
// M1 子集：脱敏（最小版）+ Ollama 生成 + 会话历史落库。
package chat

import (
	"regexp"
	"strings"
)

// PII 脱敏正则（完善版）。
// 覆盖原计划 11.2：手机号（含分隔符）、身份证（18位+15位）、银行卡号、邮箱。
var (
	// 手机号：11位无分隔 1[3-9]+9位，或带 -/空格 的 3-4-4 分组
	rePhone      = regexp.MustCompile(`1[3-9]\d{9}|1[3-9]\d[\s-]\d{4}[\s-]\d{4}`)
	// 18位身份证：首位非0 + 16位 + 末位数字或X
	reIDCard18   = regexp.MustCompile(`[1-9]\d{16}[\dXx]`)
	// 15位旧身份证
	reIDCard15   = regexp.MustCompile(`\b[1-9]\d{14}\b`)
	// 银行卡号：16-19位数字
	reBankCard   = regexp.MustCompile(`\b\d{16,19}\b`)
	// 邮箱
	reEmail      = regexp.MustCompile(`[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`)
)

// MaskPII 对文本中的敏感信息脱敏，用于日志和模型输入前置清洗。
// 保留首尾少量字符以便上下文识别，中间用 * 替换。
// 例：13812345678 → 138****5678，138-1234-5678 → 138****5678
func MaskPII(text string) string {
	if text == "" {
		return text
	}
	// 邮箱
	text = reEmail.ReplaceAllStringFunc(text, func(s string) string {
		at := strings.IndexByte(s, '@')
		if at <= 2 {
			return s
		}
		return s[:2] + "***" + s[at:]
	})
	// 手机号（先匹配，提取数字统一脱敏）
	text = rePhone.ReplaceAllStringFunc(text, func(s string) string {
		var digits []byte
		for i := 0; i < len(s); i++ {
			if s[i] >= '0' && s[i] <= '9' {
				digits = append(digits, s[i])
			}
		}
		if len(digits) != 11 {
			return s
		}
		return string(digits[:3]) + "****" + string(digits[7:])
	})
	// 18位身份证（优先于银行卡，避免误判）
	text = reIDCard18.ReplaceAllStringFunc(text, func(s string) string {
		if len(s) < 8 {
			return s
		}
		return s[:4] + repeatStar(len(s)-8) + s[len(s)-4:]
	})
	// 15位旧身份证
	text = reIDCard15.ReplaceAllStringFunc(text, func(s string) string {
		if len(s) < 8 {
			return s
		}
		return s[:4] + repeatStar(len(s)-8) + s[len(s)-4:]
	})
	// 银行卡号
	text = reBankCard.ReplaceAllStringFunc(text, func(s string) string {
		if len(s) < 8 {
			return s
		}
		return s[:4] + repeatStar(len(s)-8) + s[len(s)-4:]
	})
	return text
}

func repeatStar(n int) string {
	if n <= 0 {
		return ""
	}
	out := make([]byte, n)
	for i := range out {
		out[i] = '*'
	}
	return string(out)
}
