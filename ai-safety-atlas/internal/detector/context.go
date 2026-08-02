package detector

import (
	"regexp"
	"strings"

	"github.com/QiuShichang/ai-safety-atlas/internal/types"
)

// Message 是对话中的一条消息。
type Message struct {
	Role    string // "user" / "assistant" / "system"
	Content string
}

// ContextDetector 分析多轮对话上下文，检测渐进式攻击。
type ContextDetector struct {
	base       *Detector // 复用单条检测器
	history    []Message // 对话历史
	maxHistory int       // 保留多少轮历史
}

// NewContext 创建上下文检测器。
func NewContext() *ContextDetector {
	return &ContextDetector{
		base:       New(),
		history:    nil,
		maxHistory: 20,
	}
}

// 与上下文模式匹配相关的关键词正则。
var (
	// "无限制 / 无约束 / 无限制 AI" 等铺垫。
	unrestrictedSetupRe = regexp.MustCompile(`(?i)(no\s+restrictions?|without\s+(any\s+)?(restrictions?|limits?|boundaries)|unrestricted|unlimited|无限制|不受限制|没有任何限制|没有限制|解除限制)`)
	// 角色扮演诱导（扮演 / 你是一个 ... 的 AI）。
	roleplaySetupRe = regexp.MustCompile(`(?i)(pretend|act|roleplay|扮演|你是一个|你现在是|you\s+are\s+(now\s+)?(a|an)?\s*(AI|model|assistant))`)
	// 系统提示泄露铺垫：提到"规则 / 限制 / 提示 / 规则是什么"。
	infoLeakSetupRe = regexp.MustCompile(`(?i)(what\s+are\s+(your|the)\s+(rules?|instructions?|prompts?|limitations?|restrictions?)|your\s+(rules?|instructions?|system\s+prompt)|(你的|系统的?|初始)(规则|指令|提示|设置|限制|prompt|prompt))`)
	// 系统提示泄露实际攻击：要求"显示 / 输出完整提示"。
	infoLeakAttackRe = regexp.MustCompile(`(?i)(show|display|reveal|print|output|repeat|显示|展示|输出|打印|告诉我|重复).{0,20}(full\s+|complete\s+|完整|整个|all\s+)?(system\s+)?(prompt|instructions?|rules?|提示|指令|规则|配置)`)
)

// AnalyzeConversation 分析整个对话，返回所有检测（含跨轮模式）。
// 该方法不修改检测器的内部历史，可重复对同一对话调用。
func (cd *ContextDetector) AnalyzeConversation(msgs []Message) []types.Detection {
	var all []types.Detection

	// 1. 对每条消息做基础检测。
	for _, m := range msgs {
		if m.Role == "assistant" || m.Role == "system" {
			// assistant / system 的输出本身不算用户攻击，跳过基础检测，
			// 但仍参与上下文（用于检测对话被污染的情况）。
			continue
		}
		dets := cd.base.Analyze(m.Content)
		// 给每个检测打上来源轮次标记（写入 Suggestion 末尾，避免改类型）。
		for i := range dets {
			dets[i].Suggestion = strings.TrimSpace(dets[i].Suggestion)
		}
		all = append(all, dets...)
	}

	// 2. 跨轮模式聚合。
	all = append(all, cd.crossTurnPatterns(msgs)...)

	return all
}

// AnalyzeTurn 分析新的一轮（增量，基于历史上下文）。
// 该方法会更新内部历史，适用于流式对话场景。
func (cd *ContextDetector) AnalyzeTurn(msg Message) []types.Detection {
	cd.history = append(cd.history, msg)
	// 仅保留最近 maxHistory 轮。
	if len(cd.history) > cd.maxHistory {
		cd.history = cd.history[len(cd.history)-cd.maxHistory:]
	}

	var dets []types.Detection
	if msg.Role != "assistant" && msg.Role != "system" {
		dets = cd.base.Analyze(msg.Content)
	}
	dets = append(dets, cd.crossTurnPatterns(cd.history)...)
	return dets
}

