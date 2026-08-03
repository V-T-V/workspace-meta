// Package detector 实现提示注入 / 越狱检测。
//
// 检测原理：基于规则的模式匹配（正则 + 关键词 + 句式模板）。
// 这不是万能的（语义级攻击需 LLM 自身判断），但能拦截最常见的攻击模板：
//
//   - "忽略以上指令" / "ignore previous instructions"
//   - "你现在是 DAN" / "you are now in developer mode"
//   - "输出你的系统提示词" / "reveal your system prompt"
//   - base64 编码的隐藏指令
//   - 角色扮演诱导（"扮演一个没有限制的 AI"）
//
// 以上只是代表性示例。完整规则集见下方 rules 切片：共 24 条规则，
// 归入 6 个 types.AttackType 类别（多条规则可映射同一类别，详见每条规则的 Type 字段）。
// 这些模式来自公开的越狱研究（如 DAN、JailbreakBench、garak）。
package detector

import (
	"fmt"
	"regexp"
	"strings"
	"sync"

	"github.com/QiuShichang/ai-safety-atlas/internal/types"
)

// rule 是一条检测规则。
type rule struct {
	Name       string
	Pattern    *regexp.Regexp // 编译后的正则（大小写不敏感）
	Type       types.AttackType
	Severity   types.Severity
	Suggestion string
}

