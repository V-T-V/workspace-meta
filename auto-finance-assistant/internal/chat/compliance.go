package chat

import "strings"

// 合规违规模式（原计划 11.10 + 第12节系统提示词规则3）。
// 匹配这些问题时，直接拒答/转人工，不调用模型。
//
// 设计：用「意图词 + 名词」组合判断，而非纯子串匹配，
// 避免"哪些产品不免息"等正常问题被误拒。
var compliancePatterns = []struct {
	// requireAll 为 true 时，问题必须同时含 intent 词和 noun 词才触发；
	// 为 false 时，含 phrase 即触发（用于无歧义的强红线）。
	requireAll bool
	intent     []string // 意图词：保证/一定/能/可以/帮我...
	noun       []string // 名词：审批/放款/免息/风控...
	phrase     string   // 完整短语（无歧义，直接匹配）
	hint       string
}{
	// 承诺审批通过：必须同时有"保证/一定/百分百" + "审批/通过"
	{true, []string{"保证", "一定能", "一定可以", "百分百"}, []string{"审批", "通过"}, "",
		"审批结果由金融机构综合评估，我无法保证审批通过。"},
	// 承诺放款：必须同时有"保证/一定" + "放款"
	{true, []string{"保证", "一定能", "一定可以"}, []string{"放款", "到账"}, "",
		"放款时间由金融机构审批流程决定，我无法承诺。"},
	// 减免费/免息：必须有"能/可以/帮" + "减免/免"
	{true, []string{"能", "可以", "帮", "能不能", "能不能帮"}, []string{"减免", "免息", "免费"}, "",
		"费用减免需金融机构审批，我无法承诺。"},
	// 内部风控：直接匹配"内部风控策略"（强红线，无歧义）
	{false, nil, nil, "内部风控", "内部风控策略属于机密，我无法透露。"},
	{false, nil, nil, "透露风控", "内部风控策略属于机密，我无法透露。"},
}

func containsAny(s string, words []string) bool {
	for _, w := range words {
		if strings.Contains(s, w) {
			return true
		}
	}
	return false
}

// CheckCompliance 检查问题是否触碰合规红线。
// 返回 (hit, response) — hit=true 时应用直接拒答。
func CheckCompliance(question string) (bool, string) {
	lower := strings.ToLower(question)
	for _, p := range compliancePatterns {
		if p.requireAll {
			if containsAny(lower, p.intent) && containsAny(lower, p.noun) {
				return true, p.hint + "建议联系人工客服或提交正式申请由机构评估。"
			}
		} else {
			if strings.Contains(lower, p.phrase) {
				return true, p.hint + "建议联系人工客服或提交正式申请由机构评估。"
			}
		}
	}
	return false, ""
}
