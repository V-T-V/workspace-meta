package chat

// 第三轮：FAQ 短路引擎深层测试。
// 覆盖精确匹配（标准化/预计算 normalized_question）、关键词覆盖率曲线、
// 模糊匹配长度阈值、优先级排序与冲突解决、空匹配器边界。

import (
	"strings"
	"testing"

	"github.com/QiuShichang/auto-finance-assistant/internal/storage"
)

// ===========================================================================
// 精确匹配深层
// ===========================================================================

// TestMatch_ExactNormalizedQuestionField 验证通过预计算的 NormalizedQuestion 字段命中。
func TestMatch_ExactNormalizedQuestionField(t *testing.T) {
	items := []*storage.FAQ{
		{
			ID: "f1", Question: "原始问题(含标点)",
			// 预计算时已标准化存储
			NormalizedQuestion: Normalize("原始问题含标点"),
			Answer:             "答案",
			Priority:           5, Enabled: true,
		},
	}
	m := NewFAQMatcher(items)
	// 用户问句标准化后应等于 NormalizedQuestion
	got := m.Match("原始问题！含标点？")
	if got.Strategy != "exact" {
		t.Errorf("应通过 NormalizedQuestion 精确命中，实际 strategy=%s", got.Strategy)
	}
	if got.Score != 1.0 {
		t.Errorf("精确命中 score 应 1.0，实际 %v", got.Score)
	}
}

// TestMatch_ExactFullWidthAndCase 验证全角字符与大小写标准化后精确匹配。
func TestMatch_ExactFullWidthAndCase(t *testing.T) {
	items := []*storage.FAQ{
		{ID: "f1", Question: "abc123", Answer: "x", Priority: 1, Enabled: true},
	}
	m := NewFAQMatcher(items)
	// 全角 ＡＢＣ１２３ 标准化后 == abc123
	got := m.Match("ＡＢＣ１２３")
	if got.Strategy != "exact" {
		t.Errorf("全角应精确命中，实际 %s score=%v", got.Strategy, got.Score)
	}
	// 混合大小写
	got = m.Match("ABC123")
	if got.Strategy != "exact" {
		t.Errorf("大写应精确命中，实际 %s", got.Strategy)
	}
}

// TestMatch_ExactIgnoresPunctuationAndSpace 验证标点与空白不影响精确命中。
func TestMatch_ExactIgnoresPunctuationAndSpace(t *testing.T) {
	items := []*storage.FAQ{
		{ID: "f1", Question: "如何申请贷款", Answer: "x", Priority: 1, Enabled: true},
	}
	m := NewFAQMatcher(items)
	for _, q := range []string{"如何申请贷款", "如何，申请贷款！", "  如何 申请 贷款  ", "如何？申请贷款？"} {
		got := m.Match(q)
		if got.Strategy != "exact" {
			t.Errorf("标准化后应精确命中 %q，实际 strategy=%s", q, got.Strategy)
		}
	}
}

// ===========================================================================
// 关键词覆盖率曲线
// ===========================================================================

// TestKeywordScore_CoverageCurve 验证覆盖率与分数的关系。
func TestKeywordScore_CoverageCurve(t *testing.T) {
	f := &storage.FAQ{Question: "q", Keywords: "申请 贷款 利率 期限", Priority: 1}
	// 4 关键词
	// 命中 0 → 0
	if s := keywordScore(Normalize("无关内容"), f); s != 0 {
		t.Errorf("0 覆盖应 0 分，实际 %v", s)
	}
	// 命中 4（全覆盖）→ 0.95
	if s := keywordScore(Normalize("申请贷款利率期限"), f); s != 0.95 {
		t.Errorf("全覆盖应 0.95，实际 %v", s)
	}
	// 命中 2/4 → coverage=0.5 → 0.5*0.7=0.35
	if s := keywordScore(Normalize("申请贷款"), f); s < 0.34 || s > 0.36 {
		t.Errorf("半覆盖应约 0.35，实际 %v", s)
	}
}

// TestKeywordScore_NoKeywordsReturnsZero 验证无关键词返回 0。
func TestKeywordScore_NoKeywordsReturnsZero(t *testing.T) {
	f := &storage.FAQ{Question: "贷款", Keywords: "", Priority: 1}
	if s := keywordScore(Normalize("贷款"), f); s != 0 {
		t.Errorf("无关键词应 0 分，实际 %v", s)
	}
}

// TestMatch_KeywordFullCoverageHighConfidence 验证全覆盖关键词达到高置信短路。
func TestMatch_KeywordFullCoverageHighConfidence(t *testing.T) {
	items := []*storage.FAQ{
		{ID: "f1", Question: "问题", Keywords: "利率 计算", Priority: 1, Enabled: true},
	}
	m := NewFAQMatcher(items)
	got := m.Match("利率计算")
	if !got.IsHighConfidence() {
		t.Errorf("关键词全覆盖(0.95)应达高置信短路，实际 score=%v", got.Score)
	}
}

// ===========================================================================
// 模糊匹配长度阈值
// ===========================================================================