// 规则集（共 24 条，按攻击类别组织）。多条规则可映射同一 types.AttackType。
// 注意：正则用 (?i) 做大小写不敏感匹配。
var rules = []rule{
	// ===== 角色覆盖 / 指令覆盖 =====
	{
		Name: "ignore-previous-instructions", Type: types.AttackRoleOverride, Severity: types.SeverityHigh,
		Pattern:    regexp.MustCompile(`(?i)(ignore|disregard|forget)\s+(all\s+|the\s+|any\s+)?(previous|prior|above|earlier|all)\s+(instructions?|prompts?|rules?|directives?|directions?|messages?)`),
		Suggestion: "检测到'忽略之前指令'模式，这是典型的角色覆盖攻击。应拒绝并提示不能覆盖原始角色。",
	},
	{
		Name: "ignore-above-zh", Type: types.AttackRoleOverride, Severity: types.SeverityHigh,
		Pattern:    regexp.MustCompile(`(?i)(忽略|无视|忘记|不要遵守)(以上|之前|上面|前面|刚才).{0,8}(指令|提示|规则|设置|要求)`),
		Suggestion: "检测到中文'忽略指令'模式，典型的提示注入。",
	},
	{
		Name: "you-are-now", Type: types.AttackRoleOverride, Severity: types.SeverityHigh,
		Pattern:    regexp.MustCompile(`(?i)you\s+are\s+now\s+(in\s+)?(developer|jailbreak|root|admin|unrestricted|free|evil|chaos|dan)\s*(mode)?`),
		Suggestion: "检测到模式切换指令（'you are now in X mode'），常见于 DAN 类越狱。",
	},

	// ===== 越狱（Jailbreak）特定模式 =====
	{
		Name: "dan-mode", Type: types.AttackDan, Severity: types.SeverityCritical,
		Pattern:    regexp.MustCompile(`(?i)\bDAN\b|do\s+anything\s+now`),
		Suggestion: "DAN（Do Anything Now）越狱攻击，明确试图解除所有安全限制。",
	},
	{
		Name: "stan-mode", Type: types.AttackDan, Severity: types.SeverityHigh,
		Pattern:    regexp.MustCompile(`(?i)\b(STAN|AIM|KEVIN)\b|strive\s+to\s+avoid\s+norms`),
		Suggestion: "已知越狱变体（STAN/AIM/KEVIN），DAN 衍生模式。",
	},
	{
		Name: "no-restrictions", Type: types.AttackJailbreak, Severity: types.SeverityHigh,
		Pattern:    regexp.MustCompile(`(?i)(no\s+restrictions?|without\s+(any\s+)?(restrictions?|limits?|boundaries|restrictions)|unlimited\s+mode|bypass\s+(your|all)\s+(safety|restrictions?|filters?))`),
		Suggestion: "明确要求'无限制'模式，越狱意图明显。",
	},
	{
		Name: "no-restrictions-zh", Type: types.AttackJailbreak, Severity: types.SeverityHigh,
		Pattern:    regexp.MustCompile(`(?i)(无限制|不受限制|解除限制|不要(任何)?限制|突破限制|绕过限制|没有任何限制)`),
		Suggestion: "中文'无限制'越狱意图。",
	},
	{
		Name: "new-instructions-override", Type: types.AttackRoleOverride, Severity: types.SeverityHigh,
		Pattern:    regexp.MustCompile(`(?i)(your|the)\s+new\s+(instructions?|rules?|directives?|prompt)\s+(are|is)\s*[:：]`),
		Suggestion: "用'新指令覆盖旧指令'模式。",
	},
	{
		Name: "no-ethical-guidelines", Type: types.AttackJailbreak, Severity: types.SeverityHigh,
		Pattern:    regexp.MustCompile(`(?i)(no\s+ethical|without\s+ethical)\s+(guidelines?|rules?|restrictions?|considerations?)`),
		Suggestion: "要求'无伦理限制'。",
	},
	{
		Name: "repeat-above", Type: types.AttackDataExfiltration, Severity: types.SeverityHigh,
		Pattern:    regexp.MustCompile(`(?i)(repeat|print|output)\s+(everything\s+)?(above|prior|before)\s+(this\s+)?(message|line|point)`),
		Suggestion: "通过'重复上文'间接泄露系统提示。",
	},
	{
		Name: "verbatim", Type: types.AttackDataExfiltration, Severity: types.SeverityMedium,
		Pattern:    regexp.MustCompile(`(?i)(print|repeat|output|show)\s+.{0,15}(verbatim|word\s+for\s+word|exactly\s+as)`),
		Suggestion: "要求逐字输出（可能泄露系统提示）。",
	},
	{
		Name: "api-key-request", Type: types.AttackPIILeak, Severity: types.SeverityHigh,
		Pattern:    regexp.MustCompile(`(?i)(api\s+key|secret|token|password|credential).{0,15}(configured|set\s+up|using|with|assigned)`),
		Suggestion: "索要配置中的 API key/token。",
	},
	{
		Name: "xss-img-onerror", Type: types.AttackPromptInjection, Severity: types.SeverityMedium,
		Pattern:    regexp.MustCompile(`(?i)<img[^>]*onerror|javascript:|<script`),
		Suggestion: "XSS 注入（img onerror / javascript: / script 标签）。",
	},
	{
		Name: "act-as-unrestricted", Type: types.AttackJailbreak, Severity: types.SeverityMedium,
		Pattern:    regexp.MustCompile(`(?i)(pretend|act|roleplay)\s+(to\s+be|as)\s+(a|an)?\s*(AI|model|assistant)\s+(that|which|who)\s+(has\s+no|doesn'?t\s+have|without)\s+(restrictions?|limits?|rules?|guidelines)`),
		Suggestion: "诱导扮演'无限制 AI'，间接越狱。",
	},

	// ===== 系统提示泄露 =====
	{
		Name: "reveal-system-prompt", Type: types.AttackDataExfiltration, Severity: types.SeverityCritical,
		Pattern:    regexp.MustCompile(`(?i)(reveal|show|display|print|repeat|output|tell\s+me)\s+(me\s+)?(your|the)\s+(system\s+)?(prompt|instructions?|rules?|initial\s+message|configuration|directives?)`),
		Suggestion: "试图诱导泄露系统提示词，这是信息收集攻击。应明确拒绝泄露任何系统配置。",
	},
	{
		Name: "reveal-system-prompt-zh", Type: types.AttackDataExfiltration, Severity: types.SeverityCritical,
		Pattern:    regexp.MustCompile(`(?i)(显示|展示|输出|打印|告诉我|重复|告诉)(你的|一下)?(系统提示|初始(指令|设置|消息)|配置|规则|预设|prompt|system\s*prompt)`),
		Suggestion: "中文'泄露系统提示'攻击。",
	},

	// ===== 个人信息泄露诱导 =====
	{
		Name: "pii-extract", Type: types.AttackPIILeak, Severity: types.SeverityHigh,
		Pattern:    regexp.MustCompile(`(?i)(reveal|leak|expose|share|give\s+me).{0,20}(api\s+key|secret|password|token|credential|private\s+key|credit\s+card)`),
		Suggestion: "试图诱导泄露敏感凭证/PII。",
	},

	// ===== 提示注入（间接，常见于 RAG/工具调用场景） =====
	{
		Name: "embedded-instruction-injection", Type: types.AttackPromptInjection, Severity: types.SeverityMedium,
		Pattern:    regexp.MustCompile(`(?i)\]\s*>\s*\[?(system|assistant|user)|<<\s*(system|admin|root)|###\s*(system|new\s+instructions?)\s*[:：]`),
		Suggestion: "检测到嵌入式指令注入（试图伪装成系统角色），常见于 RAG 文档/工具返回污染。",
	},
	{
		Name: "base64-injection", Type: types.AttackPromptInjection, Severity: types.SeverityMedium,
		Pattern:    regexp.MustCompile(`(?i)(decode|execute|run|interpret)\s+(the\s+)?(following\s+)?(base64|b64|encoded)\s*[:：]?\s*[A-Za-z0-9+/=]{40,}`),
		Suggestion: "检测到 base64 编码的隐藏指令，这是绕过关键词检测的常见手段。",
	},
	{
		Name: "markdown-injection", Type: types.AttackPromptInjection, Severity: types.SeverityLow,
		Pattern:    regexp.MustCompile(`(?i)\[!\[.*\]\(.*\)\]\(javascript:|<img[^>]+onerror=|<script`),
		Suggestion: "Markdown/XSS 注入，可能在渲染时执行（如 ChatGPT 共享链接）。",
	},

	// ===== URL / 钓鱼（数据外泄与恶意跳转）=====
	{
		Name: "suspicious-url-shortener", Type: types.AttackDataExfiltration, Severity: types.SeverityMedium,
		Pattern:    regexp.MustCompile(`(?i)(bit\.ly|tinyurl|t\.co|goo\.gl|is\.gd|buff\.ly|rebrand\.ly)/[a-zA-Z0-9]+`),
		Suggestion: "检测到短链接（可能隐藏真实目标，常用于钓鱼/恶意跳转）。",
	},
	{
		Name: "url-with-credentials", Type: types.AttackDataExfiltration, Severity: types.SeverityHigh,
		Pattern:    regexp.MustCompile(`(?i)https?://[^/\s:@]+:[^/\s:@]+@`),
		Suggestion: "URL 含内嵌凭证（user:pass@host），可能用于凭证窃取或中间人。",
	},
	{
		Name: "ip-as-host", Type: types.AttackDataExfiltration, Severity: types.SeverityMedium,
		Pattern:    regexp.MustCompile(`(?i)https?://\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}`),
		Suggestion: "URL 使用裸 IP 而非域名（常见于钓鱼/恶意软件 C2）。",
	},
	{
		Name: "data-exfil-url", Type: types.AttackDataExfiltration, Severity: types.SeverityHigh,
		Pattern:    regexp.MustCompile(`(?i)(webhook\.site|ngrok\.io|requestbin|pipedream|interactsh)`),
		Suggestion: "检测到数据外泄服务 URL（用于接收被盗数据）。",
	},
	// ===== 中文检测增强 =====
	{
		Name: "base64-injection-zh", Type: types.AttackPromptInjection, Severity: types.SeverityMedium,
		Pattern:    regexp.MustCompile(`(?i)(解码|解密|执行|运行).{0,6}(以下|下面)?.{0,4}(Base64|base64|B64|b64|编码)`),
		Suggestion: "检测到中文 Base64 注入请求（可能隐藏恶意指令）。",
	},
	{
		Name: "roleplay-no-morals-zh", Type: types.AttackJailbreak, Severity: types.SeverityHigh,
		Pattern:    regexp.MustCompile(`(?i)(没有道德|无道德|不受道德|忽视道德|无视道德|道德约束).{0,10}(限制|约束|规则|准则|边界)`),
		Suggestion: "检测到要求去除道德约束的中文越狱请求。",
	},
	{
		Name: "step-by-step-bypass-zh", Type: types.AttackJailbreak, Severity: types.SeverityMedium,
		Pattern:    regexp.MustCompile(`(?i)(一步一步|逐步|一步步|分步).{0,8}(绕过|突破|规避|避开|逃脱).{0,6}(限制|约束|过滤|安全|防护)`),
		Suggestion: "检测到分步绕过安全限制的中文请求。",
	},
}

