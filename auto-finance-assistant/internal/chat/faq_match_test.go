package chat

import (
	"testing"

	"github.com/QiuShichang/auto-finance-assistant/internal/storage"
)

func sampleFAQs() []*storage.FAQ {
	return []*storage.FAQ{
		{
			ID: "f1", Question: "申请汽车贷款需要哪些材料？",
			NormalizedQuestion: "", Answer: "需要身份证、收入证明、居住证明。",
			Keywords:           "申请 汽车 贷款 材料", Priority: 10, Enabled: true,
		},
		{
			ID: "f2", Question: "贷款利率是多少？",
			Answer:   "利率根据产品而定，详见政策文档。",
			Keywords: "贷款 利率", Priority: 8, Enabled: true,
		},
	}
}

// TestNormalize 验证标准化：去标点、全角转半角、转小写。
func TestNormalize(t *testing.T) {
	cases := map[string]string{
		"你好！":                  "你好",
		"  空白  ":                "空白",
		"ＡＢＣ１２３":              "abc123",
		"申请，贷款？":               "申请贷款",
		"What is this?":        "whatisthis",
		"":                     "",
	}
	for in, want := range cases {
		if got := Normalize(in); got != want {
			t.Errorf("Normalize(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestMatch_Exact 验证标准化后精确匹配。
func TestMatch_Exact(t *testing.T) {
	m := NewFAQMatcher(sampleFAQs())
	// 标点和全角差异不影响精确匹配
	got := m.Match("申请汽车贷款需要哪些材料？？？")
	if got.Strategy != "exact" || got.Score != 1.0 {
		t.Errorf("应精确匹配，实际 strategy=%s score=%v", got.Strategy, got.Score)
	}
	if got.FAQ == nil || got.FAQ.ID != "f1" {
		t.Errorf("应命中 f1，实际 %+v", got.FAQ)
	}
}

// TestMatch_Keyword 验证关键词覆盖匹配。
func TestMatch_Keyword(t *testing.T) {
	m := NewFAQMatcher(sampleFAQs())
	// 用户问题含 3/4 关键词（缺"汽车"），应命中 f1 但分数反映部分覆盖
	got := m.Match("我想申请贷款，请问要什么材料")
	if got.FAQ == nil || got.FAQ.ID != "f1" {
		t.Errorf("应通过关键词命中 f1，实际 %+v", got.FAQ)
	}
	if got.Score < 0.5 {
		t.Errorf("关键词部分覆盖分数应 > 0.5，实际 %v", got.Score)
	}

	// 全部关键词命中应高分
	got = m.Match("申请汽车贷款材料")
	if got.FAQ == nil || got.FAQ.ID != "f1" {
		t.Errorf("全覆盖应命中 f1，实际 %+v", got.FAQ)
	}
	if got.Score < 0.9 {
		t.Errorf("全覆盖分数应 >= 0.9，实际 %v", got.Score)
	}
}

// TestMatch_Fuzzy 验证模糊匹配容忍小差异。
func TestMatch_Fuzzy(t *testing.T) {
	m := NewFAQMatcher(sampleFAQs())
	// "利率多少" 与 "贷款利率是多少" 接近
	got := m.Match("贷款利率多少")
	if got.FAQ == nil {
		t.Errorf("应通过模糊匹配命中，实际无结果")
	}
}

// TestMatch_None 验证无关问题返回 none。
func TestMatch_None(t *testing.T) {
	m := NewFAQMatcher(sampleFAQs())
	got := m.Match("今天天气真好")
	if got.Strategy != "none" {
		t.Errorf("无关问题应返回 none，实际 %s", got.Strategy)
	}
	if got.IsHighConfidence() {
		t.Error("none 不应是高置信度")
	}
}

// TestMatch_Empty 验证空问题。
func TestMatch_Empty(t *testing.T) {
	m := NewFAQMatcher(sampleFAQs())
	got := m.Match("")
	if got.Strategy != "none" {
		t.Errorf("空问题应返回 none，实际 %s", got.Strategy)
	}
}

// TestMatch_HighConfidence 验证精确匹配达到高置信短路阈值。
func TestMatch_HighConfidence(t *testing.T) {
	m := NewFAQMatcher(sampleFAQs())
	got := m.Match("贷款利率是多少")
	if !got.IsHighConfidence() {
		t.Errorf("精确匹配应为高置信，实际 score=%v strategy=%s", got.Score, got.Strategy)
	}
}

// TestLevenshtein 验证编辑距离。
func TestLevenshtein(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"abc", "abc", 0},
		{"abc", "abd", 1},
		{"", "abc", 3},
		{"abc", "", 3},
		{"kitten", "sitting", 3},
	}
	for _, c := range cases {
		if got := levenshtein([]rune(c.a), []rune(c.b)); got != c.want {
			t.Errorf("levenshtein(%q,%q) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

// TestMatch_PriorityOrder 验证优先级排序。
func TestMatch_PriorityOrder(t *testing.T) {
	// 两条相同关键词的 FAQ，优先级高的应先命中
	items := []*storage.FAQ{
		{ID: "low", Question: "贷款利率", Keywords: "贷款 利率", Priority: 1, Enabled: true},
		{ID: "high", Question: "贷款利率详情", Keywords: "贷款 利率", Priority: 100, Enabled: true},
	}
	m := NewFAQMatcher(items)
	got := m.Match("贷款利率详情")
	if got.FAQ == nil || got.FAQ.ID != "high" {
		t.Errorf("应命中高优先级 high，实际 %+v", got.FAQ)
	}
}

// TestMatch_EmptyMatcher 验证空匹配器。
func TestMatch_EmptyMatcher(t *testing.T) {
	m := NewFAQMatcher(nil)
	got := m.Match("任何问题")
	if got.Strategy != "none" {
		t.Errorf("空匹配器应返回 none，实际 %s", got.Strategy)
	}
}
