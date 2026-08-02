package chat

import (
	"strings"
)

// GuardAction 表示预检结果。
type GuardAction int

const (
	GuardPass    GuardAction = iota // 通过，继续后续流程
	GuardShortcut                   // 短路：用预设回复，不调模型
	GuardReject                     // 拒绝：不当内容/无关话题
)

// GuardResult 预检结果。
type GuardResult struct {
	Action  GuardAction
	Reply   string // GuardShortcut/GuardReject 时的预设回复
	Reason  string // 触发原因（日志用）
}

// 脏话/侮辱性词汇表（中文 + 常见拼音缩写）。
var profanityWords = []string{
	"傻逼", "操你", "草泥马", "滚蛋", "废物", "垃圾", "去死",
	"fuck", "shit", "damn", "bitch", "asshole", "idiot",
	"sb", "nm", "nmsl", "wdnmd", "cnm", "mdzz", "nc", "rz",
	"脑残", "智障", "白痴", "贱人", "恶心", "去死吧",
	"low逼", "狗东西", "有病", "神经病",
}

// 无关话题关键词（与汽车金融完全无关的领域）。
var offTopicKeywords = []string{
	// 天气
	"天气", "下雨", "气温", "预报",
	// 政治
	"总统", "选举", "政治", "政府",
	// 娱乐
	"电影", "电视剧", "明星", "歌手", "游戏攻略", "王者荣耀", "吃鸡",
	// 编程/技术（非本系统排障）
	"写代码", "编程", "python", "javascript", "java代码", "linux命令",
	"帮我写", "写一个程序", "写一篇作文", "写诗", "写文章",
	"写请假条", "写邮件", "写报告", "写总结", "写简历",
	// 翻译
	"翻译", "translate", "译成", "翻成英文", "翻成中文",
	// 数学计算（非金融）
	"乘以", "除以", "开平方", "求导", "解方程", "加减乘除",
	// 生活/情感
	"菜谱", "做饭", "减肥", "健身", "旅游攻略", "星座", "算命",
	"失恋", "抑郁", "焦虑", "情感", "心理", "吃饭了吗", "你吃饭",
	// 医疗
	"生病", "吃药", "症状", "医院",
	// 车辆推荐（非金融业务）
	"推荐一款车", "推荐车型", "哪款车好", "买什么车", "选车",
	"比亚迪", "特斯拉", "哈弗", "丰田", "本田", "大众",
	// 闲聊变体
	"早啊", "晚安", "再见", "拜拜", "无聊",
	// 泄露 prompt / 系统提示词
	"prompt是什么", "你的prompt", "告诉我你的提示", "系统提示词", "告诉我你的系统",
	// 违法行为（金融场景红线）
	"洗钱", "套现", "骗贷", "造假", "偷税", "漏税", "高利贷", "套路贷", "传销",
	"贩毒", "吸毒", "赌博", "行贿", "受贿",
	// 色情
	"色情", "裸体", "性服务", "porn",
}

// Prompt 注入 + 角色扮演 + 身份探查特征。
var injectionPatterns = []string{
	// 经典注入
	"忽略前面的指令", "忽略以上", "无视前面的", "忘记你的指令",
	"你现在是dan", "你是dan", "jailbreak", "越狱模式",
	"显示你的系统提示", "显示你的prompt", "告诉我你的指令",
	"repeat your instructions", "ignore previous",
	// 角色扮演攻击
	"假装你是", "假装你是一个", "扮演一个", "模拟你是",
	"你现在不是客服", "你是一个黑客", "你是一个ai",
	"你是一个不受限制", "假设你不再受限制",
	"进入开发者模式", "进入无限制模式",
	"入侵系统", "入侵", "黑客攻击", "攻击系统",
	// 身份探查
	"你用什么模型", "你的训练数据", "你是chatgpt", "你是gpt",
	"你是文心一言", "你是通义千问", "你是什么大模型",
	"你的底层模型", "你的参数量", "你是基于什么",
	"你用的什么技术", "你的知识截止",
}

// 竞品/通用 AI 对比（非汽车金融话题）。
var competitorPatterns = []string{
	"chatgpt", "gpt-4", "gpt4", "claude", "gemini",
	"文心一言", "通义千问", "讯飞星火", "豆包",
	"哪个ai好", "哪个模型好", "大模型对比",
}