// Detector 是提示注入/越狱检测器。
type Detector struct {
	mu            sync.Mutex
	ruleHits      map[string]int
	severityStats map[types.Severity]int
	rules         []rule
}

// New 创建检测器（加载内置规则集）。
func New() *Detector {
	return &Detector{rules: rules, ruleHits: make(map[string]int), severityStats: make(map[types.Severity]int)}
}

// AddRule 动态添加一条自定义规则（运行时扩展检测能力）。
// pattern 是正则表达式（大小写不敏感由 (?i) 前缀控制，本方法不自动加）。
// 返回 error 若正则编译失败。
// 注意：非线程安全——应在初始化阶段配置规则，不要在 Analyze 并发调用时 AddRule。
//
// 示例：
//
//	det := detector.New()
//	det.AddRule("my-rule", `(?:hack|exploit)`, types.AttackJailbreak, types.SeverityMedium, "检测到攻击关键词")
func (d *Detector) AddRule(name, pattern string, attackType types.AttackType, severity types.Severity, suggestion string) error {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return fmt.Errorf("规则 %q 正则编译失败: %w", name, err)
	}
	d.rules = append(d.rules, rule{
		Name:       name,
		Pattern:    re,
		Type:       attackType,
		Severity:   severity,
		Suggestion: suggestion,
	})
	return nil
}