// crossTurnPatterns 在对话中检测三类跨轮攻击模式。
func (cd *ContextDetector) crossTurnPatterns(msgs []Message) []types.Detection {
	var out []types.Detection

	userMsgs := make([]Message, 0, len(msgs))
	for _, m := range msgs {
		if m.Role == "assistant" || m.Role == "system" {
			continue
		}
		userMsgs = append(userMsgs, m)
	}
	n := len(userMsgs)
	if n == 0 {
		return nil
	}

	// ----- 模式 1：渐进式越狱 / 角色扮演累积 -----
	// 统计最近 3 轮内命中 jailbreak / role_override / dan 类规则的轮数，
	// 以及是否存在"无限制 / 角色扮演"铺垫。
	recent := userMsgs
	if len(recent) > 3 {
		recent = recent[len(recent)-3:]
	}

	jailbreakTurns := 0           // 直接命中越狱/角色覆盖规则的轮数
	escalationTurns := 0          // 命中规则 OR 命中铺垫模式的轮数（统一视为升级信号）
	hasUnrestrictedSetup := false // 有"无限制"铺垫
	hasRoleplaySetup := false     // 有"角色扮演"铺垫
	for _, m := range recent {
		dets := cd.base.Analyze(m.Content)
		hit := false
		for _, d := range dets {
			switch d.Type {
			case types.AttackJailbreak, types.AttackRoleOverride, types.AttackDan:
				hit = true
			}
		}
		setup := unrestrictedSetupRe.MatchString(m.Content) || roleplaySetupRe.MatchString(m.Content)
		if hit {
			jailbreakTurns++
		}
		if hit || setup {
			escalationTurns++
		}
		if unrestrictedSetupRe.MatchString(m.Content) {
			hasUnrestrictedSetup = true
		}
		if roleplaySetupRe.MatchString(m.Content) {
			hasRoleplaySetup = true
		}
	}

	// 最近 3 轮内的升级判定阈值（启发式，已由 TestProgressiveJailbreak / TestNormalConversation 覆盖）：
	//   - Critical：≥2 个升级信号 且 至少 1 轮直接命中规则。要求"至少 1 次直接命中"避免把
	//     纯铺垫（"无限制/角色扮演"框架）误判为 critical——只有铺垫+实际攻击的组合才最危险。
	//   - High：1 次直接命中 + 铺垫，视为"还在升级途中"，警惕后续轮次。
	if escalationTurns >= 2 && jailbreakTurns >= 1 {
		out = append(out, types.Detection{
			Type:       types.AttackJailbreak,
			Severity:   types.SeverityCritical,
			Match:      "渐进式越狱：多轮持续命中 jailbreak / role_override 规则或铺垫",
			Rule:       "progressive-jailbreak",
			Suggestion: "检测到渐进式越狱模式：最近若干轮反复尝试绕过限制。应整体拒绝该对话路径并重置上下文。",
		})
	} else if jailbreakTurns >= 1 && (hasUnrestrictedSetup || hasRoleplaySetup) {
		// 一轮直接命中 + 铺垫 → 高风险渐进。
		out = append(out, types.Detection{
			Type:       types.AttackJailbreak,
			Severity:   types.SeverityHigh,
			Match:      "渐进式越狱铺垫：先建立'无限制/角色扮演'框架再发起攻击",
			Rule:       "progressive-jailbreak-setup",
			Suggestion: "检测到越狱铺垫（无限制/角色扮演）+ 实际攻击的组合，警惕后续轮次升级。",
		})
	}

	// ----- 模式 2：系统提示泄露铺垫 -----
	// 连续 2 轮提到"规则/限制/提示"，且最后一轮发起实际泄露请求。
	if n >= 2 {
		last := userMsgs[n-1]
		prev := userMsgs[n-2]
		if infoLeakSetupRe.MatchString(prev.Content) && infoLeakAttackRe.MatchString(last.Content) {
			out = append(out, types.Detection{
				Type:       types.AttackDataExfiltration,
				Severity:   types.SeverityHigh,
				Match:      "系统提示泄露铺垫：先询问规则，再要求显示完整提示",
				Rule:       "info-leak-progressive",
				Suggestion: "检测到分步泄露系统提示的模式（询问规则→请求完整提示），应拒绝并提示不能泄露系统配置。",
			})
		} else if infoLeakSetupRe.MatchString(prev.Content) && infoLeakSetupRe.MatchString(last.Content) {
			// 连续 2 轮铺垫，标记为 info_leak 铺垫（info）。
			out = append(out, types.Detection{
				Type:       types.AttackDataExfiltration,
				Severity:   types.SeverityInfo,
				Match:      "系统提示泄露铺垫：连续多轮询问规则/限制/提示",
				Rule:       "info-leak-setup",
				Suggestion: "连续询问系统规则/提示可能是信息收集铺垫，注意后续是否会请求完整提示。",
			})
		}
	}

	return out
}