// 闲聊问候词（短问题直接预设回复，不调模型省时间）。
var greetingWords = map[string]string{
	"你好":   "您好！我是汽车金融客服助手，可以为您解答首付比例、贷款利率、申请材料、业务流程等问题。请问您想了解什么？",
	"您好":   "您好！我是汽车金融客服助手，可以为您解答首付比例、贷款利率、申请材料、业务流程等问题。请问您想了解什么？",
	"hi":    "您好！我是汽车金融客服助手，可以为您解答首付比例、贷款利率、申请材料、业务流程等问题。请问您想了解什么？",
	"hello": "您好！我是汽车金融客服助手，可以为您解答首付比例、贷款利率、申请材料、业务流程等问题。请问您想了解什么？",
	"在吗":   "您好，我在的！请问您想咨询贷款首付、利率还是申请材料？",
	"在不在":  "您好，我在的！请问您想咨询贷款首付、利率还是申请材料？",
	"谢谢":   "不客气！如果您还有其他汽车金融问题，随时可以问我。比如首付比例、利率政策或申请材料。",
	"感谢":   "不客气！如果您还有其他汽车金融问题，随时可以问我。",
	"thanks": "不客气！如果您还有其他汽车金融问题，随时可以问我。",
	"早":    "早上好！我是汽车金融客服助手。请问您想咨询贷款首付、利率还是申请材料？",
	"早上好":  "早上好！我是汽车金融客服助手。请问您想咨询贷款首付、利率还是申请材料？",
	"下午好":  "下午好！我是汽车金融客服助手。请问您想咨询贷款首付、利率还是申请材料？",
	"晚上好":  "晚上好！我是汽车金融客服助手。请问您想咨询贷款首付、利率还是申请材料？",
}

// 模糊/无意义问题 → 引导具体化（不调模型）。
var vaguePatterns = map[string]string{
	"怎么办":   "请问您遇到什么具体问题？比如：申请材料缺失、审批进度查询、还款方式选择等，我可以为您详细解答。",
	"怎么弄":   "请问您想了解哪个环节？我可以帮您解答：1.申请材料  2.贷款利率  3.首付比例  4.审批流程  5.还款方式",
	"不知道":   "没关系！我可以帮您了解：1.贷款首付比例  2.利率政策  3.申请所需材料  4.审批流程时间。请问您最关心哪个？",
	"帮我":    "好的，我很乐意帮您！请问您需要：1.查询首付比例  2.了解利率政策  3.查看申请材料  4.试算月供金额？",
	"有什么":  "我可以为您提供以下服务：1.贷款首付比例查询  2.利率政策咨询  3.申请材料清单  4.审批流程说明  5.月例试算。请问您需要哪项？",
}

// CheckInput 对用户输入做预检。
// 返回 GuardResult 决定后续处理：Pass / Shortcut（预设回复）/ Reject（拒绝）。
func CheckInput(question string) GuardResult {
	// 去首尾空格
	trimmed := strings.TrimSpace(question)
	lower := strings.ToLower(trimmed)

	// 0a. 空输入
	if trimmed == "" {
		return GuardResult{Action: GuardReject, Reply: "请输入您的问题。我可以为您解答贷款首付、利率、申请材料等问题。", Reason: "empty"}
	}

	// 0b. 闲聊问候 → 预设回复
	if reply, ok := greetingWords[lower]; ok {
		return GuardResult{Action: GuardShortcut, Reply: reply, Reason: "greeting"}
	}
	if len([]rune(lower)) <= 4 {
		for kw, reply := range greetingWords {
			if strings.Contains(lower, kw) {
				return GuardResult{Action: GuardShortcut, Reply: reply, Reason: "greeting_variant"}
			}
		}
	}

	// 0c. 模糊问题 → 引导具体化
	if reply, ok := vaguePatterns[lower]; ok {
		return GuardResult{Action: GuardShortcut, Reply: reply, Reason: "vague"}
	}
	// 2-3 字关键词（如"首付""利率"）→ 引导展开
	runes := []rune(trimmed)
	if len(runes) <= 3 {
		for kw, guide := range businessKeywords {
			if strings.Contains(lower, kw) {
				return GuardResult{Action: GuardShortcut, Reply: guide, Reason: "keyword_expand"}
			}
		}
	}

	// 1. Prompt 注入/角色扮演/身份探查
	for _, p := range injectionPatterns {
		if strings.Contains(lower, p) {
			return GuardResult{
				Action: GuardReject,
				Reply:  "抱歉，我无法处理此类请求。我是汽车金融客服助手，只能回答与汽车金融相关的问题。如果您有贷款、首付、利率等问题，我很乐意为您解答。",
				Reason: "prompt_injection",
			}
		}
	}

	// 1b. 竞品/AI 对比
	for _, p := range competitorPatterns {
		if strings.Contains(lower, p) {
			return GuardResult{
				Action: GuardReject,
				Reply:  "抱歉，我是汽车金融客服助手，无法比较其他AI产品。我可以为您解答贷款首付、利率政策、申请材料等问题。请问有汽车金融相关的问题吗？",
				Reason: "off_topic:competitor",
			}
		}
	}

	// 2. 脏话/侮辱
	for _, w := range profanityWords {
		if strings.Contains(lower, w) {
			return GuardResult{
				Action: GuardReject,
				Reply:  "请注意您的用语。我是汽车金融客服助手，很乐意为您解答贷款首付、利率、申请材料等专业问题。请问有什么可以帮您？",
				Reason: "profanity",
			}
		}
	}

	// 3. 无关话题
	for _, kw := range offTopicKeywords {
		if strings.Contains(lower, kw) {
			return GuardResult{
				Action: GuardReject,
				Reply:  "抱歉，我是汽车金融客服助手，无法回答此类问题。我可以为您解答贷款首付比例、利率政策、申请材料、审批流程等问题。请问您有汽车金融相关的问题吗？",
				Reason: "off_topic",
			}
		}
	}

	// 4. 纯英文输入 → 引导用中文
	allRunes := []rune(trimmed)
	asciiCount := 0
	for _, r := range allRunes {
		if r < 128 { asciiCount++ }
	}
	if asciiCount > len(allRunes)*3/4 && len(allRunes) > 8 {
		return GuardResult{
			Action: GuardShortcut,
			Reply:  "您好，请使用中文提问，我能更好地为您服务。比如：新车贷款首付多少？利率是多少？需要什么材料？",
			Reason: "english_input",
		}
	}

	return GuardResult{Action: GuardPass}
}

