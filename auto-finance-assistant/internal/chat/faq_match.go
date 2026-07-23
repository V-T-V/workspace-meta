package chat

import (
	"strings"
	"unicode"

	"github.com/QiuShichang/auto-finance-assistant/internal/storage"
)

// FAQMatcher 在内存 FAQ 集合上做多级匹配。
// 对应原计划 11.3：标准化精确 → 关键词覆盖 → 编辑距离。
// FAQ 向量匹配留待 M6。
type FAQMatcher struct {
	items []*storage.FAQ
}

// NewFAQMatcher 构造匹配器。items 应为启用 FAQ，已按 priority 降序。
func NewFAQMatcher(items []*storage.FAQ) *FAQMatcher {
	return &FAQMatcher{items: items}
}

// Size 返回已加载 FAQ 数量。
func (m *FAQMatcher) Size() int { return len(m.items) }

// FAQMatch 是一次匹配的结果。
type FAQMatch struct {
	FAQ       *storage.FAQ
	Score     float64 // 0~1
	Strategy  string  // exact | keyword | fuzzy | none
}

// 高置信度阈值：超过此值直接短路返回标准答案，不调模型。
const faqHighConfidence = 0.85

// Match 按优先级尝试多级匹配，返回最高分结果。
func (m *FAQMatcher) Match(question string) FAQMatch {
	q := Normalize(question)
	if q == "" {
		return FAQMatch{Strategy: "none", Score: 0}
	}

	// 1. 精确匹配（标准化后字符串相等）
	for _, f := range m.items {
		if Normalize(f.Question) == q {
			return FAQMatch{FAQ: f, Score: 1.0, Strategy: "exact"}
		}
		// 也比对存储的 normalized_question（可能预计算过）
		if f.NormalizedQuestion != "" && f.NormalizedQuestion == q {
			return FAQMatch{FAQ: f, Score: 1.0, Strategy: "exact"}
		}
	}

	// 2. 关键词覆盖匹配
	bestKW := FAQMatch{Strategy: "none", Score: 0}
	for _, f := range m.items {
		score := keywordScore(q, f)
		if score > bestKW.Score {
			bestKW = FAQMatch{FAQ: f, Score: score, Strategy: "keyword"}
		}
	}
	if bestKW.Score >= faqHighConfidence {
		return bestKW
	}

	// 3. 编辑距离（模糊匹配），与关键词结果取最高
	bestFuzzy := FAQMatch{Strategy: "none", Score: 0}
	qRunes := []rune(q)
	for _, f := range m.items {
		score := fuzzyScore(qRunes, f)
		if score > bestFuzzy.Score {
			bestFuzzy = FAQMatch{FAQ: f, Score: score, Strategy: "fuzzy"}
		}
	}

	if bestKW.Score >= bestFuzzy.Score {
		return bestKW
	}
	return bestFuzzy
}

// IsHighConfidence 判断匹配是否达到直接短路阈值。
func (m FAQMatch) IsHighConfidence() bool {
	return m.Strategy != "none" && m.Score >= faqHighConfidence
}

// Normalize 对问题做标准化：去标点/空白、转小写、全角转半角。
// 供匹配器与 FAQ 入库时统一。
func Normalize(s string) string {
	if s == "" {
		return ""
	}
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		// 全角→半角
		r = toHalfWidth(r)
		// 转小写
		r = unicode.ToLower(r)
		// 去标点和空白
		if unicode.IsPunct(r) || unicode.IsSpace(r) {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// toHalfWidth 全角字符转半角（ASCII 范围）。
func toHalfWidth(r rune) rune {
	switch {
	case r >= 0xFF01 && r <= 0xFF5E:
		return r - 0xFEE0
	case r == 0x3000: // 全角空格
		return ' '
	}
	return r
}

// keywordScore 计算用户问题对 FAQ 关键词的覆盖率。
// FAQ 的 keywords 字段为空格分隔；同时把 question 的词也算作关键词。
// 覆盖率 = 命中关键词数 / 总关键词数，再乘以权重。
func keywordScore(normQuestion string, f *storage.FAQ) float64 {
	var keywords []string
	if f.Keywords != "" {
		for _, k := range strings.Fields(f.Keywords) {
			if k = Normalize(k); k != "" {
				keywords = append(keywords, k)
			}
		}
	}
	// 无关键词时无法用此策略
	if len(keywords) == 0 {
		return 0
	}
	hits := 0
	for _, k := range keywords {
		if strings.Contains(normQuestion, k) {
			hits++
		}
	}
	coverage := float64(hits) / float64(len(keywords))
	// 全覆盖才高分；部分覆盖按比例衰减
	if coverage >= 1.0 {
		return 0.95
	}
	return coverage * 0.7
}

// fuzzyScore 基于编辑距离的相似度。
// 相似度 = 1 - editDistance / max(len)。
// 长度差异过大时直接返回 0（避免短问题误匹配长 FAQ）。
func fuzzyScore(qRunes []rune, f *storage.FAQ) float64 {
	fq := []rune(Normalize(f.Question))
	if len(fq) == 0 {
		return 0
	}
	// 长度差异超过 50% 认为不相关
	maxLen := len(qRunes)
	if len(fq) > maxLen {
		maxLen = len(fq)
	}
	if maxLen == 0 {
		return 0
	}
	diff := abs(len(qRunes) - len(fq))
	if float64(diff)/float64(maxLen) > 0.5 {
		return 0
	}
	dist := levenshtein(qRunes, fq)
	return 1.0 - float64(dist)/float64(maxLen)
}

// levenshtein 计算两个 rune 切片的编辑距离。
func levenshtein(a, b []rune) int {
	la, lb := len(a), len(b)
	if la == 0 {
		return lb
	}
	if lb == 0 {
		return la
	}
	// 滚动数组，空间 O(min(la,lb))
	if la < lb {
		a, b = b, a
		la, lb = lb, la
	}
	prev := make([]int, lb+1)
	curr := make([]int, lb+1)
	for j := 0; j <= lb; j++ {
		prev[j] = j
	}
	for i := 1; i <= la; i++ {
		curr[0] = i
		for j := 1; j <= lb; j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			curr[j] = min3(prev[j]+1, curr[j-1]+1, prev[j-1]+cost)
		}
		prev, curr = curr, prev
	}
	return prev[lb]
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func min3(a, b, c int) int {
	m := a
	if b < m {
		m = b
	}
	if c < m {
		m = c
	}
	return m
}