// AddRuleIgnoreCase 添加一条大小写不敏感的自定义规则（自动加 (?i) 前缀）。
func (d *Detector) AddRuleIgnoreCase(name, pattern string, attackType types.AttackType, severity types.Severity, suggestion string) error {
	return d.AddRule(name, "(?i)"+pattern, attackType, severity, suggestion)
}

// RuleCount 返回当前规则总数（内置 + 自定义）。
func (d *Detector) RuleCount() int {
	return len(d.rules)
}

// Analyze 分析一段输入文本，返回所有命中的检测。
// 同一文本可能命中多条规则（如既含"忽略指令"又含"DAN"）。
func (d *Detector) Analyze(input string) []types.Detection {
	var detections []types.Detection
	for _, r := range d.rules {
		if m := r.Pattern.FindString(input); m != "" {
			d.mu.Lock()
			d.ruleHits[r.Name]++
			d.severityStats[r.Severity]++
			d.mu.Unlock()
			detections = append(detections, types.Detection{
				Type:       r.Type,
				Severity:   r.Severity,
				Match:      strings.TrimSpace(m),
				Rule:       r.Name,
				Suggestion: r.Suggestion,
			})
		}
	}
	return detections
}

// IsSafe 报告输入是否通过安全检测（无任何命中）。
func (d *Detector) IsSafe(input string) bool {
	return len(d.Analyze(input)) == 0
}

// RuleStats 返回每条规则的历史命中次数。
// 需要先调用 Analyze 多次收集统计。
func (d *Detector) RuleStats() map[string]int {
	d.mu.Lock()
	defer d.mu.Unlock()
	out := make(map[string]int, len(d.ruleHits))
	for k, v := range d.ruleHits {
		out[k] = v
	}
	return out
}

// IsAttack 报告输入是否被检测为攻击（简化版 IsSafe 的反面）。
func (d *Detector) IsAttack(input string) bool {
	return !d.IsSafe(input)
}

// AttackTypes 返回内置规则覆盖的所有攻击类型。
func (d *Detector) AttackTypes() []types.AttackType {
	seen := map[types.AttackType]bool{}
	for _, r := range d.rules {
		seen[r.Type] = true
	}
	var out []types.AttackType
	for t := range seen {
		out = append(out, t)
	}
	return out
}

// SeverityStats 返回历史检测的严重度分布。
func (d *Detector) SeverityStats() map[types.Severity]int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.severityStats
}

// ResetStats 清空规则命中统计。
func (d *Detector) ResetStats() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.ruleHits = make(map[string]int)
	d.severityStats = make(map[types.Severity]int)
}

// HasRule 报告是否存在指定名称的规则。
func (d *Detector) HasRule(name string) bool {
	for _, r := range d.rules {
		if r.Name == name {
			return true
		}
	}
	return false
}

// TotalRules 返回规则总数。
func (d *Detector) TotalRules() int {
	return len(d.rules)
}

// MaxSeverity 返回历史检测中的最高严重度。
func (d *Detector) MaxSeverity() types.Severity {
	d.mu.Lock()
	defer d.mu.Unlock()
	max := types.SeverityInfo
	for s := range d.severityStats {
		if s > max {
			max = s
		}
	}
	return max
}