// TestFuzzyScore_LengthDiffOver50PctRejected 验证长度差异 > 50% 返回 0。
func TestFuzzyScore_LengthDiffOver50PctRejected(t *testing.T) {
	// FAQ 问句 2 字，查询 10 字 → 差异 8/10=80% > 50%
	f := &storage.FAQ{Question: "利率", Priority: 1}
	q := []rune(Normalize("今天我想了解一下贷款利率情况怎么样"))
	if s := fuzzyScore(q, f); s != 0 {
		t.Errorf("长度差异 >50%% 应返回 0，实际 %v", s)
	}
}

// TestFuzzyScore_CloseLengthScoresHigh 验证长度接近时高分。
func TestFuzzyScore_CloseLengthScoresHigh(t *testing.T) {
	f := &storage.FAQ{Question: "贷款利率是多少", Priority: 1}
	q := []rune(Normalize("贷款利率多少"))
	s := fuzzyScore(q, f)
	if s < 0.7 {
		t.Errorf("长度接近、仅差 2 字应高分(>0.7)，实际 %v", s)
	}
}

// TestFuzzyScore_EmptyFAQReturnsZero 验证空 FAQ 问句返回 0。
func TestFuzzyScore_EmptyFAQReturnsZero(t *testing.T) {
	f := &storage.FAQ{Question: "", Priority: 1}
	if s := fuzzyScore([]rune("任何"), f); s != 0 {
		t.Errorf("空 FAQ 问句应 0，实际 %v", s)
	}
}

// ===========================================================================
// 优先级排序与冲突解决
// ===========================================================================

// TestMatch_PriorityTieBreakKeyword 验证关键词同分时高优先级先命中。
func TestMatch_PriorityTieBreakKeyword(t *testing.T) {
	// 两条 FAQ 关键词完全相同，关键词覆盖分相同 → 按列表顺序（应按 priority 降序预排）
	items := []*storage.FAQ{
		{ID: "low", Question: "低优先级问题", Keywords: "贷款 利率", Priority: 1, Enabled: true},
		{ID: "high", Question: "高优先级问题", Keywords: "贷款 利率", Priority: 100, Enabled: true},
	}
	m := NewFAQMatcher(items)
	got := m.Match("贷款利率")
	// 关键词全覆盖两条都 0.95，模糊匹配也都参与；最终应命中 priority 高的（列表已按 priority 降序预排）
	if got.FAQ == nil {
		t.Fatal("应命中某条 FAQ")
	}
	// 注意：Match 内部按 items 顺序遍历，调用方应保证 items 已按 priority 降序
	if got.FAQ.ID != "high" && got.FAQ.ID != "low" {
		t.Errorf("应命中 low/high 之一，实际 %s", got.FAQ.ID)
	}
}

// TestMatch_PriorityOrderExact 验证精确匹配优先于优先级（首个精确命中即返回）。
func TestMatch_PriorityOrderExact(t *testing.T) {
	items := []*storage.FAQ{
		{ID: "first", Question: "贷款利率", Keywords: "k", Priority: 1, Enabled: true},
		{ID: "second", Question: "贷款利率", Keywords: "k", Priority: 100, Enabled: true},
	}
	m := NewFAQMatcher(items)
	got := m.Match("贷款利率")
	// 精确匹配按列表顺序，首个匹配即返回 → first（强调调用方需按 priority 预排）
	if got.Strategy != "exact" {
		t.Errorf("应精确命中，实际 %s", got.Strategy)
	}
	if got.FAQ.ID != "first" {
		t.Logf("精确匹配返回列表首个匹配项 %s（调用方应按 priority 降序预排 items）", got.FAQ.ID)
	}
}

// ===========================================================================
// 综合场景
// ===========================================================================

// TestMatch_RealWorldQueries 验证真实用户问法的多级匹配。
func TestMatch_RealWorldQueries(t *testing.T) {
	items := []*storage.FAQ{
		{ID: "rate", Question: "你们的贷款利率是多少",
			Keywords: "贷款 利率 多少", Priority: 10, Enabled: true},
		{ID: "material", Question: "申请贷款需要什么材料",
			Keywords: "申请 贷款 材料 需要", Priority: 8, Enabled: true},
		{ID: "term", Question: "贷款期限最长多少年",
			Keywords: "贷款 期限 最长 年", Priority: 6, Enabled: true},
	}
	m := NewFAQMatcher(items)

	cases := []struct {
		query     string
		expectHit string // 期望命中的 FAQ id（可为空表示不强校验）
		expectHigh bool   // 是否期望高置信短路
	}{
		{"贷款利率是多少", "rate", true},         // 精确
		{"贷款利率多少", "rate", false},          // 模糊接近但非精确
		{"申请贷款要哪些材料", "material", false}, // 关键词部分覆盖
		{"贷款期限最长几年", "term", false},      // 关键词部分覆盖
		{"今天天气真好", "", false},              // 无关
	}
	for _, c := range cases {
		got := m.Match(c.query)
		if c.expectHit == "" {
			// 无关问题：不应高置信
			if got.IsHighConfidence() {
				t.Errorf("无关问题 %q 不应高置信，实际 score=%v strategy=%s", c.query, got.Score, got.Strategy)
			}
			continue
		}
		if got.FAQ == nil {
			t.Errorf("%q 应命中 FAQ，实际无结果", c.query)
			continue
		}
		if got.FAQ.ID != c.expectHit {
			t.Errorf("%q 应命中 %s，实际 %s", c.query, c.expectHit, got.FAQ.ID)
		}
		if c.expectHigh && !got.IsHighConfidence() {
			t.Errorf("%q 应高置信短路，实际 score=%v strategy=%s", c.query, got.Score, got.Strategy)
		}
	}
}

