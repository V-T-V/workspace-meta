package chat

import "strings"

// escapeHTML 对模型输出做 HTML 实体转义，防止存储型 XSS。
// 当回答通过前端渲染时，<script> 等标签会被转义为 &lt;script&gt;。
// 保留换行符（\n）和中文，只转义 HTML 特殊字符。
func escapeHTML(s string) string {
	if s == "" {
		return s
	}
	r := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		`"`, "&quot;",
		"'", "&#39;",
	)
	return r.Replace(s)
}

// sanitizeOutput 对模型输出做安全清洗：
// 1. HTML 实体转义（防 XSS）
// 2. 移除零宽字符（防隐藏注入）
func sanitizeOutput(s string) string {
	if s == "" {
		return s
	}
	// 移除零宽字符（U+200B U+200C U+200D U+FEFF）
	s = strings.Map(func(r rune) rune {
		switch r {
		case '\u200B', '\u200C', '\u200D', '\uFEFF':
			return -1 // 丢弃
		}
		return r
	}, s)
	return escapeHTML(s)
}