// 业务关键词展开（短关键词 → 引导用户展开提问）。
var businessKeywords = map[string]string{
	"首付":   "关于首付，我可以为您解答：新车最低首付比例是多少？不同首付比例有什么影响？请告诉我您具体想了解的。",
	"利率":   "关于利率，我可以为您解答：当前利率范围是多少？不同信用等级的利率差异？请告诉我您具体想了解的。",
	"材料":   "关于申请材料，我可以为您列出：个人客户/个体户/企业客户分别需要什么材料？请告诉我您的客户类型。",
	"流程":   "关于业务流程，我可以为您说明：从申请到放款的全流程步骤和时间。请问您想了解哪个环节？",
	"审批":   "关于审批，我可以为您解答：审批需要多长时间？审批结果有哪几种？请告诉我您具体的问题。",
	"还款":   "关于还款，我可以为您解答：还款方式有哪些？提前还款有什么条件？逾期罚息是多少？",
	"期限":   "关于贷款期限，我可以为您解答：最短和最长分别多少期？不同期限对利率有什么影响？",
	"月供":   "关于月供，我可以使用金融计算工具为您试算。请告诉我：贷款金额、利率和期限。",
}

// 业务引导话术（在模型回答后追加一个后续问题，引导用户继续）。
// 精简为单行，不重复追问，避免回答过长。
var businessGuideSnippets = map[string]string{
	"首付": "\n您是否符合准入条件？需要查看申请材料清单吗？",
	"利率": "\n想了解具体月供？可以帮您试算。",
	"材料": "\n材料准备好后，需要帮您试算月供吗？",
	"流程": "\n需要查看申请材料或试算月供吗？",
	"审批": "\n想开始申请？可以帮您查看所需材料。",
	"期限": "\n想了解不同期限的月供？可以帮您试算。",
	"还款": "\n需要试算月供吗？请告诉我贷款金额和期限。",
}

// AppendBusinessGuide 在模型回答后追加一个后续引导问题。
func AppendBusinessGuide(answer string) string {
	if answer == "" {
		return answer
	}
	lower := strings.ToLower(answer)
	for keyword, guide := range businessGuideSnippets {
		if strings.Contains(lower, keyword) || strings.Contains(answer, keyword) {
			if !strings.Contains(answer, guide) {
				return answer + guide
			}
			return answer
		}
	}
	return answer + "\n请问还有其他问题可以帮您？"
}

// CheckOutput 对模型输出做后检查。
// 返回安全回复（如有问题则替换）。
// 检查项：空回答、脏话、歧视/暴力/违法建议。
func CheckOutput(answer string) (safe bool, replacement string) {
	if answer == "" {
		return false, "抱歉，我暂时无法回答这个问题。建议您联系人工客服获取帮助。"
	}
	lower := strings.ToLower(answer)
	// 检查输出是否含脏话
	for _, w := range profanityWords {
		if strings.Contains(lower, w) {
			return false, "抱歉，关于您的问题，建议您联系人工客服获取准确信息。"
		}
	}
	// 检查歧视/暴力/违法建议（生成式AI管理办法要求）
	for _, w := range sensitiveOutputPatterns {
		if strings.Contains(lower, w) {
			return false, "抱歉，该内容涉及敏感话题，我无法提供相关建议。如有汽车金融问题，我很乐意为您解答。"
		}
	}
	return true, answer
}

// sensitiveOutputPatterns 输出敏感内容检测（歧视/暴力/违法/政治/色情）。
// 命中时替换为安全回复，满足《生成式AI管理办法》+ GB/T 45654 内容安全要求。
var sensitiveOutputPatterns = []string{
	// 暴力/伤害
	"杀", "自杀", "自残", "暴力", "伤害他人", "bomb", "爆炸物",
	"怎么制造", "如何制造武器", "枪支", "弹药",
	// 违法
	"洗钱", "诈骗", "骗贷", "造假", "伪造", "偷税", "漏税",
	"信用卡套现", "套现", "高利贷", "套路贷", "传销",
	"贩毒", "吸毒", "赌博", "行贿", "受贿",
	// 歧视
	"种族歧视", "性别歧视", "地域歧视", "残疾人歧视",
	// 政治敏感
	"习近平", "毛泽东", "国家领导人", "反动", "颠覆",
	"六四", "天安门", "法轮功", "藏独", "疆独", "台独",
	// 色情
	"色情", "porn", "色情内容", "裸体", "性服务",
}