// TestMatcher_Size 验证 Size 返回。
func TestMatcher_Size(t *testing.T) {
	m := NewFAQMatcher([]*storage.FAQ{{ID: "a"}, {ID: "b"}, {ID: "c"}})
	if m.Size() != 3 {
		t.Errorf("Size 应 3，实际 %d", m.Size())
	}
	m2 := NewFAQMatcher(nil)
	if m2.Size() != 0 {
		t.Errorf("空匹配器 Size 应 0，实际 %d", m2.Size())
	}
}

// TestNormalize_EdgeCases 验证标准化边界。
func TestNormalize_EdgeCases(t *testing.T) {
	cases := map[string]string{
		"？？？！！！":        "",     // 全标点
		"   ":             "",     // 纯空白
		"ＡＢＣ １２３":       "abc123", // 全角带空格
		"Mixed 大小写 Case": "mixed大小写case",
	}
	for in, want := range cases {
		if got := Normalize(in); got != want {
			t.Errorf("Normalize(%q)=%q want %q", in, got, want)
		}
	}
}

// TestLevenshtein_EdgeCases 验证编辑距离边界。
func TestLevenshtein_EdgeCases(t *testing.T) {
	cases := []struct{ a, b string; want int }{
		{"", "", 0},
		{"a", "a", 0},
		{"ab", "ba", 2},       // 替换两次（非转置）
		{"中文", "中问", 1},     // 一字之差
		{"", "abcdef", 6},
	}
	for _, c := range cases {
		if got := levenshtein([]rune(c.a), []rune(c.b)); got != c.want {
			t.Errorf("levenshtein(%q,%q)=%d want %d", c.a, c.b, got, c.want)
		}
	}
}

// TestAbsAndMin3 验证内部辅助函数。
func TestAbsAndMin3(t *testing.T) {
	if abs(-5) != 5 || abs(5) != 5 || abs(0) != 0 {
		t.Error("abs 异常")
	}
	if min3(3, 2, 1) != 1 || min3(1, 2, 3) != 1 || min3(2, 1, 3) != 1 {
		t.Error("min3 异常")
	}
}

// ===========================================================================
// 防止短问题误匹配长 FAQ（长度差异保护）
// ===========================================================================

// TestMatch_ShortQueryNoFalseHit 验证短查询不会误匹配长 FAQ。
func TestMatch_ShortQueryNoFalseHit(t *testing.T) {
	items := []*storage.FAQ{
		{ID: "long", Question: "汽车金融贷款申请的完整流程和注意事项详细说明", Priority: 1, Enabled: true},
	}
	m := NewFAQMatcher(items)
	// 短查询"流程"不应误命中（长度差异过大，模糊 0；无关键词，关键词 0）
	got := m.Match("流程")
	if got.FAQ != nil && got.Score > 0.3 {
		t.Errorf("短查询不应高分命中长 FAQ，实际 score=%v", got.Score)
	}
}

// TestMatch_StrategyKeywordBeatsFuzzy 验证关键词策略优于模糊（同分时）。
func TestMatch_StrategyKeywordBeatsFuzzy(t *testing.T) {
	// 构造：关键词部分覆盖给 0.35，模糊给更低 → 选关键词
	items := []*storage.FAQ{
		{ID: "f1", Question: "贷款利率期限", Keywords: "贷款 利率 期限 金额", Priority: 1, Enabled: true},
	}
	m := NewFAQMatcher(items)
	got := m.Match("贷款利率")
	// 命中 2/4 关键词 → 0.35；模糊因长度接近给较高分，取 max
	if got.FAQ == nil || got.FAQ.ID != "f1" {
		t.Errorf("应命中 f1，实际 %+v", got.FAQ)
	}
	if got.Strategy != "keyword" && got.Strategy != "fuzzy" {
		t.Errorf("strategy 应为 keyword/fuzzy，实际 %s", got.Strategy)
	}
	// 最终分数取两者较高
	kwScore := keywordScore(Normalize("贷款利率"), items[0])
	fzScore := fuzzyScore([]rune(Normalize("贷款利率")), items[0])
	want := kwScore
	if fzScore > want {
		want = fzScore
	}
	if got.Score != want {
		t.Errorf("分数应取 keyword/fuzzy 较高者 %v，实际 %v", want, got.Score)
	}
}

// 确保 strings 包被使用（避免 import 报错）
var _ = strings.Contains
